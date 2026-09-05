package invitation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authz"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/mailer"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/password"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/token"
	"github.com/Pravasta/jualin-crm/crm_be/internal/user"
)

const minPasswordLength = 12

type Usecase struct {
	store     Store
	mailer    mailer.Mailer
	logger    *slog.Logger
	baseURL   string
	seatQuota PlanSeatQuota
}

func NewUsecase(store Store, m mailer.Mailer, logger *slog.Logger, baseURL string, seatQuota PlanSeatQuota) *Usecase {
	return &Usecase{store: store, mailer: m, logger: logger, baseURL: baseURL, seatQuota: seatQuota}
}

// Create invites email into t's organization. role=owner is rejected
// here (fail fast with a clean error) even though the database CHECK
// constraint would also catch it — owner is only ever created at
// registration (freeze bagian 7).
func (u *Usecase) Create(ctx context.Context, t tenant.Context, in CreateInput) (*Invitation, error) {
	if err := authz.Require(t, authz.ActionInvitationCreate); err != nil {
		return nil, err // 1. role → 403 forbidden
	}

	// 2. seat limit (Phase 8.5 #124) — checked AFTER role, same binding
	// order as every other plan gate in this codebase (subscription TD
	// §3.3): a principal with no standing to invite anyone at all must
	// never learn the organization's billing state through this branch
	// instead. used sums TWO counters — active memberships AND pending
	// invitations — because an invitation nobody has accepted yet still
	// holds a seat; without counting it, a 2-seat organization could
	// send five invitations and end up with five members.
	repos := u.store.Repos()
	activeSeats, err := repos.Member.CountActive(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("invitation: create: count active seats: %w", err)
	}
	pendingSeats, err := repos.Invitation.CountPendingSeats(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("invitation: create: count pending seats: %w", err)
	}
	if err := u.seatQuota.AllowSeat(ctx, t, activeSeats+pendingSeats); err != nil {
		return nil, err
	}

	if in.Role == tenant.RoleOwner {
		return nil, httpx.NewValidationError(httpx.ErrorDetail{Field: "role", Code: "cannot_invite_owner"})
	}

	rawToken, tokenHash, err := token.Generate()
	if err != nil {
		return nil, fmt.Errorf("invitation: create: generate token: %w", err)
	}

	inv := &Invitation{
		ID:                    uuid.Must(uuid.NewV7()),
		OrganizationID:        t.OrganizationID,
		Email:                 in.Email,
		Role:                  in.Role,
		TokenHash:             tokenHash,
		InvitedByMembershipID: *t.MembershipID,
		ExpiresAt:             time.Now().Add(invitationTTL),
	}

	txErr := u.store.InTx(ctx, func(r Repos) error {
		if err := r.Invitation.Create(ctx, inv); err != nil {
			return err
		}
		return r.Audit.Record(ctx, t, t.MembershipID, "invitation.created")
	})
	if txErr != nil {
		return nil, fmt.Errorf("invitation: create: %w", txErr)
	}

	u.sendInvitationEmail(ctx, inv.Email, rawToken)

	return inv, nil
}

func (u *Usecase) List(ctx context.Context, t tenant.Context) ([]*Invitation, error) {
	if err := authz.Require(t, authz.ActionInvitationList); err != nil {
		return nil, err
	}
	invites, err := u.store.Repos().Invitation.FindByOrgPending(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("invitation: list: %w", err)
	}
	return invites, nil
}

func (u *Usecase) Revoke(ctx context.Context, t tenant.Context, id uuid.UUID) error {
	if err := authz.Require(t, authz.ActionInvitationRevoke); err != nil {
		return err
	}
	if _, err := u.store.Repos().Invitation.FindByID(ctx, t, id); err != nil {
		return err // httpx.ErrNotFound for cross-org or missing
	}

	return u.store.InTx(ctx, func(r Repos) error {
		if err := r.Invitation.MarkRevoked(ctx, id); err != nil {
			return err
		}
		return r.Audit.Record(ctx, t, t.MembershipID, "invitation.revoked")
	})
}

