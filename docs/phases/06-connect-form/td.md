# Phase 6 — Connect & Embedded Form · TD

> **Bagaimana.** Apa & kenapa di [`prd.md`](./prd.md).
> Ini **delta** untuk phase ini. Aturan yang sudah ada di [`freeze.md`](../../architecture/freeze.md) tidak diulang, hanya dirujuk.

---

## 1. Schema — migration `0007_forms`

Nomornya sudah dipesan freeze 8.4: *"`forms` + `leads.source_form_id`"*.

```sql
CREATE TABLE forms (
    id               uuid PRIMARY KEY,
    organization_id  uuid NOT NULL REFERENCES organizations (id),
    public_key       text NOT NULL,
    name             text NOT NULL,
    fields           jsonb NOT NULL,
    allowed_origins  text[] NOT NULL DEFAULT '{}',
    submit_count     integer NOT NULL DEFAULT 0,
    created_by_membership_id uuid,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    deleted_at       timestamptz,

    CONSTRAINT uq_forms_id_org    UNIQUE (id, organization_id),
    CONSTRAINT uq_forms_public_key UNIQUE (public_key),
    CONSTRAINT ck_forms_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT fk_forms_created_by
        FOREIGN KEY (created_by_membership_id, organization_id)
        REFERENCES memberships (id, organization_id)
);

CREATE INDEX ix_forms_org ON forms (organization_id, created_at DESC)
    WHERE deleted_at IS NULL;

ALTER TABLE leads ADD COLUMN source_form_id uuid;
ALTER TABLE leads ADD CONSTRAINT fk_leads_source_form
    FOREIGN KEY (source_form_id, organization_id)
    REFERENCES forms (id, organization_id);
```

`leads.source` **sudah** menerima `'form'` sejak `0003` dan `raw_payload jsonb` sudah ada — tidak ada
`ALTER` pada keduanya. `ALTER TABLE ADD COLUMN` nullable bersifat instan.

### `uq_forms_public_key` — pengecualian unik-lintas-organization **keempat**

Sama seperti `api_keys.key_id` (0005), `device_tokens.token` (0006), dan `refresh_tokens.token_hash`
(0002), constraint ini **tidak** composite dengan `organization_id`. Alasannya sekelas `api_keys.key_id`,
bukan sekelas `device_tokens.token`:

