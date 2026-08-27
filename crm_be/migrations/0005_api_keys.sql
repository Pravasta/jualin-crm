-- Phase 4 — Public API. First migration of the phase.
-- Spec: docs/phases/04-public-api/td.md §1. Column shapes below are
-- copied from that spec, not re-derived.

-- +goose Up

-- api_keys — kredensial milik organization untuk sistem eksternal
-- pelanggan (ADR-004). Tanpa deleted_at: revoke ≠ hapus, kredensial
-- yang direvoke tetap terbaca — Owner perlu tahu ia pernah ada, siapa
-- membuatnya, kapan terakhir dipakai.
CREATE TABLE api_keys (
    id                        uuid PRIMARY KEY,
    organization_id           uuid NOT NULL REFERENCES organizations (id),

    key_id                    text NOT NULL,
    secret_hash               text NOT NULL,
    key_prefix                text NOT NULL,
    name                      text NOT NULL,
    scopes                    text[] NOT NULL,

    created_by_membership_id  uuid,
    created_at                timestamptz NOT NULL DEFAULT now(),
    last_used_at              timestamptz,
    revoked_at                timestamptz,
    expires_at                timestamptz,

    CONSTRAINT uq_api_keys_id_org UNIQUE (id, organization_id),
    -- key_id unik LINTAS organization, bukan per organization — ini
    -- pengecualian sadar terhadap kebiasaan "unik per tenant". Lookup
    -- terjadi SEBELUM organization diketahui: organization justru hasil
    -- dari lookup ini (Aturan #5, organization_id tidak pernah dari
    -- client). Bentuknya sama persis dengan refresh_tokens.token_hash
    -- dan RefreshTokenRepository.FindByHashForUpdate di Phase 1 — dicatat
    -- di sini karena ia terlihat seperti pelanggaran tenancy bila dibaca
    -- sekilas. UNIQUE ini juga membuat lookup by key_id sebuah index hit
    -- tanpa index terpisah.
    CONSTRAINT uq_api_keys_key_id UNIQUE (key_id),
    CONSTRAINT ck_api_keys_scopes CHECK (scopes <@ ARRAY['leads:write']::text[]),
    -- created_by_membership_id hanya untuk audit (ADR-004 aturan #2) —
    -- kredensial tetap hidup bila pembuatnya keluar dari organization.
    CONSTRAINT fk_api_keys_created_by
        FOREIGN KEY (created_by_membership_id, organization_id)
        REFERENCES memberships (id, organization_id)
);

CREATE INDEX ix_api_keys_org_created ON api_keys (organization_id, created_at DESC);

-- leads.source_api_key_id — nullable, ditambahkan bersama tabel
-- tujuannya (freeze 8.4). leads.source sudah menerima 'api' sejak 0003;
-- nilai enum boleh mendahului FK-nya.
ALTER TABLE leads ADD COLUMN source_api_key_id uuid;
ALTER TABLE leads ADD CONSTRAINT fk_leads_source_api_key
    FOREIGN KEY (source_api_key_id, organization_id)
    REFERENCES api_keys (id, organization_id);

-- +goose Down
ALTER TABLE leads DROP CONSTRAINT IF EXISTS fk_leads_source_api_key;
ALTER TABLE leads DROP COLUMN IF EXISTS source_api_key_id;
DROP TABLE IF EXISTS api_keys;
