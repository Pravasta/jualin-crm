# 3 — Lead & Pipeline

Prasyarat: [`02-tim-dan-undangan.md`](./02-tim-dan-undangan.md) selesai — login sebagai
`owner@test.local`. Anggota `manager1@test.local` (Admin) masih **aktif**, sengaja belum
dinonaktifkan.

Menguji: buat lead manual, filter/pencarian, detail lead, edit, transisi status (jalur utama + kalah
+ buka kembali), penugasan, task, activity timeline. Ini layar dengan hampir seluruh aksi tulis
produk — luangkan waktu paling banyak di sini.

## 3.1 Buat lead manual

1. Buka **Lead** (`/leads`).
2. Klik tombol buat lead baru. Isi:
   - **Nama**: `Andi Calon Pelanggan`
   - **Email**: `andi@calon.test`
   - **Telepon**: `081234567890`
3. Simpan.

**Hasil yang diharapkan:** muncul di daftar dengan status **Baru**, sumber **Manual**, belum ada
pemilik. Buat **satu lead lagi** dengan data berbeda (mis. `Citra Calon Pelanggan`) — beberapa
berkas berikutnya butuh lebih dari satu lead untuk menguji filter/pencarian secara berarti.

## 3.2 Filter & pencarian

Di `/leads`:

1. Ketik sebagian nama (`Andi`) di kotak pencarian **"Cari nama, email, atau telepon…"**.

**Hasil yang diharapkan:** daftar menyempit ke lead yang cocok saja, sedikit tertunda (debounce),
tidak instan per huruf.

2. Bersihkan pencarian, coba filter **status** = Baru.

**Hasil yang diharapkan:** kedua lead yang baru dibuat muncul (keduanya masih status Baru).

3. Perhatikan URL browser berubah mengikuti filter (`?status=new&q=...`) — coba **refresh halaman**
   dengan URL itu: filter harus tetap sama setelah reload (bukan ke-reset).

## 3.3 Detail lead & edit

1. Klik salah satu lead (`Andi Calon Pelanggan`) untuk membuka detailnya.
2. Klik **Edit**, ubah **Perusahaan** jadi `PT Andi Jaya`, simpan.

**Hasil yang diharapkan:** perubahan tersimpan dan langsung terlihat di halaman detail.

## 3.4 Transisi status — jalur utama sampai Menang

