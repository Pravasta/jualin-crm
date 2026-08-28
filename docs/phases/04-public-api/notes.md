# Phase 4 — Public API · Notes

One section per issue, appended as each is implemented.

---

## #46 — Migration 0005, domain `api_key`, CRUD kredensial

### Keputusan implementasi

- **256-bit secret, bukan "32char"** — ADR-004 menulis `secret: 32char` dan `entropi 256-bit` pada baris
  yang sama; keduanya tidak bisa benar bersamaan (32 karakter base64url ≈ 192 bit). Diambil 256-bit → 32
  byte → 43 karakter base64url, karena seluruh argumen "SHA-256 aman di sini" bersandar pada angka itu.
  Sudah dicatat di TD §2 sebelum implementasi (PR #50); dikonfirmasi lagi di sini setelah kode ditulis —
  `TestGenerate_ProducesExpectedLengths` mengunci panjang 12/43/65 karakter secara eksplisit.
- **Parsing kredensial pakai slicing posisional, bukan `strings.Split(raw, "_")`** —
  `base64.RawURLEncoding` memakai `-` dan `_` sebagai pengganti `+`/`/`, jadi `key_id` atau `secret` bisa
  secara sah mengandung `_`. Total panjang kredensial selalu tetap (65 karakter: `4+5+12+1+43`), jadi
  `parseCredential` memvalidasi lewat panjang + irisan indeks. `TestParseCredential_SecretContainingUnderscore`
  membangun secret yang sengaja mengandung `_` dan membuktikan ia tetap terparsing benar — kasus yang
  akan gagal diam-diam pada implementasi split-based.
- **`verifySecret`/`FindByKeyID` dibangun sekarang, tidak dipanggil dari HTTP path manapun di #46.**
  Acceptance criterion #3 issue ini ("lookup kredensial adalah index hit") butuh query nyata untuk
  di-`EXPLAIN`, jadi `FindByKeyID` ada dan diuji langsung — tapi tidak ada middleware yang memanggilnya.
  #47 menyambungkannya ke `authn.APIKeyResolver`. Pengecualian yang sama seperti
  `invitation.Repository.FindValidByHash` dan `RefreshTokenRepository.FindByHashForUpdate`: tidak
  menerima `tenant.Context` karena organization adalah *hasil* lookup ini, bukan input untuknya.
- **`key_id` unik lintas organization** (`uq_api_keys_key_id`, bukan composite dengan `organization_id`)
  — pengecualian sadar terhadap kebiasaan "unik per tenant", dicatat sebagai komentar di migration.
  Constraint ini otomatis memberi index untuk `FindByKeyID` tanpa index terpisah.
- **`internal/apikey/entity_test.go` satu-satunya file di paket ini yang pakai `package apikey` (internal),
  bukan `apikey_test`** — perlu akses langsung ke `generate`/`parseCredential`/`verifySecret`/`hashSecret`
  yang sengaja tidak diekspor (detail format, bukan permukaan publik domain). Codebase ini sebelumnya
  selalu memakai `<pkg>_test` seragam per paket; ini deviasi kecil yang disengaja, bukan kelalaian.
- **Manager dan Employee tidak punya akses sama sekali** ke ketiga endpoint (`api_key.create/list/revoke`)
  — bukan read-only seperti `membership.list` yang Manager punya. Kredensial yang bisa memasukkan lead ke
  organization, dan daftar integrasi mana yang hidup, bukan informasi level-baca biasa.
- **`Revoke` idempoten by design** — `Usecase.Revoke` memanggil `FindByID` dulu (404 untuk hilang/lintas
  organization), lalu `Repository.Revoke` menjalankan `UPDATE ... WHERE revoked_at IS NULL` yang aman
  dijalankan berkali-kali: nol baris terpengaruh pada revoke kedua bukan error, karena eksistensi sudah
  dipastikan panggilan pertama. `TestRepository_Revoke_Twice_StaysNilError` membuktikan `revoked_at`
  tetap di nilai pertama, tidak bergerak maju pada panggilan kedua.
- **Tidak ada `deleted_at`** pada `api_keys` (beda dari kebanyakan entity bisnis lain) — revoke ≠ hapus;
  kredensial yang direvoke tetap terbaca di daftar supaya Owner tahu ia pernah ada.

### Bug yang ditemukan sebelum commit

Draf pertama `TestHandler_Create_OwnerAndAdminAllowed` memakai `uuid.Must(uuid.NewV7())` acak sebagai
`membership_id` di token JWT untuk kedua role. Test gagal dengan `500 internal_error`, bukan `201` —
`created_by_membership_id` menegakkan **composite FK sungguhan** (`fk_api_keys_created_by`) ke
`memberships (id, organization_id)`, dan membership acak itu tidak pernah benar-benar ada di database.
Diperbaiki dengan menyeed membership sungguhan (`seedMembership`, role sesuai) sebelum mencetak token —
kegagalan ini justru membuktikan FK-nya bekerja, bukan sekadar disimulasikan.

### Verifikasi harness isolasi tenant — terbukti bisa gagal

Mengikuti prosedur #11/#23/#30: `internal/apikey/repository_postgres.go`'s `FindByID` diubah sementara
dari `WHERE id = $1 AND organization_id = $2` menjadi `WHERE id = $1` (predikat tenant dihapus), lalu
`go test ./cmd/api/... -run TestTenantIsolation_CrossOrgMutatingByID_Returns404 -v` dijalankan ulang.

Hasil — **merah tepat pada subtest api_key, seluruh subtest lain tetap hijau**:

```
tenant_isolation_test.go:259: expected 404 for cross-org access, got 204:
--- FAIL: TestTenantIsolation_CrossOrgMutatingByID_Returns404/DELETE_/v1/api-keys/{id}_on_another_org's_api_key (0.01s)
```

Org A's owner berhasil mencabut kredensial milik Org B — kebocoran tenant nyata, bukan simulasi. Kode
dikembalikan ke bentuk asli (`diff` dikonfirmasi identik) sebelum commit; perubahan itu sendiri tidak
pernah masuk git.

### Verifikasi manual end-to-end

`docker compose up -d postgres` → `go run ./cmd/migrate up` (0001→0005 bersih) → `go run ./cmd/migrate
down` sekali (0005 turun bersih, status kembali ke 0004) → `go run ./cmd/migrate up` lagi → jalankan
`crm_be` lokal (`APP_ENV=development`) → register org "Toko ApiKey46" → verifikasi email (token dari
log `LogMailer`) → login (`client=dashboard`, cookie `HttpOnly` + `csrf_token`):

```
POST /v1/api-keys {"name":"Website Utama"}
→ 201 {..., "key_prefix":"jln_live_lXCy", "secret":"jln_live_lXCy_u3VeTB1_w0b8RpPA5ZTQUtghSdxJEvmKgyynPatO8o1wy-21OzI"}
   (panjang secret 65 karakter — dikonfirmasi dengan python3 len(), cocok totalCredentialLen)

GET /v1/api-keys
→ 200 [{..., "key_prefix":"jln_live_lXCy", ...}]   -- TANPA field "secret" atau "secret_hash"

DELETE /v1/api-keys/{id}  → 204
DELETE /v1/api-keys/{id}  → 204   (revoke kedua, idempoten, TD §9)

GET /v1/api-keys
→ 200 [{..., "revoked_at":"2026-08-27T13:14:16.162546+07:00", ...}]  -- kunci revoked TETAP tampil
```

`psql` langsung ke tabel `api_keys` setelah create: kolom `secret_hash` berisi hex sha256
(`2465ea91e26ba868e5264661d99ee86cd693c22091bf40a343c732634cdd8376`), **bukan** plaintext; `key_id`
(`lXCy_u3VeTB1`) 12 karakter sesuai spesifikasi.

### Test

33 test baru di `internal/apikey` (8 `entity_test.go` unit murni, 12 `usecase_unit_test.go` fake `Store`
tanpa Docker, 7 `repository_test.go` Postgres asli termasuk `EXPLAIN` index-hit, 9 `handler_test.go` HTTP
end-to-end termasuk role matrix 4×3, buffer log untuk Aturan #26, dan audit log). Satu `isolationCase`
baru di `cmd/api/tenant_isolation_test.go` (`DELETE /v1/api-keys/{id}`), terbukti bisa gagal seperti di
atas.

### Batas issue ini

Autentikasi **dengan** kredensial `jln_*` (middleware, cabang `PrincipalAPIKey` di `authz.Require`,
`POST /v1/leads` jalur API key, rate limit, retensi `idempotency_key`) — seluruhnya #47. Layar dashboard
— #48. Halaman dokumentasi — #49.

---

## #47 — Autentikasi API key, `POST /v1/leads` publik, rate limit, idempotency

Risiko keamanan tertinggi Phase 4: satu peta otorisasi salah dan kredensial yang bocor dari website
statis pelanggan bisa berubah menjadi pengambilalihan organization. `internal/apikey`'s `FindByKeyID`,
`verifySecret`, `parseCredential` (dibangun #46 tanpa pemanggil) disambungkan di sini lewat
`apikey.Usecase.ResolveAPIKey`.

### Penyimpangan TD yang disengaja

**TD §5 menulis `CreateLeadInput + SourceAPIKeyID *uuid.UUID`. Field itu TIDAK ditambahkan.**
`source_api_key_id` diturunkan `lead.Usecase.Create` langsung dari `t.APIKeyID` (tenant.Context yang
sudah terautentikasi), bukan dari argumen input — field di `CreateLeadInput` akan menjadi celah yang
sama seperti "organization_id dari body" (Aturan #5): kalau pernah ada, sesuatu di masa depan bisa
lupa dan mempercayainya dari tempat yang salah alih-alih dari principal. `RawPayload []byte` tetap
ditambahkan persis seperti TD — itu memang harus datang dari handler (satu-satunya yang punya akses ke
body mentah), bukan dari `t`.

### Keputusan implementasi

- **`authz.Require` bercabang di `t.PrincipalType`, bukan di `t.Role`** — principal `api_key` dicek
  terhadap peta `apiKeyScopeFor` (satu baris: `lead.create → leads:write`) dan **tidak pernah**
  menyentuh peta `permissions[role]`. `InsufficientScopeError()` diekspor supaya `internal/lead` bisa
  memakai kode yang identik untuk satu aturan bisnis yang tidak lewat `authz.Require` sama sekali
  (penolakan `assigned_to_membership_id`) — pola yang sama seperti `customer.alreadyConvertedError`
  meniru kode `lead`.
- **Test tabel-atas-seluruh-`Action`** (`TestRequire_APIKeyPrincipal_OnlyLeadCreateAllowed`) mengulang
  26 `Action` yang ada, bukan daftar tulis tangan — action baru phase berikutnya otomatis ikut tertutup.
  **Ketemu gap nyata sambil menulisnya**: tiga `Action` `api_key.*` yang ditambahkan #46 **tidak pernah**
  dimasukkan ke tabel per-role `TestRequire` yang sudah ada — dibackfill di issue ini sebelum menulis
  test barunya, supaya test lama benar-benar mewakili matriks lengkap.
- **`ResolveAPIKey`**: `parseCredential` → `FindByKeyID` → cek `RevokedAt`/`ExpiresAt` → `verifySecret`.
  **Keempat jalur gagal mengembalikan `invalidAPIKeyError()` yang identik** — dibuktikan dua kali: unit
  test (`internal/apikey`) dan HTTP end-to-end (`cmd/api`), membandingkan string pesan persis, bukan
  hanya kode.
- **`MiddlewareWithAPIKey` dipasang HANYA pada `POST /v1/leads`.** Route lain (`GET /v1/leads`, dan
  semua domain lain) tetap `authn.Middleware` yang tidak mengenal `jln_*` sama sekali. **Konsekuensi yang
  ditemukan saat menulis test, bukan direncanakan sejak awal**: ini berarti API key yang dipakai di route
  lain gagal di lapisan **autentikasi** (`401 authentication_required` — parsing JWT gagal karena
  `jln_live_...` bukan JWT), **bukan** di lapisan otorisasi (`403`).
- **Deviasi dari teks acceptance criteria issue ini sendiri.** Checklist issue #47 menulis:
  *"GET /v1/leads, GET /v1/memberships, POST /v1/invitations, POST /v1/api-keys lewat API key → 403"*.
  Implementasi nyata (dan TD §3 sendiri) menghasilkan **401**, bukan 403 — TD §3 eksplisit menyebut
  alasannya: *"an endpoint that never wires this middleware in cannot be reached by an API key no matter
  what authz.Require would otherwise have allowed"*. Routing-level exclusion adalah bentuk Aturan #24
  yang **lebih kuat** daripada authz mengembalikan 403: memasang pengenalan API key di setiap route
  hanya demi kode status yang berbeda kosmetik akan membuat setiap route menanggung biaya lookup
  tambahan untuk kredensial yang seharusnya sudah gagal sebelum sampai sana. Teks acceptance criteria
  yang ditulis sesi lalu tidak konsisten dengan TD yang ditulis di sesi yang sama — diperbaiki di sini,
  bukan diikuti secara harfiah. Diuji langsung: `TestPublicLeadAPI_CannotReachAnyOtherEndpoint`
  (`cmd/api/public_lead_api_test.go`) membuktikan keempat endpoint mengembalikan `401
  authentication_required`, dan verifikasi manual (`curl`) mengonfirmasi hal yang sama.
- **Rate limit dievaluasi paling awal di handler**, sebelum body sama sekali dibaca — supaya header
  `X-RateLimit-*` benar-benar terpasang di **setiap** response jalur API key (termasuk `413`, `403`,
  validasi gagal), bukan hanya jalur sukses. Kunci limiter `publiclead:key:<api_key_id>`, bukan IP.
- **Body dibaca manual (`io.ReadAll` + `http.MaxBytesReader`) hanya untuk jalur API key** — jalur
  dashboard tetap `c.ShouldBindJSON` seperti sebelumnya, tidak disentuh. `raw_payload` disimpan dari
  body mentah sebelum di-`json.Unmarshal`, jadi field tak dikenal (`utm_source`, dst.) tersimpan apa
  adanya — dibuktikan lewat `curl` sungguhan (lihat di bawah).
- **Retensi `idempotency_key` sinkron, bukan goroutine** — TD §7 hanya mensyaratkan "tidak memblokir"
  dalam arti tidak menjadikan tabel *write hotspot*, bukan mewajibkan dispatch asinkron. Satu `UPDATE`
  terindeks per organization, di-throttle 1×/jam, error dibuang (tidak ada logger di paket `lead` untuk
  mencatatnya, dan TD eksplisit: kegagalan sweep tidak boleh menggagalkan lead yang sedang dibuat).
- **`last_used_at` juga sinkron** (bukan goroutine terpisah) — sama alasannya, throttle 5 menit/kunci.

### Bug nyata ditemukan lewat test

1. **`leadJSON` tidak pernah menyertakan `source_api_key_id`.** Test HTTP pertama untuk header rate-limit
   gagal karena field itu `<nil>` di response meski kolom di database sudah terisi benar — `handler_http.go`'s
   JSON builder ketinggalan menambahkannya. Diperbaiki sebelum lanjut ke test berikutnya; ini acceptance
   criterion eksplisit ("`source_api_key_id` terisi"), bukan detail kosmetik.
2. **Test fixture salah, bukan kode** — draf pertama beberapa test HTTP (`internal/lead` dan `cmd/api`)
   memakai `uuid.Must(uuid.NewV7())` acak sebagai `t.APIKeyID`, menabrak `fk_leads_source_api_key`
   (composite FK sungguhan dari migration `0005`) → `500`, bukan `201`. Sama persis kelas bug yang
   ditemukan di #46 untuk `fk_api_keys_created_by`. Diperbaiki dengan menyeed baris `api_keys` sungguhan
   (`seedAPIKey` di `internal/lead`, `seedRealAPIKey` di `cmd/api`) sebelum memakai id-nya.

### Verifikasi manual end-to-end

`docker compose up -d postgres` → `migrate up` (sudah di 0005, tidak ada migration baru di issue ini) →
jalankan `crm_be` lokal dengan `PUBLIC_API_RATE_LIMIT=5` (sengaja kecil untuk membuktikan `429`) →
register org "Toko PublicLead47" → verifikasi → login → `POST /v1/api-keys` → **kredensial dipakai dari
proses `curl` terpisah** (mensimulasikan "mesin di luar jaringan"):

```
POST /v1/leads  Authorization: Bearer jln_live_TG0u..._HaNo1QHI_qY3pxsz7s
  {"name":"Budi dari Website","email":"budi@example.com","utm_source":"facebook"}
→ 201, X-Ratelimit-Limit: 5, X-Ratelimit-Remaining: 4
  data.source = "api", data.source_api_key_id = <id kunci>

GET /v1/leads (via cookie session Owner)
→ lead di atas MUNCUL, source & source_api_key_id sama persis — dibuktikan lintas jalur

5 request beruntun lewat kredensial yang sama:
→ 201, 201, 201, 429 (Retry-After: 45, Remaining: 0), 429, 429
  (permintaan pertama sudah memakai 1 dari kuota 5 — pas di batas, tidak lebih tidak kurang)

DELETE /v1/api-keys/{id} (session Owner) → 204
POST /v1/leads dengan kredensial yang sama (setelah jendela rate limit lewat, 61 detik)
→ 401 invalid_api_key — seketika, tanpa jeda

grep raw secret di file log server → 0 kecocokan, termasuk pada request yang GAGAL

POST /v1/leads {"assigned_to_membership_id": "<membership Owner>"}
→ 403 insufficient_scope

POST /v1/leads dengan header Origin: https://toko-pelanggan.example
→ 201 (request tetap diproses — CORS ditegakkan browser, bukan server), TANPA satu pun
  header Access-Control-Allow-* di response
```

### Test

- `internal/shared/ratelimit`: `Take` (batas tepat, `Remaining`, `ResetAt` tetap dalam satu jendela)
- `internal/shared/authz`: tabel semua `Action` (26) untuk `PrincipalAPIKey`; scope kosong/salah; role
  Owner di context `PrincipalAPIKey` tetap ditolak (`Require` tidak pernah membaca `t.Role` untuk
  principal ini)
- `internal/apikey`: `ResolveAPIKey` sukses; empat skenario gagal → pesan identik; throttle
  `last_used_at` (1 panggilan `TouchLastUsed` dari 2 resolve dalam jendela); `TouchLastUsed` repository
  menulis kolom sungguhan
- `internal/lead`: unit — `assigned_to_membership_id` ditolak, `source` dipaksa `api`, `raw_payload`
  tersimpan, cleanup di-throttle per organization, cleanup **tidak pernah** terpicu untuk principal user.
  HTTP (fake `authn.APIKeyResolver`, lihat doc comment `fakeAPIKeyResolver`) — header rate-limit di
  sukses **dan** gagal, `413` sebelum parsing, `raw_payload` verbatim lewat query database langsung,
  `429` **tepat** di batas di bawah N request bersamaan, idempotent replay tepat 1 lead di bawah N
  request bersamaan lewat jalur API key (mengulang test #20, jalur kredensial baru)
- `cmd/api` (`public_lead_api_test.go`, router produksi penuh + `apikey.Usecase` sungguhan): create real
  key → lead `source=api`; revoke → `401` seketika; tiga penyebab gagal → pesan identik; empat endpoint
  lain → `401 authentication_required` (bukan 403, lihat deviasi di atas); CORS origin pelanggan → tanpa
  header; raw secret & isi payload tidak pernah di buffer log, termasuk pada request gagal

Tidak ada kasus baru di `tenant_isolation_test.go` — `POST /v1/leads` tidak menerima `:id` tenant lain;
`t.OrganizationID` datang dari hasil lookup kunci, tidak pernah dari input, sehingga tidak ada permukaan
untuk kebocoran lintas-tenant di endpoint ini secara struktural.

### Utang ditutup

Retensi `idempotency_key` (dicatat sejak #20, TD phase 2 §7/§19) — **selesai**. `docs/STATUS.md` bagian
Utang Teknis diperbarui.

---

## #48 — Dashboard: manajemen API key

Layar dashboard terakhir Phase 4 — Owner/Admin membuat dan mencabut kredensial `jln_*` tanpa `curl`.
Tiga endpoint dari #46 dikonsumsi apa adanya; #47 sama sekali tidak menyentuh bentuknya.

### Keputusan implementasi

- **Dua tahap dalam SATU komponen dialog (`CreateAPIKeyDialog`), bukan dua dialog terpisah.** Setelah
  `createAPIKey` sukses, `step` berpindah dari `"form"` ke `"reveal"` **di dalam komponen yang sama** —
  `secret` tidak pernah diteruskan ke `api-keys-screen.tsx` (parent), jadi tidak ada jalan sama sekali
  bagi secret untuk masuk ke state daftar atau context. Aturan #21 ditegakkan secara struktural
  (`APIKey` di `lib/api-keys.ts` tidak punya field `secret` sama sekali — hanya `CreatedAPIKey`, tipe
  terpisah yang hanya dikembalikan `createAPIKey`), bukan hanya disiplin penulisan kode.
- **Guard "penutupan mengharuskan konfirmasi" ada di `onOpenChange` itu sendiri, bukan cuma tombol
  footer.** `DialogContent` bawaan (`components/ui/dialog.tsx`) memicu `onOpenChange(false)` lewat tiga
  jalur berbeda — tombol X, Escape, klik backdrop (base-ui, bukan custom) — jadi memblokir hanya di
  tombol "Selesai" akan bisa dilewati lewat Escape. Diblokir di satu titik (`handleOpenChange`) yang
  ketiganya lewati; `showCloseButton={step !== "reveal"}` menyembunyikan tombol X saat reveal supaya
  tidak ada affordance yang terlihat bekerja padahal diam-diam diblokir.
- **"Menu masuk ke app shell" (checklist issue) diartikan sebagai Card baru di layar Settings**, bukan
  entri baru di `NAV_ITEMS` (`lib/nav.ts`). `NAV_ITEMS` sekarang tidak punya mekanisme per-role sama
  sekali — enam itemnya berlaku untuk semua role — sementara TD §12 menaruh path-nya sebagai sub-halaman
  Pengaturan (`/settings/api-keys`), bukan level teratas. `isActive`/`pageTitle` tetap menyorot
  "Pengaturan" di sidebar lewat prefix-match yang sudah ada tanpa perubahan di `nav.ts`.
- **Gerbang role di level fetch, bukan hanya UI.** `APIKeysScreen`'s `useEffect` yang memanggil
  `listAPIKeys()` di-skip sepenuhnya bila `!canManageAPIKeys(session.role)` — pola yang sama seperti
  `canManageTeam` menahan `listInvitations()` di #34. Memenuhi acceptance criterion "mengetik URL
  langsung tidak menampilkan daftar" secara harfiah, bukan cuma menyembunyikan tautannya di Settings.
- **`formatApproximateID(iso, now: Date)`** ditambah ke `lib/date.ts`, pola `now`-sebagai-parameter yang
  sama seperti `metrics.ts`'s `periodToRange` — `last_used_at` di-throttle 5 menit di backend (TD §10),
  jadi klien sengaja tidak pernah merender jam presisi (dikunci test:
  `not.toMatch(/\d{1,2}:\d{2}/)`).
- **`toAPIKeyRow`/`canManageAPIKeys`** dipisah ke `lib/api-key-rows.ts`, diuji tanpa React — pola yang
  sama seperti `lib/nav.ts`/`lib/team-permissions.ts`, diminta eksplisit di checklist issue.

### Tidak ada bug nyata ditemukan kali ini

Berbeda dari #32–#35 dan #46/#47, tidak ada deviasi TD/PRD atau bug lintas-paket yang ditemukan saat
mengerjakan issue ini — bentuk endpoint sudah stabil sejak #46 dan tidak berubah sama sekali di #47.
Tidak ada berkas baru di `docs/issues/` untuk issue ini (skill `jualin-issue-log`: "jangan buat berkas
kosong demi kelengkapan").

### Verifikasi

`npm run typecheck && lint && test && build` — bersih, 4 test file baru/diperluas (`api-key-rows.test.ts`
baru, `date.test.ts` +5 kasus). Manual terhadap `crm_be` sungguhan (`docker compose up -d postgres` +
`crm_be` lokal + `crm_dashboard` lokal): register → verify → login → `POST /v1/api-keys` menghasilkan
bentuk **persis** sama dengan `CreatedAPIKey` (`created_at, created_by_membership_id, expires_at, id,
key_prefix, last_used_at, name, revoked_at, scopes, secret`) → `GET /v1/api-keys` setelahnya **tidak**
membawa field `secret` sama sekali → revoke → `204`, kunci tetap muncul di list dengan `revoked_at`
terisi → `GET /v1/memberships`'s `id` cocok persis dengan `created_by_membership_id` kunci, membuktikan
kolom "Dibuat oleh" akan resolve ke nama yang benar. Verifikasi interaktif dialog (gerbang checkbox,
salin ke clipboard) lewat pembacaan kode + `build` sukses — klik-uji browser sungguhan di luar cakupan
TD §9 (test UI visual/e2e), sama seperti seluruh layar Phase 3.
