# PRD Desain — Jualin CRM: Redesain Dashboard (Responsif) + Laporan + Aplikasi Mobile

> **Dokumen ini untuk desainer (Claude Design), bukan untuk implementor.**
>
> Tujuannya satu: dalam **satu putaran desain**, menghasilkan desain lengkap dan **bisa diklik** untuk tiga
> hal sekaligus — (A) dashboard web yang dirombak dan **responsif sampai layar HP**, (B) bagian **Laporan**
> yang baru, dan (C) **aplikasi mobile** untuk Employee. Semua layar di sini sudah ada dan berjalan, kecuali
> Laporan. Yang diminta adalah **tampilan baru yang menarik, rapi, dan konsisten**, bukan fitur baru.
>
> Semua data, aturan, dan batasan di bawah diambil dari kode dan keputusan yang **sudah berjalan** per
> 21 September 2026. Bila desain terasa butuh data yang tidak disebut di sini, **tanyakan dulu** —
> kemungkinan besar data itu tidak ada di API.
>
> Brief sebelumnya — [`03-owner-dashboard/design-brief.md`](../03-owner-dashboard/design-brief.md) dan
> [`05-employee-mobile/design-brief.md`](../05-employee-mobile/design-brief.md) — **sudah basi di beberapa
> bagian** (token, label, dan layar yang ditambahkan sesudahnya). Bila bertentangan, **dokumen ini yang
> berlaku**.

---

## Daftar isi

