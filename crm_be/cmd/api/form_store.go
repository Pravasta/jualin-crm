package main

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pravasta/jualin-crm/crm_be/internal/auditlog"
	"github.com/Pravasta/jualin-crm/crm_be/internal/form"
	"github.com/Pravasta/jualin-crm/crm_be/internal/lead"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// formStore is the composition root's implementation of form.Store —
// same wiring pattern as apikeyStore.
type formStore struct {
	pool *pgxpool.Pool
}

func newFormStore(pool *pgxpool.Pool) form.Store {
	return &formStore{pool: pool}
}

func (s *formStore) InTx(ctx context.Context, fn func(form.Repos) error) error {
	return db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(formReposFor(tx))
	})
}

func (s *formStore) Repos() form.Repos {
	return formReposFor(s.pool)
}

func formReposFor(q db.Querier) form.Repos {
	return form.Repos{
		Form:  form.New(q),
		Audit: auditlog.New(q),
	}
}

// leadCreatorAdapter satisfies form.LeadCreator by translating its
// primitives-only argument list into lead.CreateLeadInput and calling
// the exact same lead.Usecase.Create every other creation path (Owner
// dashboard, API key) already goes through — the composition root
// (this file, plus main.go) is the ONE place allowed to import both
// internal/form and internal/lead (see LeadCreator's doc comment in
// internal/form/port.go for why no domain package does this itself).
type leadCreatorAdapter struct {
	usecase *lead.Usecase
}

func newLeadCreatorAdapter(usecase *lead.Usecase) form.LeadCreator {
	return &leadCreatorAdapter{usecase: usecase}
}

// CreateFromForm discards Create's isNew bool — TD §5 is explicit forms
// never carry an Idempotency-Key, so every call here is a genuinely new
// lead by construction; there is no "was this a replay" question to
// answer the way the API-key path has one.
func (a *leadCreatorAdapter) CreateFromForm(ctx context.Context, t tenant.Context, name string, email, phone, company, notes *string, rawPayload []byte) (uuid.UUID, error) {
	created, _, err := a.usecase.Create(ctx, t, lead.CreateLeadInput{
		Name: name, Email: email, Phone: phone, Company: company, Notes: notes, RawPayload: rawPayload,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}
