-- Phase 6 — Connect & Embedded Form. First migration of the phase.
-- Spec: docs/phases/06-connect-form/td.md §1. Column shapes below are
-- copied from that spec, not re-derived.

-- +goose Up

-- forms — kredensial keempat produk ini menerbitkan (public_key, ADR-005),
-- dan definisi kanal embed itu sendiri. deleted_at (bukan pola api_keys'
-- revoked_at): menonaktifkan form memang dimaksudkan menghilangkannya dari
-- daftar aktif, bukan tetap terlihat dicoret.
CREATE TABLE forms (
    id                        uuid PRIMARY KEY,
    organization_id           uuid NOT NULL REFERENCES organizations (id),

    public_key                text NOT NULL,
    name                      text NOT NULL,
    fields                    jsonb NOT NULL,
    allowed_origins           text[] NOT NULL DEFAULT '{}',
    submit_count              integer NOT NULL DEFAULT 0,

    created_by_membership_id  uuid,
    created_at                timestamptz NOT NULL DEFAULT now(),
    updated_at                timestamptz NOT NULL DEFAULT now(),
    deleted_at                timestamptz,

    CONSTRAINT uq_forms_id_org UNIQUE (id, organization_id),
    -- public_key unik LINTAS organization, bukan per organization —
    -- pengecualian keempat di codebase ini, sekelas refresh_tokens.token_hash
    -- (0002), api_keys.key_id (0005), dan device_tokens.token (0006). Lookup
    -- kredensial terjadi SEBELUM organization diketahui: organization
    -- justru hasil dari lookup ini (Aturan #5, organization_id tidak
    -- pernah dari client). Composite unique tidak mungkin dibuat — tidak
    -- ada organization_id untuk dijadikan bagian kunci sebelum barisnya
    -- ditemukan. UNIQUE ini juga membuat lookup by public_key sebuah
    -- index hit tanpa index terpisah.
    CONSTRAINT uq_forms_public_key UNIQUE (public_key),
    CONSTRAINT ck_forms_name_not_blank CHECK (btrim(name) <> ''),
    -- created_by_membership_id hanya untuk audit — form tetap hidup bila
    -- pembuatnya keluar dari organization (sama seperti api_keys).
    CONSTRAINT fk_forms_created_by
        FOREIGN KEY (created_by_membership_id, organization_id)
        REFERENCES memberships (id, organization_id)
);

-- Parsial: baris yang sudah dihapus tidak pernah muncul di daftar aktif,
-- jadi tidak perlu ikut terindeks.
CREATE INDEX ix_forms_org ON forms (organization_id, created_at DESC)
    WHERE deleted_at IS NULL;

-- leads.source sudah menerima 'form' sejak 0003; nilai enum boleh
-- mendahului FK-nya. ALTER TABLE ADD COLUMN nullable bersifat instan.
ALTER TABLE leads ADD COLUMN source_form_id uuid;
ALTER TABLE leads ADD CONSTRAINT fk_leads_source_form
    FOREIGN KEY (source_form_id, organization_id)
    REFERENCES forms (id, organization_id);

-- +goose Down
ALTER TABLE leads DROP CONSTRAINT IF EXISTS fk_leads_source_form;
ALTER TABLE leads DROP COLUMN IF EXISTS source_form_id;
DROP TABLE IF EXISTS forms;
