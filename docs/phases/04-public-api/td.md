# Phase 4 — Public API · Technical Design

> **Bagaimana.** Apa & kenapa di [`prd.md`](./prd.md).
> Hanya **delta** untuk phase ini — aturan yang sudah ada di [`freeze.md`](../../architecture/freeze.md) dirujuk, tidak diulang.
> Sumber: freeze 3.1, 5.1 (Aturan #24), 8.4 (`0005`), Aturan #21, #23, #26, #32, #33 · [ADR-004](../../decisions/ADR-004-api-key-format.md) · [ADR-011](../../decisions/ADR-011-layered-packages-and-unit-of-work.md) · [`api.md`](../../architecture/api.md)

---

## 1. Schema — migration `0005_api_keys`

### 1.1 `api_keys`

Tenant-scoped. Aturan #1, #2, #12, #13, #16 berlaku penuh.

| Kolom | Tipe | Ketentuan |
|---|---|---|
| `id` | uuid | PK, UUIDv7 dari aplikasi, **tanpa** `DEFAULT` (Aturan #12) |
| `organization_id` | uuid | NOT NULL, FK → `organizations` |
| `key_id` | text | NOT NULL, **`UNIQUE` global, ter-index** — ini yang dicari (ADR-004) |
| `secret_hash` | text | NOT NULL — SHA-256 hex dari bagian secret |
| `key_prefix` | text | NOT NULL — untuk ditampilkan (`jln_live_a3f9…`) |
| `name` | text | NOT NULL — diisi Owner (*"Website utama"*) |
| `scopes` | text[] | NOT NULL, `CHECK` isi ⊆ `{leads:write}` |
| `created_by_membership_id` | uuid | NULL — composite FK, hanya untuk audit (ADR-004 aturan #2) |
| `created_at` | timestamptz | NOT NULL |
| `last_used_at` | timestamptz | NULL |
| `revoked_at` | timestamptz | NULL |
| `expires_at` | timestamptz | NULL — **selalu NULL di Phase 4** (prd *Di luar cakupan*) |

```sql
CONSTRAINT uq_api_keys_id_org UNIQUE (id, organization_id)      -- Aturan #2
CONSTRAINT uq_api_keys_key_id UNIQUE (key_id)                   -- pencari, lintas organization
CONSTRAINT ck_api_keys_scopes CHECK (scopes <@ ARRAY['leads:write']::text[])
CONSTRAINT fk_api_keys_created_by FOREIGN KEY (created_by_membership_id, organization_id)
    REFERENCES memberships (id, organization_id)                -- Aturan #3
CREATE INDEX ix_api_keys_org_created ON api_keys (organization_id, created_at DESC);
```

**`key_id` unik lintas organization, bukan per organization.** Ini pengecualian sadar terhadap kebiasaan
"unik per tenant": lookup terjadi **sebelum** organization diketahui — organization justru hasil dari
lookup itu (Aturan #5: `organization_id` tidak pernah dari client). Bentuknya sama persis dengan
`refresh_tokens.token_hash` dan `RefreshTokenRepository.FindByHashForUpdate` di Phase 1, dan dicatat di
sini karena alasan yang sama: ia terlihat seperti pelanggaran tenancy bila dibaca sekilas.

**Tidak ada `deleted_at`.** Revoke ≠ hapus. Kredensial yang direvoke harus tetap terbaca — Owner perlu
tahu bahwa kunci itu pernah ada, siapa membuatnya, dan kapan terakhir dipakai. Aturan #18 mewajibkan
soft delete untuk entity bisnis; `api_keys` memakai `revoked_at` yang lebih kuat: ia tidak pernah hilang
dari daftar.

**`scopes` sebagai `text[]`, bukan tabel dan bukan JSONB.** Nilainya satu enum kecil dan tertutup
(ADR-004 aturan #4), tidak pernah di-query isinya kecuali sebagai keanggotaan. Aturan #17 mensyaratkan
alasan tertulis untuk JSONB — tidak ada alasan itu di sini, dan tabel `api_key_scopes` adalah tabel
untuk kebutuhan yang belum ada (Aturan #29).

### 1.2 `ALTER leads ADD source_api_key_id`

```sql
ALTER TABLE leads ADD COLUMN source_api_key_id uuid;
ALTER TABLE leads ADD CONSTRAINT fk_leads_source_api_key
    FOREIGN KEY (source_api_key_id, organization_id)
    REFERENCES api_keys (id, organization_id);                  -- Aturan #3, composite
```

Nullable, instan (Aturan 8.4 freeze). `leads.source` sudah menerima `'api'` sejak `0003` — nilai enum
mendahului FK-nya, sengaja.

**Tidak ada `CHECK` yang memaksa `source='api' ⟺ source_api_key_id IS NOT NULL`.** Lead `source='api'`
bisa lahir dari import lama atau dari kunci yang sudah dihapus di masa depan; memaksakan biconditional
di database berarti migration berikutnya yang menyentuhnya harus membongkarnya lebih dulu. Yang
ditegakkan usecase: jalur API key **selalu** mengisi keduanya.

### Larangan yang berlaku pada migration ini

- Tanpa `DEFAULT gen_random_uuid()` — UUIDv7 dari aplikasi (Aturan #12)
- Tanpa tipe `ENUM` PostgreSQL — `CHECK` (Aturan #15)
- Tanpa index yang tidak berawalan `organization_id`, **kecuali** `uq_api_keys_key_id` yang alasannya di §1.1 (Aturan #16)
- `down` harus bersih: `DROP CONSTRAINT` + `DROP COLUMN` pada `leads` sebelum `DROP TABLE api_keys`

---

## 2. Format & siklus hidup kredensial

Mengikuti [ADR-004](../../decisions/ADR-004-api-key-format.md). Yang di bawah adalah bagian yang ADR
biarkan terbuka.

```
jln_live_a3f9K2mQxL7p_<secret>
└──┬───┘ └─────┬─────┘ └──┬───┘
   │           │          └─ 32 byte crypto/rand, base64url → 43 karakter
   │           └─ key_id: 9 byte crypto/rand, base64url → 12 karakter, plaintext & ter-index
   └─ environment
```

### Dua angka di ADR-004 yang tidak konsisten satu sama lain

ADR-004 menulis *"secret: 32char"* dan *"entropi 256-bit"* pada baris yang sama. Keduanya tidak bisa
benar bersamaan: 32 karakter base64url ≈ 192 bit; 256 bit = 32 **byte** = 43 karakter base64url.

**Diambil: 256 bit.** Seluruh argumen keamanan ADR itu bersandar pada angka tersebut — *"Pada rahasia
256-bit dari `crypto/rand`, brute-force mustahil terlepas dari kecepatan hash"* — dan itulah yang membuat
SHA-256 (bukan argon2id) sah di sini. "32char" terbaca sebagai penyebutan longgar dari "32 byte".
Dicatat, bukan diperbaiki diam-diam (Aturan #30); bila pembacaan ini keliru, yang berubah hanyalah satu
konstanta.

`key_id` 12 karakter tepat = 9 byte base64url tanpa padding. Ia **bukan rahasia** — hanya pencari.

`key_prefix` = `jln_live_` + 4 karakter pertama `key_id` → `jln_live_a3f9`, persis contoh di ADR-004.
(Baris *"8 karakter pertama"* di ADR itu, dibaca harfiah, menghasilkan `jln_live` yang tidak
membedakan kunci mana pun — contoh di kolom sebelahnya adalah maksud yang sebenarnya.)

### `jln_test_` diterima bentuknya, tidak pernah diterbitkan

Parser mengenali `jln_live_` dan `jln_test_`; Phase 4 hanya menerbitkan `live`. Kunci `test` tidak akan
ditemukan di database → `401`. Membangun mode sandbox berarti membangun tempat lead palsu hidup tanpa
mengotori data pelanggan — itu fitur tersendiri, bukan awalan string (Aturan #29).

### Siklus hidup

```
create   → raw ditampilkan SEKALI di response 201, tidak pernah lagi (Aturan #21)
list     → key_prefix, name, scopes, created_at, last_used_at, revoked_at — TIDAK PERNAH secret_hash
revoke   → revoked_at = now(). Idempoten: revoke kedua tetap 204.
```

Tidak ada endpoint "lihat lagi", "regenerate", atau "ubah scope". Regenerate = buat baru + revoke lama,
dua aksi yang memang berbeda dan terlihat berbeda di audit log.

---

## 3. Autentikasi — pemilahan kredensial (keputusan D2)

Satu path, dua bentuk kredensial:

```
Authorization: Bearer eyJhbGciOi...        → JWT       → PrincipalUser
Authorization: Bearer jln_live_a3f9...      → API key   → PrincipalAPIKey
Cookie: access_token=...                    → JWT       → PrincipalUser (+ CSRF)
```

Pemilahannya **bukan** tebakan: `jln_live_`/`jln_test_` adalah prefix yang tidak mungkin muncul di JWT
(yang selalu diawali `eyJ` — base64url dari `{"`). Bila prefix cocok tapi kuncinya tidak valid, hasilnya
`401 invalid_api_key`, **bukan** jatuh kembali ke jalur JWT — kredensial yang menyatakan dirinya API key
diperlakukan sebagai API key sampai selesai.

### Letak kode

`authn` mendapat satu fungsi baru, bukan paket baru:

```go
// internal/shared/authn/authn.go
type APIKeyResolver interface {
    ResolveAPIKey(ctx context.Context, raw string) (tenant.Context, error)
}

func MiddlewareWithAPIKey(parser ClaimsParser, keys APIKeyResolver) gin.HandlerFunc
```

`APIKeyResolver` dideklarasikan **konsumennya** (Aturan ADR-011: interface milik consumer), dipenuhi
secara struktural oleh `*apikey.Usecase`. `authn` tidak pernah mengimpor `internal/apikey` — pola yang
sama persis dengan `ClaimsParser` yang membuat `authn` tidak mengimpor `internal/auth` sejak #11.

`MiddlewareWithAPIKey` dipasang **hanya** pada `POST /v1/leads`. Seluruh route lain tetap memakai
`authn.Middleware` yang tidak mengenal API key sama sekali — pertahanan pertama Aturan #24 adalah routing,
bukan pengecekan.

### CSRF

Tidak berlaku pada jalur API key: kredensialnya tidak pernah dikirim otomatis oleh browser, sama seperti
`Authorization: Bearer` pada jalur mobile (`authentication.md`). `MiddlewareWithAPIKey` mempertahankan
cabang CSRF yang sudah ada untuk kredensial yang datang **dari cookie**.

---

## 4. Otorisasi principal API key (keputusan D1)

### Masalahnya

`authz.Require(t, action)` membaca `permissions[t.Role][action]`. API key tidak punya role — `t.Role`
kosong, `permissions[""]` nil, **semuanya ditolak**. Itu jawaban yang benar untuk 23 action yang ada, dan
salah untuk satu-satunya yang harus diizinkan.

### Bentuknya

```go
// internal/shared/tenant/tenant.go
type Context struct {
    // …
    APIKeyID *uuid.UUID   // sudah ada sejak Phase 1
    Scopes   []string     // baru — hanya terisi saat PrincipalType == PrincipalAPIKey
}
```

```go
// internal/shared/authz/authz.go
// Satu-satunya action yang bisa dilakukan principal API key. Setiap
// action lain ditolak karena TIDAK ADA DI PETA INI — bukan karena
// seseorang ingat menuliskan pengecualiannya.
var apiKeyScopeFor = map[Action]string{
    ActionLeadCreate: "leads:write",
}

func Require(t tenant.Context, action Action) error {
    if t.PrincipalType == tenant.PrincipalAPIKey {
        scope, ok := apiKeyScopeFor[action]
        if !ok || !slices.Contains(t.Scopes, scope) {
            return forbidden()      // 403 insufficient_scope
        }
        return nil
    }
    return roleGate(t, action)      // perilaku Phase 1–3, tak berubah
}
```

**Kenapa di `authz`, bukan di handler publik.** Freeze 5.1 mensyaratkan penegakan Aturan #24 di lapisan
yang dilewati semua orang, bukan per handler. Menaruhnya di sini berarti endpoint baru mana pun yang
lupa memikirkan API key **otomatis menolaknya** — kegagalan yang aman. Menaruhnya di handler berarti
endpoint yang lupa **otomatis mengizinkannya**.

**Kenapa peta terpisah, bukan baris di `permissions`.** Menambahkan `tenant.Role("api_key")` akan
membuat API key tampak sebagai role kelima di seluruh matriks — dan `authorization.md` sudah menyatakan
role adalah enum tertutup berisi empat. Peta terpisah membuat pertanyaan *"apa yang bisa dilakukan API
key?"* dijawab dengan membaca sepuluh baris, bukan menyaring empat kolom.

### Test yang mengunci ini

Test tabel yang mengulang **seluruh** `Action` yang terdaftar di `authz`: setiap action selain
`ActionLeadCreate` harus ditolak untuk `PrincipalAPIKey`. Action baru yang ditambahkan phase berikutnya
otomatis ikut diuji tanpa ada yang perlu ingat menambahkannya.

---

## 5. `POST /v1/leads` — jalur API key

Endpoint yang sama, handler yang sama, usecase yang sama. Yang berbeda hanya isi `tenant.Context`.

| Aspek | Jalur user (Phase 2–3) | Jalur API key (baru) |
|---|---|---|
| `source` | dari body, default `manual` | **dipaksa** `api` — body diabaikan bila mengirim `source` |
| `source_api_key_id` | NULL | `t.APIKeyID` |
| `created_by_membership_id` | `t.MembershipID` | NULL |
| `assigned_to_membership_id` | boleh dari body | **ditolak** — `403 insufficient_scope`, bukan diabaikan diam-diam |
| `raw_payload` | NULL | seluruh body seperti diterima |
| Rate limit | tidak ada | §6 |
| Activity `lead_created` | `actor_membership_id` = membership | `actor_membership_id` = **NULL** |

**`assigned_to_membership_id` ditolak, bukan diabaikan.** Sistem eksternal tidak punya cara mengetahui
membership id siapa pun, jadi field itu hanya bisa muncul karena salah paham. Mengabaikannya diam-diam
berarti integrator mengira assignment-nya bekerja.

**`raw_payload`.** Body disimpan apa adanya termasuk field tak dikenal — alasannya sudah tertulis di
`0003` (Aturan #17): *"field itu tetap tersimpan supaya lead bisa direkonstruksi saat integrator melapor
'data saya hilang'"*. Batas ukuran body **64 KB** (`http.MaxBytesReader`); di atas itu `413
payload_too_large` sebelum apa pun di-parse. Tanpa batas, `raw_payload` adalah jalur menulis megabyte ke
database dengan satu request.

**`lead_created` tanpa aktor.** `activities.actor_membership_id` memang nullable untuk kasus ini (freeze
2.5). **Konsekuensi ke dashboard yang sudah jadi:** `#33` merender nama aktor di timeline; lead pertama
yang masuk lewat API akan menjadi yang pertama menabraknya. Diverifikasi di #2, diperbaiki di #3 bila
tampil kosong (lihat §16).

### Perubahan pada `internal/lead`

```
CreateLeadInput  + RawPayload []byte, + SourceAPIKeyID *uuid.UUID
CreateInput      + RawPayload []byte, + SourceAPIKeyID *uuid.UUID
```

Usecase `Create` menetapkan `source='api'` dan menolak `assigned_to_membership_id` **saat
`t.PrincipalType == PrincipalAPIKey`** — di usecase, bukan di handler (Aturan: otorisasi & aturan bisnis
di usecase). Jalur idempotent replay yang sudah ada (`ErrIdempotencyKeyExists` → 200) **tidak disentuh**.

---

## 6. Rate limit & header (keputusan D4)

### Yang kurang dari `ratelimit` hari ini

`Limiter.Allow(key) bool` menjawab boleh/tidak, tapi `X-RateLimit-Remaining` dan `X-RateLimit-Reset`
butuh angka. Yang ditambahkan:

```go
// internal/shared/ratelimit/limiter.go
type Result struct {
    Allowed   bool
    Limit     int
    Remaining int
    ResetAt   time.Time
}

func (f *FixedWindow) Take(key string) Result   // metode baru pada tipe yang sudah ada
```

**Bukan interface baru** — `Limiter` tetap apa adanya dan seluruh call site Phase 1 tidak berubah.
Handler publik mendeklarasikan interface konsumennya sendiri berisi satu metode `Take`. Abstraksi baru
menunggu implementasi kedua yang nyata (Aturan #28).

### Angka & kunci

| Aspek | Nilai |
|---|---|
| Batas | 60 request / menit **per `api_keys.id`** (`PUBLIC_API_RATE_LIMIT`, default 60) |
| Jendela | tetap, 1 menit |
| Kunci limiter | `publiclead:key:<api_key_id>` — bukan per IP: satu website di belakang CDN muncul sebagai banyak IP, dan kredensial adalah identitas yang sebenarnya kita batasi |

Header dikirim pada **setiap** response jalur API key, termasuk `201`, `200` (replay), dan `4xx` —
`api.md`: *"Dikirim sejak versi pertama"*. Pada `429` ditambah `Retry-After` (detik sampai `ResetAt`).

```go
// internal/shared/httpx
func SetRateLimitHeaders(c *gin.Context, r ratelimit.Result)
```

Rate limit dievaluasi **setelah** kredensial diverifikasi — kunci limiternya adalah `api_key_id`, yang
baru diketahui setelah lookup. Konsekuensi yang diterima: request dengan kredensial sampah tidak
dibatasi oleh limiter ini. Yang membatasinya adalah biaya lookup itu sendiri (satu index hit) dan
`invalid_api_key` yang tidak membedakan penyebab, sehingga penebakan tidak memberi sinyal apa pun.

---

## 7. Idempotency & retensi (keputusan D3)

Mekanismenya **tidak berubah** dari Phase 2 §7: unique violation pada `uq_leads_org_idempotency`, replay
mengembalikan lead asli dengan `200`. Yang ditambahkan hanya retensi.

`idempotency_key` adalah **kolom pada `leads`**, bukan tabel tersendiri (freeze: *"resource-nya adalah
response-nya"*). Jadi "menghapus key yang kedaluwarsa" berarti mengosongkan kolomnya, **bukan** menghapus
lead:

```sql
-- di jalur POST /v1/leads, sebelum insert, DI LUAR transaksi lead
UPDATE leads
   SET idempotency_key = NULL
 WHERE organization_id = $1
   AND idempotency_key IS NOT NULL
   AND created_at < now() - interval '48 hours';
```

Lead-nya tetap utuh; yang kedaluwarsa hanyalah jaminan *"kirim ulang mengembalikan yang ini"*.

| Aspek | Ketentuan |
|---|---|
| Umur | 48 jam (`api.md`: 24–48) |
| Kapan dijalankan | Pada `POST /v1/leads` jalur API key, **paling sering sekali per organization per jam** (throttle in-memory, sama bentuknya seperti `last_used_at` §10) |
| Di dalam transaksi lead? | **Tidak.** Pembersihan yang gagal tidak boleh menggagalkan lead yang sedang masuk. |
| Bila tidak ada traffic | Tidak ada yang dibersihkan — dan tidak ada yang perlu dibersihkan |

**Kenapa penghapusan malas, bukan cron.** Freeze bagian 6 aturan #4: tabel `jobs` + worker
diperkenalkan saat ada kebutuhan async **nyata** — push notification (Phase 5) atau webhook (Phase 7).
Membangun scheduler untuk satu `UPDATE` adalah infrastruktur yang lebih besar dari masalahnya.

**Konsekuensi yang diterima:** organization yang berhenti mengirim lead selamanya akan menyimpan key
lamanya selamanya. Tidak berbahaya (tidak pernah membuat duplikat) dan volumenya terbatas oleh jumlah
lead yang pernah mereka kirim.

---

## 8. CORS & Aturan #23 (keputusan D5)

**Tidak ada kode baru.** `CORS_ALLOWED_ORIGINS` (#30) berisi origin dashboard kita, tidak pernah domain
pelanggan. Konsekuensinya sudah berlaku hari ini: `fetch('…/v1/leads', {headers:{Authorization:'Bearer
jln_live_…'}})` dari website pelanggan diblokir browser sebelum response terbaca.

Yang ditambahkan Phase 4:

1. **Test yang menguncinya** — request dengan `Origin: https://toko-pelanggan.example` tidak menerima
   satu pun header `Access-Control-Allow-*` (§16). Tanpa test ini, seseorang yang kelak menambahkan
   wildcard "supaya lebih mudah" tidak akan tertahan apa pun.
2. **Peringatan eksplisit di halaman dokumentasi** (§13) — *"kirim dari server Anda, jangan dari
   browser"*, dengan alasannya, bukan hanya larangannya.

> Kalimat *"kenapa tidak bisa dari JavaScript website saya?"* adalah pertanyaan support yang **akan**
> datang. Menjawabnya di halaman dokumentasi lebih murah daripada menjawabnya satu per satu — dan
> jawabannya bukan "tidak didukung" melainkan "kunci Anda akan terbaca setiap pengunjung". Kebutuhan itu
> punya jawaban sendiri di Phase 6 (ADR-005).

---

## 9. Endpoint

### Manajemen kredensial — principal **user**, di belakang `authn.Middleware`

| Method | Path | Isi | Role |
|---|---|---|---|
| `POST` | `/v1/api-keys` | `{name, scopes?}` → `201 {id, name, key_prefix, scopes, created_at, secret}` | Owner, Admin |
| `GET` | `/v1/api-keys` | Daftar, terbaru dulu — **tanpa** `secret` | Owner, Admin |
| `DELETE` | `/v1/api-keys/{id}` | Revoke → `204`, idempoten | Owner, Admin |

`secret` **hanya** ada di response `POST`, sekali (Aturan #21). Tidak ada `GET /v1/api-keys/{id}` —
daftar sudah memuat seluruh field yang boleh dilihat, dan endpoint detail hanya menambah permukaan.

Manager dan Employee **tidak punya akses sama sekali** — bukan read-only. API key adalah kredensial yang
bisa memasukkan data ke organization; daftarnya sendiri adalah informasi tentang integrasi apa yang
hidup. Ini sejajar dengan `invitation.*` yang juga Owner/Admin saja.

### API publik — principal **api_key**

| Method | Path | Isi |
|---|---|---|
| `POST` | `/v1/leads` | `{name, email?, phone?, company?, notes?, …}` + `Idempotency-Key?` → `201` / `200` (replay) |

Endpoint yang **sama** dengan jalur dashboard (D2). Ini satu-satunya endpoint di API ini yang menerima
dua principal, dan §3 adalah alasan tertulisnya yang `api.md` syaratkan.

### Action `authz` baru

```
ActionAPIKeyCreate  api_key.create   Owner, Admin
ActionAPIKeyList    api_key.list     Owner, Admin
ActionAPIKeyRevoke  api_key.revoke   Owner, Admin
```

Ketiganya **tidak** ada di `apiKeyScopeFor` (§4) — API key tidak bisa membuat API key. Ini kasus yang
freeze 5.1 sebut namanya secara eksplisit.

---

## 10. `last_used_at` — di-throttle

ADR-004 aturan #3: *"di-update async atau throttled, bukan setiap request — kalau tidak, tabel ini
menjadi write hotspot."*

| Aspek | Ketentuan |
|---|---|
| Frekuensi | Paling sering **sekali per 5 menit per kunci** — peta in-memory `map[uuid.UUID]time.Time` di `apikey.Usecase` |
| Kapan | Setelah response terkirim, tidak memblokir request |
| Bila proses restart | Peta kosong → paling banyak satu penulisan ekstra per kunci. Tidak ada koreksi yang perlu dilakukan. |
| Akurasi yang dijanjikan | *"terakhir dipakai sekitar…"*, dan dashboard menuliskannya begitu — bukan timestamp presisi |

Peta ini tumbuh sebesar jumlah kunci aktif per proses; tidak ada eviction, sama seperti
`ratelimit.FixedWindow` (utang yang sudah tercatat sejak #9).

---

## 11. Audit log & logging (Aturan #26)

| Aksi | `action` | `actor_type` |
|---|---|---|
| Buat kredensial | `api_key.created` | `user` |
| Revoke kredensial | `api_key.revoked` | `user` |

Lead yang masuk lewat API **tidak** menulis audit log — ia menulis `activity`, seperti setiap lead lain.
Audit log untuk aksi sensitif; membuat lead bukan salah satunya, dan satu baris audit per lead masuk
akan membuat tabel itu tumbuh secepat `leads`.

### Yang tidak boleh masuk log

Aturan #26 sudah menyebut *raw API key* dan *payload lead lengkap*. Dua jalur baru yang berisiko:

1. **Request yang gagal.** `401 invalid_api_key` menggoda untuk mencatat kredensial yang ditolak "supaya
   bisa di-debug". Yang boleh dicatat: `key_id` (bukan rahasia). Yang tidak: apa pun setelahnya.
2. **`raw_payload`.** Disimpan ke database, **tidak pernah** ke log — termasuk saat validasi gagal.
   Pesan errornya menyebut nama field, bukan isinya.

Redaksi di logger, bukan di call site (Aturan #26).

---

## 12. Dashboard — manajemen API key

`crm_dashboard`, mengikuti seluruh konvensi Phase 3 (skill `jualin-dashboard`).

| Layar | Isi |
|---|---|
| `/settings/api-keys` | Daftar: `key_prefix`, nama, scopes, dibuat oleh, terakhir dipakai, status. Tombol *Buat kunci baru* dan *Cabut*. |
| Dialog buat | Satu field nama → response memuat `secret` |
| Dialog "kunci Anda" | **Satu-satunya kali secret terlihat.** Tombol salin, peringatan bahwa ia tidak bisa dilihat lagi, tombol tutup yang mengharuskan konfirmasi telah menyimpan |
| Dialog cabut | Konfirmasi tunggal, menyebut nama kunci — pencabutan tidak bisa dibatalkan |

Digerbang `role === 'owner' || role === 'admin'`, sejajar dengan area tim di #34. Manager/Employee tidak
melihat menunya sama sekali.

**Yang harus dihindari:** menaruh `secret` ke state yang bertahan (context, `localStorage`, URL). Ia hidup
di state dialog dan mati bersamanya. Aturan #25 melarang token di `localStorage`; raw API key lebih
sensitif dari itu — ia tidak kedaluwarsa.

---

## 13. Halaman dokumentasi integrasi

Halaman **menghadap pelanggan** di dashboard (`/settings/api-keys/docs` atau tab di layar yang sama),
bukan file Markdown di repo. Alasannya: pembacanya adalah developer milik pelanggan yang sedang membuka
dashboard untuk menyalin kunci — jarak antara kunci dan cara memakainya harus nol.

Isi minimum:

1. **Satu contoh `curl` yang bisa disalin dan langsung bekerja**, dengan `key_prefix` kunci yang sedang
   dilihat sudah terisi di dalamnya
2. Autentikasi: header, format, dan **peringatan browser** (§8)
3. Field yang diterima, mana yang wajib, apa yang terjadi pada field tak dikenal (`raw_payload`)
4. Katalog error yang mungkin diterima integrator, dengan arti dan tindakan yang disarankan
5. `Idempotency-Key`: kapan dipakai, apa yang terjadi saat diulang, retensi 48 jam
6. Header rate limit dan arti `429`

**Kriteria selesai bukan "halaman ada", melainkan "integrasi bekerja tanpa bertanya"** (acceptance
criterion #10) — diverifikasi dengan mengikuti halaman itu dari nol untuk membuat lead pertama,
bukan dengan membacanya ulang.

---

## 14. Error code baru

Ditambahkan ke katalog di [`api.md`](../../architecture/api.md):

| HTTP | `code` | Kapan |
|---|---|---|
| 401 | `invalid_api_key` | Kredensial `jln_*` tidak dikenal, sudah direvoke, atau kedaluwarsa — **ketiganya pesan yang sama** |
| 403 | `insufficient_scope` | Kredensial sah tapi scope-nya tidak mencakup aksi ini, termasuk setiap endpoint aplikasi pengguna (Aturan #24) |
| 413 | `payload_too_large` | Body `POST /v1/leads` di atas 64 KB |

**Ketiga penyebab `invalid_api_key` sengaja tidak dibedakan.** Membedakan "tidak ada" dari "sudah
direvoke" memberi tahu penebak bahwa kunci itu pernah ada — bentuk kebocoran yang sama dengan 403-vs-404
di Aturan #6.

Dipakai ulang tanpa perubahan: `validation_failed` (400), `rate_limited` (429), `internal_error` (500).

---

## 15. Paket baru

```
internal/apikey/          entity · port · usecase · repository_postgres · handler_http
```

Satu paket, bentuk lima berkas penuh (ADR-011), dengan `Store`/`Repos` sendiri dan composition root
`cmd/api/apikey_store.go`.

Yang **bertambah** pada paket yang sudah ada:

| Paket | Perubahan |
|---|---|
| `internal/shared/tenant` | `Context.Scopes []string` |
| `internal/shared/authz` | 3 `Action` baru + cabang `PrincipalAPIKey` + peta `apiKeyScopeFor` |
| `internal/shared/authn` | `APIKeyResolver` + `MiddlewareWithAPIKey` |
| `internal/shared/ratelimit` | `Result` + `(*FixedWindow).Take` |
| `internal/shared/httpx` | `SetRateLimitHeaders` |
| `internal/shared/config` | `PUBLIC_API_RATE_LIMIT` |
| `internal/lead` | `RawPayload`, `SourceAPIKeyID`, cabang `PrincipalAPIKey` di `Create` |

Tidak ada paket `internal/publicapi`. Endpoint publiknya **adalah** `POST /v1/leads` milik
`internal/lead`; memindahkannya ke paket lain berarti dua handler untuk satu route.

---

## 16. Rencana test

### Test keamanan wajib

| Skenario | Harapan |
|---|---|
| API key memanggil `GET /v1/leads` | **403** `insufficient_scope` |
| API key memanggil `GET /v1/memberships`, `POST /v1/invitations`, `POST /v1/api-keys` | **403** — dan ketiganya diuji, bukan diwakili satu |
| **Tabel atas seluruh `authz.Action`**: setiap action ≠ `lead.create` ditolak untuk `PrincipalAPIKey` | Action baru phase berikutnya ikut terjaga otomatis |
| Kunci direvoke, lalu dipakai | **401** `invalid_api_key` seketika, tanpa jeda |
| Kunci organization A membuat lead, dibaca dari organization B | **404** |
| `secret` kunci A + `key_id` kunci B | **401** — perbandingan constant-time tidak bocor |
| `key_id` ada, secret salah satu karakter | **401**, pesan **identik** dengan `key_id` yang tidak ada |
| API key mengirim `assigned_to_membership_id` | **403**, lead **tidak** dibuat |
| Body 65 KB | **413**, tidak ada baris tertulis |
| `Origin: https://toko-pelanggan.example` pada `POST /v1/leads` | Tidak ada satu pun header `Access-Control-Allow-*` |
| Response `401` mana pun | Raw kredensial tidak muncul di log (diperiksa terhadap buffer log, bukan dengan membaca kode) |

### Wajib di bawah konkurensi

| Test | Membuktikan |
|---|---|
| N request bersamaan dengan `Idempotency-Key` sama lewat API key | Tepat **satu** lead — jalur Phase 2 tetap utuh melalui kredensial baru |
| N request bersamaan melewati batas rate limit | Jumlah `201` **tepat** sama dengan batas, sisanya `429` — bukan "kira-kira" |

### Harness isolasi tenant

`cmd/api/tenant_isolation_test.go` bertambah entri `api_key` ke `[]isolationCase` yang sudah ada sejak
#11 — **menambah entri, bukan menulis harness baru**.

**Kriteria kualitas tetap berlaku: harness harus terbukti bisa gagal.** Prosedurnya sudah dijalankan di
#11, #23, dan #30; ulangi untuk `api_key` — hapus predikat `organization_id` dari
`FindByID` secara sengaja, pastikan merah, kembalikan.

### Test lain

- Format: `Generate()` menghasilkan `key_id` 12 karakter, secret 43 karakter, prefix cocok; `Parse()` menolak bentuk cacat (tanpa underscore, environment tak dikenal, secret kosong)
- `Take()` pada `FixedWindow`: `Remaining` menurun, `ResetAt` maju satu jendela, `Allowed` berubah tepat di batas
- Retensi idempotency: key berumur 49 jam menjadi NULL, key berumur 47 jam tetap
- `last_used_at`: dua request berturut-turut menghasilkan **satu** `UPDATE`
- Setiap usecase punya test unit tanpa Docker (`TestUnit_*`, fake `Store`) — ADR-011
- **Dashboard**: `secret` tidak pernah keluar dari state dialog (test unit atas fungsi murni yang membentuk baris daftar, mengikuti pola `lib/nav.ts`, `lib/team-permissions.ts`)

### Verifikasi manual yang tidak bisa digantikan test

`curl` dari mesin **di luar jaringan pengembangan** → lead muncul di dashboard dengan penanda `api`
(acceptance criterion #1). Test integrasi berjalan di dalam proses yang sama; ia tidak bisa membuktikan
bahwa endpoint ini benar-benar terjangkau dari luar.

---

## 17. Risiko teknis

| Risiko | Mitigasi |
|---|---|
| **API key bisa memanggil endpoint aplikasi pengguna** — kegagalan terparah phase ini: kredensial yang bocor dari website statis berubah menjadi pengambilalihan organization | Otorisasi di `authz`, bukan per handler (§4); routing yang hanya memasang `MiddlewareWithAPIKey` pada satu route (§3); test tabel atas **seluruh** `Action` (§16) |
| **Raw secret bocor lewat log atau response** | Tidak pernah disimpan (hanya hash); hanya ada di satu response; test yang memeriksa buffer log; dashboard menahannya di state dialog saja |
| **Rate limit di-bypass** karena kuncinya IP, bukan kredensial | Kunci limiter adalah `api_key_id` (§6) |
| **`last_used_at` menjadi write hotspot** | Throttle 5 menit (§10), diuji |
| **Perbandingan secret bocor lewat waktu** | `subtle.ConstantTimeCompare` (ADR-004), dan pesan `401` yang identik untuk ketiga penyebab (§14) |
| **Bentuk request terlanjur dipakai integrator lalu perlu berubah** | Satu-satunya endpoint publik adalah yang bentuknya sudah dipakai internal sejak Phase 2 — ia tidak dirancang di phase ini, hanya dibuka |
| **Timeline dashboard menampilkan aktor kosong** untuk lead dari API | Diverifikasi di issue #2, diperbaiki di #3 bila perlu (§5) |
| **Halaman dokumentasi ditulis dari rencana, bukan dari perilaku** | Ditempatkan sebagai issue terakhir, setelah endpointnya berjalan; diverifikasi dengan mengikutinya dari nol |

---

## 18. Yang berubah pada dokumentasi

| Berkas | Perubahan |
|---|---|
| [`api.md`](../../architecture/api.md) | **Bab baru "API Publik"** — autentikasi API key, format `jln_live_`, scope, contoh `curl`, header rate limit, idempotency. Ini yang bagian *"Yang menyusul di Phase 4"* janjikan; bagian itu diganti oleh babnya. Plus 3 error code baru (§14). |
| [`authentication.md`](../../architecture/authentication.md) | Jalur kedua dari tiga (*"API key → Phase 4"*) berubah dari rencana menjadi kenyataan: lookup, verifikasi, revoke, dan **kenapa tidak ada CSRF di jalur ini** |
| [`authorization.md`](../../architecture/authorization.md) | Baris `api_key.*` di matriks role; bagian baru untuk otorisasi berbasis scope (§4) — matriks role saja tidak lagi menggambarkan seluruh sistem |
| [`multi-tenancy.md`](../../architecture/multi-tenancy.md) | Lapis 1: `PrincipalAPIKey` akhirnya terpakai; catatan bahwa `key_id` unik lintas organization dan kenapa itu bukan pelanggaran (§1.1) |
| `STATUS.md` | Phase 4 selesai; utang retensi `idempotency_key` **ditutup**; utang eviction map bertambah satu pemakai |
| `phases/04-public-api/notes.md` | Satu bagian per issue |

---

## 19. Kewajiban yang diteruskan ke phase berikutnya

Ditulis di sini agar tidak bergantung pada ingatan:

> **Phase 5 (Mobile)** tidak menyentuh API key sama sekali (Aturan #24). Yang ia warisi dari phase ini
> hanya satu: `tenant.Context.Scopes` sekarang ada, dan jalur user **tidak boleh** mulai memakainya —
> scope adalah mekanisme untuk principal tanpa role.
>
> **Phase 6 (Embedded Form)** menambahkan `forms` + `leads.source_form_id` di `0007`, dengan kredensial
> ketiga (`public_key`, ADR-005) yang **tidak** boleh disatukan dengan API key. Cabang `PrincipalAPIKey`
> di `authz` (§4) adalah tempat `PrincipalPublicForm` akan menempel — bentuknya sudah ada, isinya belum.
> Ia juga yang menjawab *"kirim lead dari browser"* yang Phase 4 tolak (§8).
>
> **Phase 8 (Billing)** memerlukan quota per plan, yang **bukan** rate limit (§6, prd *Di luar cakupan*).
> `usage_counters` menghitung; `ratelimit` melindungi. Keduanya tidak saling menggantikan.
>
> **Kapan pun `expires_at` diisi**: kolomnya sudah ada dan `invalid_api_key` sudah mencakup kedaluwarsa
> sebagai penyebab (§14) — yang belum ada hanya UI dan pengisiannya.
