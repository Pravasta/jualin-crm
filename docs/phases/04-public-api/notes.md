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
