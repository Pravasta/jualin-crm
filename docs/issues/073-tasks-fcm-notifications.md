# Issue #73 — checklist penutup Phase 5

> Checklist ringkas, **bukan** catatan status. Status pekerjaan tetap hidup di GitHub Issues
> (ADR-008) — berkas ini mengumpulkan poin yang perlu **dicek ulang** saat Phase 6 dimulai, atau
> saat verifikasi HP akhirnya dijalankan. Detail lengkap tiap poin ada di
> `docs/phases/05-employee-mobile/notes.md` bagian `## #73`.

## Belum diverifikasi — butuh HP Android sungguhan

- [ ] **AC #1** — siklus penuh (login → lead → telepon/WA → ubah status → catatan) di perangkat fisik
- [ ] **AC #6** — biometric buka kembali, menolak masuk bila gagal
- [ ] **AC #7** — nonaktifkan employee dari dashboard → aplikasi kehilangan akses pada refresh
      berikutnya, di HP nyata
- [ ] **AC #10** — assign lead → push muncul di HP dalam hitungan detik → tekan → lead terbuka,
      termasuk kasus "ditekan saat belum login"
- [ ] **AC #12** — uninstall aplikasi → `device_tokens` dibersihkan pada percobaan kirim berikutnya

Prosedur lengkap: `docs/testing/flow/07-mobile-android.md`. Butuh `FCM_CREDENTIALS_FILE` (service
account, sengaja belum dibuat sesi ini — `td.md` §14 langkah 3) diisi di backend + `PUSH_PROVIDER=fcm`
sebelum push bisa diuji sungguhan.

## Keputusan yang perlu dicek ulang

- [ ] **`assigned_to` dikirim eksplisit di `GET /v1/tasks`** — berbeda dari `leads` yang sengaja tidak
      pernah mengirimnya. Backend Employee hanya membatasi task ke **lead** yang di-assign kepadanya,
      bukan task yang **assigned-to** dirinya sendiri — dua hal berbeda (`buildTaskWhere`,
      `repository_postgres.go`). Tinjau ulang bila kelak ada kebutuhan menampilkan task di lead saya
      yang assigned ke orang lain (bukan cakupan #73).
- [ ] **`/v1/tasks` diurutkan ulang di klien** (ascending `due_at`, null terakhir) — backend hanya
      punya `created_at DESC`. Aman untuk satu halaman tak berpaginasi; tinjau ulang bila paginasi
      akhirnya dibangun (M3-adjacent, belum ada rencana).
- [ ] **Tap baris task membuka lead terkait** — bukan literal di design brief §7.4, ditambahkan sadar
      karena murah (memakai ulang `openLeadDetail`) dan konsisten dengan misi produk. Tinjau ulang bila
      terasa mengejutkan bagi pengguna nyata.
- [ ] **`/v1/notifications` tanpa cache offline** — TD §7 tidak menyebutnya sebagai salah satu dari
      empat endpoint cacheable, design brief §10 juga tidak minta pita cache untuk Notifikasi. Tinjau
      ulang bila pengguna sering membuka Notifikasi tanpa sinyal.
- [ ] **`markRead` optimistik, bukan refetch** — beda dari `TasksBloc`'s 409 yang refetch penuh.
      Kegagalan jaringan meninggalkan baris tampak terbaca padahal server belum tahu — dianggap
      cukup rendah risiko untuk dibiarkan begitu (reload berikutnya memperbaikinya sendiri).

## Temuan — dicatat untuk arsip, sudah selesai

- **`firebase_options.dart.example`/`google-services.json.example` + step CI baru** — `firebase_
  options.dart` di-gitignore tapi `import`-nya wajib resolve untuk `flutter analyze`/`test` — tanpa
  ini, CI merah untuk **setiap** PR `crm_employee/**` berikutnya, bukan cuma PR ini. Ditemukan &
  diperbaiki sebelum PR dibuka.
- **`core/push/` (`DeviceTokenRemoteDataSource`, `PushTokenStore`)** — dipindah keluar dari
  `features/push/` supaya `AuthRepositoryImpl.logout()` bisa memanggil unregister device token tanpa
  `auth` mengimpor fitur lain. Dibuktikan sungguhan terhadap `crm_be` nyata: percobaan unregister
  kedua atas token yang sama, setelah `logout()`, mengembalikan `404` — baris memang terhapus di
  server, bukan cuma dilupakan lokal.
- **`openLeadDetail` diekstrak** dari `leads_page.dart`'s `_openLeadDetail` privat — implementasi
  ketiga/keempat yang nyata (Tugas Saya, Notifikasi, deeplink push), Aturan #28.
- **`ForegroundPushBanner` dijadikan publik** — supaya bisa diuji langsung tanpa membangun `AppShell`
  penuh (empat bloc + DI) hanya untuk satu widget.
- **`PlaceholderScreen` dihapus** — pemakai terakhirnya (Tugas Saya, Notifikasi) sudah diganti layar
  sungguhan.
- **`api.md` sengaja tidak diubah** — TD §15 minta "endpoint device-tokens di daftar endpoint", tapi
  berkas itu tidak punya daftar endpoint literal (isinya konvensi, bukan katalog rute). Dicatat sebagai
  penyimpangan dari TD yang dilaporkan, bukan dipaksa cocok.

---

**Belum ditinjau** — menunggu verifikasi HP Android dari pemilik produk, dan keputusan Phase 6.