Dari halaman detail `Andi Calon Pelanggan`. Area status di kartu atas (issue #141) punya tiga bagian:
**Tahapan lead** (lima titik: Baru → Dihubungi → Memenuhi Syarat → Penawaran → Menang, dengan penanda
**"Saat ini"** di tahap lead berada), lalu tombol yang **dikelompokkan dan berjudul** — **Lanjutkan**,
**Kembali**, **Tutup lead**. Bagian yang kosong **tidak muncul** (lead Baru tidak punya "Kembali"; lead
Menang tidak punya "Lanjutkan"). Nama tombolnya kata kerja — **"Maju ke …"** / **"Kembali ke …"** —
tanpa panah.

1. Ubah status **Baru → Dihubungi** (tombol **"Maju ke Dihubungi"**). Begitu berstatus Dihubungi,
   **jangan langsung lanjut**: periksa area status — titik **Baru** kini bercentang, **Dihubungi**
   bertanda "Saat ini", dan tombol yang ada hanya **"Maju ke Memenuhi Syarat"** (bagian Lanjutkan) dan
   Kalah / Tidak Memenuhi Syarat / Spam (bagian Tutup lead). **Tidak ada bagian "Kembali"** dan **tidak
   ada "Kembali ke Baru"** (issue #139, ADR-015). Sebuah lead yang sudah dihubungi tidak bisa kembali ke
   "belum disentuh".
2. **Dihubungi → Memenuhi Syarat**.
3. **Memenuhi Syarat → Penawaran**.
4. **Penawaran → Menang**.

**Hasil yang diharapkan tiap langkah:** berhasil, badge status berubah, dan **satu entri baru**
muncul di activity timeline (`status_changed`) mencatat dari→ke. Coba loncat status (mis. dari Baru
langsung ke Menang, kalau UI mengizinkan mengetik/memilihnya) — backend harus **menolak** lompatan
lebih dari satu langkah di jalur utama.

Lead ini (`Andi Calon Pelanggan`, sekarang **Menang**) dipakai lagi di
[`04-customer.md`](./04-customer.md) — jangan diubah statusnya lagi setelah titik ini.

## 3.5 Transisi status — Kalah + alasan, lalu buka kembali

Pada lead **kedua** (`Citra Calon Pelanggan`):

1. Ubah status ke **Kalah**.

**Hasil yang diharapkan:** diminta memilih **alasan kalah** (Harga / Kompetitor / Waktu Tidak Tepat /
Tidak Merespons / Tidak Tertarik / Lainnya) — **tidak bisa** disimpan tanpa memilih alasan.

2. Pilih alasan apa saja, simpan.

**Hasil yang diharapkan:** badge **Kalah**, dan detail menampilkan "Alasan kalah: ..." sesuai pilihan.

3. Cari opsi **"Buka kembali ke Dihubungi"** — satu-satunya opsi buka-kembali; **tidak ada**
   "Buka kembali ke Baru" (issue #139, ADR-015: `new` berarti "belum disentuh", dan lead yang pernah
   kalah tidak mungkin belum disentuh).

**Hasil yang diharapkan:** status menjadi **Dihubungi**, alasan kalah tidak lagi ditampilkan (atau
ditampilkan sebagai riwayat lama di timeline, bukan status aktif).

4. Pada lead yang sama (kini **Dihubungi**), periksa tombol status yang tersedia.

**Hasil yang diharapkan (issue #139, #141):** bagian **Lanjutkan** hanya berisi **"Maju ke Memenuhi
Syarat"**, bagian **Tutup lead** berisi Kalah / Tidak Memenuhi Syarat / Spam, dan **tidak ada bagian
"Kembali"** sama sekali (bukan bagian kosong — bagiannya tidak muncul). Sebelum ADR-015, tombol "→ Baru"
muncul di sini dan mengembalikan lead yang sudah dihubungi ke status "belum disentuh" — padahal
timeline-nya sudah memuat perubahan status.

Pada lead **Kalah** (langkah 1–2), area status berbeda: semua titik tahapan **redup** (lead yang ditutup
tidak punya posisi), ada pita **"Lead ditutup: Kalah"**, dan satu-satunya bagian adalah **Buka kembali**
dengan tombol **"Buka kembali ke Dihubungi"**. Untuk **Spam** / **Tidak Memenuhi Syarat**: pita "Lead
ditutup: …", kalimat *"Status ini bersifat final."*, dan **tidak ada tombol**.

**Periksa juga tanpa mengandalkan warna** (issue #141): tahap yang sudah lewat bertanda **centang**, tahap
sekarang **lingkaran ganda + kata "Saat ini"**, tahap berikutnya **lingkaran kosong**. Kecilkan lebar
jendela browser sampai sempit — label "Memenuhi Syarat" harus **membungkus**, tidak meluap keluar kartu.

## 3.6 Konflik versi (optimistic locking)

1. Buka detail `Citra Calon Pelanggan` di **dua tab browser** sekaligus.
2. Di tab pertama, ubah statusnya (mis. ke Memenuhi Syarat — lead ini sudah Dihubungi sejak §3.5),
   simpan — berhasil.
3. Di tab **kedua** (yang belum di-refresh, masih memegang data lama), coba ubah field apa saja dan
   simpan.

**Hasil yang diharapkan:** ditolak dengan pesan konflik (bukan menimpa diam-diam), dengan tombol
**Muat ulang**. Klik tombol itu — data terbaru dari tab pertama muncul, tab kedua **tidak**
otomatis menerapkan perubahannya sendiri di atasnya.

## 3.7 Penugasan (assignment)

1. Buka detail `Citra Calon Pelanggan`.
2. Di dropdown penugasan, pilih **`manager1@test.local`**.

**Hasil yang diharapkan:** tersimpan, muncul entri `lead_assigned` di timeline. Login sebagai
`manager1@test.local` di jendela terpisah — **lonceng notifikasi** menunjukkan notifikasi baru
(lihat [`02-tim-dan-undangan.md`](./02-tim-dan-undangan.md) §2.6).

3. **Kembali ke `/team` sebagai Owner.** Sekarang coba **Nonaktifkan** `manager1@test.local` — anggota
   ini sekarang **punya** lead terbuka yang ditugaskan padanya.

**Hasil yang diharapkan:** **ditolak** dulu (`409`), dialog muncul dengan dua pilihan — **lepas
penugasan** atau **pindahkan ke anggota lain**. Pilih salah satu, konfirmasi.

**Hasil yang diharapkan:** berhasil nonaktif, dan lead `Citra Calon Pelanggan` sesuai pilihan tadi
(kosong pemiliknya, atau berpindah ke anggota lain) — cek langsung di halaman detail leadnya.

## 3.8 Task

Pada lead `Andi Calon Pelanggan` (masih status Menang dari §3.4):

1. Buat task baru: **Judul** `Follow-up kontrak`, **Jatuh tempo** besok, **Penanggung jawab**: diri
   sendiri (Owner) atau anggota aktif mana pun.
2. Simpan — muncul di daftar task lead ini, dan entri `task_created` di timeline.
3. Tandai task **selesai** (checkbox).

**Hasil yang diharapkan:** status task berubah jadi selesai, entri `task_completed` muncul di
timeline. Coba centang lagi (klik dua kali) — task **tidak** bisa dibuka kembali lewat checkbox
(satu arah, buka→selesai saja).

4. Buka **Tugas** (`/tasks`) dari menu utama — task yang baru dibuat harus muncul di sana juga, bisa
   difilter per penanggung jawab dan tanggal jatuh tempo.

## 3.9 Activity timeline — tinjau ulang

Di halaman detail `Andi Calon Pelanggan`, timeline seharusnya sekarang berisi (urut waktu): lead
dibuat → beberapa `status_changed` → `task_created` → `task_completed`. Pastikan setiap entri
menampilkan kalimat yang masuk akal dalam Bahasa Indonesia — **bukan** kode/enum mentah seperti
`status_changed` tertulis apa adanya di layar.

---

Selesai di sini: `Andi Calon Pelanggan` — status **Menang**, satu task selesai. `Citra Calon
Pelanggan` — status **Memenuhi Syarat** (dari §3.6), sempat ditugaskan lalu dipindahkan/dilepas di §3.7.
Lanjut ke [`04-customer.md`](./04-customer.md).
