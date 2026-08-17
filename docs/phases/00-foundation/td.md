# Phase 0 — Foundation · Technical Design

> **Bagaimana.** Apa & kenapa di [`prd.md`](./prd.md).
>
> Dokumen ini memuat **delta** untuk Phase 0. Aturan yang sudah ada di [`architecture/freeze.md`](../../architecture/freeze.md) dan konvensi kode di `.claude/skills/jualin-backend/` **tidak diulang** — hanya dirujuk.

---

## 1. Toolchain

| Komponen | Versi | Catatan |
|---|---|---|
| Go | **1.25** | Terpasang di mesin dev: `go1.25.4` |
| PostgreSQL | **17** | Freeze menetapkan 16/17. Tidak bergantung fitur PG 18. |
| Docker Compose | v2+ | Terpasang: `v5.0.1` |
| Module path | `github.com/Pravasta/jualin-crm` | Mengikuti repository |

### Dependensi

| Paket | Untuk | Alasan |
|---|---|---|
| `github.com/gin-gonic/gin` | HTTP router | Ditetapkan freeze |
| `github.com/jackc/pgx/v5` + `pgxpool` | Driver & pool | Bukan ORM (Aturan layering) |
| `github.com/pressly/goose/v3` | Migration | SQL-only, bisa di-embed, punya `down` yang benar |
| `github.com/google/uuid` | UUIDv7 | `uuid.NewV7()` — Aturan #12 |
| `github.com/caarlos0/env/v11` | Parsing env | Deklaratif, mudah divalidasi saat boot |
| `github.com/testcontainers/testcontainers-go` | Test DB | Test berjalan tanpa setup manual |
| `log/slog` | Logging | Stdlib, tanpa dependensi |

