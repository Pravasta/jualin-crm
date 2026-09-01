package webhook

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/authz"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/httpx"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/safedial"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

const deliveryHistoryPageSize = 20

// Usecase depends only on Store (port.go), a URLValidator (the SSRF
// guard, port.go), and a logger — never on *pgxpool.Pool or pgx.Tx
// directly (ADR-011). The logger carries the DETAIL of a rejected URL
// (which denied range it resolved to); the HTTP response only ever gets
// the generic code (TD §7 — the detail is a network-mapping tool in a
// customer's hands).
type Usecase struct {
	store  Store
	urls   URLValidator
	logger *slog.Logger
}

func NewUsecase(store Store, urls URLValidator, logger *slog.Logger) *Usecase {
	return &Usecase{store: store, urls: urls, logger: logger}
}

// CreateInput is Create's argument. Events must be non-empty and every
// entry a known event; Description is optional.
type CreateInput struct {
	URL         string
	Events      []string
	Description string
}

func (u *Usecase) Create(ctx context.Context, t tenant.Context, in CreateInput) (*Endpoint, string, error) {
	if err := authz.Require(t, authz.ActionWebhookCreate); err != nil {
		return nil, "", err
	}
	if err := u.validateEvents(in.Events); err != nil {
		return nil, "", err
	}
	if err := u.validateURL(ctx, in.URL); err != nil {
		return nil, "", err
	}

	sec, err := generateSecret()
	if err != nil {
		return nil, "", fmt.Errorf("webhook: create: %w", err)
	}

	e := &Endpoint{
		ID:                    uuid.Must(uuid.NewV7()),
		OrganizationID:        t.OrganizationID,
		URL:                   strings.TrimSpace(in.URL),
		SecretHash:            sec.hash,
		SecretPrefix:          sec.prefix,
		Events:                dedupeEvents(in.Events),
		Description:           in.Description,
		IsActive:              true,
		CreatedByMembershipID: t.MembershipID,
	}

	txErr := u.store.InTx(ctx, func(r Repos) error {
		if err := r.Endpoint.Create(ctx, t, e); err != nil {
			return err
		}
		return r.Audit.Record(ctx, t, t.MembershipID, "webhook_endpoint.created")
	})
	if txErr != nil {
		return nil, "", fmt.Errorf("webhook: create: %w", txErr)
	}

	// rawSecret leaves here exactly once (Rule #21) — the handler puts it
	// on the create response and lets it go.
	return e, sec.rawSecret, nil
}

func (u *Usecase) List(ctx context.Context, t tenant.Context) ([]*Endpoint, error) {
	if err := authz.Require(t, authz.ActionWebhookList); err != nil {
		return nil, err
	}
	endpoints, err := u.store.Repos().Endpoint.FindByOrg(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("webhook: list: %w", err)
	}
	return endpoints, nil
}

func (u *Usecase) Get(ctx context.Context, t tenant.Context, id uuid.UUID) (*Endpoint, error) {
	if err := authz.Require(t, authz.ActionWebhookRead); err != nil {
		return nil, err
	}
	e, err := u.store.Repos().Endpoint.FindByID(ctx, t, id)
	if err != nil {
		return nil, err // httpx.ErrNotFound for cross-org or missing
	}
	return e, nil
}

func (u *Usecase) Update(ctx context.Context, t tenant.Context, id uuid.UUID, in UpdateInput) (*Endpoint, error) {
	if err := authz.Require(t, authz.ActionWebhookUpdate); err != nil {
		return nil, err
	}
	if in.Events != nil {
		if err := u.validateEvents(*in.Events); err != nil {
			return nil, err
		}
		deduped := dedupeEvents(*in.Events)
		in.Events = &deduped
	}
	if in.URL != nil {
		if err := u.validateURL(ctx, *in.URL); err != nil {
			return nil, err
		}
		trimmed := strings.TrimSpace(*in.URL)
		in.URL = &trimmed
	}

	e, err := u.store.Repos().Endpoint.Update(ctx, t, id, in)
	if err != nil {
		return nil, err // httpx.ErrNotFound for cross-org or missing
	}
	return e, nil
}

