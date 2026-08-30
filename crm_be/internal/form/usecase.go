package form

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authz"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/captcha"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/formtoken"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Usecase depends only on Store (port.go), captcha.Verifier (a shared
// infra package, imported directly the same way auth imports
// mailer.Mailer — not another domain), formTokenSecret (a raw key, not
// a package), and LeadCreator (port.go's bridge, #87) — never on
// *pgxpool.Pool or pgx.Tx directly (ADR-011).
type Usecase struct {
	store           Store
	captchaVerifier captcha.Verifier
	formTokenSecret []byte
	leadCreator     LeadCreator
}

func NewUsecase(store Store, captchaVerifier captcha.Verifier, formTokenSecret []byte, leadCreator LeadCreator) *Usecase {
	return &Usecase{
		store:           store,
		captchaVerifier: captchaVerifier,
		formTokenSecret: formTokenSecret,
		leadCreator:     leadCreator,
	}
}

// CreateInput is Create's argument. Fields nil means DefaultFields() —
// the caller doesn't have to know the shape of a blank-but-usable form.
type CreateInput struct {
	Name   string
	Fields *Fields
}

// Create issues a new form and its public_key. Unlike apikey.Create,
// there is no raw secret to hand back exactly once — public_key is
// PART of the persisted row (Form.PublicKey), readable again any time
// through List/Get, because it was never a secret to begin with (D3).
func (u *Usecase) Create(ctx context.Context, t tenant.Context, in CreateInput) (*Form, error) {
	if err := authz.Require(t, authz.ActionFormCreate); err != nil {
		return nil, err
	}
	if in.Name == "" {
		return nil, httpx.NewValidationError(httpx.ErrorDetail{Field: "name", Code: "required"})
	}

	fields := DefaultFields()
	if in.Fields != nil {
		fields = *in.Fields
	}
	if err := fields.Validate(); err != nil {
		return nil, httpx.NewValidationError(httpx.ErrorDetail{Field: "fields", Code: "invalid_value"})
	}

	publicKey, err := generate()
	if err != nil {
		return nil, fmt.Errorf("form: create: %w", err)
	}

	f := &Form{
		ID:                    uuid.Must(uuid.NewV7()),
		OrganizationID:        t.OrganizationID,
		PublicKey:             publicKey,
		Name:                  in.Name,
		Fields:                fields,
		AllowedOrigins:        []string{},
		CreatedByMembershipID: t.MembershipID,
	}

	txErr := u.store.InTx(ctx, func(r Repos) error {
		if err := r.Form.Create(ctx, t, f); err != nil {
			return err
		}
		return r.Audit.Record(ctx, t, t.MembershipID, "form.created")
	})
	if txErr != nil {
		return nil, fmt.Errorf("form: create: %w", txErr)
	}

	return f, nil
}

func (u *Usecase) List(ctx context.Context, t tenant.Context) ([]*Form, error) {
	if err := authz.Require(t, authz.ActionFormList); err != nil {
		return nil, err
	}
	forms, err := u.store.Repos().Form.FindByOrg(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("form: list: %w", err)
	}
	return forms, nil
}

func (u *Usecase) Get(ctx context.Context, t tenant.Context, id uuid.UUID) (*Form, error) {
	if err := authz.Require(t, authz.ActionFormRead); err != nil {
		return nil, err
	}
	f, err := u.store.Repos().Form.FindByID(ctx, t, id)
	if err != nil {
		return nil, err // httpx.ErrNotFound for cross-org or missing
	}
	return f, nil
}

func (u *Usecase) Update(ctx context.Context, t tenant.Context, id uuid.UUID, in UpdateInput) (*Form, error) {
	if err := authz.Require(t, authz.ActionFormUpdate); err != nil {
		return nil, err
	}
	if in.Fields != nil {
		if err := in.Fields.Validate(); err != nil {
			return nil, httpx.NewValidationError(httpx.ErrorDetail{Field: "fields", Code: "invalid_value"})
		}
	}

	f, err := u.store.Repos().Form.Update(ctx, t, id, in)
	if err != nil {
		return nil, err // httpx.ErrNotFound for cross-org or missing
	}
	return f, nil
}

// Delete is safe to call repeatedly, but NOT idempotent-success the way
// apikey.Revoke is: unlike api_keys (no deleted_at at all, so a revoked
// row stays findable), forms ARE soft-deleted, and FindByID excludes
// deleted rows by the same rule that excludes cross-org ones (Rule #6:
// a deleted form must be indistinguishable from one that never
// existed, even to the organization that deleted it). So the FIRST
// Delete call finds the row and removes it; every call after that gets
// 404 from this same FindByID check, exactly like calling it on an id
// that never existed — same externally-observable shape as
// customer.Usecase.Delete, which reaches the same outcome by checking
// RowsAffected in the repository instead of pre-checking here.
func (u *Usecase) Delete(ctx context.Context, t tenant.Context, id uuid.UUID) error {
	if err := authz.Require(t, authz.ActionFormDelete); err != nil {
		return err
	}
	if _, err := u.store.Repos().Form.FindByID(ctx, t, id); err != nil {
		return err
	}

	return u.store.InTx(ctx, func(r Repos) error {
		if err := r.Form.Delete(ctx, t, id); err != nil {
			return err
		}
		return r.Audit.Record(ctx, t, t.MembershipID, "form.deleted")
	})
}

