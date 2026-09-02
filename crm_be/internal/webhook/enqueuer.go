package webhook

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/db"
	"github.com/Pravasta/jualin-crm/crm_be/internal/shared/tenant"
)

// Enqueuer turns one domain event into pending webhook_deliveries rows —
// one per active endpoint in t.OrganizationID subscribed to that event.
//
// It is bound to a db.Querier rather than to a Store, and that is the
// whole design. The triggering domain (lead) calls it INSIDE its own
// transaction, on that transaction's querier, so the deliveries commit or
// roll back with the row that caused them. Exactly the shape
// activity.NewRecorder has, for exactly the same reason (TD §5):
//
//   - a lead that commits without its deliveries loses the event forever;
//   - deliveries that commit without the lead announce something that
//     does not exist.
//
// This does NOT violate Rule #32 (no external side effects inside a
// transaction). Writing queue rows is an ordinary database write. It is
// precisely this split — enqueue transactionally, deliver outside — that
// lets the HTTP call in #102 obey Rule #32 without dropping events.
//
// There is deliberately no Usecase.Enqueue. A Usecase method carries its
// own Store and would therefore open its own transaction, which is the
// one thing this must never do.
type Enqueuer struct {
	q db.Querier
}

// NewEnqueuer returns an Enqueuer over q. Pass a pgx.Tx to participate in
// a caller's transaction; a pool works too, but no current caller wants
// that and none should.
func NewEnqueuer(q db.Querier) *Enqueuer {
	return &Enqueuer{q: q}
}

// Enqueue writes one pending delivery per matching endpoint and reports
// how many it wrote. Zero is a completely ordinary result — an
// organization with no webhook endpoints is the common case — so callers
// must not treat it as an error.
//
// Matching means all of: same organization, is_active, not soft-deleted,
// and eventType present in the endpoint's events array. Endpoint
// selection happens in SQL against the same transaction snapshot as the
// insert, so an endpoint deleted concurrently cannot be selected here and
// then violate the foreign key on insert.
//
// payload is stored verbatim as the frozen event snapshot (TD §1.1) — it
// is never rebuilt at send time, so three status changes in five minutes
// deliver three different bodies rather than three copies of the final
// state. delivery_id is NOT part of it: each row's own id serves that
// purpose, is stable across every retry of that row (TD §4.2), and is
// injected into the body by the worker at send time (#102). Putting it in
// here would mean one payload per endpoint instead of one shared snapshot,
// for a value the row already carries.
func (e *Enqueuer) Enqueue(ctx context.Context, t tenant.Context, eventType string, payload []byte) (int, error) {
	endpoints, err := e.matchingEndpointIDs(ctx, t, eventType)
	if err != nil {
		return 0, err
	}

	// One INSERT per matching endpoint rather than a single INSERT..SELECT,
	// because every row needs its own application-generated UUIDv7 id
	// (Rule #12: no DEFAULT in the database). Endpoint counts per
	// organization are in the single digits by nature — this loop does not
	// grow with traffic.
	const insert = `
		INSERT INTO webhook_deliveries (id, organization_id, endpoint_id, event_type, payload)
		VALUES ($1, $2, $3, $4, $5)`

	for _, endpointID := range endpoints {
		_, err := e.q.Exec(ctx, insert, uuid.Must(uuid.NewV7()), t.OrganizationID, endpointID, eventType, payload)
		if err != nil {
			return 0, fmt.Errorf("webhook: enqueue %s: %w", eventType, err)
		}
	}
	return len(endpoints), nil
}

func (e *Enqueuer) matchingEndpointIDs(ctx context.Context, t tenant.Context, eventType string) ([]uuid.UUID, error) {
	const q = `
		SELECT id
		FROM webhook_endpoints
		WHERE organization_id = $1
		  AND is_active
		  AND deleted_at IS NULL
		  AND $2 = ANY (events)
		ORDER BY created_at`

	rows, err := e.q.Query(ctx, q, t.OrganizationID, eventType)
	if err != nil {
		return nil, fmt.Errorf("webhook: find endpoints for %s: %w", eventType, err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("webhook: scan endpoint id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("webhook: iterate endpoints for %s: %w", eventType, err)
	}
	return ids, nil
}
