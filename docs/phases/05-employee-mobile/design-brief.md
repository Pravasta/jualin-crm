# Design Brief — Jualin CRM Employee Mobile

> **Dokumen ini untuk desainer, bukan untuk implementor.**
> Implementasi mengikuti [`td.md`](./td.md); apa & kenapa ada di [`prd.md`](./prd.md).

---

## 1. Yang diminta

Desain aplikasi **Android** untuk employee lapangan (sales) sebuah CRM. Aplikasi Flutter,
**Android saja** untuk sekarang — iOS ditunda, tapi desain sebaiknya tidak memakai pola yang mustahil
dipindahkan ke iOS kelak.

Bukan mendesain ulang produk. Backend, aturan bisnis, istilah, dan alur sudah dikunci dan **sudah
berjalan** — dashboard web-nya sudah selesai dan dipakai. Yang diminta: bagaimana pekerjaan yang sama
terlihat dan terasa di HP, di tangan orang yang sedang berdiri di luar.

---

## 2. Produk dalam satu paragraf

Jualin CRM membantu UKM Indonesia mengelola calon pelanggan (**Lead**) dari masuk sampai menang.
Owner menerima lead, menugaskannya ke sales, lalu memantau. **Aplikasi ini milik sales-nya** — bukan
versi kecil dashboard Owner. Sales hanya melihat lead yang ditugaskan kepadanya, menelepon atau
WhatsApp calon pelanggan, mengubah status, dan menulis catatan. Ia tidak membuat lead, tidak
mengelola tim, tidak melihat laporan.

---

## 3. Pengguna & konteks pemakaian

**Ini bagian paling menentukan di brief ini.** Konteksnya berbeda jauh dari dashboard.

| Aspek | Kenyataan |
|---|---|
| Siapa | Sales lapangan UKM Indonesia. Bukan pekerja kantoran, bukan pengguna teknis |
| Perangkat | Android kelas menengah-bawah. Layar 6"-an, bukan flagship |
| Posisi tubuh | **Berdiri, satu tangan**, sering sambil berjalan. Ibu jari satu-satunya alat input |
| Cahaya | **Luar ruangan, matahari Indonesia.** Kontras rendah tidak terbaca sama sekali |
| Sinyal | **Buruk atau hilang** — basement mall, ruko pinggiran, area industri. Ini kondisi normal |
| Durasi sesi | 10–40 detik. Dibuka **di antara percakapan**, bukan untuk ditekuni |
| Momen pemakaian | Tepat sebelum menelepon, dan tepat sesudahnya |

Konsekuensi yang mengikat desain:

1. **Target sentuh besar dan di jangkauan ibu jari.** Aksi utama di paruh bawah layar, bukan pojok
   atas.
2. **Kontras tinggi.** Teks abu-abu tipis yang cantik di monitor tidak terbaca di parkiran jam 1 siang.
3. **Keadaan offline bukan kasus tepi** — ia harus punya desain, bukan pesan error generik.
4. **Sedikit langkah.** "Buka aplikasi → lihat lead → tekan telepon" harus terasa instan.

---

## 4. Yang sudah dikunci — jangan didesain ulang

### 4.1 Identitas visual dari dashboard

Dashboard sudah punya token warna yang **sudah diperbaiki kontrasnya** (semua ≥4.5:1). Mobile
sebaiknya terasa satu keluarga, tapi **tidak** wajib identik — konteks matahari boleh menuntut kontras
lebih tinggi.

| Token | Nilai | Guna |
|---|---|---|
| `--primary` | `oklch(0.56 0.19 41)` | Latar tombol dengan teks putih (4.83:1) |
| `--accent-strong` | `oklch(0.48 0.17 41)` | Warna **di atas** putih — teks/ikon beraksen (7.04:1) |
| `--foreground` | `oklch(0.145 0 0)` | Teks utama |
| `--muted-foreground` | `oklch(0.556 0 0)` | Teks sekunder |
| `--border` | `oklch(0.922 0 0)` | Garis pemisah |

> **Jangan pakai `--primary` untuk teks di atas putih** — di 0.56 ia gagal AA. Itu sebabnya ada dua
> token aksen terpisah.

