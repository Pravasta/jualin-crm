-- Phase 1 — Auth & Organization. First domain migration.
-- Spec: docs/architecture/freeze.md bagian 8.3. Column shapes below are
-- copied from that spec, not re-derived.

-- +goose Up

-- organizations — tenant root. No organization_id column: this table IS
-- the tenant, not a tenant-scoped table (freeze bagian 1.1).
CREATE TABLE organizations (
    id         uuid PRIMARY KEY,
    name       text NOT NULL,
    timezone   text NOT NULL DEFAULT 'Asia/Jakarta',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE TRIGGER trg_organizations_updated_at BEFORE UPDATE ON organizations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- users — global identity. Deliberately has no organization_id: one user
-- can hold memberships in more than one organization (ADR-007).
CREATE TABLE users (
    id                uuid PRIMARY KEY,
    email             text NOT NULL UNIQUE,
    password_hash     text NOT NULL,
    full_name         text NOT NULL,
    email_verified_at timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    deleted_at        timestamptz,

    CONSTRAINT ck_users_email_lowercase CHECK (email = lower(email))
);

CREATE TRIGGER trg_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- memberships — the anchor every other composite FK in this database
-- points back to. uq_memberships_id_org exists solely so composite FKs
-- referencing (id, organization_id) are possible at all: PostgreSQL
-- requires a unique constraint on exactly the columns a composite FK
-- targets.
CREATE TABLE memberships (
    id              uuid PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations (id),
    user_id         uuid NOT NULL REFERENCES users (id),
    role            text NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz,

    CONSTRAINT ck_memberships_role CHECK (role IN ('owner', 'admin', 'manager', 'employee')),
    CONSTRAINT uq_memberships_id_org UNIQUE (id, organization_id)
);

CREATE TRIGGER trg_memberships_updated_at BEFORE UPDATE ON memberships
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- One active membership per (user, organization). Soft-deleted rows are
-- excluded, so a re-invited former member doesn't collide with their old
-- (deactivated) membership.
CREATE UNIQUE INDEX uq_memberships_org_user_active
    ON memberships (organization_id, user_id)
    WHERE deleted_at IS NULL;

CREATE INDEX ix_memberships_org_role
    ON memberships (organization_id, role)
    WHERE deleted_at IS NULL;

-- Resolves "which memberships does this user have" during login, across
-- organizations — the query pattern ADR-007's multi-membership model
-- depends on.
CREATE INDEX ix_memberships_user
    ON memberships (user_id)
    WHERE deleted_at IS NULL;

-- ⛔ NEVER add: CREATE UNIQUE INDEX ... ON memberships (user_id);
-- This is deliberate (ADR-007), not an oversight. See freeze bagian 8.3
-- "TIDAK ADA UNIQUE(user_id)" before touching this.

-- subscriptions — minimal shape. Full plan/entitlement/usage machinery is
-- Phase 8; this only records that every organization has a free plan
-- from the moment it registers.
CREATE TABLE subscriptions (
    id                    uuid PRIMARY KEY,
    organization_id       uuid NOT NULL REFERENCES organizations (id),
    plan_code             text NOT NULL DEFAULT 'free',
    status                text NOT NULL,
    current_period_start  timestamptz,
    current_period_end    timestamptz,
    external_reference    text,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT ck_subscriptions_status CHECK (status IN ('active', 'past_due', 'suspended', 'canceled')),
    CONSTRAINT uq_subscriptions_id_org UNIQUE (id, organization_id)
);

CREATE TRIGGER trg_subscriptions_updated_at BEFORE UPDATE ON subscriptions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Only one active subscription per organization at a time; history is
-- kept by simply not deleting old rows.
CREATE UNIQUE INDEX uq_subscriptions_org_active
    ON subscriptions (organization_id)
    WHERE status = 'active';

-- invitations
CREATE TABLE invitations (
    id                       uuid PRIMARY KEY,
    organization_id          uuid NOT NULL REFERENCES organizations (id),
    email                    text NOT NULL,
    role                     text NOT NULL,
    token_hash               text NOT NULL UNIQUE,
    invited_by_membership_id uuid NOT NULL,
    expires_at               timestamptz NOT NULL,
    accepted_at              timestamptz,
    revoked_at               timestamptz,
    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT ck_invitations_email_lowercase CHECK (email = lower(email)),
    -- Owner is never invited — it is only created at registration.
    -- Allowing it here would open a privilege-escalation path through
    -- invitations.
    CONSTRAINT ck_invitations_role CHECK (role IN ('admin', 'manager', 'employee')),
    CONSTRAINT uq_invitations_id_org UNIQUE (id, organization_id),

    CONSTRAINT fk_invitations_inviter
        FOREIGN KEY (invited_by_membership_id, organization_id)
        REFERENCES memberships (id, organization_id)
);

CREATE TRIGGER trg_invitations_updated_at BEFORE UPDATE ON invitations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- One pending invitation per (organization, email).
CREATE UNIQUE INDEX uq_invitations_org_email_pending
    ON invitations (organization_id, email)
    WHERE accepted_at IS NULL AND revoked_at IS NULL;

-- email_verification_tokens & password_reset_tokens — same shape, kept
-- as two tables on purpose: their lifetimes, flows, and security
-- consequences differ, and merging them invites a reset token being
-- accepted where a verification token was expected.
CREATE TABLE email_verification_tokens (
    id         uuid PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES users (id),
    token_hash text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE password_reset_tokens (
    id         uuid PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES users (id),
    token_hash text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- refresh_tokens
CREATE TABLE refresh_tokens (
    id              uuid PRIMARY KEY,
    organization_id uuid NOT NULL,
    membership_id   uuid NOT NULL,
    token_hash      text NOT NULL UNIQUE,
    family_id       uuid NOT NULL,
    client          text NOT NULL,
    device_id       text,
    user_agent      text,
    ip              inet,
    expires_at      timestamptz NOT NULL,
    revoked_at      timestamptz,
    replaced_by_id  uuid REFERENCES refresh_tokens (id),
    created_at      timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT ck_refresh_tokens_client CHECK (client IN ('dashboard', 'mobile')),
    CONSTRAINT uq_refresh_tokens_id_org UNIQUE (id, organization_id),

    CONSTRAINT fk_refresh_tokens_membership
        FOREIGN KEY (membership_id, organization_id)
        REFERENCES memberships (id, organization_id)
);

CREATE INDEX ix_refresh_tokens_family ON refresh_tokens (family_id);

-- audit_logs — append-only. No updated_at, no deleted_at, no update/delete
-- endpoint anywhere in the application.
CREATE TABLE audit_logs (
    id                   uuid PRIMARY KEY,
    organization_id       uuid NOT NULL REFERENCES organizations (id),
    actor_type            text NOT NULL,
    actor_membership_id   uuid,
    action                text NOT NULL,
    entity_type           text,
    entity_id             uuid,
    old_values            jsonb,
    new_values            jsonb,
    metadata              jsonb,
    request_id            text,
    created_at            timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT ck_audit_logs_actor_type CHECK (actor_type IN ('user', 'api_key', 'system')),
    CONSTRAINT uq_audit_logs_id_org UNIQUE (id, organization_id),

    CONSTRAINT fk_audit_actor
        FOREIGN KEY (actor_membership_id, organization_id)
        REFERENCES memberships (id, organization_id)
);

CREATE INDEX ix_audit_org_created ON audit_logs (organization_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS password_reset_tokens;
DROP TABLE IF EXISTS email_verification_tokens;
DROP TABLE IF EXISTS invitations;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS memberships;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS organizations;
