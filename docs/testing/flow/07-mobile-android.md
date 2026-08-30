# 7 — Mobile Android

Prasyarat: [`00-menjalankan-aplikasi.md`](./00-menjalankan-aplikasi.md) selesai (backend hidup),
[`02-tim-dan-undangan.md`](./02-tim-dan-undangan.md) selesai (employee sudah diundang & aktif),
[`03-lead-dan-pipeline.md`](./03-lead-dan-pipeline.md) selesai (ada lead untuk ditugaskan).

Menguji: **kalimat inti MVP secara utuh untuk pertama kalinya** — bagian "follow-up dari HP" yang
sebelumnya dilakukan lewat dashboard (lihat `README.md`'s catatan) sekarang benar-benar di HP.
Kriteria acceptance Phase 5 #1/#2/#6/#7/#10/#12 **hanya** bisa dibuktikan di sini — tidak ada test
otomatis yang menggantikannya (`docs/phases/05-employee-mobile/td.md` §12.1).

**Butuh perangkat Android fisik** (bukan emulator saja) — mode pesawat, notifikasi push sungguhan,
dan uninstall aplikasi tidak bisa dibuktikan jujur di emulator.

## 7.1 Siapkan Firebase & build APK

Firebase project `jualin-crm` sudah ada, tapi `google-services.json`/`lib/firebase_options.dart`
di-gitignore (`docs/phases/05-employee-mobile/td.md` §14) — belum tentu ada di mesin Anda.

1. `cd crm_employee`
2. `flutterfire configure --project=jualin-crm --platforms=android` — bila belum pernah dijalankan
   di mesin ini. Menghasilkan `android/app/google-services.json` + `lib/firebase_options.dart`
   sungguhan.
3. **Backend**: `PUSH_PROVIDER=fcm`, `FCM_PROJECT_ID`, `FCM_CREDENTIALS_FILE` (service account JSON,
   Project settings → Service accounts → Generate new private key, **simpan di luar repository**) —
   `.env.example` (akar) punya template lengkap. Restart `crm_be` setelah mengisi ini.
4. `fvm flutter build apk --debug`
5. Sambungkan HP lewat USB (Developer options + USB debugging menyala), lalu:
   ```
   adb reverse tcp:8080 tcp:8080
   ```
   HP fisik tidak bisa mencapai `localhost`/`10.0.2.2` mesin dev — `adb reverse` membuat port 8080
   di HP meneruskan ke port 8080 mesin dev lewat kabel USB, jadi `API_BASE_URL` bawaan
   (`http://10.0.2.2:8080` — sebenarnya alamat emulator) **tidak dipakai** dan tidak perlu diganti
   selama USB tetap tersambung. (Alternatif tanpa kabel: `--dart-define=API_BASE_URL=http://<ip-lan-mesin-dev>:8080` saat build, HP & mesin dev di jaringan Wi-Fi yang sama.)
6. `adb install build/app/outputs/flutter-apk/app-debug.apk`

## 7.2 Login + biometric (kriteria #4, #5, #6)

1. Buka aplikasi di HP. Login sebagai employee yang diundang di
   [`02-tim-dan-undangan.md`](./02-tim-dan-undangan.md).

**Hasil yang diharapkan:** masuk ke Lead Saya. **Tidak ada** field API key di mana pun — hanya
email/password, lalu biometric.

2. Tutup aplikasi sepenuhnya (bukan sekadar pindah app), buka lagi.

**Hasil yang diharapkan:** diminta biometric (sidik jari/wajah), **bukan** password. Batalkan
prompt biometric sekali — aplikasi harus tetap di layar kunci, tidak masuk begitu saja.

## 7.3 Mode pesawat — daftar lead tetap terbaca (kriteria #2)

1. Dengan Lead Saya sudah pernah terbuka sekali (sinyal masih ada), aktifkan **mode pesawat** penuh
   di HP.
2. Tutup aplikasi, buka lagi (masih lewat biometric).

**Hasil yang diharapkan:** daftar lead **tetap muncul**, dengan pita "Data dari cache · diperbarui
HH:mm". Bukan layar error, bukan daftar kosong. Matikan mode pesawat setelah langkah ini selesai.

## 7.4 Follow-up dari HP — siklus penuh (kriteria #1, #8, #9)

Dari dashboard (mesin dev), **assign** salah satu lead dari [`03-lead-dan-pipeline.md`](./03-lead-dan-pipeline.md) ke employee ini kalau belum.

1. Di HP, buka lead itu dari Lead Saya.
2. Tekan **Telepon** — dialer OS terbuka. **Batalkan** panggilan itu (jangan benar-benar menelepon).

**Hasil yang diharapkan:** kembali ke aplikasi, **tidak ada** entri baru di timeline — activity hanya
dicatat setelah dialer benar-benar terbuka, bukan saat tombol ditekan, dan bukan sesuatu yang terjadi
di dalam aplikasi (§8.3 design brief), jadi pastikan juga langkah 3 di bawah untuk yang benar-benar
tercatat.

3. Tekan **Telepon** lagi, kali ini biarkan dialer benar-benar terbuka (tidak perlu benar-benar
   menelepon — cukup sampai layar dialer OS muncul), lalu kembali ke aplikasi.

**Hasil yang diharapkan:** entri `call_logged` muncul di timeline HP. **Buka dashboard di mesin dev**
dan lihat lead yang sama — entri yang sama harus muncul di sana juga (acceptance criterion eksplisit:
"diverifikasi dari sisi Owner, bukan hanya dari mobile").

4. Tekan **WhatsApp** (hanya aktif bila lead punya nomor valid), biarkan WhatsApp benar-benar
   terbuka, kembali ke aplikasi.

**Hasil yang diharapkan:** entri `whatsapp_opened` muncul di timeline HP **dan** dashboard.

5. Ubah status lead (mis. Baru → Dihubungi). Tambah satu catatan.

**Hasil yang diharapkan:** status berubah, catatan muncul di timeline — keduanya juga terlihat dari
dashboard.

6. **Konflik**: buka lead yang sama di dashboard (tab lain) DAN di HP secara bersamaan. Ubah status
   dari dashboard dulu, baru dari HP (tanpa refresh HP-nya lebih dulu).

**Hasil yang diharapkan:** HP menampilkan dialog konflik ("data sudah diubah") dengan tombol "muat
ulang" — **tidak pernah** menimpa diam-diam perubahan dari dashboard.

## 7.5 Tugas Saya

1. Dari dashboard, buat task untuk lead ini, tugaskan ke employee ini.
2. Di HP, buka tab **Tugas Saya**.

**Hasil yang diharapkan:** task muncul, dengan jatuh tempo bila diisi.

3. Tandai selesai (checkbox).

**Hasil yang diharapkan:** task hilang dari daftar (tampilan default hanya task terbuka). **Tidak ada**
cara membukanya kembali dari UI — coba cari, memang tidak ada (satu arah, by design).

## 7.6 Push notification + deeplink — tiga keadaan (kriteria #10)

Dari dashboard, assign lead **berbeda** (belum pernah di-assign ke employee ini sebelumnya) ke
employee ini, untuk masing-masing dari tiga langkah berikut (assign ulang lead yang sama juga boleh,
push tetap terkirim setiap assignment).

1. **Aplikasi terbuka (foreground)** — buka aplikasi di HP, assign lead dari dashboard.

**Hasil yang diharapkan:** pita notifikasi in-app muncul **di dalam** aplikasi, dalam hitungan detik.
**Tidak** berpindah layar sendiri. Tekan pita itu — baru berpindah ke lead terkait.

2. **Aplikasi di latar belakang** — buka aplikasi lalu tekan tombol Home (jangan ditutup paksa),
   assign lead dari dashboard.

**Hasil yang diharapkan:** notifikasi sistem Android muncul di tray. Tekan notifikasinya —
aplikasi terbuka **langsung ke lead yang di-assign**, bukan ke Lead Saya biasa.

3. **Aplikasi mati total** — tutup paksa aplikasi (swipe dari recent apps), assign lead dari
   dashboard.

**Hasil yang diharapkan:** notifikasi tray muncul. Tekan — aplikasi terbuka (lewat layar
kunci/biometric seperti biasa), **setelah** sesi siap langsung menuju lead yang di-assign, bukan
diam di Lead Saya.

4. **Kasus wajib — ditekan saat belum login:** logout dari aplikasi dulu. Assign lead lain dari
   dashboard, tutup paksa aplikasi, lalu tekan notifikasi tray-nya.

**Hasil yang diharapkan:** diminta login/biometric seperti biasa — **setelah** berhasil masuk,
aplikasi membuka lead yang benar, bukan cuma Lead Saya kosong (tujuan tersimpan, bukan hilang diam-diam).

## 7.7 Kehilangan akses mendadak (kriteria #7)

1. Dengan employee masih login di HP, **nonaktifkan** membership-nya dari dashboard (Tim → nonaktifkan).
2. Di HP, lakukan aksi apa pun yang memanggil API (buka tab lain, tarik untuk refresh).

**Hasil yang diharapkan:** aplikasi menampilkan layar "Sesi Berakhir" pada panggilan API berikutnya —
**bukan** kerusakan/crash, dan **bukan** tetap terlihat seolah masih punya akses. Diverifikasi
langsung di HP, bukan disimpulkan dari kode Phase 2.

## 7.8 Logout & uninstall — token dibersihkan (kriteria #12)

1. Logout dari aplikasi (lewat avatar → Keluar).

**Hasil yang diharapkan:** kembali ke layar login. Assign lead lain ke employee ini dari dashboard
setelah logout — **tidak ada** push yang sampai (token sudah dihapus dari backend saat logout).

2. Login lagi, biarkan token FCM terdaftar, lalu **uninstall aplikasi** dari HP sepenuhnya.
3. Assign lead lain ke employee ini dari dashboard — ini akan gagal terkirim (token sudah mati),
   ditangkap oleh `internal/shared/push`'s penanganan `UNREGISTERED` (#68).

**Hasil yang diharapkan:** tidak ada cara memverifikasi ini dari UI (aplikasinya sudah tidak ada) —
periksa **tabel `device_tokens`** langsung (`docker compose exec postgres psql -U jualin -d
jualin_crm -c "SELECT * FROM device_tokens WHERE membership_id = '<id>';"`): baris untuk employee ini
harus **hilang** setelah percobaan kirim push berikutnya, bukan menumpuk selamanya.

---

Selesai — kalimat inti MVP sudah terbukti berjalan ujung ke ujung di perangkat sungguhan. Lanjut ke
[`06-checklist-akhir.md`](./06-checklist-akhir.md) untuk rekap.
