# Phase 3 — Owner Dashboard · Technical Design

> **Bagaimana.** Apa & kenapa di [`prd.md`](./prd.md).
>
> Memuat **delta** untuk Phase 3. Yang sudah ada di [`freeze.md`](../../architecture/freeze.md), [`api.md`](../../architecture/api.md), [`authentication.md`](../../architecture/authentication.md), [`authorization.md`](../../architecture/authorization.md), [`multi-tenancy.md`](../../architecture/multi-tenancy.md), dan `.claude/skills/jualin-backend/` **tidak diulang** — hanya dirujuk.

---

## 0. Dua sisi, satu produk

Phase 3 menyentuh dua aplikasi:

| Aplikasi | Pekerjaan | Issue |
|---|---|---|
| `crm_be` (Go) | Middleware CORS · paket `internal/metrics` · satu `Action` authz baru | #30 |
| `crm_dashboard` (Next.js) | Seluruh antarmuka | #31–#35 |

Bagian §1–§2 adalah backend, §3 seterusnya frontend. Keduanya di satu TD karena satu phase, satu produk
(ADR-009) — bukan karena keduanya saling bergantung secara teknis di luar kontrak HTTP.

---

## 1. CORS

**Belum ada sama sekali.** Tidak ada middleware CORS di `crm_be` dan tidak ada keputusan origin di
`docs/architecture/`. Tanpa ini, keputusan C2 (browser → Go langsung) tidak bisa berjalan.

### 1.1 Konfigurasi

```go
// internal/shared/config/config.go
CORSAllowedOrigins []string `env:"CORS_ALLOWED_ORIGINS" envSeparator:","`
```

| Keputusan | Alasan |
|---|---|
| Daftar eksplisit, **tidak pernah `*`** | `Access-Control-Allow-Credentials: true` dan `Access-Control-Allow-Origin: *` **saling meniadakan** — browser menolak kombinasinya. Karena dashboard mengirim cookie, wildcard bukan pilihan yang tersedia, bukan sekadar tidak disarankan. |
| **Wajib non-kosong saat `AppEnv == "production"`** — divalidasi di `config.validate()` | Aturan #36 (fail-fast). Konfigurasi CORS yang kosong di production berarti dashboard mati total; lebih baik proses berhenti saat boot dengan pesan yang menyebut variabelnya daripada menyajikan traffic yang seluruhnya gagal di browser. |
| Default kosong di development | Pengembangan lokal memakai `http://localhost:3000` yang ditulis eksplisit di `.env`/`docker-compose.yml` — bukan disembunyikan sebagai default kode. |

`AppBaseURL` yang sudah ada **bukan** penggantinya: ia satu nilai untuk membangun tautan email, sementara
origin yang diizinkan bisa lebih dari satu (production + staging + preview).

### 1.2 Middleware

`internal/shared/httpx/cors.go` (baru), dipasang di `newRouter` **sebelum** route manapun dan sebelum
`authn.Middleware` — request preflight tidak membawa kredensial dan tidak boleh menyentuh lapisan auth.

```
Origin request tidak ada di daftar  → lanjut tanpa header CORS (browser yang menolak, bukan server
                                      yang membocorkan bahwa origin itu "salah")
Origin cocok                        → echo origin persis + Access-Control-Allow-Credentials: true
Method OPTIONS                      → 204, tanpa memanggil handler
```

Header yang diizinkan minimal: `Content-Type`, `Authorization`, `X-CSRF-Token`, `Idempotency-Key`.
Method: `GET`, `POST`, `PATCH`, `DELETE`, `OPTIONS`.

`Access-Control-Max-Age` disetel supaya preflight tidak berulang di setiap request — tanpa itu, setiap
`PATCH` menjadi dua round trip.

### 1.3 Test

| Test | Membuktikan |
|---|---|
| Origin diizinkan → header echo + `Allow-Credentials: true` | Jalur normal |
| Origin tidak diizinkan → **tidak ada** header CORS, request tetap diproses | Server tidak membocorkan daftar origin |
| `OPTIONS` → `204` tanpa menyentuh handler | Preflight tidak memicu logika bisnis |
| **Tidak pernah** mengirim `Allow-Origin: *` saat `Allow-Credentials: true` | Kombinasi yang mustahil, dikunci test |
| `config.validate()` menolak `CORS_ALLOWED_ORIGINS` kosong saat `APP_ENV=production` | Aturan #36 |

---

## 2. Metrik — `internal/metrics` (paket baru)

[`02-crm-core/td.md`](../02-crm-core/td.md) §5: *"Di Phase 2 belum ada endpoint metrik (itu Phase 3)"*.

