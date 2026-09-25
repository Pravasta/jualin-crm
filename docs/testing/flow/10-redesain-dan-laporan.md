# 10 — Redesain responsif & Laporan

Prasyarat: [`00-menjalankan-aplikasi.md`](./00-menjalankan-aplikasi.md) dan
[`03-lead-dan-pipeline.md`](./03-lead-dan-pipeline.md) selesai. Butuh sesi Owner, **minimal satu anggota lain**, dan
beberapa lead dengan status berbeda (paling tidak satu Menang, satu Kalah dengan alasan, satu Spam). Laporan paling
berguna dengan **≥ 10 lead** (di bawah itu muncul pita "data sedikit", dan itu memang diuji di §10.5).

Menguji Phase 8.6: dashboard **terbaca dan bisa dipakai dari HP**, dan layar **Laporan** menampilkan angka yang
cocok dengan daftar lead. Perilaku layar lain tidak berubah; `03`–`09` tetap berlaku apa adanya.

## 10.1 Siapkan lebar layar

Chrome → DevTools → *Toggle device toolbar* (`⌘⇧M`). Uji **empat lebar** dengan tinggi berapa pun:

| Lebar | Mewakili |
|---|---|
| **360** | HP Android kecil (lebar minimum) |
| **390** | iPhone |
| **820** | Tablet |
| **1440** | Desktop |

**Aturan yang dipakai di setiap langkah:** halaman **tidak boleh** bisa digeser ke samping. Cara cepat mengeceknya: di
Console jalankan `document.documentElement.scrollWidth === innerWidth` → harus `true`. Satu-satunya yang boleh digeser
ke samping adalah **baris chip status** di daftar lead dan **tabel referensi di halaman dokumentasi** Connect, dan
keduanya digeser di dalam kotaknya sendiri.

## 10.2 Kerangka

| Lebar | Yang harus terlihat |
|---|---|
| 1440 | Sidebar dengan label, nama organization, identitas di bawah |
| 820 | Sidebar **ikon saja**; arahkan kursor ke ikon → label muncul sebagai tooltip; jumlah "tanpa pemilik aktif" menempel di ikon Lead |
| 360/390 | Tanpa sidebar. Header atas + **bilah bawah: Beranda · Lead · Tugas · Laporan · Lainnya**. "Lainnya" membuka sheet dari bawah berisi Customer, Tim, Connect, Langganan, Pengaturan, identitas, dan **Keluar** |

- [ ] Buka setiap item "Lainnya" → item **Lainnya** di bilah bawah ikut aktif
- [ ] Lonceng notifikasi di 360: panelnya tidak keluar layar

## 10.3 Daftar & detail di HP (360)

- [ ] **Lead:** kartu, bukan tabel. Chip status bisa digeser. **Filter** membuka sheet (sumber, pemilik, tanggal), dan
      tombolnya menampilkan jumlah filter aktif. Tombol **+** mengambang di atas bilah bawah dan tidak menutupi kartu
      terakhir
- [ ] **Lead baru** muncul sebagai sheet dari bawah. Kosongkan email+telepon lalu simpan → muncul **konfirmasi
      lunak**, bukan blokir
- [ ] **Detail lead:** satu kolom. Label stepper 5 tahap tidak saling menimpa. Tombol status setinggi ibu jari.
      Telepon panjang di header tidak terpotong
- [ ] **Customer, Tugas, Tim, Connect (API/Formulir/Webhook):** semuanya kartu. **Riwayat pengiriman webhook** tetap
      menampilkan pesan error lengkap
- [ ] **Tim → Nonaktifkan** anggota yang masih memegang lead terbuka → dialog **tiga cabang** muncul sebagai sheet
      (Lepas penugasan / Pindahkan / Batal), dan **Nonaktifkan** mati sampai satu cabang dipilih
- [ ] **Connect → API → Buat kunci baru** → kunci tampil **sekali**, dengan Salin dan peringatan. **Selesai** mati sampai
      kotak "Saya sudah menyimpan…" dicentang

## 10.4 Laporan — angka cocok dengan daftar

Buka **Laporan**, periode **30 hari terakhir**, semua anggota, semua sumber.

