# Phase 2 — CRM Core · Technical Design

> **Bagaimana.** Apa & kenapa di [`prd.md`](./prd.md).
>
> Memuat **delta** untuk Phase 2. Yang sudah ada di [`freeze.md`](../../architecture/freeze.md), [`multi-tenancy.md`](../../architecture/multi-tenancy.md), [`api.md`](../../architecture/api.md), [`authentication.md`](../../architecture/authentication.md), [`authorization.md`](../../architecture/authorization.md), dan `.claude/skills/jualin-backend/` **tidak diulang** — hanya dirujuk.

---

## 1. Schema — migration `0003_crm_core`

Freeze bagian 8.4 menetapkan isinya; bentuk kolom **tidak** ditetapkan di sana (freeze 8.3 hanya merinci `0002`), sehingga diputuskan di dokumen ini.

| Tabel | Kelas | Catatan |
|---|---|---|
| `leads` | tenant-scoped | Domain inti. `lead_number` + `version` + `idempotency_key` |
| `customers` | tenant-scoped | Hasil konversi Lead, entity terpisah |
| `activities` | tenant-scoped | **Append-only** — tanpa `updated_at`, tanpa `deleted_at` |
| `tasks` | tenant-scoped | `lead_id NOT NULL` (B8), `version` |
| `organizations` | — | `ALTER ADD next_lead_number integer NOT NULL DEFAULT 1` |

### 1.1 `leads`

```sql
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
    CONSTRAINT ck_leads_lost_requires_reason CHECK (status <> 'lost' OR lost_reason IS NOT NULL),
    CONSTRAINT ck_leads_email_lowercase CHECK (email IS NULL OR email = lower(email)),

    CONSTRAINT uq_leads_id_org UNIQUE (id, organization_id),
    CONSTRAINT uq_leads_org_number UNIQUE (organization_id, lead_number),

    CONSTRAINT fk_leads_assignee
        FOREIGN KEY (assigned_to_membership_id, organization_id)
        REFERENCES memberships (id, organization_id),
    CONSTRAINT fk_leads_creator
        FOREIGN KEY (created_by_membership_id, organization_id)
        REFERENCES memberships (id, organization_id)
);
```

