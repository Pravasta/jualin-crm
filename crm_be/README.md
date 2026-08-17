# crm_be

Backend Jualin CRM. **Go + Gin + PostgreSQL, monolith.**

Module: `github.com/Pravasta/jualin-crm/crm_be`

Ini pusat seluruh business logic dan tenant isolation. Dashboard, mobile app, dan sistem eksternal semuanya berbicara ke sini.

## Menjalankan

Semua perintah dijalankan dari **root repository**, bukan dari folder ini:

```bash
make dev            # docker compose up --build
make test           # go test -race ./...
make lint           # golangci-lint run
make migrate-up     # jalankan migration
```

## Struktur

```
cmd/
  api/              HTTP server
  migrate/          runner migration          (issue #2)

internal/
  <domain>/         satu paket per domain     (Phase 1+)
    handler.go      HTTP
    service.go      business logic, otorisasi, transaksi
    repository.go   akses database — SELALU tenant-scoped
    model.go

  shared/
    config/         parsing + validasi env
    logger/         slog
    httpx/          envelope, error mapping, middleware
    db/             pgxpool, InTx              (issue #2)
    tenant/         TenantContext              (Phase 1)

migrations/         SQL, dijalankan goose      (issue #2)
```

Paket domain dibuat **hanya saat fiturnya dikerjakan** — Aturan #28.

## Sebelum menulis kode

Baca berurutan:

1. [`CLAUDE.md`](../CLAUDE.md) — aturan global
2. [`docs/STATUS.md`](../docs/STATUS.md) — sudah sampai mana
3. [`.claude/skills/jualin-backend/`](../.claude/skills/jualin-backend/) — konvensi kode
4. [`docs/architecture/multi-tenancy.md`](../docs/architecture/multi-tenancy.md) — hampir selalu relevan
5. [`docs/phases/<NN>-<slug>/td.md`](../docs/phases/) — desain teknis phase berjalan

## Aturan yang paling sering dilanggar

- Repository **tidak punya** method tanpa `TenantContext` sebagai parameter pertama
- **Tidak ada** `organization_id` di DTO manapun — ia selalu dari kredensial
- FK ke tabel tenant-scoped **selalu composite**: `(x_id, organization_id)`
- Efek samping eksternal (email, HTTP) **tidak pernah** di dalam transaksi database
- Resource tenant lain → **404**, bukan 403
