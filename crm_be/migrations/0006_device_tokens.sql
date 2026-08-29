-- Phase 5 — Employee Mobile. First migration of the phase.
-- Spec: docs/phases/05-employee-mobile/td.md §8. Column shapes below are
-- copied from that spec, not re-derived.

-- +goose Up

-- device_tokens — token FCM per instalasi aplikasi per perangkat.
-- Tanpa deleted_at (deviasi sadar Aturan #18): ini bukan entity bisnis —
-- tidak punya nilai audit, tidak pernah dirujuk laporan, dan token mati
-- memang harus benar-benar hilang (kriteria #12 phase ini), bukan
-- disimpan sebagai sampah yang harus diingat untuk difilter di setiap
-- query.
CREATE TABLE device_tokens (
    id              uuid PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations (id),
    membership_id   uuid NOT NULL,

    token           text NOT NULL,
    platform        text NOT NULL,

    created_at      timestamptz NOT NULL DEFAULT now(),
    last_seen_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT ck_device_tokens_platform CHECK (platform IN ('android', 'ios')),
    CONSTRAINT uq_device_tokens_id_org   UNIQUE (id, organization_id),
    -- token unik LINTAS organization, bukan per organization — pengecualian
    -- ketiga di codebase ini, sekelas api_keys.key_id (0005) dan
    -- refresh_tokens.token_hash (0002). Token FCM mengidentifikasi SATU
    -- instalasi aplikasi di SATU perangkat, dan perangkat itu bisa
    -- berpindah pengguna (employee resign, HP dipakai orang lain, employee
    -- pindah organization). Unik global + upsert pada kolom ini membuat
    -- pendaftaran ulang MEMINDAHKAN baris ke membership yang sekarang
    -- login — perilaku yang benar. Composite (token, organization_id)
    -- akan membiarkan satu perangkat fisik menerima push milik dua
    -- organization sekaligus.
    CONSTRAINT uq_device_tokens_token    UNIQUE (token),
    CONSTRAINT fk_device_tokens_membership
        FOREIGN KEY (membership_id, organization_id)
        REFERENCES memberships (id, organization_id)
);

CREATE INDEX idx_device_tokens_org_membership
    ON device_tokens (organization_id, membership_id);

-- +goose Down
DROP TABLE IF EXISTS device_tokens;
