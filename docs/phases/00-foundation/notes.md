# Phase 0 — Foundation · Notes

> Realitas implementasi. Satu bagian per issue.

---

## #1 — Project skeleton: config, logging, error, health, CI

### Menyimpang dari TD

| Yang berbeda | Alasan |
|---|---|
| `HandleMethodNotAllowed = true` ditambahkan eksplisit pada `gin.Engine` | Tidak disebutkan di TD. Tanpa flag ini, Gin tidak pernah memanggil `NoMethod` — request dengan method salah jatuh ke `NoRoute` (404) alih-alih 405. Ditemukan lewat test `TestWrongMethod_Returns405`. |
| `httpx.RespondError` (bukan hanya `WriteError`) | TD hanya menyebut "mapping terpusat" tanpa merinci router-level 404/405 yang tidak punya `error` Go untuk di-map. Ditambahkan sebagai fungsi terpisah yang menerima status/code/message langsung. |
| `ValidationError` sebagai tipe (bukan hanya sentinel `ErrValidation`) | TD §6 menyebut sentinel saja. Ditambahkan `ValidationError{Details}` yang meng-unwrap ke `ErrValidation`, supaya `error.details` di response (lihat `api.md`) bisa terisi tanpa menunggu Phase 2. |

Tidak ada penyimpangan pada bentuk config, logging, atau health check — sesuai TD.

### Keputusan implementasi

- **`uuid.NewV7()` bisa gagal** (sangat jarang — kegagalan `crypto/rand`). Fallback ke `uuid.New()` (v4) daripada membiarkan request tanpa `request_id`.
- **Logger diambil dari `gin.Context`**, bukan context standar Go, karena seluruh stack HTTP di Phase 0 memakai `*gin.Context`. Akan dievaluasi ulang saat service layer (Phase 1) butuh logger di luar HTTP — kemungkinan pindah ke `context.Context` standar via `context.WithValue`.
- **`go run` tidak meneruskan sinyal OS ke child process** — ditemukan saat verifikasi manual graceful shutdown. Tidak mempengaruhi kode aplikasi (binary yang di-build langsung menerima sinyal dengan benar), hanya relevan untuk cara menjalankan/menguji secara manual.

### Verifikasi manual (di luar automated test)

Binary di-build dan dijalankan langsung (bukan `go run`) untuk memverifikasi acceptance criteria end-to-end:

```
GET  /health              → 200, envelope {data:{status,version}}, header X-Request-Id (UUIDv7)
GET  /nope                → 404, envelope {error:{code:"not_found",...}}
POST /health               → 405, envelope {error:{code:"method_not_allowed",...}}, header Allow: GET
SIGINT saat serving        → "shutdown signal received" → "shutdown complete", proses keluar bersih
DATABASE_URL kosong         → exit 1, "config invalid: env: required environment variable \"DATABASE_URL\" is not set"
APP_ENV=prod (tidak valid)  → exit 1, "config invalid: APP_ENV must be one of [development production], got \"prod\""
```

Semua sesuai 7 acceptance criteria di `prd.md`.

### `golangci-lint` vs Go 1.25 — temuan CI

`golangci-lint-action@v6` dengan `version: latest` gagal: *"the Go language version (go1.24) used to build golangci-lint is lower than the targeted Go version (1.25.4)"*.

**Akar masalah bukan bug di kode.** `gin@v1.12.0` menarik `quic-go@v0.59.0` (dukungan HTTP/3) sebagai dependensi transitif, dan `quic-go` mensyaratkan Go 1.25 — dikonfirmasi lewat `go mod graph`. `go.mod` yang menargetkan 1.25 itu sah; binary rilis `golangci-lint` yang belum dibangun ulang dengan toolchain 1.25 itu yang tertinggal.

**Perbaikan:** bangun `golangci-lint` dari sumber dengan toolchain kita sendiri, bukan memakai binary pre-built Action:

```yaml
- run: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
- run: golangci-lint run
```

Diverifikasi lokal: `golangci-lint version` → *"built with go1.25.4"*, lalu `golangci-lint run` menemukan **satu** temuan sungguhan — `os.Setenv` yang error-nya tidak diperiksa di `config_test.go` (`errcheck`) — sudah diperbaiki di commit yang sama.

