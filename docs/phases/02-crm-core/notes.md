# Phase 2 — CRM Core · Notes

One section per issue, appended as each is implemented.

---

## #19 — Schema 0003, repository lead, alokasi lead_number, optimistic locking

### Menyimpang dari rencana issue

| Yang berbeda | Alasan |
|---|---|
| `internal/lead` **tidak** mendapat `port.go` — hanya `entity.go` + `repository_postgres.go`, dengan `Repository` sebagai struct konkret (bukan interface) | Issue #19 checklist menyebut `port.go`, tapi itu tidak konsisten dengan konvensi yang sudah dua kali ditetapkan codebase ini (`user`, `organization`, dan `membership` sebelum #11): paket "repository murni" tanpa usecase mengekspor struct konkret; `port.go` (interface) baru muncul saat consumer sungguhan (`Usecase`) ada untuk mendeklarasikannya (ADR-011). #20 menambah `port.go` + `usecase.go` saat consumer itu ada, dan `Repository` berubah jadi interface — migrasi yang sama persis dengan yang `membership` alami di #11. |

### Keputusan implementasi

- **Klaim awal di komentar kode terbukti berlebihan, ditemukan sebelum commit.** Draf pertama `Create`'s doc comment menyatakan "callers MUST wrap Create in db.InTx, dibuktikan di repository_concurrency_test.go". Sebelum menulis ini di notes.md, saya uji klaim itu secara langsung: memodifikasi `createConcurrently` sementara untuk memanggil `Create` langsung ke pool (tanpa `db.InTx` sama sekali) dan menjalankan `TestConcurrency_LeadNumber_NoGapsNoDuplicates` lima kali dengan `-race`. **Tetap hijau seluruhnya.** Alasannya: `UPDATE … RETURNING` tunggal sudah atomik dan mengambil row lock terlepas dari apakah ia dibungkus transaksi eksplisit — yang dibuktikan test konkurensi hanyalah alokasi nomor tidak pernah tabrakan, bukan kebutuhan `db.InTx`.

  Yang **sungguh** bergantung pada `db.InTx` adalah skenario berbeda: `INSERT INTO leads` gagal **setelah** `next_lead_number` berhasil dialokasikan (mis. pelanggaran FK). Tanpa transaksi, alokasi itu sudah ter-commit sendiri — nomor itu hilang permanen, sebuah lubang. Di dalam `db.InTx`, kegagalan `INSERT` me-rollback **seluruh** percobaan termasuk alokasinya, sehingga percobaan berikutnya mendapat nomor yang sama, bukan nomor berikutnya.

  Ditulis test baru yang secara langsung membuktikan skenario itu — `TestCreate_FailedInsertInsideInTx_DoesNotBurnLeadNumber` (`repository_test.go`): buat lead pertama (nomor 1), coba `Create` kedua yang **sengaja** melanggar FK `assigned_to_membership_id` di dalam `db.InTx`, pastikan gagal, lalu `Create` ketiga (di luar transaksi manapun, seperti biasa) harus mendapat nomor **2**, bukan 3. Ini benar-benar membuktikan kebutuhan transaksi; tiga test konkurensi tidak.

  Komentar kode (`Create`'s doc comment dan `repository_concurrency_test.go`'s doc comment) ditulis ulang supaya tidak mengklaim lebih dari yang benar-benar dibuktikan test yang menyertainya — kesalahan kecil, ditangkap sebelum commit, tapi dicatat di sini karena persis jenis overclaim yang mudah lolos review kalau tidak diuji langsung.

- **`Update`'s field nullable (`Email`, `Phone`, `Company`, `Notes`) tidak bisa di-set ke `NULL` lewat `UpdateInput`** — `nil` berarti "jangan sentuh", direpresentasikan lewat `COALESCE($n, kolom)`. Tidak ada cara membedakan "jangan sentuh" dari "kosongkan" dengan `*string` tunggal. Disengaja untuk cakupan repository-only issue ini; `#20` menambah mekanisme tri-state (mis. `**string` atau flag terpisah) bila usecase-nya benar-benar butuh mengosongkan field, bukan ditebak sekarang.

- **`ck_leads_lost_requires_reason` dan `uq_customers_org_lead` ditegakkan database**, persis seperti TD — bukan hanya usecase (yang belum ada di issue ini). Terverifikasi lewat schema migration, bukan test Go (belum ada usecase untuk memicunya lewat kode aplikasi).

### Verifikasi

