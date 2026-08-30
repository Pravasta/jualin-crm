// Package form implements the third credential this product issues
// (public_key, ADR-005) and the tables that back it. This issue (#85)
// covers only management — create/list/read/update/delete as principal
// user (Owner/Admin). Resolving a public_key into a tenant.Context and
// the public POST /v1/forms/{public_key}/submit endpoint that accepts
// it are #87's scope; the embed page is #88's. See the doc comments on
// generate/parsePublicKey below for the pieces built now but not yet
// wired to any public HTTP path.
package form

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Form struct {
	ID                    uuid.UUID
	OrganizationID        uuid.UUID
	PublicKey             string
	Name                  string
	Fields                Fields
	AllowedOrigins        []string
	SubmitCount           int
	CreatedByMembershipID *uuid.UUID
	CreatedAt             time.Time
	UpdatedAt             time.Time
	DeletedAt             *time.Time
}

// UpdateInput is Repository.Update's argument — nil means "leave
// unchanged", same convention customer.UpdateInput uses. AllowedOrigins
// is *[]string rather than []string so "clear the allowlist" (an empty
// but non-nil slice) is distinguishable from "don't touch it" (nil).
type UpdateInput struct {
	Name           *string
	Fields         *Fields
	AllowedOrigins *[]string
}

// FieldKey identifies one of the six fixed fields ADR-005 allows —
// there is no form builder in this product, and this type exists so
// "which fields exist" is a closed Go enum, not a string a caller could
// misspell into a seventh field that silently never renders.
type FieldKey string

const (
	FieldName    FieldKey = "name"
	FieldEmail   FieldKey = "email"
	FieldPhone   FieldKey = "phone"
	FieldCompany FieldKey = "company"
	FieldMessage FieldKey = "message"
	FieldProduct FieldKey = "product"
)

// AllFieldKeys is the closed list Fields.Validate checks against —
// declared once so a typo in a literal string can't silently create an
// eighth key that renders nothing and validates nothing (TD §1).
var AllFieldKeys = []FieldKey{FieldName, FieldEmail, FieldPhone, FieldCompany, FieldMessage, FieldProduct}

// FieldConfig is one field's display configuration — never data the
// submitter sent, only how the form asks for it.
type FieldConfig struct {
	Enabled  bool   `json:"enabled"`
	Required bool   `json:"required"`
	Label    string `json:"label"`
}

// Fields is forms.fields' Go shape (TD §1) — a fixed set of six keys,
// never queried or filtered on, only read whole when rendering the
// embed page (#88) and validating a submission (#87). That's the
// written reason Aturan #17 requires for storing it as JSONB instead of
// eighteen columns (six fields times three attributes) that would only
// ever be read together.
type Fields map[FieldKey]FieldConfig

// DefaultFields seeds a new form with every field enabled and a
// sensible Indonesian label — Owner adjusts from there rather than
// starting from a blank, unusable form.
func DefaultFields() Fields {
	return Fields{
		FieldName:    {Enabled: true, Required: true, Label: "Nama Lengkap"},
		FieldEmail:   {Enabled: true, Required: false, Label: "Email"},
		FieldPhone:   {Enabled: true, Required: true, Label: "Nomor WhatsApp"},
		FieldCompany: {Enabled: false, Required: false, Label: "Perusahaan"},
		FieldMessage: {Enabled: true, Required: false, Label: "Pesan"},
		FieldProduct: {Enabled: false, Required: false, Label: "Layanan Diminati"},
	}
}

// Validate reports the first problem found, or nil. Called by Usecase
// before every Create/Update — never by the repository, which persists
// whatever it's given (ADR-011: validation is usecase's job, not
// storage's).
func (f Fields) Validate() error {
	for key, cfg := range f {
		if !isKnownFieldKey(key) {
			return fmt.Errorf("form: unknown field key %q", key)
		}
		if cfg.Required && !cfg.Enabled {
			return fmt.Errorf("form: field %q is required but not enabled", key)
		}
		if cfg.Enabled && cfg.Label == "" {
			return fmt.Errorf("form: field %q is enabled but has no label", key)
		}
	}
	return nil
}

func isKnownFieldKey(key FieldKey) bool {
	for _, k := range AllFieldKeys {
		if k == key {
			return true
		}
	}
	return false
}

// publicKeyRawBytes/publicKeyPrefix are D3's format: pk_ + 22
// base64url characters (16 raw bytes, 128-bit — plenty for a value
// that is looked up by exact match, never brute-forced the way a
// password would be, since it isn't secret at all; see the doc comment
// on generate below for why plaintext storage is correct here, not a
// shortcut).
const (
	publicKeyRawBytes = 16
	publicKeyPrefix   = "pk_"
)

// generate creates one new public_key. Unlike apikey.generate, there is
// no separate secret half and nothing here is ever hashed — D3 records
// why: public_key is designed to be readable by anyone who views the
// source of a page that embeds the form (ADR-005), so hashing it would
// only make the one legitimate lookup (by the value itself) impossible
// while protecting against a threat — guessing a secret — that doesn't
// exist for a value that was never secret.
func generate() (string, error) {
	raw := make([]byte, publicKeyRawBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("form: generate public_key: %w", err)
	}
	return publicKeyPrefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

// parsePublicKey reports whether raw has the shape D3 defines — prefix
// and length only, since there is no signature or checksum to verify
// (there is nothing secret to protect against forgery of: a forged
// public_key that doesn't exist in the table simply won't be found by
// FindByPublicKey, #87's own defense). Used by entity_test.go to prove
// generate's output round-trips; #87 will use the same shape check as a
// cheap early rejection before ever touching the database.
func parsePublicKey(raw string) bool {
	if len(raw) <= len(publicKeyPrefix) {
		return false
	}
	if raw[:len(publicKeyPrefix)] != publicKeyPrefix {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(raw[len(publicKeyPrefix):])
	return err == nil
}
