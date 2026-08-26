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

---

## #33 — Detail lead: timeline, activity, task, status, assignment, konversi

Layar dengan **hampir seluruh aksi tulis produk**. Dibangun dari `<!-- ===== LEAD DETAIL ===== -->`
project `5ac090ad` (dibaca ulang dari awal, bukan cache — ukurannya tetap 111022 byte, tidak ada
perubahan sejak #32).

### Logika transisi status di mockup adalah simplifikasi prototipe — TIDAK diikuti apa adanya

Ini temuan paling penting di issue ini. Kode sumber desain punya `TRANSITIONS`/`FINAL_EXITS` sendiri
yang **berbeda** dari backend sungguhan pada satu kasus: keluar dari status `lost`. Backend
(`validateStatusTransition` di `internal/lead/usecase.go`) membolehkan `lost` pindah ke **kelima**
status jalur utama (penyimpangan terdokumentasi dari TD Phase 2 §5 — lihat notes issue #20: "satu
langkah tepat" tidak bisa ditegakkan tanpa riwayat activity), sementara mockup hanya menawarkan satu
tombol "→ Buka kembali ke Baru".

Karena acceptance criterion issue ini eksplisit — *"transisi status yang tidak sah tidak ditawarkan di
UI"* — logikanya **ditulis ulang sebagai port baris-demi-baris dari fungsi Go**, bukan disalin dari
mockup: `lib/lead-status.ts`'s `isValidStatusTransition(from, to)`. Diuji terhadap **matriks lengkap**
(`lead-status.test.ts`, seluruh 8×8 pasangan status), ditulis tangan dari TD, bukan diturunkan dari
fungsi yang sama yang diuji — supaya bug yang merusak keduanya dengan cara yang sama tidak bisa
bersembunyi.

**Satu penyempitan disengaja dipertahankan dari mockup**: untuk `lost`, hanya satu tombol "→ Buka
kembali ke Baru" yang ditawarkan, bukan kelima status yang backend benarkan. Backend membolehkan lebih
banyak; UI menawarkan lebih sedikit — itu sah (acceptance criterion melarang menawarkan yang **tidak
sah**, tidak mewajibkan menawarkan **semua** yang sah), dan menghindari lima tombol untuk kasus yang
jarang terjadi. Dikunci test (`statusTransitionOptions("lost")` harus tepat satu opsi) supaya
penyempitan ini terlihat sebagai keputusan, bukan bug yang kebetulan cocok.

### Bentuk metadata activity diverifikasi langsung dari `crm_be` sungguhan, bukan ditebak dari desain

Mockup punya akses langsung ke object `lead`/`task` JavaScript-nya sendiri untuk menyusun teks timeline
(`a.text` sudah jadi). API sungguhan tidak mengirim teks jadi — hanya `type` + `metadata` mentah. Setiap
bentuk metadata dibaca dari `internal/{lead,task,customer}/usecase.go`'s pemanggilan `Activity.Record`,
lalu **dibuktikan lagi** lewat lead sungguhan yang dijalankan melalui seluruh siklus hidupnya (buat →
ubah status → assign → catat 3 tipe catatan → buat task → selesaikan task → convert):

```
lead_created      metadata: null                          — TIDAK seperti dugaan awal, tidak ada source
status_changed    {"from":"new","to":"contacted"}          — cocok
lead_assigned     {"to":"<id>","from":null}                — from eksplisit null saat pertama kali, bukan absen
lead_unassigned   {"from":"<id>"}                          — (tidak diuji ulang di sesi ini, sudah diverifikasi #22)
lead_converted    {"customer_id":"<id>"}                   — cocok
task_created      metadata via endpoint task, bukan activity langsung — {"task_id","title"}
task_completed    {"task_id":"<id>"}                       — TANPA title, beda dari task_created
```

`lead_created` metadata **null** berarti teks timeline-nya statis ("Lead dibuat") — sumber lead sudah
terlihat di header, jadi tidak perlu di-plumbing ulang hanya untuk satu baris timeline. `task_completed`
**tidak** membawa title (beda dari `task_created`) — kalau kode mengasumsikan keduanya simetris, hasilnya
"undefined" di layar. Dikunci test eksplisit di `activity-text.test.ts`.

Fungsi murni `activityToTimelineEntry(activity, namesById)` di `lib/activity-text.ts` — menerima
`Map<membership_id, full_name>` yang sama dibangun dari `GET /v1/memberships` (dipakai lagi, sama seperti
kolom Pemilik di #32). Membership yang sudah dinonaktifkan (tidak ada di map) → "Anggota yang sudah
tidak aktif", bukan crash atau `undefined` — riwayat lama tetap harus terbaca meski pelakunya sudah
tidak aktif.

### Dua bagian layar yang **tidak ada di mockup sama sekali**, ditambahkan karena checklist mewajibkannya

- **Form edit field umum** (nama/email/telepon/perusahaan/catatan). Section LEAD DETAIL desain hanya
  punya header read-only + tombol aksi — tidak ada form edit di manapun. Checklist eksplisit: *"Ubah
  field umum (`PATCH /v1/leads/{id}`) — membawa `version`"*. Ditambahkan sebagai dialog terpisah
  (`edit-lead-dialog.tsx`), mengikuti pola modal yang sudah ada.
- **Form buat task**. `onAddLeadTask` di kode sumber desain literal membuat task dengan judul hardcode
  `"Task baru"` tanpa form apa pun (`// visual only` menurut komentar mockup sendiri). Dunia nyata butuh
  minimal judul. `new-task-dialog.tsx` ditambahkan mengikuti pola `new-lead-dialog.tsx` dari #32.

### Keputusan implementasi

- **Konflik penyimpanan (`ConflictDialog`) satu tempat untuk semua aksi pada lead** — status, assignment,
  edit field umum. Ketiganya memicu dialog yang sama; tombol "Muat ulang data terkini" memicu refetch
  penuh (lead+activities+tasks+members), **bukan** langsung menerapkan `error.current` — pengguna harus
  mengklik dulu, sesuai Aturan #35 ("tidak pernah menimpa otomatis" berarti juga tidak pernah *menerima*
  keadaan baru tanpa aksi sadar, bukan hanya soal tidak mengirim ulang data lama).
- **Konflik pada task diberi perlakuan lebih ringan** — pesan inline + refetch otomatis, bukan modal
  terpisah. Task adalah objek turunan dengan taruhan lebih rendah daripada lead itu sendiri; menambah
  modal kedua untuk kasus yang sama akan menggandakan pola tanpa menambah kejelasan.
- **`EditLeadDialog` di-*remount* lewat `key`, bukan `useEffect` + `setState`.** ESLint
  `react-hooks/set-state-in-effect` (sama seperti #32) menolak sinkronisasi form dari prop via effect.
  Diselesaikan dengan memisahkan form ke komponen `EditLeadForm` yang di-`key`-kan
  `${lead.id}-${lead.version}` — setiap kali dialog dibuka dengan `lead` yang mungkin sudah berbeda
  (mis. setelah reload dari konflik), React me-remount komponennya dan `useState(lead.name)` dkk.
  otomatis mendapat nilai segar. Cara yang direkomendasikan React sendiri untuk "reset state saat prop
  berubah", bukan sekadar menghindari lint.
- **Tombol "Konversi menjadi customer" hilang setelah konversi sungguhan terjadi**, bukan hanya
  berdasarkan `status==='won'`. Entity `Lead` tidak punya flag "sudah dikonversi" (itu ada di
  `Customer.converted_from_lead_id`, TD §12 — lead sendiri tidak pernah berubah oleh konversi). Sinyalnya
  diambil dari timeline: `activities.some(a => a.type === 'lead_converted')`.
- **Dropdown penugasan menyertakan SEMUA anggota**, tidak memfilter role `employee` seperti mockup
  (`MEMBERS.filter(m => m.role !== 'employee')`). Backend tidak membatasi assignee berdasarkan role
  (`fk_leads_assignee` tidak punya constraint role), dan Employee justru target assignment paling wajar
  di produk sungguhan — mereka yang menindaklanjuti lead lewat mobile (Phase 5). Memfilter mereka keluar
  akan salah, bukan sekadar beda gaya dari mockup.
- **Hapus lead & Konversi disembunyikan untuk role Manager** di UI (`ActionLeadDelete`/`ActionLeadConvert`
  = Owner/Admin saja, `docs/architecture/authorization.md`). UI tidak menawarkan aksi yang pasti ditolak
  backend — otorisasi sungguhan tetap di backend, tidak diduplikasi di klien.
- **Checkbox task hanya satu arah** (buka → selesai). Tidak ada endpoint "buka kembali task" di backend;
  checkbox yang sudah tercentang di-`disabled`, bukan berpura-pura bisa diklik ulang.
- **`onShowConflictPattern` dari mockup tidak diimplementasikan** — itu tombol demo prototipe murni
  ("Pola desain: konflik penyimpanan →") untuk memamerkan `ConflictDialog` tanpa aksi nyata di baliknya.
  Produk sungguhan memicu dialog yang sama itu dari kegagalan `409` asli, bukan dari tombol demo.

### Verifikasi

```
npm run typecheck · lint · test · build   → bersih; exit code diperiksa (bukan hanya `tail`)
npm run test                                → 54/54 PASS (18 baru: lead-status × 2 file, activity-text)

Terhadap crm_be sungguhan (docker compose + migrate, organization baru "Toko Lead33"), SETIAP endpoint
yang disentuh layar ini, dengan bentuk request PERSIS yang dikirim lib/*.ts:
  PATCH .../status new→contacted                          200
  PATCH .../assignment ke diri sendiri                     200
  POST .../activities × 3 (note_added, call_logged, whatsapp_opened)   201 × 3
  POST .../tasks (title+description+due_at+assignee)       201, bentuk cocok persis tipe Task
  POST /v1/tasks/{id}/complete                             200
  PATCH /v1/leads/{id} (name/email/phone/company/notes)    200, seluruh field tersimpan benar
  PATCH .../status TANPA lost_reason saat status=lost       400 (divalidasi backend, bukan cuma UI)
  PATCH .../status DENGAN lost_reason=price                200
  PATCH .../status lost→new (reopen)                       200
  new→contacted→qualified→proposal→won (jalur penuh)        200 × 4
  POST /v1/leads/{id}/convert                               201
  POST /v1/leads/{id}/convert LAGI                          409 lead_already_converted, message jelas
  DELETE /v1/tasks/{id}                                     204
  DELETE /v1/leads/{id}                                     204, GET setelahnya → 404 (soft delete)
  PATCH /v1/leads/{id} dengan version BASI                  409 version_conflict, error.current
                                                             berbentuk PERSIS tipe Lead — dibuktikan
                                                             langsung, bukan diasumsikan dari baca kode
  GET /leads/{id}, /leads/{id-tidak-valid}, /leads          200 semua (Next.js dev server, tanpa error render)
```

**Batas verifikasi, dicatat apa adanya:** seluruh interaksi dibuktikan lewat kontrak API (`curl` dengan
bentuk request persis yang dikirim kode) dan lewat unit test logika murni (transisi status, format
timeline) — **bukan** lewat klik sungguhan di browser. Tidak ada tool otomasi browser di sesi ini.
Konsekuensinya: **tata letak visual dan interaksi dialog (buka/tutup, styling hover) belum diverifikasi
otomatis** — hanya bahwa route-nya hidup tanpa error render dan setiap panggilan API menghasilkan data
yang benar.

### Utang teknis

- Tidak ada item baru dari issue ini.

### Catatan untuk session berikutnya

- **#34 (tim) dan #35 (customer/task/settings/home) bisa mulai.** Pola `ConflictDialog` dan
  `versionConflictCurrent<T>` di `auth-errors.ts` generik — dipakai ulang langsung untuk task (#35)
  tanpa perubahan.
- `activity-text.ts` sudah menangani seluruh 10 tipe activity yang backend tulis sampai Phase 3. Tidak
  ada tipe baru yang direncanakan sampai Deal ada (pasca-Phase 5).
- `EditLeadDialog`'s pola remount-lewat-`key` adalah jawaban standar untuk "reset form saat prop
  berubah" di bawah `react-hooks/set-state-in-effect` — pakai ulang pola yang sama, bukan
  `useEffect`+`setState`, kalau butuh form serupa di #34/#35.

---

## #34 — Tim: role, undang anggota, nonaktifkan, notifikasi

Layar dibangun dari section `<!-- ===== TEAM ===== -->` project `5ac090ad`, plus lonceng notifikasi di
topbar (`onToggleNotifications`/`notifSeed`) yang di #40 sengaja belum disambungkan.

### Logika permission di mockup salah untuk kasus Owner-vs-Owner — TIDAK diikuti apa adanya

Sama seperti transisi status di #33, ini temuan paling penting di issue ini. `renderTeamVals()` mockup
memakai aturan blanket `role !== 'owner'` untuk menyembunyikan baik dropdown ganti-role maupun tombol
nonaktifkan pada baris manapun yang rolenya `owner` — termasuk saat *actor*-nya sendiri juga Owner.

`docs/architecture/authorization.md` eksplisit membantah ini: "Owner yang mengangkat membership lain
menjadi Owner (co-owner) diizinkan" — aturan Rule #4 (Admin tidak bisa menyentuh Owner) berlaku untuk
Admin, bukan untuk Owner lain. Ditulis ulang sebagai `lib/team-permissions.ts`'s `canChangeRole`/
`canDeactivate`, memeriksa `actor.role === "admin" && row.role === "owner"` secara spesifik (bukan
`row.role === "owner"` polos), dikunci test eksplisit "Owner CAN change another Owner's role (co-owner)
— the design's mockup got this wrong". **Dibuktikan lagi terhadap `crm_be` sungguhan** (lihat
Verifikasi): Owner mempromosikan Admin jadi co-Owner → `200`; Admin biasa mencoba mengubah role Owner →
`403 forbidden "Admin tidak bisa mengubah Owner atau mengangkat Owner baru."`, persis cocok dua baris
kode ini.

### Deaktivasi tiga-cabang (freeze 2.3 ketentuan #3) — bukan dialog konfirmasi biasa

Tombol "Nonaktifkan" **selalu mencoba dulu** `DELETE /v1/memberships/{id}` tanpa parameter (default
`on_open_leads=reject`). Dialog tiga-cabang (`deactivate-member-dialog.tsx`) hanya terbuka setelah
percobaan itu benar-benar kembali `409 membership_has_open_leads` — tidak pernah dibuka spekulatif di
depan. Ini acceptance criterion literal ("anggota tanpa lead terbuka bisa dinonaktifkan tanpa dialog
tambahan"), dan draf pertama komponen ini salah — awalnya `onClick` langsung membuka dialog, diperbaiki
sebelum verifikasi (lihat `handleDeactivateClick` di `team-screen.tsx`).

`openLeadCount` yang ditampilkan di dialog **selalu** nilai dari body error `409` itu sendiri
(`open_lead_count`), tidak pernah dihitung ulang di klien — mencegah dialog menampilkan angka yang sudah
basi kalau lead lain berubah status di antara percobaan pertama dan render dialog.

### Undangan: role Employee hilang dari daftar pilihan mockup

`<select>` role di dialog undang mockup hanya berisi `admin`/`manager` — dua opsi, bukan tiga. Constraint
database (`ck_invitations_role`, migration identitas) eksplisit membolehkan `admin`, `manager`, DAN
`employee`. Tidak ada alasan produk untuk melarang mengundang Employee langsung (mereka pengguna mobile
utama, Phase 5) — `INVITABLE_ROLES` di `invite-member-dialog.tsx` diperbaiki jadi tiga opsi. **Dibuktikan
langsung**: `POST /v1/invitations {role:"employee"}` → `201`, bukan ditebak dari membaca constraint saja.

### Halaman terima undangan ada di `/invitations/accept`, bukan `/invite`

`crm_be`'s `internal/invitation/usecase.go` membangun link email `"%s/invitations/accept?token=%s"` —
konvensi yang sama seperti `/verify-email` dan `/reset-password` (path Next.js = nama endpoint, bukan
disingkat). Draf pertama route ini keliru diletakkan di `(auth)/invite/`; ditemukan lewat pembacaan
literal `usecase.go:274` saat verifikasi terhadap `crm_be` sungguhan (bukan dari mockup — desain sama
sekali tidak mencakup layar publik ini), dipindah ke `(auth)/invitations/accept/` sebelum commit.

### Keputusan implementasi

- **Dua cabang terima undangan** (`invite-form.tsx`): `user_exists: false` → form nama+password
  (validasi ≥12 karakter, pola sama seperti `register`), sukses → arahkan ke `/login` (tidak
  auto-login, konsisten dengan alur register #31). `user_exists: true` → tombol tunggal "Terima
  undangan" tanpa field tambahan; kasus "belum login" **tidak** ditangani manual — `apiFetch`'s
  penanganan `401` (refresh gagal → redirect `/login`) yang sudah ada dari #31 menangani ini secara
  alami tanpa cabang kode baru. Dibuktikan langsung: percobaan `accept` tanpa cookie sesi → `401
  authentication_required`.
- **`canManageTeam` (`role === "owner" || "admin"`) menjaga seluruh permukaan admin sekaligus** — tombol
  undang, fetch `listInvitations` itu sendiri (Manager sama sekali tidak memanggil endpoint itu, bukan
  hanya disembunyikan di UI — `docs/architecture/authorization.md`: Manager tidak punya
  `ActionInvitationList`), dropdown ganti-role vs label statis, tombol nonaktifkan, dan seluruh section
  "Undangan tertunda".
- **Notification bell** (`notification-bell.tsx`) fetch-on-mount + `refreshKey` untuk memicu ulang
  setelah aksi (mark-read/mark-all-read), bukan polling — tidak ada persyaratan real-time di TD Phase 3.
  Klik notifikasi menandainya terbaca **lalu** navigasi ke `/leads/{lead_id}` bila ada — dua aksi
  berurutan, bukan hanya salah satu.
- **`title` notifikasi ditampilkan apa adanya**, tidak direkonstruksi dari `type`+field lain di klien —
  backend sudah menyusun kalimat Indonesia lengkap (`"Lead #1 ditugaskan kepada Anda"`, dibuktikan
  langsung dari response). Membangun ulang kalimat itu di klien akan menduplikasi logika yang sudah benar
  di server dan bisa berbeda kata-katanya.
- **`Member`/`TeamRow` dua tipe berbeda untuk field yang sama** (`Member.id` vs `TeamRow.membershipId`)
  — dikonversi eksplisit di titik pemakaian (`{membershipId: member.id, role: member.role}`), bukan
  menyatukan kedua tipe. `Member` datang dari kontrak API (`memberships.ts`), `TeamRow` sengaja minimal
  untuk kebutuhan fungsi permission murni yang bisa diuji tanpa tipe API.

### Verifikasi

```
npm run typecheck · lint · test · build   → bersih (satu unused eslint-disable diperbaiki, exhaustive-deps
                                              tidak lagi komplain setelah refactor)
npm run test                                → 64/64 PASS (10 baru: team-permissions.test.ts)

Terhadap crm_be sungguhan (docker compose + migrate, organization baru, tiga membership: Owner/Admin/
Employee dibuat lewat alur undangan sungguhan):
  POST /v1/invitations {role:"employee"}                    201 — role Employee benar bisa diundang
  GET  /v1/invitations                                       200, array cocok tipe Invitation
  GET  /v1/invitations/token/{token} (user_exists:false)      200, cocok InvitationTokenInfo
  POST /v1/invitations/accept (user baru: full_name+password) 200 {user_id, membership_id, organization_id}
  GET  /v1/invitations/token/{token} (user_exists:true)       200
  POST /v1/invitations/accept TANPA sesi                      401 authentication_required
  POST /v1/invitations/accept DENGAN sesi user yang benar     200
  DELETE /v1/invitations/{id} (cabut)                         204, list setelahnya kosong
  PATCH .../memberships/{id} {role} — actor ganti role sendiri   403 forbidden (self-role-change)
  PATCH .../memberships/{id} {role:"owner"} — Owner promosikan Admin jadi co-Owner   200
  PATCH .../memberships/{id} {role} — Admin biasa coba ubah role Owner   403 forbidden
                                       "Admin tidak bisa mengubah Owner atau mengangkat Owner baru."
  DELETE .../memberships/{id} (default reject, punya 1 lead terbuka)   409 membership_has_open_leads,
                                       open_lead_count:1 — cocok persis bentuk yang dibaca openLeadCountFrom
  DELETE .../memberships/{id}?on_open_leads=unassign          204, GET lead setelahnya →
                                       assigned_to_membership_id:null
  DELETE .../memberships/{id}?on_open_leads=reassign&reassign_to=  204, lead pindah ke membership tujuan
  GET  /v1/notifications (setelah assignment)                 200, satu notifikasi lead_assigned,
                                       title:"Lead #1 ditugaskan kepada Anda" — dipakai apa adanya
  POST /v1/notifications/{id}/read                            204, ?unread=true setelahnya kosong
  POST /v1/notifications/read-all                              204
  GET /team, /invitations/accept?token=...                    200 (Next.js dev server, tanpa error render)
```

**Batas verifikasi, dicatat apa adanya:** seluruh interaksi dibuktikan lewat kontrak API (`curl` dengan
bentuk request persis yang dikirim `lib/*.ts`) dan lewat unit test logika permission murni — **bukan**
lewat klik sungguhan di browser. Tidak ada tool otomasi browser di sesi ini. Konsekuensinya: tata letak
visual dialog tiga-cabang dan panel notifikasi belum diverifikasi otomatis — hanya bahwa route-nya hidup
dan setiap panggilan API menghasilkan data yang benar.

### Utang teknis

- Tidak ada item baru dari issue ini.

### Catatan untuk session berikutnya

- **#35 (customer/task/settings/home) bisa mulai.** Pola fetch-on-mount+`refreshKey` di
  `notification-bell.tsx` dan pola "coba dulu, cabang hanya pada error tertentu" di
  `handleDeactivateClick` bisa dipakai ulang kalau ada kebutuhan serupa.
- `team-permissions.ts` sengaja terpisah dari `memberships.ts` (tipe API) — kalau kelak ada layar lain
  yang butuh logika permission serupa (mis. reassignment massal), pakai ulang fungsi ini, jangan tulis
  ulang aturan Admin-vs-Owner di tempat lain.
- Rute publik undangan sekarang ada di `(auth)/invitations/accept/`, bukan `(auth)/invite/` — kalau ada
  link lama yang beredar mengarah ke `/invite`, itu tidak pernah benar sejak awal (bug sesi ini, tidak
  pernah di-deploy).

---

## #35 — Home metrik, customer, daftar task, settings — penutup Phase 3

Empat layar top-level yang tersisa, dibangun dari section HOME/CUSTOMERS/TASKS/SETTINGS project
`5ac090ad` — semua empat lebih tipis di mockup daripada TEAM/LEAD DETAIL (`sectionIsCustomers` bahkan
`false` di prototipe), dan tiga dari empat butuh perluasan nyata di luar apa yang digambar, bukan hanya
port langsung.

### Home: setiap angka mockup dihitung di browser — TIDAK diikuti apa adanya

Sama seperti `by_status` di #32, kelas bug yang sama: `renderHomeVals()` mockup menghitung
`totalLeads`/`wonCount`/`conversionRate` dari array `s.leads` in-memory, dan tabel performa anggotanya
memakai **tiga nama hardcode** (`['Sari Dewi','Bagus Prakoso','Tania Putri']`) dengan waktu respons
fiktif `(2+i)+' jam'` — bukan dari endpoint agregat sama sekali. AC #10 eksplisit melarang ini. Diganti
total: `total_new`/`unassigned`/`statusCount(summary,"won")` dari `GET /v1/metrics/summary`,
tabel performa dari `GET /v1/metrics/employees` sungguhan.

Satu bagian mockup **memang benar** dan dipertahankan: perlakuan `conversion_rate` `null` sebagai
"Belum ada data" (bukan "0%"). Yang salah cuma sumber datanya (dihitung sendiri vs field asli), bukan
niatnya. `conversion_rate` backend ternyata **fraksi mentah** (`won/(total-spam-unqualified)`, tanpa
×100 — dibuktikan langsung: `0.5` untuk 1 won dari 2 lead), bukan persentase siap pakai — `formatConversionRate`
mengalikan 100 sendiri.

**Gap freeze 3.2**: empat kartu mockup tidak mencakup "lead per status" sebagai metrik tersendiri (freeze
3.2 mewajibkannya, terpisah dari "Lead menang" saja). Ditambahkan sebagai baris badge status
(`STATUS_META`) di bawah kartu, masing-masing tautan cepat ke `/leads?status=X`.

**Tautan cepat** (AC eksplisit) ke tiga dari empat kartu: Lead masuk → `/leads` (periode saja), Belum
ter-assign → `+assigned_to=none`, Lead menang → `+status=won`. Conversion rate sengaja **tidak** ditautkan
— ia rasio, bukan satu subset lead yang bisa difilter.

**Employee tidak bisa melihat Home sama sekali** — `ActionMetricsRead` mengecualikan Employee (TD §2.4).
Digerbang di layar ini sendiri (`canViewMetrics = role !== "employee"`) sebelum mencoba fetch, bukan
membiarkan permintaan 403 lalu menampilkan error yang membingungkan. **Dibuktikan langsung**:
`GET /v1/metrics/summary` sebagai Employee → `403 forbidden`.

### Customer: pencarian, pagination, DAN form edit tidak ada di mockup sama sekali

Detail customer mockup adalah **modal, bukan route** — diganti jadi `/customers/{id}` mengikuti konvensi
`/leads/{id}` yang sudah ada di seluruh app (satu pola detail-screen, bukan dua). Mockup hanya punya
tombol Hapus, **tidak ada form ubah** meski checklist mewajibkan `PATCH /v1/customers/{id}` — ditambahkan
sebagai `EditCustomerDialog`, memakai pola remount-lewat-`key` yang sama seperti `EditLeadDialog` tapi
**tanpa** `version` (entity `Customer` memang tidak punya field itu — "customers tidak diedit dari mobile
offline" per komentar TD, jadi tidak ada konflik tulis untuk dideteksi, dan tidak ada `ConflictDialog` di
layar ini sama sekali).

Tautan "Berasal dari lead" di mockup literal `'#' + detail.fromLeadNumber` — field seed lokal di objek
customer palsu, **bukan** dari lookup nyata, dan `href="#"` adalah link mati. `Customer` sungguhan hanya
membawa `converted_from_lead_id` (UUID), tanpa nama/nomor lead. Diselesaikan dengan `getLead()` kedua
setelah customer dimuat; bila lead itu sendiri sudah terhapus terpisah (bukan skenario yang mungkin
lewat produk normal, tapi mungkin lewat penghapusan manual), tautan berdegradasi jadi teks "Lead sudah
dihapus" alih-alih menggagalkan seluruh layar.

**Dibuktikan langsung** (AC: "mengubah nama customer tidak mengubah lead asalnya"): `PATCH
/v1/customers/{id}` mengubah nama customer, `GET /v1/leads/{id}` setelahnya menunjukkan nama lead
**tidak berubah** — persis yang ditampilkan tautan "Berasal dari lead" di layar untuk membuktikannya
secara visual.

`canManage` (`role==='owner'||'admin'`) di mockup **cocok persis** `ActionCustomerUpdate`/
`ActionCustomerDelete` backend — satu-satunya bagian JS mockup yang di-port apa adanya. **Dibuktikan
langsung**: Manager membaca daftar customer → `200`; Manager mencoba `PATCH`/`DELETE` → `403 forbidden`
keduanya, tombolnya sendiri tidak ditawarkan di UI untuk role ini (AC eksplisit).

### Task lintas lead: filter mockup seluruhnya client-side atas array lokal

`taskFilterAssignee` mockup mencocokkan **nama** (`t.assignee === s.taskFilterAssignee`); `GET
/v1/tasks`'s `assigned_to` sungguhan adalah **UUID membership**. Dropdown penanggung jawab dibangun dari
`listMemberships()` (id+nama), bukan dari nama unik yang dikumpulkan dari task yang sudah dimuat. Filter
`due_before` (wajib di checklist) **tidak ada di mockup sama sekali** — ditambahkan sebagai input tanggal
sederhana di samping filter lain.

Checkbox mockup adalah toggle dua arah (`onToggle` membalik `t.done` lokal). Sama seperti temuan #33 di
`lead-detail.tsx`: tidak ada endpoint "buka kembali task". Checkbox di sini hanya pernah bergerak
buka→selesai lewat `completeTask(id, version)`, dan **disabled** begitu `status==='done'` alih-alih
berpura-pura bisa dibalik. Konflik `409` pada penyelesaian task memakai perlakuan ringan yang sama seperti
#33 (pesan inline + refetch otomatis), bukan modal terpisah — **dibuktikan langsung**: menyelesaikan task
yang sama dua kali dengan `version` basi → `409 version_conflict`, `error.current` berbentuk persis
`Task`.

Baris task menampilkan referensi lead sebagai tautan berlabel "Lead" ke `/leads/{lead_id}` — bukan
`"Lead #1005"` seperti mockup, karena `taskJSON` backend tidak pernah mengirim `lead_number` (hanya
`lead_id`, sebuah UUID); menampilkan nomor yang tidak dikirim backend berarti request tambahan per baris
untuk sesuatu yang bukan bagian checklist. Backend membatasi visibilitas Employee ke lead yang di-assign
ke dirinya secara otomatis di level query (`buildTaskWhere`'s `isEmployee` branch) — tidak ada filtering
tambahan di klien untuk itu.

**Dibuktikan langsung** terhadap `crm_be` sungguhan: `GET /v1/tasks` dengan `status`, `assigned_to`, dan
`due_before` masing-masing menyaring dengan benar; `completeTask` dua kali dengan `version` yang sama →
`409` pada percobaan kedua.

### Settings: mockup menggambar form edit yang tidak pernah berfungsi, bahkan di prototipenya sendiri

`renderSettingsVals()` mockup **literal `return {}`** — setiap input memakai `defaultValue` React yang
tidak terkontrol tanpa `onChange`, dan kedua tombol "Simpan" tidak punya `onClick` sama sekali. Ini bukan
kasus "mockup benar, port apa adanya" maupun "mockup salah, backend beda" — mockup ini **tidak pernah
diimplementasikan melampaui scaffolding visual statis**. Lebih penting: `crm_be` **tidak punya** endpoint
`PATCH` untuk profil organization atau user sama sekali — kontraknya cuma `GET /v1/me` (TD §8's peta
layar, dan checklist issue ini sendiri hanya menyebut "dari `GET /v1/me`", tanpa PATCH). Dibangun sebagai
tampilan **read-only** dari `useSession()` — mempertahankan pengelompokan dua-kartu visual mockup, tapi
tanpa affordance edit palsu.

### Keputusan implementasi lain

- **`placeholder-screen.tsx` dihapus.** Lima halaman placeholder dari #40 semuanya sudah diganti layar
  sungguhan sejak #32–#35; komponennya sendiri sudah tidak dipakai di mana pun (diverifikasi lewat
  `grep` sebelum dihapus). notes.md #40 mencatat ini sebagai "utang bertanggal" — tanggalnya sekarang.
- **`lib/metrics.ts` bertambah empat fungsi murni yang diuji**: `formatConversionRate`,
  `formatAvgResponseSeconds`, `periodToRange` (mengambil `now: Date` sebagai parameter, bukan memanggil
  `new Date()` sendiri — pola yang sama seperti alasan Workflow script tidak boleh memanggil waktu
  langsung, diterapkan di sini supaya fungsinya tetap bisa diuji dengan `now` tetap).
- **`Date.now()` langsung di body render memicu `react-hooks/purity`** (ESLint, React 19 toolchain) —
  beda dari `new Date()` yang tidak kena aturan yang sama (dipakai apa adanya di `lead-detail.tsx` sejak
  #33 dan di `periodToRange`'s pemanggil). Diperbaiki di `task-list.tsx` dengan mengganti
  `Date.now()` jadi `new Date()` inline, mengikuti pola yang sudah ada.
- **Customer tidak punya `ConflictDialog`** — satu-satunya layar tulis di Phase 3 tanpa itu, karena
  `Customer` tidak punya `version` sama sekali (bukan karena lupa menambahkannya).

### Verifikasi

```
npm run typecheck · lint · test · build   → bersih; 15 route ter-generate termasuk /customers/[id]
npm run test                                → 70/70 PASS (6 baru: formatConversionRate ×2,
                                               formatAvgResponseSeconds ×3, periodToRange ×1)

Terhadap crm_be sungguhan (docker compose + migrate, organization baru "Toko Metrik35", lead → won →
convert, task overdue, membership Manager + Employee lewat alur undangan sungguhan):
  GET /v1/metrics/summary?from=&to=            200, conversion_rate:0.5 (fraksi mentah, bukan 50)
  GET /v1/metrics/employees?from=&to=           200, avg_response_seconds numerik (bukan null di sini)
  GET /v1/metrics/summary sebagai Employee      403 forbidden — menegaskan gerbang canViewMetrics perlu
  POST /v1/leads/{id}/convert                   201
  GET  /v1/customers, GET .../{id}               200, bentuk cocok persis tipe Customer
  PATCH /v1/customers/{id} {name}                200, lalu GET /v1/leads/{originating_id}
                                                 membuktikan nama lead TIDAK berubah
  GET  /v1/customers sebagai Manager             200 (read diizinkan)
  PATCH/DELETE /v1/customers/{id} sebagai Manager  403 forbidden keduanya
  POST /v1/leads/{id}/tasks (due_at masa lalu)   201
  GET /v1/tasks?status=open&assigned_to=&due_before=   masing-masing menyaring benar
  POST /v1/tasks/{id}/complete                   200, lalu diulang dengan version sama → 409
                                                 version_conflict, error.current bentuk Task persis
  GET /v1/me                                     200, bentuk cocok persis field yang dirender Settings
  Dev server: /, /customers, /customers/{id}, /tasks, /settings   semua 200, tanpa error render
```

**Batas verifikasi, dicatat apa adanya:** seluruh interaksi dibuktikan lewat kontrak API dan unit test
logika murni — **bukan** lewat klik sungguhan di browser (tidak ada tool otomasi browser di sesi ini).
Tata letak visual (grid kartu, tabel performa, dialog edit customer) belum diverifikasi otomatis.

### Utang teknis

- Tidak ada item baru dari issue ini.

### Penutup Phase 3

**Seluruh 13 acceptance criteria PRD Phase 3 terpenuhi**, dicek satu per satu terhadap kode/verifikasi
sesi ini dan sesi-sesi sebelumnya (#30–#34):

| # | Kriteria | Bukti |
|---|---|---|
| 1 | Owner selesaikan core loop tanpa terminal | #32–#35: daftar→verifikasi→masuk→lead→assign→activity→task→status→convert, seluruhnya lewat UI |
| 2 | CORS eksplisit per origin, fail-fast production | #30, `internal/shared/httpx/cors.go` + `config.validate()` |
| 3 | Cookie `HttpOnly`, CSRF di non-GET | #31, dibuktikan `logout` tanpa header → `403 csrf_token_invalid` |
| 4 | N request 401 paralel → 1 refresh | #31, test konkurensi `api-client.test.ts` |
| 5 | Filter lead: status/pemilik/sumber/periode/kata kunci + pagination | #32 |
| 6 | Filter "tanpa pemilik aktif" permanen | #32, chip permanen di `/leads` |
| 7 | Detail lead: timeline + task + seluruh aksi tulis | #33 |
| 8 | Konflik tampil, muat ulang, tidak pernah menimpa otomatis | #33, `ConflictDialog` |
| 9 | Nonaktifkan anggota ber-lead-terbuka → paksa pilih | #34, `DeactivateMemberDialog` |
| 10 | Metrik dari endpoint agregat, bukan dihitung di browser | #35 (lihat temuan di atas) |
| 11 | Conversion rate kecualikan spam/unqualified | #30 (backend), #35 (ditampilkan apa adanya) |
| 12 | Seluruh teks Bahasa Indonesia | Seluruh issue #31–#35 |
| 13 | CI terpisah `crm_dashboard/` dengan `paths:` filter | #31, `.github/workflows/ci-dashboard.yml` |

**Belum dilakukan, di luar kapasitas sesi coding:** demo ke calon pengguna (freeze bagian 4 — tujuan
phase-nya, bukan langkah opsional) dan keputusan phase berikutnya (4 — Public API, atau 5 — Employee
Mobile; freeze menempatkan keduanya tidak saling bergantung). Keduanya keputusan produk/manusia, dicatat
di `STATUS.md` sebagai item terbuka, bukan diputuskan sepihak di sesi ini.