> ⚠️ **Selisih yang diketahui di tepi periode** (`docs/issues/174-date-bounds.md`). Daftar lead membaca tanggal
> filter sebagai hari **UTC**, sedangkan Laporan sebagai hari **lokal**. Lead yang masuk pukul **00:00–06:59 WIB** pada
> hari pertama atau hari setelah hari terakhir bisa terhitung di satu sisi saja. Untuk langkah di bawah, pakai data
> yang dibuat **siang hari**. Bila angkanya berbeda **hanya** karena lead di jam-jam itu, itu temuan yang sudah
> dicatat, bukan bug baru.

- [ ] **Ringkasan → Lead masuk** = angka "Menampilkan … dari **N** lead" setelah mengklik kartu itu
- [ ] Klik batang **Menang** di *Distribusi status* → daftar lead berisi tepat sebanyak angka batangnya
- [ ] *Distribusi status* **tidak** memakai kata "corong/funnel". Setiap batang berlabel badge status, jadi Spam dan Tidak
      Memenuhi Syarat terbedakan tanpa warna
- [ ] **Conversion rate** menuliskan pengecualian Spam & Tidak Memenuhi Syarat **di layar**
- [ ] **Tren:** arahkan kursor ke sebuah kolom → tooltip "N lead · tanggal". Tekan **Tab** sampai fokus mendarat di
      sebuah kolom → tooltip yang sama muncul tanpa mouse. **Lihat sebagai tabel** berisi semua hari, termasuk yang 0
- [ ] Pilih satu **anggota** di filter → semua blok berubah, dan *Performa anggota* hanya berisi anggota itu
- [ ] Pilih **sumber** Formulir → *Sumber lead* hanya Formulir yang bernilai
- [ ] **Tugas per anggota:** kolom bernama **"Terlambat sekarang"**, dan catatannya menyebut bahwa angka itu tidak mengikuti
      periode dan bahwa tugas tanpa penanggung jawab tidak dihitung
- [ ] Muat ulang halaman → filter tetap sama (tersimpan di URL)

## 10.5 Laporan — keadaan

- [ ] **Rentang kustom** dengan tanggal akhir sebelum tanggal mulai → kalimat "Tanggal akhir tidak boleh sebelum
      tanggal mulai.", dan tidak ada blok yang memuat
- [ ] Rentang kustom di masa depan (belum ada lead) → **"Belum ada lead pada periode ini"** dengan tombol ke Lead dan
      Connect, bukan grafik kosong
- [ ] Filter sumber yang tidak dipakai lead mana pun → **"Tidak ada lead yang cocok dengan filter ini"**, tanpa ajakan
- [ ] Periode dengan < 10 lead → pita **"Data masih sedikit"**, dan angka tetap tampil
- [ ] Hentikan API (`docker compose stop api`), ganti filter → setiap blok menampilkan pesan gagal dengan **Coba lagi**,
      dan halaman tidak kosong. Nyalakan lagi (`docker compose start api`) → **Coba lagi** memulihkan bloknya

## 10.6 Mobile Employee (HP fisik)

Dijalankan bersama sesi HP Android di [`07-mobile-android.md`](./07-mobile-android.md), di **360×800** dan jika ada
iPhone **393×852**, sebaiknya di luar ruangan:

- [ ] Badge status: **ikon + label** terbaca di bawah matahari. Tidak Memenuhi Syarat (⊖) dan Spam (⊘) terbedakan
- [ ] **Lead Saya:** kartu, dengan nama panjang tidak terjepit badge. Pilih chip yang tidak punya lead → "Tidak ada lead
      yang cocok" + **Hapus filter** mengembalikan daftar
- [ ] **Detail:** tombol **Ubah status** hanya menawarkan transisi yang sah. Pilih Kalah → langkah kedua wajib alasan
- [ ] Lead dengan nomor yang **tidak bisa** diformat internasional → WhatsApp mati **dengan kalimat penjelas**
- [ ] Telepon/WhatsApp tetap mencatat aktivitas setelah aplikasi lain terbuka (`07` §7.4, tidak berubah)
