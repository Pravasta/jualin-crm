package form

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authn"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/ratelimit"
)

// maxSubmitBodyBytes is TD §5 step 3's 32KB cap on
// POST /v1/forms/{public_key}/submit — half of the public API's own
// 64KB (internal/lead/handler_http.go's maxPublicLeadBodyBytes), since
// an anonymous browser submission has no legitimate reason to carry
// more than a handful of short text fields, unlike an integrator's API
// payload which TD Phase 4 §5 already allows more room for.
const maxSubmitBodyBytes = 32 * 1024

// Wire contract for the embed page (#88) — the literal field names a
// submission's application/x-www-form-urlencoded body must use. The six
// content field names match FieldKey's own string values exactly
// (fieldNameKey == string(FieldName), etc.); honeypotFieldName and
// formTokenFieldName are this package's own choice, and
// captchaResponseFieldName is Cloudflare Turnstile's fixed convention
// (its widget injects a hidden input with this exact name into whatever
// form it's placed in — not something this codebase gets to choose).
// #88 MUST render its <input name="..."> attributes to match these
// exactly, or every submission fails at step 9 (required field
// "missing") or is silently misread as one of the excluded fields.
const (
	fieldNameKey    = "name"
	fieldEmailKey   = "email"
	fieldPhoneKey   = "phone"
	fieldCompanyKey = "company"
	fieldMessageKey = "message"
	fieldProductKey = "product"

	// honeypotFieldName is deliberately named like a plausible real
	// field — bots that fill every input they can find are exactly what
	// this layer catches (TD §6); a name like "_honeypot" would only
	// catch bots dumb enough to be caught by literally anything.
	honeypotFieldName  = "website"
	formTokenFieldName = "form_token"
	// captchaResponseFieldName — Cloudflare's own convention, not this
	// codebase's choice. See https://developers.cloudflare.com/turnstile/.
	captchaResponseFieldName = "cf-turnstile-response"
)

// publicRateLimiter is declared here (the consumer, ADR-011) — same
// shape as lead's own local interface of the same name (Phase 4 #47),
// declared independently rather than shared: three lines, not worth a
// common package for two call sites.
type publicRateLimiter interface {
	Take(key string) ratelimit.Result
}

type Handler struct {
	usecase        *Usecase
	submitIPLimit  publicRateLimiter
	submitKeyLimit publicRateLimiter
	// captchaProvider/turnstileSiteKey are ONLY consulted by the embed
	// handler (#88) to decide whether to render Turnstile's widget and
	// what public site key to embed — never used for verification
	// (Usecase.Submit already holds the real captcha.Verifier for that).
	// Deliberately NOT on Usecase: TurnstileSiteKey is a rendering
	// concern (it's meant to be embedded in client-facing HTML, never a
	// secret, unlike TurnstileSecretKey), not business logic.
	captchaProvider  string
	turnstileSiteKey string
}

// NewHandler takes two SEPARATE rate limiters (TD §6 keputusan D4) —
// one keyed by IP, one by public_key — not one limiter checked twice,
// since the two axes have independently configured budgets
// (FORM_SUBMIT_RATE_LIMIT_IP vs _FORM). captchaProvider/turnstileSiteKey
// map directly to CAPTCHA_PROVIDER/TURNSTILE_SITE_KEY (config.go).
func NewHandler(usecase *Usecase, submitIPLimit, submitKeyLimit publicRateLimiter, captchaProvider, turnstileSiteKey string) *Handler {
	return &Handler{
		usecase: usecase, submitIPLimit: submitIPLimit, submitKeyLimit: submitKeyLimit,
		captchaProvider: captchaProvider, turnstileSiteKey: turnstileSiteKey,
	}
}

