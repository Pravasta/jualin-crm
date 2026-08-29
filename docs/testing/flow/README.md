# Testing Manual — Alur

Panduan langkah-demi-langkah untuk **manusia** mengklik-klik seluruh aplikasi dari nol sampai
menjalankan skenario inti produk. Ini **bukan** pengganti test otomatis (`make test` / `npm run test`
di `crm_dashboard`) — test otomatis membuktikan kode benar; panduan ini membuktikan **rangkaian
layar sungguhan** enak dipakai dan tidak ada yang patah saat dijalankan sebagai satu alur utuh,
sesuatu yang sulit dilihat dari test per-fitur.

## Cakupan

Mengikuti kalimat inti MVP (`architecture/freeze.md` bagian 3, `product/decisions.md` §23):

> Owner mendaftar → verifikasi email → login → undang employee → employee login →
> owner membuat API key → website mengirim lead → owner meng-assign →
> employee menerima notifikasi → follow-up dari HP → update → konversi ke Customer

Bagian yang **bisa** diuji manual sekarang: semuanya kecuali "follow-up dari HP" — **Phase 5 (mobile)
belum dibangun**, jadi employee follow-up di panduan ini dilakukan lewat dashboard, bukan HP.
Phase 0–4.6 sudah selesai (`docs/STATUS.md`); panduan ini menguji hasilnya.

## Urutan berkas

| # | Berkas | Apa yang diuji |
|---|---|---|
| 0 | [`00-menjalankan-aplikasi.md`](./00-menjalankan-aplikasi.md) | Backend + dashboard menyala, bisa diakses browser |
| 1 | [`01-registrasi-dan-autentikasi.md`](./01-registrasi-dan-autentikasi.md) | Daftar org baru, verifikasi email, login, lupa/reset password |
| 2 | [`02-tim-dan-undangan.md`](./02-tim-dan-undangan.md) | Undang anggota, terima undangan, ganti role, nonaktifkan |
| 3 | [`03-lead-dan-pipeline.md`](./03-lead-dan-pipeline.md) | Buat lead, ubah status, tugaskan, task, activity timeline |
| 4 | [`04-customer.md`](./04-customer.md) | Konversi lead menang jadi Customer, edit Customer |
| 5 | [`05-api-publik.md`](./05-api-publik.md) | Buat API key, kirim lead lewat `curl`, cabut kunci |
| 6 | [`06-checklist-akhir.md`](./06-checklist-akhir.md) | Rekap satu halaman — centang semua sebelum bilang "beres" |

Kerjakan berurutan — setiap berkas mengasumsikan data dari berkas sebelumnya masih ada (org yang sama,
lead yang sama, dst).

## Yang perlu disiapkan dulu

- Docker Desktop (atau daemon Docker apa pun) menyala
- Node.js 20+ dan `npm`
- Terminal yang bisa dibuka beberapa tab/jendela sekaligus (untuk mengawasi log sambil mengklik di browser)
- Browser apa saja

## Soal email — Mailpit

Belum ada penyedia email produksi (`docs/STATUS.md`'s *Punya Lead Time* — domain & SPF/DKIM/DMARC
belum diurus), tapi sejak Phase 4.6 **email sungguhan benar-benar terkirim** saat mengembangkan —
lewat SMTP ke [Mailpit](https://mailpit.axllent.org/), server SMTP lokal yang ditangkap
`docker-compose.yml`, bukan dikirim ke internet. Setiap email (verifikasi, reset password, undangan)
muncul di **UI web Mailpit** (`http://localhost:8025`), lengkap dengan tautan yang bisa langsung
diklik — tidak perlu lagi menggali log server. Caranya dijelaskan di
[`00-menjalankan-aplikasi.md`](./00-menjalankan-aplikasi.md) §7 dan dipakai berulang di
berkas-berkas berikutnya.

## Kalau sesuatu gagal

Jangan diam-diam dilewati. Catat langkah mana yang gagal, pesan error persisnya, dan cek dulu:

1. Apakah langkah di `00-menjalankan-aplikasi.md` benar-benar semuanya lolos?
2. Apakah data dari berkas sebelumnya masih ada (mis. belum tidak sengaja `docker compose down -v`)?
3. Apakah ini regresi nyata, atau memang di luar cakupan yang tertulis di `docs/phases/*/prd.md`
   bagian *Di luar cakupan*?

Bug nyata yang ditemukan selama testing manual dicatat sebagai issue GitHub baru — bukan diperbaiki
diam-diam di sesi yang sama, ikuti workflow biasa (`docs/workflow.md`).
