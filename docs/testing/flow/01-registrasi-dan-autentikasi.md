# 1 — Registrasi & Autentikasi

Prasyarat: [`00-menjalankan-aplikasi.md`](./00-menjalankan-aplikasi.md) selesai — backend dan
dashboard menyala, `http://localhost:3000` mengarahkan ke `/login`.

Menguji: registrasi organization baru, verifikasi email (lewat Mailpit — email sungguhan terkirim,
lihat [`00-menjalankan-aplikasi.md`](./00-menjalankan-aplikasi.md) §7), login, logout, lupa password,
reset password. Ini membangun **akun Owner** yang dipakai di seluruh berkas berikutnya — jangan hapus
datanya sampai seluruh urutan panduan ini selesai.

## 1.1 Registrasi organization baru

1. Dari `/login`, klik tautan **Daftar** (atau buka langsung `http://localhost:3000/register`).
2. Isi form:
   - **Nama organization**: `Toko Testing`
   - **Nama lengkap**: `Budi Owner`
   - **Email**: `owner@test.local`
   - **Password**: `correct horse battery staple` (atau apa pun ≥ panjang minimum — kalau terlalu
     pendek, form akan menolak sebelum submit)
3. Klik **Daftar**.

**Hasil yang diharapkan:** form berganti jadi pesan *"Kami mengirim tautan verifikasi ke
owner@test.local..."* — **bukan** langsung masuk ke dashboard. Ini sengaja (keputusan B3: verifikasi
email menggerbangi login).

## 1.2 Buka email verifikasi di Mailpit

1. Buka **http://localhost:8025**.
2. Klik email terbaru — subjeknya **"Verifikasi email Jualin CRM Anda"**, ditujukan ke
   `owner@test.local`.

**Hasil yang diharapkan:** isi email berbentuk kalimat lengkap Bahasa Indonesia dengan satu tautan
`http://localhost:3000/verify-email?token=...` — utuh, tidak terpotong.

## 1.3 Verifikasi email

1. Klik tautannya (kalau tampil sebagai tautan yang bisa diklik di tampilan Mailpit), atau sorot dan
   salin teksnya lalu tempel ke address bar browser.

**Hasil yang diharapkan:** halaman menampilkan status berhasil terverifikasi, dengan tautan/tombol
menuju `/login`.

2. Coba buka **URL yang sama sekali lagi** (token sudah terpakai).

**Hasil yang diharapkan:** pesan bahwa token tidak valid / sudah kedaluwarsa — bukan halaman kosong
dan bukan error 500. (Backend menjawab `400 invalid_token` di sini; yang perlu dinilai adalah apakah
tampilannya di layar masuk akal bagi orang awam.)

## 1.4 Login

1. Buka `/login`.
2. Email: `owner@test.local`, Password: sesuai yang didaftarkan.
3. Klik **Masuk**.

**Hasil yang diharapkan:** masuk ke **Beranda** (`/`) — kartu-kartu metrik (Lead baru, per status,
dst.) semuanya kosong/nol, karena belum ada lead sama sekali. Menu kiri/atas menampilkan: Beranda,
Lead, Customer, Tugas, Tim, Pengaturan.

### Uji negatif — jangan lewati

- **Password salah**: coba login dengan password yang salah. Harus muncul pesan error yang **tidak**
  membedakan "email tidak ada" dari "password salah" (mis. "Email atau password salah.") — bukan
  detail teknis, bukan status 500.
- **Login berkali-kali dengan password salah** (5–6 kali berturut dalam beberapa detik): percobaan
  berikutnya harus mulai ditolak dengan pesan "terlalu banyak percobaan" (backoff progresif,
  `auth.LoginLimiter`) — bukan terus mencoba tanpa batas.

## 1.5 Logout

Klik ikon **Keluar** (biasanya di pojok header/sidebar app shell).

**Hasil yang diharapkan:** kembali ke `/login`. Coba buka `http://localhost:3000/` langsung — harus
diarahkan balik ke `/login`, **tidak** menampilkan Beranda sesaat (tidak ada sesi yang bocor).

## 1.6 Lupa password

1. Dari `/login`, klik **Lupa password**.
2. Masukkan `owner@test.local`, submit.

**Hasil yang diharapkan:** pesan generik seperti "kalau email terdaftar, kami mengirim tautan" —
**sama persis** baik email-nya terdaftar maupun tidak (anti-enumeration, TD §6.3). Coba juga dengan
email yang **tidak pernah didaftarkan** (mis. `tidak-ada@test.local`) — pesannya harus identik,
supaya orang luar tidak bisa menebak email mana yang punya akun.

3. Buka **http://localhost:8025**, klik email terbaru — subjeknya **"Reset password Jualin CRM
   Anda"** (pola sama seperti §1.2).
4. Buka tautannya. Isi **Password baru** + **Konfirmasi password**, submit.

**Hasil yang diharapkan:** berhasil, diarahkan ke `/login`.

5. Login dengan password **baru** — harus berhasil.
6. Coba login dengan password **lama** — harus **gagal** (password lama tidak lagi berlaku).

---

Selesai di sini: satu akun Owner (`owner@test.local`) terverifikasi dan bisa login dengan password
yang sudah diganti. Lanjut ke [`02-tim-dan-undangan.md`](./02-tim-dan-undangan.md).
