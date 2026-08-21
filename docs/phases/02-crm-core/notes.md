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
