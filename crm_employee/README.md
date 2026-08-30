# crm_employee

Aplikasi mobile untuk Employee. **Flutter**, dipin lewat FVM ke `3.44.0` (`.fvmrc`).

Cakupan: login + biometric · My Leads · My Tasks · lead detail · Call/WhatsApp dengan auto-Activity · ubah status · catatan · push notification · cache baca offline.

**Autentikasi memakai user session (JWT + refresh token opaque), bukan API key** — Aturan #24, lihat `docs/architecture/freeze.md` bagian 5.1.

**Android saja untuk sekarang** — iOS digenerate `flutter create` tapi tidak pernah dibangun atau diuji (keputusan M1, `docs/phases/05-employee-mobile/td.md` §1).

Perintah lewat `Makefile` di akar repo (`make mobile-get`, `make mobile-analyze`, `make mobile-test`, `make mobile-run`), atau `fvm flutter …` langsung di direktori ini.

## Push notification (FCM)

`android/app/google-services.json` dan `lib/firebase_options.dart` **tidak di-commit** (di-gitignore,
lihat `docs/phases/05-employee-mobile/td.md` §14) — repo tidak terikat ke satu project Firebase, dan
supaya tidak ada kebiasaan menaruh berkas Firebase apa pun di repo. `.example` untuk keduanya ada
sebagai referensi bentuk dan agar `flutter analyze`/`flutter test` tetap lolos di checkout bersih
(dipakai CI, lihat `.github/workflows/ci-employee.yml`) — **bukan** kredensial sungguhan.

Untuk build/run yang sungguhan bicara ke Firebase (push benar-benar sampai):

```
fvm dart pub global activate flutterfire_cli   # sekali saja per mesin
flutterfire configure --project=jualin-crm --platforms=android
```

Menghasilkan kedua berkas asli. Langkah lengkap (termasuk service account untuk backend) ada di
`docs/phases/05-employee-mobile/td.md` §14. Tanpa langkah ini, aplikasi tetap bisa dibangun dan
dijalankan — hanya push yang tidak akan pernah sampai (backend `PUSH_PROVIDER=none` bawaan juga
membuat sisi server tidak mencoba mengirim apa pun).

Menjalankan di HP fisik (bukan emulator) butuh satu langkah tambahan — HP tidak bisa mencapai
`localhost` mesin dev. Lihat `docs/testing/flow/07-mobile-android.md` §7.1.

Lihat `docs/architecture/freeze.md` bagian 4 dan `docs/phases/05-employee-mobile/` untuk detail.