// RegisterRoutes mounts /v1/forms as two SEPARATE gin groups sharing
// the same prefix and the same :id wildcard NAME — required by gin
// itself (a route tree can't have two different wildcard names at the
// same segment position), and the exact jebakan TD §8 calls out by
// name, with precedent in internal/task and internal/activity sharing
// /v1/leads/:id from separate packages. Here it's the same package:
// the management group (GET/PATCH/DELETE /v1/forms/:id, principal
// user, authMW) treats :id as a form's row id; the public group
// (POST /v1/forms/:id/submit) treats the identical :id segment as a
// public_key instead — deliberately NOT parsed as a UUID (submit's
// handler never calls uuid.Parse on it, unlike every other :id handler
// below).
//
// The public group carries NO auth middleware at all (§3) — public_key
// is a path segment, not an Authorization header, so there is nothing
// for authn.Middleware/MiddlewareWithAPIKey to read here.
func (h *Handler) RegisterRoutes(r gin.IRouter, authMW gin.HandlerFunc) {
	g := r.Group("/v1/forms")
	g.Use(authMW)
	g.POST("", h.create)
	g.GET("", h.list)
	g.GET("/:id", h.get)
	g.PATCH("/:id", h.update)
	g.DELETE("/:id", h.delete)

	public := r.Group("/v1/forms")
	public.POST("/:id/submit", h.submit)

	// GET /embed/{public_key} and GET /embed.js (#88) live OUTSIDE /v1
	// entirely (TD §7: "ia bukan API, ia halaman") — no envelope
	// {data, meta}, no auth middleware, registered directly on r rather
	// than any group. :id here means public_key, same non-UUID
	// treatment as the submit route above.
	r.GET("/embed/:id", h.embed)
	r.GET("/embed.js", h.embedJS)
}

type createRequest struct {
	Name   string  `json:"name"`
	Fields *Fields `json:"fields"`
}

func (h *Handler) create(c *gin.Context) {
	t := authn.TenantFromContext(c)

	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "body", Code: "invalid_json"}))
		return
	}

	f, err := h.usecase.Create(c.Request.Context(), t, CreateInput(req))
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	httpx.OK(c, http.StatusCreated, formJSON(f))
}

func (h *Handler) list(c *gin.Context) {
	t := authn.TenantFromContext(c)

	forms, err := h.usecase.List(c.Request.Context(), t)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	out := make([]gin.H, 0, len(forms))
	for _, f := range forms {
		out = append(out, formJSON(f))
	}
	httpx.OK(c, http.StatusOK, out)
}

func (h *Handler) get(c *gin.Context) {
	t := authn.TenantFromContext(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, httpx.ErrNotFound)
		return
	}

	f, err := h.usecase.Get(c.Request.Context(), t, id)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, formJSON(f))
}

type updateRequest struct {
	Name           *string   `json:"name"`
	Fields         *Fields   `json:"fields"`
	AllowedOrigins *[]string `json:"allowed_origins"`
}

func (h *Handler) update(c *gin.Context) {
	t := authn.TenantFromContext(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, httpx.ErrNotFound)
		return
	}

	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "body", Code: "invalid_json"}))
		return
	}

	f, err := h.usecase.Update(c.Request.Context(), t, id, UpdateInput(req))
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, formJSON(f))
}

