package apikey

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authz"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// lastUsedThrottleWindow is TD phase 4 §10's "paling sering sekali per 5
// menit per kunci" — ADR-004 aturan #3 warns that writing last_used_at
// on every request turns this table into a write hotspot.
const lastUsedThrottleWindow = 5 * time.Minute

// Usecase depends only on Store (port.go), never on *pgxpool.Pool or
// pgx.Tx directly (ADR-011).
type Usecase struct {
	store Store

	// lastUsedThrottle is an in-memory map[api_key_id]last-write-time,
	// same shape as ratelimit.FixedWindow's own bucket map — no
	// eviction, same accepted debt as that map (tracked since #9). A
	// process restart empties it, which costs at most one extra write
	// per key; TD §10 explicitly says no correction is needed for that.
	lastUsedMu       sync.Mutex
	lastUsedThrottle map[uuid.UUID]time.Time
}

func NewUsecase(store Store) *Usecase {
	return &Usecase{store: store, lastUsedThrottle: map[uuid.UUID]time.Time{}}
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

// ResolveAPIKey verifies a raw jln_* credential and, on success, returns
// the tenant.Context an authenticated request through it should carry.
// Structurally satisfies authn.APIKeyResolver (ADR-011: the interface is
// declared by its consumer, authn, not here) — authn never imports this
// package.
//
// Every failure reason (malformed credential, unknown key_id, revoked,
// expired, wrong secret) returns the SAME invalidAPIKeyError() —
// deliberately: distinguishing them in the response would tell a
// guesser which key_id ever existed, the same information leak Rule #6
// already forbids for 404-vs-403.
func (u *Usecase) ResolveAPIKey(ctx context.Context, raw string) (tenant.Context, error) {
	_, keyID, rawSecret, ok := parseCredential(raw)
	if !ok {
		return tenant.Context{}, invalidAPIKeyError()
	}

	found, err := u.store.Repos().APIKey.FindByKeyID(ctx, keyID)
	if err != nil {
		return tenant.Context{}, invalidAPIKeyError()
	}
	if found.RevokedAt != nil {
		return tenant.Context{}, invalidAPIKeyError()
	}
	if found.ExpiresAt != nil && !found.ExpiresAt.After(time.Now()) {
		return tenant.Context{}, invalidAPIKeyError()
	}
	if !verifySecret(rawSecret, found.SecretHash) {
		return tenant.Context{}, invalidAPIKeyError()
	}

	u.touchLastUsed(found.ID)

	return tenant.Context{
		OrganizationID: found.OrganizationID,
		PrincipalType:  tenant.PrincipalAPIKey,
		APIKeyID:       &found.ID,
		Scopes:         found.Scopes,
		// MembershipID, UserID, Role deliberately left zero — an API key
		// carries no identity of a person (Aturan #24), and authz.Require
		// never consults Role for PrincipalAPIKey anyway.
	}, nil
}

// touchLastUsed applies TD §10's throttle before writing at all — most
// calls exit here without touching the database. The write itself is
// synchronous but cheap (one indexed UPDATE); TD §10's "tidak
// memblokir" is about not turning this table into a write hotspot, not
// a hard requirement to fire a goroutine. Its error is discarded: a
// missed last_used_at write has no correctness impact (TD §10: "bukan
// timestamp presisi") and must never fail the authentication that
// triggered it.
func (u *Usecase) touchLastUsed(apiKeyID uuid.UUID) {
	u.lastUsedMu.Lock()
	last, seen := u.lastUsedThrottle[apiKeyID]
	due := !seen || time.Since(last) >= lastUsedThrottleWindow
	if due {
		u.lastUsedThrottle[apiKeyID] = time.Now()
	}
	u.lastUsedMu.Unlock()

	if !due {
		return
	}
	_ = u.store.Repos().APIKey.TouchLastUsed(context.Background(), apiKeyID)
}

func invalidAPIKeyError() error {
	return &httpx.DomainError{
		Status:  http.StatusUnauthorized,
		Code:    "invalid_api_key",
		Message: "Kredensial API tidak valid, sudah kedaluwarsa, atau sudah dicabut.",
	}
}
