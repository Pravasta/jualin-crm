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