func (h *Handler) delete(c *gin.Context) {
	t := authn.TenantFromContext(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.WriteError(c, httpx.ErrNotFound)
		return
	}

	if err := h.usecase.Delete(c.Request.Context(), t, id); err != nil {
		httpx.WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// submit is TD §5's alur, steps 1-3 — the parts that must run BEFORE
// Usecase.Submit even exists as a call, because they're about the raw
// HTTP request, not the resolved form: rate limits first (so a flood
// never reaches body parsing, let alone the database — same reasoning
// Phase 4's public API rate-limits before parsing), then the 32KB body
// cap, then parsing as application/x-www-form-urlencoded — a plain
// HTML <form method="post"> must work with zero JavaScript when
// CAPTCHA_PROVIDER=none, which rules out requiring a JSON body (a
// decision made explicitly for this issue, not assumed; see notes.md's
// "## #87"). Steps 4-10 are entirely Usecase.Submit's (usecase.go).
//
// :id here is NOT parsed as a UUID — see RegisterRoutes' own doc
// comment on why this route shares /v1/forms/:id's wildcard name while
// treating the segment as a public_key string instead.
func (h *Handler) submit(c *gin.Context) {
	publicKey := c.Param("id")

	ipResult := h.submitIPLimit.Take("formsubmit:ip:" + c.ClientIP())
	httpx.SetRateLimitHeaders(c, ipResult)
	if !ipResult.Allowed {
		httpx.RespondError(c, http.StatusTooManyRequests, "rate_limited", "Terlalu banyak percobaan dari alamat IP ini. Coba lagi nanti.")
		return
	}

	keyResult := h.submitKeyLimit.Take("formsubmit:key:" + publicKey)
	// Overwrites the four headers ipResult just set — on a response
	// that passes BOTH checks, the client sees the public_key-scoped
	// numbers; on an IP rejection above, the response already returned
	// before this line runs. TD §6 doesn't specify which axis's numbers
	// win when both pass; this is a deliberate, documented choice, not
	// an oversight (notes.md's "## #87").
	httpx.SetRateLimitHeaders(c, keyResult)
	if !keyResult.Allowed {
		httpx.RespondError(c, http.StatusTooManyRequests, "rate_limited", "Terlalu banyak percobaan untuk form ini. Coba lagi nanti.")
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSubmitBodyBytes)
	if err := c.Request.ParseForm(); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httpx.WriteError(c, payloadTooLargeError())
			return
		}
		httpx.WriteError(c, httpx.NewValidationError(httpx.ErrorDetail{Field: "body", Code: "invalid_form_body"}))
		return
	}
	form := c.Request.PostForm

	in := SubmitInput{
		Name:           form.Get(fieldNameKey),
		Email:          formValuePtr(form, fieldEmailKey),
		Phone:          formValuePtr(form, fieldPhoneKey),
		Company:        formValuePtr(form, fieldCompanyKey),
		Message:        formValuePtr(form, fieldMessageKey),
		Product:        formValuePtr(form, fieldProductKey),
		Origin:         c.GetHeader("Origin"),
		HoneypotFilled: form.Get(honeypotFieldName) != "",
		FormToken:      form.Get(formTokenFieldName),
		CaptchaToken:   form.Get(captchaResponseFieldName),
		RemoteIP:       c.ClientIP(),
		RawPayload:     buildRawPayload(form),
	}

	leadID, err := h.usecase.Submit(c.Request.Context(), publicKey, in)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}
	httpx.OK(c, http.StatusCreated, gin.H{"id": leadID})
}

// formValuePtr returns nil for a field the submitter never sent at all,
// distinct from one sent empty — mirrors every other optional-field
// handler in this codebase (e.g. lead's own createRequest) rather than
// collapsing "absent" and "empty string" into the same zero value.
func formValuePtr(form map[string][]string, key string) *string {
	values, ok := form[key]
	if !ok || len(values) == 0 {
		return nil
	}
	return &values[0]
}

// buildRawPayload re-marshals every submitted field EXCEPT the three
// protocol fields (honeypot, time-trap token, captcha response) as a
// JSON object — NOT the raw url-encoded body bytes verbatim, unlike the
// API-key path's raw_payload (that body already IS JSON; this one
// isn't, and leads.raw_payload is a jsonb column). An unmarshalable
// result here (never expected — url.Values is always plain strings)
// would be a bug, not a caller error, so it's swallowed into an empty
// object rather than failing the whole submission over a cosmetic field.
func buildRawPayload(form map[string][]string) []byte {
	excluded := map[string]bool{
		honeypotFieldName:        true,
		formTokenFieldName:       true,
		captchaResponseFieldName: true,
	}
	payload := make(map[string]string, len(form))
	for key, values := range form {
		if excluded[key] || len(values) == 0 {
			continue
		}
		payload[key] = values[0]
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return []byte("{}")
	}
	return b
}

