-- Deliberately minimal: this migration only proves that goose's up/down
-- cycle works cleanly. No domain tables here — a table created in Phase 0
-- would be dead schema unused by any code. Domain tables start at 0002
-- (Phase 1). See docs/architecture/freeze.md bagian 8.2.

-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS set_updated_at();
