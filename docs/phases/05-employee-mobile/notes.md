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

---

## #69 — Fondasi Flutter: FVM, scaffold, `ApiClient` single-flight, secure storage, login + biometric

Pembuka `crm_employee/` sungguhan — sebelumnya hanya `README.md` + persiapan FVM manual dari pemilik
produk (`.fvmrc`, `.fvm/`, `.vscode/settings.json`). Boleh paralel dengan #68 (sudah selesai duluan) —
tidak menunggu hasil desain (keputusan M6): tampilan sengaja seadanya, penataan visual jadi kerjaan #70.

### `flutter create` dan `applicationId`

`fvm flutter create --org com.jualin --project-name crm_employee --platforms android,ios .` dijalankan
di tempat, **bukan** `--org com.jualin.crm --project-name employee` — opsi kedua akan memberi
`applicationId` yang benar (`com.jualin.crm.employee`) secara otomatis tapi mengubah nama paket Dart
jadi `employee`, tidak konsisten dengan konvensi monorepo ini (`crm_dashboard`, `crm_be`). Diselesaikan
dengan membiarkan nama paket Dart `crm_employee` (default dari `--project-name`) lalu **mengubah
`applicationId` secara manual** di `android/app/build.gradle.kts` — `namespace` (dipakai untuk resolusi
`R` class internal) dan `applicationId` (identitas publik Play Store) sudah dipisah sejak Android Gradle
Plugin modern, jadi keduanya sengaja **berbeda** di sini (`namespace = com.jualin.crm_employee`,
`applicationId = com.jualin.crm.employee`) — bukan kelalaian. Bundle ID iOS (`ios/Runner.xcodeproj`)
diselaraskan ke nilai yang sama meski iOS tidak pernah dibangun phase ini (M1) — murni supaya kedua
platform konsisten sejak hari pertama, tidak menambah biaya apa pun.

`README.md` sempat **tertimpa** oleh `flutter create` (isinya diganti template default bahasa Inggris)
— dikembalikan ke isi Bahasa Indonesia semula, dengan baris "Belum dibuat" dihapus karena sudah tidak
benar.

### `ApiClient` — bentuk `api-client.ts` disalin, dengan satu penyimpangan disengaja

`lib/core/api_client.dart` meniru `crm_dashboard/src/lib/api-client.ts` baris demi baris untuk bagian
single-flight (`_refreshFuture ??= _performRefresh().whenComplete(() => _refreshFuture = null)` — Dart
`??=` pada Future punya semantik sinkron yang sama seperti JS, dibuktikan test di bawah). Yang **beda**
dari dashboard, sengaja: parameter `authorize` mengontrol dua hal sekaligus — apakah header
`Authorization` dipasang, **dan** apakah `401` boleh memicu percobaan refresh. `login()`/`refresh()`
sendiri selalu memanggil dengan `authorize: false`.