func payloadTooLargeError() error {
	return &httpx.DomainError{
		Status:  http.StatusRequestEntityTooLarge,
		Code:    "payload_too_large",
		Message: "Isi request melebihi batas 32 KB.",
	}
}

// --- embed page (#88, TD §7 keputusan D1) ---

// embedFieldView is one <input>/<textarea> the template renders — built
// from Form.Fields, never handed the map directly, so the template
// itself never has to know about Go map iteration order (which isn't
// stable) or FieldKey's zero-value semantics.
type embedFieldView struct {
	Key      string
	Label    string
	Required bool
	Type     string // "text" | "email" | "tel" | "textarea"
}

// fieldInputTypes maps each fixed field (ADR-005 — six keys, no form
// builder) to an HTML5 input type/textarea. Not configurable — TD never
// gives forms.fields a "type" a customer could set, only
// enabled/required/label.
var fieldInputTypes = map[FieldKey]string{
	FieldName:    "text",
	FieldEmail:   "email",
	FieldPhone:   "tel",
	FieldCompany: "text",
	FieldMessage: "textarea",
	FieldProduct: "text",
}

// orderedEnabledFields walks AllFieldKeys (entity.go's fixed order) —
// never ranges over Fields directly, since Go map iteration order is
// randomized and the rendered form's field order must be the SAME on
// every request, not shuffled per-render.
func orderedEnabledFields(fields Fields) []embedFieldView {
	views := make([]embedFieldView, 0, len(AllFieldKeys))
	for _, key := range AllFieldKeys {
		cfg, ok := fields[key]
		if !ok || !cfg.Enabled {
			continue
		}
		views = append(views, embedFieldView{
			Key: string(key), Label: cfg.Label, Required: cfg.Required, Type: fieldInputTypes[key],
		})
	}
	return views
}

// embedPageData feeds template/form.gohtml. AllowedOriginsJSON is
// template.JS-typed deliberately — see template.go's doc comment on why
// that's the correct, safe way to inject an already-serialized JSON
// array as a JS expression rather than a JS string literal.
type embedPageData struct {
	FormName           string
	PublicKey          string
	Fields             []embedFieldView
	FormToken          string
	AllowedOriginsJSON template.JS
	CaptchaEnabled     bool
	TurnstileSiteKey   string
	Nonce              string
}

// embed renders GET /embed/{public_key} — TD §7: no auth middleware, no
// {data, meta} envelope, Cache-Control: no-store (the page embeds a
// short-lived time-trap token). :id is public_key, not a UUID — same
// non-parsing as submit's own handler.
//
// A public_key that doesn't resolve gets the exact same 404 a deleted
// one does (Usecase.ResolvePublicKey, unchanged from #87) — still
// written through httpx.WriteError even on this HTML route: a broken
// embed link is a rare edge case, and inventing a second, HTML-flavored
// 404 page solely for it isn't worth the extra surface (Rule #27).
func (h *Handler) embed(c *gin.Context) {
	publicKey := c.Param("id")

	found, _, err := h.usecase.ResolvePublicKey(c.Request.Context(), publicKey)
	if err != nil {
		httpx.WriteError(c, err)
		return
	}

	nonce, err := generateNonce()
	if err != nil {
		httpx.WriteError(c, fmt.Errorf("form: embed: %w", err))
		return
	}

	allowedOriginsJSON, err := json.Marshal(found.AllowedOrigins)
	if err != nil {
		httpx.WriteError(c, fmt.Errorf("form: embed: marshal allowed_origins: %w", err))
		return
	}

	captchaEnabled := h.captchaProvider == "turnstile"
	data := embedPageData{
		FormName:           found.Name,
		PublicKey:          found.PublicKey,
		Fields:             orderedEnabledFields(found.Fields),
		FormToken:          h.usecase.IssueFormToken(found.ID),
		AllowedOriginsJSON: template.JS(allowedOriginsJSON), // #nosec G203 -- already-serialized JSON from encoding/json, not raw user input; template.JS is the documented html/template idiom for this
		CaptchaEnabled:     captchaEnabled,
		TurnstileSiteKey:   h.turnstileSiteKey,
		Nonce:              nonce,
	}

	var buf bytes.Buffer
	if err := formPage.Execute(&buf, data); err != nil {
		httpx.WriteError(c, fmt.Errorf("form: embed: render: %w", err))
		return
	}

	c.Header("Content-Security-Policy", cspHeader(found.AllowedOrigins, nonce, h.captchaProvider))
	// X-Frame-Options is deliberately NEVER set here — it can't express
	// a per-form allowlist the way frame-ancestors can, and setting both
	// would just be redundant everywhere frame-ancestors already covers
	// (TD §7).
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}

