---
name: jualin-backend
description: Konvensi menulis kode backend Go untuk Jualin CRM — layering, repository tenant-scoped, penanganan error, transaksi, migration, dan pola testing. Gunakan setiap kali menulis, mengubah, atau mereview kode Go di repository ini.
---

# Jualin Backend — Konvensi Menulis Kode

> **Skill ini menjawab "bagaimana menulis kode di project ini".**
> Untuk "apa yang harus dibangun dan kenapa", baca `docs/`.
>
> Aturan arsitektur yang mengikat ada di `docs/architecture/freeze.md` bagian 5. Skill ini menerjemahkannya menjadi pola konkret.

---

## Stack

Go · Gin · PostgreSQL · **sqlc atau pgx — bukan ORM** · goose/golang-migrate · `log/slog` · testcontainers

**Tanpa** Redis, message broker, atau search engine. Bila terasa butuh salah satunya, itu sinyal untuk berhenti dan mendiskusikan, bukan menambahkannya.

---

## Struktur

Repository ini **monorepo** ([ADR-009](../../../docs/decisions/ADR-009-monorepo-layout.md)). Kode Go tinggal di **`crm_be/`**, bukan di akar.

```
crm_be/                 ← module github.com/Pravasta/jualin-crm/crm_be
  cmd/api/              HTTP server
  cmd/migrate/          migration runner

  internal/
    <domain>/           satu paket per domain: auth, organization, membership, lead, ...
      handler.go        HTTP: parsing, validasi bentuk, serialisasi
      service.go        business logic, otorisasi, orkestrasi transaksi
      repository.go     akses database — SELALU tenant-scoped
      model.go          struct domain

    shared/
      tenant/           TenantContext
      httpx/            response envelope, error mapping, middleware
      db/               pool, transaction manager
      config/
      logger/

  migrations/           SQL, dijalankan goose
  .golangci.yml
```

Folder sejajar `crm_dashboard/` (Next.js), `crm_landing_page/` (Next.js), dan `crm_employee/` (Flutter) **bukan** urusan skill ini.

`Makefile`, `docker-compose.yml`, dan `.env.example` ada di **akar repository**. Jalankan perintah dari sana, bukan dari `crm_be/`.

**Buat paket hanya saat fiturnya dikerjakan.** Folder kosong adalah utang yang terlihat seperti kemajuan.

---

## Layering

```
Handler → Service → Repository → PostgreSQL
```

| Lapis | Boleh | Tidak boleh |
|---|---|---|
| Handler | Parsing, validasi bentuk, panggil service, serialisasi | Memanggil repository · business logic · query SQL |
| Service | Business logic, otorisasi, transaksi, panggil repository | Menyentuh `*gin.Context` · menentukan HTTP status |
| Repository | Query SQL, mapping ke struct | Business logic · otorisasi |

**Interface didefinisikan di sisi consumer.** Service mendefinisikan interface repository yang dibutuhkannya — bukan paket `interfaces/` terpusat.

---

## Repository tenant-scoped — pola wajib

`TenantContext` **selalu** parameter kedua (setelah `ctx`), dan `organization_id` di-inject **di dalam**, tidak pernah diserahkan ke caller.

```go
func (r *LeadRepository) FindByID(
    ctx context.Context,
    t tenant.Context,
    id uuid.UUID,
) (*Lead, error) {
    const q = `
        SELECT id, organization_id, lead_number, name, status, version, created_at
        FROM leads
        WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL`

    // organization_id datang dari t, TIDAK PERNAH dari argumen terpisah
    ...
}
```

### Yang dilarang

```go
// ⛔ Method tanpa tenant tidak boleh ADA — bukan sekadar tidak dipakai
func (r *LeadRepository) FindByID(ctx context.Context, id uuid.UUID) (*Lead, error)

// ⛔ organization_id sebagai parameter bebas — bisa diisi nilai salah
func (r *LeadRepository) FindByID(ctx context.Context, orgID, id uuid.UUID) (*Lead, error)
```

Kalau method tanpa tenant tidak ada, tidak ada yang bisa memanggilnya secara salah.

### DTO tidak pernah punya `organization_id`

```go
// ⛔ SALAH — membuka jalur pemalsuan tenant
type CreateLeadRequest struct {
    OrganizationID uuid.UUID `json:"organization_id"`
    Name           string    `json:"name"`
}

// ✅ BENAR — organization berasal dari kredensial
type CreateLeadRequest struct {
    Name string `json:"name"`
}
```

Bukan divalidasi — **tidak ada sama sekali**.

---

## Migration

```
0001_baseline.sql
0002_identity.sql
0003_crm_core.sql
```

| Aturan | |
|---|---|
| Penomoran | Berurutan, **tidak pernah diubah** setelah di-merge |
| Reversibilitas | Setiap migration punya `down` yang benar-benar bekerja |
| Isi | Hanya DDL. **Tanpa** seed data bisnis. |
| Nama constraint | Eksplisit: `fk_leads_assigned_membership`, bukan nama bawaan |
| Alasan | Setiap JSONB dan setiap penyimpangan ditulis sebagai komentar SQL |

### Template tabel tenant-scoped

```sql
CREATE TABLE leads (
    id              uuid PRIMARY KEY,              -- UUIDv7 dari aplikasi, tanpa DEFAULT
    organization_id uuid NOT NULL REFERENCES organizations (id),
    -- ... kolom domain ...
    version         integer NOT NULL DEFAULT 1,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz,

    CONSTRAINT uq_leads_id_org UNIQUE (id, organization_id),

    CONSTRAINT fk_leads_assigned_membership
        FOREIGN KEY (assigned_to_membership_id, organization_id)
        REFERENCES memberships (id, organization_id)
);

CREATE INDEX ix_leads_org_created ON leads (organization_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE TRIGGER trg_leads_updated_at BEFORE UPDATE ON leads
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
```

