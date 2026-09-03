# 8 — Formulir Embed

Prasyarat: [`01-registrasi-dan-autentikasi.md`](./01-registrasi-dan-autentikasi.md) dan
[`02-tim-dan-undangan.md`](./02-tim-dan-undangan.md) selesai — butuh sesi Owner dan satu Employee
aktif untuk menguji gerbang role. **Tidak** bergantung pada `07` (mobile).

Menguji: **capture layer tanpa developer** (Phase 6). Kriteria yang paling mudah lolos di atas kertas
tapi paling sering gagal di lapangan — *"salin satu potong HTML, tempel, selesai"*. AC #1 dan #10 PRD
hanya bisa dibuktikan dengan **benar-benar menempel snippet ke halaman kosong lalu mengisinya dari
browser**, bukan dengan membaca snippet-nya.

Butuh tambahan: cara menyajikan satu berkas HTML statis di `http://localhost` — mis.
`python3 -m http.server 9099` di folder mana pun. Snippet embed hanya bekerja dari domain yang ada di
allowlist form, jadi halaman ujinya harus benar-benar disajikan dari sebuah origin, bukan dibuka
sebagai `file://`.

> `EMBED_BASE_URL` di dashboard (`NEXT_PUBLIC_EMBED_BASE_URL`, default `http://localhost:8080`)
> menentukan URL di dalam snippet. Halaman embed disajikan oleh `crm_be` sendiri hari ini
> (`GET /embed/{public_key}`) — belum ada domain terpisah (ADR-005 D1, dikerjakan saat deployment).

## 8.1 Menu Connect → Formulir

1. Login sebagai `owner@test.local`. Klik menu **Connect** di sidebar.