> Lookup kredensial terjadi **sebelum** organization diketahui — organization justru **hasil** dari
> lookup itu (Aturan #5). Composite unique tidak mungkin dibuat: tidak ada `organization_id` untuk
> dijadikan bagian kunci sebelum barisnya ditemukan.

Komentar migration ditulis mengikuti bentuk tiga pendahulunya, dan `multi-tenancy.md` ditambah baris
keempat (§15).

### `fields` sebagai JSONB — alasan tertulis (Aturan #17)

Aturan #17 menuntut alasan tertulis untuk setiap JSONB. Isinya adalah **konfigurasi tampilan**, bukan
data yang di-query:

```json
{
  "name":    { "enabled": true,  "required": true,  "label": "Nama Lengkap" },
  "email":   { "enabled": true,  "required": false, "label": "Email" },
  "phone":   { "enabled": true,  "required": true,  "label": "Nomor WhatsApp" },
  "company": { "enabled": false, "required": false, "label": "Perusahaan" },
  "message": { "enabled": true,  "required": false, "label": "Pesan" },
  "product": { "enabled": false, "required": false, "label": "Layanan Diminati" }
}
```

Enam kunci itu **tetap** (ADR-005: field tetap, bukan builder). Tidak pernah ada query yang memfilter
atau mengurutkan berdasarkan isinya — ia dibaca utuh saat merender form dan saat memvalidasi submit.
Kolom terpisah untuk 6 field × 3 atribut berarti 18 kolom yang seluruhnya hanya dibaca bersamaan.

`submit_count` **bukan** usage counter Phase 8 — ia hanya angka untuk ditampilkan di dashboard
("form ini pernah dipakai atau tidak"). Penegakan kuota adalah `usage_counters`, mekanisme berbeda
(TD Phase 4 §19).

---

## 2. Format & siklus hidup `public_key` (keputusan D3)

```
pk_<22 karakter base64url>
```

| Aspek | Ketentuan |
|---|---|
| Pembangkitan | `crypto/rand`, 16 byte → 22 karakter base64url tanpa padding |
| Penyimpanan | **Plaintext.** Tidak di-hash, tidak ada `secret_hash` |
| Perbandingan | Kesetaraan biasa lewat query `WHERE public_key = $1`. **Bukan** `subtle.ConstantTimeCompare` |
| Prefix | `pk_` — sengaja **tidak** menyerupai `jln_live_`/`jln_test_` |
| Rotasi | Tidak ada di phase ini. Menonaktifkan form (`deleted_at`) adalah cara mencabutnya |

**Kenapa plaintext, dan kenapa itu bukan kelalaian.** `public_key` memang dirancang untuk terbaca semua
orang — ia tertanam di HTML halaman pelanggan. Meng-hash-nya akan (a) membuat lookup mustahil tanpa
menyimpan nilai mentahnya juga, dan (b) menyesatkan siapa pun yang membaca skema, seolah bocornya
adalah insiden. Yang melindungi endpoint ini **bukan** kerahasiaan kunci, melainkan daftar kemampuan
yang tertutup (§4) plus lima lapis anti-abuse (§6).

**Kenapa bukan `subtle.ConstantTimeCompare`.** Perbandingan waktu-tetap melindungi dari serangan
timing yang menebak rahasia. Di sini tidak ada rahasia untuk ditebak. Memakainya akan menyiratkan
ancaman yang tidak ada.

> Aturan #20 mengikat **password (argon2id)** dan **API key (SHA-256 + `ConstantTimeCompare`)**.
> `public_key` bukan keduanya — ADR-005 sudah menempatkannya di baris ketiga tabel kredensial freeze
> 5.1 dengan kolom *"Punya identitas orang? ❌"* dan kemampuan *"hanya submit"*.

---

## 3. Autentikasi — principal keempat

`tenant.PrincipalPublicForm` **sudah ada** di `internal/shared/tenant/tenant.go` sejak Phase 1 dengan
**nol pemakaian**. Phase 6 adalah momen itu.

`tenant.Context` bertambah satu field, mengikuti persis cara `APIKeyID` ditambahkan sebelum Phase 4:

```go
// FormID is only ever populated when PrincipalType == PrincipalPublicForm
// (Phase 6). Like APIKeyID, it is the id of the credential that resolved
// this request, and lead.Usecase.Create reads it to stamp
// leads.source_form_id — never from the request body (Rule #5).
FormID *uuid.UUID
```

### Resolusi — tidak lewat `Authorization`

Berbeda dari API key, `public_key` **tidak pernah** dikirim sebagai bearer token. Ia ada di **path**:

```
POST /v1/forms/{public_key}/submit
```

Konsekuensinya: **tidak ada middleware auth yang dipasang** di route ini. Handler-nya sendiri yang
memanggil `form.Usecase.ResolvePublicKey(ctx, key) (tenant.Context, error)`. Ini disengaja —
`authn.MiddlewareWithAPIKey` membaca `Authorization` dan akan salah memperlakukan request tanpa header
sebagai anonim.

`ResolvePublicKey` mengembalikan `tenant.Context{OrganizationID, PrincipalType: PrincipalPublicForm,
FormID: &found.ID}`. `MembershipID`, `UserID`, `Role`, `Scopes`, dan `APIKeyID` **seluruhnya kosong** —
form tidak punya identitas orang, dan `Scopes` secara eksplisit hanya milik principal API key.

Setiap kegagalan (kunci tidak ada, form dihapus, organization tidak aktif) mengembalikan **error yang
identik** `404 not_found` — bukan `401`. Kunci yang salah dan form yang dihapus tidak boleh bisa
dibedakan.

---

## 4. Otorisasi — cabang ketiga, daftar tertutup

`authz.Require` hari ini bercabang dua: `PrincipalAPIKey` → `apiKeyScopeFor`, selain itu →
`permissions[Role]`. Phase 6 menambah cabang **ketiga**, bukan menyisipkan baris ke peta API key:

```go
// publicFormAllows is the complete list of what a public form key can do.
// One line, and it must stay that way: the credential is embedded in
// every page that installs the form, so anything reachable here is
// reachable by anyone who views source (ADR-005).
var publicFormAllows = map[Action]bool{
	ActionLeadCreate: true,
}

func Require(t tenant.Context, action Action) error {
	if t.PrincipalType == tenant.PrincipalPublicForm {
		if !publicFormAllows[action] {
			return forbiddenError()
		}
		return nil
	}
	if t.PrincipalType == tenant.PrincipalAPIKey { /* unchanged */ }
	/* role path unchanged */
}
```

**Deny-by-absence, sama seperti `apiKeyScopeFor`.** Action baru yang lahir di phase manapun otomatis
tertutup untuk form tanpa siapa pun perlu menuliskan pengecualiannya. Kriteria #2 PRD diuji sebagai
**tabel atas seluruh `authz.Action`**, bukan daftar tulisan tangan — pola yang sama yang dituntut
review #47.

Dua peta tidak digabung karena keduanya menjawab pertanyaan berbeda: API key punya *scope yang bisa
dipilih pelanggan*; form punya *satu kemampuan tetap*. Menyatukannya berarti suatu hari seseorang
menambah scope ke API key dan tanpa sadar memberikannya juga kepada setiap form yang terpasang.

---

## 5. `POST /v1/forms/{public_key}/submit` — alur

```
1. Rate limit per IP        → 429 + X-RateLimit-* bila lewat
2. Rate limit per public_key → 429 + X-RateLimit-* bila lewat
3. Baca body, cap 32KB       → 413 bila lewat
4. ResolvePublicKey          → 404 bila tidak ketemu / dihapus
5. Origin allowlist          → 403 bila di luar daftar
6. Honeypot terisi?          → 200 SUKSES PALSU, tidak ada lead
7. Token time-trap           → 400 bila <2 detik atau >30 menit atau tanda tangan salah
8. CAPTCHA (bila turnstile)  → 400 bila gagal
9. Validasi field wajib      → 400 + details per field
10. lead.Usecase.Create      → 201
```

Urutan itu **mengikat**. Rate limit lebih dulu dari segalanya (termasuk sebelum body dibaca) supaya
banjir request tidak pernah sampai ke database — persis alasan yang sama di jalur API key Phase 4.
Honeypot di posisi 6, **sebelum** validasi field, supaya bot tidak pernah menerima pesan kesalahan yang
bisa dipelajari.

### Pembuatan lead (keputusan D5)

Memakai ulang `lead.Usecase.Create` apa adanya. `CreateLeadInput` diisi handler dengan:

- `Source` = `"form"` — **dipaksa**, tidak pernah dari body
- `SourceFormID` = `t.FormID` — dari principal, tidak pernah dari body
- `RawPayload` = byte mentah request, apa adanya (kriteria #13)
- `AssignedToMembershipID` = **ditolak** bila ada di body, sama seperti jalur API key

`lead.Usecase.Create` bertambah cabang untuk `PrincipalPublicForm` yang bentuknya sama dengan cabang
`PrincipalAPIKey` yang sudah ada.

**Tidak ada idempotency key.** Browser pengunjung tidak mengirimnya, dan submit ganda dari manusia yang
menekan tombol dua kali dilindungi rate limit per IP, bukan oleh key yang tidak pernah dikirim.

---

## 6. Lima lapis anti-abuse (ADR-005)

Anti-spam di sini adalah **fitur ekonomi**, bukan sekadar keamanan: di harga terjangkau, setiap lead
spam adalah biaya yang ditanggung Jualin.

| Lapis | Mekanisme | Kejujuran tentang kekuatannya |
|---|---|---|
| **Origin allowlist** | Bandingkan header `Origin` dengan `forms.allowed_origins` | `Origin` **bisa dipalsukan** oleh klien non-browser. Ini menghalangi penyalahgunaan biasa, bukan penyerang tertarget — karena itu ia berpasangan, tidak pernah berdiri sendiri |
| **Honeypot** | Field tersembunyi CSS; terisi → buang **diam-diam**, balas seperti sukses | Efektif terhadap bot naif. Bot yang merender CSS akan melewatinya |
| **Time-trap** | Token HMAC berisi `form_id` + waktu terbit; tolak <2 detik atau >30 menit | **Time-trap, bukan anti-replay.** Token sah masih bisa dipakai berulang dalam 30 menit; yang membatasi pengulangan adalah rate limit |
| **Rate limit** | `FixedWindow` per IP **dan** per form (D4) | Dua sumbu: per-IP menghalangi satu bot, per-form menghalangi botnet terhadap satu pelanggan |
| **CAPTCHA** | Cloudflare Turnstile, `CAPTCHA_PROVIDER` (D2) | Lapis terkuat, dan satu-satunya yang butuh pihak ketiga |

### Token time-trap — bentuk konkret

```
token = base64url(issued_at_unix) + "." + base64url(HMAC-SHA256(secret, form_id + "|" + issued_at))
```

`secret` dari env `FORM_TOKEN_SECRET` (wajib, min 32 byte, divalidasi saat boot — Aturan #36).
**Bukan** `JWT_SECRET`: dua kegunaan berbeda tidak berbagi kunci, supaya rotasi salah satunya tidak
memaksa rotasi yang lain.

Stateless — tidak ada tabel nonce. Konsekuensi yang diterima sadar: token yang sah bisa dipakai
berkali-kali dalam jendelanya. Menutupnya butuh penyimpanan nonce, dan itu tidak sebanding untuk
ancaman yang rate limit sudah tangani.

### `CAPTCHA_PROVIDER` (keputusan D2)

```
CAPTCHA_PROVIDER=none|turnstile     # none ditolak saat APP_ENV=production
TURNSTILE_SITE_KEY=…                # wajib bila turnstile
TURNSTILE_SECRET_KEY=…              # wajib bila turnstile
CAPTCHA_TIMEOUT=5s
```

Bentuknya persis `MailProvider`/`PushProvider`: interface `captcha.Verifier` dengan `NoopVerifier`
(selalu lolos) dan `TurnstileVerifier` (HTTP POST ke `siteverify`). Validasi konfigurasi mengikuti pola
`config.Validate` yang sudah ada.

`TURNSTILE_SECRET_KEY` **tidak pernah** di-log (Aturan #26). `TURNSTILE_SITE_KEY` publik — ia memang
tertanam di halaman embed.

---

## 7. Halaman embed (keputusan D1)

```
GET /embed/{public_key}
```

Namespace **di luar** `/v1` — ia bukan API, ia halaman. Tidak ada middleware auth, tidak ada envelope
`{data, meta}`.

| Aspek | Ketentuan |
|---|---|
| Rendering | `html/template` + `//go:embed` — paket baru `internal/form/template/` |
| Escaping | `html/template` melakukan escaping kontekstual otomatis. **Label field berasal dari input pelanggan** dan dirender ke HTML — ini satu-satunya jalur XSS di phase ini, dan `text/template` **dilarang** di sini |
| CSP | `default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors <allowlist form ini>` |
| `X-Frame-Options` | **Tidak dikirim** — ia tidak mengenal daftar origin, dan `frame-ancestors` menggantikannya sepenuhnya |
| Cache | `Cache-Control: no-store` — halaman memuat token time-trap yang berumur pendek |
| JavaScript | Hanya bila `CAPTCHA_PROVIDER=turnstile` (script Turnstile). Tanpa CAPTCHA, form murni HTML + `<form method="post">` |

`frame-ancestors` **per-form**, diambil dari `forms.allowed_origins` baris itu — bukan satu kebijakan
global. Ini justru lebih ketat daripada host statis, yang hanya bisa mengirim satu header untuk semua
pelanggan.

Bila `allowed_origins` kosong: `frame-ancestors 'none'`. Form yang belum dikonfigurasi **tidak bisa**
di-iframe di mana pun — gagal tertutup, bukan terbuka.

### Kemampuan baru yang perlu disadari

Backend hari ini **tidak pernah** menyajikan HTML: nol `html/template`, nol `http.FileServer`, nol
`.html` di repo. Seluruh response melewati `httpx.OK`/`httpx.WriteError` menjadi JSON. Menambahkan
halaman ini berarti kelas kemampuan baru dengan urusan barunya sendiri (CSP, caching, escaping) —
karena itu ia **issue tersendiri**, bukan tempelan pada issue submit.

---

## 8. Endpoint

| Method | Path | Principal | Otorisasi |
|---|---|---|---|
| `POST` | `/v1/forms` | user | `form.create` — Owner/Admin |
| `GET` | `/v1/forms` | user | `form.list` — Owner/Admin |
| `GET` | `/v1/forms/:id` | user | `form.read` — Owner/Admin |
| `PATCH` | `/v1/forms/:id` | user | `form.update` — Owner/Admin |
| `DELETE` | `/v1/forms/:id` | user | `form.delete` — Owner/Admin (soft delete) |
| `POST` | `/v1/forms/{public_key}/submit` | **public_form** | `lead.create` lewat `publicFormAllows` |
| `GET` | `/embed/{public_key}` | — | tanpa autentikasi |

**Tabrakan wildcard gin.** `/v1/forms/:id` (user) dan `/v1/forms/{public_key}/submit` (publik) berbagi
prefix. Gin menuntut **nama wildcard yang sama** pada segmen yang sama — preseden persis ada di
`internal/task` dan `internal/activity` yang berbagi `/v1/leads/:id`. Karena itu route publik
didaftarkan sebagai `/v1/forms/:id/submit` dengan handler yang memperlakukan `:id` sebagai
`public_key`, bukan UUID. Dicatat di sini karena ini **jebakan yang pasti ditemui saat implementasi**.

Lima endpoint pengelolaan memakai `authMW` biasa. Endpoint submit dan embed **tidak** memasang
middleware auth apa pun (§3).

---

## 9. Error code baru

| Status | Code | Kapan |
|---|---|---|
| `403` | `origin_not_allowed` | `Origin` di luar `forms.allowed_origins` |
| `400` | `form_token_invalid` | Token time-trap salah tanda tangan, terlalu cepat (<2s), atau kedaluwarsa (>30m) |
| `400` | `captcha_failed` | Turnstile menolak |

`404 not_found`, `413 payload_too_large`, `429 rate_limited`, dan `400 validation_failed` sudah ada di
katalog `api.md` sejak Phase 4 — dipakai ulang apa adanya.

---

## 10. Dashboard

### Connect surface

`NAV_ITEMS` bertambah satu antara Tim dan Pengaturan:

```ts
{ href: "/connect", label: "Connect" }
```

`nav.test.ts` mengunci daftar itu **persis** (`toEqual([...])`) — test ikut diperbarui, dan itu memang
yang diinginkan: perpindahan menu tidak boleh lolos diam-diam. `NAV_ICONS` di `app-shell.tsx` juga
bertambah entri (`Plug` atau serupa); ikon yang hilang tidak membuat crash (`{Icon ? … : null}`) tapi
akan terlihat kosong.

`pageTitle` memakai pencocokan href-terpanjang-dulu, jadi `/connect` saja sudah benar untuk
`/connect/api`. Waspadai tabrakan prefix — `nav.test.ts` sudah punya penjaga sejenis
(`isActive("/teams-report", "/team") === false`).

| Route | Isi |
|---|---|
| `/connect` | Tiga kartu kanal: API (aktif), Formulir (aktif), Webhook (**belum tersedia** — Phase 7) |
| `/connect/api` | Pindahan `settings/api-keys/` |
| `/connect/api/docs` | Pindahan `settings/api-keys/docs/` |
| `/connect/form` | Daftar & pengelolaan form |

Kartu Webhook menampilkan *"belum tersedia"* — **bukan** *"terkunci oleh paket"*. Ia belum ada; keadaan
terkunci-oleh-paket baru lahir di Phase 8 (ADR-012 §4). Menyatakannya terkunci sekarang akan berbohong.

### Yang patah saat folder dipindah

Disebut eksplisit karena ketiganya diam saat dilewatkan:

1. `docs/api-docs-screen.tsx` mengimpor `../create-api-key-dialog` secara **relatif**
2. Lima `router.push("/settings/api-keys…")` tersebar di `api-keys-screen.tsx`, `docs/api-docs-screen.tsx`, `settings-screen.tsx`
3. `nav.test.ts` assertion persis

`/settings/api-keys` dan `/settings/api-keys/docs` menjadi **redirect permanen** ke lokasi baru — bukan
dihapus (kriteria #11).

### Manajemen form

Layar `/connect/form`: daftar form, tombol buat, dan editor per form (nama, toggle+label 6 field,
allowlist domain, snippet embed dengan tombol salin, tombol nonaktifkan).

Gerbang role: `canManageForms(role)` di `src/lib/form-permissions.ts`, di sebelah `canManageAPIKeys`
yang sudah ada di `src/lib/api-key-rows.ts`. Owner/Admin — sama dengan API key, karena keduanya
kredensial yang memasukkan lead ke organization.

**Nav tidak difilter role** (D6): menu Connect terlihat semua orang, gerbangnya di dalam layar —
persis perilaku `/settings` hari ini.

---

## 11. Paket baru

```
crm_be/internal/form/
    entity.go              Form, FieldConfig, generate(), parsePublicKey()
    port.go                Repository, AuditRepository, Repos, Store
    usecase.go             CRUD + ResolvePublicKey + Submit orchestration
    repository_postgres.go FindByPublicKey (tanpa tenant.Context — §3)
    handler_http.go        5 endpoint kelola + submit + embed
    template/form.gohtml   halaman embed (//go:embed)

crm_be/internal/shared/captcha/
    captcha.go             interface Verifier + NoopVerifier
    turnstile.go           TurnstileVerifier

crm_be/internal/shared/formtoken/
    formtoken.go           Issue(formID) / Verify(token, formID) — HMAC (§6)

crm_be/cmd/api/form_store.go   newFormStore(pool) form.Store
```

`crm_dashboard/src/lib/forms.ts` (klien API tipis) dan `form-permissions.ts` (gerbang role), mengikuti
`api-keys.ts` / `api-key-rows.ts`.

---

## 12. Rencana test

| Berkas | Menguji |
|---|---|
| `internal/form/entity_test.go` | Format `public_key`, panjang, prefix, keacakan; parsing field config |
| `internal/form/usecase_unit_test.go` | CRUD + `ResolvePublicKey` (semua kegagalan → 404 identik) |
| `internal/form/repository_test.go` | Postgres asli: `FindByPublicKey` memakai index (`EXPLAIN`), soft delete tidak ikut terambil |
| `internal/form/handler_test.go` | Urutan §5 ditegakkan; honeypot → 200 tanpa lead; time-trap; origin allowlist |
| `internal/shared/formtoken/formtoken_test.go` | Tanda tangan valid/invalid, batas <2s dan >30m, token form lain ditolak |
| `internal/shared/captcha/turnstile_test.go` | Bentuk request `siteverify`, penanganan gagal/timeout |
| `internal/shared/authz/authz_test.go` | **Tabel atas seluruh `Action`**: `PrincipalPublicForm` hanya lolos `ActionLeadCreate` (kriteria #2) |
| `cmd/api/public_form_api_test.go` | Ujung ke ujung dengan form yang di-seed lewat store sungguhan — meniru `public_lead_api_test.go` |
| `cmd/api/tenant_isolation_test.go` | **Kasus baru**: submit ke `public_key` milik org lain, dan `GET/PATCH/DELETE /v1/forms/:id` lintas org → 404. **Tetap harus terbukti bisa gagal** |
| `crm_dashboard/src/lib/nav.test.ts` | Diperbarui untuk "Connect" |
| `crm_dashboard/src/lib/form-permissions.test.ts` | Gerbang role |

Test komponen React **tidak ada dan tidak dijanjikan** — `vitest.config.mts` sengaja hanya
`include: ["src/**/*.test.ts"]` tanpa `.tsx`. Logika yang bisa salah diam-diam ditempatkan di
`src/lib/*.ts` supaya bisa diuji tanpa merender.

### Verifikasi manual wajib

Kriteria #1 dan #10 hanya bisa dibuktikan dengan **menempel snippet ke halaman HTML kosong** lalu
mengisinya dari browser — bukan dengan `curl`, dan bukan dengan membaca snippet-nya. Prosedurnya masuk
`docs/testing/flow/` sebagai berkas baru saat issue penutup.

---

## 13. Konfigurasi

```
FORM_TOKEN_SECRET=…              # wajib, min 32 byte (Aturan #36)
CAPTCHA_PROVIDER=none|turnstile  # none ditolak saat APP_ENV=production
TURNSTILE_SITE_KEY=…             # wajib bila turnstile
TURNSTILE_SECRET_KEY=…           # wajib bila turnstile
CAPTCHA_TIMEOUT=5s
FORM_SUBMIT_RATE_LIMIT_IP=20     # per menit, per IP
FORM_SUBMIT_RATE_LIMIT_FORM=60   # per menit, per form
EMBED_BASE_URL=…                 # dipakai dashboard membangun snippet
```

Keduanya angka rate limit **bukan hasil pengukuran** — sama jujurnya dengan `PUBLIC_API_RATE_LIMIT=60`.

---

## 14. Yang harus disiapkan pemilik produk

**Cloudflare Turnstile** — gratis, hitungan menit, tapi butuh akun:

1. [dash.cloudflare.com](https://dash.cloudflare.com) → Turnstile → *Add site*
2. Domain: domain tempat form akan dipasang (boleh `localhost` untuk pengembangan)
3. Widget mode: **Managed** (tanpa puzzle — ADR-005 memilihnya justru karena ini)
4. Salin **Site Key** → `TURNSTILE_SITE_KEY`, **Secret Key** → `TURNSTILE_SECRET_KEY`

**Memblokir satu issue saja** (#3, verifikasi anti-spam sungguhan), bukan phase-nya —
`CAPTCHA_PROVIDER=none` membuat empat issue lain jalan penuh tanpa akun Cloudflare.

---

## 15. Yang berubah pada dokumentasi

| Berkas | Perubahan |
|---|---|
| `architecture/api.md` | Tiga error code baru (§9); bab endpoint submit publik |
| `architecture/authentication.md` | Baris **ketiga** tabel kredensial dari rencana jadi kenyataan; `public_key` lewat path, bukan `Authorization` |
| `architecture/authorization.md` | Matriks Phase 6 (`form.*`); bagian *"Daftar tertutup — principal public form"* di sebelah bagian scope Phase 4 |
| `architecture/multi-tenancy.md` | Pengecualian unik-lintas-organization **keempat** (`forms.public_key`); kasus isolasi baru |
| `docs/testing/flow/` | Berkas baru: memasang form di halaman kosong dan mengisinya dari browser |
| `STATUS.md` | Baris Selesai; Phase 6 di *Progress per Phase*; **kunci Turnstile** masuk *Punya Lead Time* |

`freeze.md` **tidak disentuh.** Penyimpangan D1 dari ADR-005 dicatat di `prd.md` sebagai keputusan
phase beserta kewajiban deploy-nya — bukan diselipkan seolah ADR memang mengizinkannya (Aturan #30).

---

## 16. Risiko teknis

| Risiko | Penanganan |
|---|---|
| **XSS lewat label field** — label dari pelanggan dirender ke HTML | `html/template` (escaping kontekstual otomatis), **bukan** `text/template`. Ditegaskan di §7 dan diuji |
| `Origin` bisa dipalsukan klien non-browser | Diakui terbuka di ADR-005 dan §6. Ia satu dari lima lapis, tidak pernah berdiri sendiri |
| Tabrakan wildcard gin pada `/v1/forms/:id` | Sudah dipetakan di §8 dengan preseden `task`/`activity` |
| Halaman HTML pertama di backend membawa urusan baru (CSP, cache, escaping) | Dipisah jadi issue tersendiri (#4) supaya direview sebagai kemampuan baru, bukan sebagai tempelan |
| `public_key` plaintext terlihat seperti kelalaian saat review | Alasannya ditulis di §2 **dan** di komentar migration, bukan hanya di ADR yang mungkin tidak dibuka reviewer |
| Memindahkan `/settings/api-keys` memutus tautan pelanggan Phase 4 | Redirect permanen, kriteria #11, dan tiga titik yang patah disebut eksplisit di §10 |
| Rate limit per IP salah di belakang proxy | `SetTrustedProxies` sudah ditegakkan sejak Phase 4.5 (#57); `c.ClientIP()` sudah benar selama `TRUSTED_PROXIES` diisi tepat |

---

## 17. Kewajiban yang diteruskan ke phase berikutnya

- **Saat deployment dikerjakan**: halaman embed **wajib** dari hostname berbeda dari dashboard
  (`prd.md` D1). Menyajikannya dari origin yang sama membatalkan isolasi ADR-005.
- **Phase 7 (Webhook)**: kanal ketiga menempel pada `Connect` yang sudah ada — jangan buat menu baru.
  Kredensialnya adalah yang **keempat**; jangan satukan dengan `public_key` maupun API key.
- **Phase 8 (Subscription)**: `submit_count` di `forms` **bukan** usage counter. Kuota adalah
  `usage_counters`, mekanisme terpisah (TD Phase 4 §19).
- **Bila pelanggan meminta form builder dinamis**: itu bukan perluasan `fields` JSONB, melainkan fitur
  tersendiri dengan schema field + validasi + renderer (ADR-005).
