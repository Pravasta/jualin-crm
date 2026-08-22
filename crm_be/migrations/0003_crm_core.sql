-- Phase 2 — CRM Core. First domain migration of the phase.
-- Spec: docs/phases/02-crm-core/td.md §1. Column shapes below are copied
-- from that spec, not re-derived.

-- +goose Up

-- leads — domain inti. No CHECK requiring email or phone: rejecting a
-- lead at ingest time means discarding a customer's data, which is
-- unrecoverable (freeze bagian 2.3).
CREATE TABLE leads (
    id                        uuid PRIMARY KEY,
    organization_id           uuid NOT NULL REFERENCES organizations (id),
    lead_number               integer NOT NULL,

    name                      text NOT NULL,
    email                     text,
    phone                     text,
    phone_e164                text,
    company                   text,
    notes                     text,

    status                    text NOT NULL DEFAULT 'new',
    lost_reason               text,
    source                    text NOT NULL,
    assigned_to_membership_id uuid,

    -- Sumber capture eksternal (api/form/webhook) berbentuk sembarang dan
    -- tidak boleh dibuang begitu field tak dikenal muncul — field itu
    -- tetap tersimpan di sini supaya lead bisa direkonstruksi saat
    -- integrator melapor "data saya hilang" (Aturan #17). Tidak pernah
    -- di-query isinya; hanya disimpan dan ditampilkan apa adanya.
    raw_payload               jsonb,
    idempotency_key           text,
    version                   integer NOT NULL DEFAULT 1,

    created_by_membership_id  uuid,
    created_at                timestamptz NOT NULL DEFAULT now(),
    updated_at                timestamptz NOT NULL DEFAULT now(),
    deleted_at                timestamptz,

    CONSTRAINT ck_leads_status CHECK (status IN
        ('new','contacted','qualified','proposal','won','lost','unqualified','spam')),
    CONSTRAINT ck_leads_source CHECK (source IN ('manual','api','form','webhook')),
    CONSTRAINT ck_leads_lost_reason CHECK (lost_reason IS NULL OR lost_reason IN
        ('price','competitor','timing','no_response','not_interested','other')),
    -- Aturan #4 acceptance criteria ditegakkan di database, bukan hanya
    -- usecase — tidak ada jalur lain (migration, perbaikan manual) yang
    -- bisa melanggarnya.
    CONSTRAINT ck_leads_lost_requires_reason CHECK (status <> 'lost' OR lost_reason IS NOT NULL),
    CONSTRAINT ck_leads_email_lowercase CHECK (email IS NULL OR email = lower(email)),

    CONSTRAINT uq_leads_id_org UNIQUE (id, organization_id),
    CONSTRAINT uq_leads_org_number UNIQUE (organization_id, lead_number),

    -- created_by_membership_id nullable: lead dari api/form/webhook tidak
    -- punya pembuat manusia.
    CONSTRAINT fk_leads_assignee
        FOREIGN KEY (assigned_to_membership_id, organization_id)
        REFERENCES memberships (id, organization_id),
    CONSTRAINT fk_leads_creator
        FOREIGN KEY (created_by_membership_id, organization_id)
        REFERENCES memberships (id, organization_id)
);

CREATE TRIGGER trg_leads_updated_at BEFORE UPDATE ON leads
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX ix_leads_org_status    ON leads (organization_id, status)      WHERE deleted_at IS NULL;
CREATE INDEX ix_leads_org_assignee  ON leads (organization_id, assigned_to_membership_id) WHERE deleted_at IS NULL;
CREATE INDEX ix_leads_org_created   ON leads (organization_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX ix_leads_org_phone     ON leads (organization_id, phone_e164)  WHERE deleted_at IS NULL AND phone_e164 IS NOT NULL;

-- Kolom & constraint idempotency dibuat sekarang meski belum ada yang
-- menulisnya — endpoint POST /v1/leads (issue #20) bergantung padanya.
CREATE UNIQUE INDEX uq_leads_org_idempotency
    ON leads (organization_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- customers — hasil konversi Lead, entity terpisah. converted_from_lead_id
-- NOT NULL: di MVP setiap customer berasal dari lead, tidak ada endpoint
-- "buat customer langsung".
CREATE TABLE customers (
    id                         uuid PRIMARY KEY,
    organization_id            uuid NOT NULL REFERENCES organizations (id),

    name                       text NOT NULL,
    email                      text,
    phone                      text,
    phone_e164                 text,
    company                    text,
    notes                      text,

    converted_from_lead_id     uuid NOT NULL,
    converted_by_membership_id uuid,
    converted_at               timestamptz NOT NULL DEFAULT now(),

    created_at                 timestamptz NOT NULL DEFAULT now(),
    updated_at                 timestamptz NOT NULL DEFAULT now(),
    deleted_at                 timestamptz,

    CONSTRAINT ck_customers_email_lowercase CHECK (email IS NULL OR email = lower(email)),
    CONSTRAINT uq_customers_id_org UNIQUE (id, organization_id),

    CONSTRAINT fk_customers_lead
        FOREIGN KEY (converted_from_lead_id, organization_id)
        REFERENCES leads (id, organization_id),
    CONSTRAINT fk_customers_converter
        FOREIGN KEY (converted_by_membership_id, organization_id)
        REFERENCES memberships (id, organization_id)
);

CREATE TRIGGER trg_customers_updated_at BEFORE UPDATE ON customers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Satu lead hanya boleh menghasilkan satu customer — ditegakkan database,
-- bukan sekadar dicek usecase.
CREATE UNIQUE INDEX uq_customers_org_lead ON customers (organization_id, converted_from_lead_id);

-- activities — append-only, immutable. Tanpa updated_at, tanpa deleted_at,
-- tanpa trigger set_updated_at: ketiadaan kolom itu SENDIRI adalah
-- penegakannya — tidak ada endpoint edit/hapus yang bisa ditulis tanpa
-- mengubah schema lebih dulu.
CREATE TABLE activities (
    id                  uuid PRIMARY KEY,
    organization_id     uuid NOT NULL REFERENCES organizations (id),
    lead_id             uuid NOT NULL,
    type                text NOT NULL,
    actor_membership_id uuid,
    body                text,
    -- Bentuk metadata berbeda per type (lead_assigned menyimpan {from,to},
    -- status_changed menyimpan {from,to}, call_logged menyimpan durasi).
    -- Kolom terpisah per tipe berarti belasan kolom yang mayoritas selalu
    -- NULL (Aturan #17). Tidak pernah di-query isinya — hanya dirender di
    -- timeline.
    metadata            jsonb,
    created_at          timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT ck_activities_type CHECK (type IN (
        'lead_created','lead_assigned','lead_unassigned','status_changed','lead_converted',
        'note_added','call_logged','whatsapp_opened','task_created','task_completed')),
    CONSTRAINT uq_activities_id_org UNIQUE (id, organization_id),

    CONSTRAINT fk_activities_lead
        FOREIGN KEY (lead_id, organization_id) REFERENCES leads (id, organization_id),
    CONSTRAINT fk_activities_actor
        FOREIGN KEY (actor_membership_id, organization_id) REFERENCES memberships (id, organization_id)
);

CREATE INDEX ix_activities_org_lead ON activities (organization_id, lead_id, created_at DESC);

-- tasks — lead_id NOT NULL (B8): task selalu menempel pada satu lead.
-- Melonggarkan NOT NULL nanti adalah satu ALTER; mengetatkannya berarti
-- membersihkan baris yatim lebih dulu.
CREATE TABLE tasks (
    id                         uuid PRIMARY KEY,
    organization_id            uuid NOT NULL REFERENCES organizations (id),
    lead_id                    uuid NOT NULL,

    title                      text NOT NULL,
    description                text,
    due_at                     timestamptz,
    status                     text NOT NULL DEFAULT 'open',

    assigned_to_membership_id  uuid,
    completed_at               timestamptz,
    completed_by_membership_id uuid,

    version                    integer NOT NULL DEFAULT 1,
    created_by_membership_id   uuid,
    created_at                 timestamptz NOT NULL DEFAULT now(),
    updated_at                 timestamptz NOT NULL DEFAULT now(),
    deleted_at                 timestamptz,

    -- Hanya open/done. Tidak ada "cancelled" — task yang tidak jadi
    -- dikerjakan dihapus (soft delete); deleted_at sudah membedakannya
    -- dari yang selesai.
    CONSTRAINT ck_tasks_status CHECK (status IN ('open','done')),
    CONSTRAINT ck_tasks_done_requires_completed_at CHECK (status <> 'done' OR completed_at IS NOT NULL),
    CONSTRAINT uq_tasks_id_org UNIQUE (id, organization_id),

    CONSTRAINT fk_tasks_lead      FOREIGN KEY (lead_id, organization_id) REFERENCES leads (id, organization_id),
    CONSTRAINT fk_tasks_assignee  FOREIGN KEY (assigned_to_membership_id, organization_id) REFERENCES memberships (id, organization_id),
    CONSTRAINT fk_tasks_creator   FOREIGN KEY (created_by_membership_id, organization_id)  REFERENCES memberships (id, organization_id),
    CONSTRAINT fk_tasks_completer FOREIGN KEY (completed_by_membership_id, organization_id) REFERENCES memberships (id, organization_id)
);

CREATE TRIGGER trg_tasks_updated_at BEFORE UPDATE ON tasks
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX ix_tasks_org_assignee_due ON tasks (organization_id, assigned_to_membership_id, due_at)
    WHERE deleted_at IS NULL AND status = 'open';
CREATE INDEX ix_tasks_org_lead ON tasks (organization_id, lead_id) WHERE deleted_at IS NULL;

-- next_lead_number sengaja ditambahkan sekarang, bukan di 0002 — di 0002
-- belum ada leads, sehingga kolom itu akan menjadi schema mati. ALTER
-- TABLE ADD COLUMN dengan DEFAULT konstan bersifat instan pada PostgreSQL
-- modern.
ALTER TABLE organizations ADD COLUMN next_lead_number integer NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE organizations DROP COLUMN IF EXISTS next_lead_number;
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS activities;
DROP TABLE IF EXISTS customers;
DROP TABLE IF EXISTS leads;
