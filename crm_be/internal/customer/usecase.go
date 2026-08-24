package customer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authz"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

const (
	defaultPerPage = 25
	maxPerPage     = 100
)

// Usecase depends only on Store (port.go), never on *pgxpool.Pool or
// pgx.Tx directly (ADR-011).
type Usecase struct {
	store Store
}

func NewUsecase(store Store) *Usecase {
	return &Usecase{store: store}
}

// Convert creates a customer from leadID and records lead_converted in
// the SAME transaction (TD §10/§12) — the lead itself is never
// modified or deleted; converted_from_lead_id is its only link.
func (u *Usecase) Convert(ctx context.Context, t tenant.Context, leadID uuid.UUID) (*Customer, error) {
	if err := authz.Require(t, authz.ActionLeadConvert); err != nil {
		return nil, err
	}

	var created *Customer
	txErr := u.store.InTx(ctx, func(r Repos) error {
		c, err := r.Customer.Convert(ctx, t, leadID, t.MembershipID)
		if err != nil {
			return err
		}
		created = c
		return r.Activity.Record(ctx, t, leadID, "lead_converted", t.MembershipID, map[string]any{"customer_id": c.ID})
	})
	if errors.Is(txErr, ErrLeadNotWon) {
		return nil, invalidStatusTransitionError()
	}
	if errors.Is(txErr, ErrAlreadyConverted) {
		return nil, alreadyConvertedError()
	}
	if errors.Is(txErr, httpx.ErrNotFound) {
		return nil, httpx.ErrNotFound
	}
	if txErr != nil {
		return nil, fmt.Errorf("customer: convert: %w", txErr)
	}
	return created, nil
}

func (u *Usecase) Get(ctx context.Context, t tenant.Context, id uuid.UUID) (*Customer, error) {
	if err := authz.Require(t, authz.ActionCustomerRead); err != nil {
		return nil, err
	}
	found, err := u.store.Repos().Customer.FindByID(ctx, t, id)
	if err != nil {
		return nil, err
	}
	return found, nil
}

// ListInput is List's argument — raw query-param values; List clamps
// Page/PerPage before delegating to the repository.
type ListInput struct {
	Query   string
	Page    int
	PerPage int
}

func (u *Usecase) List(ctx context.Context, t tenant.Context, in ListInput) ([]*Customer, httpx.Meta, error) {
	if err := authz.Require(t, authz.ActionCustomerRead); err != nil {
		return nil, httpx.Meta{}, err
	}

	page := in.Page
	if page < 1 {
		page = 1
	}
	perPage := in.PerPage
	if perPage <= 0 {
		perPage = defaultPerPage
	} else if perPage > maxPerPage {
		perPage = maxPerPage
	}

	filter := ListFilter{Query: in.Query, Page: page, PerPage: perPage}
	customers, total, err := u.store.Repos().Customer.FindAllByOrg(ctx, t, filter)
	if err != nil {
		return nil, httpx.Meta{}, fmt.Errorf("customer: list: %w", err)
	}
	return customers, httpx.Meta{Page: page, PerPage: perPage, Total: total}, nil
}

// UpdateCustomerInput is Usecase.Update's argument.
type UpdateCustomerInput struct {
	Name    *string
	Email   *string
	Phone   *string
	Company *string
	Notes   *string
}

func (u *Usecase) Update(ctx context.Context, t tenant.Context, id uuid.UUID, in UpdateCustomerInput) (*Customer, error) {
	if err := authz.Require(t, authz.ActionCustomerUpdate); err != nil {
		return nil, err
	}

	repoIn := UpdateInput{
		Name: in.Name, Email: normalizeEmail(in.Email), Phone: in.Phone, Company: in.Company, Notes: in.Notes,
	}
	updated, err := u.store.Repos().Customer.Update(ctx, t, id, repoIn)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (u *Usecase) Delete(ctx context.Context, t tenant.Context, id uuid.UUID) error {
	if err := authz.Require(t, authz.ActionCustomerDelete); err != nil {
		return err
	}
	return u.store.Repos().Customer.Delete(ctx, t, id)
}

func normalizeEmail(email *string) *string {
	if email == nil {
		return nil
	}
	lower := strings.ToLower(strings.TrimSpace(*email))
	return &lower
}

// invalidStatusTransitionError reuses lead's exact code — converting a
// non-won lead is, at its core, the same "state doesn't allow this
// action" shape TD §5 already catalogued; TD §14 doesn't list a
// separate code for this case.
func invalidStatusTransitionError() error {
	return &httpx.DomainError{
		Status:  http.StatusUnprocessableEntity,
		Code:    "invalid_status_transition",
		Message: "Lead harus berstatus won untuk dikonversi.",
	}
}

func alreadyConvertedError() error {
	return &httpx.DomainError{
		Status:  http.StatusConflict,
		Code:    "lead_already_converted",
		Message: "Lead ini sudah pernah dikonversi menjadi customer.",
	}
}
