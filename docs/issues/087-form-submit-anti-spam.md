# Issue #87 — checklist penutupan phase

> Checklist ringkas, **bukan** catatan status. Status pekerjaan tetap hidup di GitHub Issues
> (ADR-008) — berkas ini hanya mengumpulkan poin yang perlu **dicek ulang saat #89** (penutup Phase 6)
> menutup phase-nya. Detail lengkap tiap poin ada di `docs/phases/06-connect-form/notes.md` bagian
> `## #87`.

## Kontrak kabel untuk #88 — WAJIB dicocokkan, bukan opsional

`internal/form/handler_http.go` menetapkan nama field literal yang halaman embed (#88) **harus**
dirender persis sama, atau setiap submission gagal diam-diam (field terbaca sebagai kosong/tak
dikenal, bukan error yang jelas):

- Enam field isi: `name`, `email`, `phone`, `company`, `message`, `product` — sama persis dengan
  `FieldKey`'s nilai string sendiri.
- Honeypot: `website` — nama ini pilihan #87 sendiri (bukan dari TD), dipilih supaya terlihat seperti
  field asli bagi bot yang mengisi semua field yang ditemukannya.
- Token time-trap: `form_token`.
- Respons CAPTCHA: `cf-turnstile-response` — **bukan** pilihan codebase ini, konvensi tetap widget
  Cloudflare Turnstile sendiri.

**Dicek ulang saat #88 dibangun**: apakah keempat nama ini benar-benar dipakai di `form.gohtml`. —
✅ **Dikonfirmasi saat #88**: keenam field isi memakai `name="{{.Key}}"` dari `FieldKey`'s nilai
string sendiri, honeypot `name="website"`, token `name="form_token"`, dan `class="cf-turnstile"`
(Cloudflare sendiri yang menyuntik `cf-turnstile-response` lewat widget-nya, bukan dirender manual di
sini) — semuanya cocok persis dengan yang `handler_http.go`'s konstanta harapkan.

## Deviasi dari TD — sudah diverifikasi benar lewat AskUserQuestion, catat alasannya

- [x] **Content-Type submit `application/x-www-form-urlencoded`, bukan JSON.** TD §7 menyiratkan ini
      ("murni HTML" tanpa CAPTCHA) tapi tidak pernah menyatakannya eksplisit untuk endpoint submit.
      Dikonfirmasi eksplisit ke pemilik produk sebelum menulis kode — konsekuensinya `raw_payload`
      dibangun dari field yang sudah di-parse (bukan byte body mentah, beda dari jalur API key).
      ~~**Ditinjau ulang di #89**: apakah halaman embed (#88) benar-benar mengirim bentuk ini, bukan
      `fetch()` JSON yang diam-diam "lebih mudah" ditulis.~~ — ✅ **Diverifikasi di #98**:
      `internal/form/template/form.gohtml:57` = `<form method="post" action="/v1/forms/{{.PublicKey}}/submit">`,
      dan **nol** kemunculan `fetch(`, `JSON.stringify`, atau `application/json` di seluruh template.
      Bentuk native `<form method="post">` bertahan sampai akhir phase. Poin tertutup.
- [x] **Honeypot diperiksa sebelum time-trap/CAPTCHA (TD §5 apa adanya), celah waktu respons DITERIMA
      sadar, tidak ditutup.** Tiga opsi diajukan; opsi "ikuti TD apa adanya + catat keterbatasan"
      dipilih karena dua opsi lain menyimpang dari alasan tertulis TD §5 sendiri (jangan buang kuota
      CAPTCHA ke bot yang sudah diketahui). AC "tidak bisa dibedakan termasuk waktu respons" karena itu
      **tidak** terpenuhi penuh saat `CAPTCHA_PROVIDER=turnstile` — hanya status code dan bentuk body.
      ~~**Ditinjau ulang di #89**~~ **atau kapan pun traffic spam nyata mulai terlihat**: apakah celah
      ini pernah benar-benar dieksploitasi (bot yang secara spesifik mengukur waktu respons untuk
      mendeteksi honeypot). Kalau ya, opsi #3 dari diskusi awal (tetap jalankan time-trap+CAPTCHA di
      jalur honeypot, buang hasilnya) jadi kandidat.
      **Ditinjau di #98 — tetap terbuka, dan pemicunya diperjelas.** Celah waktu ini hanya ada saat
      `CAPTCHA_PROVIDER=turnstile`; hari ini seluruh sistem berjalan `none`, jadi **celahnya belum
      pernah aktif sama sekali**. Ia baru jadi nyata pada hari Turnstile diaktifkan — bukan sekarang,
      dan bukan sesuatu yang bisa diamati sebelum itu.