// embedJS serves the D8 companion script — a static asset, the same
// bytes for every form, cached publicly (unlike the embed page itself,
// this carries no per-form or per-request state to leak by caching).
func (h *Handler) embedJS(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=3600")
	c.Data(http.StatusOK, "text/javascript; charset=utf-8", embedJS)
}

// cspHeader builds the embed page's Content-Security-Policy — see TD §7
// for the full reasoning behind each directive. frame-ancestors is
// per-form (allowedOrigins, this row only, never a global policy); an
// empty allowlist becomes 'none' — a form nobody has configured an
// origin for yet fails CLOSED, not open (TD §7: "gagal tertutup, bukan
// terbuka"). script-src carries a per-request nonce (never
// 'unsafe-inline' — the auto-resize script is the only inline script
// this page ever emits, and a nonce means an attacker who somehow got
// OTHER markup injected still couldn't get their own script to run),
// plus Cloudflare's own origin when Turnstile is enabled.
func cspHeader(allowedOrigins []string, nonce, captchaProvider string) string {
	frameAncestors := "'none'"
	if len(allowedOrigins) > 0 {
		frameAncestors = strings.Join(allowedOrigins, " ")
	}
	scriptSrc := "'nonce-" + nonce + "'"
	if captchaProvider == "turnstile" {
		scriptSrc += " https://challenges.cloudflare.com"
	}
	return "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; script-src " +
		scriptSrc + "; frame-ancestors " + frameAncestors
}

// generateNonce returns a fresh per-request CSP nonce — never cached or
// reused across requests (the embed page itself is Cache-Control:
// no-store, so there's no risk of a stale nonce being served from a
// cache anyway, but generating fresh every render is what makes the
// nonce mean anything at all: a static, hardcoded nonce would be
// exactly as weak as 'unsafe-inline').
//
// RawURLEncoding, not StdEncoding: std base64's alphabet includes '+',
// '/', '=', all of which html/template HTML-escapes when the nonce
// lands inside the <script nonce="..."> attribute (found by this
// package's own test — a real browser decodes the entities back
// correctly, so it wasn't a functional bug, but there's no reason to
// depend on that when RawURLEncoding's alphabet needs no escaping at
// all and makes the nonce trivially comparable in page source).
func generateNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// formJSON always includes public_key in full — unlike apiKeyJSON,
// which never includes a raw secret, public_key is not a secret at all
// (D3) and is exactly the value the owner needs to copy into the embed
// snippet every time this form is viewed, not just once at creation.
func formJSON(f *Form) gin.H {
	return gin.H{
		"id":                       f.ID,
		"public_key":               f.PublicKey,
		"name":                     f.Name,
		"fields":                   f.Fields,
		"allowed_origins":          f.AllowedOrigins,
		"submit_count":             f.SubmitCount,
		"created_by_membership_id": f.CreatedByMembershipID,
		"created_at":               f.CreatedAt,
		"updated_at":               f.UpdatedAt,
	}
}
