package membership

import (
	"context"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Repository is membership's own table interface — the concrete
// implementation (postgresRepository, repository_postgres.go) satisfies
// it, but Usecase depends only on this interface (ADR-011), which is
// what makes usecase_unit_test.go's fake possible without Docker.
type Repository interface {
	Create(ctx context.Context, t tenant.Context, id, userID uuid.UUID, role tenant.Role) (*Membership, error)
	FindByID(ctx context.Context, t tenant.Context, id uuid.UUID) (*Membership, error)
	FindActiveByUserID(ctx context.Context, userID uuid.UUID) ([]*Membership, error)
	FindAllByOrgWithUser(ctx context.Context, t tenant.Context) ([]*MemberWithUser, error)
	UpdateRole(ctx context.Context, t tenant.Context, id uuid.UUID, role tenant.Role) error
	Deactivate(ctx context.Context, t tenant.Context, id uuid.UUID) error
	CountActiveOwners(ctx context.Context, t tenant.Context) (int, error)

	// CountActive is half the seat meter (Phase 8.5) — see the
	// implementation's doc comment for why the other half lives in
	// internal/invitation.
	CountActive(ctx context.Context, t tenant.Context) (int, error)
}

// AuditRepository is declared locally (not imported from internal/auditlog)
// per ADR-011 — the interface belongs to the consumer.
type AuditRepository interface {
	Record(ctx context.Context, t tenant.Context, actorMembershipID *uuid.UUID, action string) error
}

// RefreshTokenRepository is the one capability Usecase.Deactivate needs
// from refresh_tokens — satisfied by auth.NewRefreshTokenRevoker's
// return value at the composition root. This package never imports
// internal/auth; see that package's RefreshTokenRevoker doc comment.
type RefreshTokenRepository interface {
	RevokeAllByMembershipID(ctx context.Context, membershipID uuid.UUID) error
}

// OpenLeadRepository is declared locally per ADR-011 — membership needs
// only to count/release/move a membership's open leads (TD §13's own
// naming: "hitung, lepas, pindahkan"), not lead's full domain type.
// Satisfied by lead.NewOpenLeadRepository's return value at the
// composition root; this package never imports internal/lead, same as
// it never imports internal/auth for RefreshTokenRepository above.
type OpenLeadRepository interface {
	CountOpen(ctx context.Context, t tenant.Context, membershipID uuid.UUID) (int, error)
	UnassignOpen(ctx context.Context, t tenant.Context, membershipID uuid.UUID) ([]uuid.UUID, error)
	ReassignOpen(ctx context.Context, t tenant.Context, membershipID, reassignTo uuid.UUID) ([]uuid.UUID, error)
}

// ActivityRecorder is declared locally per ADR-011, same shape as
// lead's and task's own — satisfied by activity.NewRecorder's return
// value at the composition root. Deactivation with unassign/reassign
// logs one lead_assigned/lead_unassigned activity per affected lead
// (TD §13), atomically with the deactivation itself.
type ActivityRecorder interface {
	Record(ctx context.Context, t tenant.Context, leadID uuid.UUID, activityType string, actorMembershipID *uuid.UUID, metadata map[string]any) error
}

// Repos bundles every repository a single Usecase call needs.
type Repos struct {
	Member       Repository
	Audit        AuditRepository
	RefreshToken RefreshTokenRepository
	OpenLead     OpenLeadRepository
	Activity     ActivityRecorder
}

// Store is the Unit of Work Usecase depends on — same shape as auth.Store.
type Store interface {
	InTx(ctx context.Context, fn func(Repos) error) error
	Repos() Repos
}
