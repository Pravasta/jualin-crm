package activity

import (
	"context"
	"net/http"

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

// List returns leadID's timeline, newest first. Employee visibility
// (leads assigned to them only) is enforced in the repository, not
// here — same split lead itself uses (TD §9).
func (u *Usecase) List(ctx context.Context, t tenant.Context, leadID uuid.UUID) ([]*Activity, error) {
	if err := authz.Require(t, authz.ActionActivityList); err != nil {
		return nil, err
	}
	return u.store.Repos().Activity.FindAllByLead(ctx, t, leadID)
}

// CreateActivityInput is Usecase.Create's argument — Type is validated
// against the user-submittable allowlist here, before it ever reaches
// the repository.
type CreateActivityInput struct {
	Type     string
	Body     *string
	Metadata map[string]any
}

// Create appends a user-submitted activity (note_added, call_logged,
// whatsapp_opened only — TD §8). A client submitting a system type
// (status_changed, lead_created, …) is rejected 422, not 400: this is
// TD's own error code choice, distinct from a plain validation failure,
// so it's built as a one-off *httpx.DomainError rather than
// httpx.NewValidationError (which always maps to 400).
func (u *Usecase) Create(ctx context.Context, t tenant.Context, leadID uuid.UUID, in CreateActivityInput) (*Activity, error) {
	if err := authz.Require(t, authz.ActionActivityCreate); err != nil {
		return nil, err
	}
	if !IsUserType(in.Type) {
		return nil, invalidActivityTypeError()
	}

	return u.store.Repos().Activity.Create(ctx, t, CreateInput{
		LeadID:            leadID,
		Type:              in.Type,
		ActorMembershipID: t.MembershipID,
		Body:              in.Body,
		Metadata:          in.Metadata,
	})
}

func invalidActivityTypeError() error {
	return &httpx.DomainError{
		Status:  http.StatusUnprocessableEntity,
		Code:    "invalid_activity_type",
		Message: "Tipe activity ini hanya bisa dibuat oleh sistem.",
	}
}
