# Phase 5 — Employee Mobile · PRD

> **Apa & kenapa.** Detail teknis di [`td.md`](./td.md). Arahan untuk desainer di [`design-brief.md`](./design-brief.md).
> Sumber: [`architecture/freeze.md`](../../architecture/freeze.md) bagian 4 (Phase 5), A3 (notification: penyimpanan vs pengiriman), 2.3 (`version` & antrian offline), 3.3 (wajib — Mobile), 5.1 (Aturan #24) · [ADR-003](../../decisions/ADR-003-employee-as-membership.md) · [`architecture/authentication.md`](../../architecture/authentication.md) bagian *Dashboard (cookie) vs Mobile (bearer)*

---

## Tujuan

**Produk menjadi nyata.** Freeze menyebut phase ini pembeda utama produk — dan ia benar: sampai
sekarang Jualin CRM adalah alat untuk **Owner**. Employee punya akun, punya lead yang ditugaskan
kepadanya, punya task, dan punya notifikasi yang menumpuk di tabel — tapi **tidak punya cara
memakainya**. Dashboard bukan alatnya (Phase 3 TD §2.4 sudah menyatakan itu; `authz.go` bahkan
menuliskan komentarnya: *"Employee gets mobile in Phase 5"*).

Kalimat inti MVP menyebut satu langkah yang belum pernah dijalankan siapa pun:

> Owner mendaftar → … → owner meng-assign → employee menerima notifikasi →
> **follow-up dari HP** → update → konversi ke Customer

Phase ini menutupnya, dan dengan itu melengkapi seluruh siklus produk — yang membuka GATE
(freeze: cari 3–5 pengguna nyata sebelum Phase 6).

## Yang sudah siap, dan yang belum

Riset sebelum menulis PRD ini menemukan backend **hampir seluruhnya sudah siap** — Phase 5 lebih
banyak pekerjaan Flutter daripada Go:

| Kebutuhan mobile | Status |
|---|---|
| Login jalur mobile (`client: "mobile"`, refresh token di body, TTL 2160h) | ✅ Phase 1 (#10) |
| Rotasi refresh token + deteksi penggunaan ulang | ✅ Phase 1 (#10) |
| Visibilitas Employee: hanya lead yang di-assign kepadanya | ✅ Phase 1 (#11) + Phase 2, ditegakkan di repository |
| Permission Employee (`lead.read/update`, `task.*`, `activity.*`, `customer.read`) | ✅ Phase 1 (#11) |
| Activity `call_logged`, `whatsapp_opened`, `note_added` sebagai tipe yang boleh dikirim klien | ✅ Phase 2 (#21) |
| Transisi status + optimistic locking `version` → `409` | ✅ Phase 2 (#20) |
| Tabel `notifications` + endpoint daftar/mark-read | ✅ Phase 2 (#22) |
| Pencabutan refresh token saat membership dinonaktifkan | ✅ Phase 2 (#22) |
| **`device_tokens` + pengiriman push FCM** | ❌ **belum ada — satu-satunya pekerjaan backend phase ini** |

Artinya: yang membuat phase ini besar bukan backend-nya, melainkan bahwa `crm_employee/` masih
berisi satu berkas `README.md`.

---

## Android dulu — iOS ditunda

**Keputusan pemilik produk:** Phase 5 menargetkan **Android saja**. iOS ditunda sampai Apple
Developer Program tersedia (berbayar ~$99/tahun, dan enrollment badan usaha butuh D-U-N-S yang
memakan waktu).

Ini **tidak** mengorbankan kriteria selesai phase. Freeze menulis *"Siklus penuh berjalan di HP
nyata"* — tanpa menyebut platform. Yang benar-benar terhalang tanpa akun Apple hanya dua hal, dan
keduanya iOS-spesifik:

| Terhalang | Kenapa |
|---|---|
| Build & distribusi iOS | Provisioning gratis hanya berlaku 7 hari dan tidak bisa dibagikan. Tidak ada TestFlight |
| Push iOS | FCM untuk iOS mengantar lewat **APNs**; APNs key hanya bisa dibuat dari akun Apple berbayar |

Push **Android** tidak terhalang sama sekali — FCM mengantar langsung ke perangkat Android tanpa
melibatkan Apple. Firebase project gratis dan dibuat dalam hitungan menit.

> **Yang dijaga sepanjang phase ini:** tidak ada kode yang ditulis dengan asumsi "hanya Android
> selamanya". Flutter lintas platform secara bawaan; penundaan ini soal **build, uji, dan rilis**,
> bukan soal menulis kode Android-only. Detail apa yang boleh dan tidak boleh: TD §2.

---

## Kontradiksi di dalam freeze — dilaporkan, bukan diputuskan diam-diam

Freeze menyebut cakupan offline **dua kali dengan makna berbeda**:

| Lokasi | Bunyi |
|---|---|
| Bagian 4, tabel Phase 5, baris *Cakupan* | "**cache baca offline**" |
| Bagian 4, tabel Phase 5, baris *Selesai bila* | "daftar lead tetap **terbaca** saat mode pesawat" |
| Bagian 2.3, penjelasan kolom `version` | "Konsekuensi langsung dari **antrian aksi offline di Phase 5**" |

Dua yang pertama menyebut **baca**; yang ketiga menyebut **antrian aksi** (yaitu tulis offline yang
disinkronkan belakangan). Keduanya tidak bisa benar bersamaan sebagai cakupan phase — antrian tulis
offline adalah pekerjaan berkali lipat lebih besar (penyimpanan antrian, urutan, retry, UI konflik).

**Interpretasi yang diambil: cache baca saja.** Alasannya:

1. **Baris *Selesai bila* adalah kriteria yang mengikat**, dan ia menyebut *terbaca* — bukan
   "perubahan tersinkron".
2. Baris *Cakupan* di tabel yang sama juga menulis "cache **baca** offline" secara eksplisit.
3. Bagian 2.3 sedang menjelaskan **kenapa kolom `version` dibangun di Phase 2**, bukan menetapkan
   cakupan Phase 5. Kalimat lengkapnya justru menegaskan kehati-hatian: *"yang mahal adalah membangun
   antrian offline dengan asumsi last-write-wins lalu menyadari datanya sudah hilang"* — nadanya
   memperingatkan, bukan memerintahkan membangunnya sekarang.

`version` **tetap dipakai** phase ini (setiap tulis dari mobile mengirimnya, `409` ditampilkan ke
pengguna) — jadi fondasi untuk antrian offline tetap utuh bila kelak dibangun. Yang ditunda hanya
antriannya.

**Ini dilaporkan sesuai Aturan #30, tidak diperbaiki diam-diam.** Freeze tidak diubah oleh phase ini.
Bila pemilik produk membaca bagian 2.3 dengan maksud berbeda, katakan sebelum implementasi dimulai —
itu mengubah ukuran phase ini secara mendasar.

---

## Kebutuhan

| # | Sebagai… | Saya butuh… | Supaya… |
|---|---|---|---|
| 1 | Employee lapangan | Membuka daftar lead saya di HP, bukan di laptop | Saya bekerja sambil berdiri di depan toko calon pelanggan, bukan di meja |
| 2 | Employee lapangan | Daftar lead saya tetap terbaca saat sinyal hilang | Basement mall dan pinggiran kota adalah kondisi kerja normal, bukan kasus tepi |
| 3 | Employee lapangan | Menelepon / WhatsApp calon pelanggan langsung dari aplikasi | Saya tidak perlu menyalin nomor ke aplikasi lain, dan riwayatnya tercatat sendiri |
| 4 | Employee lapangan | Mengubah status dan menambah catatan segera setelah bicara | Kalau harus menunggu sampai di kantor, saya tidak akan pernah melakukannya |
| 5 | Employee lapangan | Tahu ada lead baru untuk saya tanpa membuka aplikasi | Lead yang didiamkan dua jam sudah kalah oleh kompetitor |
| 6 | Employee lapangan | Masuk kembali tanpa mengetik password tiap kali | Aplikasi yang minta password 5×/hari akan ditinggalkan |
| 7 | Owner | Yakin employee yang sudah keluar tidak bisa lagi membuka daftar lead dari HP-nya | HP pribadi tidak bisa ditarik saat orangnya resign |
| 8 | Pemilik sistem | Kegagalan push tidak pernah menghilangkan notifikasi | Push adalah pengantar; catatannya yang jadi sumber kebenaran (freeze A3) |

---

## Acceptance Criteria

Phase 5 selesai bila **semuanya** terpenuhi:

| # | Kriteria |
|---|---|
| 1 | Siklus penuh berjalan di **HP Android sungguhan** (bukan hanya emulator): login → lihat lead yang ditugaskan → telepon/WhatsApp → ubah status → tambah catatan |
| 2 | Daftar lead **tetap terbaca dalam mode pesawat** setelah pernah dibuka sekali — kriteria *Selesai bila* freeze, diverifikasi dengan benar-benar mematikan koneksi |
| 3 | Employee **hanya** melihat lead yang ditugaskan kepadanya — dibuktikan dengan dua akun employee di organization yang sama |
| 4 | Login memakai **user session (JWT + refresh opaque)**, bukan API key — dan tidak ada satu pun jalur di aplikasi yang menyentuh API key (Aturan #24) |
| 5 | Refresh token disimpan di **secure storage** OS, bukan `SharedPreferences` — dan rotasinya bekerja (token lama ditolak setelah dipakai) |
| 6 | Membuka kembali aplikasi meminta **biometric**, bukan password — dan menolak masuk bila biometric gagal |
| 7 | Menonaktifkan employee dari dashboard membuat aplikasi di HP-nya **kehilangan akses pada refresh berikutnya** — diverifikasi langsung, bukan diasumsikan dari kode Phase 2 |
| 8 | Menelepon dan membuka WhatsApp masing-masing mencatat activity (`call_logged`, `whatsapp_opened`) yang muncul di timeline dashboard |
| 9 | Ubah status yang bentrok (data sudah diubah orang lain) menampilkan **konflik kepada pengguna**, tidak pernah menimpa diam-diam (`version` → `409`) |
| 10 | Assign lead dari dashboard memunculkan **push notification di HP Android** dalam hitungan detik, dan menekannya membuka lead itu (deeplink) |
| 11 | Kegagalan kirim push **tidak** membatalkan pembuatan record notification, dan tidak menggagalkan request assignment (Aturan #32) |
| 12 | Token perangkat yang sudah tidak valid (aplikasi di-uninstall) **dibersihkan**, tidak menumpuk selamanya |
| 13 | `flutter analyze` bersih dan test unit lolos di CI, memakai versi Flutter yang **dipin** — bukan versi apa pun yang kebetulan terpasang di mesin |
| 14 | Tidak ada rahasia (refresh token, service account FCM) yang ter-commit ke repository |

---

## Keputusan yang ditutup di phase ini

| # | Pertanyaan | Keputusan | Alasan |
|---|---|---|---|
| **M1** | Platform target Phase 5? | **Android saja.** iOS digenerate Flutter tapi tidak pernah dibangun, diuji, atau dikonfigurasi Firebase. | Apple Developer Program berbayar dan belum ada; enrollment badan usaha punya lead time berminggu. Yang terhalang hanya build & push iOS — bukan satu pun kriteria selesai phase ini. Kode tetap ditulis lintas platform (Flutter bawaan), jadi mengaktifkan iOS kelak = konfigurasi + build, bukan menulis ulang. |
| **M2** | Versi Flutter dipin atau ikut mesin? | **Dipin lewat FVM**, `3.44.0`, `.fvmrc` di-commit. | Mesin ini sudah punya dua versi berbeda (`flutter` sistem 3.38.9 vs FVM 3.44.0) — tanpa pin, dua mesin sudah tidak sepakat hari ini, apalagi CI. Pola yang sama seperti `go.mod` mengunci versi Go. |
| **M3** | Cakupan offline: baca saja atau antrian tulis? | **Cache baca saja.** Antrian aksi offline **di luar cakupan**. | Kontradiksi freeze di atas — kriteria *Selesai bila* menyebut *terbaca*. `version` tetap dikirim di setiap tulis sehingga fondasi antrian tetap utuh bila kelak dibangun. |
| **M4** | Pengiriman FCM: Firebase Admin SDK atau HTTP v1 langsung? | **HTTP v1 langsung**, token OAuth2 dari service account lewat `golang.org/x/oauth2/google`. | Admin SDK membawa Firestore, Auth, dan Storage yang tidak satu pun dipakai. Aturan #27 — pola yang sama seperti memilih `net/smtp` ketimbang pustaka mail di Phase 4.6. Satu dependency baru (`x/oauth2`), bukan satu pohon dependency. |
| **M5** | Kapan push dikirim relatif transaksi? | **Setelah commit**, tidak pernah di dalamnya. Kegagalan kirim hanya dicatat. | Aturan #32, sama persis seperti email sejak Phase 1. Freeze A3 sudah menetapkan record notification adalah **source of truth** dan push hanya pengantar *best-effort* — jadi push yang gagal memang tidak boleh punya konsekuensi apa pun terhadap data. |
| **M6** | Desain: brief dulu atau langsung bangun? | **Design brief ditulis lebih dulu**, hasil Claude Design ditunggu sebelum issue layar. Issue **fondasi desain dibuat sejak awal**, bukan menyusul. | Pola Phase 3, dengan satu koreksi dari pengalamannya: di sana token/kerangka aplikasi tidak dimiliki issue manapun sehingga #40 muncul di luar rencana di tengah phase. Kali ini fondasi punya issue sendiri sejak hari pertama. |

---

## Di luar cakupan

| Tidak dikerjakan | Ke |
|---|---|
| **iOS**: build, TestFlight, App Store, APNs | Menunggu Apple Developer Program (M1) |
| **Antrian aksi offline** (ubah status/catatan saat pesawat, disinkronkan belakangan) | Belum dijadwalkan (M3) — `version` sudah siap menerimanya |
| Dashboard Owner di mobile | Bukan alat Employee. Owner memakai `crm_dashboard` |
| Membuat lead baru dari mobile | Employee tidak punya `ActionLeadCreate` (Phase 1 #11). Lead lahir dari Owner atau API publik |
| Konversi ke Customer dari mobile | `ActionLeadConvert` sengaja tidak dimiliki Employee |
| Mengelola tim / API key dari mobile | Owner/Admin saja, dan itu pekerjaan dashboard |
| Chat in-app, panggilan VoIP | Di luar scope produk (`CLAUDE.md`) — aplikasi hanya **membuka** dialer & WhatsApp milik OS |
| Peta / geolokasi kunjungan | Belum pernah diminta. Aturan #28 |
| Play Store release, signing key produksi | Phase ini selesai di **APK/AAB debug di HP nyata**. Rilis publik butuh keputusan branding & akun Play yang belum ada |
| Notifikasi selain `lead_assigned` & `task_assigned` | Hanya dua tipe itu yang ada di `ck_notifications_type` |

---

## Dependensi

| Bergantung pada | Sifat |
|---|---|
| Phase 1 (#10, #11) | **Keras.** Seluruh jalur auth mobile dan visibilitas Employee lahir di sana |
| Phase 2 (#19–#23) | **Keras.** Lead, task, activity, notification, `version` — tidak ada yang perlu ditambah |
| Phase 3 | **Praktis.** Owner butuh jalan untuk assign lead supaya ada yang muncul di aplikasi |
| Phase 4 | **Bukan prasyarat** (freeze) — mobile tidak menyentuh API key sama sekali |
| Firebase project | **Memblokir issue push saja**, bukan phase-nya. Yang dibutuhkan pemilik produk ada di TD §9 |
| Apple Developer Program | **Tidak memblokir apa pun di phase ini** (M1) |
| Hasil Claude Design | **Memblokir issue layar**, bukan issue backend & fondasi Flutter (M6) |
