# Phase 5 — Employee Mobile · Technical Design

> **Bagaimana.** Apa & kenapa di [`prd.md`](./prd.md).
> Hanya **delta** untuk phase ini — aturan yang sudah ada di [`freeze.md`](../../architecture/freeze.md) dirujuk, tidak diulang.
> Sumber: freeze bagian 4 (Phase 5), A3, 2.3, 5.1 (Aturan #24) · Aturan #1–#7, #12, #13, #16, #18, #26, #32, #36 · [ADR-011](../../decisions/ADR-011-layered-packages-and-unit-of-work.md) · [`architecture/authentication.md`](../../architecture/authentication.md)

---

## 1. Batas platform — apa arti "Android dulu" secara konkret

Keputusan M1 adalah soal **build, uji, dan rilis** — bukan soal menulis kode Android-only.

| Boleh | Tidak boleh |
|---|---|
| `crm_employee/ios/` digenerate `flutter create` dan dibiarkan apa adanya | Menghapus folder `ios/`, atau menambahkan `platforms:` yang mengecualikannya |
| Paket dipilih yang mendukung Android **dan** iOS | Memilih paket Android-only saat ada alternatif lintas platform setara |
| `flutter build apk` diuji; `flutter build ipa` tidak pernah dijalankan | Menulis `if (Platform.isAndroid)` untuk hal yang sebenarnya lintas platform |
| Firebase dikonfigurasi untuk Android saja (`google-services.json`) | Menulis kode push yang mengasumsikan hanya FCM-langsung (iOS lewat APNs punya bentuk yang sama di sisi Dart) |
| `device_tokens.platform` menerima `'android'` dan `'ios'` sejak awal | Membuat kolomnya `CHECK (platform = 'android')` |

Mengaktifkan iOS kelak harus menjadi pekerjaan **konfigurasi + build**, bukan penulisan ulang.
Setiap keputusan di bawah dinilai terhadap batasan itu.

---

## 2. Toolchain — FVM, versi dipin

```
.fvmrc                     { "flutter": "3.44.0" }        ← di-commit
crm_employee/.fvmrc        idem (FVM membaca dari akar proyek Flutter)
```

Flutter **3.44.0** (Dart 3.12.0, stable, rilis 2026-05-15) — diverifikasi jalan lewat
`fvm spawn 3.44.0 --version` sebelum ditulis di sini.

**Kenapa dipin.** Mesin pengembangan ini sudah punya dua Flutter berbeda hari ini: `flutter` di
`PATH` adalah **3.38.9**, sementara FVM menyimpan **3.44.0** dan **3.44.3**. Tanpa pin, dua mesin
sudah tidak sepakat sekarang — apalagi CI. Ini alasan yang sama kenapa `go.mod` mengunci versi Go.

Seluruh perintah Flutter di phase ini dijalankan lewat `fvm flutter …` / `fvm dart …`.
`Makefile` di akar mendapat target `mobile-*` yang membungkusnya (§13), sehingga tidak ada yang perlu
mengingat prefix `fvm`.

> `fvm list` menandai 3.44.0 sebagai *Need setup*. `fvm spawn`/`fvm use` pertama akan menyiapkannya
> sendiri — bukan langkah manual terpisah.

---

## 3. Struktur `crm_employee/`

Mengikuti bentuk yang sudah terbukti di `crm_dashboard`: **logika murni dipisah dari widget**, supaya
bisa diuji tanpa merender apa pun (pola `lib/nav.ts`, `lib/lead-filters.ts`, `lib/api-key-rows.ts`).

```
crm_employee/
  .fvmrc
  pubspec.yaml
  analysis_options.yaml          flutter_lints, dinaikkan (§12)
  lib/
    main.dart
    app.dart                     MaterialApp, router, theme
    core/
      api_client.dart            HTTP + refresh single-flight (§5)
      api_error.dart             bentuk error envelope {code,message,details}
      session.dart               ChangeNotifier — status login, membership, role
      secure_store.dart          pembungkus flutter_secure_storage
      cache.dart                 cache baca offline (§7)
      config.dart                base URL per flavor
    features/
      auth/                      login, biometric gate
      leads/                     My Leads, detail, aksi
      tasks/                     My Tasks
      notifications/             daftar notifikasi + FCM handler
    shared/
      labels.dart                status/alasan/sumber → Bahasa Indonesia (§11)
      theme.dart                 token warna & tipografi dari design output
      widgets/                   komponen dipakai >1 layar
  test/                          unit test — logika murni, tanpa widget
  android/
  ios/                           digenerate, tidak pernah dibangun (§1)
```

**`labels.dart` menyalin nilai dari `crm_dashboard/src/lib/labels.ts`, bukan mengimpornya.** Tidak ada
mekanisme berbagi kode Dart↔TypeScript, dan membangunnya (codegen dari satu sumber) adalah abstraksi
untuk masalah yang belum ada (Aturan #27). Yang dijaga: `glossary.md` tetap satu-satunya sumber
istilah, dan test mengunci daftar nilainya sama panjang dengan enum backend.

---

## 4. Autentikasi & penyimpanan token

Jalur mobile **sudah ada sejak Phase 1** — tidak ada perubahan backend di sini.

| Aspek | Ketentuan |
|---|---|
| Login | `POST /v1/auth/login` dengan `{"client": "mobile"}` → refresh token di **body**, bukan cookie |
| Access token | JWT, TTL 15 menit, dikirim `Authorization: Bearer` |
| Refresh token | Opaque, TTL 2160h (90 hari), **rotasi** setiap refresh |
| Penyimpanan | **`flutter_secure_storage`** — Android Keystore / EncryptedSharedPreferences. **Tidak pernah** `SharedPreferences` biasa (kriteria #5) |
| CSRF | **Tidak berlaku** — token dikirim eksplisit di header, tidak pernah otomatis oleh browser (`authentication.md`) |
| API key | **Tidak pernah disentuh** (Aturan #24). Tidak ada satu baris pun di `crm_employee/` yang mengenal format `jln_*` |

### 4.1 Biometric

`local_auth`. Gerbang **saat aplikasi dibuka kembali**, bukan pengganti login pertama:

```
Buka aplikasi
  └─ ada refresh token di secure storage?
       ├─ tidak → layar login (email + password)
       └─ ya   → minta biometric
                  ├─ sukses → refresh access token → masuk
                  └─ gagal/batal → TIDAK masuk; tawarkan "Masuk dengan password"
```

Kriteria #6 menuntut biometric gagal **menolak masuk** — jadi kegagalannya tidak boleh diam-diam
jatuh ke sesi yang sudah ada. Perangkat tanpa biometric terdaftar jatuh ke password, bukan dilewati.

### 4.2 Kehilangan akses saat membership dinonaktifkan

Phase 2 (#22) sudah mencabut seluruh `refresh_tokens` milik membership dalam transaksi penonaktifan.
Konsekuensinya di mobile: access token yang masih hidup tetap bekerja **sampai 15 menit**, lalu
refresh berikutnya gagal `401` dan aplikasi keluar ke layar login. Kriteria #7 menuntut ini
**diverifikasi langsung**, bukan disimpulkan dari kode Phase 2 — jendela 15 menit itu perilaku nyata
yang harus dilihat, bukan ditebak.

---

## 5. `ApiClient` — menyalin bentuk `crm_dashboard`, termasuk single-flight

`crm_dashboard/src/lib/api-client.ts` sudah memecahkan masalah yang sama dan **diuji konkurensinya
secara genuine** (#31: 6 panggilan paralel yang 401 bersamaan → tepat 1 refresh). Bentuknya disalin,
bukan ditemukan ulang:

```dart
Future<T> send<T>(...) async {
  var res = await _raw(...);
  if (res.statusCode != 401) return _decode(res);

  // Satu Future refresh dipakai bersama — ditetapkan SEBELUM await apa pun,
  // supaya request paralel yang 401 bersamaan tidak memicu refresh masing-masing.
  _refreshFuture ??= _doRefresh();
  final ok = await _refreshFuture;
  ...
}
```

Yang berbeda dari dashboard: refresh token dibaca dari secure storage dan dikirim di **body**, bukan
diambil dari cookie. Kegagalan refresh → hapus token dari secure storage, `Session` pindah ke
keadaan keluar.

**Test wajib** (§12): satu test yang menahan refresh lewat `Completer` sampai beberapa panggilan
paralel mencapai titik single-flight, membuktikan `/v1/auth/refresh` dipanggil **tepat sekali** —
pola yang sama seperti `api-client.test.ts`.

---

## 6. State — `provider`, bukan hand-rolled

`ChangeNotifier` + `provider`. Satu paket kecil, stabil, dan luas dipakai.

**Kenapa bukan tanpa paket sama sekali** (yang lebih sesuai refleks Aturan #27): `InheritedWidget`
buatan tangan untuk state sesi menghasilkan **lebih banyak** kode boilerplate untuk hasil yang sama,
dan boilerplate itu justru tempat bug muncul. Aturan #27 melarang abstraksi *sebelum kebutuhannya
nyata* — state sesi lintas layar adalah kebutuhan hari pertama, bukan antisipasi.

**Kenapa bukan Riverpod/Bloc**: keduanya membawa konsep baru (provider scope, event/state stream)
untuk aplikasi berlayar sedikit. Bisa ditinjau ulang bila jumlah layar tumbuh — itu pemicunya.

---

## 7. Cache baca offline

Kriteria #2 — *"daftar lead tetap terbaca saat mode pesawat"*.

**Bentuk: satu tabel key-value di SQLite** (`sqflite`), berisi **respons sukses terakhir** per
permintaan:

```sql
CREATE TABLE response_cache (
  key        TEXT PRIMARY KEY,   -- mis. "GET /v1/leads?status=new&page=1"
  body       TEXT NOT NULL,      -- JSON mentah, apa adanya
  fetched_at INTEGER NOT NULL    -- epoch ms, untuk ditampilkan "data per <waktu>"
);
```

Alur baca: coba jaringan → sukses → simpan & tampilkan; gagal jaringan → baca cache → tampilkan
dengan penanda **"Data terakhir diperbarui <waktu>"**.

**Kenapa key-value JSON mentah, bukan tabel domain lokal.** Yang dibutuhkan kriteria #2 adalah
*menampilkan kembali apa yang terakhir terlihat* — bukan query lokal, bukan join, bukan filter
offline. Tabel domain lokal (drift/ORM) berarti menjaga skema kedua yang harus ikut berubah setiap
kali backend berubah — biaya nyata untuk kemampuan yang tidak diminta (Aturan #27, #28).

**Yang di-cache:** `GET /v1/leads`, `GET /v1/tasks`, `GET /v1/leads/{id}`,
`GET /v1/leads/{id}/activities`. **Yang tidak:** apa pun yang bukan `GET`, dan `GET /v1/me`
(sesi ditentukan token, bukan cache).

**Dihapus saat logout** — cache berisi data lead satu organization; perangkat yang berpindah pengguna
tidak boleh menampilkan sisa data pengguna sebelumnya.

> **Bukan** antrian tulis offline (keputusan M3). Aksi tulis saat offline **gagal dengan pesan jelas**,
> tidak diantre diam-diam.

---

## 8. Backend — migration `0006_device_tokens`

Satu-satunya perubahan schema phase ini.

```sql
CREATE TABLE device_tokens (
    id              uuid PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations (id),
    membership_id   uuid NOT NULL,
    token           text NOT NULL,
    platform        text NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    last_seen_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT ck_device_tokens_platform CHECK (platform IN ('android','ios')),
    CONSTRAINT uq_device_tokens_id_org   UNIQUE (id, organization_id),
    CONSTRAINT uq_device_tokens_token    UNIQUE (token),
    CONSTRAINT fk_device_tokens_membership
        FOREIGN KEY (membership_id, organization_id)
        REFERENCES memberships (id, organization_id)
);

CREATE INDEX idx_device_tokens_org_membership
    ON device_tokens (organization_id, membership_id);
```

| Aturan | Pemenuhan |
|---|---|
| #1 `organization_id` di setiap tabel bisnis | ✅ |
| #2 `UNIQUE (id, organization_id)` | ✅ `uq_device_tokens_id_org` |
| #3 FK ke tabel tenant-scoped selalu composite | ✅ `(membership_id, organization_id)` |
| #12 PK UUIDv7 dari aplikasi, tanpa `DEFAULT` | ✅ |
| #13 `timestamptz` | ✅ |
| #16 index tenant-aware berawalan `organization_id` | ✅ |
| #18 soft delete untuk entity bisnis | ❌ **sengaja tidak** — lihat di bawah |

**`uq_device_tokens_token` unik lintas organization**, bukan composite. Ini pengecualian **ketiga**
di codebase ini, sekelas dengan `api_keys.key_id` (#46) dan `refresh_tokens.token_hash` (#10): token
FCM mengidentifikasi **satu instalasi aplikasi di satu perangkat**, dan perangkat itu bisa berpindah
pengguna (employee resign, HP dipakai orang lain, employee pindah organization). Unik global +
upsert membuat pendaftaran ulang **memindahkan baris** ke membership yang sekarang login — tepat
perilaku yang benar. Composite akan membiarkan satu perangkat fisik menerima push milik dua
organization sekaligus. Alasannya ditulis sebagai komentar di migration.

**Tanpa `deleted_at`** (deviasi sadar dari Aturan #18): `device_tokens` bukan entity bisnis — tidak
punya nilai audit, tidak pernah dirujuk laporan, dan kriteria #12 justru **menuntut** token mati
benar-benar hilang. Soft delete di sini berarti menyimpan sampah selamanya sambil harus mengingat
memfilternya di setiap query.

---

## 9. Backend — pengiriman FCM

### 9.1 Paket `internal/device`

Bentuk lima berkas ADR-011 (`entity/port/usecase/repository_postgres/handler_http`).

| Endpoint | Principal | Guna |
|---|---|---|
| `POST /v1/device-tokens` | user (Employee dkk.) | Daftarkan/segarkan token perangkat. Upsert pada `token`; memperbarui `membership_id` + `last_seen_at` |
| `DELETE /v1/device-tokens` | user | Hapus token milik perangkat ini saat logout |

Action `authz` baru: `device_token.register`, `device_token.delete` — dimiliki **seluruh role**
(Owner sampai Employee). Owner memakai dashboard hari ini, tapi tidak ada alasan menutup pintu bagi
Owner yang kelak memasang aplikasi.

### 9.2 Pengirim — HTTP v1 langsung (keputusan M4)

`internal/shared/push`, sepola `internal/shared/mailer`:

```go
type Message struct {
    Token string
    Title string
    Body  string
    Data  map[string]string  // {"type":"lead_assigned","lead_id":"…"} → deeplink
}

type Sender interface {
    Send(ctx context.Context, msg Message) error
}
```

Dua implementasi, dipilih `PUSH_PROVIDER` — **bentuk yang sama persis seperti `MAIL_PROVIDER`**
(Phase 4.6):

| `PUSH_PROVIDER` | Implementasi | Catatan |
|---|---|---|
| `none` | `NoopSender` | Mencatat ke log, tidak mengirim. **Ditolak boot saat `APP_ENV=production`** (Aturan #36) |
| `fcm` | `FCMSender` | `POST https://fcm.googleapis.com/v1/projects/{id}/messages:send` |

Token OAuth2 dari service account lewat `golang.org/x/oauth2/google` (`JWTConfigFromJSON` →
`TokenSource`, yang menyegarkan sendiri). **Satu dependency baru** — bukan pohon dependency Firebase
Admin SDK, yang membawa Firestore/Auth/Storage yang tidak satu pun dipakai (Aturan #27, alasan yang
sama seperti memilih `net/smtp` di Phase 4.6).

Batas waktu HTTP eksplisit (`PUSH_TIMEOUT`, default `10s`) — **pelajaran langsung dari #63**: di sana
`smtp.SendMail` ternyata tidak punya batas waktu sama sekali, dan `Send` dipanggil sinkron di jalur
request. Di sini jalurnya sama, jadi batas waktunya dipasang sejak awal, bukan ditemukan belakangan.

### 9.3 Kapan dikirim (keputusan M5)

```go
// internal/lead/usecase.go — UpdateAssignment
txErr := u.store.InTx(ctx, func(r Repos) error {
    …
    return r.Notification.Notify(ctx, t, newAssignee, "lead_assigned", &id, nil, title, &updated.Name)
})
if txErr != nil { … }

// SETELAH commit — Aturan #32. Kegagalan hanya dicatat, tidak pernah
// membatalkan assignment atau record notification yang sudah tersimpan.
u.pushAssignmentNotification(ctx, t, newAssignee, updated)
```

Persis pola `sendVerificationEmail` sejak Phase 1. Freeze A3 sudah menetapkan record notification
adalah **source of truth** dan push hanya pengantar *best-effort* — jadi push yang gagal memang tidak
boleh punya konsekuensi apa pun terhadap data (kriteria #11).

`lead` mendeklarasikan interface konsumennya sendiri (`PushSender`, satu method) sesuai ADR-011,
dijembatani di composition root — pola yang sama seperti `NotificationSender` dan
`ActivityRecorder`.

### 9.4 Token mati dibersihkan (kriteria #12)

FCM v1 mengembalikan `404 UNREGISTERED` atau `400 INVALID_ARGUMENT` untuk token yang aplikasinya
sudah di-uninstall. Respons itu **dipakai**: token tersebut dihapus dari `device_tokens`. Tanpa ini
tabelnya hanya bertambah selamanya — kelas masalah yang sama seperti peta tanpa eviction di Phase 4.5.

### 9.5 Rahasia (Aturan #26)

| Aturan |
|---|
| Service account JSON **tidak pernah** di-commit. `FCM_CREDENTIALS_FILE` menunjuk path di luar repo; `.gitignore` menutup polanya |
| Isi token FCM perangkat **tidak pernah** masuk log — hanya `membership_id` dan hasil kirim |
| Body notifikasi boleh dicatat (judul lead), token tidak — sepola `mailer` yang mencatat `to` tapi bukan isi |

---

## 10. Deeplink dari push

`firebase_messaging` menyediakan tiga jalur, ketiganya harus ditangani:

| Keadaan aplikasi | Jalur |
|---|---|
| Terbuka (foreground) | `onMessage` → tampilkan in-app banner, jangan pindah layar paksa |
| Latar belakang, ditekan | `onMessageOpenedApp` → navigasi ke lead |
| Mati, dibuka dari notifikasi | `getInitialMessage` → navigasi ke lead **setelah** sesi siap |

`data.lead_id` yang menentukan tujuan. Kasus yang wajib benar: notifikasi ditekan saat **belum
login** → simpan tujuan, buka setelah login berhasil, bukan hilang diam-diam.

**Tanpa paket deeplink eksternal** — tidak ada tautan `https://` dari luar aplikasi yang perlu
ditangani di phase ini (itu Phase 6+ bila pernah ada).

---

## 11. Bahasa & istilah

Aturan #12 `CLAUDE.md`: seluruh teks antarmuka **Bahasa Indonesia**. `shared/labels.dart` menyalin
peta dari `crm_dashboard/src/lib/labels.ts` — 8 status, 6 alasan kalah, 4 sumber, 4 role.
"Lead" dan "Customer" **tetap** sebagaimana di dashboard: keduanya istilah `glossary.md`.

---

## 12. Rencana test

Aplikasi Flutter tidak punya harness sekelas testcontainers, dan **memaksakannya bukan tujuan phase
ini**. Yang diuji adalah bagian yang paling mungkin salah diam-diam:

| Berkas | Menguji |
|---|---|
| `test/api_client_test.dart` | Refresh **single-flight** — beberapa 401 paralel → tepat 1 refresh (pola `api-client.test.ts` #31). `MockClient` dari `package:http` |
| `test/api_client_test.dart` | Refresh gagal → token dihapus dari secure storage, sesi keluar |
| `test/cache_test.dart` | Simpan→baca respons; jaringan gagal → cache dipakai; logout → cache kosong |
| `test/labels_test.dart` | Jumlah & nilai enum cocok dengan backend — 8/6/4/4, mengunci drift dari `labels.ts` |
| `test/lead_status_test.dart` | Transisi status yang ditawarkan UI ⊆ yang backend izinkan (pola `lead-status.ts` #33) |
| `crm_be/internal/device/*_test.go` | Repository (Postgres asli), handler, upsert token pindah membership |
| `crm_be/internal/shared/push/*_test.go` | Bentuk payload FCM v1, penanganan `UNREGISTERED` → hapus token |
| `crm_be/cmd/api/tenant_isolation_test.go` | **Kasus baru** `DELETE /v1/device-tokens` — dan **tetap terbukti bisa gagal** |

**Widget test tidak diwajibkan** phase ini. Yang mengikat adalah kriteria #1/#2 — diverifikasi di HP
Android sungguhan, bukan lewat widget test yang lulus di CI sambil aplikasinya tidak jalan.

### 12.1 Verifikasi manual wajib (di HP nyata)

Kriteria #1, #2, #6, #7, #10 **hanya** bisa dibuktikan di perangkat. Prosedurnya ditulis ke
`docs/testing/flow/` sebagai berkas baru saat issue penutup, mengikuti bentuk yang sudah ada.

---

## 13. Konfigurasi & `Makefile`

Env baru di `internal/shared/config` (pola `MAIL_PROVIDER` Phase 4.6):

```
PUSH_PROVIDER=none|fcm          # none ditolak saat APP_ENV=production
FCM_PROJECT_ID=…                # wajib bila PUSH_PROVIDER=fcm
FCM_CREDENTIALS_FILE=…          # path service account JSON, wajib bila fcm
PUSH_TIMEOUT=10s
```

`Makefile` akar bertambah:

```make
mobile-get:      cd $(MOBILE) && fvm flutter pub get
mobile-analyze:  cd $(MOBILE) && fvm flutter analyze
mobile-test:     cd $(MOBILE) && fvm flutter test
mobile-run:      cd $(MOBILE) && fvm flutter run
mobile-apk:      cd $(MOBILE) && fvm flutter build apk --debug
```

CI: `.github/workflows/ci-employee.yml` dengan `paths: crm_employee/**`, Flutter dipin **3.44.0**,
menjalankan `analyze` + `test`. Tidak membangun APK di CI — build Android butuh
`google-services.json` yang sengaja tidak di-commit (§14).

---

## 14. Yang harus disiapkan pemilik produk — Firebase

Diblokir sampai ini ada: **hanya issue push**, bukan phase-nya.

### Langkah

1. **Buat project Firebase** — [console.firebase.google.com](https://console.firebase.google.com) →
   *Add project*. Google Analytics boleh dilewati (tidak dipakai).
2. **Daftarkan aplikasi Android** — *Add app* → Android:
   - **Package name**: `com.jualin.crm.employee` ← harus sama persis dengan `applicationId` di
     `android/app/build.gradle`
   - SHA-1 **tidak perlu** (itu untuk Google Sign-In/Dynamic Links, keduanya tidak dipakai)
   - Unduh **`google-services.json`** → taruh di `crm_employee/android/app/google-services.json`
3. **Buat service account untuk backend** — *Project settings* → *Service accounts* → *Generate new
   private key* → JSON. Simpan **di luar repository**, arahkan `FCM_CREDENTIALS_FILE` ke sana.
4. **Catat Project ID** (ada di *Project settings* → *General*) → `FCM_PROJECT_ID`.

### Alternatif: FlutterFire CLI

Belum terpasang di mesin ini (`flutterfire not found`, `~/.pub-cache/bin` kosong). Memasangnya:

```bash
fvm dart pub global activate flutterfire_cli
export PATH="$PATH":"$HOME/.pub-cache/bin"
cd crm_employee && flutterfire configure --platforms=android
```

Ia melakukan langkah 2 secara otomatis **dan** menghasilkan `lib/firebase_options.dart`. **Langkah 3
tetap manual** — service account untuk backend tidak disentuh FlutterFire CLI sama sekali.

### Yang tidak boleh di-commit

`.gitignore` `crm_employee/` menutup:

```
android/app/google-services.json
lib/firebase_options.dart
**/*serviceAccount*.json
```

`google-services.json` bukan rahasia dalam arti ketat (ia ikut terkirim di dalam APK), tapi
di-*ignore* supaya repository tidak terikat pada satu project Firebase, dan supaya tidak ada
kebiasaan menaruh berkas Firebase apa pun di repo — service account JSON yang **benar-benar** rahasia
kelihatan seperti berkas serupa (kriteria #14). Disediakan `.example` untuk keduanya.

---

## 15. Yang berubah pada dokumentasi

| Berkas | Perubahan |
|---|---|
| `architecture/authentication.md` | Bagian *Dashboard (cookie) vs Mobile (bearer)* — kolom mobile dari rencana jadi kenyataan; biometric & secure storage |
| `architecture/api.md` | Dua endpoint `device-tokens` di daftar endpoint |
| `architecture/authorization.md` | Matriks Phase 5 — dua action `device_token.*` |
| `architecture/multi-tenancy.md` | Pengecualian **ketiga** unik-lintas-organization (`device_tokens.token`); kasus isolasi baru di lapis 4 |
| `docs/testing/flow/` | Berkas baru: alur testing manual mobile di HP Android |
| `crm_employee/README.md` | Dari "belum dibuat" jadi cara menjalankan |
| `.env.example`, `docker-compose.yml` | `PUSH_PROVIDER` dkk. |
| `STATUS.md` | Baris Selesai; Phase 5 di *Progress per Phase*; *Punya Lead Time* — Apple/Firebase diperbarui |

`freeze.md` **tidak disentuh**. Kontradiksi cakupan offline (PRD) **dilaporkan**, bukan diperbaiki —
Aturan #30.

---

## 16. Risiko teknis

| Risiko | Penanganan |
|---|---|
| Phase ini jauh lebih besar dari phase manapun sebelumnya | 6 issue dengan batas tegas (`issues.md`). Backend & fondasi Flutter **tidak** menunggu desain |
| Hasil desain datang di tengah phase, seperti Phase 3 | Issue fondasi desain dibuat **sejak awal** (M6) — persis pelajaran dari #40 |
| Test Flutter jauh lebih tipis dari backend | Disadari & disengaja (§12). Yang mengikat kriteria #1/#2 adalah verifikasi di HP nyata |
| FCM tidak terkonfigurasi menghambat pengembangan | `PUSH_PROVIDER=none` membuat seluruh phase bisa dikerjakan tanpa Firebase sama sekali, kecuali satu issue push |
| Menulis kode yang mengunci ke Android | §1 menetapkan batasnya secara eksplisit dan bisa diperiksa saat review |
| `flutter_secure_storage` bermasalah di sebagian perangkat Android | Diketahui punya kasus tepi pada Android lama. Verifikasi kriteria #1 di HP nyata akan menangkapnya; bila muncul, dicatat sebagai temuan `docs/issues/`, bukan diakali diam-diam |

---

## 17. Kewajiban yang diteruskan ke phase berikutnya

- **Saat Apple Developer Program ada**: aktifkan iOS — `google-services.json` versi iOS, APNs key di
  Firebase, `flutter build ipa`. Tidak ada kode Dart yang perlu ditulis ulang bila §1 dipatuhi.
- **GATE setelah Phase 5** (freeze): cari 3–5 pengguna nyata sebelum Phase 6. Urutan Phase 6–9
  ditentukan apa yang mereka minta, bukan tebakan.
- **Antrian aksi offline** (M3): `version` sudah siap menerimanya. Membangunnya butuh keputusan
  eksplisit soal UI konflik — dan freeze bagian 2.3 sudah memperingatkan biayanya.
- **Bila jumlah layar tumbuh banyak**: tinjau ulang pilihan `provider` (§6). Itu pemicunya, bukan
  sebelum.
