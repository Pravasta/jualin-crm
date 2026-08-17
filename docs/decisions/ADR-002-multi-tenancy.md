# ADR-002 — Multi-Tenancy: Shared Schema + Empat Lapis Isolasi

> **Status:** ✅ Accepted — 17 Agustus 2026
> **Detail teknis:** `docs/architecture/multi-tenancy.md`

## Konteks

Jualin CRM adalah SaaS multi-tenant. Data antar organization harus benar-benar terisolasi — satu kebocoran lintas tenant menghapus kredibilitas B2B secara permanen, dan tidak bisa dipulihkan.

Tiga strategi umum: database-per-tenant, schema-per-tenant, shared schema dengan kolom tenant.

## Keputusan

### Strategi: shared database, shared schema, `organization_id`

| Alternatif | Kenapa ditolak |
|---|---|
| Database-per-tenant | Terlalu mahal untuk model harga terjangkau |
| Schema-per-tenant | Meledak saat 1000 tenant — setiap migration harus jalan 1000 kali |

### Empat lapis pertahanan

`organization_id` saja hanya mengandalkan disiplin developer, dan disiplin gagal pada sprint yang sibuk.

| Lapis | Isi | Status |
|---|---|---|
| **1. Repository tenant-scoped** | Setiap method menerima `TenantContext` di parameter pertama. Method tanpa tenant **tidak ada sama sekali**. | ✅ Phase 1 |
| **2. Composite foreign key** | `FOREIGN KEY (x_id, organization_id) REFERENCES t (id, organization_id)` pada setiap FK antar entity bisnis | ✅ Phase 1 |
| **3. Row Level Security** | `SET LOCAL app.current_org_id` + policy per tabel | ⏸️ Ditunda |
| **4. Test isolasi generik** | Satu harness di atas daftar route, blocking di CI | ✅ Phase 1 |

## Alasan tiap lapis

**Lapis 1** menerapkan *make illegal states unrepresentable* pada tenancy. Kalau method tanpa tenant tidak ada, tidak ada yang bisa memanggilnya secara salah. Ini juga alasan tambahan menolak ORM — query implisit tidak bisa diaudit.

**Lapis 2 adalah yang paling sering terlewat.** Lapis 1 melindungi *pembacaan*; yang tidak terlindungi adalah *referensi silang tenant*:

```
Owner Org A → PATCH /v1/leads/{lead_A}
              { "assigned_to_membership_id": "<membership Org B>" }
```

Query tenant-scoped lolos. FK ke `memberships(id)` valid. **Data korup lintas tenant tanpa satupun error.** Composite FK membuat ini mustahil di level database, apapun bug di aplikasi. Biayanya satu unique index per tabel — rasio manfaat-per-biaya tertinggi di seluruh sistem.

**Lapis 4** dibuat generik atas daftar route, bukan manual per endpoint, agar endpoint baru otomatis ikut teruji tanpa ada yang perlu mengingatnya.

## Kenapa RLS ditunda

RLS berinteraksi rumit dengan connection pooling dan menambah beban debugging yang nyata untuk tim kecil. Lapis 1, 2, dan 4 sudah memberi perlindungan sangat besar.

**Ini penundaan, bukan pembatalan.** Dua hal dijaga sejak awal agar RLS bisa masuk nanti tanpa nyeri — keduanya gratis hari ini, mahal diperbaiki nanti:

1. Nama kolom **konsisten `organization_id`** di semua tabel, tanpa pengecualian
2. **Semua** akses DB lewat satu transaction manager

**Dievaluasi ulang bila:** ada pelanggan enterprise yang menanyakannya, atau tim bertambah sehingga review manual tidak lagi menjangkau semua query.

## Konsekuensi

**Positif:** biaya per tenant rendah · migration sekali jalan · kebocoran struktural dicegah database, bukan hanya kode

**Negatif:** satu database berarti satu titik kegagalan · noisy neighbour mungkin terjadi · setiap tabel baru menanggung kewajiban konvensi

**Mitigasi negatif ketiga:** test katalog (`information_schema`) menegakkan Aturan #1 dan #2 secara otomatis untuk setiap tabel baru, selamanya.

## Aturan turunan

Aturan #1–#7 di `freeze.md` bagian 5. Yang paling mudah dilanggar tanpa sadar:

> **`organization_id` tidak pernah datang dari client.** Tidak ada DTO yang boleh punya field itu — bukan divalidasi, melainkan **tidak ada sama sekali**. Kalau field-nya tidak ada, tidak ada yang bisa memalsukannya.
