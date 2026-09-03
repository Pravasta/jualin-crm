# API Conventions

> Sumber: `freeze.md` bagian 5.2 (Aturan #33) · 5.1 (Aturan #24)
>
> **Dikunci sejak bootstrap, bukan Phase 4.** Konvensi ini terbentuk secara de facto di Session 3 dan Session 5 — bila baru ditulis di Phase 4, ia hanya akan mendokumentasikan apapun yang kebetulan terbentuk. Begitu Next.js dan Flutter menempel, mengubahnya berarti memutus dua client sekaligus.

> **Berkas ini bukan katalog rute** (diputuskan di issue #98). Isinya **konvensi** — envelope, error,
> pagination, autentikasi, rate limit — plus satu bab per **jalur publik** yang menghadap klien di luar
> kendali kita (API Publik Phase 4, Formulir Publik Phase 6). Daftar endpoint sebuah phase hidup di
> `docs/phases/<NN>-<slug>/td.md` §8, dan kebenaran terakhirnya ada di `RegisterRoutes` masing-masing
> paket (Aturan #30).
>
> Dicatat karena sudah **dua kali** sebuah TD meminta "tambahkan endpoint ke daftar endpoint" di berkas
> yang tidak punya daftar itu — TD Phase 5 §15 (device-tokens, dilaporkan di
> `docs/issues/073-tasks-fcm-notifications.md`), dan berpotensi terulang. TD berikutnya cukup menulis
> daftarnya di §8-nya sendiri; yang masuk ke sini hanya **konvensi baru** atau **jalur publik baru**.

---

## Versioning

Prefix path `/v1/` pada **seluruh** endpoint, termasuk yang hanya dipakai internal.

```
/v1/auth/register
/v1/auth/login
/v1/leads
/v1/leads/{id}
/v1/metrics/summary
/v1/metrics/employees
```

Murah sekarang, mustahil ditambahkan setelah ada integrator.

---

## Payload

| Hal | Ketentuan |
|---|---|
| Penamaan field | `snake_case` — konsisten dengan kolom database |
| Format waktu | ISO 8601 UTC dengan sufiks `Z` — `2026-08-17T09:30:00Z` |
| ID | UUIDv7 sebagai string |
| Uang | String desimal (`"1500000.00"`) + field `currency` terpisah. **Jangan float di JSON.** |
| Field tak dikenal pada request | **Diabaikan**, bukan ditolak — agar client lama tidak rusak saat field baru ditambahkan |

> Field tak dikenal pada `POST /v1/leads` tetap tersimpan di `leads.raw_payload`.

---

## Response — sukses

### Single resource

```json
{
  "data": {
    "id": "01931f2e-8c4a-7...",
    "lead_number": 1024,
    "name": "Budi Santoso",
    "status": "new",
    "created_at": "2026-08-17T09:30:00Z"
  }
}
```

### List

```json
{
  "data": [ ... ],
  "meta": {
    "page": 1,
    "per_page": 25,
    "total": 143
  }
}
```

**Envelope `data`/`meta` sejak awal.** Mengembalikan array telanjang terasa lebih bersih, tapi menambahkan metadata pagination belakangan berarti mengubah tipe akar response — breaking change untuk setiap client. Envelope adalah biaya satu kali yang sangat kecil.

---

## Response — error

```json
{
  "error": {
    "code": "lead_not_found",
    "message": "Lead tidak ditemukan.",
    "details": [
      { "field": "email", "code": "invalid_format" }
    ]
  }
}
```

| Field | Kontrak |
|---|---|
| `code` | **Stabil, machine-readable, `snake_case`.** Client tidak pernah mem-parsing `message`. Mengubah `code` = breaking change. |
| `message` | Untuk manusia. **Boleh berubah** tanpa dianggap breaking. |
| `details` | Opsional. Untuk error validasi per-field. |

### Katalog error code

Bertambah seiring fitur. Setiap kode baru dicatat di sini.

| HTTP | `code` | Kapan |
|---|---|---|
| 400 | `validation_failed` | Bentuk request salah; `details` berisi per-field |
| 401 | `unauthenticated` | Tidak ada kredensial, atau kredensial invalid/kedaluwarsa |
| 403 | `forbidden` | Terautentikasi, tapi role tidak mengizinkan |
| 404 | `not_found` | Resource tidak ada — **atau milik tenant lain** |
| 409 | `conflict` | Bentrok state umum |
| 409 | `version_conflict` | Optimistic locking gagal (Aturan #35); body memuat keadaan terkini |
| 409 | `email_already_registered` | Registrasi dengan email yang sudah ada |
| 422 | `invalid_status_transition` | Transisi status lead tidak diizinkan |
| 429 | `rate_limited` | Melebihi batas; sertakan `Retry-After` |
| 500 | `internal_error` | Tidak pernah membocorkan detail internal |
| 401 | `invalid_credentials` | Login gagal — email atau password salah, atau refresh token tidak valid/kedaluwarsa/sudah dirotasi (issue #10) |
| 403 | `email_not_verified` | Login dengan akun yang belum verifikasi email (issue #10) |
| 409 | `organization_selection_required` | Login: user punya >1 membership aktif dan belum memilih organization — lihat perluasan envelope di bawah (issue #10) |
| 400 | `invalid_token` | Token verifikasi/reset/undangan tidak valid atau kedaluwarsa (issue #9, #10) |
| 401 | `authentication_required` | Endpoint terautentikasi diakses tanpa kredensial valid (issue #10) |
| 403 | `csrf_token_invalid` | Request cookie non-GET tanpa `X-CSRF-Token` yang cocok (issue #10) |
| 409 | `last_owner_cannot_be_removed` | Owner terakhir organization mencoba menonaktifkan dirinya sendiri (issue #11) |
| 409 | `invitation_already_accepted` | Undangan yang tokennya valid tapi sudah pernah diterima (issue #11) |
| 422 | `invalid_activity_type` | Client mengirim tipe activity sistem (`lead_created`, `status_changed`, dst.) ke `POST /v1/leads/{id}/activities` (issue #21) |
| 409 | `membership_has_open_leads` | Penonaktifan membership ditolak karena masih ada lead terbuka; body memuat `open_lead_count` (issue #22) |
| 409 | `lead_already_converted` | `POST /v1/leads/{id}/convert` pada lead yang sudah pernah dikonversi — ditegakkan `uq_customers_org_lead` (issue #23) |
| 401 | `invalid_api_key` | Kredensial `jln_*` tidak dikenal, sudah direvoke, atau kedaluwarsa — ketiganya pesan yang sama (issue #47) |
| 403 | `insufficient_scope` | Principal `api_key` mencoba aksi di luar scope-nya — termasuk mengirim `assigned_to_membership_id` ke `POST /v1/leads` (issue #47) |
| 413 | `payload_too_large` | Body `POST /v1/leads` jalur API key melebihi 64 KB (issue #47) |
| 403 | `origin_not_allowed` | Header `Origin` di luar `forms.allowed_origins` — termasuk saat header itu tidak dikirim sama sekali, dan saat allowlist-nya sendiri kosong (issue #87) |
| 400 | `form_token_invalid` | Token time-trap salah tanda tangan, terlalu cepat (<2 detik), kedaluwarsa (>30 menit), atau untuk `form_id` lain — keempatnya pesan yang sama (issue #87) |
| 400 | `captcha_failed` | Turnstile menolak, token tidak dikirim, atau verifikasi ke Cloudflare sendiri gagal — ketiganya jalur yang sama, gagal tertutup (issue #87) |
| 400 | `webhook_url_not_allowed` | URL webhook menunjuk alamat privat/loopback/link-local, skema bukan `http(s)`, atau DNS tidak bisa diresolusi — **satu kode untuk semua alasan**, membedakannya memberi pelanggan alat memetakan jaringan internal kita (issue #100) |
| 409 | `delivery_not_retryable` | `POST /v1/webhook-deliveries/:id/retry` dipanggil untuk pengiriman yang statusnya bukan `failed` (issue #100) |

> `invalid_activity_type` dan `membership_has_open_leads` seharusnya masuk katalog ini saat issue #21
> dan #22 selesai ("setiap kode baru dicatat di sini") — luput saat itu, ditambahkan di sini saat
> mengerjakan #23 sambil menambah `lead_already_converted`. Dicatat sebagai celah yang tertangkap, bukan
> diperbaiki diam-diam (Aturan #30).

### Perluasan envelope — `organization_selection_required`

TD phase 1 §6.2 mengizinkan pengecualian sadar terhadap bentuk error di atas: respons `409 organization_selection_required` membawa field tambahan `organizations`, bukan hanya `code`/`message`/`details`.

```json
{
  "error": {
    "code": "organization_selection_required",
    "message": "Pilih organization untuk melanjutkan.",
    "organizations": [{ "id": "01931f2e-...", "name": "Toko ABC" }]
  }
}
```

Ini **satu-satunya** tempat di API ini sebuah error membawa field domain di luar `code`/`message`/`details` — bukan preseden untuk payload error bebas. Alternatif (alur dua langkah dengan token seleksi) menambah tabel dan endpoint untuk kasus yang dialami <1% pengguna (ADR-007); tidak sepadan.

### Aturan 404 vs 403

> Resource milik tenant lain **selalu 404**, tidak pernah 403 (Aturan #6).
>
> 403 mengonfirmasi bahwa resource dengan id tersebut ada — itu kebocoran informasi. 403 hanya untuk resource yang memang milik tenant ini tapi role-nya tidak mengizinkan.

---

## Pagination

Offset: `?page=1&per_page=25`. Default `per_page = 25`, maksimum `100`.

**Kenapa offset, bukan cursor:** offset punya masalah "halaman bergeser" saat data baru masuk di atas. Tapi endpoint list bersifat **internal** (dashboard & mobile) dan bukan bagian API publik Phase 4 — yang hanya `POST /v1/leads`. Jadi ia bisa diganti ke cursor kapan saja tanpa memutus integrator siapapun.

---

## CORS (issue #30)

Browser (dashboard) memanggil API ini langsung, lintas subdomain — keputusan C2, `docs/phases/03-owner-dashboard/prd.md`. `CORS_ALLOWED_ORIGINS` (daftar dipisah koma di `internal/shared/config`) menentukan origin yang diizinkan; **wajib non-kosong saat `APP_ENV=production`** (Aturan #36).

| Ketentuan | Alasan |
|---|---|
| Origin di-echo eksplisit, **tidak pernah `*`** | `Access-Control-Allow-Credentials: true` (dibutuhkan cookie sesi) tidak bisa digabung dengan wildcard origin |
| Origin tak dikenal → lanjut **tanpa** header CORS | Server tidak membocorkan daftar origin lewat penolakan eksplisit; browser yang menolak |
| `OPTIONS` → `204` tanpa menyentuh handler | Preflight tidak membawa kredensial dan tidak boleh menyentuh lapis auth |

Implementasi: `internal/shared/httpx/cors.go`, dipasang di `newRouter` sebelum route manapun dan sebelum `authn.Middleware`. Detail: `docs/phases/03-owner-dashboard/td.md` §1.

---

## Autentikasi — tiga jalur terpisah (Aturan #24)

| Jalur | Dipakai oleh | Kredensial | Principal | Identitas orang? |
|---|---|---|---|---|
| **User session** | Dashboard, **Mobile** | JWT + refresh token opaque | `membership` | ✅ |
| **API key** | Sistem eksternal pelanggan | `Authorization: Bearer jln_live_...` | `organization` | ❌ |
| **Public form key** | Browser pengunjung | `public_key` di path | `form` | ❌ |

**Tidak boleh saling menggantikan.** Mobile memakai user session — jalur yang sama dengan dashboard. Yang berbeda hanya penyimpanan token dan masa berlaku.

Setiap endpoint menyatakan **secara eksplisit** principal apa yang diterimanya. Tidak ada endpoint yang menerima "salah satu dari keduanya" tanpa alasan tertulis.

---

## Idempotency (Phase 4)

```http
POST /v1/leads
Idempotency-Key: <uuid dari client>
```

- Unik per organization (`UNIQUE (organization_id, idempotency_key)`)
- Pengulangan mengembalikan **response asli** — 200 + lead yang sama, **bukan error**
- Retensi 24–48 jam

---

## Rate limiting

| Header | Isi |
|---|---|
| `X-RateLimit-Limit` | Batas per jendela |
| `X-RateLimit-Remaining` | Sisa |
| `X-RateLimit-Reset` | Unix timestamp saat reset |
| `Retry-After` | Detik, hanya pada 429 |

Endpoint yang wajib dibatasi sejak Phase 1: login, kirim ulang verifikasi, lupa password, undangan (Aturan #34).

### Dua kelas rate limit — headernya sengaja berbeda

> **Koreksi (issue #98).** Bagian ini sebelumnya menyatakan keempat header di atas *"dikirim sejak
> versi pertama"*. Itu **tidak pernah benar**: `POST /v1/auth/register` dan `/v1/auth/password/forgot`
> dibatasi sejak Phase 1 tapi tidak pernah mengirim satu pun `X-RateLimit-*` — dibuktikan lewat request
> sungguhan, bukan dari membaca kode. Yang salah adalah dokumen ini, bukan kodenya; alasannya di bawah.

| Kelas | Endpoint | `X-RateLimit-*` | `Retry-After` |
|---|---|---|---|
| **Kredensial publik** — pemanggilnya terautentikasi & diketahui | `POST /v1/leads` (api_key, Phase 4) · `POST /v1/forms/{public_key}/submit` (Phase 6) | ✅ di **setiap** response, termasuk yang gagal | ✅ pada 429 |
| **Anti-abuse auth** — pemanggilnya anonim | register · kirim ulang verifikasi · lupa password | ❌ | ❌ |
| **Anti-abuse auth, backoff progresif** | login | ❌ | ✅ pada 429 |

**Kenapa dibedakan.** Integrator yang memegang API key atau pemilik form yang memasang `public_key`
adalah pihak yang **kita ingin** berperilaku baik — header itu memberi tahu mereka kapan harus
melambat, dan itu tujuannya. Penyerang anonim yang menebak password atau membanjiri endpoint
registrasi adalah pihak yang **tidak** kita bantu: memberi tahu sisa kuotanya sama saja menyerahkan
oracle gratis untuk menyetel serangannya tepat di bawah ambang. Login tetap mengirim `Retry-After`
karena pengguna sah yang salah ketik password berhak tahu kapan bisa mencoba lagi — dan angka itu
tidak membocorkan sisa kuota siapa pun.

**Untuk phase berikutnya:** kredensial baru yang menerima traffic dari luar (webhook **inbound** Phase
7.5, dan seterusnya) masuk **kelas pertama** — `ratelimit.FixedWindow.Take` +
`httpx.SetRateLimitHeaders`, bukan `Allow` tanpa header. Jangan menambah header ke kelas kedua.

> Webhook **keluar** (Phase 7) tidak menambah kelas rate limit apa pun: di sana **kita** yang
> memanggil, bukan yang dipanggil, jadi tidak ada endpoint baru yang menerima traffic. Yang membatasi
> pengiriman keluar adalah kebijakan retry dan batch worker (lihat *Webhook Keluar* di bawah), bukan
> `ratelimit`.

### Angka batasnya belum pernah diukur

Seluruh angka default (`PUBLIC_API_RATE_LIMIT=60`, `FORM_SUBMIT_RATE_LIMIT_IP=20`,
`FORM_SUBMIT_RATE_LIMIT_FORM=60`, dan batas endpoint auth) adalah **tebakan konservatif, bukan hasil
pengukuran** — dinyatakan terbuka sejak Phase 4 (`docs/issues/047-public-lead-api.md`) dan Phase 6
(`docs/issues/087-form-submit-anti-spam.md`).

**Phase 7 menambah angka ke daftar yang sama**, bukan keraguan baru yang terpisah
(`docs/issues/102-webhook-worker.md`):

- `WEBHOOK_MAX_ATTEMPTS=5` (percobaan ulang setelah kiriman pertama), `WEBHOOK_WORKER_INTERVAL=10s`,
  `WEBHOOK_WORKER_BATCH=20`, `WEBHOOK_DELIVERY_TIMEOUT=10s`, `WEBHOOK_DELIVERY_RETENTION_DAYS=30`, dan
  jeda backoff `1m → 5m → 30m → 2j → 6j` — semuanya default konservatif. Bentuknya (`4xx` tidak
  diulang, `5xx` diulang) **bukan** tebakan; angkanya iya.
- **Biaya `DisableKeepAlives: true` pada worker HTTP client belum diukur.** Ia wajib demi keamanan —
  daftar tolak SSRF hanya dievaluasi ulang tiap kirim bila tiap kirim membuka koneksi baru
  (`docs/issues/102`) — tapi membebani setiap pengiriman dengan satu handshake TLS. Kalau ini jadi
  masalah di volume nyata, jalan keluarnya **bukan** menyalakan kembali keep-alive, melainkan
  memindahkan cek deny-list ke tempat yang tetap dievaluasi per-request.

Karena itu semuanya **env-configurable**, dan peninjauannya **satu kali untuk semua** begitu ada
traffic produksi nyata — bukan satu peninjauan per angka. Menambah kredensial baru di phase berikutnya
berarti menambah satu angka ke daftar yang sama, bukan memulai keraguan baru yang terpisah.

Header per-IP di atas hanya berarti sesuatu bila `TRUSTED_PROXIES` dikonfigurasi benar — tanpanya,
`ClientIP()` bisa dipalsukan lewat `X-Forwarded-For` dan seluruh batas per-IP bisa dilewati satu
header. Lihat `authentication.md` bagian *Model kepercayaan jaringan* (Phase 4.5, issue #57).

---

## API Publik (Phase 4)

Satu-satunya endpoint publik: `POST /v1/leads`, dengan principal `api_key` (lihat bagian *Autentikasi*
di atas). Endpoint yang **sama persis** dengan yang dipakai dashboard — bukan salinan, bukan versi
publik terpisah; yang membedakan hanya kredensial dan `tenant.Context` yang dihasilkannya.

### Kredensial

```
jln_live_<key_id:12>_<secret:43>
```

`jln_live_` untuk produksi; `jln_test_` dikenali bentuknya tapi tidak pernah diterbitkan di Phase 4.
Dikirim lewat `Authorization: Bearer <kredensial penuh>` — **tidak pernah** lewat cookie (kredensial ini
tidak boleh hadir di browser sama sekali, Aturan #23).

### Scope

Kredensial API key hanya membawa satu scope: `leads:write`. Ini **satu-satunya** aksi yang bisa
dilakukannya — mencoba memanggil endpoint aplikasi pengguna apa pun (mengelola tim, membuat kunci lain,
membaca lead) selalu berakhir `403 insufficient_scope`, bukan karena setiap handler memeriksanya satu
per satu, melainkan karena `authz.Require` tidak punya jalan untuk mengizinkannya (Aturan #24,
`authorization.md` bagian "Otorisasi berbasis scope").

### Contoh integrasi

Contoh `curl` yang **sungguhan bekerja** hanya bisa dibuat dengan kredensial nyata, dan raw secret hanya
pernah tersedia sekali saat kunci dibuat (Aturan #21) — dokumen statis ini sengaja **tidak** memuat
contoh dengan kredensial palsu yang terlihat berfungsi padahal tidak. Contoh langsung-pakai yang
sungguhan ada di dashboard, `/connect/api/docs`, dan tersambung otomatis dengan kunci yang sedang
Anda lihat.

### Error khusus jalur ini

Ditambahkan ke katalog di atas, dipakai **hanya** oleh jalur API key:

| HTTP | `code` | Kapan |
|---|---|---|
| 401 | `invalid_api_key` | Kredensial `jln_*` tidak dikenal, sudah direvoke, atau kedaluwarsa — ketiganya pesan yang sama, sengaja tidak dibedakan (sama alasannya dengan Aturan #6: membedakan "tidak ada" dari "sudah direvoke" membocorkan bahwa kunci itu pernah ada) |
| 403 | `insufficient_scope` | Kredensial sah tapi scope-nya tidak mencakup aksi ini — termasuk mengirim `assigned_to_membership_id`, yang API key tidak pernah punya cara mengisi secara sah |
| 413 | `payload_too_large` | Body melebihi 64 KB, ditolak sebelum satu byte pun di-parse |

### Idempotency, rate limit — sudah dijelaskan di atas, berlaku penuh di sini

Bagian *Idempotency* dan *Rate limiting* di dokumen ini berlaku apa adanya untuk jalur API key — retensi
`idempotency_key` 48 jam, header `X-RateLimit-*` di **setiap** response jalur ini termasuk yang gagal.
Detail mekanisme (throttle, penghapusan malas) ada di `docs/phases/04-public-api/td.md` §6–§7.

### Field tak dikenal

Body `POST /v1/leads` yang dikirim lewat API key disimpan **apa adanya** di `leads.raw_payload`,
termasuk field yang tidak dikenal sama sekali (bukan hanya field opsional yang dikenal tapi kosong) —
lihat baris "Field tak dikenal pada request" di bagian *Payload* di atas.

---

## Formulir Publik (Phase 6, issue #87)

Satu-satunya endpoint: `POST /v1/forms/{public_key}/submit`, dengan principal `public_form` (lihat
bagian *Autentikasi* di atas). Berbeda dari jalur API key dalam satu hal mendasar: kredensialnya
(`public_key`) **sengaja terbuka untuk semua orang** — ia tertanam di HTML halaman pelanggan (ADR-005).
Yang melindungi endpoint ini bukan kerahasiaan kredensial, melainkan lima lapis anti-abuse di bawah.

### Content-Type — `application/x-www-form-urlencoded`, bukan JSON

Berbeda dari **setiap** endpoint lain di API ini (Aturan #33 tetap berlaku untuk *response*-nya).
Halaman embed (#88) tanpa `CAPTCHA_PROVIDER=turnstile` adalah `<form method="post">` HTML murni, tanpa
JavaScript sama sekali — itu berarti body-nya harus bisa diterima browser secara native, dan browser
tidak bisa mengirim JSON tanpa `fetch`. Keputusan ini diambil eksplisit saat implementasi #87, bukan
default yang kebetulan terjadi.

### Kredensial

```
pk_<22 karakter base64url>
```

Di **path**, bukan `Authorization` — tidak ada middleware auth yang dipasang di route ini; handler
memanggil `form.Usecase.ResolvePublicKey` secara langsung. Setiap kegagalan (kunci tidak ada, form
dihapus) mengembalikan `404 not_found` yang identik — bukan `401`, karena kredensial ini memang bukan
rahasia (lihat `multi-tenancy.md` untuk detail pengecualian unik-lintas-organization-nya).

### Otorisasi — daftar tertutup satu baris

`public_form` hanya bisa melakukan **satu** hal: `lead.create`. Mencoba memanggil endpoint apa pun yang
lain tidak mungkin secara struktural — `authz.Require` menolak berdasarkan **ketiadaan** aksi di peta
`publicFormAllows`, bukan karena setiap handler memeriksanya (pola sama dengan `apiKeyScopeFor`, Aturan
#24). Dibuktikan sebagai tabel atas seluruh `authz.Action`, bukan daftar tulisan tangan.

### Lima lapis anti-abuse (ADR-005, TD Phase 6 §6)

Ditegakkan berurutan — **urutan ini mengikat**, rate limit selalu lebih dulu dari segalanya, termasuk
sebelum body dibaca:

| # | Lapis | Kejujuran tentang kekuatannya |
|---|---|---|
| 1 | Rate limit per IP | `FixedWindow`, `FORM_SUBMIT_RATE_LIMIT_IP` (default 20/menit) |
| 2 | Rate limit per form | `FixedWindow` terpisah, `FORM_SUBMIT_RATE_LIMIT_FORM` (default 60/menit) — dua sumbu independen, bukan satu limiter dicek dua kali |
| 3 | Cap body 32KB | Setengah dari cap `POST /v1/leads` (64KB) — pengunjung anonim tidak pernah butuh lebih |
| 4 | Origin allowlist | `Origin` **bisa dipalsukan** klien non-browser — menghalangi penyalahgunaan biasa, bukan penyerang tertarget |
| 5 | Honeypot | Field tersembunyi CSS; terisi → **sukses palsu**, tidak ada lead. Efektif terhadap bot naif saja |
| 6 | Time-trap (token HMAC) | **Bukan anti-replay** — token sah bisa dipakai berulang dalam jendela 2 detik–30 menitnya; yang membatasi pengulangan adalah rate limit |
| 7 | CAPTCHA (Turnstile, opsional) | Lapis terkuat, satu-satunya yang butuh pihak ketiga — `CAPTCHA_PROVIDER=none` cukup untuk pengembangan |

Honeypot diperiksa **sebelum** time-trap dan CAPTCHA — supaya bot tidak pernah menerima pesan kesalahan
yang bisa dipelajari, dan supaya kuota verifikasi CAPTCHA tidak terbuang ke request yang sudah diketahui
bot. **Konsekuensi yang diterima sadar**: submission honeypot-tripped karena itu bisa berbalas jauh
lebih cepat daripada submission asli yang sampai memanggil Turnstile — celah waktu ini terdokumentasi,
bukan tidak disadari (`docs/phases/06-connect-form/notes.md`'s `## #87`).

### Error khusus jalur ini

Ditambahkan ke katalog di atas, dipakai **hanya** oleh jalur formulir:

| HTTP | `code` | Kapan |
|---|---|---|
| 404 | `not_found` | `public_key` tidak dikenal atau formnya sudah dihapus |
| 403 | `origin_not_allowed` | `Origin` di luar `forms.allowed_origins`, termasuk saat kosong/tidak dikirim |
| 400 | `form_token_invalid` | Token time-trap salah tanda tangan, <2 detik, >30 menit, atau untuk `form_id` lain |
| 400 | `captcha_failed` | Turnstile menolak (bila `CAPTCHA_PROVIDER=turnstile`) |
| 413 | `payload_too_large` | Body melebihi 32 KB |
| 400 | `validation_failed` | Field yang ditandai wajib di `forms.fields` kosong |

### Field tak dikenal, dan yang sengaja tidak tersimpan sebagai kolom

Seluruh field yang dikirim disimpan di `leads.raw_payload` sebagai JSON, **kecuali** tiga field
protokol (honeypot, token time-trap, respons CAPTCHA) — field itu bukan isi submission, tidak pernah
ikut tersimpan. `product` (satu dari enam field tetap ADR-005) tidak punya kolom `leads` sendiri; ia
tetap divalidasi bila ditandai wajib, dan tetap tersimpan di `raw_payload` seperti field tak dikenal
lainnya.

### Penugasan — tidak pernah bisa dikirim

Sama seperti jalur API key: pengunjung situs tidak punya cara mengirim `assigned_to_membership_id` sama
sekali — `SubmitInput` tidak punya field untuk itu, bukan sekadar diabaikan setelah diterima.

### CORS tidak relevan di sini

`CORS_ALLOWED_ORIGINS`/middleware CORS di atas menjaga panggilan `fetch`/XHR dari dashboard — sebuah
`<form method="post">` native browser bukan permintaan yang digerbangi CORS sama sekali (CORS membatasi
JavaScript **membaca** respons lintas-origin, bukan apakah request-nya terkirim). Satu-satunya yang
menjaga siapa boleh submit ke endpoint ini adalah Origin allowlist di atas (lapis #4). Alasan yang
sama persis berlaku untuk `GET /embed/{public_key}` di bawah — iframe `src` adalah navigasi biasa,
bukan permintaan yang digerbangi CORS.

---

## Halaman embed — bukan bagian API ini (Phase 6, issue #88)

`GET /embed/{public_key}` dan `GET /embed.js` **sengaja tidak** didokumentasikan di halaman ini —
keduanya bukan API: tanpa envelope `{data, meta}`, tanpa `Authorization`, respons HTML/JS mentah,
bukan JSON. Ini konsisten dengan TD §7's kalimat sendiri: *"ia bukan API, ia halaman"*. Dokumentasi
lengkapnya (CSP, `frame-ancestors` per-form, `X-Frame-Options` yang sengaja tidak dikirim, escaping
`html/template`, auto-resize D8) ada di
[`docs/phases/06-connect-form/td.md`](../phases/06-connect-form/td.md) §7 dan
[`notes.md`](../phases/06-connect-form/notes.md)'s `## #88`.

---

## Webhook Keluar (Phase 7, issue #100–#104)

Phase pertama di mana **kita yang memanggil**. Semua bagian di atas menerima request dan memutuskan
menolaknya; di sini pelanggan memberi kita URL dan **server kita** yang meneleponnya saat sesuatu
terjadi di Jualin. Konsekuensi keamanannya terbalik arah — lihat *Pertahanan SSRF* di bawah.

Ini **bukan endpoint API**: tidak ada envelope `{data, meta}` di sisi kiriman, karena yang menerima
adalah server pelanggan, bukan kita. Dokumentasi menghadap-penerima (bentuk payload + contoh
verifikasi signature Node/PHP/Python yang langsung bisa dijalankan) ada di dashboard,
`/connect/webhook/docs`, mengikuti preseden `/connect/api/docs` (#49). Bagian ini adalah ringkasan
kontrak; halaman itu adalah sumber lengkapnya.

### Event

Dua, dikunci `webhook.KnownEvents`: `lead.created` dan `lead.status_changed`. Event ketiga kelak =
satu pemanggil baru, bukan mekanisme baru (keputusan D3).

### Bentuk payload

`POST` dengan body JSON. Payload adalah **snapshot yang dibekukan saat event terjadi** (bukan
dibangun ulang saat kirim), sehingga tiga perubahan status dalam lima menit menghasilkan tiga
kiriman berisi tiga keadaan berbeda.

```json
{
  "delivery_id": "0192f0a1-…",
  "event": "lead.created",
  "occurred_at": "2026-09-03T04:21:07.531933Z",
  "organization_id": "0192e0b3-…",
  "data": { "lead": { "…": "bentuk leadJSON yang sama dengan API dashboard" } }
}
```

- `data.lead` memakai **`leadJSON` yang sama** dengan API dashboard — satu bentuk lead di seluruh
  produk, bukan bentuk kedua (`internal/lead/entity.go`'s `Lead.Fields`).
- `lead.status_changed` menambahkan `"changes": {"status": {"from": "new", "to": "contacted"}}` —
  bentuk yang sama dengan `activities.metadata` untuk `status_changed` (#21), bukan bentuk ketiga.
- `delivery_id` stabil lintas percobaan ulang untuk satu kiriman; ia disuntikkan worker saat kirim
  (`= webhook_deliveries.id`), bukan bagian dari snapshot yang disimpan.

### Signature — `X-Jualin-Signature`

```
signed_payload = "<t>" + "." + <body mentah, byte demi byte>
header         = X-Jualin-Signature: t=<unix detik>,v1=<hex HMAC-SHA256(secret, signed_payload)>
```

Secret per endpoint (`whsec_…`), ditampilkan **sekali** saat dibuat, disimpan **terenkripsi**
([ADR-013](../decisions/ADR-013-signing-secret-storage.md), `authentication.md` bagian *Signing
secret webhook*). `t` **ikut ditandatangani** — kalau tidak, request yang tertangkap bisa diputar
ulang selamanya dengan menulis ulang `t`. Toleransi replay 5 menit adalah **penerima yang
menegakkannya**; pengirim tidak. Retry berjam kemudian ditandatangani dengan waktu saat dikirim,
bukan saat di-enqueue, jadi ia tetap berada dalam jendela toleransinya sendiri.

Header lain: `Content-Type: application/json`, `User-Agent: Jualin-Webhook/1`.

### Kebijakan retry

| Respons penerima | Perlakuan |
|---|---|
| `2xx` | `succeeded` |
| `429` | Dicoba ulang |
| `4xx` lain | `failed` **permanen** — "permintaanmu salah" tidak berubah dengan diulang (D5) |
| `3xx` | `failed` permanen — redirect **tidak pernah** diikuti (pertahanan SSRF) |
| `5xx`, timeout, DNS gagal, koneksi ditolak | Dicoba ulang sampai `WEBHOOK_MAX_ATTEMPTS` |

**5 percobaan ulang setelah kiriman pertama** = maksimal 6 panggilan HTTP, jeda `1m → 5m → 30m → 2j
→ 6j`. Angka ini masuk daftar *Angka batasnya belum pernah diukur* di atas.

### Pengiriman `at-least-once`

Crash antara "HTTP terkirim" dan "hasil tercatat" menghasilkan pengiriman **ganda**. Ini keputusan
sadar — `exactly-once` lintas jaringan tidak bisa dicapai tanpa kerja sama penerima. `delivery_id`
yang stabil lintas percobaan adalah alat deduplikasi yang **didokumentasikan ke penerima**, bukan
disembunyikan (TD §4.2).

### Pertahanan SSRF

URL divalidasi **dua kali** (saat disimpan **dan** setiap kali dikirim), terhadap **IP hasil
resolusi**, bukan hostname — validasi sekali bisa dilewati DNS rebinding. Koneksi memakai IP hasil
resolusi itu lewat `DialContext` kustom (`internal/shared/safedial`), bukan menyerahkan hostname ke
`http.Client` (TOCTOU). Redirect **tidak pernah** diikuti. Daftar tolak: loopback, privat,
link-local (`169.254.169.254` — metadata cloud), CGNAT, unspecified, multicast/broadcast/reserved,
IPv4-mapped IPv6. Detail: `docs/phases/07-outbound-webhook/td.md` §3.

### Endpoint pengelolaan (principal user, Owner/Admin)

| Method | Path | `Action` |
|---|---|---|
| `POST` | `/v1/webhook-endpoints` | `webhook.create` |
| `GET` | `/v1/webhook-endpoints` | `webhook.list` — array polos, tidak berpaginasi |
| `GET` | `/v1/webhook-endpoints/:id` | `webhook.read` |
| `PATCH` | `/v1/webhook-endpoints/:id` | `webhook.update` — last-write-wins, tanpa `version` (Aturan #35 tidak mengikat tabel ini) |
| `DELETE` | `/v1/webhook-endpoints/:id` | `webhook.delete` — soft delete |
| `GET` | `/v1/webhook-endpoints/:id/deliveries` | `webhook.read` — berpaginasi (`meta.total`) |
| `POST` | `/v1/webhook-deliveries/:id/retry` | `webhook.update` — hanya sah untuk status `failed`, selain itu `409 delivery_not_retryable` |

`webhook-endpoints` (bukan `webhooks`) supaya `/v1/webhook-deliveries/…` di sebelahnya tidak ambigu.
Matriks role di `authorization.md` bagian *Matriks (Phase 7)*.

### Retensi

`webhook_deliveries` adalah tabel dengan pertumbuhan tercepat di produk (satu baris per lead × per
endpoint × per percobaan). Baris `succeeded`/`failed` yang lebih tua dari
`WEBHOOK_DELIVERY_RETENTION_DAYS` (default 30) dihapus **malas, tanpa scheduler**, di-throttle 1×/jam
dari dalam putaran worker — pola persis retensi `idempotency_key` (#47), termasuk keraguan yang sama:
belum pernah diuji di volume produksi (`docs/issues/047`, `docs/issues/102`). Baris
`pending`/`delivering` tidak pernah dihapus. Aturan #18 (*"Activity & audit log tidak pernah
dihapus"*) **tidak** berlaku — riwayat pengiriman adalah alat diagnosis, bukan catatan audit; yang
bersifat audit (endpoint dibuat/dihapus) tetap masuk `audit_log`.
