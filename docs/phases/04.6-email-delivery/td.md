# Phase 4.6 — Email Delivery · Technical Design

> **Bagaimana.** Apa & kenapa di [`prd.md`](./prd.md).
> Hanya **delta** untuk phase ini — aturan yang sudah ada di [`freeze.md`](../../architecture/freeze.md) dirujuk, tidak diulang.
> Sumber: freeze bagian 6 (Aturan #32 — efek samping di luar transaksi), Aturan #26, #36 · [ADR-010](../../decisions/ADR-010-fail-fast-startup.md)

---

## 1. Yang sudah ada, dan yang tidak berubah

```go
// internal/shared/mailer/mailer.go — TIDAK diubah oleh phase ini
type Message struct {
	To      string
	Subject string
	Body    string
}

type Mailer interface {
	Send(ctx context.Context, msg Message) error
}
```

Interface ini sudah ada sejak Phase 1, dan komentarnya sudah menjelaskan kenapa ia dibuat lebih awal
dari biasanya: implementasi kedua sudah diketahui akan datang. **Phase ini adalah implementasi kedua
itu** — tidak ada abstraksi baru yang perlu dibuat (Aturan #27–#29).

**Yang tidak disentuh sama sekali:** `Message`, `Mailer`, `LogMailer`, `spyMailer` di test, isi teks
setiap email, dan setiap pemanggil `Send` di `internal/auth` dan `internal/invitation`. Satu-satunya
perubahan di luar paket `mailer` adalah `config` dan composition root.

---

## 2. Konfigurasi

Menyusul bentuk `CORS_ALLOWED_ORIGINS` (#30) dan `TRUSTED_PROXIES` (#57) — satu env per hal, di-parse
di `internal/shared/config`, divalidasi saat boot.

```go
// validMailProviders bertambah "smtp" — sebelumnya hanya {"log"} sejak Phase 1.
var validMailProviders = []string{"log", "smtp"}

// MailFrom sudah ada sejak Phase 1 tapi TIDAK PERNAH DIBACA siapa pun —
// LogMailer mengabaikan From sepenuhnya. Phase ini yang membuatnya hidup.
MailFrom string `env:"MAIL_FROM" envDefault:"no-reply@localhost"`

SMTPHost     string `env:"SMTP_HOST"`
SMTPPort     int    `env:"SMTP_PORT" envDefault:"587"`
SMTPUsername string `env:"SMTP_USERNAME"`
SMTPPassword string `env:"SMTP_PASSWORD"`
// starttls (default) | none. TLS implisit port 465 sengaja tidak didukung — keputusan E3.
SMTPTLS      string `env:"SMTP_TLS" envDefault:"starttls"`
// Batas waktu SELURUH percakapan SMTP, bukan hanya dial — keputusan E1, §4.
SMTPTimeout  time.Duration `env:"SMTP_TIMEOUT" envDefault:"10s"`
```

### 2.1 Validasi (Aturan #36)

Ditambahkan ke `Config.validate()`, mengikuti pola baris `CORS_ALLOWED_ORIGINS`/`TRUSTED_PROXIES`:

| Kondisi | Pesan |
|---|---|
| `AppEnv == production && MailProvider == "log"` | `MAIL_PROVIDER must not be "log" when APP_ENV=production` — keputusan E2 |
| `MailProvider == "smtp" && SMTPHost == ""` | `SMTP_HOST must be set when MAIL_PROVIDER=smtp` |
| `MailProvider == "smtp" && (SMTPPort <= 0 \|\| > 65535)` | pola sama seperti `HTTP_PORT` |
| `SMTPTLS` bukan `starttls`/`none` | daftar nilai sah, pola sama seperti `LOG_LEVEL` |
| `AppEnv == production && MailProvider == "smtp" && SMTPTLS == "none"` | kredensial dan token verifikasi lewat jaringan tanpa enkripsi |
| `SMTPTimeout <= 0` | pola sama seperti `PUBLIC_API_RATE_LIMIT` |

`SMTP_USERNAME`/`SMTP_PASSWORD` **boleh kosong** — Mailpit tidak memakai auth. Bila keduanya kosong,
langkah `AUTH` dilewati (§4).

> **Tidak ada validasi "MAIL_FROM harus alamat sah"** di luar pengecekan karakter kontrol (§5).
> Memvalidasi bentuk alamat email dengan benar adalah lubang kelinci; server SMTP-lah yang akan
> menolaknya, dan itu terjadi saat boot-nya sudah lewat. Yang penting dijaga di sini adalah keamanan
> (injeksi header), bukan kebenaran alamat.

---

## 3. Paket — `internal/shared/mailer`

```
mailer.go        Message, Mailer, LogMailer          (sudah ada — tidak diubah)
smtp.go          SMTPMailer, SMTPConfig              (baru)
message.go       buildRFC5322 — fungsi murni         (baru)
smtp_test.go     integrasi, Mailpit via testcontainers (baru)
message_test.go  unit, tanpa jaringan sama sekali    (baru)
```

`buildRFC5322` **dipisah sebagai fungsi murni** dari `SMTPMailer.Send` supaya format pesan bisa diuji
tanpa server, jaringan, atau container — pola yang sama seperti `lib/nav.ts`/`lib/api-key-rows.ts` di
dashboard, dan `apikey.parseCredential` di backend.

---

## 4. `SMTPMailer.Send` — kenapa bukan `smtp.SendMail`

`smtp.SendMail` adalah satu panggilan yang menutup seluruh percakapan SMTP, dan itu menggoda. Tapi ia
**tidak punya batas waktu sama sekali** — ia memakai `net.Dial` telanjang. `Send` dipanggil **sinkron
di jalur request** (`auth.Usecase.sendVerificationEmail` dipanggil langsung dari `Register`), jadi
server SMTP yang menggantung akan menggantung request HTTP sampai timeout OS. Itu melanggar kriteria
#7.

Karena itu percakapannya dibangun manual:

```go
func (m *SMTPMailer) Send(ctx context.Context, msg Message) error {
	raw, err := buildRFC5322(m.from, msg, m.now())
	if err != nil {
		return err   // injeksi header ditolak SEBELUM koneksi dibuka (§5)
	}

	deadline := time.Now().Add(m.timeout)
	conn, err := (&net.Dialer{Timeout: m.timeout}).DialContext(ctx, "tcp", m.addr)
	if err != nil {
		return fmt.Errorf("mailer: dial %s: %w", m.addr, err)
	}
	defer conn.Close()
	// Satu deadline untuk SELURUH percakapan, bukan per operasi — server yang
	// menjawab satu byte per detik tetap terputus tepat waktu.
	_ = conn.SetDeadline(deadline)

	c, err := smtp.NewClient(conn, m.host)
	if err != nil { return ... }
	defer func() { _ = c.Close() }()

	if m.tls == "starttls" {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return fmt.Errorf("mailer: server %s does not advertise STARTTLS", m.addr)
		}
		if err := c.StartTLS(&tls.Config{ServerName: m.host, MinVersion: tls.VersionTLS12}); err != nil { ... }
	}

	if m.username != "" || m.password != "" {
		if err := c.Auth(smtp.PlainAuth("", m.username, m.password, m.host)); err != nil {
			return errors.New("mailer: smtp authentication failed")  // §6 — err asli TIDAK dibungkus
		}
	}

	if err := c.Mail(m.from); err != nil { ... }
	if err := c.Rcpt(msg.To); err != nil { ... }
	w, err := c.Data(); ...
	if _, err := w.Write(raw); err != nil { ... }
	if err := w.Close(); err != nil { ... }
	return c.Quit()
}
```

Catatan yang mengikat:

- **`ctx` dipakai di `DialContext`** — request yang dibatalkan tidak meninggalkan dial menggantung.
  Percakapan setelahnya dijaga `SetDeadline`, karena `net/smtp` tidak menerima `context`.
- **STARTTLS diperiksa lewat `Extension` lebih dulu**, bukan langsung `StartTLS` — supaya pesan
  errornya menyebutkan servernya yang tidak mendukung, bukan kegagalan handshake yang membingungkan.
- **`smtp.PlainAuth` menolak mengirim kredensial di koneksi tak terenkripsi** kecuali host-nya
  localhost. Itu perilaku stdlib dan **sengaja tidak diakali** — ia justru menegakkan E3 secara gratis.
- Kegagalan `Send` **tidak** membatalkan apa pun yang sudah commit; pemanggilnya sudah mencatat error
  secara terstruktur sejak Phase 1 (Aturan #32). Phase ini tidak mengubah kontrak itu.

---

## 5. `buildRFC5322` — format dan pertahanan injeksi header

```go
func buildRFC5322(from string, msg Message, now time.Time) ([]byte, error)
```

Header yang ditulis:

```
From: <MAIL_FROM>
To: <msg.To>
Subject: <RFC 2047 bila perlu>
Date: <RFC1123Z>
MIME-Version: 1.0
Content-Type: text/plain; charset=UTF-8
Content-Transfer-Encoding: 8bit
```

### 5.1 Injeksi header — kriteria #8

`msg.To` berasal dari **input pengguna** (form registrasi, form undangan). Alamat yang mengandung
`\r` atau `\n` bisa menyisipkan header tambahan — mis. `korban@x.com\r\nBcc: penyerang@y.com` membuat
salinan email verifikasi terkirim ke penyerang.

`buildRFC5322` **menolak** (`error`, bukan sanitasi diam-diam) bila `from`, `To`, atau `Subject`
mengandung `\r` atau `\n`. Diverifikasi saat menulis TD ini: `strings.ContainsAny(s, "\r\n")`
mendeteksi ketiga bentuk (`\r\n`, `\n` telanjang, `\r` telanjang).

> **Kenapa menolak, bukan membuang karakternya.** Membuang menghasilkan alamat yang *terlihat* sah
> tapi bukan yang diminta pemanggil — email terkirim ke tempat yang salah tanpa jejak. Menolak
> menghasilkan error terstruktur yang sudah ditangani pemanggil sejak Phase 1.

`Body` **tidak** perlu diperiksa: ia berada setelah baris kosong pemisah header, jadi `\n` di sana
memang isi pesan, bukan header baru.

### 5.2 Encoding subject — kriteria #10

`mime.QEncoding.Encode("utf-8", subject)`. Diverifikasi saat menulis TD ini:

| Input | Output |
|---|---|
| `Verifikasi email Jualin CRM Anda` | tidak berubah — ASCII dibiarkan apa adanya |
| `Anda diundang bergabung di Jualin CRM` | tidak berubah |
| `Sudah — kami kirim ulang` | `=?utf-8?q?Sudah_=E2=80=94_kami_kirim_ulang?=` |

Ketiga subject yang dipakai produk ini hari ini semuanya ASCII, jadi header-nya tetap terbaca biasa;
encoding baru aktif kalau nanti ada teks Indonesia dengan tanda baca tipografis. **Justru itu
alasannya dipasang sekarang** — kalau tidak, subject pertama yang memakai `—` akan sampai sebagai
karakter rusak, dan tidak ada yang akan mengaitkannya dengan perubahan teks yang tampak sepele.

Body dikirim `8bit` dengan `charset=UTF-8` — Mailpit dan setiap MTA modern menerimanya.

---

## 6. Rahasia dan log (Aturan #26, kriteria #9)

| Aturan |
|---|
| `SMTPPassword` **tidak pernah** masuk pesan error atau log. Kegagalan auth dikembalikan sebagai `errors.New("mailer: smtp authentication failed")` — error asli dari `net/smtp` **tidak** dibungkus, karena ia bisa memuat echo kredensial dari server |
| `SMTPMailer` **tidak** memiliki logger sama sekali. Ia mengembalikan error; pemanggil yang mencatat — dan pemanggil sudah mencatat `err` + `to` saja sejak Phase 1, bukan isi pesan |
| `Config` **tidak** punya `String()`/`LogValue()` yang mencetak seluruh struct. Tidak ada yang menambahkannya di phase ini |
| Body email (yang memuat token verifikasi) **tidak pernah** dicatat oleh `SMTPMailer` — berbeda dari `LogMailer`, yang memang tugasnya begitu, dan justru karena itu dilarang di produksi (E2) |

---

## 7. Composition root

```go
func newMailer(cfg *config.Config, log *slog.Logger) mailer.Mailer {
	switch cfg.MailProvider {
	case "log":
		return mailer.NewLogMailer(log)
	case "smtp":
		return mailer.NewSMTPMailer(mailer.SMTPConfig{
			Host: cfg.SMTPHost, Port: cfg.SMTPPort,
			Username: cfg.SMTPUsername, Password: cfg.SMTPPassword,
			From: cfg.MailFrom, TLS: cfg.SMTPTLS, Timeout: cfg.SMTPTimeout,
		})
	default:
		panic(...)  // sudah ada — tidak berubah bentuknya
	}
}
```

Cabang `default` yang sudah ada tetap: ia tidak bisa tercapai karena `validate()` sudah menyaring
nilainya, dan itu memang pesannya.

---

## 8. Mailpit di `docker-compose.yml`

```yaml
mailpit:
  image: axllent/mailpit:latest
  ports:
    - "1025:1025"   # SMTP
    - "8025:8025"   # web UI
```

`api` mendapat:

```yaml
MAIL_PROVIDER: smtp
MAIL_FROM: no-reply@jualin.local
SMTP_HOST: mailpit
SMTP_PORT: 1025
SMTP_TLS: none          # Mailpit lokal, tanpa TLS — ditolak kalau APP_ENV=production (§2.1)
```

Tanpa `depends_on: mailpit`: SMTP hanya dipakai saat ada email dikirim, bukan saat boot, dan `api`
yang menunggu Mailpit sehat hanya memperlambat `docker compose up` tanpa manfaat. Kegagalan kirim
sudah punya perlakuan yang benar sejak Phase 1 (Aturan #32).

**`make dev` setelah ini langsung menyalakan Mailpit** — memenuhi kriteria #2 tanpa langkah tambahan.
UI di `http://localhost:8025`.

---

## 9. Rencana test

### 9.1 Unit — tanpa jaringan (`message_test.go`)

| Test | Membuktikan |
|---|---|
| `TestBuildRFC5322_HeadersAndBody` | Seluruh header wajib ada, urutannya benar, body dipisah baris kosong |
| `TestBuildRFC5322_RejectsHeaderInjectionInTo` | `\r\n`, `\n`, dan `\r` di `To` ketiganya ditolak — kriteria #8 |
| `TestBuildRFC5322_RejectsHeaderInjectionInSubjectAndFrom` | Sama untuk dua field lain |
| `TestBuildRFC5322_ASCIISubjectNotEncoded` | Subject produk hari ini tetap terbaca polos |
| `TestBuildRFC5322_NonASCIISubjectEncoded` | `=?utf-8?q?…?=` — kriteria #10 |
| `TestBuildRFC5322_BodyNewlinesAreNotHeaders` | `\n` di body bukan kerentanan, tidak ikut ditolak |

### 9.2 Integrasi — Mailpit sungguhan (`smtp_test.go`, keputusan E5)

Testcontainers `axllent/mailpit`, tunggu port 8025 siap, kirim lewat `SMTPMailer`, lalu **baca kembali
pesannya lewat API Mailpit** (`GET /api/v1/messages`, lalu detailnya) — bukan memeriksa bahwa `Send`
mengembalikan `nil`.

| Test | Membuktikan |
|---|---|
| `TestSMTPMailer_MessageActuallyArrives` | Kriteria #1 — pesan ada di kotak masuk, `From`/`To`/`Subject`/body cocok persis |
| `TestSMTPMailer_UsesMailFromAsSender` | Kriteria #4 — `MAIL_FROM` benar-benar jadi pengirim, bukan diabaikan seperti sebelumnya |
| `TestSMTPMailer_NonASCIISubjectArrivesIntact` | Kriteria #10 dari sisi penerima, bukan hanya dari sisi encoder |
| `TestSMTPMailer_UnreachableServerFailsWithinTimeout` | Kriteria #7 — port tertutup/host blackhole gagal **dalam** batas waktu, diukur, bukan menggantung |

`smtp_test.go` memakai build constraint/skip yang sama seperti harness `dbtest` bila Docker tidak
tersedia — polanya diikuti dari `internal/shared/db/dbtest`, tidak dibuat baru.

### 9.3 Config (`config_test.go`, menambah — tidak mengubah yang ada)

| Test | Membuktikan |
|---|---|
| `TestLoad_ProductionRejectsLogMailProvider` | Kriteria #5 |
| `TestLoad_SMTPRequiresHost` | Kriteria #6 |
| `TestLoad_ProductionRejectsSMTPWithoutTLS` | E3 |
| `TestLoad_InvalidSMTPTLSMode` | Nilai `SMTP_TLS` di luar daftar ditolak saat boot |

Test produksi yang sudah ada (`TestLoad_ProductionMode`, `…RequiresCookieSecure`,
`…RequiresCORSAllowedOrigins`, `…RequiresTrustedProxies`) **perlu ditambahi `MAIL_PROVIDER=smtp` +
`SMTP_HOST`** supaya tetap lolos — penambahan setup, **bukan** perubahan asersi. Kriteria #11 tetap
terpenuhi; ini konsekuensi wajar dari menambah aturan validasi produksi baru, sama seperti yang
terjadi saat `TRUSTED_PROXIES` ditambahkan di #57.

---

## 10. Yang berubah pada dokumentasi

| Berkas | Perubahan |
|---|---|
| `architecture/authentication.md` | Bagian baru **Pengiriman email** — dua provider, kenapa `log` dilarang di produksi, di mana batas waktunya, kenapa kegagalan tidak membatalkan commit |
| `docs/testing/flow/README.md` | Bagian *Soal email — LogMailer* diganti: email sekarang **benar-benar terkirim** ke Mailpit |
| `docs/testing/flow/00-menjalankan-aplikasi.md` | §8 (mengambil tautan dari log) diganti alur Mailpit; §7 bertambah langkah membuka `http://localhost:8025` |
| `docs/testing/flow/01-…`, `02-…` | Perintah `docker compose logs api \| grep …` diganti "buka Mailpit, klik email terbaru" |
| `docs/testing/flow/06-checklist-akhir.md` | Baris yang menyebut "tautan di log" disesuaikan |
| `.env.example` | `MAIL_PROVIDER`, `SMTP_*`, dan `MAIL_FROM` yang kini benar-benar dipakai |
| `docker-compose.yml` | Service `mailpit` + env `api` |
| `STATUS.md` | Baris Selesai; Phase 4.6 di *Progress per Phase*; *Punya Lead Time* diperbarui — pemilihan provider kini **tidak lagi memblokir demo** |

`freeze.md` **tidak disentuh** — tidak ada aturannya yang berubah. Keputusan freeze bagian 6 (kirim
setelah commit, tanpa `jobs`) justru ditegaskan, bukan direvisi. Tidak ada ADR baru: memilih SMTP
sebagai transport adalah pengisian `mailer.Mailer` yang sudah ada, bukan keputusan arsitektur baru.

---

## 11. Risiko teknis

| Risiko | Penanganan |
|---|---|
| `net/smtp` "frozen" di stdlib | Frozen = tidak menerima fitur baru, bukan tidak dipelihara. Kebutuhan di sini persis yang sudah didukungnya. Bila kelak butuh TLS implisit atau OAuth2, itu pemicu sah untuk meninjau ulang — dicatat di §12 |
| Provider produksi ternyata menuntut port 465 | E3 menyatakan ini eksplisit di luar cakupan. Ketiga kandidat di `freeze.md` mendukung 587, jadi tidak ada yang terhalang hari ini |
| Test Mailpit menambah waktu CI | Satu container ringan, sepola dengan Postgres yang sudah ada. Test unit `buildRFC5322` sengaja bebas jaringan supaya bagian tercepatnya tetap cepat |
| `MAIL_FROM` default `no-reply@localhost` ditolak MTA produksi | Sudah tertangani: produksi tanpa `MAIL_FROM` yang benar akan ditolak server SMTP, dan itu kegagalan yang terlihat. Domain sungguhan tetap item *Punya Lead Time* |
| Orang mengira phase ini menyelesaikan deliverability | Tidak. SPF/DKIM/DMARC eksplisit di luar cakupan (PRD) dan tetap wajib sebelum email produksi bisa dipercaya. Dinyatakan lagi di `STATUS.md` |

---

## 12. Kewajiban yang diteruskan ke phase berikutnya

- **Sebelum email produksi sungguhan dipercaya:** domain final + SPF/DKIM/DMARC — tetap item *Punya
  Lead Time* di `STATUS.md`, **tidak** ditutup oleh phase ini.
- **Saat provider dipilih:** cukup mengganti `SMTP_HOST`/`SMTP_PORT`/kredensial. Bila provider yang
  dipilih **hanya** menyediakan TLS implisit (465) atau menuntut OAuth2, barulah E1/E3 ditinjau ulang
  — itu pemicunya, bukan sebelumnya.
- **Phase 5 (Mobile):** tidak terdampak. Mobile memakai jalur user session yang sama; email verifikasi
  dan reset password justru baru benar-benar bekerja setelah phase ini.
- **Kalau kelak butuh retry/outbox:** freeze bagian 6 sudah memutuskan **tidak**, dengan alasan yang
  masih berlaku (jalur pemulihannya adalah tombol kirim ulang). Mengubah itu butuh ADR, bukan
  keputusan di tengah implementasi.
