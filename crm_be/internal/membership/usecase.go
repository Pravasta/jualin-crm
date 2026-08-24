// Package membership manages an organization's existing members — list,
// role change, deactivation. Creating the very first membership (the
// owner, at registration) and resolving a user's memberships at login
// stay in internal/auth, which already owns that flow end to end; this
// package only handles what happens to a membership once it exists.
package membership

import (
	"context"
	"errors"
	"fmt"
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

// List returns every active member of t's organization. Manager-level
// access is read-only — authz.Require is the only gate this method
// needs, since "list" has no per-row relationship rule.
func (u *Usecase) List(ctx context.Context, t tenant.Context) ([]*MemberWithUser, error) {
	if err := authz.Require(t, authz.ActionMembershipList); err != nil {
		return nil, err
	}
	members, err := u.store.Repos().Member.FindAllByOrgWithUser(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("membership: list: %w", err)
	}
	return members, nil
}

// UpdateRole changes a member's role. Two relationship rules apply on
// top of the coarse authz gate (docs/architecture/authorization.md):
//
//   - Rule 3: nobody can change their own role — no exception for Owner.
//   - Rule 4: Admin can't touch an existing Owner's role, and can't
//     promote anyone to Owner. Owner promoting someone else to Owner
//     (a co-owner) is allowed — nothing in the matrix forbids it, and
//     the schema has no uniqueness constraint on the owner count.
func (u *Usecase) UpdateRole(ctx context.Context, t tenant.Context, in UpdateRoleInput) error {
	if err := authz.Require(t, authz.ActionMembershipUpdateRole); err != nil {
		return err
	}

	if t.MembershipID != nil && in.MembershipID == *t.MembershipID {
		return selfRoleChangeError()
	}

	target, err := u.store.Repos().Member.FindByID(ctx, t, in.MembershipID)
	if err != nil {
		if errors.Is(err, httpx.ErrNotFound) {
			return httpx.ErrNotFound
		}
		return fmt.Errorf("membership: update role: find target: %w", err)
	}

	if t.Role == tenant.RoleAdmin && (target.Role == tenant.RoleOwner || in.NewRole == tenant.RoleOwner) {
		return adminCannotTouchOwnerError()
	}

	if err := u.store.Repos().Member.UpdateRole(ctx, t, in.MembershipID, in.NewRole); err != nil {
		return fmt.Errorf("membership: update role: %w", err)
	}
	if err := u.store.Repos().Audit.Record(ctx, t, t.MembershipID, "membership.role_changed"); err != nil {
		return fmt.Errorf("membership: update role: audit: %w", err)
	}
	return nil
}

// DeactivateInput carries the on_open_leads decision (TD §13) — closing
// the Phase 1 obligation that a membership can't be deactivated while
// it still owns open leads without an explicit choice about what
// happens to them.
type DeactivateInput struct {
	// OnOpenLeads is "reject" (default, when empty), "unassign", or
	// "reassign". Anything else is a validation error.
	OnOpenLeads string
	// ReassignTo is required when OnOpenLeads is "reassign", ignored
	// otherwise.
	ReassignTo *uuid.UUID
}

const (
	onOpenLeadsReject   = "reject"
	onOpenLeadsUnassign = "unassign"
	onOpenLeadsReassign = "reassign"
)

// Deactivate soft-deletes a membership and revokes every refresh token
// issued for it in the same transaction (freeze §2.3 rule #2 — without
// this, someone who just lost access keeps working sessions until their
// access token happens to expire). Since #22, the same transaction also
// settles what happens to leads still assigned to targetID (TD §13):
// by default the whole deactivation is refused while any are open,
// closing the obligation freeze 2.3 and 01-auth-organization/td.md §17
// left open since Phase 1.
func (u *Usecase) Deactivate(ctx context.Context, t tenant.Context, targetID uuid.UUID, in DeactivateInput) error {
	if err := authz.Require(t, authz.ActionMembershipDeactivate); err != nil {
		return err
	}

	onOpenLeads := in.OnOpenLeads
	if onOpenLeads == "" {
		onOpenLeads = onOpenLeadsReject
	}
	if onOpenLeads != onOpenLeadsReject && onOpenLeads != onOpenLeadsUnassign && onOpenLeads != onOpenLeadsReassign {
		return httpx.NewValidationError(httpx.ErrorDetail{Field: "on_open_leads", Code: "invalid_value"})
	}
	if onOpenLeads == onOpenLeadsReassign && in.ReassignTo == nil {
		return httpx.NewValidationError(httpx.ErrorDetail{Field: "reassign_to", Code: "required"})
	}

	target, err := u.store.Repos().Member.FindByID(ctx, t, targetID)
	if err != nil {
		if errors.Is(err, httpx.ErrNotFound) {
			return httpx.ErrNotFound
		}
		return fmt.Errorf("membership: deactivate: find target: %w", err)
	}

	if t.Role == tenant.RoleAdmin && target.Role == tenant.RoleOwner {
		return adminCannotTouchOwnerError()
	}

	isSelf := t.MembershipID != nil && targetID == *t.MembershipID
	if isSelf && target.Role == tenant.RoleOwner {
		count, err := u.store.Repos().Member.CountActiveOwners(ctx, t)
		if err != nil {
			return fmt.Errorf("membership: deactivate: count owners: %w", err)
		}
		if count <= 1 {
			return lastOwnerCannotBeRemovedError()
		}
	}

	return u.store.InTx(ctx, func(r Repos) error {
		switch onOpenLeads {
		case onOpenLeadsReject:
			count, err := r.OpenLead.CountOpen(ctx, t, targetID)
			if err != nil {
				return err
			}
			if count > 0 {
				return &OpenLeadsError{Count: count}
			}
		case onOpenLeadsUnassign:
			leadIDs, err := r.OpenLead.UnassignOpen(ctx, t, targetID)
			if err != nil {
				return err
			}
			for _, leadID := range leadIDs {
				if err := r.Activity.Record(ctx, t, leadID, "lead_unassigned", t.MembershipID, map[string]any{"from": targetID}); err != nil {
					return err
				}
			}
		case onOpenLeadsReassign:
			leadIDs, err := r.OpenLead.ReassignOpen(ctx, t, targetID, *in.ReassignTo)
			if err != nil {
				return err
			}
			for _, leadID := range leadIDs {
				if err := r.Activity.Record(ctx, t, leadID, "lead_assigned", t.MembershipID, map[string]any{"from": targetID, "to": *in.ReassignTo}); err != nil {
					return err
				}
			}
		}

		if err := r.Member.Deactivate(ctx, t, targetID); err != nil {
			return err
		}
		if err := r.RefreshToken.RevokeAllByMembershipID(ctx, targetID); err != nil {
			return err
		}
		return r.Audit.Record(ctx, t, t.MembershipID, "membership.deactivated")
	})
}

func selfRoleChangeError() error {
	return &httpx.DomainError{
		Status:  http.StatusForbidden,
		Code:    "forbidden",
		Message: "Anda tidak bisa mengubah role diri sendiri.",
	}
}

func adminCannotTouchOwnerError() error {
	return &httpx.DomainError{
		Status:  http.StatusForbidden,
		Code:    "forbidden",
		Message: "Admin tidak bisa mengubah Owner atau mengangkat Owner baru.",
	}
}

func lastOwnerCannotBeRemovedError() error {
	return &httpx.DomainError{
		Status:  http.StatusConflict,
		Code:    "last_owner_cannot_be_removed",
		Message: "Owner terakhir tidak bisa menonaktifkan diri sendiri.",
	}
}