```
go build ./...                              → bersih
go vet ./...                                → bersih
golangci-lint run                           → 0 issues
gofmt -l .                                  → bersih
go test -race -count=1 ./... (2x)           → semua PASS

go test ./internal/lead/... -race -count=1  → 10/10 PASS
  TestConcurrency_LeadNumber_NoGapsNoDuplicates          — 20 goroutine, {1..20} tepat
  TestConcurrency_LeadNumber_IndependentPerOrganization  — dua org paralel, {1..10} masing-masing
  TestConcurrency_VersionConflict_ExactlyOneWins         — 20 update bersamaan, tepat 1 sukses
  TestCreate_FailedInsertInsideInTx_DoesNotBurnLeadNumber — lihat di atas

goose up/down round-trip (0003)             → bersih, tabel 0002 & fungsi 0001 tetap ada setelah rollback 0003
TestCatalog_TenantScopedTables*             → leads/customers/activities/tasks otomatis lolos TANPA perubahan test (dari #8)

go list -deps ./cmd/api | grep docker       → kosong
```

Seluruh acceptance criteria issue #19 terpenuhi.

### Utang teknis

- Tidak ada item baru dari issue ini sendiri.

### Catatan untuk session berikutnya

- **`internal/lead` mengikuti pola `entity.go`+`repository_postgres.go` murni.** #20 menambah `port.go` (interface `Repository` + `Store`/`Repos`) dan `usecase.go` — pada titik itu, struct `Repository` di `repository_postgres.go` berganti nama jadi `postgresRepository` (unexported), persis migrasi `membership` di #11. Jangan kaget saat rename itu terjadi — sudah direncanakan, bukan refactor darurat.
- **`Update` di repository ini hanya field umum** (`name`, `email`, `phone`, `company`, `notes`). Transisi status (`PATCH /status`) dan assignment (`PATCH /assignment`) di TD adalah endpoint terpisah dengan aturan sendiri — `#20` kemungkinan menambah method `Repository` baru untuk itu (`UpdateStatus`, atau usecase memanggil `Update` plus logic tambahan), bukan memperluas `UpdateInput` yang ada.
- **Uji klaim atomisitas secara langsung sebelum menuliskannya di komentar kode**, bukan berasumsi dari desain. Pengalaman di atas (klaim `db.InTx` yang ternyata tidak dibuktikan test konkurensi) adalah contoh konkret kenapa ini penting — desain yang *terlihat* benar bisa salah soal *test mana* yang sebenarnya membuktikannya.

---

## #20 — Lead CRUD, transisi status, filter, pagination, idempotency, E.164

### Menyimpang dari rencana issue

