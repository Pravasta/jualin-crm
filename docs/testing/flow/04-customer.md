# 4 — Customer

Prasyarat: [`03-lead-dan-pipeline.md`](./03-lead-dan-pipeline.md) selesai — lead `Andi Calon
Pelanggan` berstatus **Menang**, login sebagai `owner@test.local`.

Menguji: konversi lead → customer, halaman detail customer, edit customer, dan bukti bahwa lead
sumbernya tidak ikut berubah saat customer-nya diedit.

## 4.1 Konversi lead yang menang

1. Buka detail lead `Andi Calon Pelanggan` (masih status **Menang**).
2. Klik **Konversi menjadi customer**.

**Hasil yang diharapkan:** berhasil, diarahkan (atau muncul tautan) ke halaman Customer baru. Kembali
ke detail lead ini — tombol Konversi **sudah hilang** (tidak bisa dikonversi dua kali), dan timeline-nya
punya entri konversi.

3. Coba konversi lead yang **belum** menang (mis. `Citra Calon Pelanggan`, masih Dihubungi) — cek
   apakah tombol Konversi memang tidak muncul/tidak bisa diklik untuk lead berstatus selain Menang.

## 4.2 Halaman detail customer

1. Buka **Customer** (`/customers`) dari menu — pastikan customer hasil konversi tadi muncul di
   daftar.
2. Klik untuk membuka detailnya (`/customers/{id}`).

**Hasil yang diharapkan:** data (nama, email, telepon) sama dengan lead asalnya. Ada tautan **"Berasal
dari lead"** — klik, harus membawa balik ke `/leads/{id}` lead `Andi Calon Pelanggan`.

## 4.3 Edit customer — dan buktikan lead tidak ikut berubah

1. Di halaman detail customer, klik **Ubah customer**.
2. Ubah **Nama** jadi `Andi Sudah Jadi Pelanggan`, simpan.

**Hasil yang diharapkan:** nama customer berubah.

3. **Buka lagi lead asalnya** (`Andi Calon Pelanggan`, lewat tautan "Berasal dari lead" atau langsung
   dari `/leads`).

**Hasil yang diharapkan — ini bagian pentingnya:** nama di lead **tetap** `Andi Calon Pelanggan`,
**tidak** ikut berubah jadi nama customer yang baru. Konversi menyalin data sekali saat itu terjadi;
setelahnya lead dan customer adalah dua record independen.

## 4.4 Akses Manager (RBAC baca-tulis)

`manager1@test.local` tidak bisa dipakai untuk uji ini — perannya sudah **Admin** (dipromosikan di
[`02-tim-dan-undangan.md`](./02-tim-dan-undangan.md) §2.4) dan sudah **dinonaktifkan** di
[`03-lead-dan-pipeline.md`](./03-lead-dan-pipeline.md) §3.7. Undang satu anggota **baru** dengan
role **Manager** khusus untuk langkah ini (mis. `manager2@test.local`), selesaikan penerimaan
undangannya seperti di §2.2, lalu login sebagai dia.

**Hasil yang diharapkan:** Manager bisa **membaca** daftar dan detail customer (`200`), tapi
tombol **Ubah customer**/**Hapus** tidak tersedia atau ditolak (`403`) — RBAC customer adalah
Owner/Admin untuk tulis, siapa saja yang berhak melihat untuk baca.

---

Selesai di sini: satu Customer (`Andi Sudah Jadi Pelanggan`) hasil konversi, terverifikasi tidak
memutasi lead asalnya. Lanjut ke [`05-api-publik.md`](./05-api-publik.md).
