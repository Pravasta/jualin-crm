package lead

import (
	"context"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Repository is lead's own table interface — postgresRepository
// (repository_postgres.go) satisfies it, but Usecase depends only on
// this interface (ADR-011), which is what makes usecase_unit_test.go's
// fake possible without Docker. Through #19 this was a concrete
// exported struct (Repository) with no interface — the same migration
// internal/membership went through in #11, the moment a real usecase
// exists to declare the interface it needs.
type Repository interface {
	Create(ctx context.Context, t tenant.Context, in CreateInput) (*Lead, error)
	FindByID(ctx context.Context, t tenant.Context, id uuid.UUID) (*Lead, error)
	FindByIdempotencyKey(ctx context.Context, t tenant.Context, key string) (*Lead, error)
	FindAllByOrg(ctx context.Context, t tenant.Context, filter ListFilter) ([]*Lead, int, error)
	Update(ctx context.Context, t tenant.Context, id uuid.UUID, expectedVersion int, in UpdateInput) (*Lead, error)
	UpdateStatus(ctx context.Context, t tenant.Context, id uuid.UUID, expectedVersion int, status string, lostReason *string) (*Lead, error)
	Delete(ctx context.Context, t tenant.Context, id uuid.UUID) error
}

// Repos bundles what a single Usecase call needs — one field today.
// #21 adds an Activity field when lead events need cross-table
// atomicity with activity rows; #20 writes no activities at all (that's
// explicitly #21's job), so there's nothing to add yet (Rule #27/#28).
type Repos struct {
	Lead Repository
}

// Store is the Unit of Work Usecase depends on — same shape as every
// other domain's. Required even though most of Repository's methods are
// single statements: Create internally needs a transaction (see its doc
// comment in repository_postgres.go and the proof in #19's
// repository_test.go), so Usecase.Create must be able to call
// store.InTx.
type Store interface {
	InTx(ctx context.Context, fn func(Repos) error) error
	Repos() Repos
}
