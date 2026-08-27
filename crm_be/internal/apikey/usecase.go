package apikey

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authz"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Usecase depends only on Store (port.go), never on *pgxpool.Pool or
// pgx.Tx directly (ADR-011).
type Usecase struct {
	store Store
}

func NewUsecase(store Store) *Usecase {
	return &Usecase{store: store}
}

// CreateInput is Create's argument. Scopes empty means "the only scope
// that exists" (ADR-004 aturan #4) — the caller doesn't have to know the
// enum has exactly one member today.
type CreateInput struct {
	Name   string
	Scopes []string
}

// Create issues a new credential. The raw, unhashed secret is returned
// alongside the persisted row and NOWHERE ELSE (Rule #21) — it is never
// stored on APIKey, never logged, and the caller (handler_http.go) must
// put it in the response body of this one call and then let it go out
// of scope.
func (u *Usecase) Create(ctx context.Context, t tenant.Context, in CreateInput) (result *APIKey, raw string, err error) {
	if err := authz.Require(t, authz.ActionAPIKeyCreate); err != nil {
		return nil, "", err
	}
	if in.Name == "" {
		return nil, "", httpx.NewValidationError(httpx.ErrorDetail{Field: "name", Code: "required"})
	}

	scopes := in.Scopes
	if len(scopes) == 0 {
		scopes = []string{ScopeLeadsWrite}
	}
	for _, s := range scopes {
		if s != ScopeLeadsWrite {
			return nil, "", httpx.NewValidationError(httpx.ErrorDetail{Field: "scopes", Code: "invalid_value"})
		}
	}

	gen, err := generate()
	if err != nil {
		return nil, "", fmt.Errorf("apikey: create: %w", err)
	}

	k := &APIKey{
		ID:                    uuid.Must(uuid.NewV7()),
		OrganizationID:        t.OrganizationID,
		KeyID:                 gen.keyID,
		SecretHash:            gen.secretHash,
		KeyPrefix:             gen.keyPrefix,
		Name:                  in.Name,
		Scopes:                scopes,
		CreatedByMembershipID: t.MembershipID,
	}

	txErr := u.store.InTx(ctx, func(r Repos) error {
		if err := r.APIKey.Create(ctx, t, k); err != nil {
			return err
		}
		return r.Audit.Record(ctx, t, t.MembershipID, "api_key.created")
	})
	if txErr != nil {
		return nil, "", fmt.Errorf("apikey: create: %w", txErr)
	}

	return k, rawCredential(gen.keyID, gen.rawSecret), nil
}

func (u *Usecase) List(ctx context.Context, t tenant.Context) ([]*APIKey, error) {
	if err := authz.Require(t, authz.ActionAPIKeyList); err != nil {
		return nil, err
	}
	keys, err := u.store.Repos().APIKey.FindByOrg(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("apikey: list: %w", err)
	}
	return keys, nil
}

// Revoke marks a credential unusable. Idempotent by design — revoking an
// already-revoked key still succeeds (204), matching TD §9's "revoke
// kedua tetap 204": FindByID confirms the row exists at all (404 for
// missing/cross-org, Rule #6), and the repository UPDATE simply has no
// rows to affect when revoked_at is already set — that's not an error,
// existence was already proven.
func (u *Usecase) Revoke(ctx context.Context, t tenant.Context, id uuid.UUID) error {
	if err := authz.Require(t, authz.ActionAPIKeyRevoke); err != nil {
		return err
	}
	if _, err := u.store.Repos().APIKey.FindByID(ctx, t, id); err != nil {
		return err // httpx.ErrNotFound for cross-org or missing
	}

	return u.store.InTx(ctx, func(r Repos) error {
		if err := r.APIKey.Revoke(ctx, t, id); err != nil {
			return err
		}
		return r.Audit.Record(ctx, t, t.MembershipID, "api_key.revoked")
	})
}