// TokenInfo is the public preview GET /v1/invitations/token/{token}
// serves before the invitee decides which branch of Accept they're in.
func (u *Usecase) TokenInfo(ctx context.Context, rawToken string) (*TokenInfo, error) {
	inv, err := u.findActionable(ctx, rawToken)
	if err != nil {
		return nil, err
	}

	org, err := u.store.Repos().Org.FindByID(ctx, inv.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("invitation: token info: load organization: %w", err)
	}

	_, err = u.store.Repos().User.FindByEmail(ctx, inv.Email)
	userExists := err == nil

	return &TokenInfo{OrganizationName: org.Name, Email: inv.Email, UserExists: userExists}, nil
}

// AcceptOutput is Accept's result — enough for the handler to respond,
// nothing more.
type AcceptOutput struct {
	UserID         uuid.UUID
	MembershipID   uuid.UUID
	OrganizationID uuid.UUID
}

// Accept resolves which of TD §6.1's two branches applies and dispatches
// to the matching internal method. fullName/password are only ever
// forwarded into acceptNewUser — acceptExistingUser's parameter list
// (AcceptExistingUserInput) has no password field to receive them, so
// there is no code path in the existing-user branch that can read or set
// a password. This is the freeze bagian 7 (B4) guarantee: "cabang kedua
// tidak boleh mengizinkan penyetelan password tanpa autentikasi."
func (u *Usecase) Accept(ctx context.Context, authenticated *tenant.Context, rawToken, fullName, rawPassword string) (*AcceptOutput, error) {
	inv, err := u.findActionable(ctx, rawToken)
	if err != nil {
		return nil, err
	}

	existingUser, err := u.store.Repos().User.FindByEmail(ctx, inv.Email)
	if err != nil && !errors.Is(err, httpx.ErrNotFound) {
		return nil, fmt.Errorf("invitation: accept: find user: %w", err)
	}

	if existingUser == nil {
		return u.acceptNewUser(ctx, inv, AcceptNewUserInput{Token: rawToken, FullName: fullName, Password: rawPassword})
	}
	return u.acceptExistingUser(ctx, inv, existingUser, authenticated, AcceptExistingUserInput{Token: rawToken})
}

func (u *Usecase) acceptNewUser(ctx context.Context, inv *Invitation, in AcceptNewUserInput) (*AcceptOutput, error) {
	var details []httpx.ErrorDetail
	if in.FullName == "" {
		details = append(details, httpx.ErrorDetail{Field: "full_name", Code: "required"})
	}
	if len(in.Password) < minPasswordLength {
		details = append(details, httpx.ErrorDetail{Field: "password", Code: "too_short"})
	}
	if len(details) > 0 {
		return nil, httpx.NewValidationError(details...)
	}

	hash, err := password.Hash(in.Password)
	if err != nil {
		return nil, fmt.Errorf("invitation: accept new user: hash password: %w", err)
	}

	userID := uuid.Must(uuid.NewV7())
	membershipID := uuid.Must(uuid.NewV7())
	t := tenant.Context{OrganizationID: inv.OrganizationID, PrincipalType: tenant.PrincipalUser}

	txErr := u.store.InTx(ctx, func(r Repos) error {
		created, err := r.User.Create(ctx, userID, inv.Email, hash, in.FullName)
		if err != nil {
			return err
		}
		// B3: accepting an invitation also verifies the email — the token
		// was mailed to that exact address, so ownership is already
		// proven; a newly-invited employee shouldn't need a second,
		// separate verification step.
		if err := r.User.MarkEmailVerified(ctx, created.ID); err != nil {
			return err
		}
		if _, err := r.Member.Create(ctx, t, membershipID, created.ID, inv.Role); err != nil {
			return err
		}
		if err := r.Invitation.MarkAccepted(ctx, inv.ID); err != nil {
			return err
		}
		return r.Audit.Record(ctx, t, &membershipID, "invitation.accepted")
	})
	if txErr != nil {
		if errors.Is(txErr, user.ErrEmailTaken) {
			// Lost the race with a concurrent registration/accept for the
			// same email — treat identically to "you must already have an
			// account", which is exactly what just became true.
			return nil, invalidTokenError()
		}
		return nil, fmt.Errorf("invitation: accept new user: %w", txErr)
	}

	return &AcceptOutput{UserID: userID, MembershipID: membershipID, OrganizationID: inv.OrganizationID}, nil
}