## Keputusan yang tetap dibawa maju — butuh traffic nyata untuk diputuskan, bukan bisa diselesaikan sekarang

- [ ] **`FORM_SUBMIT_RATE_LIMIT_IP=20`/`FORM_SUBMIT_RATE_LIMIT_FORM=60` per menit bukan hasil
      pengukuran** (TD §6, keputusan D4) — sama persis situasi `PUBLIC_API_RATE_LIMIT=60` di
      `docs/issues/047-public-lead-api.md`. Perlu ditinjau ulang begitu form sungguhan dipasang di
      situs pelanggan nyata dan menerima traffic (spam maupun sah).
      **Ditinjau di #98 — tetap terbuka, tapi tidak lagi berdiri sendiri**: ketiga angka
      (`PUBLIC_API_RATE_LIMIT` + kedua angka form) sekarang punya **satu** pemicu peninjauan bersama
      yang ditulis di `architecture/api.md` bagian *Angka batasnya belum pernah diukur*, bukan tiga
      keraguan terpisah yang masing-masing menunggu. Kredensial baru Phase 7 menambah angka ke daftar
      yang sama.
- [ ] **Time-trap 2 detik–30 menit (TD §6) juga angka yang tidak diukur** — masuk akal secara intuitif
      (manusia butuh >2 detik mengisi form, jarang meninggalkan tab terbuka >30 menit sebelum submit),
      tapi belum pernah diuji terhadap perilaku pengisi form sungguhan.
      **Ditinjau di #98 — tetap terbuka.** Berbeda dari angka rate limit, ini tidak bisa ikut ke
      peninjauan bersama itu: pemicunya bukan volume traffic melainkan **perilaku pengisi form
      sungguhan** (berapa lama manusia nyata mengisi enam field). Data itu baru ada setelah form
      terpasang di situs pelanggan. Gejala yang harus diwaspadai: submission sah ditolak
      `form_token_invalid` — bukan spam yang lolos.

## Backfill dari #85 — dua celah ditemukan sambil mengerjakan #87, sudah diperbaiki (arsip)

- [x] `internal/shared/authz/authz_test.go` — `allActions` dan `TestRequire`'s kasus per-role tidak
      pernah menyentuh `ActionForm*` sejak #85 (file itu sendiri sudah punya komentar eksplisit
      tentang pola celah yang sama terjadi di #46). Diperbaiki di #87 karena toh menyentuh file yang
      sama untuk `PrincipalPublicForm`'s test sendiri.
- [x] `docs/architecture/multi-tenancy.md` — TD §1 (#85) menyebut "multi-tenancy.md ditambah baris
      keempat (§15)" sebagai bagian cakupannya sendiri; tidak pernah dilakukan. Ditambahkan di #87.

---

**Ditinjau 31 Agustus 2026 di #98, bukan di #89 seperti direncanakan.** #89 menutup Phase 6 **tanpa**
membaca berkas ini — `docs/phases/06-connect-form/issues.md` tidak pernah memuat langkah "cek
`docs/issues/*`" yang skill `jualin-issue-log` wajibkan, jadi tidak ada yang memaksa membacanya. Celah
prosesnya ditutup di #98 (pointer dipasang, templat issue diperbaiki).

**Hasil peninjauan:** satu poin **tertutup dengan bukti** (Content-Type — `form.gohtml` terbukti
memakai `<form method="post">`, nol `fetch()`). Tiga poin **tetap terbuka dengan pemicu yang kini
eksplisit**: angka rate limit (ikut peninjauan bersama di `api.md`), batas time-trap (butuh perilaku
pengisi form nyata), celah waktu honeypot (belum pernah aktif — `CAPTCHA_PROVIDER=none`).

> Yang membuat berkas ini hampir sia-sia bukan isinya, melainkan tidak adanya penunjuk ke arahnya.
> Berkas temuan yang tidak dirujuk dari mana pun sama saja dengan tidak ditulis.
