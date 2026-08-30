package form

import (
	"encoding/json"
	"errors"
	"net/http"

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
}

// NewHandler takes two SEPARATE rate limiters (TD §6 keputusan D4) —
// one keyed by IP, one by public_key — not one limiter checked twice,
// since the two axes have independently configured budgets
// (FORM_SUBMIT_RATE_LIMIT_IP vs _FORM).
func NewHandler(usecase *Usecase, submitIPLimit, submitKeyLimit publicRateLimiter) *Handler {
	return &Handler{usecase: usecase, submitIPLimit: submitIPLimit, submitKeyLimit: submitKeyLimit}
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
