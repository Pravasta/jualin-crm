-- Phase 2 — CRM Core. Split from 0003 per freeze bagian 8.4.
-- Spec: docs/phases/02-crm-core/td.md §2. Column shapes below are copied
-- from that spec, not re-derived.

-- +goose Up

-- notifications — tanpa updated_at, tanpa deleted_at: notifikasi dibaca
-- (read_at), tidak dihapus. Bukan entity bisnis yang dikelola pengguna.
CREATE TABLE notifications (
    id                      uuid PRIMARY KEY,
    organization_id         uuid NOT NULL REFERENCES organizations (id),
    recipient_membership_id uuid NOT NULL,
    type                    text NOT NULL,
    lead_id                 uuid,
    task_id                 uuid,
    title                   text NOT NULL,
    body                    text,
    read_at                 timestamptz,
    created_at              timestamptz NOT NULL DEFAULT now(),

    -- task_assigned adalah perluasan kecil yang disengaja terhadap freeze
    -- A3 — biayanya satu nilai enum plus satu call site yang mekanismenya
    -- sudah identik. Tidak dipicu di issue ini (hanya lead_assigned);
    -- disiapkan agar migration tidak perlu diubah lagi saat dipakai.
    CONSTRAINT ck_notifications_type CHECK (type IN ('lead_assigned','task_assigned')),
    CONSTRAINT uq_notifications_id_org UNIQUE (id, organization_id),

    CONSTRAINT fk_notifications_recipient
        FOREIGN KEY (recipient_membership_id, organization_id) REFERENCES memberships (id, organization_id),
    CONSTRAINT fk_notifications_lead
        FOREIGN KEY (lead_id, organization_id) REFERENCES leads (id, organization_id),
    CONSTRAINT fk_notifications_task
        FOREIGN KEY (task_id, organization_id) REFERENCES tasks (id, organization_id)
);

CREATE INDEX ix_notifications_recipient_unread
    ON notifications (organization_id, recipient_membership_id, created_at DESC)
    WHERE read_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS notifications;