1. [Yang diminta — ringkas](#1-yang-diminta--ringkas)
2. [Produk dalam satu paragraf](#2-produk-dalam-satu-paragraf)
3. [Dua "mobile" yang berbeda — jangan dicampur](#3-dua-mobile-yang-berbeda--jangan-dicampur)
4. [Pengguna & role](#4-pengguna--role)
5. [Arah visual — yang boleh dan tidak boleh diubah](#5-arah-visual--yang-boleh-dan-tidak-boleh-diubah)
6. [Kosakata wajib](#6-kosakata-wajib)
7. [Nilai yang butuh perlakuan visual](#7-nilai-yang-butuh-perlakuan-visual)
8. [A. Dashboard web — inventaris layar](#8-a-dashboard-web--inventaris-layar)
9. [A. Aturan responsif dashboard](#9-a-aturan-responsif-dashboard)
10. [B. Laporan — baru, desain saja](#10-b-laporan--baru-desain-saja)
11. [C. Aplikasi mobile Employee — inventaris layar](#11-c-aplikasi-mobile-employee--inventaris-layar)
12. [Pola yang tidak boleh disederhanakan](#12-pola-yang-tidak-boleh-disederhanakan)
13. [Keadaan yang wajib punya desain](#13-keadaan-yang-wajib-punya-desain)
14. [Data contoh untuk prototipe](#14-data-contoh-untuk-prototipe)
15. [Di luar cakupan](#15-di-luar-cakupan)
16. [Bentuk keluaran yang diharapkan](#16-bentuk-keluaran-yang-diharapkan)
17. [Pertanyaan terbuka untuk pemilik produk](#17-pertanyaan-terbuka-untuk-pemilik-produk)

---

## 1. Yang diminta — ringkas

| # | Bagian | Target | Prioritas |
|---|---|---|---|
| **A** | **Dashboard web**, dirombak visualnya | Desktop **dan** tablet **dan** HP (Owner/Admin/Manager membuka lewat browser HP) | Tinggi |
| **B** | **Laporan** — item baru di navigasi | Desktop, tablet, HP | Tinggi — **desain saja**, API-nya dibangun belakangan |
| **C** | **Aplikasi mobile Employee** (Flutter) | HP Android & iOS, acuan 360×800 dp | Tinggi |

**Hasil yang diharapkan:** satu sistem desain yang sama untuk ketiganya, seluruh layar di bawah dalam
keadaan normal **dan** keadaan pentingnya (§13), serta **prototipe yang bisa diklik** memakai data contoh
§14 — supaya alur bisa dicoba seperti aplikasi sungguhan, bukan sekadar gambar diam.

**Prioritas perhatian tidak dibagi rata.** *"Daftar lead & detail lead adalah produknya — dibuka ratusan kali
sehari; sisanya sesekali."* Dua layar itu (di dashboard **dan** di mobile) layak mendapat porsi terbesar.

---

## 2. Produk dalam satu paragraf

Jualin CRM mencatat **lead** (seseorang menunjukkan minat), menugaskannya ke anggota tim, merekam apa yang
terjadi padanya, dan mengubah yang berhasil menjadi **customer**. Alurnya: **Capture → Manage → Assign →
Follow-up → Customer**. Lead masuk dari empat jalur: diketik manual, lewat **API**, lewat **formulir** yang
ditempel di website pelanggan, atau lewat **webhook**. SaaS multi-tenant berharga terjangkau untuk UMKM
Indonesia — satu akun bisnis = satu *organization*.

**Bukan** produk ini: HRIS, akuntansi, inventory, payroll, invoice, email campaign, chat inbox
WhatsApp/IG/FB, payment gateway. Bila ide desain menyentuh domain itu, **hentikan dan tandai sebagai
*scope discussion*** — jangan didesain.

---

## 3. Dua "mobile" yang berbeda — jangan dicampur

Ini sumber kebingungan paling mungkin. Ada **dua produk berbeda** yang sama-sama dilihat di HP:

| | **Dashboard web di HP** (bagian A) | **Aplikasi mobile** (bagian C) |
|---|---|---|
| Teknologi | Next.js di browser HP | Aplikasi Flutter terpasang (Android & iOS) |
| Siapa | **Owner, Admin, Manager** | **Employee saja** |
| Tujuan | Memantau dan mengelola: lead seluruh organisasi, tim, laporan, pengaturan | Mengerjakan: lead **miliknya sendiri**, menelepon, mencatat |
| Cakupan | **Seluruh** dashboard, disusun ulang untuk layar kecil | **Enam layar** saja (§11) |
| Employee | ⛔ **Tidak bisa masuk dashboard sama sekali** — ditolak di layar masuk dengan pesan yang mengarahkan ke aplikasi mobile | ✅ Satu-satunya pengguna |

**Keduanya satu keluarga visual** (warna, tipografi, bentuk status sama), tetapi **tata letak dan pola
interaksinya berbeda**: dashboard di HP tetap alat manajemen yang padat; aplikasi mobile adalah alat
lapangan satu tangan.

---

## 4. Pengguna & role

### 4.1 Role dan akses

| Role | Dashboard | Mobile | Yang dilakukan |
|---|---|---|---|
| **Owner** | ✅ | ✅ (boleh, tapi jarang) | Semuanya, termasuk mengubah paket langganan |
| **Admin** | ✅ | ✅ | Hampir semuanya — tidak bisa menyentuh Owner, tidak bisa mengubah paket |
| **Manager** | ✅ | ✅ | Kelola lead & lihat metrik/laporan. **Tidak bisa**: hapus lead, konversi, ubah customer, kelola tim (undang/nonaktifkan), Connect (API/formulir/webhook), Langganan |
| **Employee** | ⛔ | ✅ | Hanya lead yang ditugaskan kepadanya |

**Aksi yang tidak diizinkan untuk role itu tidak ditampilkan** — bukan ditampilkan lalu ditolak. Untuk layar
yang seluruhnya tidak diizinkan (mis. Manager membuka Connect → API), tampilkan keadaan *"tidak tersedia
untuk role Anda"*, bukan layar kosong atau error.

Navigasi **tidak** difilter per role (semua item terlihat); pembatasan ada di dalam layarnya. Ini keputusan
yang sudah berjalan — pertahankan.

### 4.2 Konteks pemakaian

| | Dashboard | Mobile |
|---|---|---|
| Perangkat | Laptop di toko/kantor kecil; **kini juga HP** saat pemilik di luar | Android kelas menengah-bawah, layar ~6"; juga iPhone |
| Posisi | Duduk | **Berdiri, satu tangan**, sering sambil berjalan |
| Cahaya | Dalam ruangan | **Luar ruangan, matahari Indonesia** — kontras rendah tidak terbaca |
| Sinyal | Umumnya ada | **Sering buruk atau hilang** — kondisi normal, bukan kasus tepi |
| Sesi | Menit | **10–40 detik**, di antara percakapan |
| Pertanyaan utama | *"Mana yang belum ditangani? Bagaimana keadaan tim?"* | *"Siapa yang harus saya telepon sekarang?"* |

---

## 5. Arah visual — yang boleh dan tidak boleh diubah

### 5.1 Yang diminta dari arah visual

Pemilik produk menilai tampilan sekarang **kurang menarik**. Yang diharapkan: terasa seperti **produk SaaS
modern yang layak dibayar**, tetap **alat kerja** yang padat informasi — bukan etalase.

Prinsip:
1. **Alat kerja, bukan etalase.** Kepadatan informasi lebih berharga dari ruang kosong. Owner memindai
   puluhan baris.
2. **Jawab pertanyaan dalam hitungan detik.** Tiap layar utama punya satu pertanyaan yang terjawab tanpa
   menggulir.
3. **Yang berbahaya terasa berbeda.** Hapus, nonaktifkan, dan menimpa data orang lain bukan aksi rutin.
4. **Ringan.** Tanpa gambar besar, banyak font, atau animasi berat. Animasi mikro yang halus boleh.
5. **Jujur.** Data basi terlihat basi; yang terpotong diumumkan; yang tidak bisa dilakukan tidak ditawarkan.
6. **Satu sistem.** Dashboard desktop, dashboard HP, laporan, dan aplikasi mobile terasa satu keluarga.

### 5.2 Terkunci — jangan diubah

| Hal | Keputusan | Konsekuensi untuk desain |
|---|---|---|
| **Bahasa** | Bahasa Indonesia saja, tanpa i18n | Tidak ada teks Inggris di antarmuka — label, header tabel, keadaan kosong, pesan |
| **Design system dashboard** | **shadcn/ui** (style `base-nova`) + **Tailwind v4**, ikon **lucide** | Desain harus bisa dibangun dari komponen shadcn/ui; komponen kustom ditandai eksplisit |
| **Design system mobile** | **Flutter Material 3** | Berlaku di Android **dan** iOS — jangan pola yang hanya masuk akal di satu platform |
| **Tema** | **Terang saja** | Dark mode di luar cakupan (§15) |
| **Aturan transisi status** | Ditegakkan backend (§7.1) | UI hanya boleh menawarkan transisi yang sah |
| **Pemilihan organization** | Saat masuk, bukan switcher di dalam aplikasi | Tidak ada dropdown "pindah organization" di header |
| **Aksesibilitas** | **WCAG AA wajib** | Setiap pasangan teks/latar **≥4.5:1** (teks besar ≥3:1). **Tulis rasionya** di keluaran |

> ⚠️ **Pelajaran dari putaran desain sebelumnya:** aksen yang diusulkan gagal AA tipis (4.45:1), dan **lima
> dari delapan badge status gagal AA** (terburuk 3.14:1). Implementor harus menurunkan *lightness* satu per
> satu. **Hitung kontras sebelum menyerahkan**, bukan sesudah.

### 5.3 Boleh diperbarui — tunjukkan alasannya

| Hal | Sekarang | Catatan |
|---|---|---|
| **Warna aksen** | Amber hangat dari logo Jualin: `--primary oklch(0.56 0.19 41)` (latar tombol, teks putih 4.83:1) dan `--accent-strong oklch(0.48 0.17 41)` (teks/ikon di atas putih, 7.04:1) | **Pertahankan amber sebagai jangkar merek.** Boleh memperkaya palet (netral yang lebih hangat, aksen sekunder, warna semantik). Beri nilai `oklch` |
| **Warna status** | 8 warna badge (§7.1) | Boleh diperbarui total, asal lolos AA **dan** tidak hanya mengandalkan warna |
| **Tipografi** | Geist Sans + Geist Mono | Boleh diganti, sertakan alasan. **Satu** keluarga font untuk UI (performa) |
| **Radius** | `0.625rem` | Boleh disesuaikan sedikit; jangan sudut tajam atau pil penuh sebagai gaya global |
| **Kerangka navigasi** | Sidebar kiri di desktop | Boleh diubah, sertakan alasan dan versi HP-nya (§9) |

**Mobile harus satu keluarga dengan dashboard**, tetapi konteks matahari boleh menuntut kontras **lebih
tinggi** dari dashboard.

---

## 6. Kosakata wajib

Istilah yang salah di layar menjanjikan fitur yang tidak ada. Ini **bukan** preferensi gaya.

| ⛔ Jangan | ✅ Pakai | Alasan |
|---|---|---|
| "Workspace" | **Organization** | Satu istilah di seluruh produk |
| "Team" / "Tim saya" sebagai kelompok | "Semua lead" / "seluruh organisasi" | Entity Team **tidak ada**. Menu **Tim** = daftar anggota, bukan kelompok |
| "Karyawan" / "Staff" | **Anggota** | Employee adalah **role**, bukan entity |
| "Pipeline" sebagai papan Kanban | **Status lead** | Tidak ada entity pipeline |
| "Deal", "Nilai deal", "Revenue", "Omzet" | — (**jangan tampilkan**) | Deal belum ada. **Angka uang tidak boleh muncul di mana pun, termasuk Laporan** — satu-satunya pengecualian: **harga paket** di layar Langganan |

**Istilah yang dipakai di layar:** Lead · Customer · Aktivitas · Tugas · Anggota · Undangan · Organization ·
Notifikasi · Connect · Formulir · Webhook · Langganan · Laporan.

"Lead" dan "Customer" **tidak** diterjemahkan (istilah produk). Lead ditampilkan dengan **nomor urut per
organization** (`#1024`), bukan UUID.

---

## 7. Nilai yang butuh perlakuan visual

Enumerasi lengkap dari database — tidak bertambah tanpa perubahan skema.

### 7.1 Status lead (8) — dan transisi yang sah

| Nilai | Label | Warna teks sekarang (boleh diganti, AA wajib) |
|---|---|---|
| `new` | **Baru** | biru — `oklch(0.52 0.18 255)` |
| `contacted` | **Dihubungi** | ungu — `oklch(0.535 0.16 300)` |
| `qualified` | **Memenuhi Syarat** | teal — `oklch(0.49 0.12 195)` |
| `proposal` | **Penawaran** | amber — `oklch(0.53 0.15 75)` |
| `won` | **Menang** | hijau — `oklch(0.5 0.15 145)` |
| `lost` | **Kalah** | merah — `oklch(0.54 0.2 25)` |
| `unqualified` | **Tidak Memenuhi Syarat** | abu — `oklch(0.5 0 0)` |
| `spam` | **Spam** | abu kecoklatan — `oklch(0.42 0.03 30)` |

Badge sekarang: teks berwarna di atas latar warna yang sama dicampur 85% putih.

**Aturan transisi — mengikat, sudah berjalan, tidak boleh diubah oleh desain:**

```
Baru → Dihubungi ⇄ Memenuhi Syarat ⇄ Penawaran ⇄ Menang
```

- Maju **satu langkah**. Mundur **satu langkah** — **kecuali ke Baru**: tidak ada jalan kembali ke Baru dari
  mana pun (Baru berarti *"belum disentuh"*).
- **Kalah**, **Tidak Memenuhi Syarat**, **Spam** bisa dicapai dari status mana pun di jalur utama.
- **Kalah wajib alasan** (§7.2). Lead Kalah bisa **dibuka kembali ke Dihubungi** (satu-satunya pilihan yang
  ditawarkan UI).
- **Tidak Memenuhi Syarat** dan **Spam** bersifat **final** — tidak ada tombol status sama sekali.
- **Menang yang sudah dikonversi jadi Customer: statusnya terkunci** — tidak ada tombol status; tampilkan
  kalimat *"Lead ini sudah dikonversi menjadi Customer; statusnya tidak dapat diubah lagi."* Menang yang
  **belum** dikonversi masih bisa mundur/ditutup.

**Delapan status harus bisa dibedakan sekilas** — di tabel padat, dan di bawah matahari di mobile — **tanpa
mengandalkan warna saja** (bentuk/ikon/teks).

**Tidak Memenuhi Syarat** dan **Spam** harus terlihat berbeda satu sama lain: keduanya **dikecualikan dari
conversion rate**, tetapi artinya berbeda (salah sasaran vs sampah).

### 7.2 Alasan kalah (6)
Harga · Kompetitor · Waktu Tidak Tepat · Tidak Merespons · Tidak Tertarik · Lainnya

### 7.3 Sumber lead (4) — metode capture, bukan channel marketing
Manual · API · Formulir · Webhook

### 7.4 Tipe aktivitas di timeline (10)

| Dibuat sistem | Dibuat manusia |
|---|---|
| Lead dibuat · Ditugaskan · Dilepas · Status berubah (*"Baru → Dihubungi"*) · Dikonversi · Tugas dibuat · Tugas selesai | **Catatan** · **Telepon** · **WhatsApp** |

Timeline harus membedakan **peristiwa sistem** dari **jejak manusia** — pengguna membaca timeline untuk
mencari apa yang dilakukan orang. Membuka **email** dari mobile **tidak** tercatat di timeline (keputusan
pemilik produk).

### 7.5 Role (4)
Owner · Admin · Manager · Employee

### 7.6 Paket langganan (3)

| Paket | Lead/bulan | Anggota | Harga |
|---|---|---|---|
| **Free** | 100 | 2 | Gratis |
| **Pro** | 2.000 | 10 | **Rp99.000/bulan** |
| **Enterprise** | Tanpa batas | Tanpa batas | Hubungi kami (tanpa angka) |

---

## 8. A. Dashboard web — inventaris layar

Semua layar ini **sudah ada dan berfungsi**; datanya sudah tersedia di API. Yang diminta: tampilan baru +
versi responsif (§9).

### 8.1 Navigasi utama (kerangka aplikasi)

Urutan item sekarang, **ditambah Laporan**:

**Beranda · Lead · Customer · Tugas · Laporan** *(baru)* **· Tim · Connect · Langganan · Pengaturan**

Plus: **lonceng notifikasi** dengan jumlah belum dibaca, **identitas pengguna** (nama, nama organization,
role), dan **Keluar**. Badge jumlah di item **Lead** = jumlah lead **tanpa pemilik aktif**.

Desain versi desktop **dan** HP (§9.2).

### 8.2 Auth — layar publik, kartu sempit di tengah

| Layar | Isi | Catatan |
|---|---|---|
| **Masuk** | Email, password, *Lupa password?*, *Daftar organization baru* | Bila akun punya >1 organization, form berubah jadi **pemilih organization** tanpa mengetik ulang email/password (§12.2). Employee **ditolak** di sini dengan banner *"Akun Anda terdaftar sebagai Employee. Gunakan aplikasi mobile Jualin untuk masuk."* |
| **Daftar** | Nama organization, nama lengkap, email, password (min. 12 karakter) | Sukses → layar "cek email", bukan langsung masuk |
| **Verifikasi email** | Otomatis dari tautan email | Tiga keadaan: memverifikasi / berhasil / gagal + form kirim ulang |
| **Lupa password** | Satu field email | Selalu tampil sama, terdaftar atau tidak |
| **Atur ulang password** | Password baru + konfirmasi | |
| **Terima undangan** | Dari tautan email, **dua cabang** | Pengguna baru: isi nama + password. Pengguna yang sudah punya akun: satu tombol "Gabung" |

### 8.3 Beranda — *"bagaimana keadaan bisnis periode ini?"*

- Pemilih **periode**.
- Kartu metrik: **Lead masuk**, **Belum ter-assign**, **Conversion rate**, jumlah **per status**.
- Tabel **performa per anggota**: jumlah lead, **waktu respons rata-rata**, jumlah konversi.
- Setiap angka **bisa diklik** menuju daftar lead yang sudah ter-filter.
- **Conversion rate bisa "belum ada data"** — harus tampil **berbeda dari 0%** ("belum ada yang bisa
  dihitung" ≠ "sudah dicoba, gagal semua").
- Pintasan ke **Laporan** untuk analisis lebih dalam.

### 8.4 Daftar lead — **layar terpenting**. *"Mana yang belum ditangani?"*

**Per baris:** `#1024`, nama, status, pemilik, sumber, tanggal masuk; email/telepon bila muat.
**Badge "Belum ada kontak"** (netral, bukan merah) bila lead tidak punya email **dan** telepon.

**Filter — semua terlihat sekaligus, bisa dikombinasikan, tercermin di URL:**
- Status (multi-pilih, 8) — sebagai **chip dengan jumlah**
- Sumber (multi-pilih, 4) · Pemilik (termasuk **"tanpa pemilik aktif"**) · Periode masuk · Kata kunci (nama/email/telepon)

> **Chip "tanpa pemilik aktif" wajib terlihat permanen dengan jumlahnya** — jaring pengaman. Lead milik
> anggota yang dinonaktifkan bisa tetap tercatat atas namanya dan **tidak muncul di daftar siapa pun**.

**Juga:** tombol **Lead baru** (dialog: nama wajib, email, telepon — bila email **dan** telepon kosong, muncul
**konfirmasi lunak** *"Lead ini belum punya kontak dan tidak bisa ditindaklanjuti. Tetap simpan?"* dengan
**Kembali isi kontak / Tetap simpan** — konfirmasi, **bukan** blokir), **pagination dengan jumlah total**,
indikator filter aktif yang bisa dihapus.

**Dua keadaan kosong berbeda:** belum ada lead sama sekali vs tidak ada yang cocok filter.

### 8.5 Detail lead — **layar terpenting kedua**. *"Apa yang sudah terjadi, dan apa berikutnya?"*

| Bagian | Isi |
|---|---|
| **Header** | `#1024`, nama, tombol **Ubah**, email, telepon, perusahaan, sumber, tanggal masuk, badge status, catatan. Badge **"Belum ada kontak"** + kalimat *"Tambahkan email atau telepon lewat Ubah agar bisa ditindaklanjuti."* bila perlu. Alasan kalah bila status Kalah |
| **Area status** | **Stepper tahapan** (5 tahap jalur utama, penanda **"Saat ini"**) + tombol **dikelompokkan dan berjudul**: **Lanjutkan** ("Maju ke Menang"), **Kembali** ("Kembali ke Memenuhi Syarat"), **Buka kembali** ("Buka kembali ke Dihubungi"), **Tutup lead** (Kalah / Tidak Memenuhi Syarat / Spam). **Tanpa panah** di label. Bagian kosong **tidak ditampilkan**. Lead ditutup: semua titik stepper redup + pita *"Lead ditutup: Kalah"*. Lihat §7.1 untuk semua kasus |
| **Penugasan** | Pilih anggota, atau lepaskan |
| **Tambah catatan** | Tiga tipe: Catatan · Telepon · WhatsApp |
| **Timeline** | Terbaru di atas. Sistem vs manusia dibedakan (§7.4) |
| **Tugas** | Daftar tugas pada lead ini: buat (judul, deskripsi, jatuh tempo, penanggung jawab), tandai selesai (**satu arah**) |
| **Konversi** | **Hanya** saat status Menang dan belum pernah dikonversi. Owner/Admin saja |
| **Hapus lead** | Owner/Admin saja. Aksi merusak |

Setiap aksi tulis memperbarui timeline tanpa muat ulang manual. **Konflik** (§12.3) punya dialog sendiri.

### 8.6 Customer

Daftar (pencarian, pagination) · Detail dengan tautan **"Berasal dari lead #…"** · Ubah/Hapus (Owner/Admin;
Manager melihat tanpa tombol itu). Data customer disalin dari lead saat konversi dan bisa berubah terpisah.

### 8.7 Tugas (lintas lead)

Daftar semua tugas; filter penanggung jawab, status (**Belum selesai / Selesai**), jatuh tempo. Tandai selesai
dari daftar. **Tugas lewat jatuh tempo harus menonjol.** Label jatuh tempo kalender: *hari ini / besok / dalam
3 hari / 12 Okt / Terlambat 2 hari*.

### 8.8 Tim

| Bagian | Isi |
|---|---|
| **Anggota** | Nama, email, role, bergabung. Ubah role (aturan: tidak bisa mengubah role sendiri; Admin tidak menyentuh Owner; Owner boleh mengangkat Owner lain) |
| **Undangan** | Undang via email + role; daftar tertunda, bisa dicabut. Kena **batas seat paket** → pesan dengan tautan ke Langganan |
| **Nonaktifkan** | Alur **tiga cabang** (§12.1) |

Manager melihat daftar anggota **tanpa** tombol kelola dan tanpa bagian undangan.

### 8.9 Connect — tiga kanal masuknya lead

Halaman induk berisi **tiga kartu**: **API** · **Formulir** · **Webhook**. Kartu bisa berada dalam keadaan
**terkunci oleh paket** (redup, tak bisa diklik, **tanpa tombol upgrade dan tanpa harga**) — beda dari
badge "Belum tersedia" untuk kanal yang memang belum ada. Owner/Admin saja; Manager melihat *"tidak tersedia
untuk role Anda"*.

| Kanal | Layar | Isi penting |
|---|---|---|
| **API** | Daftar API key · Buat · Cabut · **Dokumentasi integrasi** | **Kunci mentah tampil satu kali saja** saat dibuat — desain harus membuat ini jelas dan tidak terlewat |
| **Formulir** | Daftar formulir · Detail/ubah · Snippet embed | Enam field tetap (nama, email, telepon, perusahaan, pesan, layanan) masing-masing **aktif/wajib/label**; **domain yang diizinkan** (alamat lengkap `https://…`); **snippet** untuk disalin; aktif/nonaktif |
| **Webhook** | Daftar endpoint · Detail · **Riwayat pengiriman** · Dokumentasi verifikasi | Status pengiriman (Menunggu / Berhasil / Gagal) dengan percobaan ke-N, **kirim ulang** untuk yang gagal, **signing secret tampil satu kali** |

Layar dokumentasi (API & webhook) berisi blok kode — tipografi monospace yang nyaman dibaca.

### 8.10 Langganan

**Paket aktif** · **Pemakaian** (lead bulan ini / batas, anggota / batas — bar tidak pernah >100%, "tanpa
batas" untuk Enterprise) · **Perbandingan tiga paket** (§7.6). Tombol coba Pro hanya untuk Owner di lingkungan
uji. **Enterprise tanpa tombol beli** — teks "Hubungi kami" (dengan tautan bila tersedia). Manager: *"tidak
tersedia untuk role Anda"*.

### 8.11 Pengaturan

Profil organization (nama) dan pengguna (nama, email, role). Sederhana, baca saja.

### 8.12 Notifikasi (lonceng)

Daftar notifikasi (mis. lead ditugaskan ke Anda; kuota lead bulanan habis — untuk Owner), penanda belum
dibaca, tandai satu / semua. **Tidak realtime** — diambil saat halaman dimuat; jangan desain seolah-olah
langsung.

---

## 9. A. Aturan responsif dashboard

**Ini perubahan terbesar dari desain sebelumnya:** dashboard dulu *desktop-first, HP bukan target*. Sekarang
**HP adalah target** — Owner/Admin/Manager memeriksa lead dan tim dari HP saat di luar.

### 9.1 Breakpoint

| Nama | Lebar | Acuan desain |
|---|---|---|
| **HP** | 360–767 px | **390 px** |
| **Tablet** | 768–1023 px | **820 px** |
| **Desktop** | ≥1024 px | **1440 px** |

**Minimum layar yang wajib didesain di ketiga breakpoint:** Masuk · Beranda · **Daftar lead** · **Detail lead**
· Laporan (ringkasan) · Kerangka navigasi. Layar lain cukup desktop + HP.

### 9.2 Pola wajib di HP

| Elemen desktop | Di HP |
|---|---|
| Sidebar navigasi | Bilah bawah untuk 4–5 item terpenting + menu "Lainnya", **atau** drawer — pilih dan beri alasan |
| **Tabel** (lead, customer, tugas, anggota, API key, pengiriman webhook) | **Daftar kartu** — tidak ada tabel yang harus digeser ke samping |
| Baris filter bertumpuk | Chip status yang bisa digeser + tombol **Filter** membuka *sheet* |
| Detail lead dua kolom | **Satu kolom**; urutan: header → area status → catatan → timeline; tugas & penugasan bisa jadi bagian yang dilipat |
| Dialog | *Sheet* dari bawah atau layar penuh |
| Stepper 5 tahap | Tetap terbaca di 360 px — label boleh membungkus, **tidak boleh meluap** |
| Tombol aksi utama | Terjangkau ibu jari |

**Tidak ada guliran horizontal halaman di lebar mana pun.** Target sentuh minimal **44 px** di HP.

---

## 10. B. Laporan — baru, desain saja

> **Status: DESAIN SAJA.** API untuk sebagian besar laporan **belum ada** dan akan dibangun belakangan.
> Setiap blok di bawah diberi tanda **[TERSEDIA]** (data sudah ada di API sekarang) atau **[API BARU]**
> (butuh endpoint baru). Desain keduanya dengan kualitas yang sama.

### 10.1 Batasan yang mengikat

- **Tanpa angka uang.** Tidak ada revenue, omzet, nilai deal, target penjualan, komisi. Deal belum ada, dan
  komisi berbatasan payroll (di luar cakupan).
- **Laporan dasar saja.** "Laporan lanjutan" (kustom, pivot, pembanding antar-periode rumit) adalah fase
  berikutnya — jangan didesain.
- **Akses:** Owner, Admin, Manager. Employee tidak (ia tidak masuk dashboard).
- **Bukan ekspor.** Tidak ada unduh CSV/PDF di putaran ini (§15). Boleh disiapkan tempatnya secara visual
  bila memang tidak mengganggu, ditandai sebagai "nanti".
- **Conversion rate** selalu **mengecualikan Spam dan Tidak Memenuhi Syarat** dari penyebutnya — tulis
  keterangannya di layar, karena pengguna akan bertanya kenapa angkanya berbeda dari hitungan sendiri.

### 10.2 Kerangka layar

- **Filter global di atas:** periode (7 hari, 30 hari, bulan ini, bulan lalu, rentang kustom) — **[TERSEDIA]**
  untuk semua blok [TERSEDIA].
- Filter anggota dan sumber — **[API BARU]**.
- Blok-blok di bawah, yang di HP menjadi satu kolom yang bisa digulir.

### 10.3 Blok laporan

| # | Blok | Isi | Data |
|---|---|---|---|
| 1 | **Ringkasan** | Kartu: lead masuk, conversion rate, belum ter-assign, jumlah lead Menang | **[TERSEDIA]** |
| 2 | **Distribusi status** | Berapa lead (yang masuk pada periode) di tiap status, bar horizontal per status | **[TERSEDIA]** — ⚠️ ini **sebaran status saat ini**, **bukan** corong konversi bertahap. Jangan beri label "funnel/corong"; datanya tidak mencatat berapa yang *pernah* melewati tiap tahap |
| 3 | **Performa anggota** | Tabel/daftar per anggota: jumlah lead, waktu respons rata-rata, konversi, **conversion rate per anggota** (turunan) — bisa diurutkan | **[TERSEDIA]** |
| 4 | **Tren lead masuk** | Grafik garis/batang per hari atau minggu | **[API BARU]** |
| 5 | **Sumber lead** | Sebaran Manual / API / Formulir / Webhook, dan conversion rate per sumber | **[API BARU]** |
| 6 | **Alasan kalah** | Sebaran 6 alasan (§7.2) | **[API BARU]** |
| 7 | **Waktu respons** | Sebaran waktu dari lead masuk sampai sentuhan pertama (mis. < 1 jam, 1–4 jam, 4–24 jam, > 1 hari) | **[API BARU]** |
| 8 | **Tugas** | Tugas selesai vs lewat jatuh tempo per anggota | **[API BARU]** |

### 10.4 Grafik

- Sederhana: **batang, garis, bar horizontal.** Hindari pie/donut untuk lebih dari 4 kategori, 3D, atau
  grafik yang butuh legenda panjang.
- **Setiap grafik punya padanan angka** yang terbaca (label nilai atau tabel ringkas) — grafik tanpa angka
  tidak bisa dipakai untuk mengambil keputusan, dan tidak terbaca pembaca layar.
- Warna status di grafik **sama** dengan warna badge status.
- Tidak mengandalkan warna saja untuk membedakan seri.
- Tampil di 360 px tanpa guliran horizontal.

### 10.5 Keadaan khusus Laporan

- **Belum ada data** pada periode (organization baru) — ajakan, bukan grafik kosong.
- **Data sedikit** (mis. 3 lead) — conversion rate dan grafik tetap tampil, tapi dengan catatan bahwa
  angkanya belum bermakna.
- **Blok [API BARU]** dalam prototipe: tampilkan dengan data contoh seperti blok lain. (Keputusan apakah blok
  itu disembunyikan atau ditandai "segera hadir" sampai API-nya ada — lihat §17.)
- **Tidak tersedia untuk role** — tidak berlaku untuk Owner/Admin/Manager; sudah tertutup di pintu masuk.

---

## 11. C. Aplikasi mobile Employee — inventaris layar

Pengguna: **Employee saja**. Aplikasi terpasang, Flutter Material 3, **Android dan iOS**. Acuan **360×800 dp**
(Android) dan cek di **393×852** (iPhone). Semua layar sudah ada dan berfungsi.

Prinsip khusus mobile: **kecepatan mengalahkan kelengkapan**; satu layar satu tujuan; **Telepon dan WhatsApp
adalah alasan aplikasi dibuka**; data basi harus jujur; **jangan meniru dashboard** (tanpa tabel, tanpa
filter bertumpuk).

### 11.1 Masuk + biometrik

- **Masuk:** email + password, sekali. Keadaan: gagal (*"Email atau password salah."*), terlalu banyak
  percobaan.
- **Gerbang biometrik** saat aplikasi dibuka kembali (sidik jari/wajah), dengan jalan keluar **"Masuk dengan
  password"**. Membatalkan biometrik **tetap** di layar kunci.
- **Sesi berakhir** (mis. dinonaktifkan Owner) — layar tersendiri yang tidak terlihat seperti aplikasi rusak.

### 11.2 Lead Saya — **layar terpenting**

- Per baris: **nama**, `#1024`, **kapan terakhir disentuh** (*"disentuh 2j lalu"* — lead yang didiamkan dua jam
  sudah kalah), badge status, dan badge **"Belum ada kontak"** bila perlu.
- **Pencarian** + **chip filter status**.
- **Pita cache**: *"Data dari cache · diperbarui 08:14"* saat offline.
- **Pita pemotongan**: *"Menampilkan 100 dari 134 lead. 34 lead terlama tidak ditampilkan. Persempit dengan
  status atau pencarian."* — hanya bila benar-benar terpotong.
- Tarik untuk menyegarkan.

### 11.3 Detail lead — **layar terpenting kedua**

- **Header:** nama, `#1024`, sumber, kapan disentuh, badge status, perusahaan, **email sebagai tautan** (ikon
  surat, membuka aplikasi email — tidak tercatat di timeline), catatan, alasan kalah bila ada, badge **"Belum
  ada kontak"** bila perlu.
- **Bilah aksi bawah — Telepon dan WhatsApp**, terjangkau ibu jari tanpa menggulir. Bila nomor tidak ada,
  kedua tombol mati **dengan kalimat penjelas** *"Lead ini belum punya nomor telepon."* WhatsApp juga mati bila
  nomor tidak bisa dijadikan format internasional. Bila aplikasi dialer/WhatsApp/email tidak tersedia di HP,
  muncul pesan — tombol tidak pernah diam.
- **Ubah status:** tombol membuka *bottom sheet* berisi **hanya transisi yang sah** (§7.1); memilih Kalah
  membuka langkah kedua **wajib memilih alasan**. Lead terkonversi: kalimat kunci, tanpa tombol. Status final:
  *"Status ini bersifat final."*
- **Form catatan:** tiga tipe (Catatan · Telepon · WhatsApp) + isian teks.
- **Timeline:** aktivitas yang dibuat dari aplikasi ini (**Telepon**, **WhatsApp**) dibedakan dari yang datang
  dari dashboard.
- **Dialog konflik** (§12.3).

> **Pertimbangkan membawa stepper tahapan** dari dashboard (§8.5) ke mobile dalam bentuk ringkas — ia
> membantu Employee tahu posisi lead. Bila tidak muat dengan tujuan "10–40 detik", katakan dan beri alasan.

### 11.4 Tugas Saya

- Dua segmen: **Belum selesai** (default) · **Selesai** (riwayat, terbaru diselesaikan di atas).
- Belum selesai: judul, label jatuh tempo kalender (*"Jatuh tempo hari ini / besok / dalam 3 hari / 12 Okt"*,
  *"Terlambat 2 hari"* menonjol), checkbox **satu arah**.
- Selesai: dicoret, *"Selesai 2j lalu"*, checkbox terkunci; baris tetap bisa diketuk membuka lead-nya.
- Mengetuk baris membuka lead terkait. Pita cache dan pita pemotongan seperti Lead Saya.

### 11.5 Notifikasi

Daftar notifikasi (lead ditugaskan ke Anda). Mengetuknya membuka lead. Belum dibaca dibedakan.

### 11.6 Kerangka aplikasi

Navigasi bawah: **Lead Saya · Tugas · Notifikasi** (+ tempat **Keluar**). Header, identitas pengguna. Push
notification membuka detail lead langsung (tiga keadaan: aplikasi terbuka, di latar, tertutup).

---

## 12. Pola yang tidak boleh disederhanakan

Masing-masing melindungi kegagalan yang **tidak terlihat** bila salah.

### 12.1 Menonaktifkan anggota yang masih memegang lead terbuka (dashboard)
Backend menolak dan mengembalikan **jumlah lead terbuka**. Dialog **memaksa memilih** di antara:
**Lepas penugasan** · **Pindahkan ke anggota lain** (dengan pemilih) · **Batal**.
⛔ **Tidak boleh** jadi *"Yakin? [Ya] [Batal]"* — lead yang tetap atas nama orang yang tidak bisa login lagi
hilang dari daftar siapa pun. Anggota tanpa lead terbuka dinonaktifkan tanpa dialog tambahan.

### 12.2 Pengguna dengan lebih dari satu organization (dashboard)
Layar masuk berubah menjadi pemilih organization, lalu lanjut — **tanpa** mengetik ulang email/password.
Organization tempat pengguna berperan Employee **tidak** ditawarkan.

### 12.3 Data sudah diubah orang lain — konflik (dashboard & mobile)
Penyimpanan kedua ditolak, backend mengirim keadaan terkini. Layar **memberi tahu, memuat ulang, dan tidak
pernah menimpa otomatis** — pengguna memutuskan. Perlakuannya tidak boleh sama dengan notifikasi kesalahan
biasa yang bisa diabaikan.

### 12.4 Kesalahan validasi per field (dashboard & mobile)
Tampil **di bawah field yang bersangkutan**. Pesan global hanya untuk yang bukan milik satu field.

### 12.5 Kredensial yang tampil satu kali (dashboard)
API key mentah dan signing secret webhook **hanya tampil sekali**. Desain harus membuat ini mustahil
terlewat: tombol salin, peringatan jelas, dan konfirmasi sebelum menutup.

### 12.6 Offline (mobile)
Bukan pesan error. Daftar **tetap terbaca** dari cache dengan penanda waktu. Aksi tulis (ubah status, catatan)
**tidak bisa** offline — perlakuannya harus jelas sebelum ditekan, bukan gagal setelahnya.

### 12.7 Aksi eksternal (mobile)
Telepon/WhatsApp/email membuka aplikasi lain. Aktivitas Telepon/WhatsApp dicatat **setelah** aplikasi lain
benar-benar terbuka — desain tidak boleh menjanjikan "tercatat" sebelum itu.

---

## 13. Keadaan yang wajib punya desain

| Keadaan | Dashboard | Mobile |
|---|---|---|
| Memuat (skeleton) | Semua daftar, Laporan | Semua daftar |
| Kosong — belum ada data | Lead, Customer, Tugas, Laporan, Connect | Lead Saya, Tugas (kedua segmen), Notifikasi |
| Kosong — tidak cocok filter | Lead, Customer, Tugas | Lead Saya |
| Gagal memuat / kehilangan koneksi | Semua | Semua |
| **Dari cache, tanpa sinyal** | — | Lead Saya, Detail, Tugas |
| **Daftar terpotong** (pita "Menampilkan 100 dari N") | — | Lead Saya, Tugas |
| Sesi berakhir | ✅ | ✅ |
| Tidak diizinkan untuk role | Connect, Langganan, aksi tertentu | — |
| Terkunci oleh paket | Kartu Connect | — |
| Kuota/seat habis | Dialog lead baru, undangan | — |
| Terlalu banyak percobaan | Masuk, kirim ulang email | Masuk |
| Konflik (§12.3) | Detail lead, Tugas | Detail lead |
| Notifikasi belum dibaca | Lonceng | Tab Notifikasi |
| Lead tanpa kontak | Daftar, detail, dialog buat | Daftar, detail, tombol telepon |
| Lead terkonversi / status final | Area status | Pemilih status |
| Gagal biometrik | — | Gerbang biometrik |

---

## 14. Data contoh untuk prototipe

Pakai data ini supaya seluruh layar **saling konsisten** saat diklik (lead yang sama muncul di daftar, detail,
beranda, dan laporan dengan angka yang cocok).

**Organization:** Toko Maju Jaya (paket **Pro**, 64 dari 2.000 lead bulan ini, 3 dari 10 anggota aktif).

**Anggota:**

| Nama | Role | Lead | Waktu respons rata-rata | Konversi |
|---|---|---|---|---|
| Budi Santoso | Owner | 10 | 38 menit | 3 |
| Sari Wulandari | Manager | 18 | 1 jam 12 menit | 4 |
| Andi Pratama | Employee | 24 | 2 jam 40 menit | 2 |
| Rina Kartika *(nonaktif)* | Employee | 3 *(masih atas namanya)* | — | — |

**Lead contoh:**

| # | Nama | Status | Pemilik | Sumber | Kontak | Catatan |
|---|---|---|---|---|---|---|
| #1024 | Dewi Lestari | Penawaran | Andi | Formulir | 0812-3456-7890 · dewi@contoh.id | Tertarik paket katering 50 porsi |
| #1025 | Hendra Wijaya | Baru | — *(tanpa pemilik)* | API | 0813-1111-2222 | |
| #1026 | CV Sinar Abadi | Menang *(sudah dikonversi)* | Sari | Manual | sinar@contoh.id | |
| #1027 | Maya Anggraini | Kalah — *Harga* | Andi | Webhook | 0857-9999-0000 | |
| #1028 | Dodo | Baru | Budi | Manual | **— (belum ada kontak)** | |
| #1029 | Rudi Hartono | Dihubungi | Rina *(nonaktif)* | Formulir | 0821-4444-5555 | Lead tanpa pemilik aktif |
| #1030 | promo-bot | Spam | — | Formulir | spam@spam.xyz | |

**Ringkasan periode "30 hari":** 64 lead masuk (10 + 18 + 24 + 3 milik anggota, + 9 belum ter-assign) ·
conversion rate 18% (9 Menang dari 50 lead, setelah mengecualikan 9 Spam + 5 Tidak Memenuhi Syarat) ·
**3 lead tanpa pemilik aktif** (milik Rina yang sudah nonaktif — muncul di chip "tanpa pemilik aktif").

Untuk keadaan **kuota/seat habis**, tampilkan varian organization yang sama di paket **Free** (100 lead, 2
anggota) — di sana undangan anggota ketiga ditolak dan lead ke-101 bulan itu ditolak.

**Tugas contoh:** *Kirim penawaran ke Dewi* (jatuh tempo besok) · *Telepon ulang Hendra* (terlambat 2 hari) ·
*Follow up Rudi* (hari ini) · *Konfirmasi order CV Sinar Abadi* (selesai kemarin).

**Timeline #1024:** Lead dibuat (Formulir) → Ditugaskan ke Andi → Status: Baru → Dihubungi → **Telepon**
(Andi, dari HP) → Catatan: "Minta dikirim menu" → Status: Dihubungi → Memenuhi Syarat → Status: Memenuhi Syarat
→ Penawaran → Tugas dibuat: *Kirim penawaran ke Dewi*.

---

## 15. Di luar cakupan

Jangan desain, jangan sertakan sebagai "bonus":

- **Dark mode / tema / kustomisasi tampilan**
- **Angka uang** — revenue, nilai deal, target, komisi (kecuali harga paket di Langganan)
- **Papan Kanban / drag-and-drop pipeline** (entity pipeline tidak ada)
- **Laporan lanjutan** — builder laporan kustom, pivot, perbandingan antar-periode rumit
- **Ekspor/impor CSV atau PDF**
- **Realtime** — jangan desain sesuatu yang mengandaikan pembaruan langsung tanpa muat ulang
- **Chat in-app, VoIP, peta/geolokasi, inbox WhatsApp**
- **Membuat lead, konversi, kelola tim, laporan di aplikasi mobile** — bukan milik Employee
- **Landing page / halaman marketing** (proyek terpisah)
- **Onboarding tour, ilustrasi bergerak besar, splash bermerek, ikon aplikasi** — bila terasa perlu, usulkan
  terpisah dengan alasan

---

## 16. Bentuk keluaran yang diharapkan

1. **Token desain** — warna (`oklch`), tipografi, spacing, radius, bayangan. **Rasio kontras tertulis** untuk
   setiap pasangan teks/latar, termasuk 8 badge status. Sebutkan token shadcn yang diganti dan padanannya di
   Flutter.
2. **Peta komponen** — dashboard: komponen shadcn/ui per elemen; mobile: widget Material 3. Tandai eksplisit
   yang kustom.
3. **Dashboard (A):** seluruh layar §8 di desktop **dan** HP; layar minimum §9.1 juga di tablet; keadaan §13.
4. **Laporan (B):** seluruh blok §10.3 di desktop dan HP, dengan tanda [TERSEDIA]/[API BARU] tetap terlihat di
   catatan desain (bukan di UI).
5. **Mobile (C):** seluruh layar §11 di 360×800, dicek di 393×852, dengan keadaan §13.
6. **Pola §12** didesain eksplisit — terutama dialog penonaktifan tiga cabang, konflik, dan kredensial satu kali.
7. **Prototipe yang bisa diklik** memakai data §14, minimal alur:
   - Dashboard: Masuk → Beranda → klik "Belum ter-assign" → Daftar lead ter-filter → Detail #1024 → ubah status
     → tugaskan → kembali; Laporan; Tim → nonaktifkan Rina (dialog tiga cabang).
   - Mobile: buka aplikasi → biometrik → Lead Saya → #1024 → Telepon → kembali, timeline bertambah → ubah status
     → Tugas Saya → tandai selesai → segmen Selesai.
8. **Catatan singkat** untuk keputusan yang tidak terlihat dari gambar (kenapa navigasi HP dipilih begitu, kenapa
   warna status dipilih, dsb.).

Bila sebuah layar terasa butuh data yang tidak disebut di sini, **tanyakan** — jangan mengarang kolom baru.

---

## 17. Pertanyaan terbuka untuk pemilik produk

Tidak menghalangi desain; desainer boleh mengusulkan jawaban.

1. **Blok Laporan [API BARU]** saat dirilis sebelum API-nya ada: disembunyikan, atau tampil sebagai "segera
   hadir"? *(Usul: disembunyikan — blok kosong yang dijanjikan terlihat seperti produk belum jadi.)*
2. **Laporan untuk semua paket, atau sebagian untuk Pro?** Belum diputuskan. Bila sebagian dikunci, pakai pola
   "terkunci oleh paket" yang sama dengan kartu Connect (tanpa harga, tanpa tombol upgrade di situ).
3. **Navigasi dashboard di HP:** bilah bawah atau drawer? *(Desainer mengusulkan dengan alasan.)*
4. **Stepper tahapan di mobile** (§11.3): dibawa atau tidak?

---

## Referensi

| Dokumen | Isi |
|---|---|
| [`../../product/scope.md`](../../product/scope.md) | Batas produk — "reports dasar" masuk, "reports lanjutan" fase berikutnya, tanpa revenue |
| [`../../product/glossary.md`](../../product/glossary.md) | Kosakata mengikat |
| [`../../decisions/ADR-015-nothing-leads-back-to-new.md`](../../decisions/ADR-015-nothing-leads-back-to-new.md) | Tidak ada jalan kembali ke Baru |
| [`../../decisions/ADR-016-status-locked-after-conversion.md`](../../decisions/ADR-016-status-locked-after-conversion.md) | Status terkunci setelah konversi |
| [`../03-owner-dashboard/design-brief.md`](../03-owner-dashboard/design-brief.md) | Brief dashboard pertama (sebagian basi) |
| [`../05-employee-mobile/design-brief.md`](../05-employee-mobile/design-brief.md) | Brief mobile pertama (sebagian basi) |