**Ini bukan sekadar gaya berbeda — ini memperbaiki kelas bug yang ada di `api-client.ts`.**
`crm_dashboard`'s `login()` memanggil `apiFetch()` yang sama seperti endpoint lain, dan `apiFetch`'s
percabangan 401 tidak mengecualikan path `/v1/auth/login` (hanya mengecualikan `/v1/auth/refresh`
sendiri). Password salah di `crm_be` mengembalikan `401 invalid_credentials` (dikonfirmasi langsung dari
`internal/auth/usecase.go`'s `invalidCredentialsError()`) — artinya di dashboard, percobaan login gagal
memicu `doRefresh()` dengan sesi yang belum pernah ada, refresh itu pasti gagal (tidak ada cookie
`refresh_token`), lalu `redirectToLogin()` + `sessionExpiredError` ("Sesi Anda berakhir. Silakan masuk
kembali.") ditampilkan — **bukan** "Email atau password salah." yang sebenarnya. Ditemukan sambil
membaca `api-client.ts` untuk menyalinnya, bukan dicari sengaja. **Tidak diperbaiki di `crm_dashboard`**
— itu di luar cakupan #69 (aplikasi berbeda, PR berbeda); dicatat di sini apa adanya sebagai temuan,
bukan diam-diam dibiarkan tidak tercatat. `authorize: false` di Flutter menghindari mewarisi kelas bug
yang sama, didokumentasikan langsung sebagai komentar kode di `ApiClient`.

### `TokenStorage` — interface dipisah dari `flutter_secure_storage`, alasan sama seperti `port.go`

`ApiClient`/`Session` bergantung pada `TokenStorage` (abstract), bukan `FlutterSecureStorage` langsung —
`flutter_secure_storage` tidak punya channel platform sungguhan di lingkungan host `flutter test`, jadi
tanpa lapis ini `api_client_test.dart` tidak bisa jalan tanpa perangkat. Pola yang sama seperti
ADR-011's `port.go` (consumer mendeklarasikan interface), diterapkan di Flutter meski TD tidak
menuliskannya eksplisit sejauh itu.

### `Session` sengaja tidak menyentuh jaringan

`core/session.dart` hanya menyimpan `SessionStatus` (`unknown`/`authenticated`/`unauthenticated`) dan
membaca `TokenStorage` saat `bootstrap()` — **tidak** memanggil `AuthApi` sama sekali. Draf pertama
menaruh `login()`/`logout()` langsung di `Session` (memanggil `AuthApi` di dalamnya), tapi itu berarti
`core/session.dart` mengimpor `features/auth/auth_api.dart` — arah dependensi terbalik dari yang
digambar TD §3 (`core/` semestinya lebih rendah dari `features/`, bukan sebaliknya). Diperbaiki sebelum
lanjut menulis layar: `Session` hanya punya `markAuthenticated()`/`markUnauthenticated()`, dipanggil
oleh `features/auth/` setelah `AuthApi` selesai — `core/` tidak pernah mengimpor `features/` satu pun.

### Biometric — `local_auth` 3.0.2's API sudah berbeda dari yang diasumsikan draf pertama

Draf pertama menulis `AuthenticationOptions(biometricOnly: true, stickyAuth: true)` sebagai argumen
`authenticate()`, mengikuti dokumentasi lama yang pernah dibaca — `flutter analyze` langsung menandai
`options` sebagai parameter yang tidak ada. Versi terpasang (3.0.2, dari `pubspec.lock`) sudah pindah ke
named parameter datar (`biometricOnly`, `persistAcrossBackgrounding` menggantikan `stickyAuth`).
**Kesalahan kedua ditemukan sambil memperbaiki yang pertama**: `canCheckBiometrics` di versi ini berarti
"perangkat mendukung biometric secara hardware", **bukan** "ada biometric terdaftar" — draf pertama
memakainya sendirian untuk kriteria acceptance #6 ("perangkat tanpa biometric terdaftar jatuh ke
password"), yang berarti perangkat dengan hardware sidik jari tapi **belum pernah didaftarkan sama
sekali** akan tetap lolos ke `authenticate()` alih-alih jatuh ke password lebih dulu. Diperbaiki dengan
menambah `getAvailableBiometrics().isNotEmpty` — method yang dokumentasi `local_auth` sendiri sebutkan
mencerminkan apa yang **benar-benar bisa dipakai sekarang**, bukan `isDeviceSupported()` (yang juga
`true` untuk perangkat yang hanya punya fallback PIN/pola tanpa biometric sama sekali).

### Android — tiga penyesuaian yang tidak disebut TD, ketahuan saat `flutter build apk` sungguhan dicoba

`flutter analyze`/`flutter test` bersih tidak menjamin **Android sungguhan bisa dibangun** — tiga
masalah baru muncul saat `fvm flutter build apk --debug` benar-benar dijalankan, seluruhnya persyaratan
`local_auth`/`flutter_secure_storage` di Android yang tidak otomatis terpasang `flutter create`:
`MainActivity.kt` harus `FlutterFragmentActivity` (bukan `FlutterActivity`), `AndroidManifest.xml` butuh
izin `USE_BIOMETRIC`, `LaunchTheme` harus berparent `Theme.AppCompat.DayNight.NoActionBar` (tema polos
bawaan bikin dialog biometric crash di Android 8 ke bawah), dan `compileSdk` dinaikkan manual ke **37**
(`flutter_secure_storage` mensyaratkannya, default Flutter 3.44.0 masih 36 — build gagal dengan pesan
error yang eksplisit menyebutkan ini, bukan ditebak). Detail lengkap ada di
`docs/issues/069-flutter-foundation.md`. Build APK debug akhirnya sukses (`app-debug.apk`, ~keluaran
`assembleDebug`), dijalankan dua kali — sebelum dan sesudah seluruh perbaikan ini — untuk memastikan
tidak regresi.

### Test

`test/api_client_test.dart` — 5 test, satu-satunya file yang TD §12 sebut eksplisit untuk issue ini:

- **Single-flight terbukti dengan konkurensi nyata**: 6 panggilan `apiClient.send()` paralel yang
  sama-sama mendapat `401`, refresh ditahan lewat `Completer` (pola `deferred()` `api-client.test.ts`)
  sampai keenam panggilan sempat mencapai titik pemeriksaan single-flight, lalu dilepas — **tepat 1**
  panggilan ke `/v1/auth/refresh` tercatat, seluruh 6 hasil retry sukses dengan token baru. Diulang
  manual 5× berturut untuk memastikan tidak flaky (delay 20ms nyata, bukan yield microtask, memberi
  margin besar dibanding kerja non-jaringan `MockClient`).
- Permintaan yang tidak pernah `401` tidak pernah memicu refresh sama sekali (bukti negatif — mencegah
  regresi "refresh dipanggil terlalu bersemangat").
- Refresh yang gagal (`401` dari `/v1/auth/refresh`) membersihkan kedua token dan melempar
  `SessionExpiredException` — juga untuk kasus tidak ada refresh token tersimpan sama sekali.
- `authorize: false` (dipakai `login()`) terbukti **tidak pernah** memicu refresh pada `401` —
  mengunci perbaikan kelas bug di atas sebagai perilaku, bukan cuma niat di komentar.

### Verifikasi manual — apa yang bisa dan tidak bisa dilakukan di lingkungan ini

**Tidak ada perangkat/emulator Android tersedia** di lingkungan kerja sesi ini (`flutter devices` hanya
melaporkan macOS desktop dan Chrome; `flutter emulators` hanya punya iOS Simulator) — kriteria
acceptance yang secara eksplisit butuh "HP Android sungguhan" (biometric sungguhan, instalasi APK
sungguhan) **tidak bisa diklaim terverifikasi** dan harus dilakukan pemilik produk sendiri sebelum #73
menutup phase. Dicatat jujur di sini, bukan diasumsikan lolos.

Yang **bisa** dan **sudah** diverifikasi langsung terhadap `crm_be` sungguhan (Postgres asli via
`docker compose`, migrasi 0001→0006, server lokal `PUSH_PROVIDER=none`) — lewat skrip `flutter test`
sekali-pakai yang memanggil `ApiClient`/`AuthApi` **produksi** yang sama persis dengan yang dipakai
aplikasi (bukan reimplementasi), `TokenStorage` diganti fake in-memory (satu-satunya bagian yang perlu
perangkat sungguhan untuk `flutter_secure_storage`'s channel platform asli):

1. Undang & terima karyawan sungguhan (`POST /v1/invitations` → `accept`) → `AuthApi.login(client:
   "mobile")` sungguhan → `200`, token tersimpan.
2. `AuthApi.me()` dengan access token segar → `200`, data cocok (`full_name`, `role=employee`,
   `organization_name`).
3. Access token **dirusak sengaja** (mensimulasikan kedaluwarsa) → panggilan berikutnya tetap `200` —
   membuktikan jalur `401 → refresh single-flight → retry` benar-benar berjalan melawan backend
   sungguhan, bukan cuma lolos lewat mock.
4. **Rotasi refresh token dibuktikan langsung**: refresh token sebelum dan sesudah langkah 3 dibandingkan
   — berbeda, sesuai TD §4 ("rotasi setiap refresh").
5. `AuthApi.logout()` → kedua token lokal terhapus; panggilan setelahnya gagal seperti seharusnya.
6. **Kriteria acceptance #7 dibuktikan lewat kode produksi, bukan `curl` saja**: karyawan kedua login
   (`AuthApi.login`, token asli tersimpan) → membership-nya dinonaktifkan dari sisi Owner
   (`DELETE /v1/memberships/{id}`, di luar aplikasi, mensimulasikan aksi dashboard) → **`ApiClient.send()`
   milik karyawan itu sendiri** dipanggil dengan access token yang sudah kedaluwarsa (memicu refresh) →
   refresh ditolak backend (`401`, membership tidak aktif) → `ApiClient` melempar
   `SessionExpiredException` **dan** kedua token lokal terhapus — persis perilaku yang diminta TD §4.2,
   dibuktikan lewat pemanggilan `ApiClient` sungguhan, bukan diasumsikan dari baca kode `internal/membership`
   Phase 2 saja.

Skrip-skrip verifikasi ini **tidak di-commit** — sekali pakai, dihapus dari `crm_employee/` setelah
dijalankan (isinya cuma memanggil ulang kode produksi yang sudah diuji `api_client_test.dart` terhadap
jaringan sungguhan, tidak menambah cakupan test yang perlu dipertahankan).

Temuan yang perlu ditinjau ulang saat #73 dicatat di `docs/issues/069-flutter-foundation.md`.

### Addendum — ditulis ulang jadi Bloc + Clean Architecture, sebelum PR digabung

PR #76 (isi di atas) awalnya dibangun dengan `provider`/`ChangeNotifier` mengikuti TD §6 aslinya. Sebelum
PR digabung, pemilik produk secara eksplisit meminta **Bloc + Clean Architecture** menggantikannya —
permintaan langsung, bukan bug yang ditemukan. TD §3 dan §6 (`docs/phases/05-employee-mobile/td.md`)
direvisi untuk mencatat pergantian ini beserta alasan pemilihan tiap dependency baru; ringkasannya:

- `core/session.dart` (`ChangeNotifier` tunggal) **dihapus total**, digantikan `AuthBloc`
  (`features/auth/presentation/bloc/`) — satu instance untuk umur aplikasi, diregistrasi lewat `get_it`.
- `AuthApi`/`Session`/`BiometricAuthenticator`/`AuthGate`/`LoginScreen`/`PlaceholderHomeScreen` (kode
  provider lama) **dihapus**, digantikan struktur tiga lapis per fitur (`domain/data/presentation`,
  TD §3): `AuthUser` (entity) · `AuthRepository`/`BiometricRepository` (interface domain) ·
  `LoginUseCase`/`LogoutUseCase`/`GetCurrentUserUseCase`/`CheckStoredSessionUseCase`/
  `CheckBiometricAvailabilityUseCase`/`AuthenticateWithBiometricsUseCase` (use case, satu operasi
  repository per kelas — pola Clean Architecture standar, sengaja melewati Aturan #28 "abstraksi hanya
  setelah implementasi kedua nyata" karena ini pola yang diminta eksplisit, dicatat sebagai pengecualian
  sadar bukan pelanggaran diam-diam) · `AuthRemoteDataSource`/`BiometricLocalDataSource` +
  `AuthRepositoryImpl`/`BiometricRepositoryImpl` (data layer — mengoordinasikan API/`local_auth` dengan
  `TokenStorage`, dan **satu-satunya tempat** `ApiError`/`SessionExpiredException` [vocabulary jaringan,
  `core/api_error.dart`] diterjemahkan jadi `Failure` [vocabulary domain, `core/error/failures.dart` baru,
  `dartz`'s `Either<Failure, T>`]) · `AuthGatePage`/`LoginPage`/`BiometricGatePage`/`HomePage` (presentasi
  — `BlocBuilder`, tidak ada logika di widget).
- **`core/api_client.dart`/`core/secure_store.dart` TIDAK berubah sama sekali** — keduanya sudah
  infrastruktur murni sejak awal (tidak tahu apa pun tentang Bloc atau `provider`), jadi
  `test/api_client_test.dart` (5 test dari draf pertama) tetap lolos tanpa satu baris pun diubah,
  membuktikan batas lapis yang digambar sejak awal ternyata benar.
- **`hasStoredSession`/`canAuthenticate`/`authenticate` sengaja TIDAK dibungkus `Either<Failure, T>`** —
  ketiganya pembacaan boolean lokal yang kegagalannya sudah collapse jadi `false` di lapis data;
  membungkusnya di `Either` yang tidak pernah benar-benar `Left(...)` hanya kosmetik seragam, bukan
  kejujuran tipe. Dicatat sebagai keputusan desain eksplisit, bukan inkonsistensi pola.
- **`HomePage` tidak lagi memanggil `GET /v1/me` sendiri** — `AuthBloc` sudah memuat `AuthUser` lewat
  `GetCurrentUserUseCase` sebagai bagian alur bootstrap/login, dan `HomePage` membacanya langsung dari
  `AuthState.AuthAuthenticated` lewat `BlocBuilder`. Ini menghilangkan satu panggilan jaringan berlebih
  yang ada di draf `PlaceholderHomeScreen` lama.

**Test ditambah, bukan dikurangi**: `test/features/auth/presentation/bloc/auth_bloc_test.dart` (9 test,
`bloc_test`+`mocktail`, seluruh transisi §4.1 termasuk kasus sesi kedaluwarsa persis setelah biometric
sukses) dan `test/features/auth/data/repositories/auth_repository_impl_test.dart` (9 test, membuktikan
pemetaan `ApiError`→`Failure` secara langsung — kelas bug paling mudah salah diam-diam di seluruh
rewrite ini). Total naik dari 5 jadi 22 test, seluruhnya lolos.

**Verifikasi ulang terhadap `crm_be` sungguhan**, kali ini lewat lapis Clean Architecture penuh
(`AuthRemoteDataSourceImpl` → `AuthRepositoryImpl` → `LoginUseCase`/`GetCurrentUserUseCase`/
`LogoutUseCase`), bukan cuma `ApiClient` langsung: login → `/v1/me` → logout → `/v1/me` setelah logout
gagal — seluruh 4 langkah sukses persis seperti verifikasi pertama, membuktikan rewrite tidak diam-diam
mengubah perilaku, hanya strukturnya. `flutter build apk --debug` diulang sekali lagi, sukses.

Kesimpulan: **tidak ada perilaku yang berubah dari perspektif pengguna** — login, biometric, refresh,
logout bekerja identik. Yang berubah murni struktural: 6 berkas provider lama → ~25 berkas across
domain/data/presentation, 3 dependency baru (`flutter_bloc`, `get_it`, `dartz`, `equatable` — 4
sebenarnya) + 2 dev dependency (`bloc_test`, `mocktail`), `provider` (dependency langsung) dihapus
(tetap ada transitif lewat `flutter_bloc`).

### Batas issue ini

Tidak ada layar selain login + satu `HomePage` (memanggil `GetCurrentUserUseCase` lewat `AuthBloc`,
menyediakan tombol Keluar — dibutuhkan supaya login+biometric+refresh punya sesuatu nyata untuk
dibuktikan, digantikan total oleh #71). Tema, navigasi antarlayar, daftar lead — seluruhnya #70/#71.
Push notification, deeplink — #73.

---

## #70 — Fondasi desain mobile: tema, token, labels, kerangka navigasi

Hasil desain dibaca dari project Claude Design "Employee mobile design spec"
(`https://claude.ai/design/p/11071bff-85b7-4d26-aeed-c5fd8f60c668`) lewat `DesignSync` — dikonfirmasi
lewat `github.md`-nya sendiri di project itu: dibangun dari `design-brief.md`/`prd.md`/`td.md` phase ini
langsung, sinkron terakhir 29 Agustus 2026. Enam layar, seluruh keadaan wajib TD §10, kerangka navigasi,
token — persis cakupan design brief §12.

### Kontras diverifikasi ulang secara independen — semua lolos, tiga angka desain sendiri meleset

Design brief menuntut "dihitung, bukan dikira". Project desain sudah mencantumkan rasio kontras di
setiap pasangan warna (mis. `--primary ... 4.83:1`) — tapi angka itu **klaim si alat desain**, bukan
bukti. Setiap pasangan dihitung ulang dari nol (formula luminansi relatif WCAG 2.1, skrip Python
sekali-pakai, tidak di-commit) sebelum satu pun nilai diterima ke `theme.dart`.

**Hasil: seluruh 15 pasangan lolos AA (≥4.5:1) — tidak ada yang perlu diperbaiki kali ini**, beda dari
Phase 3 #40 (5 dari 8 badge status gagal saat itu). Tapi tiga angka yang dicetak desain sendiri
**meleset** dari hasil hitung ulang (tetap lolos AA, bukan masalah fungsional, tapi tetap dicatat karena
AC-nya minta dihitung bukan ditebak):

| Pasangan | Diklaim desain | Dihitung ulang | Status |
|---|---|---|---|
| `--primary` (teks putih di atasnya) | 4.83:1 | **5.05:1** | Lolos, angka desain terlalu rendah |
| `--warning` di atas `--warning-tint` | 7.6:1 | **6.65:1** | Lolos, angka desain terlalu tinggi |
| `--success` di atas `--success-tint` | 7.0:1 | **7.16:1** | Lolos, selisih kecil |

Nilai yang **dipakai** di `theme.dart`'s dokumentasi komentar adalah hasil hitung ulang session ini,
bukan angka yang dicetak project desain — dikunci sebagai fakta terverifikasi, bukan disalin mentah.
`--muted-foreground` (4.74:1, desain menyebut 4.73:1 — selisih presisi pembulatan saja) tetap ditandai
"pas ambang" mengikuti peringatan desain sendiri: dibatasi metadata ≥13px, tidak pernah info kritikal —
dipatuhi di kode (`AppTextStyles.metadata` satu-satunya pemakai `mutedForeground` untuk teks berjalan).

### `theme.dart` — `ThemeExtension`, bukan `ColorScheme` dipaksa memuat semuanya

`ColorScheme` bawaan Flutter tidak punya slot untuk `accentStrong`/`mutedForeground`/`warning`/`success`
dkk. — dipaksakan ke situ berarti menyalahgunakan slot yang artinya beda (mis. memakai `secondary` untuk
`accentStrong` akan membingungkan pembaca kode berikutnya). `AppColorsExtension` (pola resmi Flutter
untuk token kustom) menampung token tambahan, dibaca lewat
`Theme.of(context).extension<AppColorsExtension>()`. Tipografi (Roboto) **tidak** menambah dependency
font — Flutter's Material widgets sudah resolve ke Roboto di target Android tanpa `fontFamily` custom,
persis alasan design brief sendiri sebut Roboto (font sistem, tidak menambah bobot unduhan).

### `labels.dart` — menyalin `labels.ts`, sengaja tidak menyalin `SCOPE_LABELS`

Peta status/alasan-kalah/sumber/role disalin nilai (bukan kode) dari `crm_dashboard/src/lib/labels.ts`,
dikunci `test/shared/labels_test.dart` terhadap `ck_leads_status`/`ck_leads_lost_reason`/
`ck_leads_source`/`ck_memberships_role` di migration `crm_be` langsung (bukan dipercaya dari `labels.ts`
begitu saja) — 8/6/4/4, cocok. **`SCOPE_LABELS` (skop API key) sengaja TIDAK ikut disalin** —
`crm_dashboard` punya peta itu karena dashboard mengelola API key; `crm_employee` tidak pernah menyentuh
format `jln_*` sama sekali (Aturan #24), jadi menyalinnya berarti satu-satunya kosakata terkait API key
yang ada di aplikasi ini justru diimpor tanpa alasan.

### `nav.dart` — `switch` eksponen menggantikan pola `Array.find` web

`crm_dashboard/src/lib/nav.ts`'s `isActive`/`pageTitle` (dasar acuan AC issue ini) pernah punya jebakan
nyata: prefix-match naif bikin `/` cocok dengan setiap rute. Flutter tidak punya konsep URL, jadi
jebakan yang sama secara harfiah tidak ada — tapi `navTitle`/`navIcon` tetap ditulis sebagai `switch`
eksponen atas `enum AppDestination`, bukan `Map`/`firstWhere`: menambah destination baru tanpa
memperbarui kedua fungsi ini adalah **error kompilasi**, jaminan yang lebih kuat dari sekadar test.
`initialsOf` disalin baris-demi-baris dari `nav.ts` (aturan yang sama, kasus tepi yang sama), diuji
`test/shared/nav_test.dart` termasuk nama kosong dan whitespace ganda.

### Kerangka aplikasi — `AppShell`, satu-satunya jalan masuk akun lewat avatar

Sesuai design brief §4: header 56dp (judul statis, tanpa tombol) + `NavigationBar` 3 tujuan (Lead Saya,
Tugas Saya, Notifikasi — ikon+label selalu tampil). **Detail Lead sengaja bukan tujuan nav** — design
brief eksplisit: diakses via push dari Lead Saya/Notifikasi, jadi tidak pernah masuk `IndexedStack`
`AppShell`. Menu akun **hanya** avatar inisial di header kanan → bottom sheet (nama, organization,
tombol Keluar) — tidak ada hamburger menu, sesuai desain. Ketiga tab isinya `PlaceholderScreen`
(pola `crm_dashboard`'s `placeholder-screen.tsx` dari #40, dikonfirmasi dari git history sebelum ditulis
ulang di Dart) — Lead Saya menyebut #71, Tugas Saya & Notifikasi menyebut #73 (issues.md: My Tasks +
FCM + deeplink satu issue, notifikasi ikut di situ).

### Perilaku baru: layar Sesi Berakhir sungguhan, bukan lagi jatuh diam ke login

Draf #69 menangani `SessionExpiredFailure` dengan diam-diam emit `AuthNeedsPassword()` — pesannya hilang
sama sekali. Design brief §10 secara eksplisit minta layar sendiri ("Sesi Anda berakhir... Data yang
sudah tersimpan tidak hilang", tombol "Masuk kembali") karena **kejadiannya bisa di tengah pemakaian**,
bukan cuma saat app dibuka. State baru `AuthSessionExpired` ditambahkan ke `AuthState`, `_loadCurrentUser`
diubah untuk emit itu, `SessionExpiredPage` baru dibangun, `AuthGatePage`'s `switch` menambah satu cabang.
**Test lama yang mengasumsikan perilaku diam (`AuthNeedsPassword` langsung) diperbarui**, bukan dihapus —
`auth_bloc_test.dart`'s kasus "session expires right after biometric success" sekarang menegaskan
`AuthSessionExpired`, plus satu test baru untuk tombol "Masuk kembali"-nya.

### Yang sengaja tidak diikuti dari desain — dicatat, bukan didiamkan

- **State "backoff" (percobaan login berlebih) tidak dapat hitung mundur langsung** ("Coba lagi dalam
  04:37") seperti digambar desain. Itu butuh header `Retry-After` mengalir dari rate limiter `crm_be`
  sampai ke `ApiClient`/`ApiError` — keduanya belum pernah membaca header respons sama sekali hari ini.
  Kegagalan rate-limit tetap tampil (pesan asli `crm_be`, dalam banner bertema), hanya tanpa jam
  berjalan. Dicatat di `docs/issues/070-design-foundation.md` untuk ditinjau ulang, bukan dibangun
  setengah jalan dengan `Retry-After` ditebak dari `DateTime.now()` lokal (bisa salah kalau jam
  perangkat tidak akurat).
- **Gerbang biometric tidak menampilkan nama/organization** ("Budi Santoso" / "Toko Sinar Jaya" di
  desain) — aplikasi ini tidak menyimpan profil pengguna secara lokal sama sekali; satu-satunya cache
  yang TD §7 gambarkan adalah data lead/task (#71), bukan identitas, dan memanggil `/v1/me` sebelum
  biometric berhasil akan mengalahkan tujuan gerbangnya sendiri. Salinan generik dipakai sebagai
  gantinya.

### Test

`test/shared/labels_test.dart` (13 test — jumlah **dan** nilai keempat enum dikunci langsung terhadap
`ck_*` migration `crm_be`, bukan hanya terhadap `labels.ts`) dan `test/shared/nav_test.dart` (11 test —
`navTitle`/`navIcon` tiap destination, `initialsOf` termasuk kasus tepi) baru. Satu test `auth_bloc_test.dart`
lama diperbarui (state `AuthSessionExpired`), satu ditambah (tombol "Masuk kembali"). Total 41 test,
seluruhnya lolos.

Widget smoke test tambahan (skrip sekali-pakai, **tidak di-commit** — TD §12 tidak mewajibkan widget
test phase ini) memompa `LoginPage`/`BiometricGatePage`/`SessionExpiredPage`/`AppShell`/`AuthGatePage`
lewat seluruh kombinasi state, termasuk tap tab dan tap avatar → bottom sheet — tidak ada exception
render, tidak ada overflow. `flutter build apk --debug` diulang setelah seluruh perubahan, sukses.

### Batas issue ini

Tidak membangun Lead Saya, Detail Lead, atau Tugas Saya sungguhan — ketiganya tetap `PlaceholderScreen`.
Notifikasi sungguhan, push, deeplink — #73.

---

## #71 — My Leads + cache baca offline

Layar terpenting phase ini (kriteria selesai phase menempel langsung: "daftar lead tetap terbaca saat
mode pesawat"). Backend Employee visibility **diverifikasi langsung** di `crm_be/internal/lead/
repository_postgres.go` sebelum diasumsikan dari teks issue — `isEmployee(t)` memaksa
`assigned_to_membership_id = <membership pemanggil>` **tanpa syarat**, terlepas dari `assigned_to` apa
pun yang dikirim klien. Klien karena itu tidak pernah mengirim `assigned_to` sama sekali.

### `core/cache/` — mekanisme cache lahir di sini, pemakai pertamanya sekaligus

TD §7: satu tabel key-value SQLite (`sqflite`), **bukan** skema domain lokal — yang dibutuhkan kriteria
#2 adalah menampilkan kembali persis yang terakhir terlihat, bukan query/join/filter offline. Tiga
lapis:

- **`ResponseCache`** (interface) + **`SqfliteResponseCache`** — `get`/`put`/`clear` mentah,
  `key`→`{body, fetched_at}`.
- **`cachedGet()`** — coba jaringan dulu; sukses → simpan & kembalikan (`fromCache: false`); jaringan
  **tidak terjangkau** → baca cache (`fromCache: true` + `fetchedAt` asli). **Sengaja TIDAK menangkap
  `ApiError`/`SessionExpiredException`** — keduanya berarti server benar-benar menjawab (error nyata
  atau sesi habis), situasi yang sama sekali berbeda dari "jaringan tidak terjangkau" dan tidak boleh
  diam-diam ditutupi data basi. Dibuktikan test khusus (`cached_get_test.dart`).
- **`runApiCall()`** (`core/network/`) — pemetaan `ApiError`/`SessionExpiredException` → `Failure` yang
  dulu ditulis tangan tiga kali di `AuthRepositoryImpl`, diekstrak sekarang karena `LeadRepositoryImpl`
  butuh bentuk **persis sama** (Aturan #28: ini implementasi kedua yang nyata, bukan abstraksi
  spekulatif). `AuthRepositoryImpl` ditulis ulang memakainya — perilaku tidak berubah, dibuktikan
  seluruh test lama `auth_repository_impl_test.dart` tetap lolos tanpa perubahan asersi.

### `ApiClient.sendListEnvelope` — celah nyata ditemukan sebelum dipakai

`ApiClient.send()` (dari #69) menyempitkan hasil ke field `data` saja, membuang `meta` — cukup untuk
endpoint auth yang tidak pernah butuh paginasi. **My Leads butuh `meta.total`**, dan `cachedGet` perlu
menyimpan `{data, meta}` **utuh** supaya pembacaan dari cache nanti tetap punya `total`-nya. Ditemukan
saat menulis `LeadRemoteDataSource`, sebelum sempat jadi bug diam-diam (`meta.total` selalu 0/salah
setelah baca dari cache). `sendListEnvelope()` baru, `_decode`/`_decodeChecked` dipecah supaya logika
"periksa status → lempar `ApiError`" tidak diduplikasi antara dua method.

### `AuthBloc` menerima event dari fitur LAIN — celah arsitektur pertama yang genap ketahuan

#69/#70 membangun `AuthSessionExpired` hanya untuk jalur bootstrap/login `AuthBloc` sendiri. My Leads
adalah **fitur pertama** yang membuat panggilan API terautentikasi sendiri, di luar alur auth — dan
langsung memunculkan celah: kalau `LeadsBloc`'s panggilan sendiri mendapat `SessionExpiredFailure`,
tidak ada mekanisme sebelumnya untuk memberi tahu `AuthBloc`. Design brief §10 eksplisit: layar Sesi
Berakhir bisa muncul "di layar mana pun", bukan cuma di titik masuk. **`AuthSessionInvalidated`** event
baru ditambahkan ke `AuthBloc` — fitur mana pun (lewat `sl<AuthBloc>()`, yang memang singleton) bisa
memberi tahu "sesi baru saja berakhir" tanpa `AuthBloc` perlu tahu fitur itu ada. `LeadRepositoryImpl`/
`GetMyLeadsUseCase` sendiri **tidak pernah** tahu `AuthBloc` ada — koordinasi terjadi murni di
`LeadsBloc` (presentasi-ke-presentasi), domain/data tetap bersih.

### Status filter — chip horizontal, bukan bottom sheet multi-select

Mockup desain menunjukkan **dua bentuk berbeda** untuk filter status: baris chip horizontal (state
"Normal", terlihat single-select) dan bottom sheet dengan checkbox (state "Filter status (bottom
sheet)", terlihat multi-select) — **tidak jelas triggernya apa**, dan dua bentuk itu sendiri
berkontradiksi (single vs multi-select untuk field yang sama). Diselesaikan dengan memilih **satu**
bentuk yang konsisten dengan backend (`status` query param menerima CSV, tapi UI single-select tetap
valid subset-nya) dan dengan state "Normal" yang jadi rujukan utama: chip horizontal, scroll ke samping
untuk kedelapan status. Bottom sheet multi-select **tidak dibangun** — dicatat di
`docs/issues/071-my-leads.md`, bukan diam-diam diabaikan.

### Field pencarian — tidak ada di mockup, tapi eksplisit di cakupan issue

Tidak satu pun state mockup Lead Saya menampilkan kotak pencarian — hanya chip status. Teks cakupan
issue #71 sendiri eksplisit menyebut "Filter status + pencarian". Dibangun mengikuti bahasa visual tema
(`shared/theme.dart`'s token) karena tidak ada rujukan mockup langsung — placeholder "Cari nama lead...",
debounce 300ms (pola persis `crm_dashboard` #32's search box).

### Tanpa paginasi UI — keputusan sadar, bukan kelupaan

`meta.total` dibaca dan disimpan di `LeadListResult`, tapi tidak ada scroll-tak-berhingga atau navigasi
halaman di `LeadsPage` — hanya memuat halaman pertama (`per_page` bawaan backend). Untuk MVP dengan
jumlah lead per-Employee yang wajar, kompleksitas paginasi belum sepadan (Aturan #27). Kalau jumlah
lead per Employee pernah melebihi satu halaman, lead lama akan hilang dari daftar — dicatat sebagai
keterbatasan diketahui di `docs/issues/071-my-leads.md`.

### "Aksi tulis saat offline gagal dengan pesan jelas" — vakum terpenuhi, bukan diverifikasi aktif

Kriteria acceptance ini ada di daftar #71 meski **cakupan issue murni baca** ("Hanya daftar. Menekan
satu lead belum membuka detail"). Tidak ada satu pun aksi tulis di layar ini untuk diuji. Dipenuhi
secara vakum oleh dua fakta: (1) tidak ada mekanisme antrian tulis offline di mana pun di codebase ini
(keputusan M3) untuk secara diam-diam mengaktifkannya, dan (2) `runApiCall`/pola repository yang sudah
ada untuk `AuthRepositoryImpl.login` sejak #69 sudah membuktikan kegagalan jaringan pada aksi tulis
menghasilkan `Failure` yang jelas, bukan diam. Diverifikasi sungguhan begitu #72 membangun aksi tulis
pertama (ubah status, catatan).

### Verifikasi manual — lapis penuh terhadap `crm_be` sungguhan, termasuk SQLite asli

Skrip `flutter test` sekali-pakai (tidak di-commit) menjalankan seluruh stack produksi
(`LeadRemoteDataSourceImpl` → `cachedGet` → `LeadRepositoryImpl` → `GetMyLeadsUseCase`) terhadap
`crm_be` sungguhan (Postgres asli via `docker compose`) **dan** `SqfliteResponseCache` sungguhan (lewat
`sqflite_common_ffi`, database SQLite betulan — bukan cuma fake yang mengimplementasikan interface):

1. Dua employee, dua lead — masing-masing di-assign ke satu employee berbeda. Employee pertama login →
   `GetMyLeadsUseCase` → **tepat 1 lead**, namanya cocok yang di-assign ke dia, bukan lead yang lain.
2. `ApiClient` diarahkan ke `http://127.0.0.1:1` (connection refused sungguhan, bukan simulasi) — hasil
   kedua **identik** dengan hasil pertama, `fromCache: true`, `fetchedAt` terisi nyata.
3. `AuthRepositoryImpl.logout()` sungguhan dipanggil → cache dikonfirmasi kosong (panggilan offline
   berikutnya gagal, bukan lagi mengembalikan data basi).
4. Login sebagai employee kedua → melihat lead **yang berbeda** (namanya cocok assignment-nya sendiri),
   membuktikan AC #3 dan cache-per-user tidak bocor lintas sesi pada perangkat yang sama.

Widget smoke test terpisah (juga tidak di-commit) memompa `LeadsPage` lewat keadaan loading/isi/
kosong/error/banner-cache, termasuk interaksi tap chip dan input pencarian — tidak ada exception
render. **Satu bug harness ditemukan dan diperbaiki di skrip verifikasi itu sendiri** (bukan di kode
produksi): pemompaan awal tidak membungkus `LeadsPage` dengan `Scaffold`, memicu "No Material widget
found" pada `TextField` — `LeadsPage` memang tidak punya `Scaffold` sendiri (selalu hidup di dalam
`Scaffold` milik `AppShell`), jadi ini murni kesalahan harness test, bukan kode aplikasi.
`flutter build apk --debug` diulang, sukses.

### Test

30 test baru: `sqflite_response_cache_test.dart` (6, SQLite asli lewat `sqflite_common_ffi`),
`cached_get_test.dart` (5), `run_api_call_test.dart` (6), `lead_repository_impl_test.dart` (5),
`leads_bloc_test.dart` (8, termasuk pembuktian nyata bahwa `SessionExpiredFailure` benar-benar
mengubah state `AuthBloc`, bukan cuma memanggil `add()` lalu berasumsi). Satu test lama
(`auth_repository_impl_test.dart`) ditambah satu kasus untuk `logout` membersihkan cache. Total 72
test, seluruhnya lolos.

### Batas issue ini

Menekan satu lead **belum** membuka detail — itu #72. Notifikasi, push, Tugas Saya sungguhan — #73.

---

## #72 — Detail Lead: timeline, telepon/WhatsApp auto-Activity, ubah status, catatan

Layar dengan seluruh aksi tulis pertama di mobile — dan alasan produk ini ada (*follow-up dari HP*).
Empat aksi (telepon, WhatsApp, ubah status, catatan) + timeline, dibangun di atas fondasi Bloc + Clean
Architecture #69 sudah tetapkan: domain (entities, repository interfaces, use cases) → data (models,
data sources, repository impl) → presentation (`LeadDetailBloc` + halaman).

### Dua port logika murni — sebelum Bloc-nya ditulis

`lead_status.dart` (transisi status yang sah) dan `activity_text.dart` (format timeline) ditulis
**duluan**, sebagai dependency `LeadDetailBloc`/UI, bukan sesudahnya:

- **`lead_status.dart`** — port baris-per-baris dari `crm_dashboard/src/lib/lead-status.ts`'s
  `isValidStatusTransition`/`statusTransitionOptions`, yang sendiri adalah port tangan dari
  `crm_be/internal/lead/usecase.go`'s `validateStatusTransition` (diverifikasi ulang langsung terhadap
  source Go, bukan diasumsikan tidak berubah sejak Phase 2). `statusTransitionOptionsTest` di-porting
  **satu-satu**, termasuk test "lost dibatasi ke satu opsi reopen-ke-new" — penyempitan UI yang sengaja
  dari backend yang sebenarnya izinkan reopen ke status main-path mana pun.
- **`activity_text.dart`** — port dari `crm_dashboard/src/lib/activity-text.ts`'s
  `activityToTimelineEntry`, dengan **satu perbedaan disengaja**: dashboard resolve `actor_membership_id`
  ke nama lewat `namesById` (`GET /v1/memberships`); Employee **tidak punya** `ActionMembershipList`
  (`crm_be/internal/shared/authz/authz.go` — dikonfirmasi langsung di source, bukan diasumsikan dari
  nama issue). Diganti: `"Anda"` bila `actor_membership_id` cocok sesi sendiri, `"Anggota tim lain"`
  (istilah generik, sengaja tidak mengklaim nama/peran) untuk yang lain. Dicocokkan terhadap mockup
  Claude Design asli, yang menampilkan teks literal "Ditugaskan ke Anda" — desain memang sudah dibangun
  di atas batasan ini, bukan kebetulan cocok.

### `LeadDetailResult` — celah cache banner ditemukan sebelum jadi bug

`LeadRepository.getLeadDetail` semula (draf pertama) mengembalikan `Lead` telanjang, membuang
`fromCache`/`fetchedAt` yang `cachedGet` sudah hitung — padahal design brief §10 eksplisit: banner
"dari cache, tanpa sinyal" berlaku untuk Detail Lead, bukan cuma Lead Saya. Ditemukan saat menulis
`LeadDetailBloc`'s state (butuh dua field itu untuk `LeadDetailLoaded`), sebelum dipakai di UI —
diperbaiki dengan `LeadDetailResult` (pola sama seperti `LeadListResult`/`ActivityListResult`),
`GetLeadDetailUseCase` dan `LeadRepositoryImpl.getLeadDetail` disesuaikan.

### `LeadDetailBloc` — satu state `Loaded`, empat sub-state transien di atasnya

Berbeda dari `LeadsBloc`'s beberapa top-level state (`Loading`/`Loaded`/`Error`), keempat aksi tulis
(ubah status, catatan, telepon, WhatsApp) semuanya **sub-state transien di atas `LeadDetailLoaded`
yang sama** (`isUpdatingStatus`/`statusError`/`conflict`, `isSubmittingNote`/`noteError`,
`isLaunchingExternalAction`/`externalActionError`) — bukan state top-level terpisah. Alasannya:
lead + timeline harus tetap di layar sepanjang aksi berlangsung (dialog konflik §8.2 melayang **di
atas** konten terakhir yang valid, tidak menggantikannya).

Pola tulis-lalu-refresh: aksi tulis yang berhasil (ubah status, catatan, telepon, WhatsApp) selalu
diikuti **refetch aktivitas sungguhan** dari server (`_withFreshActivities`), bukan menyisipkan entry
palsu di klien — server-lah yang mencatat `status_changed`, jadi menebak bentuknya di klien berisiko
berbeda diam-diam. `LeadStatusConflictAcknowledged` ("muat ulang") memuat ulang **penuh** (lead +
aktivitas), bukan sekadar menutup dialog — satu-satunya keluar dari data basi.

**Celah kedua ditemukan sebelum jadi bug**: `LeadDetailError` semula tidak membawa `leadId` — cukup
untuk kegagalan muat pertama, tapi retry setelah kegagalan **refresh** (dari state `Loaded` yang sudah
kehilangan lead-nya begitu error) tidak punya cara mengetahui lead mana yang harus dimuat ulang.
Ditemukan saat menulis `_ErrorView`'s tombol "Coba lagi" — `LeadDetailError` sekarang membawa `leadId`.

### Aksi eksternal — `_launchAndLog` dipakai bersama telepon & WhatsApp

Design brief §8.3: activity hanya dicatat **setelah** OS benar-benar membuka aplikasi eksternal, tidak
pernah saat tombol ditekan. `ExternalActionRepository.launchDialer`/`launchWhatsApp` mengembalikan
`bool` polos (bukan `Either`) — sinyal terkuat yang bisa didapat Flutter, tidak bisa tahu apakah
telepon **benar-benar tersambung** atau WhatsApp benar-benar dibalas. `_launchAndLog` (satu helper,
dipakai `LeadCallRequested` maupun `LeadWhatsAppRequested`): panggil launch → `false` (dibatalkan
sebelum handoff) → tidak ada activity, tidak ada error (pilihan sadar pengguna, bukan kegagalan);
`true` → log activity, lalu refetch timeline. Bila log-nya sendiri gagal (mis. koneksi putus tepat
setelah dialer terbuka), pesan eksplisit "Aksi berhasil, tapi gagal dicatat" — **tidak pernah**
berpura-pura aksinya sendiri tidak terjadi.

`Lead.phone` vs `Lead.phoneE164` (acceptance criterion #6): dialer selalu pakai `phone` apa adanya
(nomor tanpa format E.164 tetap bisa ditelepon); tombol WhatsApp hanya aktif bila `phoneE164` non-null
(`wa.me` butuh nomor internasional asli). `_ActionBar` menggerbangi kedua tombol secara independen.

### `noteError` — draf pertama salah menaruhnya di snackbar, dikoreksi sebelum PR

Draf pertama menyalurkan `noteError` lewat `ScaffoldMessenger` yang sama dengan `statusError`/
`externalActionError` (toast generik) — tapi design brief §10 eksplisit minta "kesalahan validasi per
field" untuk **form catatan** secara khusus, bukan toast yang bisa terlewat atau bertahan lebih lama
dari field yang dibicarakannya. Dikoreksi sebelum PR dibuka: `noteError` dikeluarkan dari
`BlocConsumer`'s listener, dialirkan sebagai `InputDecoration.errorText` langsung di bawah `TextField`
catatan (`_NoteForm`) — `statusError`/`externalActionError` tetap lewat snackbar karena keduanya bukan
kesalahan field, melainkan hasil aksi (ubah status, telepon/WhatsApp) yang tidak terikat satu widget
input.

### `CacheBanner` diekstrak ke `shared/widgets/` — implementasi kedua yang nyata

Lead Saya (#71) sudah punya `_CacheBanner` privat; Detail Lead butuh **persis** yang sama (Aturan #28:
ini implementasi kedua yang nyata, bukan abstraksi spekulatif). Diekstrak ke
`shared/widgets/cache_banner.dart`, `LeadsPage` ditulis ulang memakainya — tidak ada perubahan
perilaku, dibuktikan `flutter analyze`/test lama tetap lolos tanpa perubahan asersi.

### Verifikasi manual — end-to-end penuh terhadap `crm_be` sungguhan

Backend dev (`docker compose` Postgres, `api` lokal) dipakai untuk membuktikan seluruh AC issue ini
dengan data nyata, bukan hanya unit test dengan mock:

1. Owner + Employee + satu lead (di-assign lewat `PATCH /v1/leads/{id}/assignment`) disiapkan lewat
   `crm_be` sungguhan.
2. **`PATCH .../status`** — sukses (`new`→`contacted`, version naik 2→3) **dan** versi basi (`version:
   2` setelah versi sungguhan sudah 3) → **`409 version_conflict`** nyata, `current` berisi state
   terkini persis seperti `respondLeadError`'s Go source.
3. **`POST .../activities`** — `note_added`, `call_logged`, `whatsapp_opened` masing-masing dikirim
   sebagai Employee, lalu **dibaca ulang lewat sesi Owner yang terpisah** (`GET .../activities` dengan
   token Owner, bukan token Employee yang sama) — memenuhi acceptance criterion secara harfiah:
   "diverifikasi dari sisi Owner, bukan hanya dari mobile". Seluruhnya muncul, urutan terbaru dulu.
4. **Isolasi tenant** dicek ulang khusus untuk endpoint baru: Employee ketiga yang **tidak** di-assign
   ke lead ini → `GET /v1/leads/{id}` → `404`, bukan `403` (Aturan #6), dikonfirmasi bukan hanya untuk
   `GET /v1/leads` (#71) tapi juga endpoint detail yang baru ditambahkan issue ini.
5. Skrip `flutter test` sekali-pakai (tidak di-commit) menjalankan stack produksi **sungguhan**
   (`LeadRemoteDataSourceImpl`/`ActivityRemoteDataSourceImpl` → `runApiCall` → `LeadRepositoryImpl`/
   `ActivityRepositoryImpl`) terhadap `crm_be` di atas via `ApiClient` asli dengan token asli —
   membuktikan `phone_e164`, `version_conflict` → `VersionConflictFailure<Lead>` dengan `current` yang
   ter-parse benar, dan urutan timeline (`whatsapp_opened, call_logged, note_added, status_changed,
   lead_assigned, lead_created`) — seluruhnya lolos terhadap JSON **asli** dari server, bukan fixture
   tangan.

Widget smoke test terpisah (juga tidak di-commit) memompa `LeadDetailPage` lewat keadaan loading/
error/loaded (dengan & tanpa telepon/WhatsApp, status `lost`)/transisi-ke-konflik — tidak ada
exception render; transisi state nyata (bukan initial state) dipakai khusus untuk membuktikan dialog
konflik muncul lewat `BlocConsumer.listenWhen`, bukan cuma dirender statis. `flutter build apk --debug`
sukses.

**Tidak diverifikasi** (keterbatasan lingkungan sandbox ini, sama seperti #69–#71): dialer/WhatsApp OS
sungguhan tidak bisa dibuka dari lingkungan ini — `ExternalActionRepositoryImpl`/`url_launcher` hanya
teruji lewat mock (`external_action_repository_impl_test.dart`, membuktikan URI `tel:`/`wa.me` yang
dibangun, bukan handoff OS-nya). Perangkat Android sungguhan masih jadi kewajiban manusia sebelum #73
menutup phase.

### Test

64 test baru: `lead_status_test.dart` (9, matriks penuh + `statusTransitionOptions` — pola #33),
`activity_text_test.dart` (25, seluruh 10 tipe activity + kasus "Anda" vs generik vs `null`),
`activity_repository_impl_test.dart` (4, file baru), `external_action_repository_impl_test.dart` (4,
file baru, URI `tel:`/`wa.me`), `lead_repository_impl_test.dart` (+5 kasus: `getLeadDetail` ×2,
`updateStatus` ×3 termasuk `VersionConflictFailure<Lead>` nyata), `lead_detail_bloc_test.dart` (12,
file baru — termasuk pembuktian "canceled launch tidak pernah memanggil log" dan "conflict tidak
pernah memanggil refresh aktivitas"). Total suite sekarang 130 test, seluruhnya lolos. `flutter
analyze` bersih.

### Batas issue ini

Tidak membuat lead baru, tidak mengonversi ke Customer — sesuai cakupan (Employee tidak punya
`ActionLeadCreate`/`ActionLeadConvert`). Edit `secure_store.dart` (`_storage`→`storage`, dihasilkan
IDE, tidak berbahaya) masih tertahan di `git stash` sejak #71 — belum diterapkan atau dibuang, masih
menunggu keputusan.

