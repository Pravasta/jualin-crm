# Phase 3 — Owner Dashboard · Notes

One section per issue, appended as each is implemented.

---

## #30 — CORS + endpoint metrik

### Keputusan implementasi

- **`internal/metrics` benar-benar tidak punya `Store`/`InTx`**, sesuai TD §2 — hanya `entity.go` +
  `port.go` (interface `Repository`, tanpa `Store`) + `usecase.go` (menerima `Repository` langsung,
  bukan `Store`) + `repository_postgres.go` + `handler_http.go`. Tidak ada `cmd/api/metrics_store.go`:
  composition root memanggil `metrics.New(pool)` langsung karena `*pgxpool.Pool` sudah memenuhi
  `db.Querier`, sama seperti `customer.New(pool)` dipakai di test — tidak ada wrapper yang perlu ditulis
  untuk paket yang tidak pernah membuka transaksi.

- **Query dibangun dengan closure `arg()` yang sama seperti `internal/lead`/`internal/customer`**
  (`WHERE ... created_at >= $2`), bukan pola `$2::timestamptz IS NULL OR ...` yang sempat ada di draf
  TD §1.1's contoh kode — mengikuti konvensi yang sudah dipakai `lead.FindAllByOrg` untuk
  `created_from`/`created_to`, bukan menciptakan pola baru untuk hal yang sama.

- **`avg_response_seconds` memakai `avg()` Postgres apa adanya**, tanpa `FILTER` atau `COALESCE` untuk
  menangani lead yang belum tersentuh — `avg()` sudah mengabaikan `NULL` secara native, dan
  `MIN(activities.created_at WHERE type <> 'lead_created') - leads.created_at` menghasilkan `NULL`
  persis saat lead itu belum pernah disentuh activity non-`lead_created`. Ini yang membuat "dikecualikan
  dari rata-rata, bukan dihitung nol" (TD §2.3) benar tanpa cabang logic tambahan di Go maupun SQL.
  Dibuktikan lewat `TestRepository_Employees_AvgResponseSeconds_ExcludesUntouchedLeadsAndLeadCreated`,
  yang menguji **dua** klaim sekaligus: `lead_created` tidak ikut terhitung (lead yang sama juga punya
  activity `lead_created` di waktu pembuatan — kalau ini ikut terhitung, hasilnya akan ~0 detik, bukan
  ~10 menit), dan lead kedua yang sama sekali tidak tersentuh tidak menarik rata-rata turun ke arah nol.

- **`Employees` mengagregasi seluruh membership aktif di organization, bukan hanya `role = 'employee'`.**
  Assignment lead (`leads.assigned_to_membership_id`) tidak dibatasi role — Owner/Admin/Manager juga bisa
  jadi assignee. Field responsnya tetap `membership_id`/`full_name` (generik), bukan menyaring ke satu
  role tertentu sebelum agregasi.

- **Harness isolasi tenant (`cmd/api/tenant_isolation_test.go`) mendapat test terpisah, bukan entri di
  slice `isolationCase` yang ada.** Kasus itu berbentuk "resource by-id milik org lain → 404"; endpoint
  metrik tidak punya `:id` sama sekali — kebocorannya berbentuk agregat (angka yang salah), bukan baris
  yang salah. `TestTenantIsolation_MetricsAggregate_ScopedToOrganization` ditulis sebagai test terpisah
  di file yang sama, dan **benar-benar dibuktikan bisa gagal**: mengubah predikat
  `organization_id = $1` di `metrics.postgresRepository.Summary` sementara jadi
  `(organization_id = $1 OR true)` membuat test merah — `total_new` organisasi A terbaca 3, bukan 1,
  ikut menghitung dua lead milik organisasi B. Perubahan itu tidak pernah di-commit; lihat komentar di
  test itu sendiri untuk detail.

### Menyimpang dari rencana issue

Tidak ada penyimpangan dari checklist issue #30 atau TD §1–§2 — implementasi mengikuti keduanya persis.

### Verifikasi

```
go build ./...                    → bersih
go vet ./...                      → bersih
gofmt -l .                        → bersih
go test ./... (real Postgres via testcontainers, semua paket)  → semua PASS, termasuk 33 test lama
                                     tanpa perubahan asersi

go test ./internal/shared/config/...    → 11/11 PASS (termasuk 3 test CORS_ALLOWED_ORIGINS baru)
go test ./internal/shared/httpx/... -run TestCORS  → 5/5 PASS
go test ./internal/shared/authz/...     → PASS (matrix bertambah metrics.read × 4 role)
go test ./internal/metrics/...          → 18/18 PASS
  TestUnit_*        (6)  — fake Repository, tanpa Docker
  TestRepository_*  (7)  — Postgres asli: by_status/unassigned/range filter, conversion_rate
                            (denominator excludes spam/unqualified, nil saat 0), avg_response_seconds
                            (excludes lead_created & untouched lead), scoping per organization (×2)
  TestHandler_*     (5)  — HTTP end-to-end: 200 Owner, 403 Employee (×2 endpoint), 401 tanpa kredensial

go test ./cmd/api/... -run TestTenantIsolation  → PASS, termasuk
  TestTenantIsolation_MetricsAggregate_ScopedToOrganization (dibuktikan bisa gagal, lihat di atas)
```

