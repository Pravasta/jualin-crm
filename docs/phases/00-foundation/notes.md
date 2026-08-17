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

### Utang teknis

- Tidak ada test end-to-end otomatis untuk graceful shutdown (diverifikasi manual di atas). Menambahkannya butuh test yang menjalankan binary sungguhan dan mengirim sinyal OS — dipertimbangkan lagi bila perilaku shutdown berubah.
- `golangci-lint` belum dijalankan lokal (tidak terpasang di mesin dev). Diverifikasi lewat CI saat PR dibuka.

### Catatan untuk session berikutnya

- Titik ekstensi error: tambahkan sentinel baru + case di `MapError` saat domain pertama (Phase 1) butuh kode error baru. Jangan taruh logika pemetaan di handler manapun.
- `httpx.Logger(c)` dan `httpx.RequestIDFromContext(c)` adalah API publik paket ini — dipakai lagi saat `TenantContext` (Phase 1) perlu menyertakan `request_id`.