// ResolvePublicKey resolves a public_key into the form it names and the
// tenant.Context a submission through it acts as — the role
// apikey.Usecase.ResolveAPIKey plays for Authorization-header
// credentials, called directly by the submit handler rather than
// through shared middleware, because public_key lives in the URL path,
// not a header (TD §3). MembershipID, UserID, Role, Scopes, and APIKeyID
// stay unset — a form has no identity beyond the organization it
// belongs to and the row that names it.
//
// Every failure (key not found, form soft-deleted) collapses to the
// same httpx.ErrNotFound (from Repository.FindByPublicKey) — a wrong
// key and a deleted form must be indistinguishable, same reasoning
// apikey.ResolveAPIKey's identical-401 design already established for
// its own credential (Rule #6 applied to a credential rather than a
// resource). Has one caller today (Submit, below); #88's embed page is
// its second, exactly the FindByPublicKey-built-for-#87 precedent
// repeated one level up.
func (u *Usecase) ResolvePublicKey(ctx context.Context, publicKey string) (*Form, tenant.Context, error) {
	found, err := u.store.Repos().Form.FindByPublicKey(ctx, publicKey)
	if err != nil {
		return nil, tenant.Context{}, err
	}
	return found, tenant.Context{
		OrganizationID: found.OrganizationID,
		PrincipalType:  tenant.PrincipalPublicForm,
		FormID:         &found.ID,
	}, nil
}

// SubmitInput is Submit's argument — everything the handler already
// extracted from the HTTP request before calling in. TD §5's alur steps
// 1-3 (both rate limits, the 32KB body cap) already happened in the
// handler by the time Submit runs; Submit picks up at step 4
// (ResolvePublicKey) and owns everything through step 10 (lead
// creation).
type SubmitInput struct {
	Name    string
	Email   *string
	Phone   *string
	Company *string
	// Message maps to lead.CreateLeadInput's Notes — "message" is the
	// form field's own name (ADR-005's six fixed keys), "notes" is
	// lead's; the rename happens at the LeadCreator call below, not
	// here.
	Message *string
	// Product has NO Lead column at all (see entity.go's Fields doc
	// comment) — carried here only so a form configured with product
	// marked Required can still be validated against it. It rides along
	// inside RawPayload like any other field with no dedicated column,
	// never reaching LeadCreator as its own argument.
	Product *string

	// Origin is the Origin header verbatim, "" if the request sent none
	// — an absent header is treated exactly like one that doesn't match
	// the allowlist (originAllowed below), not as a pass.
	Origin string
	// HoneypotFilled is true when the hidden honeypot field arrived
	// non-empty — the handler is responsible for knowing which form
	// field name is the honeypot; this package only sees the boolean.
	HoneypotFilled bool
	FormToken      string
	// CaptchaToken is "" when CAPTCHA_PROVIDER=none — NoopVerifier
	// ignores it either way, so Submit never branches on whether
	// CAPTCHA is actually enabled.
	CaptchaToken string
	RemoteIP     string
	// RawPayload is the full submitted field set, re-marshaled as JSON
	// by the handler — NOT the raw application/x-www-form-urlencoded
	// bytes verbatim (unlike the API-key path's RawPayload, which
	// stores the request body byte-for-byte because that body already
	// IS JSON). leads.raw_payload is a jsonb column; url-encoded bytes
	// would fail to insert as anything meaningful. See handler_http.go
	// for exactly which fields are excluded (the honeypot field, the
	// time-trap token, the captcha response — protocol fields, not
	// submission content).
	RawPayload []byte
}

// honeypotFakeLeadID is Submit's answer when HoneypotFilled is true — a
// freshly generated, NEVER PERSISTED id, not uuid.Nil and not a
// sentinel a bot could learn to recognize. #87's own acceptance
// criterion is that a honeypot response is indistinguishable from a
// genuine one, including its shape: returning uuid.Nil (or any fixed
// value) here would make every honeypot response identical to every
// other, a pattern real create responses never have. See Submit's doc
// comment for what this does NOT solve (response timing).
func honeypotFakeLeadID() uuid.UUID {
	return uuid.Must(uuid.NewV7())
}