**Tidak** menurunkan `gin` untuk menghindari `quic-go`: itu kebutuhan transitif yang sah di versi ini, bukan sesuatu yang perlu "dihindari".

### Utang teknis

- Tidak ada test end-to-end otomatis untuk graceful shutdown (diverifikasi manual, lihat di atas). Menambahkannya butuh test yang menjalankan binary sungguhan dan mengirim sinyal OS — dipertimbangkan lagi bila perilaku shutdown berubah.

### Catatan untuk session berikutnya

- Titik ekstensi error: tambahkan sentinel baru + case di `MapError` saat domain pertama (Phase 1) butuh kode error baru. Jangan taruh logika pemetaan di handler manapun.
- `httpx.Logger(c)` dan `httpx.RequestIDFromContext(c)` adalah API publik paket ini — dipakai lagi saat `TenantContext` (Phase 1) perlu menyertakan `request_id`.

---

## #2 — Database, Docker Compose, dan migration tooling

### Menyimpang dari TD

| Yang berbeda | Alasan |
|---|---|
| `migrations/embed.go` — paket kecil terpisah hanya untuk `//go:embed *.sql` | TD menulis `migrations/0001_baseline.sql` langsung di bawah `crm_be/`, sejajar dengan `cmd/`. Tapi `//go:embed` tidak bisa menjangkau di luar direktori paket yang mendeklarasikannya — `cmd/migrate` tidak bisa meng-embed berkas di `../../migrations/`. Solusinya: paket `migrations` kecil yang hidup **bersama** berkas SQL-nya (lokasi TD tetap sama), mengekspos `migrations.FS`, lalu `cmd/migrate` mengimpornya. Bukan penyimpangan lokasi, hanya penambahan satu berkas Go yang dituntut aturan bahasa. |
| `docker-compose.yml` tidak memakai `env_file: .env` | Rencana awal mempertimbangkan `env_file`, tapi itu akan membuat `docker compose up` gagal pada clone bersih yang belum punya `.env` — melanggar acceptance criteria "tanpa langkah manual tambahan". Diganti: default development ditulis langsung di blok `environment:` compose, meniru `.env.example`. `.env` lokal (bila ada) tetap bisa override lewat variabel shell / `--env-file` manual. |
| `DB_MAX_CONNS` diberi batas atas (1–1000), tidak hanya "positif" | Ditemukan lewat `gosec` (G115: integer overflow saat `int32(cfg.DBMaxConns)`). Tanpa batas atas, nilai yang sangat besar bisa overflow saat dikonversi ke `int32` untuk `pgxpool.Config.MaxConns`. Diperbaiki di validasi config (fail-fast, pesan jelas) + anotasi `#nosec G115` di titik konversi yang merujuk balik ke validasi ini. |

Tidak ada penyimpangan pada bentuk `db.InTx`, struktur Dockerfile, atau perilaku `/health/ready` — sesuai TD.

### Keputusan implementasi

- **Boot gagal keras bila database tidak terjangkau saat startup** (`db.New` melakukan `Ping` dengan timeout 5 detik; gagal → `os.Exit(1)`). Konsisten dengan kegagalan validasi config — infrastruktur yang tidak lengkap dihentikan saat deploy, bukan menyurfa sebagai request yang gagal satu-satu. `docker-compose`'s `depends_on: condition: service_healthy` mencegah ini terpicu pada `docker compose up` normal.
  **✅ Dikonfirmasi** oleh pemilik produk saat review PR #6, dan digeneralisasi sebagai prinsip startup — bukan hanya untuk PostgreSQL. Dicatat sebagai [ADR-010](../../decisions/ADR-010-fail-fast-startup.md) dan Aturan #36 di `CLAUDE.md`.
- **Setelah boot berhasil, `/health/ready` independen dari state boot** — ia melakukan `Ping` baru di **setiap** request, sehingga benar-benar mencerminkan konektivitas saat itu. Diverifikasi: mematikan Postgres setelah API berjalan membuat `/health/ready` melapor 503 sementara `/health` tetap 200 (tidak menyentuh DB sama sekali), dan `/health/ready` **pulih otomatis ke 200** begitu Postgres hidup lagi — tanpa restart aplikasi. `pgxpool` menangani reconnect secara transparan.
- **`db.InTx` adalah satu-satunya jalan masuk ke transaksi**, sesuai TD — tidak ada cara lain untuk mendapatkan `pgx.Tx` di luar closure ini.