**Hasil yang diharapkan:** tiga kartu kanal — **API**, **Formulir**, dan **Webhook**, ketiganya
aktif dan bisa diklik sejak Phase 7 (#103). Tidak ada yang berbunyi *"terkunci oleh paket"* — keadaan
itu baru lahir di Phase 8. Webhook diuji terpisah di [`09-webhook.md`](./09-webhook.md).

2. Klik kartu **Formulir**.

**Hasil yang diharapkan:** halaman `/connect/form`, daftar formulir kosong dengan ajakan membuat yang
pertama, dan tombol **+ Buat formulir**.

## 8.2 Buat formulir

1. Klik **+ Buat formulir**. **Nama:** `Formulir Kontak Website`. Submit.

**Hasil yang diharapkan:** langsung diarahkan ke editor formulir (`/connect/form/{id}`). Enam field
sudah terisi label Indonesia default (Nama Lengkap, Email, Nomor WhatsApp, Perusahaan, Pesan, Layanan
Diminati); Nama dan Nomor WhatsApp aktif + wajib, Perusahaan dan Layanan Diminati nonaktif.

## 8.3 Ubah label mengikuti bahasa bisnis

1. Ubah label **Layanan Diminati** → `Layanan yang Anda cari` dan aktifkan field-nya.
2. Ubah label **Pesan** → `Pertanyaan Anda`.
3. Coba tandai **Perusahaan** sebagai *Wajib diisi* tanpa mengaktifkannya.

**Hasil yang diharapkan:** mengaktifkan "Wajib diisi" ikut mengaktifkan field-nya (dua hal tidak bisa
lepas — backend menolak kombinasi "wajib tapi nonaktif"). Kosongkan salah satu label field yang aktif
lalu **Simpan perubahan** → pesan merah di bawah field itu, simpan dibatalkan.

4. Kembalikan semua label yang valid, klik **Simpan perubahan**.

**Hasil yang diharapkan:** penanda **Tersimpan** muncul.

## 8.4 Atur domain yang diizinkan

1. Di **Domain yang diizinkan**, ketik `localhost` saja → **Tambah**.

**Hasil yang diharapkan:** ditolak dengan petunjuk — harus alamat lengkap `https://…` tanpa path.

2. Ketik `http://localhost:9099` → **Tambah**. Klik **Simpan perubahan**.

**Hasil yang diharapkan:** `http://localhost:9099` muncul sebagai baris dengan tombol **Hapus**;
**Tersimpan**.

## 8.5 Salin snippet dan tempel ke halaman kosong — **inti AC #10**

1. Di bagian **Snippet embed**, klik **Salin** pada varian **Dianjurkan**.
2. Buat berkas `index.html` di folder kosong, isinya **hanya** ini (`<body>` selain snippet boleh apa
   saja):

   ```html
   <!doctype html>
   <html lang="id"><head><meta charset="utf-8"><title>Situs Uji</title></head>
   <body>
   <h1>Hubungi kami</h1>
   <!-- TEMPEL SNIPPET DI SINI -->
   </body></html>
   ```

3. Sajikan folder itu: `python3 -m http.server 9099`. Buka `http://localhost:9099/` di browser.

**Hasil yang diharapkan:**

- Formulir muncul di dalam iframe, dengan label yang **persis** seperti yang diubah di §8.3
  (`Layanan yang Anda cari`, `Pertanyaan Anda`) — bukan label default.
- Field **Perusahaan** (nonaktif) **tidak** muncul.
- Tinggi iframe menyesuaikan isi formulir (script `embed.js` jalan) — tidak ada ruang kosong besar di
  bawahnya, tidak ada scrollbar dalam iframe.

## 8.6 Isi formulir dari browser → lead masuk — **inti AC #1**

1. Di formulir yang tampil di `http://localhost:9099/`, isi Nama, Email, Nomor WhatsApp, dan
   pertanyaan. **Tunggu minimal 2–3 detik** sebelum menekan kirim (time-trap menolak submit <2 detik).
2. Kirim.

**Hasil yang diharapkan:** pesan sukses di dalam iframe.

3. Buka dashboard → **Lead**. Filter sumber **Formulir**.

**Hasil yang diharapkan:** lead baru dengan data yang tadi diisi, **Sumber: Formulir**,
`assigned_to_membership_id` kosong (belum ditugaskan), `created_by_membership_id` kosong (bukan dibuat
anggota). Buka detailnya — pertanyaan tadi muncul sebagai catatan.

4. Kembali ke editor formulir (`/connect/form/{id}`).

**Hasil yang diharapkan:** angka **Submission** di daftar formulir bertambah jadi `1`.

## 8.7 Domain di luar allowlist ditolak

1. Sajikan `index.html` yang sama dari port lain: `python3 -m http.server 9098`. Buka
   `http://localhost:9098/`.

**Hasil yang diharapkan:** iframe **tidak** menampilkan formulir yang bisa dipakai — browser menolak
menyematkannya (`frame-ancestors` hanya mengizinkan `http://localhost:9099`). Kalaupun formulirnya
sempat tampil, submit dari origin ini ditolak (`403 origin_not_allowed`) dan tidak ada lead baru.

## 8.8 Varian tanpa script & varian JSX

1. Di editor, salin varian **Tanpa script**, ganti snippet di `index.html`, muat ulang
   `http://localhost:9099/`.

**Hasil yang diharapkan:** formulir tetap tampil dan tetap bisa dikirim — hanya tingginya tetap
(`height="620"`), tidak menyesuaikan isi.

2. Klik **Memakai React / Next.js? Lihat varian JSX**.

**Hasil yang diharapkan:** kotak ketiga muncul dengan `style={{ border: 0 }}` dan `height={620}` —
sintaks JSX, bukan atribut HTML.

## 8.9 Gerbang role — nol panggilan API

1. Login sebagai Employee (dari `02`). Buka `/connect` — kartu **Formulir** tetap terlihat (nav tidak
   difilter role, keputusan D6). Klik kartu itu, atau ketik `/connect/form` langsung di URL.

**Hasil yang diharapkan:** pesan *"Pengelolaan formulir tidak tersedia untuk role Anda."* Buka
DevTools → Network, muat ulang: **tidak ada** panggilan ke `/v1/forms` sama sekali — gerbangnya di
atas fetch, bukan sekadar menyembunyikan tombol (pola sama seperti `/connect/api` untuk Employee).

2. Ketik `/connect/form/{id}` (id form dari §8.2) langsung di URL sebagai Employee.

**Hasil yang diharapkan:** pesan yang sama, tetap nol panggilan API.

## 8.10 Nonaktifkan formulir

1. Login lagi sebagai Owner. Buka editor formulir → **Nonaktifkan formulir** → konfirmasi.

**Hasil yang diharapkan:** kembali ke daftar formulir, formulir tadi **hilang** dari daftar.

2. Muat ulang `http://localhost:9099/`.

**Hasil yang diharapkan:** iframe sekarang `404` — formulir yang dinonaktifkan tidak bisa dibedakan
dari yang tidak pernah ada.

3. Lead yang sudah masuk sebelum nonaktif (§8.6) **tetap ada** di dashboard — menonaktifkan formulir
   tidak menghapus lead yang sudah ditangkapnya.