| Keputusan | Alasan |
|---|---|
| **Tidak ada `CHECK` yang mewajibkan email atau telepon** | Menolak lead di titik ingest berarti membuang data pelanggan — kerusakan yang tidak bisa dibatalkan (freeze 2.3). Lead tanpa kontak diterima; UI menandainya tidak dapat ditindaklanjuti. |
| `created_by_membership_id` **nullable** | Lead dari `api`/`form`/`webhook` tidak punya pembuat manusia. `NOT NULL` akan memaksa membuat membership palsu untuk sistem. |
| `ck_leads_lost_requires_reason` di **database**, bukan hanya usecase | Aturan #4 acceptance criteria. Validasi usecase memberi pesan error yang baik; constraint database memastikan tidak ada jalur lain (migration, perbaikan manual) yang bisa melanggarnya. |
| Tidak ada kolom nilai/revenue | Deal ditunda pasca-Phase 5; freeze 3.2 menyatakan Phase 3 **tanpa** angka revenue. Menambah kolom uang sekarang berarti kolom mati yang mengundang pengisian tidak konsisten. |
| `raw_payload jsonb` — **alasan tertulis wajib (Aturan #17)** | Payload dari sumber eksternal (`api`, `form`, `webhook`) berbentuk sembarang dan **tidak boleh dibuang**: field tak dikenal tetap tersimpan supaya lead bisa direkonstruksi saat integrator melapor "data saya hilang". Tidak pernah di-query isinya di Phase 2 — hanya disimpan dan ditampilkan apa adanya. |

**Index** (Aturan #16 — selalu berawalan `organization_id`):

```sql
CREATE INDEX ix_leads_org_status    ON leads (organization_id, status)      WHERE deleted_at IS NULL;
CREATE INDEX ix_leads_org_assignee  ON leads (organization_id, assigned_to_membership_id) WHERE deleted_at IS NULL;
CREATE INDEX ix_leads_org_created   ON leads (organization_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX ix_leads_org_phone     ON leads (organization_id, phone_e164)  WHERE deleted_at IS NULL AND phone_e164 IS NOT NULL;

CREATE UNIQUE INDEX uq_leads_org_idempotency
    ON leads (organization_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
```

`ix_leads_org_assignee` adalah index yang menopang aturan visibilitas employee (§9) — ia dipakai di **setiap** request employee, bukan sesekali.

### 1.2 `customers`

```sql
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

CREATE UNIQUE INDEX uq_customers_org_lead ON customers (organization_id, converted_from_lead_id);
```

`converted_from_lead_id` **NOT NULL**: di MVP setiap customer berasal dari lead — tidak ada endpoint "buat customer langsung". `uq_customers_org_lead` menjamin satu lead hanya menghasilkan satu customer, sehingga konversi ganda ditolak **database**, bukan sekadar dicek usecase.

> Melonggarkan `NOT NULL` nanti (saat customer boleh dibuat langsung) adalah satu `ALTER`; alasan yang sama dengan B8.

### 1.3 `activities` — append-only

```sql
CREATE TABLE activities (
    id                  uuid PRIMARY KEY,
    organization_id     uuid NOT NULL REFERENCES organizations (id),
    lead_id             uuid NOT NULL,
    type                text NOT NULL,
    actor_membership_id uuid,
    body                text,
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
```

**Tanpa `updated_at`, tanpa `deleted_at`, tanpa trigger `set_updated_at`** — persis seperti `audit_logs` (Aturan #18). Ketiadaan kolom itu adalah penegakannya: tidak ada endpoint edit/hapus yang *bisa* ditulis tanpa mengubah schema lebih dulu.

`actor_membership_id` nullable — activity bertipe sistem (`lead_created` dari API, `status_changed` oleh automation kelak) tidak punya aktor manusia.

`metadata jsonb` — **alasan tertulis (Aturan #17)**: bentuknya berbeda per `type` (`lead_assigned` menyimpan `{from, to}`, `status_changed` menyimpan `{from, to}`, `call_logged` menyimpan durasi). Kolom terpisah untuk tiap tipe berarti belasan kolom yang mayoritas selalu `NULL`. Tidak pernah di-query isinya di Phase 2 — hanya dirender di timeline.

### 1.4 `tasks`

```sql
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

    CONSTRAINT ck_tasks_status CHECK (status IN ('open','done')),
    CONSTRAINT ck_tasks_done_requires_completed_at CHECK (status <> 'done' OR completed_at IS NOT NULL),
    CONSTRAINT uq_tasks_id_org UNIQUE (id, organization_id),

    CONSTRAINT fk_tasks_lead     FOREIGN KEY (lead_id, organization_id) REFERENCES leads (id, organization_id),
    CONSTRAINT fk_tasks_assignee FOREIGN KEY (assigned_to_membership_id, organization_id) REFERENCES memberships (id, organization_id),
    CONSTRAINT fk_tasks_creator  FOREIGN KEY (created_by_membership_id, organization_id)  REFERENCES memberships (id, organization_id),
    CONSTRAINT fk_tasks_completer FOREIGN KEY (completed_by_membership_id, organization_id) REFERENCES memberships (id, organization_id)
);

CREATE INDEX ix_tasks_org_assignee_due ON tasks (organization_id, assigned_to_membership_id, due_at)
    WHERE deleted_at IS NULL AND status = 'open';
CREATE INDEX ix_tasks_org_lead ON tasks (organization_id, lead_id) WHERE deleted_at IS NULL;
```

**Status hanya `open` dan `done`.** Tidak ada `cancelled`: task yang tidak jadi dikerjakan **dihapus** (soft delete), dan `deleted_at` sudah membedakannya dari task yang selesai. Menambah status ketiga yang artinya "diabaikan" berarti dua mekanisme untuk satu maksud.

### 1.5 `ALTER organizations`

```sql
ALTER TABLE organizations ADD COLUMN next_lead_number integer NOT NULL DEFAULT 1;
```

Sengaja di `0003`, bukan `0002` — di `0002` belum ada `leads`, sehingga kolom itu akan menjadi schema mati (freeze 8.4). `ADD COLUMN` dengan `DEFAULT` konstan bersifat instan pada PostgreSQL modern.

### Larangan yang berlaku pada migration ini

- **Tidak ada** `source_api_key_id` maupun `source_form_id` — menyusul bersama tabel tujuannya di `0005`/`0007` (freeze 8.4)
- **Tidak ada** kolom uang di manapun — Deal ditunda
- **Tidak ada** `updated_at`/`deleted_at` pada `activities`
- Setiap FK ke tabel tenant-scoped **wajib composite** (Aturan #3) — termasuk FK antar tabel baru

---

## 2. Migration `0004_notifications`

```sql
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

    CONSTRAINT ck_notifications_type CHECK (type IN ('lead_assigned','task_assigned')),
    CONSTRAINT uq_notifications_id_org UNIQUE (id, organization_id),

    CONSTRAINT fk_notifications_recipient
        FOREIGN KEY (recipient_membership_id, organization_id) REFERENCES memberships (id, organization_id),
    CONSTRAINT fk_notifications_lead FOREIGN KEY (lead_id, organization_id) REFERENCES leads (id, organization_id),
    CONSTRAINT fk_notifications_task FOREIGN KEY (task_id, organization_id) REFERENCES tasks (id, organization_id)
);

CREATE INDEX ix_notifications_recipient_unread
    ON notifications (organization_id, recipient_membership_id, created_at DESC)
    WHERE read_at IS NULL;
```

Dipisah dari `0003` mengikuti freeze 8.4. **`task_assigned` adalah perluasan kecil yang disengaja** terhadap freeze A3 (yang menyebut "pembuatan record saat assignment" tanpa membatasi ke lead): menugaskan task ke seseorang tanpa memberitahunya adalah fitur setengah jadi, dan biayanya satu nilai enum plus satu call site yang mekanismenya sudah identik.

`notifications` **tanpa `deleted_at`** — notifikasi dibaca (`read_at`), tidak dihapus. Ia bukan entity bisnis yang dikelola pengguna.

---

## 3. `lead_number` — alokasi berurutan per organization

```sql
UPDATE organizations
SET next_lead_number = next_lead_number + 1
WHERE id = $1
RETURNING next_lead_number - 1;
```

Dijalankan **di dalam transaksi pembuatan lead yang sama**, sebelum `INSERT INTO leads`.

> **Penyimpangan tercatat dari freeze 2.3.** Freeze menyebut `SELECT … FOR UPDATE`. Satu `UPDATE … RETURNING` mengambil row lock yang **sama persis** dan menahannya sampai commit, tetapi dalam satu statement, bukan dua — menghilangkan celah antara membaca dan menulis yang harus diingat untuk dijaga. Efek serialisasinya identik; yang berbeda hanya jumlah statement.

Konsekuensi yang diterima: pembuatan lead terserialisasi **per organization**. Pada volume MVP tidak terasa. `uq_leads_org_number` adalah jaring pengaman terakhir — bila serialisasi ini pernah bocor, database menolak, bukan diam-diam menerima nomor ganda.

**Wajib diuji di bawah konkurensi** (§16), bukan hanya secara berurutan. Test yang membuat lead satu per satu akan hijau bahkan bila locking-nya salah total.

---

## 4. Optimistic locking — `version`

Berlaku pada `leads` dan `tasks` (Aturan #35).

```sql
UPDATE leads
SET <kolom> = …, version = version + 1, updated_at = now()
WHERE id = $1 AND organization_id = $2 AND version = $3 AND deleted_at IS NULL;
```

| Hasil | Respons |
|---|---|
| 1 baris terpengaruh | 200, body memuat `version` baru |
| 0 baris, dan baris **tidak ada** dalam tenant ini | **404** (Aturan #6) |
| 0 baris, dan baris **ada** tapi `version` berbeda | **409 `version_conflict`**, body memuat **keadaan terkini** |

Membedakan dua kasus terakhir memerlukan satu `SELECT` lanjutan setelah `UPDATE` mengembalikan 0 baris. Itu disengaja: mengembalikan 404 untuk konflik versi akan membuat client menghapus lead dari cache lokalnya, dan pada Phase 5 (antrian offline) itu berarti **kehilangan data pengguna**.

**Client tidak pernah menimpa otomatis.** Body 409 memuat keadaan terkini justru supaya client bisa menampilkan konflik, bukan menyelesaikannya sendiri.

---

## 5. Status lead & validasi transisi

Diagram, arti tiap status, dan aturan pengecualian metrik: freeze 2.4. Yang ditetapkan di sini adalah **cara menegakkannya**.

```
new ──► contacted ──► qualified ──► proposal ──► won
 │          │             │             │
 └──────────┴─────────────┴─────────────┴──────► lost        (wajib lost_reason)
 │
 ├──────────────────────────────────────────────► unqualified
 └──────────────────────────────────────────────► spam
```

| Aturan | Ketentuan |
|---|---|
| Maju | Hanya **satu langkah** pada jalur utama `new → contacted → qualified → proposal → won` |
| Mundur | **Satu langkah** (B7) — `qualified → contacted` boleh, `proposal → new` tidak |
| Terminal samping | `lost`, `unqualified`, `spam` dapat dicapai dari status manapun di jalur utama |
| Keluar dari terminal | `lost` → status jalur utama **diizinkan satu langkah** kembali ke status sebelum kalah (lead hidup lagi); `unqualified` dan `spam` **final** |
| `lost` | **Wajib** `lost_reason` ∈ B6 |
| Keluar dari `lost` | `lost_reason` di-`NULL`-kan kembali |
| `won` | **Tidak** mengonversi apapun secara otomatis (B9) |

Ditegakkan di **usecase**, bukan handler dan bukan repository — ia aturan bisnis, dan usecase adalah satu-satunya lapis yang tahu status lama *dan* status baru sekaligus. Transisi tidak sah → **422 `invalid_status_transition`** (sudah ada di katalog `api.md`).

`spam` dan `unqualified` dikecualikan dari metrik konversi. Di Phase 2 belum ada endpoint metrik (itu Phase 3); yang wajib sekarang hanyalah **kedua status itu ada dan terpisah**, sehingga Phase 3 bisa mengecualikannya tanpa migration.

---

## 6. Normalisasi telepon — E.164

Paket baru `internal/shared/phone`, satu fungsi:

```go
func ToE164(raw string) (string, bool)   // "" , false bila tidak bisa diurai
```

| Masukan | Hasil |
|---|---|
| `0812-3456-7890` | `+6281234567890` |
| `+62 812 3456 7890` | `+6281234567890` |
| `62812 3456 7890` | `+6281234567890` |
| `812 3456 7890` | `+6281234567890` |
| `1234` | `""`, `false` |

**Indonesia-first, disengaja.** Bukan implementasi E.164 umum: memakai port libphonenumber berarti dependensi besar dengan tabel metadata seluruh dunia untuk satu negara. Ditulis tangan, ~40 baris, dengan asumsi yang **ditulis di doc comment paket**, bukan disembunyikan.

**Nomor yang tidak bisa diurai tetap diterima.** `phone` menyimpan apa yang diketik pengguna; `phone_e164` `NULL`. Menolak lead karena format telepon adalah membuang data pelanggan — alasan yang sama dengan tidak mewajibkan kontak sama sekali (§1.1).

Jalur upgrade tercatat: begitu ada pelanggan dengan nomor non-Indonesia, ganti isi `ToE164` dengan libphonenumber. Signature-nya sudah benar; yang berubah hanya implementasinya.

---

## 7. Idempotency

Freeze memutuskan penyimpanannya: **kolom di `leads`, bukan tabel tersendiri** — resource-nya **adalah** response-nya.

```
POST /v1/leads
Idempotency-Key: <uuid dari client>
```

| Keadaan | Perilaku |
|---|---|
| Header tidak ada | Lead dibuat biasa, `idempotency_key` `NULL` |
| Key baru | Lead dibuat, key tersimpan → **201** |
| Key sudah ada di organization ini | Lead **yang sama** dikembalikan → **200**, bukan 201, bukan error |

Deteksi lewat pelanggaran `uq_leads_org_idempotency` (unique violation `23505`), bukan `SELECT`-lalu-`INSERT` — pola yang sama dengan `user.ErrEmailTaken` di Phase 1, dan karena alasan yang sama: cek-lalu-tulis adalah race di bawah retry bersamaan, yang justru **persis situasi** yang memicu pemakaian idempotency key.

**Retensi ditunda, tercatat sebagai utang.** `api.md` menyebut retensi 24–48 jam. Phase 2 tidak punya scheduler (tanpa message broker, tanpa cron), sehingga key disimpan selamanya. Konsekuensinya: key yang dipakai ulang setahun kemudian mengembalikan lead lama. Tidak berbahaya, tetapi salah — dan baru benar-benar relevan di Phase 4, saat integrator sungguhan mulai mengirim key. Dicatat di `STATUS.md` bagian Utang Teknis.

---

## 8. Endpoint

Seluruhnya berprefix `/v1` (Aturan #33), di belakang `authn.Middleware`.

### Lead

| Method | Path | Isi |
|---|---|---|
| `POST` | `/v1/leads` | `{name, email?, phone?, company?, notes?, source?, assigned_to_membership_id?}` → 201 (atau 200 bila idempotent replay) |
| `GET` | `/v1/leads` | Filter + pagination (§8.1) |
| `GET` | `/v1/leads/{id}` | Detail |
| `PATCH` | `/v1/leads/{id}` | `{version, name?, email?, phone?, company?, notes?}` → 200 / 409 |
| `PATCH` | `/v1/leads/{id}/status` | `{version, status, lost_reason?}` → 200 / 409 / 422 |
| `PATCH` | `/v1/leads/{id}/assignment` | `{version, assigned_to_membership_id \| null}` → 200 / 409 |
| `DELETE` | `/v1/leads/{id}` | Soft delete → 204 |

`status` dan `assignment` dipisah dari `PATCH` umum karena keduanya punya aturan sendiri (validasi transisi, pembuatan notification) dan menghasilkan activity bertipe berbeda. Menggabungkannya berarti satu handler yang harus menebak maksud dari kombinasi field yang terisi.

### 8.1 Filter & pagination lead

```
GET /v1/leads?status=new,contacted&assigned_to=<membership_id>&source=manual
             &q=budi&created_from=2026-08-01&created_to=2026-08-31
             &page=1&per_page=25
```

| Parameter | Ketentuan |
|---|---|
| `status` | Daftar dipisah koma |
| `assigned_to` | `membership_id`, atau **`none`** untuk "belum ter-assign" |
| `source` | Daftar dipisah koma |
| `q` | Cocok pada `name`, `email`, `phone_e164` — `ILIKE` sederhana, **bukan** full-text |
| `created_from` / `created_to` | ISO 8601 |
| Pagination | Offset (`api.md`): `page`, `per_page` (default 25, maks 100), `meta` memuat `page`/`per_page`/`total` |

`q` memakai `ILIKE '%…%'` tanpa index pendukung. Disengaja: search engine dilarang freeze, dan pada volume MVP sequential scan per organization tidak terasa. Bila suatu saat terasa, jawabannya `pg_trgm`, bukan Elasticsearch — dicatat di sini agar tidak dibahas ulang dari nol.

### Activity

| Method | Path | Isi |
|---|---|---|
| `GET` | `/v1/leads/{id}/activities` | Timeline, terbaru dulu |
| `POST` | `/v1/leads/{id}/activities` | `{type, body?, metadata?}` — **hanya** tipe buatan pengguna |

`type` pada `POST` dibatasi ke `note_added`, `call_logged`, `whatsapp_opened`. Tipe sistem (`lead_created`, `status_changed`, `lead_assigned`, `lead_unassigned`, `lead_converted`, `task_created`, `task_completed`) **ditolak 422** bila dikirim client — activity sistem hanya lahir dari peristiwa yang sebenarnya (§10), dan mengizinkan client memalsukannya berarti timeline berhenti bisa dipercaya.

**Tidak ada** `PATCH` maupun `DELETE`. Append-only.

### Task

| Method | Path | Isi |
|---|---|---|
| `POST` | `/v1/leads/{id}/tasks` | `{title, description?, due_at?, assigned_to_membership_id?}` → 201 |
| `GET` | `/v1/tasks` | Task saya / filter (`assigned_to`, `status`, `due_before`) |
| `GET` | `/v1/leads/{id}/tasks` | Task pada satu lead |
| `PATCH` | `/v1/tasks/{id}` | `{version, title?, description?, due_at?, assigned_to_membership_id?}` |
| `POST` | `/v1/tasks/{id}/complete` | `{version}` → 200, menyetel `status`, `completed_at`, `completed_by` |
| `DELETE` | `/v1/tasks/{id}` | Soft delete → 204 |

`complete` sebagai endpoint tersendiri, bukan `PATCH status=done`: ia menyetel tiga kolom sekaligus dan menghasilkan activity — sama alasannya dengan pemisahan `PATCH /status` pada lead.

### Customer

| Method | Path | Isi |
|---|---|---|
| `POST` | `/v1/leads/{id}/convert` | `{}` → 201 customer. Menolak bila lead bukan `won`, atau sudah pernah dikonversi |
| `GET` | `/v1/customers` | Daftar + pagination |
| `GET` | `/v1/customers/{id}` | Detail |
| `PATCH` | `/v1/customers/{id}` | `{name?, email?, phone?, company?, notes?}` |
| `DELETE` | `/v1/customers/{id}` | Soft delete → 204 |

Konversi berada di bawah `/v1/leads/{id}` karena ia **aksi pada lead**, bukan pembuatan customer dari ketiadaan. `customers` tidak punya `version` — ia tidak diubah dari mobile offline, sehingga tidak ada konflik tulis untuk dideteksi (Aturan #35 menyebut `leads` dan `tasks` secara spesifik).

### Notification

| Method | Path | Isi |
|---|---|---|
| `GET` | `/v1/notifications` | Milik **saya** saja, `?unread=true` opsional |
| `POST` | `/v1/notifications/{id}/read` | → 204 |
| `POST` | `/v1/notifications/read-all` | → 204 |

Notification selalu di-scope ke `recipient_membership_id = t.MembershipID` di **repository**, bukan handler — pola yang sama dengan visibilitas employee (§9). Tidak ada endpoint yang bisa membaca notifikasi orang lain, terlepas dari role.

---

## 9. Visibilitas employee — ditegakkan di repository

Aturan #1 dari empat aturan di [`authorization.md`](../../architecture/authorization.md). Phase 1 belum bisa menegakkannya karena belum ada resource milik employee; Phase 2 adalah tempatnya.

| Role | Lead yang terlihat |
|---|---|
| Owner, Admin | Seluruh organization |
| Manager | Seluruh organization (baca) — freeze 6.3 opsi A, tanpa konsep Team |
| **Employee** | **Hanya `assigned_to_membership_id = t.MembershipID`** |

Ditegakkan di **repository**, bukan handler dan bukan usecase:

```go
func (r *postgresRepository) FindByID(ctx context.Context, t tenant.Context, id uuid.UUID) (*Lead, error) {
    // organization_id SELALU; assigned_to SELALU bila t.Role == employee.
}
```

**Lead employee lain → `httpx.ErrNotFound` (404), bukan 403** — alasan identik Aturan #6: 403 mengonfirmasi bahwa lead dengan id itu ada.

> Ini permukaan kebocoran paling mungkin di mobile app (`architecture_product_review.md` §6). Menegakkannya di usecase berarti setiap usecase baru harus mengingatnya; menegakkannya di repository berarti tidak ada jalur query yang bisa melewatkannya.

Pembagian tugas dengan `authz` tetap seperti [`authorization.md`](../../architecture/authorization.md): `authz.Require` menjawab *"boleh melakukan kelas aksi ini?"*, repository menjawab *"atas baris yang mana?"*.

**`Action` baru** di `internal/shared/authz`:

| Action | Owner | Admin | Manager | Employee |
|---|---|---|---|---|
| `lead.create` | ✅ | ✅ | ✅ | — |
| `lead.list` / `lead.read` | ✅ | ✅ | ✅ | ✅¹ |
| `lead.update` | ✅ | ✅ | ✅ | ✅¹ |
| `lead.delete` | ✅ | ✅ | — | — |
| `lead.assign` | ✅ | ✅ | ✅ | — |
| `lead.convert` | ✅ | ✅ | — | — |
| `activity.create` / `activity.list` | ✅ | ✅ | ✅ | ✅¹ |
| `task.create` / `task.update` / `task.complete` | ✅ | ✅ | ✅ | ✅¹ |
| `task.delete` | ✅ | ✅ | ✅ | — |
| `customer.list` / `customer.read` | ✅ | ✅ | ✅ | ✅¹ |
| `customer.update` / `customer.delete` | ✅ | ✅ | — | — |

¹ **Dibatasi repository** ke lead yang di-assign kepadanya (dan turunannya: activity, task, customer dari lead itu).

---

## 10. Activity otomatis — dalam transaksi yang sama

| Peristiwa | Activity |
|---|---|
| Lead dibuat | `lead_created` |
| Assignment ditetapkan/diubah | `lead_assigned`, `metadata = {from, to}` |
| Assignment dilepas | `lead_unassigned`, `metadata = {from}` |
| Status berubah | `status_changed`, `metadata = {from, to}` |
| Konversi | `lead_converted`, `metadata = {customer_id}` |
| Task dibuat | `task_created`, `metadata = {task_id, title}` |
| Task diselesaikan | `task_completed`, `metadata = {task_id}` |

Seluruhnya ditulis **di dalam `Store.InTx` yang sama** dengan perubahan yang memicunya. Keduanya adalah tulis database — tidak ada efek samping eksternal — sehingga Aturan #32 tidak menghalangi, dan atomisitas justru wajib: timeline yang bercerai dari kejadiannya lebih buruk daripada tidak ada timeline, karena ia terlihat lengkap padahal bohong.

> Bandingkan dengan email di Phase 1, yang justru **wajib di luar** transaksi. Perbedaannya bukan selera — email adalah panggilan jaringan yang bisa menggantung; `INSERT` ke tabel yang sama tidak.

---

## 11. Assignment & notification

`PATCH /v1/leads/{id}/assignment` di dalam **satu** transaksi:

```
1. UPDATE leads … version = version + 1     (optimistic locking, §4)
2. INSERT activities (lead_assigned | lead_unassigned)
3. INSERT notifications                      (hanya bila di-assign ke seseorang)
```

`assigned_to_membership_id` divalidasi milik organization yang sama — dan karena FK-nya composite `(assigned_to_membership_id, organization_id)`, **database menolak** meski usecase lalai. Itu lapis 2 `multi-tenancy.md` bekerja sebagaimana dirancang.

Assign ke diri sendiri **tidak** menghasilkan notification — memberi tahu seseorang tentang tindakannya sendiri hanya menambah bising.

---

## 12. Konversi ke Customer

```
POST /v1/leads/{id}/convert
```

| Prasyarat | Kegagalan |
|---|---|
| Lead ada, dalam tenant, terlihat oleh pemanggil | 404 |
| `authz.Require(t, ActionLeadConvert)` | 403 |
| Status lead **`won`** | 422 `invalid_status_transition` |
| Belum pernah dikonversi | 409 `lead_already_converted` (kode baru) |

Dalam satu transaksi: `INSERT customers` (menyalin `name`/`email`/`phone`/`phone_e164`/`company`) + `INSERT activities (lead_converted)`.

**Lead tidak dihapus dan statusnya tidak berubah.** Ia tetap `won` dan tetap menjadi jejak proses penjualannya; `customers.converted_from_lead_id` adalah tautannya. Menghapus lead setelah konversi berarti membuang seluruh timeline yang menjelaskan bagaimana pelanggan itu didapat.

Data disalin, bukan dirujuk: customer boleh berubah nama atau telepon tanpa mengubah catatan historis lead-nya.

---

## 13. Kewajiban Phase 1 yang ditutup di sini

[`01-auth-organization/td.md`](../01-auth-organization/td.md) §17 dan freeze 2.3:

> Penonaktifan membership **wajib menolak berjalan** bila masih ada lead terbuka, kecuali disertai keputusan eksplisit.

`DELETE /v1/memberships/{id}` bertambah parameter:

```
DELETE /v1/memberships/{id}?on_open_leads=reject|unassign|reassign&reassign_to=<membership_id>
```

| Nilai | Perilaku |
|---|---|
| tidak diisi / `reject` | **Default.** Bila ada lead terbuka → **409 `membership_has_open_leads`** (kode baru), body memuat jumlahnya |
| `unassign` | `assigned_to_membership_id = NULL` untuk lead terbukanya, + activity `lead_unassigned` per lead |
| `reassign` | Dipindah ke `reassign_to`, + activity `lead_assigned` per lead |

"Lead terbuka" = status **bukan** `won`, `lost`, `unqualified`, `spam`, dan `deleted_at IS NULL`.

Seluruhnya dalam **satu transaksi** dengan penonaktifan membership dan pencabutan refresh token yang sudah ada sejak #11. Default `reject` disengaja: diam-diam melepas assignment saat seseorang resign adalah persis kegagalan senyap yang aturan ini ada untuk mencegahnya.

> `internal/membership` akan memerlukan akses ke lead untuk ini — lewat interface sempit yang **dideklarasikan `membership`** (`OpenLeadRepository`: hitung, lepas, pindahkan), diimplementasikan `internal/lead`, dirakit di composition root. Pola yang sama dengan `auth.RefreshTokenRevoker` di #11; `membership` tidak mengimpor `lead`.

---

## 14. Error code baru

Ditambahkan ke katalog di [`api.md`](../../architecture/api.md):

| HTTP | `code` |
|---|---|
| 409 | `lead_already_converted` |
| 409 | `membership_has_open_leads` |

Sudah ada dan dipakai ulang: `version_conflict` (409), `invalid_status_transition` (422), `not_found` (404), `forbidden` (403), `validation_failed` (400), `conflict` (409).

---

## 15. Paket baru

```
internal/lead/            entity · port · usecase · repository_postgres · handler_http
internal/activity/        entity · port · usecase · repository_postgres · handler_http
internal/task/            entity · port · usecase · repository_postgres · handler_http
internal/customer/        entity · port · usecase · repository_postgres · handler_http
internal/notification/    entity · port · usecase · repository_postgres · handler_http
internal/shared/phone/    ToE164
```

Masing-masing dengan `Store`/`Repos` sendiri dan composition root `cmd/api/<domain>_store.go` — pola ADR-011, **bukan** satu `Store` raksasa lintas domain.

`activity` sengaja menjadi paket tersendiri meski selalu ditulis bersama lead/task: ia punya aturan sendiri (append-only, tipe sistem vs pengguna) dan endpoint sendiri. Yang membuatnya bekerja lintas transaksi adalah interface `ActivityRecorder` yang **dideklarasikan konsumennya** (`lead`, `task`, `customer`) — sama seperti `AuditRepository` di Phase 1.

---

## 16. Rencana test

### Wajib di bawah konkurensi

| Test | Membuktikan |
|---|---|
| N goroutine membuat lead bersamaan di satu organization | `lead_number` = 1..N, **tanpa lubang, tanpa duplikat** |
| N goroutine membuat lead di organization berbeda | Nomor per organization saling bebas, keduanya mulai dari 1 |
| Dua `PATCH` bersamaan dengan `version` sama | Tepat **satu** 200, satunya **409** — tidak pernah dua-duanya sukses |
| Dua `POST /v1/leads` bersamaan dengan `Idempotency-Key` sama | Tepat **satu** lead tercipta, keduanya menerima lead yang sama |

Keempatnya **tidak bisa** dibuktikan oleh test berurutan. Test yang membuat lead satu per satu akan hijau bahkan bila locking-nya dihapus seluruhnya — itulah yang membuat keempatnya wajib, bukan opsional.

### Test keamanan wajib

| Skenario | Harapan |
|---|---|
| Employee membaca lead yang di-assign ke employee lain | **404** |
| Employee membaca lead organization lain | **404** |
| Employee membuat lead | **403** |
| Employee membaca activity pada lead orang lain | **404** |
| Employee membaca notification orang lain | **404** / tidak muncul di daftar |
| Menugaskan lead ke membership organization lain | Ditolak **database** (composite FK) |
| Client mengirim activity bertipe sistem (`status_changed`) | **422** |
| Konversi lead yang belum `won` | **422** |
| Konversi lead dua kali | **409** |
| Nonaktifkan membership dengan lead terbuka, tanpa parameter | **409** |

### Harness isolasi tenant

`cmd/api/tenant_isolation_test.go`'s `[]isolationCase` bertambah entri `lead`, `task`, `customer`, `activity` — **menambah entri, bukan menulis harness baru** (itulah bentuk generiknya dirancang sejak #11).

Ini juga yang **mengaktifkan kasus #1 dan #4** di tabel [`multi-tenancy.md`](../../architecture/multi-tenancy.md) lapis 4, yang sampai akhir Phase 1 masih kosong karena belum ada resource milik employee.

**Kriteria kualitas tetap berlaku:** harness harus terbukti bisa gagal. Prosedurnya sudah dijalankan sekali di #11 dan dicatat di `notes.md`; ulangi untuk kasus `lead` — hapus filter `assigned_to` dari repository lead secara sengaja, pastikan merah.

### Test lain

- Transisi status: setiap pasangan sah dan tidak sah, termasuk `lost` tanpa `lost_reason` dan keluar dari `lost`
- `ToE164`: tabel masukan/keluaran termasuk yang tidak bisa diurai
- Activity append-only: tidak ada route `PATCH`/`DELETE` yang terdaftar (diperiksa lewat daftar route, bukan asumsi)
- Setiap usecase punya test unit tanpa Docker (`TestUnit_*`, fake `Store`) — ADR-011

---

## 17. Risiko teknis

| Risiko | Mitigasi |
|---|---|
| **Alokasi `lead_number` salah di bawah beban** — risiko terbesar phase ini; kegagalannya senyap dan datanya tidak bisa diperbaiki retroaktif | Test konkurensi wajib (§16) + `uq_leads_org_number` sebagai jaring pengaman database |
| **Visibilitas employee bocor** karena ditegakkan di usecase, bukan repository | Ditegakkan di repository; harness isolasi + test keamanan tersendiri; review khusus setiap query lead baru |
| **Optimistic locking dilewati** oleh endpoint baru yang lupa meminta `version` | Signature repository `Update(…, version int)` — endpoint yang lupa tidak akan bisa dikompilasi |
| **Activity sistem dipalsukan client** | Allowlist tipe di handler `POST /activities`, ditolak 422; test tersendiri |
| **`ILIKE '%…%'` menjadi lambat** | Diterima untuk MVP; jalur upgrade (`pg_trgm`) dicatat di §8.1 agar tidak jadi diskusi ulang |
| **Idempotency key tanpa retensi** | Dicatat sebagai utang teknis, bukan diam-diam; baru relevan di Phase 4 |

---

## 18. Yang berubah pada dokumentasi

| Berkas | Perubahan |
|---|---|
| [`api.md`](../../architecture/api.md) | Dua error code baru (§14); contoh filter & pagination lead |
| [`authorization.md`](../../architecture/authorization.md) | Matriks bertambah baris Lead/Task/Activity/Customer yang **nyata**; aturan #1 berubah dari "belum berlaku" menjadi terwujud |
| [`multi-tenancy.md`](../../architecture/multi-tenancy.md) | Tabel lapis 4: kasus #1 dan #4 dari "belum ada kasus nyata" menjadi ✅ |
| `STATUS.md` | Phase 2 selesai; utang teknis (retensi idempotency) |
| `phases/02-crm-core/notes.md` | Satu bagian per issue |

---

## 19. Kewajiban yang diteruskan ke phase berikutnya

Ditulis di sini agar tidak bergantung pada ingatan:

> **Phase 3 (Dashboard)** wajib menyediakan filter permanen **"lead tanpa pemilik aktif"** sebagai jaring pengaman terhadap membership yang dinonaktifkan (freeze 2.3 ketentuan #3). Query-nya sudah didukung Phase 2 lewat `assigned_to=none`; yang belum ada hanyalah layarnya.
>
> **Phase 4 (API Publik)** mewarisi retensi `idempotency_key` (§7) dan menambahkan `leads.source_api_key_id` di `0005`.
>
> **Phase 5 (Mobile)** adalah alasan `version` ada. Antrian offline wajib menampilkan 409 kepada pengguna, **tidak pernah** menimpa otomatis (§4).