### Verifikasi manual (di luar automated test)

Test otomatis terhadap PostgreSQL asli sengaja diserahkan ke issue #3 (harness testcontainers) — cakupan ini sudah tertulis eksplisit di checklist issue #3 sejak awal. Issue #2 diverifikasi manual, pola yang sama seperti graceful shutdown di issue #1:

```
docker compose up --build     → build sukses, api merespons tanpa langkah manual tambahan
GET  /health                  → 200 (tidak menyentuh DB)
GET  /health/ready (DB hidup) → 200 {data:{status:"ok",database:"reachable"}}

make migrate-up   → goose: successfully migrated database to version: 1
                     psql \df set_updated_at → 1 baris (fungsi ada)
make migrate-down → OK 0001_baseline.sql
                     psql \df set_updated_at → 0 baris (fungsi benar-benar hilang)

docker compose stop postgres
GET /health        → tetap 200
GET /health/ready   → 503 {data:{status:"degraded",database:"unreachable"}}
docker compose start postgres
GET /health/ready   → pulih ke 200 tanpa restart api

db.InTx — diverifikasi lewat program sekali-pakai (dibuat & dihapus dalam
sesi yang sama, tidak ter-commit) terhadap Postgres yang sama:
  commit                    → baris tersimpan
  rollback saat fn error    → baris TIDAK tersimpan, error asli diteruskan
  rollback saat panic       → baris TIDAK tersimpan, panic tetap menjalar
```

Semua sesuai acceptance criteria di `issue #2`.

### Utang teknis

- Belum ada test otomatis untuk `db.InTx` dan migration round-trip terhadap Postgres asli — keduanya diverifikasi manual di atas. Menjadi bagian eksplisit dari cakupan issue #3.
- Tidak ada auto-migrate saat container `api` start. `make migrate-up` dijalankan manual. Dipertimbangkan lagi bila alur deploy butuh migration otomatis (mis. init container terpisah) — jangan digabung diam-diam ke entrypoint `api` karena migration dan serving punya kegagalan yang beda kelas.

### Catatan untuk session berikutnya

- `internal/shared/db.New` dan `db.Ping` adalah API publik paket ini. Repository Phase 1 akan menerima `*pgxpool.Pool` (via DI dari `main.go`), bukan membuat pool sendiri.
- Pola composite FK (freeze lapis 2) belum relevan sampai ada tabel tenant-scoped pertama di `0002_identity` (Phase 1) — `0001_baseline` sengaja tidak menyentuhnya.
- Bila RLS (freeze lapis 3, ditunda) suatu saat diaktifkan, `db.InTx` adalah **satu-satunya** titik untuk menyisipkan `SET LOCAL app.current_org_id` — jangan buat jalur transaksi kedua.

---

## #3 — Test harness terhadap PostgreSQL asli

### Menyimpang dari TD

| Yang berbeda | Alasan |
|---|---|
| `internal/shared/db/dbtest` sebagai **subpaket terpisah**, bukan berkas `testdb.go` di paket `db` | **Kesalahan yang dikoreksi di tengah implementasi, bukan keputusan awal.** Percobaan pertama menaruh helper testcontainers langsung sebagai berkas non-`_test.go` di paket `db` — karena `cmd/api` mengimpor `internal/shared/db`, itu akan menyeret `testcontainers-go` dan seluruh Docker client library ke **binary produksi**. Dipindah ke subpaket `dbtest` yang hanya diimpor oleh berkas `_test.go` di seluruh repo. Diverifikasi: `go list -deps ./cmd/api \| grep -i testcontainers` → kosong. |
| `TestMigrationRoundTrip` membuka koneksi `database/sql` sendiri, bukan lewat `dbtest.NewPool` | `NewPool` men-truncate tabel tapi mengasumsikan schema sudah ter-migrasi (dipakai bersama oleh test lain di container yang sama). Test ini justru perlu menjalankan `down` lalu `up` lagi — kalau memakai pool bersama yang sama, ia akan merusak state migration untuk test lain yang berjalan setelahnya (sequential, tapi berbagi satu container per paket). Koneksi terpisah + restore eksplisit ke `up` di akhir test menjaga container tetap konsisten untuk test berikutnya. |
| `CREATE TABLE IF NOT EXISTS _intx_test` (bukan `CREATE TABLE`) di `db_test.go` | Ditemukan saat run pertama: `dbtest.NewPool` men-truncate tabel yang sudah ada (bukan drop), sehingga test kedua yang mencoba `CREATE TABLE` tanpa `IF NOT EXISTS` gagal dengan "relation already exists". |