> **Tanpa** Redis, message broker, atau ORM. Bila terasa butuh salah satunya, berhenti dan diskusikan (Aturan #27).

---

## 2. Struktur project

> Diperbarui oleh [ADR-009](../../decisions/ADR-009-monorepo-layout.md) — monorepo dengan empat folder aplikasi.
> Modul Go tinggal di `crm_be/`, bukan di akar repository.

```
jualin_crm/                       ← akar repository
├── crm_be/                       ← Go, satu-satunya yang dikerjakan di Phase 0
│   ├── go.mod                    module github.com/Pravasta/jualin-crm/crm_be
│   ├── cmd/
│   │   ├── api/main.go           HTTP server
│   │   └── migrate/main.go       runner migration              (issue #2)
│   ├── internal/shared/
│   │   ├── config/config.go      parsing + validasi env
│   │   ├── logger/logger.go      slog setup
│   │   ├── httpx/
│   │   │   ├── response.go       envelope {data,meta}
│   │   │   ├── error.go          sentinel + mapping ke HTTP
│   │   │   └── middleware.go     request id, logging, recovery
│   │   └── db/                   pgxpool, InTx                 (issue #2)
│   ├── migrations/0001_baseline.sql                            (issue #2)
│   ├── Dockerfile                                              (issue #2)
│   ├── .golangci.yml
│   └── README.md
│
├── crm_dashboard/                Next.js       — Phase 3, baru README
├── crm_landing_page/             Next.js       — belum terjadwal, baru README
├── crm_employee/                 Flutter       — Phase 5, baru README
│
├── docker-compose.yml            orkestrator lintas aplikasi   (issue #2)
├── Makefile                      entry point tunggal
├── .env.example
└── .github/workflows/ci-backend.yml
```

### Aturan penempatan

| Berkas | Lokasi | Alasan |
|---|---|---|
| `Makefile` | **akar** | Satu entry point; target mendelegasikan ke folder aplikasi |
| `docker-compose.yml` | **akar** | Nanti mengorkestrasi lebih dari satu aplikasi |
| `.golangci.yml` | `crm_be/` | Milik satu aplikasi |
| `.env.example` | **akar** | Dibaca `docker compose` |
| CI workflow | `.github/workflows/ci-backend.yml` | Satu berkas per aplikasi, **dengan `paths:` filter** |

Mengikuti konvensi paket di `.claude/skills/jualin-backend/`. **Paket domain (`internal/lead`, dst.) belum dibuat** — Aturan #28.

---

## 3. Config

`internal/shared/config` — dibaca **sekali** saat boot, divalidasi, lalu di-passing sebagai struct. Tidak ada `os.Getenv` yang tersebar.

| Variabel | Tipe | Default | Wajib |
|---|---|---|---|
| `APP_ENV` | `development` \| `production` | `development` | — |
| `HTTP_PORT` | int | `8080` | — |
| `HTTP_READ_TIMEOUT` | duration | `10s` | — |
| `HTTP_WRITE_TIMEOUT` | duration | `10s` | — |
| `HTTP_SHUTDOWN_TIMEOUT` | duration | `15s` | — |
| `DATABASE_URL` | string | — | ✅ |
| `DB_MAX_CONNS` | int | `10` | — |
| `LOG_LEVEL` | `debug`\|`info`\|`warn`\|`error` | `info` | — |

### Kegagalan config

Proses **exit non-zero saat startup**, dengan pesan yang menyebut variabel mana:

```
config invalid: DATABASE_URL is required
config invalid: APP_ENV must be one of [development production], got "prod"
```

**Bukan** panic tanpa konteks, bukan fallback diam-diam ke nilai default.

> `.env` hanya untuk pengembangan lokal dan **tidak pernah** di-commit. `.env.example` di-commit sebagai dokumentasi variabel.

---

## 4. Logging

`log/slog`. Handler dipilih berdasarkan `APP_ENV`:

| Env | Handler |
|---|---|
| `production` | JSON |
| `development` | Text (lebih terbaca) |

### Wajib ada di setiap log yang berasal dari request

```json
{
  "time": "2026-08-17T09:30:00Z",
  "level": "INFO",
  "msg": "request completed",
  "request_id": "01931f2e-8c4a-7...",
  "method": "GET",
  "path": "/health",
  "status": 200,
  "duration_ms": 3
}
```

Logger diambil dari `context`, bukan variabel global, sehingga `request_id` ikut otomatis.

### Redaksi (Aturan #26)

Tidak pernah dicatat: password · raw API key · token · payload lead lengkap.
Ditegakkan di **logger**, bukan di setiap call site.

---

## 5. Request ID

Middleware paling luar:

1. Baca header `X-Request-ID` bila ada — agar bisa dikorelasikan dengan reverse proxy
2. Bila tidak ada, generate **UUIDv7**
3. Simpan di `context`
4. Kembalikan di response header `X-Request-ID`

Nilai ini nanti mengisi `TenantContext.RequestID` (Phase 1) dan `audit_logs.request_id`.

---

## 6. Error

Sesuai [`architecture/api.md`](../../architecture/api.md). Bentuk response:

```json
{ "error": { "code": "not_found", "message": "Resource tidak ditemukan." } }
```

### Pola

Sentinel error di domain, **mapping terpusat** di `httpx`. Handler tidak menentukan status code sendiri.

```go
// internal/shared/httpx/error.go
var (
    ErrNotFound       = errors.New("not found")
    ErrValidation     = errors.New("validation failed")
    ErrInternal       = errors.New("internal error")
)

func MapError(err error) (int, ErrorBody)
```

### Katalog yang aktif di Phase 0

| HTTP | `code` |
|---|---|
| 404 | `not_found` — route tidak dikenal |
| 405 | `method_not_allowed` |
| 500 | `internal_error` |

Kode lain menyusul bersama fiturnya.

### Recovery middleware

Panic → log **beserta stack trace** → response `500 internal_error`.
**Detail internal tidak pernah bocor ke response.**

---

## 7. Health check

| Endpoint | Memeriksa | Dipakai untuk |
|---|---|---|
| `GET /health` | Proses hidup. **Tanpa** menyentuh database. | Liveness probe |
| `GET /health/ready` | Ping database (timeout 2 detik) | Readiness probe |

```json
{ "status": "ok", "version": "dev" }
```

`/health/ready` mengembalikan **503** dengan `{"status":"degraded","database":"unreachable"}` bila DB tidak terjangkau.

### ⚠️ Pengecualian versioning yang disengaja

Aturan #33 mensyaratkan prefix `/v1/` pada seluruh endpoint. **Health check dikecualikan** — ia infrastruktur, bukan API produk, dan dikonsumsi oleh orchestrator yang tidak mengenal versi produk.

> Ditulis eksplisit agar session mendatang tidak "memperbaikinya" menjadi `/v1/health`.

---

## 8. Database

`internal/shared/db` — `pgxpool` dengan konfigurasi dari config, plus transaction manager:

```go
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error
```

`InTx` menangani begin / commit / rollback-on-error / rollback-on-panic.

> **Ini satu-satunya jalan masuk ke transaksi.** Aturan #32 (efek samping di luar transaksi) dan Lapis 3 multi-tenancy (penyisipan `SET LOCAL` bila RLS diaktifkan nanti) sama-sama bergantung pada adanya **satu** titik ini.

---

## 9. Migration

### Tooling

`goose` dengan migration **SQL-only**, di-`embed` ke dalam binary `cmd/migrate` sehingga tidak ada berkas yang perlu ikut di-deploy terpisah.

```
migrate up        jalankan semua yang tertunda
migrate down      turunkan satu langkah
migrate status    tampilkan status
```

### `0001_baseline`

**Isi:** hanya fungsi trigger `set_updated_at()`.

**Tidak berisi:** tabel domain, extension, seed data.

```sql
-- +goose Up
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- +goose Down
DROP FUNCTION IF EXISTS set_updated_at();
```

> Migration ini **kecil dengan sengaja**. Tugasnya membuktikan `up` dan `down` bekerja bersih; sebuah fungsi sudah cukup untuk itu. Tabel domain dimulai di `0002` (Phase 1), sesuai freeze bagian 8.

### Konvensi

Sesuai skill: penomoran berurutan dan **tidak pernah diubah** setelah merge · setiap migration punya `down` yang benar-benar bekerja · nama constraint eksplisit · alasan JSONB ditulis sebagai komentar SQL.

---

## 10. Docker

### `Dockerfile`

Multi-stage: builder Go → runtime **distroless/alpine non-root**. Binary statis, tanpa toolchain di image akhir.

### `docker-compose.yml`

| Service | Isi |
|---|---|
| `postgres` | `postgres:17-alpine`, named volume, healthcheck `pg_isready` |
| `api` | build dari Dockerfile, `depends_on: postgres (service_healthy)`, port dari config |

`docker compose up` harus cukup — tanpa langkah manual tambahan.

---

## 11. Test harness

**Repository diuji terhadap PostgreSQL asli, bukan mock** (skill: *"mock database tidak akan pernah menangkap bug isolasi tenant"*).

### Mekanisme

`testcontainers-go` + modul postgres. Container di-start **sekali per paket test** (`TestMain`), migration dijalankan otomatis, lalu helper menyediakan koneksi bersih:

```go
func NewTestDB(t *testing.T) *pgxpool.Pool
```

Setiap test mendapat state bersih — **truncate**, bukan re-migrate, agar cepat.

### Yang diuji di Phase 0

| Test | Memastikan |
|---|---|
| Migration round-trip | `up` lalu `down` bersih, tanpa objek tersisa |
| Fungsi `set_updated_at()` | Trigger benar-benar memperbarui `updated_at` |
| Config | Env invalid → error yang menyebut variabelnya |
| Error mapping | Setiap sentinel error → status & `code` yang benar |
| Health | `/health` 200 · `/health/ready` 503 saat DB mati |

> Harness **isolasi tenant** adalah Phase 1 — belum ada tabel tenant-scoped untuk diuji.

---

## 12. CI

`.github/workflows/ci.yml` — jalan pada PR ke `main` dan push ke `main`.

| Job | Isi |
|---|---|
| `lint` | `golangci-lint run` |
| `test` | `go test -race ./...` (runner GitHub sudah menyediakan Docker untuk testcontainers) |
| `build` | `go build ./...` |

Ketiganya harus hijau sebelum PR bisa di-merge.

### ⚠️ `paths:` filter wajib sejak workflow pertama

Karena monorepo (ADR-009), workflow backend **hanya** berjalan bila ada perubahan yang relevan:

```yaml
on:
  pull_request:
    paths: ['crm_be/**', '.github/workflows/ci-backend.yml']
  push:
    branches: [main]
    paths: ['crm_be/**', '.github/workflows/ci-backend.yml']
```

Semua job memakai `defaults.run.working-directory: crm_be`.

> Menambahkan filter ini belakangan berarti sudah membakar waktu CI berbulan-bulan — setiap perubahan dokumentasi menjalankan seluruh test backend.

### `.golangci.yml`

Linter awal: `govet` · `errcheck` · `staticcheck` · `ineffassign` · `misspell` · `gosec` · `bodyclose` · `sqlclosecheck`

Ditambah seperlunya. Linter yang menghasilkan banyak temuan tanpa nilai akan dimatikan **dengan alasan tertulis** di berkas config.

---

## 13. Makefile

| Target | Isi |
|---|---|
| `make dev` | `docker compose up --build` |
| `make down` | `docker compose down` |
| `make test` | `go test -race ./...` |
| `make lint` | `golangci-lint run` |
| `make migrate-up` / `migrate-down` / `migrate-status` | Jalankan `cmd/migrate` |
| `make tidy` | `go mod tidy` |

---

## 14. Graceful shutdown

`cmd/api` menangkap `SIGINT`/`SIGTERM` → berhenti menerima koneksi baru → menunggu request berjalan selesai sampai `HTTP_SHUTDOWN_TIMEOUT` → menutup pool database.

Murah dilakukan sekarang, menyakitkan ditambal setelah ada trafik.

---

## 15. Risiko teknis

| Risiko | Mitigasi |
|---|---|
| **testcontainers lambat** — startup container per paket menambah waktu CI | Container di-share per paket via `TestMain`; reset antar-test memakai truncate, bukan re-migrate. Bila terbukti mengganggu, pindah ke `services:` PostgreSQL di GitHub Actions — perubahan lokal di satu helper. |
| **Over-scaffolding** — godaan menyiapkan `tenant`, `auth`, atau paket domain "sekalian" | Daftar di luar cakupan PRD bersifat mengikat. Yang tidak dipakai kode Phase 0 tidak dibuat. |
| **Abstraksi logger/error terlalu dini** | Bentuk paling sederhana yang memenuhi 7 acceptance criteria. Abstraksi menyusul saat ada pemakai kedua yang nyata (Aturan #27). |
| **Docker di CI** | Runner GitHub `ubuntu-latest` sudah menyediakan Docker. Tidak perlu setup tambahan. |

---

## 16. Yang berubah pada dokumentasi

Setelah Phase 0 selesai:

| Berkas | Perubahan |
|---|---|
| `docs/architecture/api.md` | Katalog error diperbarui bila ada kode baru |
| `docs/architecture/overview.md` | **Dibuat** — struktur project & alur boot yang benar-benar terwujud |
| `docs/STATUS.md` | Phase 0 ditandai selesai, utang teknis dicatat |
| `docs/phases/00-foundation/notes.md` | Satu bagian per issue |
