package form

import (
	"context"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Repository is form's own table interface.
type Repository interface {
	Create(ctx context.Context, t tenant.Context, f *Form) error
	FindByOrg(ctx context.Context, t tenant.Context) ([]*Form, error)
	FindByID(ctx context.Context, t tenant.Context, id uuid.UUID) (*Form, error)
	Update(ctx context.Context, t tenant.Context, id uuid.UUID, in UpdateInput) (*Form, error)
	Delete(ctx context.Context, t tenant.Context, id uuid.UUID) error

	// IncrementSubmitCount bumps forms.submit_count by one — TD §1's
	// "angka untuk ditampilkan di dashboard ('form ini pernah dipakai
	// atau tidak')", NOT a Phase 8 usage-quota mechanism (usage_counters
	// is a separate, unrelated table). #87 is the only place a
	// submission event can ever originate, so it's the only caller.
	// Best-effort from Usecase.Submit's side (see its own call site) —
	// a failure here never fails the HTTP response, since the lead this
	// counts has already been created by the time this runs.
	IncrementSubmitCount(ctx context.Context, t tenant.Context, id uuid.UUID) error

	// FindByPublicKey deliberately does NOT take tenant.Context — same
	// documented exception as apikey.Repository.FindByKeyID (Phase 4
	// #46), itself following invitation.Repository.FindValidByHash and
	// RefreshTokenRepository.FindByHashForUpdate (Phase 1): which
	// organization a form belongs to is exactly what resolving
	// public_key tells the caller, not something known beforehand (Rule
	// #5). No HTTP path calls this in #85 — it's exercised directly by
	// repository_test.go (including an EXPLAIN proving it's an index
	// hit) and exists now because #87's ResolvePublicKey needs the
	// exact same interface, not a reshaped one added mid-phase.
	FindByPublicKey(ctx context.Context, publicKey string) (*Form, error)
}

// AuditRepository is declared here (the consumer) per ADR-011 —
// *auditlog.Repository already satisfies it structurally.
type AuditRepository interface {
	Record(ctx context.Context, t tenant.Context, actorMembershipID *uuid.UUID, action string) error
}

// PlanGate is declared here, consumer-side (ADR-011) — same shape as
// apikey.PlanGate and webhook.PlanGate, each declared independently
// rather than shared (TD §3.2). Satisfied structurally by
// *subscription.Usecase through cmd/api/subscription_gate.go's planGate
// wrapper. "form" is a wire contract with subscription.ChannelForm —
// locked by cmd/api/plan_gate_test.go (TD §7).
type PlanGate interface {
	RequireChannel(ctx context.Context, t tenant.Context, ch string) error
}

type Repos struct {
	Form  Repository
	Audit AuditRepository
}

// Store is the Unit of Work Usecase depends on — same shape as every
// other domain's Store since ADR-011.
type Store interface {
	InTx(ctx context.Context, fn func(Repos) error) error
	Repos() Repos
}

// LeadCreator lets Usecase.Submit (#87) create a lead without
// internal/form importing internal/lead directly. ADR-011's rule that
// cross-domain interfaces are declared by the consumer using only
// primitives — never the producer's own domain types — extends here in
// the opposite direction from lead.ActivityRecorder/NotificationSender/
// PushSender: those let lead call INTO other domains without importing
// them; this lets form call OUT to lead without importing it. Verified
// against the actual codebase before choosing this shape: no domain
// package other than cmd/api imports internal/lead anywhere in this
// repository (only cmd/api's own *_store.go/main.go do) — importing
// lead.CreateLeadInput/*lead.Lead here directly would be the one
// exception with no precedent.
//
// *lead.Usecase does NOT satisfy this directly (its own Create keeps
// lead.CreateLeadInput/*lead.Lead, per D5: "memakai ulang
// lead.Usecase.Create apa adanya" — TD never asks lead's own signature
// to change shape). A thin adapter wired at the composition root
// (cmd/api/form_store.go) translates between the two; internally it
// still calls the exact same lead.Usecase.Create every other creation
// path uses, including the SAME authz.Require(ActionLeadCreate) call
// that routes a PrincipalPublicForm tenant.Context through authz's
// publicFormAllows map (TD §4) — Usecase.Submit below never calls
// authz.Require itself, it relies entirely on this.
type LeadCreator interface {
	CreateFromForm(ctx context.Context, t tenant.Context, name string, email, phone, company, notes *string, rawPayload []byte) (leadID uuid.UUID, err error)
}
