package lead

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authz"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/phone"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

const (
	defaultPerPage = 25
	maxPerPage     = 100

	// idempotencyCleanupThrottleWindow is TD phase 4 §7's "paling sering
	// sekali per organization per jam" — same in-memory throttle shape
	// as apikey.Usecase's last_used_at (TD §10), just a longer window
	// since this sweep is cheaper to skip.
	idempotencyCleanupThrottleWindow = time.Hour
)

var validSources = map[string]bool{"manual": true, "api": true, "form": true, "webhook": true}

// Usecase depends only on Store (port.go), never on *pgxpool.Pool or
// pgx.Tx directly (ADR-011). No mailer, no logger — leads don't send
// email.
type Usecase struct {
	store Store

	// idempotencyCleanupThrottle is an in-memory map[organization_id]
	// last-sweep-time. Deliberately WITHOUT eviction, unlike
	// ratelimit.FixedWindow and auth.LoginLimiter (Phase 4.5 #58) —
	// those are keyed by IP/email, input from someone who hasn't
	// authenticated yet, so an attacker can grow them without limit.
	// This map is keyed by organization_id, bounded by how many tenants
	// actually exist. Adding eviction here would be solving a problem
	// that doesn't exist (Rule #27–#29).
	idempotencyCleanupMu       sync.Mutex
	idempotencyCleanupThrottle map[uuid.UUID]time.Time
}

func NewUsecase(store Store) *Usecase {
	return &Usecase{store: store, idempotencyCleanupThrottle: map[uuid.UUID]time.Time{}}
}

// Create validates and normalizes in, then persists it. isNew is false
// when in.IdempotencyKey collided with an existing lead — the caller
// gets that lead back instead of an error, and the handler responds 200
// instead of 201 (TD §7: idempotent replay returns the ORIGINAL
// response, never a second lead and never an error).
func (u *Usecase) Create(ctx context.Context, t tenant.Context, in CreateLeadInput) (result *Lead, isNew bool, err error) {
	if err := authz.Require(t, authz.ActionLeadCreate); err != nil {
		return nil, false, err
	}

	// Public API jalur (Phase 4 #47, TD §5): source is never trusted
	// from an external caller, and assignment can never be REQUESTED by
	// one — an external system has no way to know a membership id, so
	// the field's mere presence can only mean a misunderstanding, not a
	// legitimate request. Rejected here, not silently dropped, so the
	// integrator's own client sees its assignment did NOT take effect.
	if t.PrincipalType == tenant.PrincipalAPIKey {
		if in.AssignedToMembershipID != nil {
			return nil, false, insufficientScopeError()
		}
		in.Source = "api"
		u.maybeCleanupExpiredIdempotencyKeys(ctx, t)
	}

	if in.Name == "" {
		return nil, false, httpx.NewValidationError(httpx.ErrorDetail{Field: "name", Code: "required"})
	}
	source := in.Source
	if source == "" {
		source = "manual"
	} else if !validSources[source] {
		return nil, false, httpx.NewValidationError(httpx.ErrorDetail{Field: "source", Code: "invalid_value"})
	}

	email := normalizeEmail(in.Email)
	phoneE164 := normalizePhone(in.Phone)

	repoIn := CreateInput{
		Name:                   in.Name,
		Email:                  email,
		Phone:                  in.Phone,
		PhoneE164:              phoneE164,
		Company:                in.Company,
		Notes:                  in.Notes,
		Source:                 source,
		AssignedToMembershipID: in.AssignedToMembershipID,
		CreatedByMembershipID:  t.MembershipID,
		IdempotencyKey:         in.IdempotencyKey,
		RawPayload:             in.RawPayload,
		// SourceAPIKeyID comes from the authenticated principal, never
		// from in — nil automatically for every user-principal create,
		// since t.APIKeyID is only ever set on a PrincipalAPIKey
		// tenant.Context (apikey.Usecase.ResolveAPIKey).
		SourceAPIKeyID: t.APIKeyID,
	}

	var created *Lead
	txErr := u.store.InTx(ctx, func(r Repos) error {
		c, err := r.Lead.Create(ctx, t, repoIn)
		if err != nil {
			return err
		}
		created = c
		// lead_created is written in the SAME transaction as the row
		// itself (TD §10) — a lead that exists without this activity,
		// or an activity for a lead that got rolled back, are both
		// worse than neither existing.
		return r.Activity.Record(ctx, t, c.ID, "lead_created", repoIn.CreatedByMembershipID, nil)
	})
	if txErr != nil {
		if errors.Is(txErr, ErrIdempotencyKeyExists) {
			existing, err := u.store.Repos().Lead.FindByIdempotencyKey(ctx, t, *in.IdempotencyKey)
			if err != nil {
				return nil, false, fmt.Errorf("lead: create: resolve idempotent replay: %w", err)
			}
			return existing, false, nil
		}
		if errors.Is(txErr, ErrAssigneeNotFound) {
			return nil, false, httpx.NewValidationError(httpx.ErrorDetail{Field: "assigned_to_membership_id", Code: "not_found"})
		}
		return nil, false, fmt.Errorf("lead: create: %w", txErr)
	}
	return created, true, nil
}

