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

---

## #40 — Fondasi desain: token warna, label Indonesia, app shell

Issue ini **tidak ada di rencana awal Phase 3**. Ia muncul setelah hasil Claude Design masuk: desainnya
mencakup ~15 layar sekaligus (seluruh #32–#35), sementara token warna, peta label, dan kerangka aplikasi
dipakai bersama oleh semuanya dan tidak dimiliki satu pun dari mereka. Design brief §7.6 memang sudah
menandai kerangka aplikasi sebagai *"belum ada dan dibutuhkan"*. Dipisah supaya empat issue berikutnya
tinggal mengisi layar, bukan masing-masing ikut memindahkan fondasi.

Sumber desain: project `5ac090ad-4737-47cf-8fb1-7866eb0d865c`, file `Jualin CRM Dashboard.dc.html`
(dibaca lewat `DesignSync`). `github.md` di project itu mencatat ia disinkronkan dari commit `7b88273`
— merge design brief kita, jadi desainnya memang dibangun di atas brief itu.

### Aksen desain gagal WCAG AA — diperbaiki, bukan disalin apa adanya

Design brief §12 meminta *"pastikan kontrasnya lolos WCAG AA terhadap `--primary-foreground`"*.
Diperiksa dengan menghitung langsung (oklch → sRGB → luminansi relatif → rasio kontras), bukan
dikira-kira:

| Warna | Dipakai untuk | Rasio | |
|---|---|---|---|
| `oklch(0.58 0.19 41)` — usulan desain | latar tombol, teks putih 14px | **4.45:1** | ⛔ di bawah 4.5:1 |
| `oklch(0.56 0.19 41)` — dipakai | sama | **4.83:1** | ✅ |

Selisihnya tidak terlihat mata (`#D14400` → `#CA3C00`), tetapi 4.45 vs 4.50 adalah beda antara lolos
dan tidak. Desain memakai warna itu sebagai **latar tombol dengan teks putih 14px** (terverifikasi dari
markup-nya, bukan diasumsikan) — persis kasus "teks normal" yang ambangnya 4.5:1.

**Lima dari delapan badge status juga gagal**, terburuk `proposal` di **3.14:1**. Diperbaiki dengan
menurunkan *lightness* saja — hue dan chroma desain dipertahankan persis, dan latar badge tetap
dicampur dari nilai desain asli sehingga badge-nya terlihat sebagaimana digambar:

```
new 4.05→4.58 · contacted 4.24→4.53 · qualified 4.33→4.51
proposal 3.14→4.55 (0.62→0.53, satu-satunya perubahan yang terlihat) · lost 4.37→4.56
won 4.54 · unqualified 4.81 · spam 6.62   (ketiganya sudah lolos, tidak disentuh)
```

### Keputusan implementasi

- **`--primary` diganti amber, `--accent-strong` token baru.** Keduanya perlu ada karena tugasnya
  berlawanan: `--primary` (0.56) disetel untuk **putih di atas warna**, `--accent-strong` (0.48) untuk
  **warna di atas putih** (nav aktif, tautan — 7.04:1). Memakai satu nilai untuk keduanya berarti salah
  satunya gagal kontras.
- **Warna status tinggal di `labels.ts`, bukan jadi token CSS.** Masing-masing milik satu nilai enum,
  bukan milik tema; dan Tailwind tidak bisa membuat class dari nilai runtime, jadi ia jadi inline style
  apa pun caranya. Menaruhnya sebagai 8 variabel CSS hanya menambah lapisan tanpa menambah kemampuan.
- **Label nav diterjemahkan.** Desain mengusulkan "Home"/"Task"/"Settings" — tiga kata Inggris yang
  punya padanan wajar, sementara acceptance criterion #12 mewajibkan seluruh antarmuka Bahasa
  Indonesia. Dipakai **Beranda/Tugas/Pengaturan**; "Lead" dan "Customer" tetap karena keduanya istilah
  yang `glossary.md` kunci sebagai kosakata produk. Dikunci test, bukan hanya konvensi.
- **Logika nav dipisah ke `lib/nav.ts`.** `isActive`/`pageTitle` punya jebakan nyata: prefix-match naif
  membuat `/` aktif di setiap halaman, dan urutan pencarian yang salah membuat setiap halaman berjudul
  "Beranda". Dipisah dari komponen supaya bisa diuji tanpa merender React.
- **Lima halaman placeholder** (`/leads`, `/customers`, `/tasks`, `/team`, `/settings`). Shell
  mengirim enam tujuan nav sekaligus; tanpa ini lima di antaranya `404` selama beberapa siklus PR.
  Masing-masing menyebut issue yang akan menggantinya, dan dihapus utuh oleh issue itu.
- **Lonceng notifikasi sengaja belum ada** di topbar — ia milik #34 bersama layar yang memakai
  `/v1/notifications`. Membuat lonceng yang tidak berfungsi lebih buruk daripada tidak ada.
- **`.dark` tidak disentuh.** Dark mode di luar cakupan Phase 3; palet gelap yang belum pernah diperiksa
  di layar sungguhan adalah tebakan, bukan desain. `--accent-strong`/`--accent-tint` tetap mengalir dari
  `:root` bila suatu saat dinyalakan.

### Bug yang diperbaiki di sini

`--font-sans: var(--font-sans)` — menunjuk dirinya sendiri, sementara `layout.tsx` mendefinisikan
`--font-geist-sans`. Akibatnya Geist Sans **tidak pernah benar-benar diterapkan**; seluruh dashboard
jatuh ke font bawaan browser sejak #31. Ditemukan saat memverifikasi klaim "font sudah terpasang" untuk
design brief. Diperbaiki di sini karena `globals.css` memang sedang disentuh — dibuktikan dari CSS hasil
build: `html{font-family:var(--font-geist-sans)}`.

### Verifikasi

```
npm run typecheck · lint · build   → bersih; 12 route ter-generate
npm run test                        → 15/15 PASS (6 lama + 9 baru di nav.test.ts)

CSS hasil build (bukan hanya sumbernya):
  .bg-accent-tint{background-color:var(--accent-tint)}    ← class benar-benar ter-generate
  .text-accent-strong{color:var(--accent-strong)}
  --primary:#c54300                                        ← amber, bukan netral lama
  html{font-family:var(--font-geist-sans)}                 ← bug font terperbaiki

Terhadap crm_be sungguhan (docker compose + migrate):
  seluruh 7 route → 200
  POST /v1/auth/login → 200
  GET  /v1/me → organization_name "Toko Budi", full_name "Budi Santoso", role "owner"
       — persis tiga field yang dirender shell (sidebar, inisial "BS", label "Owner")
```

**Batas verifikasi, dicatat apa adanya:** shell dirender di sisi klien di balik `SessionGate`, jadi
`curl` hanya membuktikan route-nya hidup dan datanya benar — **bukan** bahwa tata letaknya terlihat
seperti desain. Tidak ada tool otomasi browser di sesi ini. Yang bisa dibuktikan otomatis sudah
dibuktikan (token ter-generate, logika nav diuji, data API cocok); penilaian visual perlu dibuka di
browser.

### Utang teknis

- Lima halaman placeholder adalah utang yang **disengaja dan bertanggal** — masing-masing menyebut issue
  penggantinya. Bila #32–#35 selesai dan masih ada `placeholder-screen.tsx`, berarti ada yang terlewat.

### Catatan untuk session berikutnya

- **#32 tinggal mengisi `/leads`** — shell, token, dan `STATUS_META`/`SOURCE_LABELS` sudah siap pakai.
- `NavItemConfig.badge` sudah ada tetapi **belum ada yang mengisinya**. #32 menyambungkannya ke jumlah
  lead tanpa pemilik aktif (`assigned_to=none`) — desainnya menaruh angka itu di item nav "Lead".
- Sisa desain (#33–#35) ada di project `5ac090ad`, dibaca lewat `DesignSync` — `get_file` pada
  `Jualin CRM Dashboard.dc.html`. Berkasnya 111KB; petakan dulu dengan mencari penanda
  `<!-- ===== NAMA ===== -->` daripada memuat seluruhnya ke context.

---

## #32 — Daftar lead: filter, pencarian, pagination

Layar dibangun dari section `<!-- ===== LEADS LIST ===== -->` project `5ac090ad` (dibaca ulang, bukan
dari cache lama, untuk memastikan tidak ada perubahan yang terlewat sejak #40). Desain memberi struktur
visual (input pencarian, chip filter, tabel, empty state, modal "Lead baru") tetapi **tidak** menunjukkan
filter periode atau kontrol pagination — keduanya wajib menurut TD §7.1/issue tapi tidak ada di mockup
(prototipe render semua data sekaligus dari array in-memory, tidak benar-benar berpaginasi). Ditambahkan
mengikuti bahasa visual yang sama (tinggi 34px, radius 8px/999px, warna token), dicatat sebagai perluasan
sadar dari mockup — bukan penyimpangan diam-diam.

### Bug nyata ditemukan saat verifikasi terhadap `crm_be` sungguhan

`GET /v1/metrics/summary`'s `by_status` adalah **Go map** — status dengan nol lead **dihilangkan
sepenuhnya** dari JSON, bukan dikirim sebagai `0`. Dibuktikan langsung: org dengan 5 lead di dua status
mengembalikan `{"by_status":{"contacted":1,"new":4}}` — enam status lainnya sama sekali tidak ada di
body. Kode awal menulis `summary?.by_status[status] ?? "…"` — untuk status manapun yang belum pernah
dipakai satu organization pun, ini akan menampilkan "…" **selamanya**, tidak bisa dibedakan dari keadaan
sedang memuat.

Diperbaiki dengan memisahkan dua pertanyaan yang berbeda: *"apakah summary sudah dimuat?"* (baru boleh
menampilkan "…") vs *"apakah status ini ADA di summary yang sudah dimuat?"* (key hilang = `0`, bukan
"belum tahu"). Fungsi murni `statusCount(summary, status)` di `lib/metrics.ts`, dikunci test
(`metrics.test.ts`) yang secara eksplisit membangun ulang response asli itu sebagai fixture.

### Keputusan implementasi

- **URL adalah sumber kebenaran filter** — dibaca dari `useSearchParams`, ditulis lewat
  `router.replace(..., {scroll:false})`. Setiap perubahan filter mereset `page` ke 1: bertahan di
  halaman 4 dari filter yang sekarang hanya punya 2 hasil menampilkan halaman kosong yang terlihat
  seperti bug.
- **Kata kunci pencarian di-debounce 300ms** sebelum ditulis ke URL — tanpa ini setiap ketikan memicu
  request. State lokal (`keywordInput`) terpisah dari nilai URL (`urlKeyword`) supaya tidak ada sinkronisasi
  dua arah yang bisa loop; satu-satunya tempat URL menimpa state lokal adalah `handleClearFilters`.
- **`AbortController` di setiap fetch**, dibatalkan saat filter berganti sebelum respons sebelumnya
  selesai — mencegah respons lambat untuk filter yang sudah ditinggalkan menimpa respons yang lebih baru
  untuk filter saat ini. Berlaku untuk daftar lead, memberships, dan metrics summary.
- **`loading` adalah derived state, bukan `setState` langsung di body effect.** ESLint (rule
  `react-hooks/set-state-in-effect`, baru di React 19 toolchain) menolak `setLoading(true)` sinkron di
  awal effect. Diselesaikan dengan membandingkan `requestKey` (serialisasi seluruh filter+page) terhadap
  `loadedKey` (di-set hanya di dalam `.then`/`.catch`, yang diizinkan karena berjalan setelah jeda async
  sungguhan) — `loading = loadedKey !== requestKey`. Tidak ada `setLoading` sama sekali.
- **Hitungan chip status/tanpa-pemilik dari `GET /v1/metrics/summary`, di-scope periode saja** — bukan
  dipersempit oleh sumber/pemilik/kata kunci, karena endpoint metrik tidak mendukung kombinasi itu. Satu
  request untuk delapan angka, bukan delapan request. Didokumentasikan di `lib/metrics.ts`, bukan
  disembunyikan sebagai perilaku tak terjelaskan.
- **Chip sumber sengaja tanpa angka** — desain sendiri tidak memberi `sourceChips` field `count` (beda
  dari `statusChips`/`unassignedChip`, diverifikasi dari kode sumbernya), dan tidak ada satu request
  murah untuk itu. Konsisten dengan mockup, bukan kekurangan.
- **Baris tabel tidak bisa diklik** — desain menaruh `onClick` di setiap baris menuju detail lead, tapi
  issue #32 eksplisit: *"Detail lead (klik satu baris belum membuka apa pun) → #33"*. Membuatnya terlihat
  bisa diklik padahal tidak kemana-mana lebih buruk daripada tidak sama sekali.
- **Kolom Pemilik diselesaikan di klien** — `leadJSON` backend hanya mengirim
  `assigned_to_membership_id` (uuid), bukan nama. `GET /v1/memberships` diambil sekali (tidak berpaginasi,
  independen dari filter lead) dan di-lookup lewat `Map`. Lead tanpa pemilik → "—" miring, warna redup —
  persis perlakuan desain (`ownerColor`/`ownerFontStyle` bersyarat).
- **`source` selalu `"manual"` saat membuat lead dari layar ini** — modal desain hanya punya field
  Nama/Email/Telepon, tidak ada pemilihan sumber. `manual` adalah satu-satunya metode capture yang
  make sense untuk form yang diisi manusia (glossary: source = metode capture, bukan channel marketing).
- **Badge nav "Lead" (dari #40, sebelumnya kosong) disambungkan** — `AppShell` mengambil
  `getMetricsSummary({})` sendiri (tanpa filter periode) sekali saat mount, independen dari state layar
  `/leads` mana pun. Duplikasi satu request kecil dengan layar lead saat keduanya aktif bersamaan,
  diterima sadar — lebih sederhana daripada membuat context/state lifting untuk satu angka.
- **Logika murni dipisah ke `lib/` dan diuji**, mengikuti pola `lib/nav.ts` dari #40:
  `lib/lead-filters.ts` (parse/toggle CSV param, `hasAnyLeadFilter`), `lib/date.ts` (konversi
  `<input type="date">` ↔ ISO 8601 UTC awal/akhir hari, format tampilan `id-ID`), `buildQuery` di
  `lib/leads.ts` diekspor eksplisit untuk diuji.

### Verifikasi

```
npm run typecheck · lint · build   → bersih; 12 route ter-generate
npm run test                        → 33/33 PASS (15 lama + 18 baru: lead-filters, date, leads, metrics)

Terhadap crm_be sungguhan (docker compose + migrate, organization baru "Toko Lead32"):
  register → verify → login (client=dashboard) → GET /v1/me                    semua sesuai kontrak
  POST /v1/leads × 5 (source=manual, bentuk request PERSIS createLead())        201 × 5
  PATCH .../status new→contacted, PATCH .../assignment ke diri sendiri          200, 200
  GET /v1/leads?status=new           → meta.total=4 (bukan 5 — transisi status ikut terhitung)
  GET /v1/leads?assigned_to=none     → meta.total=4 (bukan 5 — assignment ikut terhitung)
  GET /v1/leads?q=Test%202           → meta.total=1, lead yang benar
  GET /v1/metrics/summary            → ditemukan bug by_status di atas
  GET /leads, /leads?status=won (Next.js dev server)                            200, tanpa error render
```

**Batas verifikasi, dicatat apa adanya:** filter/chip/pagination/dialog dibuktikan lewat kontrak API
(`curl` dengan query string persis yang dibangun `buildQuery`) dan lewat unit test logika murninya —
**bukan** lewat interaksi klik di browser sungguhan. Tidak ada tool otomasi browser di sesi ini.

### Utang teknis

- Tidak ada item baru dari issue ini.

### Catatan untuk session berikutnya

- **#33 (detail lead) bisa mulai.** Baris tabel di `/leads` belum navigasi kemana pun — sambungkan ke
  `/leads/{id}` saat halamannya ada, dan hapus komentar "Tidak termasuk" terkait di `leads-list.tsx`.
- `getMetricsSummary` dipanggil dua kali secara independen saat berada di `/leads` (sekali oleh
  `AppShell` untuk badge nav, sekali oleh `leads-list.tsx` untuk chip) — kalau kelak terasa perlu
  disatukan, itu keputusan produk (mis. "badge nav harus ikut ter-filter periode juga?"), bukan sekadar
  refactor teknis.
- `apiFetchList`/`buildQuery` di `lib/leads.ts` adalah pola yang akan dipakai ulang oleh `/customers`
  dan `/tasks` (#35) — keduanya endpoint list berpaginasi dengan bentuk yang sama.
