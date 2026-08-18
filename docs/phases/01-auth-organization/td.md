# Phase 1 — Auth & Organization · Technical Design

> **Bagaimana.** Apa & kenapa di [`prd.md`](./prd.md).
>
> Memuat **delta** untuk Phase 1. Yang sudah ada di [`freeze.md`](../../architecture/freeze.md), [`multi-tenancy.md`](../../architecture/multi-tenancy.md), [`api.md`](../../architecture/api.md), dan `.claude/skills/jualin-backend/` **tidak diulang** — hanya dirujuk.

---

## 1. Schema — migration `0002_identity`

Spesifikasi kolom lengkap ada di **[freeze bagian 8.3](../../architecture/freeze.md)**. Bagian ini hanya memuat ringkasan dan hal yang freeze belum tetapkan.

| Tabel | Kelas | Catatan |
|---|---|---|
| `organizations` | tenant root | `timezone` default `Asia/Jakarta` |
| `users` | **global** | Tanpa `organization_id`. `email` UNIQUE + `CHECK (email = lower(email))` |
| `memberships` | tenant-scoped | **Jangkar composite FK seluruh database** |
| `subscriptions` | tenant-scoped | Versi minimal: `plan_code='free'`, tanpa mesin limit |
| `invitations` | tenant-scoped | `role <> 'owner'` |
| `email_verification_tokens` | global | Terikat user, bukan org |
| `password_reset_tokens` | global | Bentuk sama, **tabel terpisah** — jangan digabung |
| `refresh_tokens` | session | Terikat membership + organization |
| `audit_logs` | tenant-scoped | Append-only |

### Yang freeze belum tetapkan — diputuskan di sini

| Hal | Keputusan |
|---|---|
| `memberships` index untuk login | `CREATE INDEX ix_memberships_user ON memberships (user_id) WHERE deleted_at IS NULL` — dipakai resolusi membership saat login |
| `refresh_tokens` index | `ix_refresh_tokens_family (family_id)` untuk revoke-family · `uq_refresh_tokens_hash (token_hash)` UNIQUE |
| Token hash | **SHA-256**, bukan argon2 — alasan identik [ADR-004](../../decisions/ADR-004-api-key-format.md): token di-generate `crypto/rand` (256-bit), sehingga hash lambat tidak menambah keamanan tapi memperlambat setiap refresh |
| Panjang token | 32 byte `crypto/rand`, di-encode base64url tanpa padding |
| `users.password_hash` | `NOT NULL`. User hanya dibuat saat registrasi atau penerimaan undangan oleh orang baru — keduanya menyetel password. |

### Larangan yang berlaku pada migration ini

