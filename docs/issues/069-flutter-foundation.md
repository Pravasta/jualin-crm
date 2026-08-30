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
      spesifiknya (biasanya satu pekerjaan, satu organization) — tapi harus ditinjau ulang saat #73:
      apakah kasus ini pernah benar-benar terjadi di penggunaan nyata, dan kalau ya, apakah pesan error
      mentah itu cukup atau perlu picker sungguhan.
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

**Belum ditinjau** — menunggu penutupan Phase 5 (#73).
