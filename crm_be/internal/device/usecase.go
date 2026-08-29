package device

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authz"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/push"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Usecase depends only on Store (port.go) and push.Sender — never on
// *pgxpool.Pool or pgx.Tx directly (ADR-011).
type Usecase struct {
	store  Store
	sender push.Sender
	logger *slog.Logger
}

func NewUsecase(store Store, sender push.Sender, logger *slog.Logger) *Usecase {
	return &Usecase{store: store, sender: sender, logger: logger}
}

type RegisterInput struct {
	Token    string
	Platform string
}

// Register upserts the calling device's token (see Repository.Upsert's
// doc comment for what "upsert" means here — a re-registration MOVES
// the row to the caller's current organization/membership rather than
// erroring).
func (u *Usecase) Register(ctx context.Context, t tenant.Context, in RegisterInput) (*Token, error) {
	if err := authz.Require(t, authz.ActionDeviceTokenRegister); err != nil {
		return nil, err
	}
	if in.Token == "" {
		return nil, httpx.NewValidationError(httpx.ErrorDetail{Field: "token", Code: "required"})
	}
	if !IsValidPlatform(in.Platform) {
		return nil, httpx.NewValidationError(httpx.ErrorDetail{Field: "platform", Code: "invalid_value"})
	}

	tok := &Token{
		ID:           uuid.Must(uuid.NewV7()),
		MembershipID: *t.MembershipID,
		Token:        in.Token,
		Platform:     in.Platform,
	}
	if err := u.store.Repos().DeviceToken.Upsert(ctx, t, tok); err != nil {
		return nil, fmt.Errorf("device: register: %w", err)
	}
	return tok, nil
}

// Unregister removes the caller's own device token — called on logout
// (Phase 5 TD §9.1). Ownership is checked here, in Go, rather than
// folded into DeleteByToken's SQL: FindByToken is scoped to the caller's
// organization only (Rule #6), and a token that DOES exist in this
// organization but belongs to a DIFFERENT membership is treated
// identically to one that doesn't exist at all — 404, never 403 — so a
// client can't learn anything about who else has a device registered.
func (u *Usecase) Unregister(ctx context.Context, t tenant.Context, token string) error {
	if err := authz.Require(t, authz.ActionDeviceTokenDelete); err != nil {
		return err
	}
	found, err := u.store.Repos().DeviceToken.FindByToken(ctx, t, token)
	if err != nil {
		return err // httpx.ErrNotFound for cross-org or missing
	}
	if t.MembershipID == nil || found.MembershipID != *t.MembershipID {
		return httpx.ErrNotFound
	}
	if err := u.store.Repos().DeviceToken.DeleteByToken(ctx, t, token); err != nil {
		return fmt.Errorf("device: unregister: %w", err)
	}
	return nil
}

// PushToMembership structurally satisfies lead.PushSender (ADR-011: the
// interface is declared by its consumer, lead, not here — lead never
// imports this package). Called AFTER lead's own transaction has
// already committed (Phase 5 TD §9.3, Rule #32) — every failure path
// below is logged and swallowed, never returned in a way that could
// look like something worth retrying at that call site.
//
// t carries the ASSIGNING actor's tenant.Context (Owner/Admin/Manager
// doing the reassignment); membershipID is the RECIPIENT, almost always
// a different person. Only t.OrganizationID is used for scoping — no
// method here touches t.MembershipID.
func (u *Usecase) PushToMembership(ctx context.Context, t tenant.Context, membershipID uuid.UUID, title, body string, data map[string]string) error {
	tokens, err := u.store.Repos().DeviceToken.FindByMembership(ctx, t, membershipID)
	if err != nil {
		u.logger.Error("failed to look up device tokens", "err", err, "membership_id", membershipID)
		return err
	}

	for _, tok := range tokens {
		sendErr := u.sender.Send(ctx, push.Message{Token: tok.Token, Title: title, Body: body, Data: data})
		if sendErr == nil {
			continue
		}

		// Rule #26: never log tok.Token itself — only which membership
		// and what happened, the same shape mailer.Send's own failure
		// log uses (logs "to", never the message body).
		u.logger.Error("failed to send push notification", "err", sendErr, "membership_id", membershipID)

		if errors.Is(sendErr, push.ErrTokenInvalid) {
			// TD §9.4, kriteria #12 — a token FCM will never accept
			// again must actually be removed, or this table only grows
			// (the same class of debt Phase 4.5 closed for
			// ratelimit.FixedWindow). Deletion failure is logged but
			// still not propagated — one token's cleanup failing must
			// never stop the loop from reaching the rest.
			if delErr := u.store.Repos().DeviceToken.DeleteByToken(ctx, t, tok.Token); delErr != nil {
				u.logger.Error("failed to delete unregistered device token", "err", delErr, "membership_id", membershipID)
			}
		}
	}
	return nil
}
