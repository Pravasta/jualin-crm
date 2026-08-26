# Authentication

> Sumber: `freeze.md` bagian 5.1 (Aturan #24, #25) · `docs/phases/01-auth-organization/td.md` §2, §4, §5, §6, §11, §12, §13 · Issue #10.
> Dibuat saat issue #10 dikerjakan, sesuai TD §16.

---

## Tiga jalur, satu prinsip

```
User session   → Dashboard, Mobile      → JWT + refresh token opaque → principal: membership
API key        → Sistem eksternal       → jln_live_...                → principal: organization  (Phase 4)
Public form key→ Browser pengunjung     → public_key di path          → principal: form            (Phase 4)
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

## Rate limiting

| Endpoint | Batas | Implementasi |
|---|---|---|
| `login` | per IP **dan** per email, backoff progresif, tanpa lockout permanen | `internal/auth.LoginLimiter` — tipe khusus, bukan `ratelimit.Limiter`, karena perlu tahu sukses vs gagal untuk menaikkan/mereset backoff |
| `register`, `verify-email/resend`, `password/forgot` | Flat window per IP dan/atau per email | `ratelimit.FixedWindow` |

Angka-angka di kode adalah default konservatif, bukan hasil tuning — freeze mencatat "strategi rate limit final" sebagai keputusan terbuka hingga Phase 4.
