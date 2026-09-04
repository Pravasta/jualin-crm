package subscription

import (
	"context"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Repository is subscription's own read interface — *Repository
// (repository_postgres.go) satisfies it, but Usecase depends only on
// this interface (ADR-011).
//
// No Store here: this package only reads (TD §6). CreateFree — the one
// write this domain has — is called as a plain repository method from
// inside internal/auth's registration transaction (auth.Repos.Sub),
// same precedent as internal/metrics (#30).
type Repository interface {
	FindActiveByOrg(ctx context.Context, t tenant.Context) (*Subscription, error)
}