// Submit runs TD §5's alur steps 4-10 — ResolvePublicKey through lead
// creation. Steps 1-3 (rate limits, body cap) already happened in the
// handler.
//
// Honeypot (step 6) is checked BEFORE the time-trap and CAPTCHA checks
// (steps 7-8), per TD §5's own ordering: "supaya bot tidak pernah
// menerima pesan kesalahan yang bisa dipelajari" — a bot that filled
// the honeypot must never learn anything about field validation, let
// alone burn a real CAPTCHA verification call. ACCEPTED CONSEQUENCE,
// not fixed here: this makes a honeypot-tripped response measurably
// FASTER than a genuine submission whenever CAPTCHA_PROVIDER=turnstile
// (a real submission pays Turnstile's network round trip, up to
// CAPTCHA_TIMEOUT; a honeypot one never reaches that check at all) —
// the acceptance criterion "tidak bisa dibedakan ... termasuk waktu
// respons" is met for status code and body shape, NOT fully for timing
// under Turnstile. Re-ordering the checks to close that gap would mean
// running CAPTCHA verification against submissions already KNOWN to be
// bot traffic, spending real Cloudflare quota on garbage — directly
// against TD §5's own stated reasoning for the current order. This
// tradeoff was raised and confirmed deliberately during #87's
// implementation, not discovered after the fact; see notes.md's "## #87".
func (u *Usecase) Submit(ctx context.Context, publicKey string, in SubmitInput) (uuid.UUID, error) {
	found, t, err := u.ResolvePublicKey(ctx, publicKey)
	if err != nil {
		return uuid.Nil, err
	}

	if !originAllowed(in.Origin, found.AllowedOrigins) {
		return uuid.Nil, originNotAllowedError()
	}

	if in.HoneypotFilled {
		return honeypotFakeLeadID(), nil
	}

	if err := formtoken.Verify(u.formTokenSecret, in.FormToken, found.ID); err != nil {
		return uuid.Nil, formTokenInvalidError()
	}

	if err := u.captchaVerifier.Verify(ctx, in.CaptchaToken, in.RemoteIP); err != nil {
		return uuid.Nil, captchaFailedError()
	}

	if err := validateRequiredFields(found.Fields, in); err != nil {
		return uuid.Nil, err
	}

	// authz.Require(ActionLeadCreate) happens INSIDE leadCreator's real
	// implementation (lead.Usecase.Create, via the publicFormAllows
	// branch TD §4 adds) — Submit never calls authz itself. See
	// LeadCreator's doc comment in port.go.
	leadID, err := u.leadCreator.CreateFromForm(ctx, t, in.Name, in.Email, in.Phone, in.Company, in.Message, in.RawPayload)
	if err != nil {
		return uuid.Nil, err
	}

	// Best-effort, deliberately outside any transaction with the lead
	// create above (the two can't share one — CreateFromForm's real
	// implementation runs lead.Usecase.Create's own InTx, opaque to this
	// package by design, port.go's LeadCreator doc comment). A failure
	// here never fails the response: the lead this counts already
	// exists, and telling the submitter their own request failed
	// because a dashboard-only display counter didn't increment would
	// be actively misleading.
	_ = u.store.Repos().Form.IncrementSubmitCount(ctx, t, found.ID)

	return leadID, nil
}

// originAllowed fails closed on both an absent Origin header and an
// empty allowlist — TD §7 states the empty-allowlist case explicitly
// for the embed page's frame-ancestors ("form yang belum dikonfigurasi
// tidak bisa di-iframe di mana pun — gagal tertutup, bukan terbuka");
// the identical reasoning applies to submission: a form installed
// nowhere yet shouldn't accept submissions from anywhere either.
func originAllowed(origin string, allowed []string) bool {
	if origin == "" {
		return false
	}
	return slices.Contains(allowed, origin)
}

// validateRequiredFields checks TD §5 step 9 — every field fields marks
// Required must have a non-blank submitted value. Unknown/disabled
// fields are never checked; a field disabled after being required in
// the past simply stops being enforced, matching Fields.Validate's own
// "Required implies Enabled" invariant (entity.go) that made this
// combination impossible to configure in the first place.
func validateRequiredFields(fields Fields, in SubmitInput) error {
	values := map[FieldKey]*string{
		FieldName:    &in.Name,
		FieldEmail:   in.Email,
		FieldPhone:   in.Phone,
		FieldCompany: in.Company,
		FieldMessage: in.Message,
		FieldProduct: in.Product,
	}
	for _, key := range AllFieldKeys {
		cfg, ok := fields[key]
		if !ok || !cfg.Required {
			continue
		}
		value := values[key]
		if value == nil || strings.TrimSpace(*value) == "" {
			return httpx.NewValidationError(httpx.ErrorDetail{Field: string(key), Code: "required"})
		}
	}
	return nil
}

func originNotAllowedError() error {
	return &httpx.DomainError{
		Status:  http.StatusForbidden,
		Code:    "origin_not_allowed",
		Message: "Origin permintaan tidak diizinkan untuk form ini.",
	}
}

func formTokenInvalidError() error {
	return &httpx.DomainError{
		Status:  http.StatusBadRequest,
		Code:    "form_token_invalid",
		Message: "Token form tidak valid atau sudah kedaluwarsa.",
	}
}

func captchaFailedError() error {
	return &httpx.DomainError{
		Status:  http.StatusBadRequest,
		Code:    "captcha_failed",
		Message: "Verifikasi CAPTCHA gagal.",
	}
}