func (u *Usecase) Delete(ctx context.Context, t tenant.Context, id uuid.UUID) error {
	if err := authz.Require(t, authz.ActionWebhookDelete); err != nil {
		return err
	}
	if _, err := u.store.Repos().Endpoint.FindByID(ctx, t, id); err != nil {
		return err
	}
	return u.store.InTx(ctx, func(r Repos) error {
		if err := r.Endpoint.Delete(ctx, t, id); err != nil {
			return err
		}
		return r.Audit.Record(ctx, t, t.MembershipID, "webhook_endpoint.deleted")
	})
}

// ListDeliveries returns one page of an endpoint's delivery history
// (kriteria #10). FindByID first so a cross-org or missing endpoint id
// 404s before the history query runs.
func (u *Usecase) ListDeliveries(ctx context.Context, t tenant.Context, endpointID uuid.UUID, page int) ([]*Delivery, int, error) {
	if err := authz.Require(t, authz.ActionWebhookRead); err != nil {
		return nil, 0, err
	}
	if _, err := u.store.Repos().Endpoint.FindByID(ctx, t, endpointID); err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	deliveries, total, err := u.store.Repos().Delivery.FindByEndpoint(ctx, t, endpointID, page, deliveryHistoryPageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("webhook: list deliveries: %w", err)
	}
	return deliveries, total, nil
}

// RetryDelivery re-queues one failed delivery for an immediate attempt
// (kriteria #11). The pre-check here gives a clean 409 for the common
// case; MarkForRetry's own WHERE status='failed' is the real guard
// against a race with the worker.
func (u *Usecase) RetryDelivery(ctx context.Context, t tenant.Context, id uuid.UUID) (*Delivery, error) {
	if err := authz.Require(t, authz.ActionWebhookUpdate); err != nil {
		return nil, err
	}
	d, err := u.store.Repos().Delivery.FindByID(ctx, t, id)
	if err != nil {
		return nil, err
	}
	if d.Status != StatusFailed {
		return nil, notRetryableError()
	}
	updated, err := u.store.Repos().Delivery.MarkForRetry(ctx, t, id)
	if errors.Is(err, ErrDeliveryNotRetryable) {
		return nil, notRetryableError()
	}
	if err != nil {
		return nil, fmt.Errorf("webhook: retry delivery: %w", err)
	}
	return updated, nil
}

func (u *Usecase) validateEvents(events []string) error {
	if len(events) == 0 {
		return httpx.NewValidationError(httpx.ErrorDetail{Field: "events", Code: "required"})
	}
	for _, ev := range events {
		if !slices.Contains(KnownEvents, ev) {
			return httpx.NewValidationError(httpx.ErrorDetail{Field: "events", Code: "invalid_value"})
		}
	}
	return nil
}

// validateURL maps safedial's single ErrURLNotAllowed to the generic
// 400 code, logging the specific reason (TD §7). Any other error from
// the validator (unexpected) bubbles up as a 500.
func (u *Usecase) validateURL(ctx context.Context, rawURL string) error {
	err := u.urls.ValidateURL(ctx, rawURL)
	if err == nil {
		return nil
	}
	if errors.Is(err, safedial.ErrURLNotAllowed) {
		u.logger.Warn("webhook endpoint url rejected", "reason", err.Error())
		return urlNotAllowedError()
	}
	return fmt.Errorf("webhook: validate url: %w", err)
}

// dedupeEvents removes duplicate entries while preserving first-seen
// order — a customer subscribing to the same event twice is harmless but
// shouldn't be stored twice.
func dedupeEvents(events []string) []string {
	seen := make(map[string]bool, len(events))
	out := make([]string, 0, len(events))
	for _, ev := range events {
		if !seen[ev] {
			seen[ev] = true
			out = append(out, ev)
		}
	}
	return out
}

func urlNotAllowedError() error {
	return &httpx.DomainError{
		Status:  http.StatusBadRequest,
		Code:    "webhook_url_not_allowed",
		Message: "URL webhook tidak diizinkan.",
	}
}

func notRetryableError() error {
	return &httpx.DomainError{
		Status:  http.StatusConflict,
		Code:    "delivery_not_retryable",
		Message: "Hanya pengiriman yang gagal yang bisa dikirim ulang.",
	}
}
