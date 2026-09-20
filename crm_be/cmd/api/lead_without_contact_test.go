package main

// Issue #143. freeze.md accepts a lead with no contact at all — refusing one
// at the ingest point would throw away a customer — and the UI now marks such
// leads "Belum ada kontak" instead. That marker is only honest if ingest keeps
// accepting them, and a later "improvement" that starts requiring an email or
// phone at the API or the public form would silently reverse a decision the
// product owner made on purpose. These tests are that guard: they change no
// behaviour, they pin it.
//
// The split they protect: only a lead created BY HAND is ever asked about its
// contact (a confirmation in the dashboard dialog). A form or API lead has
// nobody to ask.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/form"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// storedContact reads what was actually stored, and whether the email is absent — null OR
// blank. The UI's hasContact treats both the same, and that is only justified
// if the backend can really produce either.
func storedContact(t *testing.T, pool *pgxpool.Pool, leadID string) (email, phone *string, emailBlank bool) {
	t.Helper()
	err := pool.QueryRow(context.Background(),
		`SELECT email, phone, (email IS NULL OR btrim(email) = '') FROM leads WHERE id = $1`, leadID,
	).Scan(&email, &phone, &emailBlank)
	if err != nil {
		t.Fatalf("read stored contact: %v", err)
	}
	return email, phone, emailBlank
}

func createdLeadID(t *testing.T, body []byte) string {
	t.Helper()
	var resp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil || resp.Data.ID == "" {
		t.Fatalf("decode created lead: %v (%s)", err, body)
	}
	return resp.Data.ID
}

func TestLeadWithoutContact_APIKey_NameOnly_IsAcceptedWithNoContactStored(t *testing.T) {
	r, pool := newIsolationRouter(t)
	org, _, ownerMembership := seedOrgOwner(t, pool, "No Contact API Org", "nc-api-owner@example.com")
	raw, _ := seedRealAPIKey(t, pool, org, ownerMembership)

	w := doBearerRequest(r, http.MethodPost, "/v1/leads", raw, map[string]string{"name": "Tanpa Kontak"})
	if w.Code != http.StatusCreated {
		t.Fatalf("a lead with no contact must still be accepted from the API, got %d: %s", w.Code, w.Body.String())
	}

	email, phone, _ := storedContact(t, pool, createdLeadID(t, w.Body.Bytes()))
	if email != nil || phone != nil {
		t.Errorf("nothing was sent, so nothing may be stored; got email=%v phone=%v", email, phone)
	}
}

// crm_be trims an email and stores what is left, so "   " can come back as ""
// rather than null. The dashboard and mobile therefore treat a blank contact
// as absent; this test is what makes that necessary rather than fussy.
func TestLeadWithoutContact_APIKey_WhitespaceOnlyEmail_IsAcceptedAndStoredBlank(t *testing.T) {
	r, pool := newIsolationRouter(t)
	org, _, ownerMembership := seedOrgOwner(t, pool, "Blank Email Org", "nc-blank-owner@example.com")
	raw, _ := seedRealAPIKey(t, pool, org, ownerMembership)

	w := doBearerRequest(r, http.MethodPost, "/v1/leads", raw, map[string]string{"name": "Email Kosong", "email": "   "})
	if w.Code != http.StatusCreated {
		t.Fatalf("a whitespace-only email must not be refused, got %d: %s", w.Code, w.Body.String())
	}

	_, _, emailBlank := storedContact(t, pool, createdLeadID(t, w.Body.Bytes()))
	if !emailBlank {
		t.Error("expected the stored email to be null or blank — if it kept the spaces, the UI's blank-handling is guarding against something that no longer happens")
	}
}

// A form is the one ingest path where contact CAN be required, and the default
// form does: DefaultFields marks phone Required, so a stock form never produces
// a contactless lead. That is the owner's control, and it is why the backend
// itself does not need to refuse one.
func TestLeadWithoutContact_DefaultForm_AsksForAPhone(t *testing.T) {
	r, pool := newIsolationRouter(t)
	org, _, ownerMembership := seedOrgOwner(t, pool, "Default Form Org", "nc-default-owner@example.com")
	f := seedRealForm(t, pool, org, ownerMembership, []string{"https://customer-site.example"})

	values := url.Values{"name": {"Tanpa Nomor"}, "form_token": {validFormToken(f.ID)}}
	w := doFormPost(r, "/v1/forms/"+f.PublicKey+"/submit", "https://customer-site.example", values)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("the default form requires a phone, so a name-only submission must be refused there, got %d: %s", w.Code, w.Body.String())
	}
}

// A form whose owner chose NOT to require any contact still accepts a
// name-only submission: the backend adds no requirement of its own. Deleting
// this guarantee would throw a customer away at the ingest point (freeze.md).
func TestLeadWithoutContact_FormWithNoRequiredContact_NameOnly_IsAccepted(t *testing.T) {
	r, pool := newIsolationRouter(t)
	org, _, ownerMembership := seedOrgOwner(t, pool, "No Contact Form Org", "nc-form-owner@example.com")
	f := seedRealForm(t, pool, org, ownerMembership, []string{"https://customer-site.example"})

	fields := form.DefaultFields()
	fields[form.FieldPhone] = form.FieldConfig{Enabled: true, Required: false, Label: "Nomor WhatsApp"}
	u := form.NewUsecase(newFormStore(pool), nil, []byte(isolationFormTokenSecret), nil, alwaysOpenPlanGate{})
	actor := tenant.Context{OrganizationID: org, PrincipalType: tenant.PrincipalUser, MembershipID: &ownerMembership, Role: tenant.RoleOwner}
	if _, err := u.Update(context.Background(), actor, f.ID, form.UpdateInput{Fields: &fields}); err != nil {
		t.Fatalf("make the phone optional: %v", err)
	}

	values := url.Values{
		"name":       {"Pengunjung Tanpa Kontak"},
		"form_token": {validFormToken(f.ID)},
	}
	w := doFormPost(r, "/v1/forms/"+f.PublicKey+"/submit", "https://customer-site.example", values)
	if w.Code != http.StatusCreated {
		t.Fatalf("a form that requires no contact must still accept a name-only submission, got %d: %s", w.Code, w.Body.String())
	}

	email, phone, _ := storedContact(t, pool, createdLeadID(t, w.Body.Bytes()))
	if email != nil || phone != nil {
		t.Errorf("nothing was submitted, so nothing may be stored; got email=%v phone=%v", email, phone)
	}
}