**Paket sendiri, bukan tempelan di `internal/lead`.** Ia membaca lintas `leads`/`customers`/`activities`
dan tidak dimiliki satu pun dari ketiganya; package-by-feature (ADR-011) menempatkan fitur di foldernya
sendiri. Membaca tabel lintas domain lewat **SQL langsung**, bukan interface Go — pola yang sama
`customer.Convert` pakai terhadap `leads` di #23, dengan alasan yang sama: satu database, agregat lintas
tabel, tidak ada yang bisa diambil dari interface sempit tanpa N+1.

Paket ini **read-only** — tidak ada `Store.InTx`, tidak ada `Repos`, karena tidak ada satu pun tulis.
Ini penyimpangan sadar dari bentuk lima berkas: `entity.go` · `port.go` · `usecase.go` ·
`repository_postgres.go` · `handler_http.go` tetap ada, tetapi `port.go` hanya berisi `Repository`
(tanpa `Store`), dan `NewUsecase` menerima `Repository` langsung. Menambahkan `Store`/`InTx` untuk paket
yang tidak pernah menulis adalah mesin yang tidak dipakai (Aturan #27).

### 2.1 Dua endpoint, bukan lima

Satu layar home tidak boleh butuh lima round trip.

| Method | Path | Isi |
|---|---|---|
| `GET` | `/v1/metrics/summary?from=&to=` | `total_new` · `by_status{}` · `unassigned` · `conversion_rate` |
| `GET` | `/v1/metrics/employees?from=&to=` | array per employee: `membership_id`, `full_name`, `lead_count`, `avg_response_seconds`, `converted_count` |

`from`/`to` ISO 8601 UTC (Aturan #33), opsional — kosong berarti seluruh riwayat. Rentang membatasi
`leads.created_at`, bukan waktu peristiwa lain: "lead masuk periode ini, apa yang terjadi padanya".

### 2.2 Conversion rate — definisi, bukan tebakan

```
conversion_rate = jumlah lead won / (total lead - lead spam - lead unqualified)
```

**`spam` dan `unqualified` dikeluarkan dari penyebut.** Ini akhirnya menegakkan acceptance criterion #5
Phase 2, yang sampai sekarang hanya berupa "kedua status itu ada dan terpisah" (`02-crm-core/td.md` §5).
Lead sampah bukan kegagalan penjualan; memasukkannya ke penyebut membuat angka konversi turun karena
alasan yang tidak ada hubungannya dengan performa.

Penyebut nol → `conversion_rate` dikirim `null`, **bukan** `0`. Nol berarti "sudah dicoba, tidak ada yang
berhasil"; `null` berarti "belum ada yang bisa dihitung". Keduanya tampil berbeda di layar.

### 2.3 Waktu respons — definisi, bukan tebakan

```
waktu respons satu lead = MIN(activities.created_at WHERE type <> 'lead_created') - leads.created_at
```

Artinya: **berapa lama sampai ada yang benar-benar menyentuh lead ini.** Setiap peristiwa nyata sejak #21
menghasilkan activity, jadi tabel itu adalah jejak yang paling jujur — lebih baik daripada memakai
`status_changed` saja (mencatat pindah status tapi melewatkan catatan dan telepon) atau `updated_at`
(berubah karena hal yang bukan respons manusia).

**Lead yang belum pernah tersentuh dikecualikan dari rata-rata, bukan dihitung nol.** Memasukkannya
sebagai nol membuat rata-rata terlihat membaik justru saat lead diabaikan.

### 2.4 RBAC

`ActionMetricsRead` baru di `internal/shared/authz`:

| Action | Owner | Admin | Manager | Employee |
|---|---|---|---|---|
| `metrics.read` | ✅ | ✅ | ✅ | — |

Employee tidak dapat: dashboard memang bukan alatnya (PRD *Di luar cakupan*), dan angka lintas
organization adalah informasi manajemen. Employee mendapat mobile di Phase 5.

> Konsekuensinya `internal/metrics` **tidak** punya cabang `isEmployee` di query-nya sama sekali —
> tidak seperti `lead`/`task`/`customer`. Employee tidak pernah sampai ke repository ini.

### 2.5 Test

- Unit tanpa Docker (`TestUnit_*`, fake `Repository`): gerbang `authz` per role; `conversion_rate`
  `null` saat penyebut nol; rentang `from`/`to` diteruskan apa adanya.
- Postgres asli: `spam`/`unqualified` benar-benar keluar dari penyebut (bukan sekadar dari pembilang);
  lead tak tersentuh tidak menarik rata-rata waktu respons ke bawah; agregat di-scope ke satu
  organization saja — **wajib**, karena kebocoran di endpoint agregat berarti membocorkan bentuk bisnis
  tenant lain, bukan satu baris.
- Harness isolasi tenant (`cmd/api/tenant_isolation_test.go`): entri baru untuk kedua endpoint metrik.
  Slice `[]isolationCase` yang sama, sesuai desainnya sejak #11.

---

## 3. Fondasi `crm_dashboard`

| Keputusan | Nilai |
|---|---|
| Framework | Next.js, **App Router**, TypeScript |
| Styling | Tailwind |
| Komponen | shadcn/ui (di-copy ke repo, bukan dependensi runtime — C3) |
| Bahasa | Indonesia, **tanpa library i18n** (C1) |
| Konfigurasi | `NEXT_PUBLIC_API_BASE_URL` — satu-satunya yang dibutuhkan untuk bicara ke API |

### 3.1 CI

`.github/workflows/ci-dashboard.yml` (baru), dengan `paths:` filter `crm_dashboard/**` — satu workflow
per aplikasi, persis bentuk `ci-backend.yml` yang sudah ada (CLAUDE.md, ADR-009). Isinya: install,
typecheck, lint, build. Backend tidak ikut jalan saat hanya UI berubah, dan sebaliknya.

---

## 4. Klien API & sesi

Bagian tersulit di seluruh phase ini, dan satu-satunya yang bisa rusak dengan cara yang tidak terlihat
di layar.

### 4.1 Cookie yang tidak bisa dibaca

Aturan #25: token dashboard di cookie `HttpOnly` — **tidak pernah** `localStorage`. Konsekuensi yang
harus diterima klien:

- `access_token` dan `refresh_token` **tidak bisa disentuh JavaScript sama sekali.** Klien tidak pernah
  tahu apakah ia punya sesi; ia hanya tahu setelah bertanya.
- `csrf_token` **sengaja bukan** `HttpOnly` (sudah begitu sejak #10) — klien membacanya dan
  mengirimkannya kembali sebagai header `X-CSRF-Token` di setiap request non-GET. Itulah keseluruhan
  mekanisme double-submit: penyerang lintas situs bisa memicu request dengan cookie ikut terkirim, tetapi
  tidak bisa **membaca** cookie untuk menyusun headernya.
- Setiap `fetch` memakai `credentials: 'include'`.

Sesi ditegakkan dengan memanggil `GET /v1/me` di layout terproteksi — bukan dengan memeriksa token
(tidak bisa). `401` berarti belum masuk; arahkan ke login.

### 4.2 Refresh single-flight — wajib, bukan optimasi

> Satu layar dengan enam widget yang semuanya menerima `401` bersamaan akan memanggil
> `/v1/auth/refresh` enam kali. Rotasi refresh token (#10) menganggap pemakaian token yang sudah
> dirotasi sebagai **reuse attack** dan mencabut seluruh `family_id` — pengguna terlempar keluar.
> Tanpa single-flight, sesi mati sendiri di layar yang paling sibuk.

Kontraknya:

```
401 diterima
  → apakah sudah ada refresh berjalan?
      ya    → tunggu hasil yang sama
      tidak → mulai satu refresh
  → refresh berhasil → ulangi request asli SEKALI
  → refresh gagal    → bersihkan state, arahkan ke login
```

Request yang diulang **tidak pernah** memicu refresh kedua — satu percobaan, lalu menyerah. Loop
refresh→401→refresh adalah cara paling mudah membuat backend melihat pola serangan dari client sendiri.

`/v1/auth/refresh` sendiri dikecualikan dari mekanisme ini; `401` darinya berarti sesi memang habis.

### 4.3 Bentuk respons

Envelope `{data, meta}` dan `{error}` (Aturan #33) sudah tetap. Klien mengetiknya sekali sebagai tipe
generik dan memakainya di semua tempat — bukan mengurai bentuk yang sama berulang kali di tiap layar.

---

## 5. Error di layar

Backend mengirim `error.message` **dalam Bahasa Indonesia** — ditampilkan **apa adanya**. Tidak
diterjemahkan ulang di klien: dua sumber kebenaran untuk satu kalimat berarti keduanya akan berbeda
suatu saat, dan yang di layar belum tentu yang benar.

`error.code` yang menggerakkan **perilaku** (kontrak `api.md`: `code` stabil, `message` boleh berubah).
Empat kode butuh perlakuan khusus, bukan toast biasa:

| `code` | HTTP | Perilaku UI |
|---|---|---|
| `version_conflict` | 409 | Body memuat `error.current`. Tampilkan "data sudah diubah orang lain", muat ulang form dari `current`, dan **jangan pernah menimpa otomatis** (Aturan #35). Ini satu-satunya tempat pengguna harus memilih secara sadar. |
| `organization_selection_required` | 409 | Body memuat `organizations`. Layar pemilihan organization saat login (ADR-007) — pengguna dengan lebih dari satu membership aktif. |
| `membership_has_open_leads` | 409 | Body memuat `open_lead_count`. Dialog yang **memaksa memilih** `unassign` / `reassign` / batal sebelum penonaktifan berjalan (#22). |
| `validation_failed` | 400 | `details[]` berisi `{field, code}` — tampilkan di bawah field yang bersangkutan, bukan sebagai pesan global. |

Sisanya (`forbidden`, `not_found`, `rate_limited`, `internal_error`, …) ditampilkan sebagai pesan biasa.

---

## 6. `version` wajib bolak-balik lewat form

`leads` dan `tasks` memakai optimistic locking (Aturan #35). Setiap form edit **menyimpan `version` yang
diterima saat membaca** dan mengirimkannya kembali saat menyimpan.

Form yang lupa membawanya akan bekerja tepat sekali lalu gagal `409` di setiap penyimpanan berikutnya —
gejala yang mudah salah dibaca sebagai "backend rusak". Dicatat di sini supaya tidak didiagnosis dari
nol saat terjadi.

---

## 7. Endpoint yang dikonsumsi

Seluruhnya **sudah ada** kecuali dua endpoint metrik (§2), yang dibuat issue #30.

| Area | Endpoint | Sejak |
|---|---|---|
| Auth | `POST /v1/auth/{register,verify-email,verify-email/resend,login,refresh,logout,password/forgot,password/reset}` · `GET /v1/me` | #9, #10 |
| Lead | `POST/GET /v1/leads` · `GET/PATCH/DELETE /v1/leads/{id}` · `PATCH /v1/leads/{id}/{status,assignment}` | #20, #22 |
| Activity | `GET/POST /v1/leads/{id}/activities` | #21 |
| Task | `POST/GET /v1/leads/{id}/tasks` · `GET /v1/tasks` · `PATCH/DELETE /v1/tasks/{id}` · `POST /v1/tasks/{id}/complete` | #21 |
| Customer | `POST /v1/leads/{id}/convert` · `GET /v1/customers` · `GET/PATCH/DELETE /v1/customers/{id}` | #23 |
| Notification | `GET /v1/notifications` · `POST /v1/notifications/{id}/read` · `POST /v1/notifications/read-all` | #22 |
| Membership | `GET /v1/memberships` · `PATCH/DELETE /v1/memberships/{id}` | #11, #22 |
| Invitation | `POST/GET /v1/invitations` · `DELETE /v1/invitations/{id}` · `GET /v1/invitations/token/{token}` · `POST /v1/invitations/accept` | #11 |
| **Metrik** | `GET /v1/metrics/{summary,employees}` | **#30 — belum ada** |

> Bila sebuah layar terasa membutuhkan endpoint yang tidak ada di tabel ini, periksa dulu apakah yang
> ada sudah cukup. Menambah endpoint di tengah phase UI berarti dua aplikasi berubah dalam satu PR
> tanpa direncanakan.

### 7.1 Filter yang sudah didukung

`GET /v1/leads` menerima `status` · `source` (keduanya dipisah koma) · `assigned_to` (id **atau**
`none`) · `q` · `created_from` · `created_to` · `page` · `per_page` — sejak #20.

**`assigned_to=none` adalah "lead tanpa pemilik aktif"** — kewajiban warisan Phase 2
(`02-crm-core/td.md` §19, freeze 2.3 ketentuan #3). Query-nya sudah ada; Phase 3 membangun layarnya dan
menjadikannya filter **permanen** yang terlihat, bukan opsi tersembunyi di menu lanjutan.

---

## 8. Peta layar

| Layar | Endpoint utama | Issue |
|---|---|---|
| Login · Register · Verifikasi email · Lupa/reset password · Pilih organization | `/v1/auth/*`, `GET /v1/me` | #31 |
| Daftar lead + filter + pagination | `GET /v1/leads` | #32 |
| Detail lead: timeline, activity, task, status, assignment, konversi | `GET /v1/leads/{id}` + activities/tasks/status/assignment/convert | #33 |
| Daftar & undangan anggota · ubah role · nonaktifkan | `/v1/memberships`, `/v1/invitations` | #34 |
| Notifikasi | `/v1/notifications` | #34 |
| Home metrik | `GET /v1/metrics/*` | #35 |
| Daftar customer + detail | `/v1/customers` | #35 |
| Daftar task lintas lead | `GET /v1/tasks` | #35 |
| Settings organization | `GET /v1/me` | #35 |

---

## 9. Rencana test

Dashboard **tidak** memakai testcontainers — ia tidak menyentuh database. Yang diuji adalah lapisan yang
bisa salah tanpa terlihat:

| Yang diuji | Kenapa wajib |
|---|---|
| **Refresh single-flight** — N request paralel yang `401` menghasilkan **tepat satu** panggilan refresh | §4.2: tanpa ini rotasi token mencabut sesi. Ini bug yang hanya muncul di bawah konkurensi, persis seperti alokasi `lead_number` di #19 — dan sama seperti di sana, test berurutan akan hijau meski logikanya salah total. |
| Header `X-CSRF-Token` terpasang di setiap request non-GET, dan **tidak** di `GET` | Satu request tulis yang lupa header → `403` yang membingungkan |
| Pemetaan `error.code` → perilaku (empat kode di §5) | Perilaku, bukan tampilan — dan `version_conflict` yang salah tangani berarti data pengguna tertimpa |
| `version` ikut terkirim di setiap form edit | §6 |
| Backend: seluruh test §1.3 dan §2.5 | Sisi Go |

Test UI visual (snapshot, e2e penuh) **di luar cakupan** Phase 3 — nilai terbesarnya ada setelah bentuk
layar stabil, dan bentuk itu justru yang paling mungkin berubah setelah demo pertama ke calon pengguna.

---

## 10. Risiko teknis

| Risiko | Mitigasi |
|---|---|
| **Refresh storm mencabut sesi pengguna** — kegagalannya terlihat seperti "aplikasi mengeluarkan saya sendiri", bukan seperti bug klien | Single-flight (§4.2) + test konkurensi wajib (§9) |
| **CORS salah konfigurasi di production** — dashboard mati total, tanpa error di sisi server | Validasi fail-fast saat boot (§1.1, Aturan #36) |
| **Metrik dihitung di browser** dari halaman yang terlihat | Endpoint agregat (§2); tidak ada jalur lain yang tersedia karena list berpaginasi |
| **`version` lupa dibawa form** → `409` di setiap penyimpanan kedua | §6 + test (§9) |
| **Endpoint baru diminta di tengah phase UI** | Tabel §7 mengunci permukaan di muka; permintaan baru = periksa dulu, bukan langsung tambah |
| **Cakupan layar membengkak** (grafik, realtime, dark mode) | PRD *Di luar cakupan* menyebut ketiganya eksplisit |

---

## 11. Yang berubah pada dokumentasi

| Berkas | Perubahan |
|---|---|
| [`api.md`](../../architecture/api.md) | Dua endpoint metrik; catatan CORS pada bagian konvensi |
| [`authentication.md`](../../architecture/authentication.md) | Bagian klien dashboard: cookie tak terbaca, CSRF double-submit dari sisi klien, refresh single-flight |
| [`authorization.md`](../../architecture/authorization.md) | Baris `metrics.read` pada matriks |
| [`multi-tenancy.md`](../../architecture/multi-tenancy.md) | Entri harness isolasi untuk endpoint metrik |
| `STATUS.md` | C1 dicoret dari *Keputusan Belum Diambil*; Phase 3 berjalan |
| [`product/glossary.md`](../../product/glossary.md) | Bahasa UI: "belum diputuskan" → Indonesia |
| `phases/03-owner-dashboard/notes.md` | Satu bagian per issue (dibuat saat issue pertama dikerjakan) |

---

## 12. Kewajiban yang diteruskan ke phase berikutnya

Ditulis di sini agar tidak bergantung pada ingatan:

> **Phase 4 (API Publik)** menambah layar manajemen API key ke dashboard ini. Bentuk daftar + aksi
> revoke sudah punya preseden di layar undangan (#34) — ikuti itu, jangan rancang ulang.
>
> **Phase 5 (Mobile)** memakai jalur autentikasi yang sama dengan dashboard (Aturan #24), tetapi
> menyimpan token di secure storage, bukan cookie. Yang **tidak** boleh ikut disalin: mekanisme CSRF
> double-submit (§4.1) tidak relevan di mobile — ia ada khusus karena cookie terkirim otomatis oleh
> browser.
>
> **Angka revenue** tetap absen dari metrik sampai Deal ada (pasca-Phase 5). Freeze 3.2 mencatatnya
> sebagai ekspektasi yang disepakati; jangan menambahkan kolom uang ke `internal/metrics` lebih awal
> (Aturan #14, #28).
