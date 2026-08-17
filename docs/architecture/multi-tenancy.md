# Multi-Tenancy

> **Dokumen ini hampir selalu relevan.** Baca di setiap session yang menyentuh data.
> Sumber: `freeze.md` bagian 5 (Aturan #1–#7) · ADR-002

---

## Strategi

```
PostgreSQL
  └── shared database
       └── shared schema
            └── organization_id di setiap tabel bisnis
```

Satu-satunya yang layak secara ekonomi di model harga Jualin. Schema-per-tenant meledak saat 1000 tenant (migrasi harus jalan 1000 kali); database-per-tenant terlalu mahal.

**Kegagalan di area ini bersifat fatal.** Satu kebocoran lintas tenant menghapus kredibilitas B2B secara permanen, dan tidak ada permintaan maaf yang memulihkannya.

`organization_id` saja **tidak cukup** — itu hanya mengandalkan disiplin developer, dan disiplin gagal pada jam 2 pagi di sprint yang sibuk. Karena itu: empat lapis.

---

## Lapis 1 — Repository yang tidak bisa lupa

Tidak boleh ada jalur query yang menerima org sebagai parameter opsional.

```go
// BENAR — org_id di-inject di dalam, bukan diserahkan ke caller
func (r *LeadRepo) FindByID(ctx context.Context, t TenantContext, id uuid.UUID) (*Lead, error)

// DILARANG — method seperti ini tidak boleh ada sama sekali
func (r *LeadRepo) FindByID(ctx context.Context, id uuid.UUID) (*Lead, error)
```

**Prinsipnya:** kalau method tanpa tenant tidak ada, tidak ada yang bisa memanggilnya secara salah. *Make illegal states unrepresentable*, diterapkan pada tenancy.

Ini bukan abstraksi tambahan — ini bentuk signature yang sama-sama harus ditulis, hanya dipilih yang aman.

**Konsekuensi:** ini alasan tambahan menolak ORM. Dengan sqlc/pgx, setiap query bisa dibaca dan diaudit sebagai teks.

---

## Lapis 2 — Composite foreign key

**Bagian yang hampir selalu terlewat.** Lapis 1 melindungi *pembacaan*. Yang tidak terlindungi: **referensi silang tenant**.

### Skenario nyata

```
Owner Org A → PATCH /v1/leads/{lead_milik_A}
              { "assigned_to_membership_id": "<membership milik Org B>" }
```

- Query lead-nya tenant-scoped → **lolos**
- FK ke `memberships(id)` valid → **lolos**
- **Hasil: data korup lintas tenant tanpa satupun error**

Kalau `assigned_to` menunjuk `user_id`, kerusakannya lebih parah: karyawan Org B melihat lead Org A di aplikasi mobile-nya.

### Pencegahan di database

```sql
-- Pada SETIAP tabel tenant-scoped — jangkar composite FK
CONSTRAINT uq_<tabel>_id_org UNIQUE (id, organization_id)

-- Pada SETIAP FK antar entity bisnis
CONSTRAINT fk_leads_assigned_membership
  FOREIGN KEY (assigned_to_membership_id, organization_id)
  REFERENCES memberships (id, organization_id)
```

Sekarang database **mustahil** menyimpan referensi lintas tenant, apapun bug di kode aplikasi.

> `UNIQUE (id, organization_id)` terlihat berlebihan karena `id` sudah PK. Ia **wajib**: PostgreSQL mensyaratkan unique constraint tepat pada kolom yang direferensikan composite FK.

### Berlaku untuk

`lead → membership` · `lead → customer` · `lead → api_key` · `lead → form` · `task → lead` · `task → membership` · `activity → lead` · `activity → membership` · `notification → membership` · `invitation → membership` · `refresh_token → membership` · `audit_log → membership` · `deal → customer`

**Tanpa pengecualian.** FK yang menunjuk `organizations` atau `users` memakai bentuk biasa, karena keduanya bukan tabel tenant-scoped.

> Biayanya satu unique index tambahan per tabel. **Ini rasio manfaat-per-biaya tertinggi di seluruh sistem**, dan ia bekerja bahkan ketika kode aplikasi salah.

---

## Lapis 3 — Row Level Security (ditunda)

RLS PostgreSQL dengan `SET LOCAL app.current_org_id` + policy per tabel.

**Ditunda ke pasca-MVP.** Alasan jujur: RLS berinteraksi rumit dengan connection pooling dan menambah beban debugging nyata untuk tim kecil. Lapis 1, 2, dan 4 sudah memberi perlindungan sangat besar.

**Dua hal yang wajib dijaga sekarang agar RLS bisa masuk nanti tanpa nyeri** — keduanya gratis hari ini, mahal diperbaiki nanti:

1. Nama kolom **konsisten `organization_id`** di semua tabel, tanpa pengecualian
2. **Semua** akses DB lewat satu transaction manager — satu tempat untuk menyisipkan `SET LOCAL`

---

## Lapis 4 — Test suite isolasi tenant

**Wajib. Blocking di CI.**

### Bentuknya generik, bukan per endpoint

Test manual per endpoint akan tertinggal begitu endpoint bertambah. Buat **satu test yang berjalan di atas daftar route**:

```
Untuk setiap endpoint tenant-scoped:
  siapkan Org A + data, Org B + data
  panggil endpoint dengan kredensial A terhadap resource id milik B
  harapkan 404
```

**Selalu 404, jangan 403** — 403 mengonfirmasi bahwa resource dengan id tersebut ada, dan itu kebocoran informasi tersendiri.

### Kasus wajib

| # | Kasus | Menjaga |
|---|---|---|
| 1 | Baca resource tenant lain → 404 | Lapis 1 |
| 2 | Ubah/hapus resource tenant lain → 404 | Lapis 1 |
| 3 | Menunjuk membership tenant lain di body → ditolak **database** | Lapis 2 |
| 4 | Employee membaca lead employee lain di org yang sama → 404 | Otorisasi |
| 5 | **User dengan dua membership** tidak bisa melihat data org yang tidak sedang aktif di token | ADR-007 |
| 6 | Katalog: setiap tabel tenant-scoped punya `organization_id` + `UNIQUE (id, organization_id)` | Aturan #1, #2 |

**Kasus #5 tidak boleh dilewatkan.** Schema mengizinkan multi-membership yang tidak diekspos UI; satu-satunya penjaganya adalah test ini.

**Kasus #6 adalah penegak otomatis.** Sekali ditulis, ia menangkap setiap tabel baru yang lupa mengikuti konvensi — untuk selamanya, tanpa ada yang perlu mengingatnya saat review.

### Kriteria kualitas harness

> Harness harus terbukti **bisa gagal**. Hapus `organization_id` dari satu query secara sengaja; kalau test tetap hijau, harness-nya belum benar.
>
> Test isolasi yang selalu hijau karena tidak benar-benar menguji apapun **lebih berbahaya daripada tidak ada test** — ia memberi rasa aman palsu pada semua session berikutnya.

---

## Tenant context

```
Request
  → Auth middleware (user session | API key | public form key)
  → resolve principal
  → resolve organization_id     ← SELALU dari kredensial
  → cek status subscription
  → cek quota                    (Phase 8)
  → rate limit
  → Service
  → Repository
```

```go
type TenantContext struct {
    OrganizationID uuid.UUID
    PrincipalType  PrincipalType   // user | api_key | public_form | system
    MembershipID   *uuid.UUID      // nil bila API key
    UserID         *uuid.UUID
    Role           Role
    APIKeyID       *uuid.UUID
    RequestID      string
}
```

### Dua aturan yang mudah dilanggar tanpa sadar

**1. `organization_id` tidak pernah datang dari client** (Aturan #5)

Tidak ada DTO yang boleh punya field `organization_id`. Bukan divalidasi — **tidak ada sama sekali**. Kalau field-nya tidak ada, tidak ada yang bisa memalsukannya.

**2. `PrincipalType` wajib diperiksa, bukan hanya `OrganizationID`** (Aturan #24)

Dua request dengan `organization_id` sama tapi principal berbeda **bukan** request yang setara. Tanpa pemeriksaan ini, endpoint yang mengecek tenant dengan benar tetap bisa menerima API key di tempat yang seharusnya hanya menerima user session.

---

## Checklist saat menambah tabel baru

- [ ] Diklasifikasikan: tenant root / global identity / tenant-scoped / session
- [ ] Bila tenant-scoped: `organization_id NOT NULL`
- [ ] Bila tenant-scoped: `UNIQUE (id, organization_id)`
- [ ] Setiap FK ke tabel tenant-scoped berbentuk composite
- [ ] Index tenant-aware berawalan `organization_id`
- [ ] Repository-nya menerima `TenantContext` di parameter pertama
- [ ] Masuk ke harness test isolasi
- [ ] Bila melanggar salah satu di atas: **tulis ADR**, jangan diam-diam
