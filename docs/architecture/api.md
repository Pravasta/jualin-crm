# API Conventions

> Sumber: `freeze.md` bagian 5.2 (Aturan #33) · 5.1 (Aturan #24)
>
> **Dikunci sejak bootstrap, bukan Phase 4.** Konvensi ini terbentuk secara de facto di Session 3 dan Session 5 — bila baru ditulis di Phase 4, ia hanya akan mendokumentasikan apapun yang kebetulan terbentuk. Begitu Next.js dan Flutter menempel, mengubahnya berarti memutus dua client sekaligus.

---

## Versioning

Prefix path `/v1/` pada **seluruh** endpoint, termasuk yang hanya dipakai internal.

```
/v1/auth/register
/v1/auth/login
/v1/leads
/v1/leads/{id}
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

**Dikirim sejak versi pertama.** Menambahkannya nanti membingungkan integrator yang sudah jalan.

Endpoint yang wajib dibatasi sejak Phase 1: login, kirim ulang verifikasi, lupa password, undangan (Aturan #34).

---

## Yang menyusul di Phase 4

Dokumen ini akan bertambah satu bab **API Publik**: detail autentikasi API key, scope, format `jln_live_`, contoh integrasi `curl`, dan halaman dokumentasi yang menghadap pelanggan.

Kualitas halaman itu berdampak langsung ke biaya support — di produk berharga murah, pelanggan harus bisa onboard sendiri.
