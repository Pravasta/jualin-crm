-- +goose Up

-- Phase 8.5 (#123): adds a third notification type so the product can
-- tell an Owner their lead quota is exhausted for the month.
--
-- TD 8.5 §1 originally claimed this phase needed no migration at all —
-- that was wrong, caught while implementing #123 rather than after the
-- fact. ck_notifications_type has closed the set to exactly
-- ('lead_assigned', 'task_assigned') since 0004_notifications.sql; a
-- third value cannot be inserted without widening it here.
ALTER TABLE notifications DROP CONSTRAINT ck_notifications_type;
ALTER TABLE notifications ADD CONSTRAINT ck_notifications_type
    CHECK (type IN ('lead_assigned', 'task_assigned', 'plan_quota_exceeded'));

-- +goose Down

-- Symmetric with Up. Safe only if no plan_quota_exceeded row has been
-- written yet — the same assumption every down migration in this
-- product makes about rolling back before new data is in wide use.
ALTER TABLE notifications DROP CONSTRAINT ck_notifications_type;
ALTER TABLE notifications ADD CONSTRAINT ck_notifications_type
    CHECK (type IN ('lead_assigned', 'task_assigned'));
