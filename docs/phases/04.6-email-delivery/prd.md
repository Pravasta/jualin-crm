# Phase 4.6 — Email Delivery · PRD

> **Apa & kenapa.** Detail teknis di [`td.md`](./td.md).
> Sumber: [`architecture/freeze.md`](../../architecture/freeze.md) bagian 6 (efek samping di luar transaksi — Aturan #32), Aturan #26 (jangan log token), #34 (rate limit endpoint email), #36 (fail-fast) · [`architecture/authentication.md`](../../architecture/authentication.md) · [`docs/STATUS.md`](../../STATUS.md) bagian *Punya Lead Time* (domain + email sender)

---

## Kenapa phase ini ada

Pemilik produk ingin **demo ke calon pengguna** — tujuan yang sudah tercatat sejak Phase 3 tutup dan
diingatkan lagi di penutupan Phase 4 dan 4.5. Demo itu tidak bisa jalan dengan keadaan sekarang:

**Tidak ada satu pun email yang pernah benar-benar terkirim.** Sejak Phase 1, `MAIL_PROVIDER` hanya
punya satu nilai sah (`log`), dan `LogMailer` hanya mencatat pesan ke log aplikasi. Verifikasi email
menggerbangi login (keputusan B3) — artinya calon pengguna yang mencoba produk ini **tidak akan bisa
masuk sama sekali** kecuali seseorang membacakan tautan dari log server untuknya.

Dua hal lain yang ikut ketahuan saat menyiapkan phase ini:

- **`MAIL_FROM` tidak pernah dipakai.** Ia ada di `internal/shared/config` sejak Phase 1 dengan default
  `no-reply@localhost`, tapi tidak ada satu baris kode pun yang membacanya — `LogMailer` mengabaikan
  `From` sepenuhnya. Konfigurasi yang tidak pernah dibaca adalah janji yang belum ditepati; phase ini
  yang menepatinya.
- **`LogMailer` menulis token verifikasi ke log.** Wajar untuk pengembangan lokal, tapi berarti
  `MAIL_PROVIDER=log` di produksi menaruh kredensial sekali-pakai ke berkas log — bertentangan dengan
  semangat Aturan #26. Sekarang belum bisa terjadi karena `smtp` belum ada sebagai alternatif; begitu
  ada, kombinasi itu harus ditolak secara eksplisit.

## Yang diminta

Pemilik produk meminta secara spesifik: **SMTP**, dengan **Mailpit** untuk pengembangan, disusun
sedemikian rupa sehingga **ganti ke SMTP produksi nanti tidak menyentuh kode**.

Itu tepat, dan sudah didukung bentuk yang ada: `mailer.Mailer` sudah interface sejak Phase 1 —
komentarnya bahkan menyebut alasannya, bahwa implementasi kedua "sudah diketahui akan datang".
Phase ini adalah implementasi kedua itu. Tidak ada abstraksi baru yang perlu dibuat (Aturan #27–#29);
yang ada tinggal diisi.

**SMTP dipilih, bukan SDK provider tertentu** — dan itu justru yang membuat "ganti provider nanti"
menjadi perubahan konfigurasi, bukan perubahan kode. Resend, Postmark, dan SES ketiganya menyediakan
endpoint SMTP; memilih di antaranya nanti berarti mengganti `SMTP_HOST`/kredensial, bukan menulis
adapter baru. Keputusan provider (`docs/STATUS.md`'s *Keputusan Belum Diambil*) **tetap terbuka** —
phase ini justru membuatnya bisa ditunda tanpa biaya.

---

## Kebutuhan

| # | Sebagai… | Saya butuh… | Supaya… |
|---|---|---|---|
| 1 | Pemilik produk | Calon pengguna yang mencoba produk menerima email verifikasi sungguhan | Demo tidak berhenti di layar "cek email Anda" |
| 2 | Pemilik produk | Bisa melihat email yang terkirim saat mengembangkan, tanpa mengirim ke alamat sungguhan | Tidak ada risiko mengirim email uji ke orang nyata, dan tidak perlu inbox sungguhan untuk mengembangkan |
| 3 | Pemilik produk | Ganti ke SMTP produksi tanpa menyentuh kode | Keputusan provider bisa ditunda sampai benar-benar perlu, dan bisa diubah lagi kalau ternyata salah |
| 4 | Pemilik sistem | Proses gagal boot bila konfigurasi email tidak masuk akal untuk lingkungannya | Tidak ada produksi yang diam-diam tidak pernah mengirim email |
| 5 | Pemilik sistem | Kegagalan kirim tidak menggantung request atau membatalkan pendaftaran yang sudah tersimpan | Satu server SMTP lambat tidak menjatuhkan pendaftaran |
| 6 | Developer berikutnya | Bukti bahwa email benar-benar sampai, bukan hanya "fungsi Send dipanggil" | Jalur email tidak rusak diam-diam di refactor berikutnya |

---

## Acceptance Criteria

Phase 4.6 selesai bila **semuanya** terpenuhi:

| # | Kriteria |
|---|---|
| 1 | `MAIL_PROVIDER=smtp` mengirim email sungguhan lewat SMTP — dibuktikan dengan pesan yang **benar-benar sampai** dan bisa dibaca isinya, bukan dengan memeriksa bahwa `Send` dipanggil |
| 2 | `make dev` menyalakan Mailpit; email dari alur registrasi/undangan/reset password terlihat di UI-nya **tanpa** konfigurasi tambahan |
| 3 | Ganti ke SMTP lain (host, port, kredensial berbeda) **tidak** menyentuh satu baris kode Go pun |
| 4 | `MAIL_FROM` benar-benar dipakai sebagai alamat pengirim — dan berhenti menjadi konfigurasi mati |
| 5 | `MAIL_PROVIDER=log` saat `APP_ENV=production` **menghentikan boot** (Aturan #36) |
| 6 | `MAIL_PROVIDER=smtp` tanpa host/port yang sah **menghentikan boot**, bukan gagal saat pengguna pertama mendaftar |
| 7 | Server SMTP yang menggantung **tidak** menggantung request HTTP — ada batas waktu, dan kegagalannya tidak membatalkan pendaftaran yang sudah commit (Aturan #32) |
| 8 | Alamat penerima yang mengandung karakter kontrol **tidak** bisa menyuntikkan header email tambahan |
| 9 | Password SMTP **tidak pernah** muncul di log, termasuk saat pengiriman gagal (Aturan #26) |
| 10 | Subject dan body non-ASCII sampai utuh, tidak jadi karakter rusak |
| 11 | Seluruh test yang sudah ada tetap lulus **tanpa perubahan asersi** — `LogMailer` dan `spyMailer` di test tidak berubah perilakunya |
| 12 | `docs/testing/flow/` diperbarui — instruksi "cari tautan di log" diganti alur Mailpit, dan `06-checklist-akhir.md` ikut menyesuaikan |

---

## Keputusan yang ditutup di phase ini

| # | Pertanyaan | Keputusan | Alasan |
|---|---|---|---|
| **E1** | Pustaka SMTP atau `net/smtp` stdlib? | **`net/smtp` stdlib**, dengan koneksi dibangun manual (bukan `smtp.SendMail`). | Aturan #27 — jangan tambah dependensi untuk yang sudah ada di stdlib. `net/smtp` memang "frozen" di Go, tapi frozen berarti tidak menerima fitur baru, bukan rusak; kebutuhan di sini (plaintext + STARTTLS + PLAIN auth) persis yang sudah didukungnya. Koneksi dibangun manual karena `smtp.SendMail` **tidak punya batas waktu sama sekali** — dan `Send` dipanggil sinkron di jalur request (kriteria #7). |
| **E2** | Boleh `MAIL_PROVIDER=log` di produksi? | **Tidak. Boot gagal.** | Dua alasan yang masing-masing sudah cukup: (a) produksi yang tidak pernah mengirim email **tidak menghasilkan satu pun error** — funnel registrasi mati diam-diam, gejala yang sama persis dengan yang `STATUS.md` peringatkan soal email masuk spam; (b) `LogMailer` menulis **token verifikasi** ke log, dan token itu kredensial sekali-pakai (Aturan #26). |
| **E3** | Mode TLS apa yang didukung? | **STARTTLS** (port 587) dan **tanpa TLS** (dev saja). TLS implisit (port 465) **tidak** didukung dulu. | Ketiga kandidat provider di `freeze.md` bagian 7 — Resend, Postmark, SES — semuanya menyediakan 587/STARTTLS, jadi tidak ada yang terhalang. TLS implisit butuh jalur dial terpisah; menuliskannya sekarang berarti membangun untuk kebutuhan yang belum ada (Aturan #28). Tanpa-TLS ditolak saat `APP_ENV=production`. |
| **E4** | Email HTML atau plaintext? | **Plaintext saja**, `text/plain; charset=UTF-8`. | `mailer.Message` sudah berbentuk `To/Subject/Body` sejak Phase 1 dan tidak punya field HTML. Seluruh email di produk ini transaksional dan pendek (verifikasi, reset, undangan). Email campaign eksplisit **di luar scope** (`CLAUDE.md`). Menambah HTML sekarang berarti menambah templating untuk kebutuhan yang belum ada. |
| **E5** | Bagaimana jalur SMTP diuji? | **Mailpit sungguhan lewat testcontainers**, membaca kembali pesan yang masuk lewat API-nya — bukan mock, bukan server SMTP tiruan buatan sendiri. | Pola yang sama persis dengan harness PostgreSQL sejak issue #3: menguji terhadap barang sungguhan menangkap kelas kesalahan yang mock tidak akan pernah tangkap (format RFC 5322 salah, header hilang, encoding rusak). Kriteria #1 menuntut bukti "sampai dan terbaca", dan hanya server SMTP sungguhan yang bisa memberikannya. |

---

## Di luar cakupan

| Tidak dikerjakan | Kenapa |
|---|---|
| Memilih provider produksi (Resend / Postmark / SES) | Justru **dibuat bisa ditunda** oleh phase ini — ganti provider = ganti env. Tetap terbuka di `STATUS.md` |
| Domain, SPF, DKIM, DMARC | Pekerjaan DNS/administratif di luar repository. Tetap item *Punya Lead Time* di `STATUS.md`, dan tetap wajib sebelum email produksi sungguhan dipercaya |
| Email HTML, template, branding visual | Keputusan E4 |
| Tabel `jobs` / outbox / worker retry | Freeze bagian 6 memutuskan **kirim setelah commit tanpa `jobs`**, dan alasannya masih berlaku: jalur pemulihannya sudah ada (tombol kirim ulang) |
| Retry otomatis saat SMTP gagal | Sama — pemulihannya adalah tombol kirim ulang yang sudah ada sejak Phase 1, bukan mekanisme baru |
| Bounce/complaint handling, webhook provider | Butuh provider yang sudah dipilih. Belum dijadwalkan |
| Email campaign / marketing | **Di luar scope produk** (`CLAUDE.md`) |
| Mengubah bentuk `mailer.Message` atau isi teks email | Bukan pekerjaan phase ini. Isi email tidak disentuh sama sekali — hanya cara mengirimnya |

---

## Dependensi

| Bergantung pada | Sifat |
|---|---|
| Phase 1 (#9, #10, #11) | **Keras.** Seluruh alur email (verifikasi, reset, undangan) lahir di sana; phase ini hanya mengganti transportasinya |
| Pilihan provider produksi | **Tidak memblokir** — itu justru intinya |
| Domain + SPF/DKIM/DMARC | **Tidak memblokir pekerjaan ini**, tapi **memblokir email produksi yang bisa dipercaya**. Dicatat lagi di `STATUS.md` |

**Tidak memblokir Phase 5.** Dikerjakan sekarang karena demo ke calon pengguna — tujuan yang sudah
tertunda tiga penutupan phase — tidak bisa berjalan tanpanya.
