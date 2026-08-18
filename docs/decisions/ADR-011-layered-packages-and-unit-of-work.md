# ADR-011 — Layering per Paket, Interface di Sisi Consumer, Unit of Work

> **Status:** ✅ Accepted — 19 Agustus 2026
> **Konteks:** Diminta pemilik produk sebelum melanjutkan Phase 1 ke #10/#11 — lihat issue #15
> **Mengubah:** [freeze.md](../architecture/freeze.md) bagian 5 (Aturan #8, #11), `CLAUDE.md` bagian Layering
> **Tidak mengubah:** Aturan #9, #10 (repository tanpa business logic, otorisasi di service layer) · struktur package-by-feature · penolakan Clean Architecture folder-per-lapis

## Konteks

Dua hal terjadi bersamaan dan perlu dipisahkan dengan jelas, karena satu adalah **penegakan aturan yang sudah ada** dan satu lagi adalah **perubahan aturan**.

### 1. Aturan #11 sudah ada sejak freeze, dan sedang dilanggar

Freeze menyatakan sejak awal: *"Interface didefinisikan di sisi consumer."* Implementasi `internal/auth` (issue #9) tidak mengikutinya — `Service` mengonstruksi tipe konkret langsung di dalam `db.InTx`:

```go
txErr := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
    orgRepo := organization.New(tx)      // *organization.Repository
    userRepo := user.New(tx)             // *user.Repository
    membershipRepo := membership.New(tx) // *membership.Repository
    ...
```

**Konsekuensi yang sudah terasa, bukan teoretis:** seluruh 19 test `internal/auth` membutuhkan testcontainers — termasuk `TestRegister_WeakPassword_Rejected`, yang hanya memvalidasi panjang password dan tidak menyentuh database sama sekali. Business logic tidak bisa diuji terpisah dari infrastruktur karena keduanya menyatu di titik konstruksi.

### 2. Permintaan eksplisit: struktur bergaya Clean Architecture

Pemilik produk meminta restrukturisasi mengikuti Clean Architecture, dengan penekanan pada penggunaan interface. Opsi yang dipilih setelah dipaparkan trade-off-nya: **layering per paket** (bukan folder-per-lapis di level `internal/`), dengan Unit of Work untuk transaksi.

## Keputusan

### Yang berubah

**Aturan #8** direvisi dari:

> `Handler → Service → Repository → PostgreSQL`

menjadi:

> `Handler → Usecase → Repository (interface) → Repository (implementasi) → PostgreSQL`, dengan setiap paket domain mendeklarasikan lapisnya lewat penamaan berkas eksplisit.

**Struktur per paket domain:**

```
internal/<domain>/
  entity.go               tipe domain murni — tanpa dependensi infrastruktur
  port.go                 interface yang DIBUTUHKAN paket ini, + Repos + Store
  usecase.go               business logic — hanya bergantung pada port.go
  repository_postgres.go  implementasi PostgreSQL dari interface repository
  handler_http.go          gin — mengubah HTTP ↔ usecase
```

**Aturan #11 ditegakkan secara harfiah**, bukan sekadar dinyatakan: interface didefinisikan di `port.go` milik **consumer**, memuat hanya method yang benar-benar dipakai paket itu — bukan cermin seluruh repository.

### Unit of Work untuk transaksi lintas repository

```go
// internal/auth/port.go
type Repos struct {
    User   UserRepository
    Org    OrganizationRepository
    Member MembershipRepository
    Sub    SubscriptionRepository
    Verify VerificationTokenRepository
    Audit  AuditRepository
}

type Store interface {
    InTx(ctx context.Context, fn func(Repos) error) error
    Repos() Repos // non-transactional, untuk operasi baca tunggal
}
```

```go
// usecase.go — tidak pernah menyentuh pgx.Tx
func (u *Usecase) Register(ctx context.Context, in RegisterInput) (*RegisterOutput, error) {
    err := u.store.InTx(ctx, func(r Repos) error {
        if _, err := r.Org.Create(ctx, ...); err != nil { return err }
        if _, err := r.User.Create(ctx, ...); err != nil { return err }
        ...
        return nil
    })
    ...
}
```

`db.InTx` (Phase 0) **tetap ada** sebagai primitif tingkat rendah — `Store` membungkusnya, tidak menggantikannya. Lihat "Keputusan desain: lokasi `Repos`/`Store`" di bawah untuk alasan kenapa keduanya didefinisikan per-domain, bukan di `internal/shared/db`.

### Yang TIDAK berubah

| Aturan | Status |
|---|---|
| **#9** — Repository tidak berisi business logic, service (kini usecase) tidak tahu HTTP | Tetap. `usecase.go` tidak mengimpor `gin`; `repository_postgres.go` tidak berisi keputusan bisnis. |
| **#10** — Otorisasi di service layer | Tetap, sekarang di `usecase.go`. |
| **Package-by-feature** | Tetap. `internal/auth`, `internal/lead`, dst. — satu fitur, satu folder. |
| **Penolakan Clean Architecture folder-per-lapis** | **Tetap ditolak.** Lihat bagian berikut. |

## Kenapa bukan folder-per-lapis (`domain/`, `usecase/`, `adapter/`, `infrastructure/`)

Ini opsi yang paling dekat dengan diagram Clean Architecture kanonik, dan sengaja **tidak** dipilih.

| | Folder-per-lapis | Layering per paket (dipilih) |
|---|---|---|
| Menambah satu endpoint | Menyentuh 4 direktori | Menyentuh 1 direktori |
| Menemukan kode satu fitur | Tersebar di `domain/x`, `usecase/x`, `adapter/x`, `infrastructure/x` | Satu folder: `internal/x/` |
| Lapis terlihat dari mana | Struktur direktori | Nama berkas (`entity.go`, `port.go`, `usecase.go`) |
| Konsisten dengan review arsitektur awal | Melanggar — review eksplisit menolak `usecase/`+`interactor/`+`entity/`+`gateway/` terpisah | Konsisten |

Alasan intinya sama dengan alasan awal freeze menolaknya: pada skala tim ini, folder-per-lapis menambah biaya navigasi (satu fitur = banyak lokasi) tanpa menambah manfaat yang tidak sudah didapat dari sekadar mendisiplinkan interface. **Interface di sisi consumer sudah memberi seluruh manfaat inti Clean Architecture** — business logic yang tidak bergantung pada framework/database, bisa diuji dengan fake — tanpa biaya navigasi folder-per-lapis.

## Keputusan desain: lokasi `Repos`/`Store`

Ini bukan detail implementasi — ini keputusan arsitektur yang menjaga arah dependensi tetap benar.

**`Repos` dan `Store` didefinisikan di `port.go` masing-masing paket domain, bukan di `internal/shared/db`.**

Alasan: bila `Repos`/`Store` generik tinggal di `shared/db`, ia harus memuat interface yang menyebut tipe domain (`user.User`, `organization.Organization`, dst.) — yang berarti `internal/shared/db` harus mengimpor paket domain. Itu membalik arah dependensi yang dijaga sejak issue #8 (`shared/*` tidak pernah mengimpor `domain/*`).

Solusinya memakai **interface implisit** Go: `*user.Repository` (di `internal/user`) memenuhi `auth.UserRepository` (dideklarasikan di `internal/auth/port.go`) **tanpa paket `user` perlu tahu interface itu ada, apalagi mengimpornya**. Setiap domain yang butuh transaksi lintas repository mendeklarasikan `Repos`/`Store`-nya sendiri, berisi hanya interface yang ia butuhkan.

**Perakitan** (`auth.Repos{User: user.New(tx), Org: organization.New(tx), ...}`) terjadi di **composition root** — lokasi finalnya (`internal/app` atau langsung di `cmd/api`) diputuskan saat implementasi, dengan satu batasan tetap: bukan di `internal/shared/`, dan bukan di paket domain manapun.

## Konsekuensi

**Positif:**
- Business logic bisa diuji dengan fake `Store`/`Repos`, tanpa Docker/testcontainers untuk jalur yang tidak menyentuh database
- Aturan #11 dari freeze akhirnya ditegakkan, bukan sekadar dinyatakan
- Interface minimal per consumer membuat kontrak antar paket eksplisit dan mudah diaudit

**Negatif:**
- Lebih banyak berkas per paket domain (5 dibanding 2-3 sebelumnya)
- Satu lapis tidak langsung (`Repos`/`Store`) antara usecase dan database

**Mitigasi negatif kedua:** `db.InTx` tetap menjadi primitif tunggal di baliknya — Aturan #32 (efek samping di luar transaksi) dan rencana RLS (`multi-tenancy.md` lapis 3) tetap berlaku tanpa perubahan, karena `Store.InTx` hanya membungkus `db.InTx`, bukan menggantikan pola commit/rollback-nya.

## Risiko

**Ini refactor pada kode yang sudah bekerja dan teruji (issue #8, #9).** Risikonya memperkenalkan regresi pada area yang saat ini benar.

**Mitigasi:** 33 test yang ada (level integrasi, testcontainers) menjadi jaring pengaman — **asersinya tidak diubah** selama refactor. Unit test dengan fake ditambahkan di atasnya untuk membuktikan lapis usecase benar-benar lepas dari infrastruktur, bukan menggantikan cakupan test yang sudah ada.

## Kapan dievaluasi ulang

Bila suatu saat folder-per-lapis penuh terasa perlu (mis. tim membesar signifikan dan navigasi per-fitur mulai bermasalah), itu keputusan baru lewat ADR terpisah — bukan migrasi diam-diam dari pola ini.
