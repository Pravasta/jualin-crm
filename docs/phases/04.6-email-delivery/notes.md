# Phase 4.6 — Email Delivery · Notes

One section per issue, appended as each is implemented.

---

## #63 — `SMTPMailer`: kirim email sungguhan, tutup `MAIL_FROM` yang tidak pernah dipakai

### Keputusan implementasi

- **`buildRFC5322` dipisah sebagai fungsi murni yang tidak diekspor** — `SMTPMailer.Send` satu-satunya
  pemanggil. Karena tidak diekspor, test yang menyentuhnya langsung harus berada di `package mailer`
  (internal), bukan `mailer_test` — deviasi kecil yang disengaja, pola yang sama seperti
  `internal/apikey/entity_test.go` (#46) dan `ratelimit/limiter_internal_test.go` (#58). Berkasnya
  diberi nama `message_internal_test.go`, bukan `message_test.go` seperti disebut TD §3 — supaya
  perbedaan konvensinya terlihat dari nama berkas, bukan hanya isi paketnya.
- **Percakapan SMTP dibangun manual, bukan `smtp.SendMail`** — persis seperti TD §4 memutuskan.
  `smtp.SendMail` memakai `net.Dial` telanjang tanpa batas waktu sama sekali; `Send` dipanggil sinkron
  di jalur request (`auth.Usecase.sendVerificationEmail` dkk.), jadi server yang menggantung akan
  menggantung request HTTP. Satu `conn.SetDeadline` menutup seluruh percakapan sejak dial sampai QUIT.
- **`smtp.PlainAuth` dilewati sepenuhnya bila `Username`/`Password` kosong** — bukan dipanggil dengan
  kredensial kosong. Mailpit tidak memakai auth sama sekali; memanggil `c.Auth` terhadap server yang
  tidak mengiklankan `AUTH` akan gagal tanpa alasan yang jelas.
- **Error dari `c.Auth` gagal tidak pernah dibungkus** — dikembalikan sebagai
  `errors.New("mailer: smtp authentication failed")` polos. `net/smtp` bisa meng-echo balik detail dari
  respons server saat auth gagal; membungkusnya dengan `%w` berisiko membocorkan sesuatu yang berasal
  dari upaya autentikasi (Aturan #26).
- **`validMailProviders` bertambah `"smtp"`, dan `MAIL_PROVIDER=log` ditolak saat `APP_ENV=production`**
  (keputusan E2) — dua validasi terpisah, bukan satu. Yang pertama membolehkan nilainya ada; yang kedua
  melarang nilai tertentu di lingkungan tertentu. Digabung jadi satu kondisi akan membuat pesan error
  yang kabur soal *kenapa* ditolak.
- **`SMTP_TLS=none` ditolak saat `APP_ENV=production`**, terlepas dari nilai lain — kredensial dan token
  verifikasi tidak boleh melintasi jaringan produksi tanpa enkripsi (keputusan E3).

### Perilaku stdlib diverifikasi sebelum ditulis ke kode — dan hasilnya cocok dengan TD

Dua asumsi TD §5 diuji langsung sebelum implementasi ditulis, bukan diasumsikan dari dokumentasi:

```
mime.QEncoding.Encode("utf-8", "Verifikasi email Jualin CRM Anda")
  → "Verifikasi email Jualin CRM Anda"           (tidak berubah)
mime.QEncoding.Encode("utf-8", "Sudah — kami kirim ulang")
  → "=?utf-8?q?Sudah_=E2=80=94_kami_kirim_ulang?="

strings.ContainsAny("bad@example.com\r\nBcc: evil@x.com", "\r\n")  → true
strings.ContainsAny("bad@example.com\nX: y", "\r\n")               → true
```

Keduanya cocok persis dengan yang TD nyatakan — tidak ada penyesuaian desain yang diperlukan setelah
verifikasi.

### API Mailpit — diperiksa langsung sebelum menulis `smtp_test.go`

TD tidak menetapkan bentuk response Mailpit secara rinci karena memang harus diperiksa terhadap
Mailpit sungguhan, bukan diasumsikan dari dokumentasinya. Dikonfirmasi dengan mengirim pesan uji lewat
`python3 -m smtplib` ke Mailpit lokal lalu membaca `GET /api/v1/messages` dan
`GET /api/v1/message/{id}`:

- `messages[].From.Address`, `messages[].To[].Address`, `messages[].Subject` — cukup untuk memverifikasi
  pengirim/penerima/subjek tanpa mengambil detail penuh
- `message/{id}.Text` — body plaintext, **subject RFC 2047 sudah didekode balik** oleh Mailpit sendiri
  (`Sudah — kami kirim ulang` datang utuh, bukan bentuk `=?utf-8?q?...?=` mentah) — ini yang membuat
  `TestSMTPMailer_NonASCIISubjectArrivesIntact` bisa membandingkan string apa adanya, bukan mem-parse
  encoding secara manual di sisi test

### Test yang membuktikan batas waktu — diukur, bukan dibaca

`TestSMTPMailer_UnreachableServerFailsWithinTimeout` memakai `net.Listener` lokal yang menerima
koneksi lalu tidak pernah menulis satu byte pun — mensimulasikan server yang berhenti merespons
di tengah percakapan tanpa perlu jaringan yang benar-benar diblackhole. `SMTPTimeout` diset 500ms;
`Send` dibuktikan gagal dalam hitungan detik (bukan menggantung sampai OS TCP timeout, yang bisa
menit).

### Verifikasi — terbukti bisa gagal

Prosedur sama seperti #57/#58 — dua titik dirusak sementara, dijalankan, dicatat, dikembalikan:

```
message.go — rejectHeaderInjection dipaksa `return nil` (defense dimatikan):
--- FAIL: TestBuildRFC5322_RejectsHeaderInjectionInTo
    ketiga bentuk (\r\n, \n, \r) tidak lagi ditolak
--- FAIL: TestBuildRFC5322_RejectsHeaderInjectionInSubjectAndFrom
    Subject dan From ikut tidak ditolak

smtp.go — conn.SetDeadline dihapus (batas waktu percakapan dimatikan):
--- FAIL: TestSMTPMailer_UnreachableServerFailsWithinTimeout
    test digantung sampai dipotong paksa oleh `go test -timeout=15s` —
    tanpa SetDeadline, Send menggantung tanpa batas (dibuktikan, bukan
    diasumsikan: goroutine dump menunjukkan blok di net.(*TCPListener).Accept
    lewat pembacaan client, persis titik yang deadline seharusnya memotong)
```

Ketiga kali merah persis seperti diprediksi. Tidak satu pun perusakan ter-commit — `git diff` bersih
setelah setiap pengembalian.

### Verifikasi

- `go test -race ./...` — seluruh paket lulus, termasuk 10 test baru di `internal/shared/mailer`
  (6 unit tanpa jaringan + 4 integrasi Mailpit) dan 6 test baru di `internal/shared/config`, tanpa
  perubahan asersi pada test lama manapun.
- `go build ./...`, `go vet ./...`, `gofmt -l .` — bersih.
- `go mod tidy` — `testcontainers-go` (core, bukan modul `postgres`) naik dari `// indirect` jadi
  dependency langsung, karena `smtp_test.go` sekarang mengimpornya langsung. Tidak ada dependency baru
  yang ditambahkan — sudah ada di `go.sum` sejak `dbtest` menariknya secara transitif.
- Test produksi yang sudah ada (`TestLoad_ProductionMode`, `…RequiresCookieSecure`,
  `…RequiresCORSAllowedOrigins`, `…RequiresTrustedProxies`) ditambahi `MAIL_PROVIDER=smtp` +
  `SMTP_HOST` di setup-nya masing-masing — penambahan setup, **bukan** perubahan asersi, konsekuensi
  wajar dari menambah aturan validasi produksi baru (pola yang sama seperti saat `TRUSTED_PROXIES`
  ditambahkan di #57).

### Batas — bukan penutup phase

Issue ini membuat SMTP **bisa** dipakai dan **terbukti** mengirim ke Mailpit sungguhan di test. Tapi
`make dev` **belum** menyalakan Mailpit, dan `docs/testing/flow/` masih menyuruh penguji menggali
tautan dari log `api`. Itu #64, penutup phase.

---

## #64 — Mailpit di dev environment + perbarui alur testing, penutup Phase 4.6

### Keputusan implementasi

- **Tanpa `depends_on: mailpit` di service `api`** — persis seperti TD §8 memutuskan. SMTP hanya
  disentuh saat ada email yang benar-benar dikirim, bukan saat boot; menunggu Mailpit sehat sebelum
  `api` boleh menyajikan traffic hanya memperlambat `docker compose up` tanpa manfaat nyata. Diverifikasi
  langsung: registrasi segera setelah `docker compose up --build -d` (tanpa jeda) tetap `201` — Mailpit
  yang tanpa healthcheck sudah siap menerima koneksi jauh lebih cepat daripada Postgres yang punya
  healthcheck `interval: 2s, retries: 15`.
- **`.env.example`'s `SMTP_HOST=localhost`, bukan `mailpit`** — berbeda sengaja dari nilai
  `docker-compose.yml`'s `SMTP_HOST=mailpit`. `.env.example` dipakai proses yang jalan **di host**
  (`cmd/migrate` lewat `make migrate-up`, dan siapa pun yang menjalankan `cmd/api` secara native tanpa
  Docker) — dari situ, container `mailpit` hanya terlihat lewat port yang dipetakan ke `localhost:1025`,
  bukan lewat nama service Docker. Dua nilai berbeda untuk dua sudut pandang jaringan yang berbeda,
  bukan inkonsistensi.
- **`MAIL_FROM=no-reply@jualin.local`** dipakai di kedua tempat (compose dan `.env.example`) — domain
  `.local` sengaja dipilih supaya tidak ada yang salah kira ini alamat produksi sungguhan; nilainya
  hanya perlu valid secara sintaks untuk Mailpit, yang tidak memvalidasi domain penerima sama sekali.

### `docs/testing/flow/` — pola perubahan yang konsisten

Setiap tempat yang sebelumnya menyuruh `docker compose logs api | grep -o '...' | tail -1` diganti
dua langkah yang sama: **"Buka `http://localhost:8025`, klik email terbaru — subjeknya '...'"**.
Enam tempat di lima berkas: `README.md` (bagian *Soal email*, judulnya sendiri diganti dari
"LogMailer" ke "Mailpit"), `00-menjalankan-aplikasi.md` (§7 bertambah langkah buka Mailpit; §8 lama —
"cara mengambil tautan dari log" — **dihapus seluruhnya**, bukan disunting, karena masalah yang
dipecahkannya, `\n\n` literal yang ikut tersalin dari baris log, tidak ada lagi begitu tidak ada lagi
yang perlu disalin dari log), `01-registrasi-dan-autentikasi.md` (§1.2 verifikasi, §1.6 reset
password), `02-tim-dan-undangan.md` (§2.1 undangan pertama, §2.3 undangan kedua ke organization
lain), `06-checklist-akhir.md` (tiga baris checklist + satu baris baru "Mailpit bisa dibuka" di
bagian 0).

**Satu baris troubleshooting baru ditambahkan, bukan dihapus** — `00-menjalankan-aplikasi.md`'s
tabel *Kalau ada yang tidak beres* bertambah baris untuk kasus email terkirim (`201`/`202`) tapi
tidak muncul di Mailpit. Draf pertama baris ini mengklaim race condition boot antara `api` dan
`mailpit` sebagai penyebab utama — **diverifikasi langsung sebelum ditulis final, dan klaimnya
tidak terbukti** (percobaan registrasi 0.3 detik setelah `docker compose up` tetap `201`, bukan
karena race itu tidak mungkin terjadi sama sekali, tapi karena urutan langkah di berkas ini sendiri
— `make migrate-up` berjalan sebelum aksi manapun yang mengirim email — sudah memberi Mailpit banyak
waktu). Baris troubleshooting final ditulis lebih berhati-hati: menyebut kemungkinan itu **jarang**
dan mengarahkan ke `grep "failed to send"` (frasa log yang sungguhan dipakai `internal/auth` dan
`internal/invitation` sejak Phase 1) alih-alih mengklaim penyebab pasti yang belum tentu benar.

### Verifikasi manual end-to-end — dari nol, mengikuti `docs/testing/flow/00` persis

```
docker compose down -v && rm .env && cp .env.example .env
docker compose up --build -d        → postgres, mailpit, api ketiganya naik
make migrate-up                     → goose: successfully migrated database to version: 5
curl /health, /health/ready         → 200, 200
curl -o /dev/null http://localhost:8025 → 200

POST /v1/auth/register              → 201
GET Mailpit /api/v1/messages        → Subject "Verifikasi email Jualin CRM Anda",
                                       From no-reply@jualin.local, To (email yang didaftarkan)
                                     → token diekstrak dari body, POST /v1/auth/verify-email → 200 verified

POST /v1/auth/password/forgot       → 202
GET Mailpit /api/v1/messages        → Subject "Reset password Jualin CRM Anda" muncul sebagai pesan terbaru
```

Ketiga jenis email (verifikasi, reset password — di atas; undangan diverifikasi terpisah lewat
langkah manual `docs/testing/flow/02` §2.1) dikonfirmasi benar-benar terkirim, bukan diasumsikan dari
membaca kode `internal/auth`/`internal/invitation` yang tidak berubah sejak Phase 1.

### `authentication.md` — bagian baru *Pengiriman email*

Ditambahkan setelah bagian *Reset password* (tempat paling dekat pemakainya) — bukan menggantikan
bagian manapun yang sudah ada. Isinya: tabel dua provider (`log`/`smtp`) dan kapan masing-masing
ditolak boot, kenapa `SMTPMailer` tidak memakai `smtp.SendMail`, kenapa kegagalan kirim tidak pernah
membatalkan apa pun yang sudah commit (menegaskan kembali Aturan #32, bukan mengubahnya), Mailpit di
development, dan batas yang jelas untuk produksi (SPF/DKIM/DMARC tetap di luar cakupan, dicatat lagi
supaya phase ini tidak terbaca sebagai "email selesai").

### Verifikasi

- `docker compose config` — valid.
- Seluruh langkah `docs/testing/flow/00-menjalankan-aplikasi.md` dijalankan ulang dari nol setelah
  setiap perubahan berkas, bukan hanya sekali di awal — memastikan urutan baru (termasuk §7 yang
  bertambah langkah Mailpit) benar-benar bisa diikuti apa adanya oleh pembaca, bukan cuma masuk akal
  di atas kertas.
- Tidak ada perubahan kode Go di issue ini — `go test -race ./...` tidak dijalankan ulang karena tidak
  ada yang bisa berubah hasilnya; `gofmt -l .` dicek tetap bersih sebagai sanity check murah.

### Seluruh 12 acceptance criteria PRD Phase 4.6

| # | Kriteria | Bukti |
|---|---|---|
| 1 | SMTP mengirim sungguhan, dibuktikan pesan sampai & terbaca | `TestSMTPMailer_MessageActuallyArrives` (#63) + verifikasi manual end-to-end (#64) |
| 2 | `make dev` menyalakan Mailpit, email terlihat tanpa konfigurasi tambahan | Service `mailpit` di `docker-compose.yml`, dikonfirmasi lewat verifikasi manual (#64) |
| 3 | Ganti SMTP lain tidak menyentuh kode | `SMTPConfig` seluruhnya dari env (#63); `.env.example`/`docker-compose.yml` dua nilai `SMTP_HOST` berbeda untuk dua sudut pandang jaringan tanpa menyentuh kode (#64) |
| 4 | `MAIL_FROM` benar-benar dipakai | `TestSMTPMailer_UsesMailFromAsSender` (#63) + verifikasi manual: `From: no-reply@jualin.local` (#64) |
| 5 | `MAIL_PROVIDER=log` + produksi → boot gagal | `TestLoad_ProductionRejectsLogMailProvider` (#63) |
| 6 | `smtp` tanpa host sah → boot gagal | `TestLoad_SMTPRequiresHost` (#63) |
| 7 | Server menggantung tidak menggantung request | `TestSMTPMailer_UnreachableServerFailsWithinTimeout` + adversarial proof (#63) |
| 8 | Karakter kontrol tidak bisa menyuntik header | `TestBuildRFC5322_RejectsHeaderInjection*` + adversarial proof (#63) |
| 9 | Password SMTP tidak pernah di log | `smtp.go`'s `c.Auth` error tidak pernah dibungkus (#63, verifikasi kode) |
| 10 | Subject/body non-ASCII sampai utuh | `TestBuildRFC5322_NonASCIISubjectEncoded` + `TestSMTPMailer_NonASCIISubjectArrivesIntact` (#63) |
| 11 | Test lama lulus tanpa perubahan asersi | `go test -race ./...` bersih di #63, nol asersi diubah |
| 12 | `docs/testing/flow/` diperbarui | 5 berkas, 6 tempat, pola konsisten (#64) |

**Seluruh 12 kriteria terpenuhi. Phase 4.6 — Email Delivery selesai.**
