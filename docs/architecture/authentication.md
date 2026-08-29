# Authentication

> Sumber: `freeze.md` bagian 5.1 (Aturan #24, #25) · `docs/phases/01-auth-organization/td.md` §2, §4, §5, §6, §11, §12, §13 · Issue #10.
> Dibuat saat issue #10 dikerjakan, sesuai TD §16.

---

## Tiga jalur, satu prinsip

```
User session   → Dashboard, Mobile      → JWT + refresh token opaque → principal: membership
API key        → Sistem eksternal       → jln_live_...                → principal: organization
Public form key→ Browser pengunjung     → public_key di path          → principal: form            (Phase 6)
```

**Tidak ada satupun yang boleh menggantikan yang lain** (Aturan #24, `multi-tenancy.md`). Dokumen ini hanya membahas jalur pertama — user session — yang mencakup dashboard **dan** mobile. Keduanya memakai kredensial yang sama; yang berbeda hanya cara token dikirim (lihat di bawah) dan masa berlaku refresh token.

---

## Access token — JWT

`golang-jwt/jwt/v5`, HS256. Satu service menerbitkan dan memverifikasi — asimetris tidak memberi manfaat di sini.

```json
{
  "sub": "<user_id>",
  "org": "<organization_id>",
  "mem": "<membership_id>",
  "role": "owner|admin|manager|employee",
  "iat": 1755500000,
  "exp": 1755500900,
  "jti": "<uuidv7>"
}
```

`organization_id` ada **di dalam token**, tidak pernah dikirim client — penegakan Aturan #5 di level kriptografi. TTL default `15m` (`ACCESS_TOKEN_TTL`).

Implementasi: `internal/shared/accesstoken` — pembungkus tipis, satu consumer (`internal/auth.Usecase`). `Usecase.ParseAccessToken` adalah satu-satunya cara handler menyentuh klaim token; `JWT_SECRET` sendiri tidak pernah keluar dari `Usecase`.

---

## Refresh token — opaque, rotasi, deteksi penggunaan ulang

| Aspek | Ketentuan |
|---|---|
| Bentuk | 32 byte `crypto/rand`, base64url — dibuat lewat `internal/shared/token.Generate()`, sama seperti token verifikasi email dan reset password |
| Disimpan | SHA-256 di `refresh_tokens.token_hash` |
| TTL | `720h` (dashboard) / `2160h` (mobile) — `REFRESH_TOKEN_TTL_DASHBOARD` / `REFRESH_TOKEN_TTL_MOBILE` |
| Rotasi | Setiap `/v1/auth/refresh` menerbitkan pasangan baru, menyetel `replaced_by_id` pada baris lama |
| `family_id` | Menghubungkan setiap token yang lahir dari satu login yang sama |

```
Refresh diterima
  ├── token tidak ditemukan / kedaluwarsa   → 401 invalid_credentials
  ├── token sudah punya replaced_by_id
  │   atau revoked_at                        → 🚨 revoke SELURUH family_id
  │                                             + audit auth.refresh_reused + 401
  └── token valid                            → terbitkan pasangan baru, rotasi
```

Token yang sudah dirotasi lalu dipakai lagi berarti salah satu pihak memegang salinan curian — karena tidak mungkin tahu yang mana, seluruh family dicabut dan keduanya dipaksa login ulang.

Rotasi berjalan di dalam satu transaksi dengan `SELECT ... FOR UPDATE` pada baris token (`RefreshTokenRepository.FindByHashForUpdate`) — dua refresh bersamaan pada token yang sama harus berurutan, tidak boleh keduanya melihat state "belum dirotasi" dan sama-sama berhasil.

`RefreshTokenRepository.FindByHashForUpdate` dan `RevokeAllByUserID` adalah pengecualian tertulis terhadap pola "repository tenant-scoped selalu menerima `tenant.Context`" (TD §8): yang pertama karena organisasi baru diketahui *dari* hasil lookup itu sendiri, yang kedua karena reset password sengaja mencabut sesi di **semua** organization milik user, bukan satu.

---

## API key — lookup, verifikasi, revoke (Phase 4)

Jalur kedua dari tiga di atas — kredensial `jln_live_<key_id>_<secret>` yang mengautentikasi sistem
eksternal pelanggan, bukan orang. Format lengkap dan siklus hidupnya (buat/cabut) ada di
[ADR-004](../decisions/ADR-004-api-key-format.md) dan `api.md` bagian *API Publik*; bagian ini hanya
membahas **verifikasi saat request masuk**, sisi yang sama dengan "Refresh token" di atas tapi untuk
kredensial yang berbeda bentuknya.

```
Authorization: Bearer jln_live_...
  ├── parse gagal (bentuk tidak valid)         ┐
  ├── key_id tidak ditemukan                    ├─→ 401 invalid_api_key — PESAN YANG SAMA
  ├── revoked_at terisi                         │   (membedakan penyebabnya membocorkan
  ├── expires_at sudah lewat                    │   bahwa key_id itu pernah ada, Aturan #6)
  ├── secret tidak cocok (constant-time)        ┘
  └── seluruhnya lolos                          → tenant.Context{PrincipalType: api_key,
                                                    OrganizationID, APIKeyID, Scopes}
                                                    — Role/MembershipID/UserID kosong
```

`internal/apikey.Usecase.ResolveAPIKey` (bukan `internal/auth`) yang menjalankan alur ini —
`internal/shared/authn.APIKeyResolver` adalah interface consumer (ADR-011) yang dipenuhinya secara
struktural, sama pola dengan `ClaimsParser` yang membuat `authn` tidak pernah mengimpor `internal/auth`.

Lookup `key_id` (`Repository.FindByKeyID`) **tidak** menerima `tenant.Context` — pengecualian
terdokumentasi yang sama seperti `RefreshTokenRepository.FindByHashForUpdate` di atas: organization
adalah *hasil* lookup ini, bukan sesuatu yang sudah diketahui sebelumnya (Aturan #5).

### Dipasang hanya di satu route

`authn.MiddlewareWithAPIKey` dipasang **hanya** di `POST /v1/leads` (`cmd/api/main.go`). Setiap route
lain tetap memakai `authn.Middleware` biasa, yang tidak mengenali prefix `jln_*` sama sekali — sebuah
kredensial API key yang dikirim ke route lain gagal parsing JWT dan ditolak `401
authentication_required`, **tidak pernah** sampai ke lapisan otorisasi. Ini bentuk Aturan #24 yang
paling kuat: bukan keputusan authz yang bisa saja salah ditulis, melainkan endpoint yang secara harfiah
tidak mengerti kredensial itu ada.

### Kenapa tidak ada CSRF di jalur ini

CSRF melindungi kredensial yang **dikirim otomatis oleh browser** (cookie). Kredensial API key tidak
pernah begitu — ia harus ditulis eksplisit ke header `Authorization` oleh kode integrator, sama seperti
alasan jalur mobile (`Authorization: Bearer` JWT) juga dikecualikan di bagian *CSRF* di bawah. Tidak ada
yang bisa "menumpang" pada request lintas situs karena tidak ada apa pun yang terkirim tanpa
sepengetahuan pengirimnya.

### `last_used_at` — di-throttle, bukan presisi

Diperbarui paling sering sekali per 5 menit per kunci (peta in-memory di `apikey.Usecase`, tanpa
eviction — utang yang sama bentuknya dengan `ratelimit.FixedWindow` sejak #9) — menuliskannya di setiap
request akan membuat `api_keys` jadi *write hotspot* ([ADR-004](../decisions/ADR-004-api-key-format.md)
aturan #3). Dashboard merender ini sebagai perkiraan ("sekitar N menit lalu"), tidak pernah jam presisi.

---

## Dashboard (cookie) vs Mobile (bearer)

```
POST /v1/auth/login  { email, password, client: "dashboard" | "mobile" }
```

| `client` | Access + refresh token | CSRF token |
|---|---|---|
| `dashboard` | Cookie `HttpOnly; Secure; SameSite=Lax` — **body tidak memuat token sama sekali** | Cookie `csrf_token`, **bukan** `HttpOnly` |
| `mobile` | Body JSON — **tidak ada `Set-Cookie` sama sekali** | Tidak berlaku (lihat CSRF di bawah) |

Ini membuat Aturan #25 struktural: dashboard secara harfiah tidak pernah melihat token dalam bentuk yang bisa ditaruh di `localStorage`.

`GET /v1/me` dan endpoint terautentikasi lain menerima **kedua** bentuk kredensial (`Authorization: Bearer` atau cookie `access_token`) lewat `internal/auth.AuthMiddleware` — bearer diprioritaskan bila keduanya hadir.

`/v1/auth/refresh` dan `/v1/auth/logout` membaca refresh token dari cookie dulu, baru fallback ke body — client menentukan jalurnya sendiri secara implisit lewat mana yang ia kirim.

### Klien dashboard — cookie tak terbaca, refresh single-flight

`crm_dashboard` (Next.js, Phase 3 #31) tidak punya `middleware.ts` yang membaca token — `access_token` `HttpOnly` secara harfiah tidak bisa disentuh JavaScript. Satu-satunya penjaga route adalah memanggil `GET /v1/me` di layout terproteksi (`SessionGate`, `src/lib/session-context.tsx`); gagal (401 yang bertahan setelah refresh) → redirect `/login`.

`src/lib/api-client.ts`'s `apiFetch` menegakkan dua hal di sisi klien:

- **CSRF**: header `X-CSRF-Token` dibaca dari cookie `csrf_token` (non-`HttpOnly`, lihat di bawah) dan disertakan di setiap request non-GET.
- **Refresh single-flight**: satu `refreshPromise` modul-level ditetapkan **sinkron**, sebelum `await` apa pun — request paralel yang sama-sama menerima `401` memakai ulang Promise yang sama alih-alih masing-masing memanggil `/v1/auth/refresh` sendiri. Tanpa ini, N request paralel yang kedaluwarsa bersamaan memicu N rotasi refresh token yang saling balapan — hasilnya salah satu dicabut karena "sudah dirotasi" (§ di atas) walau pengguna tidak melakukan apa pun yang salah. Dibuktikan lewat test konkurensi asli (`src/lib/api-client.test.ts`), bukan diasumsikan dari membaca kode.

---

## CSRF — double-submit

Konsekuensi wajib dari cookie (Aturan #25): karena cookie dikirim otomatis oleh browser, endpoint yang menerima autentikasi lewat cookie wajib diproteksi CSRF.

| Aspek | Ketentuan |
|---|---|
| Pola | Double-submit token |
| Cookie | `csrf_token` — **bukan** `HttpOnly`, agar JavaScript bisa membacanya |
| Header | `X-CSRF-Token` wajib pada request non-GET yang terautentikasi **lewat cookie** |
| Perbandingan | `subtle.ConstantTimeCompare` (`httpx.VerifyCSRF`) |
| Dikecualikan | Request yang terautentikasi lewat `Authorization: Bearer`, atau refresh/logout yang membawa token lewat body (mobile) — kredensial itu tidak pernah dikirim otomatis oleh browser, jadi tidak ada yang bisa dinaiki request lintas situs |

`internal/shared/httpx.VerifyCSRF` adalah generik (cookie vs header, tidak tahu domain). Yang memutuskan **kapan** ia dipanggil bersifat lokal per titik: `AuthMiddleware` untuk endpoint di belakangnya, handler `refresh`/`logout` untuk keduanya secara langsung — karena hanya masing-masing yang tahu dari mana kredensialnya berasal pada request itu.

---

## Login dengan >1 membership (ADR-007)

```
POST /v1/auth/login tanpa organization_id, user punya >1 membership aktif
  → 409 organization_selection_required + daftar organization (lihat api.md)

Client memanggil ulang dengan organization_id terisi
  → organization_id yang tidak cocok salah satu membership → 401 invalid_credentials
    (bukan error terpisah — mencegah probing keanggotaan organization)
```

---

## Reset password

`POST /v1/auth/password/forgot` selalu 202 (anti-enumerasi, TD §6.3). `POST /v1/auth/password/reset` mengonsumsi token satu-pakai (`password_reset_tokens`, tabel terpisah dari `email_verification_tokens` meski bentuknya sama — TD §1) dan **mencabut seluruh refresh token milik user, lintas organization** (`RevokeAllByUserID`) — password bocor berarti setiap sesi yang ada menjadi mencurigakan, bukan hanya sesi di perangkat tempat reset terjadi.

---

## Pengiriman email (Phase 4.6)

Verifikasi email, reset password, dan undangan tim (`internal/invitation`) semuanya lewat satu
interface, `mailer.Mailer` — sudah ada sejak Phase 1, sengaja dibuat lebih awal karena implementasi
kedua (provider sungguhan) sudah diketahui akan datang.

| Provider | `MAIL_PROVIDER` | Dipakai untuk | Catatan |
|---|---|---|---|
| `LogMailer` | `log` | Test otomatis saja | Mencatat pesan ke logger, **tidak pernah dikirim**. **Ditolak boot saat `APP_ENV=production`** — proses yang tidak pernah mengirim email gagal sepenuhnya diam-diam (tidak ada crash, tidak ada error, hanya funnel registrasi yang berhenti bekerja), dan `LogMailer` juga menulis token verifikasi — kredensial sekali-pakai — ke log (Aturan #26) |
| `SMTPMailer` | `smtp` | Development (→ Mailpit) dan produksi | Satu-satunya provider yang benar-benar mengirim |

### `SMTPMailer` — kenapa bukan `smtp.SendMail`

`smtp.SendMail` (stdlib) memakai `net.Dial` telanjang — **tidak ada batas waktu sama sekali**. Setiap
pemanggil `mailer.Send` (`auth.Usecase.sendVerificationEmail`, `…sendPasswordResetEmail`,
`invitation.Usecase.sendInvitationEmail`) memanggilnya **sinkron di jalur request**, sesudah commit
(Aturan #32) tapi masih di dalam handler HTTP yang sama. Server SMTP yang menggantung berarti request
HTTP ikut menggantung.

`SMTPMailer` karena itu membangun percakapannya sendiri: `net.Dialer{Timeout: ...}.DialContext` untuk
dial, lalu satu `conn.SetDeadline` yang menutup **seluruh** sisa percakapan (STARTTLS, AUTH, MAIL
FROM, RCPT TO, DATA, QUIT) — bukan batas waktu per operasi. Server yang menjawab satu byte per detik
tetap terputus tepat waktu, bukan lolos operasi demi operasi.

### Kenapa kegagalan kirim tidak membatalkan apa pun

Freeze bagian 6 (Aturan #32) sudah memutuskan ini sebelum Phase 1: kirim email **selalu setelah**
commit, tidak pernah di dalam transaksi yang sama. Konsekuensinya, kegagalan `Send` **tidak pernah**
membatalkan pendaftaran/undangan yang sudah tersimpan — ia dicatat sebagai error terstruktur
(`u.logger.Error("failed to send ...", "err", err, "to", email)`) dan pemulihannya adalah tombol
**kirim ulang**, yang sudah ada sejak Phase 1 untuk verifikasi email dan sejak awal untuk undangan.
Password SMTP tidak pernah muncul di pesan error ini — kegagalan `AUTH` dikembalikan sebagai pesan
polos (`"mailer: smtp authentication failed"`), bukan error asli dari `net/smtp`, yang bisa
meng-echo detail dari respons server.

### Development — Mailpit

`docker-compose.yml`'s service `mailpit` menangkap setiap email `make dev` kirim, tanpa benar-benar
mengirimkannya ke internet — dibaca lewat UI web-nya di `http://localhost:8025`. `api` **tidak**
menunggu `mailpit` sehat sebelum boot (tidak ada `depends_on`) — SMTP hanya disentuh saat ada email
yang benar-benar dikirim, bukan saat boot, jadi container yang lambat naik tidak pernah memblokir
`docker compose up` menyajikan traffic.

### Produksi

Domain pengirim, SPF/DKIM/DMARC **tidak** disentuh oleh `SMTPMailer` — itu pekerjaan DNS di luar
repository, tercatat sebagai item *Punya Lead Time* di `docs/STATUS.md`. Tanpanya, email yang
terkirim secara teknis benar bisa tetap berakhir di folder spam. `SMTP_TLS=none` ditolak saat
`APP_ENV=production` (Aturan #36) — kredensial dan token verifikasi tidak boleh melintasi jaringan
produksi tanpa enkripsi.

---

## Rate limiting

| Endpoint | Batas | Implementasi |
|---|---|---|
| `login` | per IP **dan** per email, backoff progresif, tanpa lockout permanen | `internal/auth.LoginLimiter` — tipe khusus, bukan `ratelimit.Limiter`, karena perlu tahu sukses vs gagal untuk menaikkan/mereset backoff |
| `register`, `verify-email/resend`, `password/forgot` | Flat window per IP dan/atau per email | `ratelimit.FixedWindow` |

Angka-angka di kode adalah default konservatif, bukan hasil tuning. **Sebagian ditutup di Phase 4**:
batas `POST /v1/leads` jalur API key ditetapkan (`PUBLIC_API_RATE_LIMIT`, `api.md` bagian *Rate
limiting*) — tapi baris di atas (`login`, `register`, dst.) belum ikut ditinjau, masih default sejak
#9. Dua mekanisme berbeda, dan hanya satu yang sudah dipertimbangkan ulang.

Setiap baris di tabel di atas ber-key `c.ClientIP()`. Sebelum Phase 4.5, key itu bisa dipalsukan
lewat header — lihat bagian *Model kepercayaan jaringan* di bawah. Angka yang benar tidak berarti
apa-apa bila key-nya bisa dipilih penyerang.

## Model kepercayaan jaringan (Phase 4.5)

`c.ClientIP()` (Gin) bukan fakta jaringan — ia adalah **isi header**, dan hanya benar bila diminta
percaya dari sumber yang tepat. Tanpa konfigurasi eksplisit, Gin 1.x mempercayai **setiap** peer
sebagai proxy (`trustedProxies: ["0.0.0.0/0", "::/0"]`), sehingga siapa pun bisa mengatur
`X-Forwarded-For: 1.2.3.4` dan `ClientIP()` akan memercayainya begitu saja. Setiap baris di tabel
*Rate limiting* di atas bergantung pada `ClientIP()` — dan sebelum Phase 4.5, **Aturan #34 secara
faktual tidak ditegakkan** karena inilah yang terjadi.

### Konfigurasi

`TRUSTED_PROXIES` (`internal/shared/config`) — daftar CIDR/IP dipisah koma, atau literal `none`
untuk "tidak ada proxy di depan proses ini, pakai alamat peer koneksi langsung". **Wajib diisi**
saat `APP_ENV=production` (Aturan #36) — boot gagal tanpanya, pola yang sama seperti
`CORS_ALLOWED_ORIGINS` (#30).

```go
r := gin.New()
r.SetTrustedProxies(cfg.TrustedProxyCIDRs())   // sebelum middleware apa pun
r.Use(httpx.RequestID(), httpx.Logging(log), ...)
```

Dipasang di `cmd/api/main.go`'s `newRouter`, **sebelum** middleware apa pun — `httpx.Logging` sendiri
membaca `ClientIP()`, jadi ia harus melihat konfigurasi yang benar sejak baris pertama.

### Bagaimana `ClientIP()` memutuskan

```
Request masuk
     │
     ▼
Peer (RemoteAddr) ada di TRUSTED_PROXIES?
     │                              │
    Ya                             Tidak
     │                              │
     ▼                              ▼
Baca X-Forwarded-For          Abaikan header —
(header terakhir yang         ClientIP() = RemoteAddr
belum tepercaya, dari         (alamat peer koneksi
kanan ke kiri)                 langsung)
     │
     ▼
ClientIP() = alamat itu
```

Konsekuensinya: mempercayai **sebagian** proxy tidak berarti mempercayai **semua** koneksi. Peer di
luar `TRUSTED_PROXIES` tetap diperlakukan sebagai klien langsung — headernya diabaikan sepenuhnya,
bukan dipercaya sebagian.

### Dua kesalahan konfigurasi, dua gejala berlawanan

| Kesalahan | Gejala | Kenapa buruk |
|---|---|---|
| `TRUSTED_PROXIES` **terlalu longgar** (atau tidak diisi — default Gin) | `ClientIP()` bisa dipalsukan lewat header | Rate limit per-IP bisa dilewati siapa pun (Aturan #34) |
| `TRUSTED_PROXIES=none` **padahal ada** load balancer di depan | Semua pengguna terlihat berasal dari satu IP (alamat LB) | Semua pengguna berbagi satu jatah rate limit — pengguna keenam diblokir tanpa sebab, bukan penyerang yang diblokir |

Tidak ada nilai default yang benar untuk keduanya sekaligus — itu sebabnya ia wajib dinyatakan, bukan
diberi default diam-diam. Nilai lokal (`.env.example`, `docker-compose.yml`) adalah `none`: `api`
diekspos langsung ke host, tidak ada proxy di depannya. Nilai produksi ditentukan saat hosting
dipilih.

**Dibuktikan bisa gagal** (freeze bagian 5, lapis 4) — `cmd/api/trusted_proxy_test.go` mencoba
melewati Aturan #34 secara aktif (header dipalsukan, IP diganti tiap request) alih-alih hanya
membaca kode. Prosedurnya sama seperti harness isolasi tenant: `SetTrustedProxies` dilepas sementara
→ lima dari enam test merah → dikembalikan. Detail di `docs/phases/04.5-hardening/notes.md`.