| Yang berbeda | Alasan |
|---|---|
| Keluar dari status `lost` mengizinkan pindah ke **status jalur utama manapun**, bukan spesifik "satu langkah kembali ke status sebelum kalah" seperti bunyi literal TD §5 | `leads` hanya menyimpan status **saat ini** — tidak ada memori status sebelumnya. Riwayat itu baru tersedia lewat `activities` (#21). Didokumentasikan sebagai pendekatan sementara di `usecase.go`'s `validateStatusTransition`; tidak ada acceptance criteria issue #20 yang menguji kasus spesifik ini. Akan disempurnakan begitu `activities` memberi riwayat yang bisa dipakai, bukan ditebak dari nol. |

### Keputusan implementasi

- **Migrasi `internal/lead` dari repository murni ke domain penuh** persis seperti direncanakan di notes.md `## #19`: `Repository` (struct konkret) → interface, implementasi berganti nama `postgresRepository`, `port.go`+`usecase.go` baru. Tidak ada kejutan — sudah dicatat sebelum dikerjakan.
- **Bug nyata ditemukan lewat test yang salah pakai, bukan lewat desain**: saat menulis `TestHandler_Get_Employee_OtherPersonsLead_Returns404`, lead di-assign ke `assigned_to_membership_id` acak yang **tidak** merujuk baris `memberships` sungguhan. `INSERT` melanggar FK composite `fk_leads_assignee` — dan pelanggaran itu **tidak tertangani**, jatuh ke `fmt.Errorf` generik yang dipetakan `internal_error` (500) oleh `httpx.MapError`. Test berikutnya yang memakai id kosong hasil parse gagal itu justru gagal dengan cara membingungkan (301 redirect ke `/v1/leads`, gara-gara path menjadi `/v1/leads/` kosong). Diperbaiki dua arah: (1) test itu sendiri diperbaiki untuk menyeed membership sungguhan, (2) **gap produksi yang terungkap ditutup**: `Create` sekarang mendeteksi pelanggaran FK 23503 pada `fk_leads_assignee` dan mengembalikan `ErrAssigneeNotFound`, dipetakan usecase menjadi `400 validation_failed` yang jelas. Test regresi `TestHandler_Create_InvalidAssignee_Returns400NotInternalError` memastikan ini tidak kembali menjadi 500 diam-diam.
- **Deteksi idempotency key duplikat lewat unique-violation database** (`uq_leads_org_idempotency`, kode `23505`), persis pola `user.ErrEmailTaken` — bukan `SELECT`-lalu-`INSERT`, yang justru race persis di bawah kondisi retry bersamaan yang menjadi alasan idempotency key ada. Dibuktikan dengan test HTTP 10 request `POST` bersamaan memakai key yang sama: tepat 1 `201`, sisanya `200`, seluruhnya merujuk lead yang sama, dan hanya 1 baris di database.
- **`FindAllByOrg` membangun `WHERE` secara manual** (bukan query-builder library — "sqlc/pgx bukan ORM") — daftar kondisi + slice argumen dirakit sekali, dipakai bersama oleh query `count(*)` dan query berpaginasi supaya keduanya tidak pernah berbeda filter.
- **Visibilitas employee di `FindAllByOrg` diterapkan tanpa syarat** — filter `assigned_to` dari query param employee **diabaikan**, diganti paksa dengan `assigned_to_membership_id = t.MembershipID`. Employee yang mencoba memfilter ke lead orang lain mendapat daftar kosong, bukan error — aman by construction, sama seperti `FindByID`/`Update` sejak #19.

### Verifikasi

```
go build ./...                              → bersih
go vet ./...                                → bersih
golangci-lint run                           → 0 issues (setelah perbaikan G101 + 2x S1016)
gofmt -l .                                  → bersih
go test -race -count=1 ./... (2x)           → semua PASS

go test ./internal/lead/... -race -count=1  → 34/34 PASS
go test ./internal/shared/phone/... -count=1 → semua PASS

DOCKER_HOST=unix:///tmp/nonexistent-docker.sock \
  go test ./internal/... -run 'TestUnit|TestRequire|TestToE164' -count=1
                                             → PASS tanpa Docker (lead, authz, phone, + semua paket lama)

go list -deps ./cmd/api | grep docker       → kosong
```

**Test konkurensi idempotency (wajib di Definition of Done)**: 10 request `POST /v1/leads` bersamaan dengan `Idempotency-Key` sama → tepat 1× `201`, 9× `200`, seluruhnya mengembalikan `id` yang sama, dan `SELECT count(*)` di database mengonfirmasi tepat 1 baris.

**Smoke test manual** (`docker compose up --build`): register+verify+login owner → `POST /v1/leads` dengan `Idempotency-Key` → `201`, `lead_number=1`, `phone_e164` ternormalisasi `+6281234567890`, email lower-case → replay key yang sama dengan body berbeda → `200`, lead **sama persis**, nama tidak berubah → `PATCH /status` `new→won` → `422` → `PATCH /status` `new→contacted` → `200` → `PATCH` field umum dengan `version` basi → `409` dengan `error.current` memuat status terkini (`contacted`, `version=2`) → `GET /v1/leads?status=contacted` → daftar & `meta.total` benar.

Seluruh acceptance criteria issue #20 terpenuhi.

### Utang teknis

- Tidak ada item baru dari issue ini sendiri. `docs/STATUS.md` bagian Utang Teknis tetap seperti sebelumnya.

### Catatan untuk session berikutnya

- **`internal/lead/port.go`'s `Repos{Lead Repository}` punya satu field secara sengaja.** #21 menambah field `Activity` saat menulis `activities` perlu atomik dengan perubahan lead (`status_changed`, `lead_assigned`, dst) — pada titik itu, `UpdateStatus`/assignment di usecase perlu dibungkus `store.InTx` juga (saat ini hanya `Create` yang InTx, karena `Create` sendiri butuh atomisitas internal, bukan karena ada tabel lain yang ditulis).
- **Pola deteksi pelanggaran constraint database (unique ATAU foreign key) sebagai sinyal, bukan `SELECT` dulu**, sekarang punya dua contoh di paket ini (`ErrIdempotencyKeyExists`, `ErrAssigneeNotFound`) selain `user.ErrEmailTaken` di Phase 1. Pola yang sama berlaku untuk constraint baru yang muncul di #21/#22/#23 (mis. `uq_customers_org_lead` saat konversi).
- **Manual smoke test menemukan bug produksi untuk ketiga kalinya berturut-turut** (setelah #11 dan sesi ini sendiri, lewat jalur test bukan `docker compose` kali ini) — pola yang konsisten cukup kuat untuk terus dipertahankan di issue berikutnya, bukan dianggap kebetulan.
