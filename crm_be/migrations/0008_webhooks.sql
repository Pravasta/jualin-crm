-- Phase 7 — Outbound Webhook. First migration of the phase.
-- Spec: docs/phases/07-outbound-webhook/td.md §1. Column shapes below are
-- copied from that spec, not re-derived.

-- +goose Up

-- webhook_endpoints — kredensial keempat produk ini menerbitkan, dan yang
-- PERTAMA dengan arah kepercayaan terbalik: tiga sebelumnya (api_key,
-- public_key, device_token) untuk pihak lain membuktikan diri ke kita;
-- signing secret di sini untuk KITA membuktikan diri ke pihak lain.
-- deleted_at (pola forms, bukan api_keys' revoked_at): menonaktifkan
-- endpoint memang dimaksudkan menghilangkannya dari daftar aktif.
CREATE TABLE webhook_endpoints (
    id                        uuid PRIMARY KEY,
    organization_id           uuid NOT NULL REFERENCES organizations (id),

    url                       text NOT NULL,
    secret_hash               text NOT NULL,
    secret_prefix             text NOT NULL,
    events                    text[] NOT NULL,
    description               text NOT NULL DEFAULT '',
    is_active                 boolean NOT NULL DEFAULT true,

    created_by_membership_id  uuid,
    created_at                timestamptz NOT NULL DEFAULT now(),
    updated_at                timestamptz NOT NULL DEFAULT now(),
    deleted_at                timestamptz,

    CONSTRAINT uq_webhook_endpoints_id_org UNIQUE (id, organization_id),
    -- http:// diizinkan di level DB — pengembangan lokal butuh
    -- http://localhost (WEBHOOK_ALLOW_PRIVATE_TARGETS). Penegakan https di
    -- produksi ada di usecase, bukan CHECK: kalau di sini, ia tidak bisa
    -- dilonggarkan per-environment tanpa migration (TD §1).
    CONSTRAINT ck_webhook_endpoints_url_scheme
        CHECK (url LIKE 'https://%' OR url LIKE 'http://%'),
    CONSTRAINT ck_webhook_endpoints_events_not_empty
        CHECK (cardinality(events) > 0),
    -- created_by_membership_id hanya untuk audit — endpoint tetap hidup
    -- bila pembuatnya keluar dari organization (sama seperti api_keys/forms).
    CONSTRAINT fk_webhook_endpoints_created_by
        FOREIGN KEY (created_by_membership_id, organization_id)
        REFERENCES memberships (id, organization_id)
);

-- Parsial: baris yang sudah dihapus tidak pernah muncul di daftar aktif.
CREATE INDEX ix_webhook_endpoints_org ON webhook_endpoints (organization_id, created_at DESC)
    WHERE deleted_at IS NULL;

-- webhook_deliveries — SEKALIGUS riwayat pengiriman (dibaca dashboard) DAN
-- antrian worker. Tidak ada tabel `jobs` generik: outbound webhook
-- satu-satunya konsumen async di produk ini (email & push keduanya
-- fire-and-forget), dan tabel generik untuk satu konsumen melanggar
-- Aturan #28. Penyimpangan tertulis dari freeze bagian 5 ketentuan #4,
-- dicatat penuh di docs/phases/07-outbound-webhook/prd.md D1 beserta
-- kewajiban evaluasi ulangnya.
CREATE TABLE webhook_deliveries (
    id               uuid PRIMARY KEY,
    organization_id  uuid NOT NULL REFERENCES organizations (id),
    endpoint_id      uuid NOT NULL,

    event_type       text NOT NULL,
    -- payload sebagai JSONB (Aturan #17): ia SNAPSHOT event pada saat
    -- terjadi, bukan data yang di-query. Harus dibekukan saat di-enqueue,
    -- bukan dibangun ulang saat dikirim — kalau lead berubah status tiga
    -- kali dalam lima menit, tiga pengiriman itu membawa tiga isi berbeda,
    -- bukan tiga salinan keadaan terakhir. Tidak pernah difilter/diurutkan
    -- berdasarkan isinya.
    payload          jsonb NOT NULL,

    status           text NOT NULL DEFAULT 'pending',
    attempt          integer NOT NULL DEFAULT 0,
    next_attempt_at  timestamptz NOT NULL DEFAULT now(),
    response_status  integer,
    error            text,
    -- delivering_since menandai kapan worker mengklaim baris ini; reaper
    -- (#102) memakainya untuk memulihkan baris yang menggantung setelah
    -- crash. NULL kecuali status = 'delivering'.
    delivering_since timestamptz,

    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT uq_webhook_deliveries_id_org UNIQUE (id, organization_id),
    CONSTRAINT ck_webhook_deliveries_status
        CHECK (status IN ('pending', 'delivering', 'succeeded', 'failed')),
    CONSTRAINT fk_webhook_deliveries_endpoint
        FOREIGN KEY (endpoint_id, organization_id)
        REFERENCES webhook_endpoints (id, organization_id)
);

-- Riwayat per endpoint, dibaca dashboard (kriteria #10). Tenant-aware,
-- berawalan organization_id sesuai Aturan #16.
CREATE INDEX ix_webhook_deliveries_org ON webhook_deliveries
    (organization_id, endpoint_id, created_at DESC);

-- Antrian worker. SENGAJA TIDAK berawalan organization_id — pengecualian
-- tertulis atas Aturan #16 (TD §1.2). Worker mengambil kerja LINTAS
-- seluruh organization: ia infrastruktur, bukan pemanggil tenant-scoped.
-- organization_id adalah HASIL dari baris yang terambil, bukan input untuk
-- mencarinya — predikat query klaim hanya status dan next_attempt_at.
-- Mengawali index dengan organization_id membuatnya tidak terpakai sama
-- sekali oleh query itu. Sekelas apikey.FindByKeyID / form.FindByPublicKey
-- (organization adalah keluaran lookup), bedanya di sini yang dikecualikan
-- index, bukan constraint unik.
CREATE INDEX ix_webhook_deliveries_claim ON webhook_deliveries (next_attempt_at)
    WHERE status = 'pending';

-- +goose Down
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_endpoints;