Seluruh acceptance criteria issue #30 terpenuhi.

### Utang teknis

- Tidak ada item baru dari issue ini.

### Catatan untuk session berikutnya

- **#31 dan seterusnya (`crm_dashboard`) sekarang bisa mulai** — CORS dan kedua endpoint metrik yang
  memblokirnya sudah ada. `CORS_ALLOWED_ORIGINS=http://localhost:3000` sudah ditambahkan ke
  `.env.example` dan `docker-compose.yml`; dashboard lokal di port 3000 akan langsung bisa memanggil API
  ini tanpa konfigurasi tambahan.
- `GET /v1/metrics/employees` **tidak dipaginasi** — TD §2.1 tidak menyebutnya, dan jumlah membership per
  organization di volume MVP kecil. Bila ini berubah, itu percakapan produk baru, bukan diam-diam
  ditambah paginasi.

---

## #31 — Setup Next.js, klien API, sesi, auth UI

### Keputusan tooling (tidak ditentukan TD, dikonfirmasi pemilik produk sebelum implementasi)

- **Test runner: Vitest**, bukan Jest — acceptance criterion utama issue ini ("N request paralel yang
  401 menghasilkan TEPAT SATU panggilan refresh") harus dibuktikan di bawah konkurensi asli, dan Vitest
  jalan native terhadap App Router modern tanpa transform config tambahan.
- **Form handling: state polos (`useState`) + validasi manual**, tanpa react-hook-form/zod — form
  auth hanya 2-4 field per layar; dua dependency runtime baru untuk itu tidak sepadan (CLAUDE.md:
  "bila solusi sederhana sudah cukup, pakai itu").

### Keputusan implementasi

- **Refresh single-flight** (`src/lib/api-client.ts`) — satu variabel modul-level
  `refreshPromise: Promise<boolean> | null`. `doRefresh()` melakukan `refreshPromise ??= rawFetch(...)`
  **sebelum** `await` apa pun, sehingga penetapannya sinkron: pemanggil kedua yang tiba di baris yang
  sama (bahkan hanya berbeda satu microtask) pasti melihat `refreshPromise` sudah terisi dan memakai
  ulang objek Promise yang **sama**, bukan memanggil `doRefresh()` lagi. `.finally()` mereset variabel
  ke `null` setelah settle — supaya siklus 401 **berikutnya** (bukan yang sedang menunggu) memicu
  refresh baru, bukan hasil basi. Dibuktikan lewat test yang benar-benar konkuren (lihat di bawah),
  bukan diasumsikan dari membaca kode.
- **`(auth)` vs `(protected)` sebagai dua route group terpisah** (App Router), bukan satu layout dengan
  flag kondisional — layar auth tidak pernah memanggil `GET /v1/me` (belum tentu ada sesi), layar
  protected selalu memanggilnya di layout-nya (`SessionGate`, `src/lib/session-context.tsx`). Ini
  membuat "proteksi route lewat `GET /v1/me` di layout terproteksi" (checklist issue) literal.
- **Tidak ada `middleware.ts`** yang membaca token — TD §4.1 eksplisit menyatakan itu tidak mungkin
  (`access_token` `HttpOnly`). Satu-satunya penjaga adalah `SessionGate`.
- **Layar pilih organization bukan route terpisah** — `409 organization_selection_required` (ADR-007)
  ditangani inline di `(auth)/login/page.tsx`: form yang sama berganti mode jadi picker organization,
  email/password tetap di state komponen (tidak pernah disimpan di localStorage), lalu `login` dipanggil
  ulang dengan `organization_id` terpilih. Menghindari layar terpisah yang butuh cara membawa
  email/password melewati navigasi.
- **"Cek email" (register) dan "cek email" (lupa password) juga inline**, bukan route terpisah — state
  swap di komponen yang sama, konsisten dengan alasan yang sama seperti pilih organization: tidak ada
  informasi yang perlu selamat melewati navigasi URL.
- **Kontrak backend diverifikasi langsung dari kode**, bukan ditebak dari TD — `src/lib/auth.ts` dan
  `src/lib/api-types.ts` dibangun dari pembacaan literal
  `crm_be/internal/auth/{handler_http,entity}.go` dan `internal/shared/httpx/csrf.go` (nama field body,
  `ClientDashboard = "dashboard"`, `CSRFCookieName`/`CSRFHeaderName`, `minPasswordLength = 12`) —
  lihat verifikasi end-to-end di bawah, bukan hanya pembacaan kode.
- **`create-next-app` menulis `AGENTS.md` + `crm_dashboard/CLAUDE.md`** (yang isinya `@AGENTS.md`) —
  **dipertahankan**, bukan dihapus. Berkas ini milik tooling Next.js sendiri (di-generate ulang oleh
  `next dev`, lihat komentar di dalamnya), bukan scope creep dari sesi ini.
- **`components.json` (shadcn) memakai `registries: {}` kosong** — tidak ada private component registry
  di project ini; menambahkannya sekarang akan melanggar anti-overengineering (CLAUDE.md).

### Menyimpang dari rencana

- `LayoutProps<"/">` (tipe generated Next.js untuk root layout) **tidak dipakai** — bergantung pada
  `.next/types` yang belum ada sebelum build/dev pertama kali jalan, membuat `npm run typecheck`
  gagal di checkout bersih sebelum `npm run build` sempat jalan (urutan job CI). Diganti
  `{ children: React.ReactNode }` biasa, sama seperti layout `(auth)`/`(protected)`.
- `vitest.config.ts` → **`vitest.config.mts`** — `package.json` tidak punya `"type": "module"` (Next.js
  App Router tidak butuh itu), dan Vite/Vitest memperingatkan soal ESM-syntax-di-file-CommonJS untuk
  ekstensi `.ts` biasa. Ekstensi `.mts` eksplisit menghilangkan peringatan tanpa mengubah konfigurasi
  project secara global.

### Verifikasi

Build/typecheck/lint/test:
```
npm run typecheck   → bersih
npm run lint        → bersih (0 error, 0 warning)
npm run test         → 6/6 PASS, diulang 5× berturut-turut tanpa flaky
npm run build        → sukses, 6 route auth + / ter-generate sebagai static
```

Test `src/lib/api-client.test.ts` (6 test) — yang dianggap "layak direview" per issue body:
- **Konkurensi**: 6 panggilan `apiFetch` paralel yang sama-sama dapat `401` menghasilkan **tepat 1**
  panggilan ke `/v1/auth/refresh` (dibuktikan lewat refresh yang sengaja ditahan/`deferred()` sampai
  seluruh 6 panggilan pertama mencapai titik single-flight-nya — bukan diuji berurutan)
- Request yang diulang dan tetap dapat `401` kedua kalinya **tidak** memicu refresh kedua
- `401` dari `/v1/auth/refresh` sendiri **tidak** memicu refresh lain, langsung redirect ke `/login`
- `X-CSRF-Token` terpasang di `POST`, **tidak ada** di `GET`
- `ApiError` membawa `code`/`status`/`details` dari body error asli

Verifikasi end-to-end **terhadap `crm_be` sungguhan** (`docker compose up` + migration + `npm run dev`),
lewat `curl` (bukan browser interaktif — tidak ada tool browser automation tersedia di sesi ini, dicatat
apa adanya, bukan diklaim sebagai "diuji di browser"):
```
POST /v1/auth/register              → 201 {user_id, organization_id}     — cocok persis dgn auth.ts
(baca token dari log LogMailer)     → docker compose logs api
POST /v1/auth/verify-email          → 200 {status:"verified"}
POST /v1/auth/login (client=dashboard) → 200 {status:"ok"} + Set-Cookie:
    access_token   HttpOnly; SameSite=Lax
    refresh_token  HttpOnly; SameSite=Lax
    csrf_token     SameSite=Lax (TANPA HttpOnly — sesuai csrf.ts)
GET  /v1/me (cookie)                → 200, field persis cocok MeResponse
POST /v1/auth/logout TANPA X-CSRF-Token → 403 csrf_token_invalid (membuktikan CSRF benar-benar aktif)
POST /v1/auth/logout DENGAN X-CSRF-Token → 204
OPTIONS preflight, Origin: http://localhost:3000 → 204 + header CORS lengkap
GET /, /login, /register, /forgot-password, /reset-password, /verify-email → semua 200
```
Seluruh acceptance criteria issue #31 terpenuhi. Port 3000 kebetulan dipakai proses lain yang tidak
terkait di mesin pengembangan sesi ini — dev server otomatis pindah ke 3001; tidak memengaruhi
verifikasi karena `curl` tidak menegakkan CORS (itu mekanisme browser).

### Utang teknis

- Tidak ada item baru dari issue ini.

### Catatan untuk session berikutnya

- **#32 (daftar lead) sekarang bisa mulai** — sesi, klien API, dan proteksi route sudah ada.
- `apiFetch<T>` saat ini mengembalikan `data` saja, membuang `meta` — cukup untuk issue ini (tidak ada
  endpoint list yang dipanggil). #32 butuh varian yang mengembalikan `{data, meta}` untuk pagination
  `GET /v1/leads` — tambahkan fungsi baru (mis. `apiFetchList`), jangan ubah signature `apiFetch` yang
  ada karena akan memaksa setiap pemanggil non-list menangani `meta` yang tidak mereka butuhkan.
- `auth-errors.ts` sengaja **belum** menangani `version_conflict`/`membership_has_open_leads` — tidak
  ada layar yang membutuhkannya di issue ini. #33/#34 menambah handler-nya saat layarnya sungguhan ada.