**Setiap warna teks yang Anda usulkan harus ≥4.5:1 terhadap latarnya.** Di Phase 3, lima dari delapan
badge status hasil desain gagal ambang ini dan harus diperbaiki implementor — pekerjaan dua kali yang
bisa dihindari.

### 4.2 Yang tidak boleh diubah

- **Aturan transisi status** (§9.1) — ditegakkan backend, UI hanya boleh menawarkan yang sah
- **Istilah** (§6) — sudah dikunci glosarium produk
- **Bahasa Indonesia** untuk seluruh teks antarmuka
- **Employee hanya melihat lead miliknya** — ini aturan keamanan backend, bukan pilihan tampilan

---

## 5. Prinsip desain untuk produk ini

1. **Kecepatan mengalahkan kelengkapan.** Layar yang memuat 3 informasi penting dalam 1 detik lebih
   berguna daripada 10 informasi dalam 4 detik.
2. **Satu layar, satu tujuan.** Sales tidak sedang menjelajah; ia sedang mengerjakan satu hal.
3. **Aksi utama tidak boleh disembunyikan.** Telepon dan WhatsApp adalah alasan aplikasi ini dibuka.
4. **Keadaan basi harus jujur.** Data dari cache offline harus terlihat sebagai data cache, bukan
   dipoles seolah baru.
5. **Jangan meniru dashboard.** Tabel, filter bertumpuk, dan kolom padat tidak berpindah ke layar 6".

---

## 6. Kosakata wajib

Bahasa Indonesia, kecuali dua istilah yang memang dipertahankan:

| Backend | Tampil sebagai |
|---|---|
| Lead | **Lead** (istilah produk, tidak diterjemahkan) |
| Customer | **Customer** (idem) |
| Task | **Tugas** |
| Activity | **Aktivitas** |
| Note | **Catatan** |
| Assigned to me | **Lead Saya** |

---

## 7. Inventaris layar

Enam layar. Tidak lebih — setiap tambahan harus punya alasan yang menang melawan §5 prinsip 1.

### 7.1 Login

Email + password. Sekali saja; sesudahnya **biometric**.

Butuh desain: keadaan gagal (kredensial salah), keadaan "terlalu banyak percobaan" (backend
memberlakukan backoff), dan **layar biometric** saat aplikasi dibuka kembali — termasuk jalan keluar
"Masuk dengan password" bila biometric gagal atau tidak terdaftar.

### 7.2 Lead Saya — **layar terpenting**

Daftar lead yang ditugaskan ke sales ini. Layar yang paling sering dibuka; alokasikan usaha sesuai.

Tiap baris minimal perlu: nama, status, dan **kapan terakhir disentuh** (lead yang didiamkan dua jam
sudah kalah). Nomor lead (`#1024`) dipakai saat bicara dengan Owner.

Butuh desain: daftar normal · **daftar dari cache offline** (§10) · kosong · sedang memuat · filter
status.

### 7.3 Detail Lead — **layar terpenting kedua**

Semua aksi tulis ada di sini: **telepon**, **WhatsApp**, **ubah status**, **tambah catatan**, plus
timeline aktivitas.

Telepon dan WhatsApp adalah alasan layar ini dibuka — keduanya harus terjangkau ibu jari tanpa
menggulir. Timeline penting tapi sekunder.

Butuh desain: susunan aksi utama · pemilih status (hanya transisi sah) · form catatan · timeline ·
**dialog konflik** (§8.2).

### 7.4 Tugas Saya

Daftar task, dengan jatuh tempo. Menandai selesai **satu arah** — tidak bisa dibuka kembali.

### 7.5 Notifikasi

Daftar notifikasi (lead baru ditugaskan). Menekannya membuka lead terkait.

### 7.6 Kerangka aplikasi

Navigasi antar Lead Saya / Tugas / Notifikasi, header, dan tempat tombol keluar. **Ini dipakai
seluruh layar** — mohon didesain eksplisit, jangan diasumsikan.

---

## 8. Pola yang tidak boleh disederhanakan

### 8.1 Offline

Bukan pesan error. Sales **memang** akan membuka aplikasi tanpa sinyal, dan daftar leadnya **harus
tetap terbaca**. Butuh: penanda bahwa data berasal dari cache + kapan terakhir diperbarui, dan
perlakuan berbeda untuk aksi tulis (yang memang tidak bisa dilakukan offline).