**Tiga hal yang tidak boleh terlewat:** `UNIQUE (id, organization_id)` · composite FK · index berawalan `organization_id`.

---

## Transaksi

**Aturan #32: efek samping eksternal tidak pernah di dalam transaksi.**

```go
// ✅ BENAR
err := db.InTx(ctx, func(tx db.Tx) error {
    user, err := userRepo.Create(ctx, tx, ...)
    org, err := orgRepo.Create(ctx, tx, ...)
    membership, err := membershipRepo.Create(ctx, tx, ...)
    subscription, err := subRepo.CreateFree(ctx, tx, ...)
    return err
})
if err != nil { return err }

// Email dikirim SETELAH commit
if err := mailer.SendVerification(ctx, user); err != nil {
    logger.Error("gagal kirim email verifikasi", "user_id", user.ID, "err", err)
    // TIDAK membatalkan registrasi yang sudah commit
}
```

Setiap alur yang bergantung pada email wajib punya jalur pemulihan mandiri (kirim ulang).

---

## Optimistic locking

`leads` dan `tasks` diubah dari dashboard **dan** mobile — mobile bisa mengirim perubahan yang tertunda beberapa menit dari antrian offline.

```go
const q = `
    UPDATE leads
    SET status = $1, version = version + 1, updated_at = now()
    WHERE id = $2 AND organization_id = $3 AND version = $4`

res, err := tx.Exec(ctx, q, status, id, t.OrganizationID, expectedVersion)
if res.RowsAffected() == 0 {
    return ErrVersionConflict   // → 409 + keadaan terkini di body
}
```

**Jangan pernah menimpa diam-diam.** Konflik ditampilkan ke pengguna.

---

## Error

Sentinel error di domain, mapping terpusat ke HTTP. **Handler tidak menentukan status code sendiri.**

```go
// internal/lead/errors.go
var (
    ErrLeadNotFound         = errors.New("lead not found")
    ErrVersionConflict      = errors.New("version conflict")
    ErrInvalidStatusChange  = errors.New("invalid status transition")
)
```

```go
// shared/httpx — satu tempat
func MapError(err error) (int, ErrorBody) {
    switch {
    case errors.Is(err, lead.ErrLeadNotFound):
        return 404, ErrorBody{Code: "not_found", Message: "Lead tidak ditemukan."}
    case errors.Is(err, lead.ErrVersionConflict):
        return 409, ErrorBody{Code: "version_conflict", ...}
    ...
    }
}
```

**Resource milik tenant lain → 404, tidak pernah 403.** 403 mengonfirmasi resource itu ada.

Bentuk response dan katalog error code: `docs/architecture/api.md`.

---

## Logging

`log/slog`, JSON di produksi, **selalu** dengan `request_id`.

```go
logger.Info("lead created",
    "request_id", t.RequestID,
    "organization_id", t.OrganizationID,
    "lead_id", lead.ID,
)
```

**Jangan pernah mencatat:** password, raw API key, token, payload lead lengkap (PII). Redaksi di logger, bukan di call site.

---

## Testing

| Jenis | Cara |
|---|---|
| Business logic | Unit test murni di service |
| Repository | **PostgreSQL asli** via testcontainers — bukan mock |
| Endpoint | Test HTTP untuk jalur penting |
| Otorisasi | Setiap role × setiap operasi |
| **Isolasi tenant** | **Wajib** untuk setiap feature yang menyentuh tenant boundary |

**Mock database tidak akan pernah menangkap bug isolasi tenant.** Itu alasan repository diuji terhadap PostgreSQL asli.

### Pola test isolasi

```go
func TestLeadIsolation(t *testing.T) {
    orgA, orgB := setupTwoOrgs(t)
    leadB := createLead(t, orgB)

    _, err := leadRepo.FindByID(ctx, orgA.TenantContext(), leadB.ID)
    require.ErrorIs(t, err, lead.ErrLeadNotFound)   // 404, bukan 403
}
```

Kasus wajib lainnya ada di `docs/architecture/multi-tenancy.md` lapis 4 — termasuk kasus **user dengan dua membership**, yang menjaga ADR-007.

---

## Penamaan

Ikuti `docs/product/glossary.md`. Yang paling sering keliru:

| ⛔ Jangan | ✅ Pakai |
|---|---|
| `employee_id`, `staff_id`, `member_id` | `membership_id` |
| `tenant_id` | `organization_id` |
| tabel `employees`, `assignments`, `form_submissions` | `memberships`, kolom di `leads`, `leads` |

Bahasa: **kode dan kolom dalam Inggris**, dokumentasi dan komunikasi dalam Indonesia.

---

## Sebelum menyatakan sebuah feature selesai

- [ ] Test isolasi tenant untuk endpoint baru
- [ ] Otorisasi diuji per role
- [ ] Migration punya `down` yang bekerja
- [ ] Composite FK terpasang pada setiap FK antar entity bisnis
- [ ] Tidak ada `organization_id` di DTO manapun
- [ ] Tidak ada efek samping eksternal di dalam transaksi
- [ ] Error termapping ke `code` yang stabil
- [ ] `docs/STATUS.md` diperbarui
- [ ] `docs/features/<feature>/notes.md` ditulis