// acceptExistingUser's parameter list has no access to a password value
// at all — see Accept's doc comment. authenticated must be non-nil and
// must be the exact user this invitation targets; a different logged-in
// user is rejected the same way as no session at all (both
// authentication_required) so the response can't be used to enumerate
// which account owns an email (freeze bagian 7, B4 security test).
func (u *Usecase) acceptExistingUser(
	ctx context.Context,
	inv *Invitation,
	existingUser *user.User,
	authenticated *tenant.Context,
	in AcceptExistingUserInput,
) (*AcceptOutput, error) {
	if authenticated == nil || authenticated.UserID == nil || *authenticated.UserID != existingUser.ID {
		return nil, authenticationRequiredError()
	}

	membershipID := uuid.Must(uuid.NewV7())
	t := tenant.Context{OrganizationID: inv.OrganizationID, PrincipalType: tenant.PrincipalUser}

	txErr := u.store.InTx(ctx, func(r Repos) error {
		if _, err := r.Member.Create(ctx, t, membershipID, existingUser.ID, inv.Role); err != nil {
			return err
		}
		if err := r.Invitation.MarkAccepted(ctx, inv.ID); err != nil {
			return err
		}
		return r.Audit.Record(ctx, t, &membershipID, "invitation.accepted")
	})
	if txErr != nil {
		return nil, fmt.Errorf("invitation: accept existing user: %w", txErr)
	}
	_ = in.Token // token already consumed via inv; kept on the type for symmetry with AcceptNewUserInput

	return &AcceptOutput{UserID: existingUser.ID, MembershipID: membershipID, OrganizationID: inv.OrganizationID}, nil
}

// findActionable resolves a raw token to its invitation row and
// classifies why it might not be usable — distinguishing
// invitation_already_accepted (TD §13's dedicated code) from every other
// failure (invalid_token), which FindValidByHash's un-filtered query
// makes possible.
func (u *Usecase) findActionable(ctx context.Context, rawToken string) (*Invitation, error) {
	hash := token.Hash(rawToken)
	inv, err := u.store.Repos().Invitation.FindValidByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, httpx.ErrNotFound) {
			return nil, invalidTokenError()
		}
		return nil, fmt.Errorf("invitation: find actionable: %w", err)
	}
	if inv.AcceptedAt != nil {
		return nil, &httpx.DomainError{
			Status:  http.StatusConflict,
			Code:    "invitation_already_accepted",
			Message: "Undangan sudah diterima sebelumnya.",
		}
	}
	if inv.RevokedAt != nil || !inv.ExpiresAt.After(time.Now()) {
		return nil, invalidTokenError()
	}
	return inv, nil
}

func (u *Usecase) sendInvitationEmail(ctx context.Context, email, rawToken string) {
	link := fmt.Sprintf("%s/invitations/accept?token=%s", u.baseURL, rawToken)
	err := u.mailer.Send(ctx, mailer.Message{
		To:      email,
		Subject: "Anda diundang bergabung di Jualin CRM",
		Body:    fmt.Sprintf("Klik tautan berikut untuk bergabung: %s\n\nTautan berlaku 7 hari.", link),
	})
	if err != nil {
		// Rule #32: send failure never rolls back work that already
		// committed.
		u.logger.Error("failed to send invitation email", "err", err, "to", email)
	}
}

func invalidTokenError() error {
	return &httpx.DomainError{
		Status:  http.StatusBadRequest,
		Code:    "invalid_token",
		Message: "Token tidak valid atau sudah kedaluwarsa.",
	}
}

func authenticationRequiredError() error {
	return &httpx.DomainError{
		Status:  http.StatusUnauthorized,
		Code:    "authentication_required",
		Message: "Autentikasi diperlukan untuk menerima undangan ini.",
	}
}
