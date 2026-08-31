# Issue #69 — checklist penutupan phase

> Checklist ringkas, **bukan** catatan status. Status pekerjaan tetap hidup di GitHub Issues
> (ADR-008) — berkas ini hanya mengumpulkan poin yang perlu **dicek ulang saat #73** (penutup Phase 5)
> menutup phase-nya, supaya tidak ada yang terlewat di antara PR-PR yang sudah lama merge. Detail lengkap
> tiap poin ada di `docs/phases/05-employee-mobile/notes.md` bagian `## #69`.

## Keputusan yang perlu dicek ulang

- [ ] **Login mobile tidak menangani `409 organization_selection_required` sama sekali** — kegagalan
      login untuk user multi-organization akan tampil sebagai pesan error mentah dari `ApiErrorBody`
      ("Pilih organization untuk melanjutkan."), bukan halaman pemilih organization seperti
      `crm_dashboard`. Ini **bukan** kasus hipotetis: ADR-007 membolehkan satu `users` row punya banyak
      `memberships`, dan alur terima undangan (#11, #34) membolehkan user yang sudah ada diundang ke
      organization kedua — jadi seorang Employee **bisa** secara sah multi-organization. Diputuskan
      sengaja tidak dibangun di #69 karena TD tidak memintanya dan kasusnya jarang untuk akun Employee
      spesifiknya (biasanya satu pekerjaan, satu organization) — ~~tapi harus ditinjau ulang saat #73:
      apakah kasus ini pernah benar-benar terjadi di penggunaan nyata, dan kalau ya, apakah pesan error
      mentah itu cukup atau perlu picker sungguhan.~~
      **Ditinjau di #98 — dinaikkan dari "keputusan UX" jadi CACAT FUNGSIONAL.** `grep` seluruh
      `crm_employee/lib/` menemukan **nol** kemunculan `organization_selection_required` — bukan
      "ditangani seadanya", melainkan tidak ditangani sama sekali. Akibatnya bukan pesan yang kurang
      rapi: Employee multi-organization **tidak bisa login sama sekali** ke aplikasi mobile, karena
      tidak ada jalan apa pun untuk memilih organization. Aplikasi ini satu-satunya alat follow-up di
      lapangan, jadi kelasnya bukan kosmetik.
      Framing awal ("tinjau ulang bila terasa janggal") **salah menilai dampaknya** — ia menganggap
      hasilnya pesan yang jelek, padahal hasilnya pintu terkunci. Perlu issue tersendiri; tidak
      diperbaiki di #98 (issue ini memutuskan, bukan mengimplementasi).
- [ ] **"Masuk dengan password" (fallback biometric gagal) memakai ulang `LoginPage` penuh**
      (email+password), bukan layar "konfirmasi password" yang lebih ringan untuk sesi yang tokennya
      sebenarnya masih hidup. Sengaja disederhanakan (TD tidak merinci UI-nya) — tinjau ulang bila
      terasa janggal secara UX setelah dipakai sungguhan di HP.

## Implementasi non-trivial yang tidak disebut TD secara eksplisit

Bukan deviasi — kebutuhan nyata `package:local_auth` di Android yang baru ketahuan saat build
sungguhan dicoba, dicatat di sini supaya tidak perlu ditemukan ulang:

- `MainActivity.kt` harus `FlutterFragmentActivity`, bukan `FlutterActivity` — `local_auth`'s
  `BiometricPrompt` butuh `FragmentActivity`.
- `AndroidManifest.xml` butuh `<uses-permission android:name="android.permission.USE_BIOMETRIC"/>`.
- `LaunchTheme` (`values/styles.xml` dan `values-night/styles.xml`) harus berparent
  `Theme.AppCompat.DayNight.NoActionBar` — tema Android polos bawaan `flutter create` membuat dialog
  biometric crash di Android 8 ke bawah.
- `compileSdk` dinaikkan ke **37** secara eksplisit (`flutter.compileSdkVersion` bawaan Flutter 3.44.0
  masih 36) — `flutter_secure_storage` mensyaratkan 37, backward compatible sesuai peringatan build
  Flutter sendiri.

---

**Ditinjau 31 Agustus 2026 di #98.**

**Koreksi terhadap dugaan awal #98 sendiri:** pemeriksaan ini semula menyimpulkan berkas ini "tidak
pernah dibaca sama sekali" saat Phase 5 ditutup. **Itu terlalu keras.** `docs/STATUS.md` bagian *Utang
Teknis* membuktikan #73 **memang** meninjau poin pertama di bawah — lengkap dengan `grep` sendiri —
dan membawanya ke sana. Yang benar: **tidak ada peninjauan sistematis** atas berkas ini (tidak ada
langkah "cek `docs/issues/*`" di `issues.md` Phase 5, dan berkas ini tidak pernah ditutup), tapi satu
poin kebetulan terbawa lewat jalur lain. Dicatat karena membedakan "tidak ada prosesnya" dari "tidak
ada yang peduli" itu penting — yang pertama benar, yang kedua tidak.

**Hasil:**

- Poin pertama **dinaikkan jadi cacat fungsional**, dan yang berubah bukan faktanya (#73 sudah tahu)
  melainkan **penilaian dampaknya**: baik berkas ini maupun baris Utang Teknis sama-sama menunggu
  "skenario multi-organization sungguhan muncul", padahal orang yang terkena **tidak bisa login sama
  sekali** — ia tidak akan pernah muncul sebagai keluhan tentang fitur ini, hanya sebagai "aplikasinya
  error". Pemicu tunggu itu tidak akan pernah terpicu. Layak jadi issue tersendiri; tidak diperbaiki
  di #98 (issue ini memutuskan, bukan mengimplementasi).
  **Diputuskan pemilik produk 31 Agustus 2026 (saat Phase 7 dibuka): digabung ke sesi verifikasi HP
  Android**, bukan issue terpisah sekarang. Alasannya kuat — perbaikannya butuh layar pemilih
  organization di Flutter, dan itu **tidak bisa diverifikasi tanpa perangkat fisik**, persis seperti
  lima AC Phase 5 yang sudah menunggu di `073`. Satu sesi dengan HP di tangan menyelesaikan keduanya;
  dua sesi terpisah berarti yang satu tetap menunggu perangkat yang sama. Pemicunya sekarang **konkret
  dan pasti terjadi** (sesi verifikasi HP), bukan "sampai skenarionya muncul" yang tidak akan pernah.
- Poin kedua (fallback biometric memakai `LoginPage` penuh) **tetap terbuka** — murni UX, pemicunya
  pemakaian nyata di HP, yang juga belum terjadi.