func (u *Usecase) Get(ctx context.Context, t tenant.Context, id uuid.UUID) (*Lead, error) {
	if err := authz.Require(t, authz.ActionLeadRead); err != nil {
		return nil, err
	}
	found, err := u.store.Repos().Lead.FindByID(ctx, t, id)
	if err != nil {
		return nil, err
	}
	return found, nil
}

// ListInput is List's argument — raw query-param values; List clamps
// Page/PerPage before delegating to the repository.
type ListInput struct {
	Status         []string
	Source         []string
	AssignedTo     *uuid.UUID
	AssignedToNone bool
	Query          string
	CreatedFrom    *time.Time
	CreatedTo      *time.Time
	Page           int
	PerPage        int
}

func (u *Usecase) List(ctx context.Context, t tenant.Context, in ListInput) ([]*Lead, httpx.Meta, error) {
	if err := authz.Require(t, authz.ActionLeadRead); err != nil {
		return nil, httpx.Meta{}, err
	}

	page := in.Page
	if page < 1 {
		page = 1
	}
	perPage := in.PerPage
	if perPage <= 0 {
		perPage = defaultPerPage
	} else if perPage > maxPerPage {
		perPage = maxPerPage
	}

	filter := ListFilter{
		Status:         in.Status,
		Source:         in.Source,
		AssignedTo:     in.AssignedTo,
		AssignedToNone: in.AssignedToNone,
		Query:          in.Query,
		CreatedFrom:    in.CreatedFrom,
		CreatedTo:      in.CreatedTo,
		Page:           page,
		PerPage:        perPage,
	}

	leads, total, err := u.store.Repos().Lead.FindAllByOrg(ctx, t, filter)
	if err != nil {
		return nil, httpx.Meta{}, fmt.Errorf("lead: list: %w", err)
	}
	return leads, httpx.Meta{Page: page, PerPage: perPage, Total: total}, nil
}