### Keputusan implementasi

- **Satu container per paket test** (bukan per test individual) — `dbtest.ConnString` memakai `sync.Once` per-proses. `cmd/api` dan `internal/shared/db` masing-masing punya test binary sendiri, sehingga **masing-masing menjalankan container Postgres-nya sendiri** (tidak saling berbagi lintas paket). Diverifikasi: dua container berbeda muncul di log saat `go test ./...` dijalankan, keduanya start paralel (~12 detik cold start, berjalan bersamaan, bukan berurutan).
- **Migration dijalankan sekali per proses** (`sync.Once` terpisah dari container startup), `NewPool` hanya men-truncate. Karena `0001_baseline` belum punya tabel domain, truncate saat ini efektif no-op — `truncateAll` menangani kasus "tidak ada tabel" dengan aman (query mengembalikan `NULL`, fungsi return lebih awal) sehingga tidak error saat dipanggil pada database yang baru saja di-migrate tanpa tabel domain.
- **Test `/health/ready` yang gagal** memakai pool ke `127.0.0.1:1` (port tertutup, `connect_timeout=1`) — bukan mematikan container asli. Lebih cepat (tidak ada I/O container) dan tidak mengganggu container yang dipakai test lain di paket yang sama.
- **`t.Context()`** (Go 1.24+) dipakai untuk pool yang sengaja gagal connect — otomatis di-cancel saat test selesai, tanpa perlu `context.Background()` + cleanup manual.

### Verifikasi

```
go test -race ./...                    → semua paket PASS
go test -race -count=1 ./... (2×)      → konsisten PASS, tidak ada state leakage antar-run
go list -deps ./cmd/api | grep docker  → kosong (binary produksi bersih dari testcontainers)
go list -deps ./cmd/migrate | grep docker → kosong
```

Tiga hal yang tadinya diverifikasi manual di issue #1 dan #2 sekarang otomatis:

| Sebelumnya manual (issue) | Sekarang otomatis |
|---|---|
| `db.InTx` commit / rollback-on-error / rollback-on-panic (#2) | `TestInTx_CommitsOnSuccess`, `TestInTx_RollsBackOnError`, `TestInTx_RollsBackOnPanic` |
| Migration round-trip up/down (#2) | `TestMigrationRoundTrip` |
| `/health/ready` 200 vs 503 (#2) | `TestHealthReady_ReturnsOK_WhenDatabaseReachable`, `TestHealthReady_Returns503_WhenDatabaseUnreachable` |

Graceful shutdown (issue #1) **tetap manual** — sesuai catatan di bagian #1, `go run` tidak meneruskan sinyal OS ke child process sehingga tidak bisa diuji lewat `go test` tanpa membangun binary sungguhan di dalam test, yang di luar cakupan issue #3.

### Utang teknis

- Tidak ada. Kedua item utang teknis dari issue #2 (test `db.InTx` dan migration round-trip) selesai di sini.

### Catatan untuk session berikutnya

- **Pola `dbtest.NewPool(t)` dipakai ulang di Phase 1+** untuk test repository tenant-scoped. Saat tabel domain pertama (`organizations`, dst.) muncul di `0002_identity`, `truncateAll` otomatis mencakupnya tanpa perubahan — ia men-truncate seluruh tabel `public` kecuali `goose_db_version`.
- **Harness isolasi tenant (freeze lapis 4) belum dibangun di sini** — itu baru bisa ditulis begitu ada tabel `organizations` + endpoint tenant-scoped pertama (Phase 1). `dbtest` menyediakan fondasinya (Postgres asli, migration otomatis), tapi loop generik "untuk setiap route tenant-scoped, tembak dengan kredensial tenant lain, harap 404" adalah pekerjaan Phase 1.
- Bila jumlah paket yang butuh `dbtest` bertambah banyak dan waktu CI mulai terasa (masing-masing paket start container sendiri), pertimbangkan `TestMain` bersama di level lebih tinggi atau `testcontainers-go`'s reuse-by-label. Belum perlu sekarang — total waktu test masih di bawah 15 detik.