- **Tidak ada** tabel domain CRM (`leads`, dst.) — Phase 2
- **Tidak ada** `UNIQUE(user_id)` pada `memberships` — [ADR-007](../../decisions/ADR-007-user-organization-cardinality.md), disengaja
- Setiap FK ke tabel tenant-scoped **wajib composite** (Aturan #3)

---

## 2. Config — variabel baru

| Variabel | Default | Wajib | Catatan |
|---|---|---|---|
| `JWT_SECRET` | — | ✅ | Minimal 32 byte. Divalidasi saat boot (Aturan #36). |
| `ACCESS_TOKEN_TTL` | `15m` | — | |
| `REFRESH_TOKEN_TTL_DASHBOARD` | `720h` (30 hari) | — | |
| `REFRESH_TOKEN_TTL_MOBILE` | `2160h` (90 hari) | — | Lebih panjang, sesuai freeze 5.6 |
| `APP_BASE_URL` | `http://localhost:3000` | — | Basis tautan di email |
| `MAIL_FROM` | `no-reply@localhost` | — | |
| `MAIL_PROVIDER` | `log` | — | `log` \| (provider sungguhan menyusul) |
| `COOKIE_DOMAIN` | kosong | — | Kosong = host-only cookie, benar untuk localhost |
| `COOKIE_SECURE` | `false` di development | — | **Wajib `true`** saat `APP_ENV=production` — divalidasi saat boot |

> `COOKIE_SECURE=false` di production adalah kesalahan konfigurasi yang mengirim token lewat HTTP polos. Validasi config **menolak** kombinasi itu, konsisten dengan Aturan #36.

---

## 3. Password — argon2id

`golang.org/x/crypto/argon2`.

| Parameter | Nilai |
|---|---|
| Varian | `argon2id` |
| Memory | 19 MiB (`19456`) |
| Iterations | 2 |
| Parallelism | 1 |
| Salt | 16 byte `crypto/rand` |
| Key length | 32 byte |

Disimpan dalam **format PHC**: `$argon2id$v=19$m=19456,t=2,p=1$<salt-b64>$<hash-b64>`

> Parameter ikut tersimpan di dalam string hash, sehingga menaikkannya nanti tidak memerlukan migration — verifikasi membaca parameter dari hash yang tersimpan, dan re-hash saat login berikutnya bila parameternya lebih rendah dari yang sekarang.

**Verifikasi wajib `subtle.ConstantTimeCompare`.**

### Kebijakan password

| Aturan | Nilai |
|---|---|
| Panjang minimum | 12 karakter |
| Aturan komposisi | **Tidak ada** — NIST SP 800-63B |
| Cek daftar bocor (HIBP) | **Ditunda** — butuh panggilan HTTP eksternal; dicatat sebagai kandidat, bukan cakupan Phase 1 |

---

## 4. Token

### Access token — JWT

`github.com/golang-jwt/jwt/v5`, algoritma **HS256** (satu service menerbitkan dan memverifikasi; asimetris tidak memberi manfaat di sini).

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

> `org` dan `mem` ada **di dalam token**, bukan dikirim client. Ini penegakan Aturan #5 di level kriptografi: `organization_id` tidak bisa dipalsukan tanpa memalsukan tanda tangan token.

### Refresh token — opaque

| Aspek | Ketentuan |
|---|---|
| Bentuk | 32 byte `crypto/rand`, base64url |
| Disimpan | SHA-256 di `refresh_tokens.token_hash` |
| Lookup | Langsung by hash (deterministik — tidak perlu pola `key_id` seperti API key) |
| Rotasi | Setiap refresh menerbitkan token baru, menyetel `replaced_by_id` pada yang lama |

### Deteksi penggunaan ulang

```
Refresh diterima
  ├── token tidak ditemukan            → 401
  ├── token kedaluwarsa                → 401
  ├── token sudah punya replaced_by_id
  │   atau revoked_at                  → 🚨 REVOKE SELURUH family_id
  │                                       + audit log + 401
  └── token valid                      → terbitkan pasangan baru, rotasi
```

Token yang sudah dirotasi lalu dipakai lagi berarti **satu dari dua pihak memegang salinan curian**. Karena tidak mungkin tahu yang mana, seluruh family dicabut dan keduanya dipaksa login ulang.

---

## 5. Dashboard vs Mobile — dua cara mengirim token

Freeze mensyaratkan token dashboard di cookie `HttpOnly` (Aturan #25) sementara mobile memakai secure storage. Endpoint-nya sama, jadi request harus menyatakan jenis client — dan `refresh_tokens.client` sudah ada di schema untuk itu.

```
POST /v1/auth/login  { email, password, client: "dashboard" | "mobile" }
```

| `client` | Response |
|---|---|
| `dashboard` | Set cookie `HttpOnly; Secure; SameSite=Lax` untuk access + refresh. **Body tidak memuat token sama sekali.** |
| `mobile` | Token di body. **Tidak ada cookie yang di-set.** |

> Ini membuat Aturan #25 **struktural, bukan imbauan**: dashboard secara harfiah tidak pernah menerima token dalam bentuk yang bisa ditaruh di `localStorage`, karena ia tidak pernah melihatnya.

### CSRF — konsekuensi wajib dari cookie

Karena cookie dikirim otomatis oleh browser, endpoint yang menerima autentikasi cookie **wajib** diproteksi CSRF (Aturan #25).

| Aspek | Ketentuan |
|---|---|
| Pola | Double-submit token |
| Cookie | `csrf_token` — **bukan** HttpOnly, agar bisa dibaca JavaScript |
| Header | `X-CSRF-Token` wajib pada semua request non-GET yang terautentikasi **lewat cookie** |
| Perbandingan | `subtle.ConstantTimeCompare` |
| Dikecualikan | Request yang terautentikasi lewat `Authorization: Bearer` (mobile) — bearer token tidak dikirim otomatis browser, jadi tidak bisa di-CSRF |

> Meski dashboard baru dibangun di Phase 3, cookie sudah **diterbitkan** sejak Phase 1. Menunda CSRF berarti meninggalkan lubang terbuka selama dua phase.

---

## 6. Endpoint

Seluruhnya berprefix `/v1` (Aturan #33). Bentuk response mengikuti [`api.md`](../../architecture/api.md).

### Auth — publik

| Method | Path | Isi |
|---|---|---|
| `POST` | `/v1/auth/register` | `{organization_name, full_name, email, password}` → 201 |
| `POST` | `/v1/auth/verify-email` | `{token}` → 200 |
| `POST` | `/v1/auth/verify-email/resend` | `{email}` → 202 (selalu, lihat catatan enumerasi) |
| `POST` | `/v1/auth/login` | `{email, password, client, organization_id?}` → 200 |
| `POST` | `/v1/auth/refresh` | Cookie atau `{refresh_token}` → 200 |
| `POST` | `/v1/auth/logout` | → 204, mencabut refresh token yang dipakai |
| `POST` | `/v1/auth/password/forgot` | `{email}` → 202 (selalu) |
| `POST` | `/v1/auth/password/reset` | `{token, password}` → 200, **mencabut seluruh sesi user** |

### Auth — terautentikasi

| Method | Path | Isi |
|---|---|---|
| `GET` | `/v1/me` | Profil + membership aktif + organization |

### Invitation

| Method | Path | Role | Isi |
|---|---|---|---|
| `POST` | `/v1/invitations` | owner, admin | `{email, role}` — `role <> owner` |
| `GET` | `/v1/invitations` | owner, admin | Daftar yang masih tertunda |
| `DELETE` | `/v1/invitations/{id}` | owner, admin | Batalkan |
| `GET` | `/v1/invitations/token/{token}` | publik | `{organization_name, email, user_exists}` |
| `POST` | `/v1/invitations/accept` | publik **atau** terautentikasi | Bercabang — lihat 6.1 |

### Membership

| Method | Path | Role | Isi |
|---|---|---|---|
| `GET` | `/v1/memberships` | owner, admin, manager | Daftar anggota organization |
| `PATCH` | `/v1/memberships/{id}` | owner, admin¹ | Ubah role |
| `DELETE` | `/v1/memberships/{id}` | owner, admin¹ | Nonaktifkan (soft delete) |

¹ Admin tidak boleh menyentuh Owner, tidak boleh mengangkat siapapun jadi Owner (freeze 6.2)

### 6.1 Penerimaan undangan — dua cabang (B4)

```
POST /v1/invitations/accept

  ├── email belum punya user   → body: {token, full_name, password}
  │                              buat user (email_verified_at = now) + membership
  │
  └── email SUDAH punya user   → WAJIB terautentikasi (cookie/bearer)
                                 body: {token}
                                 verifikasi: user login == email yang diundang
                                 tambah membership
```

**Larangan mutlak pada cabang kedua:** tidak boleh menerima `password` dan tidak boleh menyetel password. Bila diizinkan, siapapun yang bisa mengundang sebuah alamat email bisa menyetel ulang password pemiliknya — undangan menjadi jalur pengambilalihan akun.

> Ini **wajib punya test keamanan tersendiri**, bukan sekadar dicatat (freeze bagian 7).

### 6.2 Login saat user punya > 1 membership (B2)

```
409 Conflict
{
  "error": {
    "code": "organization_selection_required",
    "message": "Pilih organization untuk melanjutkan.",
    "organizations": [{"id": "...", "name": "..."}]
  }
}
```

Client memanggil ulang `/v1/auth/login` dengan `organization_id` terisi.

> **Perluasan yang disengaja terhadap envelope error di `api.md`**: `error` boleh membawa field domain `organizations` untuk kasus ini. Alternatifnya — alur dua langkah dengan token seleksi berumur pendek — menambah tabel dan endpoint untuk cabang yang, menurut [ADR-007](../../decisions/ADR-007-user-organization-cardinality.md), dialami di bawah 1% pengguna. `api.md` diperbarui saat issue ini dikerjakan.

### 6.3 Mencegah enumerasi email

`register`, `verify-email/resend`, dan `password/forgot` **selalu** mengembalikan response yang sama, terlepas dari apakah email itu ada.

| Endpoint | Perilaku |
|---|---|
| `register` dengan email yang sudah ada | 409 `email_already_registered` — **pengecualian sadar**: registrasi memang harus memberi tahu, atau pengguna terjebak pada form yang gagal diam-diam. Diimbangi rate limit per IP. |
| `verify-email/resend` | 202 selalu. Email hanya benar-benar dikirim bila akunnya ada dan belum terverifikasi. |
| `password/forgot` | 202 selalu. |

---

## 7. Tenant context & middleware

```
RequestID → Logging → Recovery → [Auth] → [CSRF] → [RBAC] → Handler
```

```go
type Context struct {
    OrganizationID uuid.UUID
    PrincipalType  PrincipalType // user | api_key | public_form | system
    MembershipID   *uuid.UUID
    UserID         *uuid.UUID
    Role           Role
    APIKeyID       *uuid.UUID
    RequestID      string
}
```

Di Phase 1 hanya `PrincipalType = user` yang terwujud. Field `APIKeyID` tetap ada agar bentuknya tidak berubah saat Phase 4 — **satu-satunya** future-proofing yang diizinkan di sini, dan biayanya satu field nil.

`organization_id` **selalu** dari klaim token. Tidak ada DTO yang boleh punya field itu (Aturan #5).

---

## 8. Repository — pola yang ditiru seluruh phase berikutnya

```go
func (r *MembershipRepository) FindByID(
    ctx context.Context,
    t tenant.Context,
    id uuid.UUID,
) (*Membership, error)
```

| Aturan | |
|---|---|
| `tenant.Context` **selalu** parameter kedua | Aturan #4 |
| `organization_id` di-inject **di dalam** repository | Bukan diserahkan ke caller |
| Method tanpa tenant **tidak boleh ada** | Bukan sekadar tidak dipakai |
| Resource tenant lain → `ErrNotFound` (404) | Aturan #6 — **tidak pernah** 403 |

**Pengecualian yang harus ditulis eksplisit:** `users` dan tabel token bersifat global. Repository-nya **tidak** menerima `tenant.Context` — dan justru karena itu, setiap method di sana wajib punya komentar yang menyatakan kenapa ia dikecualikan. Pengecualian tanpa alasan tertulis akan disalin ke tempat yang salah.

---

## 9. RBAC

Matriks lengkap: [freeze bagian 6.2](../../architecture/freeze.md).

Implementasi: enum action + fungsi tunggal, **bukan** tabel permission.

```go
authz.Require(t tenant.Context, action Action) error
```

### Empat aturan yang harus punya test tersendiri

| # | Aturan |
|---|---|
| 1 | Employee hanya melihat resource miliknya — ditegakkan di **repository**, bukan handler |
| 2 | Owner terakhir tidak bisa menghapus atau menurunkan dirinya sendiri |
| 3 | Tidak ada yang bisa mengubah role dirinya sendiri |
| 4 | Admin tidak bisa menyentuh Owner, tidak bisa mengangkat Owner |

Aturan #2 dan #3 adalah eskalasi hak akses bila luput — bukan kenyamanan UI.

---

## 10. Email

```go
type Mailer interface {
    Send(ctx context.Context, msg Message) error
}
```

| Implementasi | Kapan |
|---|---|
| `LogMailer` | Development & test — mencatat isi email ke log, tidak mengirim |
| Provider sungguhan | Menyusul saat domain + provider siap (item lead-time di `STATUS.md`) |

> Interface dengan satu implementasi biasanya melanggar Aturan #27. Dibenarkan di sini karena implementasi kedua **sudah pasti datang dan sudah diketahui bentuknya** — dan tanpa interface ini, Phase 1 terblokir menunggu keputusan domain yang berada di luar kendali phase ini.

**Pengiriman selalu setelah commit** (Aturan #32, [ADR-010](../../decisions/ADR-010-fail-fast-startup.md) bagian efek samping). Kegagalan kirim dicatat sebagai error terstruktur dan **tidak** membatalkan operasi yang sudah commit.

Email yang dikirim di Phase 1: verifikasi, kirim ulang verifikasi, undangan, reset password.

---

## 11. Rate limiting

In-memory (freeze: tanpa Redis di MVP), di balik interface agar bisa ditukar.

| Endpoint | Batas |
|---|---|
| `login` | per IP **dan** per email — backoff progresif, **tanpa** lockout permanen |
| `register` | per IP |
| `verify-email/resend` | per email **dan** per IP (Aturan #34) |
| `password/forgot` | per email **dan** per IP (Aturan #34) |
| `invitations` (create) | per organization |

> Lockout permanen ditolak: ia mengubah brute-force menjadi serangan DoS terhadap pengguna yang sah.

Header `X-RateLimit-*` + `Retry-After` dikirim sejak sekarang ([`api.md`](../../architecture/api.md)).

---

## 12. Audit log

Tenant-scoped, append-only. Aksi yang dicatat di Phase 1:

```
user.registered          email.verified
auth.login               auth.logout
auth.refresh_reused      ← 🚨 keamanan
password.reset_requested password.reset_completed
invitation.created       invitation.accepted        invitation.revoked
membership.role_changed  membership.deactivated
```

**Login gagal TIDAK masuk audit log** — ia belum punya tenant, dan `audit_logs.organization_id` adalah `NOT NULL`. Login gagal masuk **log aplikasi** (freeze bagian 7, keputusan default).

---

## 13. Error code baru

Ditambahkan ke katalog di [`api.md`](../../architecture/api.md):

| HTTP | `code` |
|---|---|
| 401 | `invalid_credentials` |
| 403 | `email_not_verified` |
| 409 | `email_already_registered` |
| 409 | `organization_selection_required` |
| 400 | `invalid_token` — verifikasi / reset / undangan tidak valid atau kedaluwarsa |
| 409 | `invitation_already_accepted` |
| 401 | `authentication_required` — cabang undangan untuk user yang sudah ada |
| 403 | `csrf_token_invalid` |
| 409 | `last_owner_cannot_be_removed` |
| 429 | `rate_limited` |

---

## 14. Rencana test

### Test katalog — penegak otomatis Aturan #1 & #2

```sql
-- Setiap tabel tenant-scoped wajib punya organization_id
-- DAN UNIQUE (id, organization_id)
-- Allowlist global: users, email_verification_tokens,
--                   password_reset_tokens, goose_db_version
```

Sekali ditulis, ia menangkap **setiap tabel baru** yang lupa mengikuti konvensi — untuk selamanya, tanpa ada yang perlu mengingatnya saat review.

### Harness isolasi tenant — [multi-tenancy.md lapis 4](../../architecture/multi-tenancy.md)

Generik atas daftar route, **bukan** manual per endpoint.

| # | Kasus | Menjaga |
|---|---|---|
| 1 | Baca resource tenant lain → 404 | Lapis 1 |
| 2 | Ubah/hapus resource tenant lain → 404 | Lapis 1 |
| 3 | Menunjuk membership tenant lain di body → ditolak **database** | Lapis 2 |
| 4 | **User dengan dua membership** tidak melihat data org yang tidak aktif di token | [ADR-007](../../decisions/ADR-007-user-organization-cardinality.md) |

**Kriteria kualitas:** harness harus **terbukti bisa gagal**. Hapus `organization_id` dari satu query dengan sengaja; bila test tetap hijau, harness-nya belum benar.

> Test isolasi yang selalu hijau karena tidak benar-benar menguji apapun **lebih berbahaya daripada tidak ada test** — ia memberi rasa aman palsu pada seluruh session berikutnya.

### Test keamanan wajib

| Skenario | Harapan |
|---|---|
| Undangan untuk email yang sudah punya akun, tanpa autentikasi | 401 — **tidak pernah** menyetel password |
| Refresh token dipakai ulang setelah rotasi | Seluruh family dicabut |
| Owner terakhir menghapus dirinya | 409 |
| Mengubah role diri sendiri | 403 |
| Request cookie tanpa `X-CSRF-Token` | 403 |
| `COOKIE_SECURE=false` + `APP_ENV=production` | Gagal saat boot |

Semua test yang menyentuh database memakai `dbtest` dari Phase 0.

---

## 15. Risiko teknis

| Risiko | Mitigasi |
|---|---|
| **Harness isolasi yang tidak benar-benar menguji** — risiko terbesar phase ini | Kriteria "terbukti bisa gagal" di atas dijadikan bagian acceptance, bukan catatan |
| **Argon2 memperlambat test** — 19 MiB × banyak test | Parameter lebih rendah khusus test lewat build tag/env; parameter produksi tetap dipakai di test yang memverifikasi hashing itu sendiri |
| **Alur undangan bercabang** — cabang yang salah = pengambilalihan akun | Test keamanan tersendiri (14), bukan hanya test alur bahagia |
| **Deteksi reuse token salah positif** — race saat client mengirim dua refresh bersamaan | Rotasi dalam satu transaksi + `SELECT ... FOR UPDATE` pada baris token; race yang tersisa memang seharusnya dianggap mencurigakan |
| **Cookie + bearer di endpoint yang sama** membingungkan aturan CSRF | CSRF ditegakkan **hanya** untuk request yang terautentikasi lewat cookie — diputuskan di middleware auth, bukan ditebak per handler |

---

## 16. Yang berubah pada dokumentasi

| Berkas | Perubahan |
|---|---|
| [`api.md`](../../architecture/api.md) | Katalog error (13) + perluasan envelope untuk `organization_selection_required` (6.2) |
| `architecture/authentication.md` | **Dibuat** — tiga jalur, token, rotasi, cookie vs bearer, CSRF |
| `architecture/authorization.md` | **Dibuat** — matriks RBAC yang terwujud + empat aturan bertest |
| [`multi-tenancy.md`](../../architecture/multi-tenancy.md) | Diperbarui dengan harness yang benar-benar dibangun |
| `STATUS.md` | Phase 1 selesai, utang teknis |
| `phases/01-auth-organization/notes.md` | Satu bagian per issue |

---

## 17. Kewajiban yang diteruskan ke Phase 2

Ditulis di sini agar tidak bergantung pada ingatan:

> **Penonaktifan membership wajib menolak berjalan bila masih ada lead terbuka**, kecuali disertai keputusan eksplisit (reassign atau lepas assignment) — [freeze bagian 2.3](../../architecture/freeze.md).
>
> Di Phase 1 aturan ini **belum bisa ditegakkan** karena tabel `leads` belum ada. Yang sudah wajib sekarang hanyalah pencabutan sesi. Titik penegakannya ditambahkan bersama `leads` di Phase 2.