func (u *Usecase) Update(ctx context.Context, t tenant.Context, id uuid.UUID, in UpdateLeadInput) (*Lead, error) {
	if err := authz.Require(t, authz.ActionLeadUpdate); err != nil {
		return nil, err
	}

	repoIn := UpdateInput{
		Name:    in.Name,
		Email:   normalizeEmail(in.Email),
		Phone:   in.Phone,
		Company: in.Company,
		Notes:   in.Notes,
	}
	// Phone normalization only replaces PhoneE164 when the new phone
	// parses — UpdateInput has no PhoneE164 field (see its doc comment
	// in entity.go: this minimal Update can't clear a field to NULL,
	// and phone_e164 has the same limitation for now).

	updated, err := u.store.Repos().Lead.Update(ctx, t, id, in.Version, repoIn)
	if errors.Is(err, ErrVersionConflict) {
		return nil, &VersionConflictError{Current: updated}
	}
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// UpdateStatus validates the transition (TD §5) before touching the
// database — loading the current lead first is what makes that possible
// (transition validity depends on the FROM status, which the client's
// requested Version alone doesn't tell us). The whole thing — load,
// validate, update, record status_changed — runs inside one
// store.InTx: TD §10 requires the activity to be atomic with the status
// change it describes, so this can no longer read the current lead via
// a plain store.Repos() call the way #20 originally wrote it.
func (u *Usecase) UpdateStatus(ctx context.Context, t tenant.Context, id uuid.UUID, in UpdateStatusInput) (*Lead, error) {
	if err := authz.Require(t, authz.ActionLeadUpdate); err != nil {
		return nil, err
	}

	var updated *Lead
	var conflict bool
	txErr := u.store.InTx(ctx, func(r Repos) error {
		current, err := r.Lead.FindByID(ctx, t, id)
		if err != nil {
			return err
		}
		// Captured now, not read from current after UpdateStatus runs:
		// current may be the same backing struct a subsequent call
		// mutates in place (true of the in-memory test fake; not
		// something the real Postgres repository does, but nothing here
		// should depend on that).
		fromStatus := current.Status

		if !validateStatusTransition(fromStatus, in.Status) {
			return invalidStatusTransitionError()
		}

		var lostReason *string
		if in.Status == StatusLost {
			if in.LostReason == nil || *in.LostReason == "" {
				return httpx.NewValidationError(httpx.ErrorDetail{Field: "lost_reason", Code: "required"})
			}
			lostReason = in.LostReason
		}
		// Leaving lost (or any non-lost destination) always clears
		// lost_reason, regardless of what the client sent — TD §5.

		result, err := r.Lead.UpdateStatus(ctx, t, id, in.Version, in.Status, lostReason)
		if errors.Is(err, ErrVersionConflict) {
			updated = result
			conflict = true
			return ErrVersionConflict
		}
		if err != nil {
			return err
		}
		updated = result

		return r.Activity.Record(ctx, t, id, "status_changed", t.MembershipID, map[string]any{
			"from": fromStatus, "to": in.Status,
		})
	})
	if conflict {
		return nil, &VersionConflictError{Current: updated}
	}
	if txErr != nil {
		return nil, txErr
	}
	return updated, nil
}

// UpdateAssignment sets or clears id's assignee and records the
// resulting activity (lead_assigned or lead_unassigned) atomically with
// it (TD §11), plus a notification when assigning to someone other than
// the actor — assigning to yourself is deliberately silent (TD §11:
// "memberi tahu seseorang tentang tindakannya sendiri hanya menambah
// bising").
func (u *Usecase) UpdateAssignment(ctx context.Context, t tenant.Context, id uuid.UUID, in UpdateAssignmentInput) (*Lead, error) {
	if err := authz.Require(t, authz.ActionLeadAssign); err != nil {
		return nil, err
	}

	var updated *Lead
	var conflict bool
	txErr := u.store.InTx(ctx, func(r Repos) error {
		current, err := r.Lead.FindByID(ctx, t, id)
		if err != nil {
			return err
		}
		fromAssignee := current.AssignedToMembershipID

		result, err := r.Lead.UpdateAssignment(ctx, t, id, in.Version, in.AssignedToMembershipID)
		if errors.Is(err, ErrVersionConflict) {
			updated = result
			conflict = true
			return ErrVersionConflict
		}
		if err != nil {
			return err
		}
		updated = result

		if in.AssignedToMembershipID == nil {
			return r.Activity.Record(ctx, t, id, "lead_unassigned", t.MembershipID, map[string]any{"from": fromAssignee})
		}

		newAssignee := *in.AssignedToMembershipID
		if err := r.Activity.Record(ctx, t, id, "lead_assigned", t.MembershipID, map[string]any{"from": fromAssignee, "to": newAssignee}); err != nil {
			return err
		}

		if t.MembershipID != nil && newAssignee == *t.MembershipID {
			return nil // self-assignment — no notification
		}
		title := fmt.Sprintf("Lead #%d ditugaskan kepada Anda", updated.LeadNumber)
		return r.Notification.Notify(ctx, t, newAssignee, "lead_assigned", &id, nil, title, &updated.Name)
	})
	if conflict {
		return nil, &VersionConflictError{Current: updated}
	}
	if errors.Is(txErr, ErrAssigneeNotFound) {
		return nil, httpx.NewValidationError(httpx.ErrorDetail{Field: "assigned_to_membership_id", Code: "not_found"})
	}
	if txErr != nil {
		return nil, txErr
	}
	return updated, nil
}

func (u *Usecase) Delete(ctx context.Context, t tenant.Context, id uuid.UUID) error {
	if err := authz.Require(t, authz.ActionLeadDelete); err != nil {
		return err
	}
	return u.store.Repos().Lead.Delete(ctx, t, id)
}

// validateStatusTransition implements TD §5, with one documented
// approximation: leaving "lost" allows moving to ANY main-path status,
// not specifically the one it left from — that requires history only
// activities (#21) provides, and leads itself never remembers what its
// status was before the last transition. See notes.md's "## #20" for
// the full reasoning; none of this issue's acceptance criteria exercise
// that specific case.
func validateStatusTransition(from, to string) bool {
	if from == StatusUnqualified || from == StatusSpam {
		return false // final — no outgoing transition at all
	}
	if to == from {
		return to == StatusLost // only allowed same-status case: updating lost_reason
	}
	if to == StatusUnqualified || to == StatusSpam || to == StatusLost {
		return true // reachable from any non-final status
	}

	toIdx := mainPathIndex(to)
	if toIdx == -1 {
		return false // not a real status at all
	}
	if from == StatusLost {
		return true // leaving lost — approximation, see doc comment above
	}
	fromIdx := mainPathIndex(from)
	if fromIdx == -1 {
		return false
	}
	diff := toIdx - fromIdx
	return diff == 1 || diff == -1
}

func mainPathIndex(status string) int {
	for i, s := range mainPath {
		if s == status {
			return i
		}
	}
	return -1
}

func normalizeEmail(email *string) *string {
	if email == nil {
		return nil
	}
	lower := strings.ToLower(strings.TrimSpace(*email))
	return &lower
}

// normalizePhone returns the E.164 form when phone parses, nil
// otherwise — never an error (freeze bagian 2.3, TD §6).
func normalizePhone(raw *string) *string {
	if raw == nil {
		return nil
	}
	e164, ok := phone.ToE164(*raw)
	if !ok {
		return nil
	}
	return &e164
}

func invalidStatusTransitionError() error {
	return &httpx.DomainError{
		Status:  http.StatusUnprocessableEntity,
		Code:    "invalid_status_transition",
		Message: "Transisi status tidak diizinkan.",
	}
}

// insufficientScopeError mirrors authz.InsufficientScopeError's exact
// code and status locally rather than calling it — the same duplication
// customer.alreadyConvertedError already accepts for reusing lead's own
// codes across a package boundary. This is a business rule outside
// authz.Require's own gate entirely (an API key sending
// assigned_to_membership_id is syntactically valid, not an action
// authz.Action enumerates), but Rule #24's spirit says a caller should
// see the identical shape either way.
func insufficientScopeError() error {
	return &httpx.DomainError{
		Status:  http.StatusForbidden,
		Code:    "insufficient_scope",
		Message: "Kredensial API tidak memiliki scope untuk field ini.",
	}
}

// maybeCleanupExpiredIdempotencyKeys sweeps t.OrganizationID's
// idempotency_key column for entries older than 48h (TD §7, keputusan
// D3 — closing the retention debt Phase 2 TD §19 recorded). Throttled
// to at most once per organization per hour; runs synchronously but
// cheap (one indexed UPDATE), and its error is deliberately discarded —
// TD §7 is explicit that a failed sweep must never fail the lead
// actually being created, and this package has never taken a logger
// dependency (unlike invitation, which sends email).
func (u *Usecase) maybeCleanupExpiredIdempotencyKeys(ctx context.Context, t tenant.Context) {
	u.idempotencyCleanupMu.Lock()
	last, seen := u.idempotencyCleanupThrottle[t.OrganizationID]
	due := !seen || time.Since(last) >= idempotencyCleanupThrottleWindow
	if due {
		u.idempotencyCleanupThrottle[t.OrganizationID] = time.Now()
	}
	u.idempotencyCleanupMu.Unlock()

	if !due {
		return
	}
	_ = u.store.Repos().Lead.CleanupExpiredIdempotencyKeys(ctx, t)
}
