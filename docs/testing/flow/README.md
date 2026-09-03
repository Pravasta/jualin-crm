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

**Sejak Phase 5 (#73) selesai, seluruh kalimat inti bisa diuji apa adanya** — termasuk "follow-up
dari HP" di [`07-mobile-android.md`](./07-mobile-android.md), yang sebelumnya dilakukan lewat
dashboard sebagai gantinya. Phase 0–5 sudah selesai (`docs/STATUS.md`); panduan ini menguji hasilnya.

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
| 7 | [`07-mobile-android.md`](./07-mobile-android.md) | Login+biometric, mode pesawat, telepon/WhatsApp dengan auto-Activity, ubah status, catatan, Tugas Saya, push+deeplink (3 keadaan), kehilangan akses, uninstall — **butuh HP Android fisik** |
| 8 | [`08-formulir-embed.md`](./08-formulir-embed.md) | Buat & kelola formulir dari dashboard, ubah label, allowlist domain, **tempel snippet ke halaman HTML kosong lalu isi dari browser** → lead masuk, domain di luar allowlist ditolak, gerbang role, nonaktifkan (Phase 6) |
| 9 | [`09-webhook.md`](./09-webhook.md) | Daftarkan endpoint dari dashboard, **jalankan server penerima lokal**, buat lead → request sungguhan sampai, **verifikasi signature dari sisi penerima dengan contoh dari halaman docs**, payload diubah satu byte → ditolak, endpoint mati → retry + kirim ulang manual, URL privat ditolak (SSRF), gerbang role (Phase 7) |

Kerjakan berurutan — setiap berkas mengasumsikan data dari berkas sebelumnya masih ada (org yang sama,
lead yang sama, dst). `07` dikerjakan setelah `03` (butuh lead untuk ditugaskan) dan `02` (butuh
employee aktif) — sebelum `06` (rekap akhir). `08` (Phase 6, di luar kalimat inti MVP) butuh sesi
Owner dan satu Employee aktif (`01` + `02`); ia **tidak** bergantung pada `07`, jadi bisa dikerjakan
tanpa HP Android. `09` (Phase 7, di luar kalimat inti MVP) butuh `01`–`03` dan satu server penerima
lokal (`python3`); **tidak** bergantung pada `07` maupun `08`.

## Yang perlu disiapkan dulu

- Docker Desktop (atau daemon Docker apa pun) menyala
- Node.js 20+ dan `npm`
- Terminal yang bisa dibuka beberapa tab/jendela sekaligus (untuk mengawasi log sambil mengklik di browser)
- Browser apa saja
- Untuk `07-mobile-android.md`: HP Android fisik, FVM Flutter (`crm_employee/.fvmrc`), Firebase
  project sudah dikonfigurasi (`flutterfire configure`) — detail lengkap di berkas itu sendiri

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