### 8.2 Data sudah diubah orang lain

Sales mengubah status; ternyata Owner sudah memindahkan lead itu ke orang lain. Sistem **menolak** dan
mengembalikan keadaan terkini. **Tidak pernah menimpa diam-diam.**

Butuh desain dialog yang jujur tanpa menyalahkan pengguna, dengan jalan keluar "muat ulang".

### 8.3 Aksi eksternal yang mungkin dibatalkan

Menekan "Telepon" membuka dialer OS. Sales bisa saja membatalkannya. Aktivitas hanya dicatat bila
aplikasi eksternal **benar-benar** terbuka — desain tidak boleh menjanjikan "tercatat" sebelum itu
pasti.

### 8.4 Kehilangan akses mendadak

Employee yang dinonaktifkan Owner akan kehilangan akses **saat token berikutnya disegarkan** — bisa
di tengah pemakaian. Butuh desain keadaan "sesi berakhir" yang tidak terlihat seperti kerusakan
aplikasi.

---

## 9. Nilai yang butuh perlakuan visual

### 9.1 Status lead (8) — dan transisi yang sah

`new` Baru · `contacted` Dihubungi · `qualified` Memenuhi Syarat · `proposal` Penawaran ·
`won` Menang · `lost` Kalah · `unqualified` Tidak Memenuhi Syarat · `spam` Spam

Jalur utama bergerak **satu langkah** maju/mundur:

```
Baru ⇄ Dihubungi ⇄ Memenuhi Syarat ⇄ Penawaran ⇄ Menang
```

`Kalah`, `Tidak Memenuhi Syarat`, `Spam` bisa dicapai dari status mana pun. `Kalah` **wajib** disertai
alasan (§9.2). `Tidak Memenuhi Syarat` dan `Spam` bersifat final.

Delapan status ini butuh perlakuan visual yang bisa dibedakan **sekilas, di bawah matahari**.

### 9.2 Alasan kalah (6)

Harga · Kompetitor · Waktu Tidak Tepat · Tidak Merespons · Tidak Tertarik · Lainnya

Muncul sebagai langkah wajib saat memilih `Kalah`.

### 9.3 Tipe aktivitas di timeline (10)

Dibuat · Ditugaskan · Dilepas · Status berubah · Dikonversi · Catatan · **Telepon** · **WhatsApp** ·
Tugas dibuat · Tugas selesai

Yang ditebalkan berasal dari aplikasi ini sendiri — layak dibedakan dari yang datang dari dashboard.

---

## 10. Keadaan yang wajib punya desain

Bukan hanya keadaan "semuanya baik":

| Keadaan | Di mana |
|---|---|
| Sedang memuat | Semua daftar |
| Kosong (belum ada lead ditugaskan) | Lead Saya, Tugas |
| **Dari cache, tanpa sinyal** | Lead Saya, Detail Lead |
| Aksi tulis gagal karena offline | Detail Lead |
| Konflik data (§8.2) | Detail Lead |
| Sesi berakhir (§8.4) | Global |
| Gagal biometric | Login |
| Kesalahan validasi per field | Login, form catatan |

---

## 11. Di luar cakupan desain

- **iOS** — ditunda; jangan mendesain pola iOS-spesifik
- Membuat lead baru, konversi ke Customer, kelola tim, laporan/metrik — **bukan** milik employee
- Chat in-app, VoIP, peta/geolokasi
- Mode gelap — belum diputuskan; kalau ingin diusulkan, sampaikan sebagai catatan, bukan set kedua
- Onboarding/tur, splash screen bermerek
- Ikon aplikasi & branding (menunggu keputusan branding produk)

---

## 12. Bentuk keluaran yang diharapkan

1. **Token**: warna, tipografi, spacing — dengan **rasio kontras tertulis** untuk tiap pasangan
   teks/latar
2. **Enam layar** (§7) dalam keadaan normalnya
3. **Keadaan di §10** — ini yang paling sering terlewat, dan yang paling mahal ditemukan saat
   implementasi
4. **Kerangka navigasi** (§7.6)
5. Catatan singkat untuk keputusan yang tidak terlihat dari gambar

Ukuran acuan: **Android 6", 360×800 dp**. Bila satu layar hanya bekerja di layar besar, katakan.
