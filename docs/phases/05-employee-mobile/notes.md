# Phase 5 — Employee Mobile · Notes

One section per issue, appended as each is implemented.

---

## #68 — Backend mobile: `device_tokens`, pengiriman FCM, hook setelah commit

Issue pertama Phase 5 yang dikerjakan — murni `crm_be`, tanpa UI (lihat batas issue di `issues.md`).
Android-first (keputusan M1), FCM lewat HTTP v1 API langsung (keputusan M4, Rule #27 — meniru pilihan
`net/smtp` Phase 4.6 daripada menambah Firebase Admin SDK sebagai dependency), push dikirim **setelah**
commit (keputusan M5, Rule #32, freeze A3: baris `notifications` adalah source of truth, push cuma
usaha pengiriman terbaik).

### Migration `0006_device_tokens.sql` — pengecualian unique ketiga lintas organization

`uq_device_tokens_token` unik lintas organization, **bukan** composite `(token, organization_id)` —
pengecualian sadar terhadap kebiasaan "unik per tenant" di kode ini, dan yang **ketiga** dengan alasan
persis sama seperti dua pendahulunya (`api_keys.key_id` #46, `refresh_tokens.token_hash` Phase 1):
organization di sini adalah *hasil* lookup, bukan input untuknya. Tapi alasan device_tokens berbeda dari
keduanya — bukan soal urutan lookup, tapi soal **kepemilikan fisik yang bisa berpindah**: satu perangkat
Android bisa berpindah tangan (karyawan resign, HP kantor dipakai orang lain, reinstall app di
organization berbeda saat testing). Registrasi ulang token yang sama harus **memindahkan** baris ke
pemilik/organization baru lewat `ON CONFLICT (token) DO UPDATE`, bukan menolak atau menduplikasi.

Sengaja **tanpa `deleted_at`** (deviasi dari Aturan #18) — device token bukan entity bisnis yang perlu
riwayat; baris yang dihapus (logout, atau token FCM sudah dianggap invalid) memang seharusnya hilang,
tidak ada nilai menyimpan "device token yang pernah ada".

### `internal/shared/push` — meniru bentuk `internal/shared/mailer` persis

`Sender` interface + `NoopSender` (development, `PUSH_PROVIDER=none`) + `FCMSender` (`PUSH_PROVIDER=fcm`)
— bentuknya sama persis dengan `Mailer`/`LogMailer`/`SMTPMailer` Phase 4.6, termasuk validasi config
"tolak provider no-op di production" (Rule #36 tetap berlaku untuk push: proses yang diam-diam tidak
pernah mengirim push gagal tanpa error apa pun, sama seperti `MAIL_PROVIDER=log` di production).

`FCMSender.isUnregisteredError` membedakan token yang **benar-benar mati** (404 UNREGISTERED, 400
INVALID_ARGUMENT dengan `errorCode` itu di `details[]`) dari kegagalan sementara (429 rate limit, 500
internal, 400 dengan alasan lain seperti `THIRD_PARTY_AUTH_ERROR`) — hanya kasus pertama yang boleh
memicu penghapusan token di `device.Usecase.PushToMembership`. `ErrTokenInvalid` sentinel dipakai
`errors.Is` di pemanggil, bukan dicocokkan lewat string.

### `internal/device` — domain baru, `PushSender` dibridge lewat `lead.Repos`

`Register`/`Unregister`/`PushToMembership` (lihat kode untuk detail). Satu keputusan desain yang
menghindari kerusakan luas: `lead.PushSender` interface dideklarasikan di `internal/lead/port.go`
(ADR-011 — interface milik consumer), dan `device.Usecase` mengimplementasikannya secara struktural
tanpa `internal/lead` pernah mengimpor `internal/device`. Bridge-nya diletakkan di **`lead.Repos.Push`**
(field baru, nilai nil-safe), **bukan** di parameter `lead.NewUsecase` — pola yang sama seperti
`NotificationSender`/`ActivityRecorder` sebelumnya. Alasannya murni praktis: `lead.NewUsecase(store)`
dipanggil di ~20 lokasi test yang sudah ada; mengubah signature-nya berarti ~20 titik itu perlu
diperbarui untuk dependency yang sebagian besar tidak pernah mereka uji. Menaruhnya di `Repos` berarti
test lama yang membangun `Repos{}` literal tanpa field `Push` tetap kompilasi dan tetap lulus —
`pushAssignmentNotification` memeriksa nil sebelum memanggil apa pun.

`Unregister`'s pemeriksaan kepemilikan dilakukan di Go (`found.MembershipID != *t.MembershipID` → 404),
bukan dilipat ke SQL `DeleteByToken` — lihat bagian isolasi tenant di bawah untuk kenapa ini penting.

### `lead.Usecase.UpdateAssignment` — push ditangkap lewat closure, dikirim setelah `InTx` kembali

```go
var pushRecipient *uuid.UUID
txErr := u.store.InTx(ctx, func(r Repos) error {
    // ... assignment + Activity.Record + Notification.Notify di dalam transaksi ...
    if t.MembershipID != nil && newAssignee == *t.MembershipID {
        return nil // self-assignment — tidak ada notifikasi, tidak ada push (TD §11)
    }
    // ...
    pushRecipient = &newAssignee
    return nil
})
// ...
if pushRecipient != nil {
    u.pushAssignmentNotification(ctx, t, *pushRecipient, updated)
}
```

`pushRecipient` sengaja variabel di luar closure, diset di dalam, dibaca setelah `InTx` selesai —
memastikan kegagalan push (timeout FCM, token invalid, dst.) **tidak pernah** menggagalkan transaksi
yang sudah committed (Rule #32). Assign ke diri sendiri tetap diam total — TD §11: "memberi tahu
seseorang tentang tindakannya sendiri hanya menambah bising" — baris ini sekaligus berlaku untuk
notifikasi in-app (sudah ada sejak Phase 3) dan push (baru).

### Wiring composition root

`cmd/api/device_store.go` (baru, meniru `apikey_store.go`), `cmd/api/lead_store.go`'s
`newLeadStore(pool, push)` sekarang menerima `push lead.PushSender`, `cmd/api/main.go`'s `newPushSender`
membangun `push.NoopSender` atau `push.FCMSender` dari config lalu **panic** bila konstruksi FCM gagal
(kredensial file tidak terbaca, dst.) — bukan mengembalikan error lewat `newRouter`'s signature, supaya
6 file test yang memanggil `newRouter` langsung tidak perlu diubah. Konsisten dengan Rule #36 (fail-fast
sebelum server menerima koneksi): kegagalan ini terjadi sebelum `http.Server` mulai `ListenAndServe`.

Dependency baru satu-satunya: `golang.org/x/oauth2` (+ `cloud.google.com/go/compute/metadata` sebagai
indirect) — dikonfirmasi lewat `git diff go.mod`, sesuai janji TD "satu dependency baru".

### Bug ditemukan sebelum commit

- **Hang nyata di `fcm_internal_test.go`** — `TestFCMSender_Send_RespectsTimeout`'s handler awalnya
  memakai `<-r.Context().Done()` untuk mendeteksi client yang menyerah. Ini tidak reliable: server tidak
  selalu tahu client-nya sudah timeout kecuali sedang aktif membaca/menulis koneksi. Test ini
  benar-benar hang >180 detik di run background sebelum ditemukan (dikonfirmasi lewat goroutine dump —
  handler diam di `chan receive`). Diperbaiki dengan `time.Sleep(2 * time.Second)` tetap — jauh melewati
  timeout client (300ms) tapi handler-nya sendiri selalu selesai sesuai jadwalnya sendiri, tidak
  bergantung pada sinyal cancellation yang tidak terjamin datang.
- **Potensi panic di dua file test config literal** — setelah `newPushSender(cfg, log)` dipasang tanpa
  syarat di `newRouter`, `config.Config{}` literal (bukan lewat `env.Parse`) punya `PushProvider == ""`
  (bukan default `"none"` dari tag `envDefault`), yang jatuh ke cabang `default: panic(...)` di
  `newPushSender`. Ditemukan sebelum menjalankan test lewat `grep -rln "config.Config{"` — hanya dua
  situs (`router_test.go`'s `testConfig()`, `tenant_isolation_test.go`'s `isolationTestConfig()`).
  Ditambahkan `PushProvider: "none"` ke keduanya sebelum ini sempat menggagalkan test apa pun.
- **Empat test produksi di `config_test.go` gagal pada full-suite run**, bukan pada test package
  `config` sendirian saat pertama ditulis: `TestLoad_ProductionMode`,
  `TestLoad_ProductionRequiresCookieSecure`, `TestLoad_ProductionRequiresCORSAllowedOrigins`,
  `TestLoad_ProductionRequiresTrustedProxies` — keempatnya set `APP_ENV=production` tapi tidak pernah
  set `PUSH_PROVIDER`, sehingga jatuh ke default `"none"`, yang sekarang ditolak di production (validasi
  baru issue ini). Gate `PUSH_PROVIDER` baru ini datang **sebelum** gate yang sebenarnya ingin diuji
  test-test itu, jadi errornya menyebut `PUSH_PROVIDER`, bukan `COOKIE_SECURE`/`CORS_ALLOWED_ORIGINS`/
  `TRUSTED_PROXIES`. Diperbaiki persis seperti `MAIL_PROVIDER=smtp`+`SMTP_HOST` sudah dipatch ke test
  yang sama untuk gate `MAIL_PROVIDER` — ditambah `PUSH_PROVIDER=fcm`+`FCM_PROJECT_ID`+
  `FCM_CREDENTIALS_FILE` ke keempat test itu. Dua test produksi lain
  (`TestLoad_ProductionRejectsLogMailProvider`, `TestLoad_ProductionRejectsSMTPWithoutTLS`) tidak perlu
  ditambal — gate yang mereka uji (`MAIL_PROVIDER`, `SMTP_TLS`) ada **sebelum** gate `PUSH_PROVIDER` di
  urutan `validate()`, jadi error-nya sudah keluar lebih dulu.
- **Backfill wajib di `authz_test.go`** — dua `Action` baru (`ActionDeviceTokenRegister`,
  `ActionDeviceTokenDelete`) perlu masuk ke tabel per-role `TestRequire` dan `allActions`, mengikuti
  komentar peringatan yang sudah ada di file itu sejak precedent #46/#47 tentang kelas kesalahan ini.

### Verifikasi harness isolasi tenant — dua lapis proteksi redundan yang saling tidak sadar

Mengikuti prosedur #11/#23/#30/#46: harness baru
`TestTenantIsolation_DeviceTokenDelete_CrossOrgReturns404` menguji `DELETE /v1/device-tokens` dari Org A
terhadap token milik Org B, mengharapkan 404.

**Percobaan pertama TIDAK berjalan seperti yang lain.** Predikat tenant di `FindByToken`'s SQL
(`WHERE token = $1 AND organization_id = $2`) diubah sementara menjadi `WHERE token = $1` — dan test
**tetap hijau**, bukan merah. Ini bukan bug di test, melainkan temuan desain nyata: `Unregister`'s
pemeriksaan kepemilikan tingkat Go (`found.MembershipID != *t.MembershipID` → 404) bekerja independen
dari filter `organization_id` SQL — kasus lintas-organization *juga* selalu lintas-membership, jadi
lapis kedua menangkapnya sendirian.

Untuk membuktikan harness ini benar-benar bisa gagal, **kedua lapis** dirusak bersamaan (predikat SQL
dihapus **dan** pemeriksaan `found.MembershipID` di `Unregister` dilewati) — barulah merah:

```
tenant_isolation_test.go: expected 404 for cross-org access, got 204
```

Kode dikembalikan ke bentuk asli (diff dikonfirmasi identik) sebelum commit; kerusakan itu sendiri tidak
pernah masuk git. Dua test unit/handler yang sudah ada
(`TestUnit_Unregister_SomeoneElsesToken_Returns404NotForbidden`,
`TestHandler_Unregister_SomeoneElsesToken_Returns404`) sudah membuktikan lapis Go-level itu sendirian
bisa gagal untuk skenario yang sebenarnya jadi tanggung jawabnya — sama-organization, beda-membership,
kasus yang filter SQL organization_id sama sekali tidak bisa bantu.

**Kesimpulan**: bukan cacat desain, tapi proteksi berlapis yang muncul secara tidak sengaja dari dua
aturan berbeda yang sama-sama benar (Aturan #6 "404 bukan 403" diterapkan di lapis repository untuk
kasus lintas-organization, dan aturan bisnis "hanya pemilik boleh unregister" diterapkan di lapis
usecase). Dicatat di sini apa adanya, bukan digambarkan sebagai keberhasilan langsung — percobaan
pertama gagal menunjukkan kegagalan, dan itu sendiri temuan yang harus jujur ditulis.

### Verifikasi manual end-to-end

`docker compose up -d postgres` → `go run ./cmd/migrate up` (0001→0006 bersih) → `go run ./cmd/migrate
down` sekali (0006 turun bersih, status kembali ke 0005) → `go run ./cmd/migrate up` lagi → jalankan
`crm_be` lokal (`APP_ENV=development`, `PUSH_PROVIDER=none`) → register org "Toko Device68" → verifikasi
email → login Owner:

```
POST /v1/device-tokens {"token":"dummy-fcm-token-abc123","platform":"android"}
→ 201 {"id":"...","platform":"android",...}   -- tanpa field "token" di respons

POST /v1/device-tokens {"token":"dummy-fcm-token-abc123","platform":"ios"}  (token SAMA, platform beda)
→ 201, id TIDAK berubah, platform="ios"
psql: SELECT count(*) FROM device_tokens → 1 baris   -- dipindahkan, bukan diduplikasi

DELETE /v1/device-tokens {"token":"dummy-fcm-token-abc123"} → 204
DELETE /v1/device-tokens {"token":"dummy-fcm-token-abc123"} → 404   -- kedua kali, bukan error 500

POST /v1/leads {"name":"Budi Santoso", ...} → 201, lead_number=1

PATCH /v1/leads/{id}/assignment {"assigned_to_membership_id": <membership OWNER SENDIRI>}
→ 200, TIDAK ADA baris log push   -- self-assignment diam, sesuai TD §11

-- undang karyawan, accept, login sebagai karyawan, daftarkan device token miliknya --

PATCH /v1/leads/{id}/assignment {"assigned_to_membership_id": <membership KARYAWAN>}
→ 200
log: msg="push (not sent — NoopSender)" title="Lead #1 ditugaskan kepada Anda"
     -- TIDAK ADA field token atau data di baris log ini (Rule #26)
```

Membuktikan tiga hal sekaligus: upsert benar-benar memindahkan baris (bukan menduplikasi), delete
idempoten secara eksternal (404 kedua kali, bukan crash), dan hook push-setelah-commit benar-benar
tersambung dari `lead.Usecase` sampai `device.Usecase.PushToMembership` — termasuk membedakan
self-assignment (diam) dari assignment sungguhan (push tercatat).

### Test

30 test baru: 6 `internal/shared/push/fcm_internal_test.go` (`package push`, akses `baseURL` unexported —
mengikuti pola pengecualian `mailer`'s `message_internal_test.go`), 6 `internal/device/repository_test.go`
(Postgres asli, termasuk `TestRepository_Upsert_SameToken_MovesRowToNewOwner` dan
`TestRepository_DeleteByToken_CrossOrg_DoesNotDelete`), 11 `internal/device/usecase_unit_test.go` (fake
`Store`+`Sender`, tanpa Docker), 5 `internal/device/handler_test.go`, 2 backfill di `authz_test.go`. Satu
`isolationCase` baru di `cmd/api/tenant_isolation_test.go` — terbukti bisa gagal seperti dijelaskan di
atas, dengan temuan dua-lapis yang didokumentasikan, bukan disembunyikan.

`go test -race ./...` (seluruh module, bukan per-paket), `go vet ./...`, `gofmt -l .` — bersih.

### Batas issue ini

Registrasi token dari aplikasi (Flutter belum ada — diverifikasi `curl` + token dummy, sesuai batas
issue di `issues.md`). Push benar-benar sampai ke perangkat fisik lewat `FCMSender` — butuh kredensial
FCM asli dan aplikasi terpasang, keduanya di luar cakupan #68 (klien FCM: #73). Layar apa pun di
`crm_employee` — #69 dan seterusnya.

### Addendum — CI gagal setelah PR dibuka, bukan disebabkan #68 sendiri

`gh pr checks` menunjukkan `lint` dan `test` merah pada PR #68. Diperiksa satu per satu sebelum
disimpulkan:

- **`lint` sudah merah di `main` sejak PR #46 digabung (27 Agustus)** — dikonfirmasi lewat
  `gh run list --branch main`, seluruh 5 CI run `main` sejak itu (#46, #47, #57, #58, #63/#64) gagal di
  `lint` dengan temuan yang **sama persis**: `gosec` G101 ("Potential hardcoded credentials") pada
  `apiKeyColumns` (daftar kolom SQL) dan dua konstanta `Action` (`ActionAPIKeyList`,
  `ActionAPIKeyRevoke`) — bukan kredensial sungguhan, gosec mencocokkan substring `apiKey`/`token` di
  **nama identifier**, bukan isinya. Penyebabnya `ci-backend.yml` memasang `golangci-lint@latest` tanpa
  pin versi, jadi setiap update `gosec` di hulu bisa mengubah hasil linting tanpa perubahan kode apa
  pun. Ini utang yang sudah ada lima PR sebelum #68 dibuka dan tidak pernah diperbaiki — dilaporkan di
  sini apa adanya, bukan disembunyikan sebagai "sudah diperbaiki #68".
  **Kode #68 sendiri menambah dua temuan baru dengan pola persis sama** (`deviceTokenColumns`,
  `ActionDeviceTokenRegister`) — jadi tetap bagian tanggung jawab issue ini untuk tidak menambah utang
  yang sama. Diperbaiki dengan `#nosec G101 -- <alasan>` inline, mengikuti konvensi yang sudah ada di
  codebase ini (`internal/auth/login_limiter.go`, `internal/shared/password/argon2.go`,
  `internal/shared/db/db.go`) — bukan mematikan `gosec` secara global di `.golangci.yml`, yang akan ikut
  membungkam temuan sungguhan di masa depan. **Satu kesalahan sempat dibuat lalu ketahuan sebelum
  commit**: menambahkan `// #nosec G101` sebagai trailing comment tepat setelah backtick pembuka raw
  string (`` const apiKeyColumns = ` // #nosec... ``) — di Go, isi antara dua backtick adalah literal,
  bukan komentar, jadi baris itu **akan menyisipkan teks komentar ke dalam string SQL sungguhan**.
  Diperbaiki dengan menyatukan daftar kolom jadi satu baris supaya trailing comment valid di baris yang
  sama dengan `const`, bukan di tengah string. Dua temuan `staticcheck` S1016 (`CreateInput{Name:
  req.Name, Scopes: req.Scopes}` → `CreateInput(req)`) juga diperbaiki — struct sumber dan tujuan cocok
  field-demi-field, jadi konversi tipe langsung valid, bukan sekadar kosmetik.
- **`test` gagal karena bug flaky nyata di `internal/apikey`, bukan `internal/device`** —
  `TestUnit_ResolveAPIKey_EveryFailureReasonIsIdentical` (dari #47, jauh sebelum #68) menyeed dua kunci
  API dalam satu `fakeStore`, keduanya lewat `seedResolvableKey` yang selalu men-stamp `KeyID` yang sama
  (`testKeyID`) kecuali diubah manual setelahnya. Kunci "revoked" DIUBAH `RevokedAt`-nya tapi TIDAK
  `KeyID`-nya, jadi `FindByKeyID`'s `range` atas `map[uuid.UUID]*apikey.APIKey` — urutan iterasi Go
  **diacak setiap pemanggilan** — bisa mengembalikan objek "valid" alih-alih "revoked" untuk `key_id`
  yang sama, membuat skenario "revoked key" kadang lolos otentikasi alih-alih ditolak. **Dibuktikan
  lewat pengulangan**: `go test -run TestUnit_ResolveAPIKey_EveryFailureReasonIsIdentical -count=200`
  gagal berulang kali sebelum perbaikan, `-count=500` lolos bersih setelahnya. Diperbaiki dengan satu
  baris — `revoked.KeyID = "revokedkey01"` setelah seeding, memberi kunci revoked `key_id` yang tidak
  bertabrakan dengan kunci `valid` di map yang sama. Ini bukan bug yang diperkenalkan #68; kebetulan
  baru terpicu pada run CI PR #68 karena keacakan `range` map, bukan karena kode #68 menyentuh paket
  `apikey` sama sekali.

Kedua perbaikan ini menyentuh berkas di luar `internal/device`/`internal/shared/push`/migration 0006 —
dicatat secara eksplisit di sini karena itu, bukan diam-diam dianggap bagian normal cakupan #68. Baik
alasan **kenapa** disertakan dalam PR yang sama (memblokir CI hijau PR #68 sendiri, dan keduanya kecil,
terverifikasi, tidak mengubah perilaku produksi) dicatat di badan commit terpisah, bukan digabung ke
commit awal #68.
