package form

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
