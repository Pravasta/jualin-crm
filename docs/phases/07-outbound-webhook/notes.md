# Phase 7 — Outbound Webhook · Implementation Notes

> Realitas implementasi, satu bagian per issue. Penyimpangan dari TD/PRD beserta alasannya,
> utang teknis, catatan untuk session berikutnya. Naratif lengkap — berbeda dari `docs/issues/07*`
> yang hanya checklist penutupan phase.

---

## #100 — Migration `0008`, `safedial`, domain `webhook`, CRUD endpoint

Pembuka Phase 7. Murni Go, tanpa UI dan tanpa worker. Dibangun mengikuti bentuk `internal/form` (#85)
berkas demi berkas.

### `internal/shared/safedial` — inti keamanan, dibangun tabel-atas-seluruh-rentang

`isDeniedIP(netip.Addr)` menutup seluruh rentang TD §3.1 lewat kombinasi predikat stdlib
(`IsLoopback`/`IsPrivate`/`IsLinkLocalUnicast`/`IsMulticast`/`IsUnspecified`/…) plus daftar prefix
eksplisit untuk yang stdlib tidak punya helper-nya (CGNAT `100.64/10`, `0.0.0.0/8` yang lebih luas dari
`IsUnspecified`, reserved `240/4`, dokumentasi, NAT64, discard-only, protocol assignments IPv6).

**`addr.Unmap()` dipanggil paling awal — bukan opsional.** Tanpa itu, `::ffff:169.254.169.254`
(IPv4-mapped IPv6) melewati setiap cek karena dievaluasi terhadap bentuk IPv6-nya. Ini bypass yang
paling mungkin dieksploitasi, dan diuji eksplisit di `denylist_test.go` (`::ffff:127.0.0.1`,
`::ffff:169.254.169.254`, `::ffff:10.0.0.1` — semuanya ditolak; `::ffff:8.8.8.8` lolos).

Test-nya sengaja **ekshaustif** — tabel ~35 alamat "ditolak" (representatif di tiap rentang, IPv4 dan
IPv6) plus ~12 "diizinkan" (publik, plus alamat tepat di luar batas CGNAT/private/link-local supaya
prefix-nya tidak kelebaran). Ini test paling penting di seluruh phase; gap di sini = lubang SSRF.

`Validator.ValidateURL` adalah paruh **saat disimpan** dari TD §3.2's "divalidasi dua kali" — parse,
tolak skema non-http(s), resolve host, tolak bila **ada** IP hasil resolusi yang masuk daftar tolak.
Literal IP di URL dicek langsung tanpa DNS. `allowPrivate` (dari `WEBHOOK_ALLOW_PRIVATE_TARGETS`)
melewati cek IP sepenuhnya tapi tetap menegakkan skema — supaya `http://localhost:9099` bisa dipakai di
dev tanpa membuka jalur URL rusak. Paruh **saat dikirim** (resolve ulang + dial IP tervalidasi lewat
`DialContext` kustom) adalah #102.

`safedial` dipisah dari `internal/webhook` karena ia primitif jaringan, bukan logika domain webhook —
inbound webhook (Phase 7.5) memakainya lagi.

### `ErrURLNotAllowed` — satu error untuk semua alasan penolakan

`safedial.ValidateURL` selalu mengembalikan `ErrURLNotAllowed` (di-`fmt.Errorf("%w: <detail>")`).
Usecase memetakannya ke `400 webhook_url_not_allowed` **generik**, dan me-`logger.Warn` detailnya
(rentang mana yang kena). Membedakan "alamat privat" dari "tidak bisa diresolusi" di response HTTP
memberi pelanggan alat memetakan jaringan internal kita lewat pesan error (TD §7) — dibuktikan lewat
`curl`: response `{"code":"webhook_url_not_allowed"}`, log server `reason="...: 169.254.169.254 is in
a denied range"`.

### Migration `0008` — `webhook_deliveries` adalah antriannya (penyimpangan freeze D1)

Tidak ada tabel `jobs` generik. Alasan penuh di `prd.md` D1 dan komentar migration itu sendiri.
Konsekuensi konkret yang tercatat: `ix_webhook_deliveries_claim` **sengaja tidak berawalan
`organization_id`** (pengecualian tertulis atas Aturan #16) — worker mengambil kerja lintas seluruh
organization, `organization_id` adalah hasil klaim bukan input. Dibuktikan `EXPLAIN` di
`repository_test.go`: query klaim memakai `ix_webhook_deliveries_claim`, bukan seq scan.

`ck_webhook_endpoints_url_scheme` sengaja hanya `http://`/`https://` di level DB, **tidak** memaksa
`https` — dev butuh `http://localhost`. Penegakan `https` di produksi (kalau kelak diputuskan) ada di
usecase, bukan CHECK yang tidak bisa dilonggarkan per-environment.

### `ClaimDue`/`Reap`/`Purge` dibangun tanpa pemanggil — preseden `form.FindByPublicKey`

Ketiganya ada di `DeliveryRepository` dan diuji penuh terhadap Postgres asli (`repository_test.go`),
tapi **nol pemanggil HTTP** di #100 — persis pola `apikey.FindByKeyID` (#46) / `form.FindByPublicKey`
(#85) yang dibangun satu issue sebelum yang memakainya. Worker (#102) menyambungkannya.
`ClaimDue` sudah pakai `FOR UPDATE SKIP LOCKED`; buktinya "diklaim tepat sekali" ada di test
(seed pending → claim → status `delivering` → claim kedua nihil). Uji konkurensi nyata (N goroutine
paralel) adalah #102.

### `backoff()` juga dibangun sekarang, dipakai #102

`entity.go`'s `backoff(attempt)` mengembalikan jeda D5 (1m→5m→30m→2j→6j) — pure, diuji terhadap jadwal
persis, meski worker yang memakainya belum ada. `retryDelays` adalah **array** `[5]time.Duration`
(bukan slice) supaya `MaxAttempts = len(retryDelays)` bisa jadi konstanta compile-time.

### Otorisasi — lima `Action` baru, ditambahkan ke `allActions` di PR yang sama

`webhook.create/list/read/update/delete`, Owner/Admin saja — bentuk dan alasan sama dengan `api_key.*`
(#46) dan `form.*` (#85): kredensial yang mengalirkan data organization ke luar. **Celah #46/#85 tidak
terulang**: kelima `Action` masuk `authz_test.go`'s `allActions` **dan** tabel per-role di PR yang
sama yang mendefinisikannya, bukan di-backfill belakangan. Tabel-atas-seluruh-`Action` yang sudah ada
(`TestRequire_APIKeyPrincipal_OnlyLeadCreateAllowed` / `…PublicForm…`) otomatis membuktikan principal
`api_key`/`public_form` tidak dapat satu pun dari kelimanya — tanpa baris baru.

Tidak ada peta gaya `publicFormAllows` untuk webhook: endpoint webhook tidak pernah memanggil balik ke
sistem (ia hanya menerima pengiriman), jadi tidak ada principal "webhook" — kelima `Action` itu seluruh
permukaannya.

### `WEBHOOK_ALLOW_PRIVATE_TARGETS` — satu env var, ditolak saat production

Bentuk persis `CAPTCHA_PROVIDER=none` (#87) / `PUSH_PROVIDER=none` (#68): `envDefault:"false"`, dan
`APP_ENV=production && true` → boot gagal. Dibuktikan `docker compose run` dengan env lengkap
production → `config invalid: WEBHOOK_ALLOW_PRIVATE_TARGETS must not be true when APP_ENV=production`.
`docker-compose.yml` men-set `"true"` (dev) supaya receiver lokal bisa didaftarkan; `.env.example`
default `false`.

### Retry manual — pre-check di usecase + `WHERE status='failed'` di repo

`Usecase.RetryDelivery` → `Delivery.FindByID` (404 lintas-org / hilang) → cek `status != failed` → 409
`delivery_not_retryable` → `MarkForRetry` (repo `UPDATE ... WHERE status='failed'`, 0 baris →
`ErrDeliveryNotRetryable`). Dua lapis: pre-check untuk pesan bersih di kasus umum, `WHERE` di repo
sebagai penjaga nyata terhadap race dengan worker. Dibuktikan `curl`: retry pengiriman `failed` →
`200` status `pending`; retry kedua (sekarang `pending`) → `409`.

### Harness isolasi tenant — lima kasus baru, terbukti bisa gagal

`tenant_isolation_test.go` bertambah `GET/PATCH/DELETE /v1/webhook-endpoints/{id}`,
`GET …/{id}/deliveries`, dan `POST /v1/webhook-deliveries/{id}/retry` — semua lintas org → `404`.
**Dibuktikan bisa gagal**: predikat `organization_id` dihapus sementara dari `postgresRepository.FindByID`
→ 3 subtest merah (`GET`/`DELETE`/`deliveries` bocor `200`; `PATCH` tetap hijau karena scoping-nya di
`Update`'s sendiri, `retry` tetap hijau karena pakai `Delivery.FindByID` yang tidak disentuh) →
dikembalikan, tidak pernah commit.

### Test

- `internal/shared/safedial/denylist_test.go` (3), `validator_test.go` (7 — termasuk 2 yang query DNS
  nyata: `example.com` lolos, `.invalid` ditolak)
- `internal/webhook/entity_test.go` (4), `usecase_unit_test.go` (8, fake Store tanpa Docker),
  `repository_test.go` (13, Postgres asli — round-trip, cross-org, COALESCE parsial, soft delete,
  ketiga CHECK/FK di level DB, pagination, `ClaimDue`/`Reap`/`Purge`, `EXPLAIN` index-hit),
  `handler_test.go` (8, Postgres asli — role matrix atas 6 route, secret sekali + query DB, SSRF
  reject dengan IP literal, update+delete, list deliveries + retry + 409)
- `internal/shared/config/config_test.go` (+2), `authz/authz_test.go` (+20 baris)
- `cmd/api/tenant_isolation_test.go` (+5 kasus)
- `go test ./...` hijau, `golangci-lint run` 0 issues termasuk `gosec`.

### Verifikasi manual end-to-end — `docker compose` + `curl`

`docker compose up -d --build api` + login Owner. Dibuktikan langsung terhadap server nyata:

1. `POST /v1/webhook-endpoints` (`http://host.docker.internal:9200/hook`) → `201`, `whsec_` secret di
   response **sekali**, `secret_prefix` terisi.
2. `GET` list dan `GET /:id` → **tidak ada** field `secret`.
3. DB (`psql`): `secret_hash` 64 char hex, **bukan** raw secret; `secret_prefix` = 14 char.
4. `PATCH {is_active:false, events:["lead.status_changed"]}` → diterapkan, `url` tak tersentuh.
5. `PATCH {events:["lead.deleted"]}` → `400`.
6. `DELETE` → `204`; `GET` setelahnya → `404`.
7. **SSRF** (instance terpisah, `WEBHOOK_ALLOW_PRIVATE_TARGETS=false`): `169.254.169.254`,
   `127.0.0.1`, `10.1.2.3`, `[::1]`, `ftp://` → semua `400 webhook_url_not_allowed`; `https://example.com`
   → `201`; log server mencatat rentang spesifik, response generik; `grep` secret di log → nihil.
8. **Boot**: `APP_ENV=production` + `WEBHOOK_ALLOW_PRIVATE_TARGETS=true` → `config invalid`, proses tidak
   start.
9. `migrate up`→`down`→`up` bersih.

### Menyimpang dari TD

Tidak ada penyimpangan berarti. `td.md` §11 mendaftarkan `signature.go` di paket `internal/webhook`;
di #100 berkas itu **belum ada** — ia #101 (issue #100 checklist: `entity.go (whsec_, backoff())`;
issue #101: `signature.go`). `entity.go` sudah menerbitkan `whsec_` secret karena `create` wajib
mengembalikannya (Aturan #21), tapi HMAC signing payload menunggu #101.

### Catatan untuk session berikutnya (#101)

- `DeliveryRepository.Enqueue` sudah ada dan diuji — #101's `Usecase.Enqueue` memanggilnya **di dalam**
  `Store.InTx` yang sama dengan `lead.Usecase.Create`/`UpdateStatus`.
- `lead.WebhookEnqueuer` interface (konsumen-deklarasi, primitif saja) belum ada — pola persis
  `lead.ActivityRecorder` (#21) / `form.LeadCreator` (#87). Jembatan di `cmd/api/`, `internal/lead`
  tidak pernah mengimpor `internal/webhook`.
- Payload TD §5 memakai `leadJSON` yang **sama** dengan dashboard/API — jangan bentuk kedua.
- `webhook_deliveries.payload` adalah `[]byte` di `Delivery` struct; `deliveryJSON` di handler
  meng-embed-nya sebagai `json.RawMessage` (degradasi ke `null` bila tidak valid).

---

## #101 — Signature HMAC + enqueue: pemicu dari lead

PR: [#107](https://github.com/Pravasta/jualin-crm/pull/107) · branch `feat/101-webhook-signature-enqueue`

Setelah issue ini, sebuah lead yang dibuat menghasilkan baris antrian yang menumpuk sebagai `pending`.
**Belum ada satu pun request keluar** — itu #102.

### Yang tidak direncanakan: `0008` menyimpan secret dalam bentuk yang mustahil dipakai

Ditemukan saat membaca ulang TD §2 sebelum menulis `signature.go`, bukan lewat test — dan tidak ada test
yang bisa menemukannya, karena seluruh #100 hijau justru **karena** belum ada pemanggil yang membutuhkan
secret itu kembali.

`0008` menyimpan signing secret sebagai SHA-256 hash, mengikuti `api_key` (Aturan #20). Tapi untuk webhook
keluar, **kita** yang menandatangani:

```
X-Jualin-Signature: t=<unix>,v1=HMAC-SHA256(secret, "<t>.<body>")
```

HMAC butuh secret **asli** sebagai kunci. SHA-256 searah, dan tidak ada jalur lain: bukan dari
`secret_prefix` (8 dari 49 karakter), bukan dari pelanggan (`createRequest` tidak punya field `secret`).
Dibuktikan sebelum menulis kode apa pun — dua implementasi HMAC independen (Python `hmac`, OpenSSL)
menunjukkan `HMAC(raw) ≠ HMAC(hash)`.

Yang membuat ini layak dicatat bukan perbaikannya, tapi **cara ia lolos**: komentar `0008` sudah menyatakan
dengan benar bahwa kredensial ini "yang PERTAMA dengan arah kepercayaan terbalik", lalu tetap menerapkan
pola default di kolom tepat di bawahnya. Mengenali sebuah kasus itu berbeda tidak otomatis mencegah
penerapan pola yang sudah dikenal.

**Perbaikannya**, disetujui pemilik produk sebelum dikerjakan:

- `migrations/0009_webhook_secret_encrypted.sql` — `secret_hash` → `secret_ciphertext bytea`. Baris yang
  ada **dihapus**, bukan diberi pengisi kosong: endpoint yang secret-nya tidak bisa dipulihkan bukan
  endpoint yang setengah bekerja, dan ciphertext kosong hanya memindahkan kegagalan dari migrasi (terlihat)
  ke worker (senyap).
- `internal/shared/crypter/` — AES-256-GCM, satu-satunya tempat di codebase yang **mengenkripsi**, bukan
  meng-hash. GCM authenticated, jadi satu byte berubah → gagal, bukan plaintext yang berubah.
- `WEBHOOK_SECRET_ENC_KEY` — required tanpa default, min 32 byte. **Bukan** cek production-only seperti
  `WEBHOOK_ALLOW_PRIVATE_TARGETS`: tanpa kunci ini, membuat endpoint gagal total di environment mana pun,
  jadi tidak ada environment di mana boot tanpanya berguna.

Penyimpangan dari Aturan #20 ditulis di tiga tempat — `td.md` §2.1, komentar `0009`, dan
`docs/issues/101-*.md` — **bukan** diselipkan ke `freeze.md` seolah aturannya memang begitu (Aturan #30).
Keputusan apakah freeze perlu klausa baru diserahkan ke #104 lewat ADR, bukan diambil diam-diam di sini.

Regresinya dijaga oleh test yang menguji properti yang benar: **"yang tersimpan bisa kembali menjadi
secret yang ditampilkan"**, bukan "tidak tersimpan sebagai plaintext" — yang dulu juga dipenuhi hash.

### Signature

`Sign(secret, ts, body)` → `t=<unix>,v1=<hex>` atas `"<ts>.<body>"`.

Vektornya **dihitung di luar Go** oleh dua implementasi yang sepakat (Python `hmac`, OpenSSL), lalu
di-pin sebagai konstanta. Ini bukan formalitas: test yang menandatangani dengan `Sign` lalu memverifikasi
dengan `Sign` tetap hijau meski konstruksinya salah dengan cara yang dibagi kedua sisi — menandatangani
`"<body>.<ts>"`, atau menghilangkan pemisah. Bug seperti itu tak terlihat dari dalam dan fatal dari luar:
tidak ada penerima yang mengikuti dokumentasi bisa mereproduksinya.

`TestSign_TimestampIsActuallySigned` sengaja **tidak** membandingkan header utuh — `t` muncul apa adanya
di keluaran, jadi perbandingan itu lolos trivial. Ia mengisolasi bagian `v1` dan membuktikan bagian itu
ikut berubah. Kalau `t` hanya bersanding di luar signature, siapa pun yang menangkap satu request sah bisa
menulis ulang `t` ke waktu sekarang dan memutarnya ulang selamanya.

### Enqueue — kenapa `Enqueuer`, bukan `Usecase.Enqueue`

Checklist issue menulis "`Usecase.Enqueue`". Itu tidak bisa dipenuhi apa adanya: method di `Usecase`
membawa `Store`-nya sendiri, jadi ia akan **membuka transaksi sendiri** — persis satu-satunya hal yang
tidak boleh terjadi di sini.

`webhook.NewEnqueuer(q db.Querier)` terikat pada querier, bukan Store, sehingga `lead` memanggilnya di
dalam transaksinya sendiri. Bentuk yang persis sama dengan `activity.NewRecorder(q)` (#21), untuk alasan
yang sama (TD §5): lead yang commit tanpa baris pengiriman kehilangan event selamanya, dan baris
pengiriman yang commit tanpa lead mengumumkan sesuatu yang tidak ada.

Ini **tidak** melanggar Aturan #32. Yang dilarang aturan itu efek samping **eksternal** di dalam
transaksi; menulis baris antrian adalah operasi database biasa. Justru pemisahan inilah yang membuat
Aturan #32 bisa dipenuhi worker nanti tanpa kehilangan event.

`Enqueue` mengembalikan `(int, error)` — jumlah baris yang ditulis. Nol adalah hasil **normal** (organisasi
tanpa endpoint adalah kasus paling umum), bukan error.

### Payload — satu bentuk lead, bukan dua

`leadJSON` di `handler_http.go` dulu membangun `gin.H` inline. Dipindahkan ke `Lead.Fields()` di
`entity.go`, dan `leadJSON` tinggal `gin.H(l.Fields())` — konversi gratis, `gin.H` adalah alias
`map[string]any`. Alasannya: `usecase.go` butuh bentuk itu untuk membangun payload dan **tidak boleh
mengimpor gin** (ADR-011).

`TestUnit_Create_PayloadIsTheSameShapeAsTheAPI` membandingkan himpunan kunci di kedua arah, jadi menambah
field ke salah satu tanpa yang lain akan gagal — bukan sekadar berkomentar bahwa keduanya "harus sama".

`delivery_id` **tidak** ikut dalam snapshot yang disimpan, berbeda dari gambar di TD §5. Satu snapshot
dipakai bersama semua endpoint yang melanggan, sedangkan `delivery_id` berbeda per baris — dan setiap
baris sudah membawanya sebagai `id`-nya sendiri, stabil lintas percobaan (TD §4.2). Worker menyuntikkannya
saat kirim. Dicatat sebagai kontrak kabel untuk #102 karena **gagal diam-diam**: tanpa penyuntikan itu,
payload tetap valid JSON, hanya kehilangan satu-satunya alat deduplikasi yang kita janjikan.

### Atomisitas dibuktikan dua lapis

Fake membuktikan kabel Go-nya meneruskan error; Postgres sungguhan membuktikan barisnya benar-benar
dibatalkan. Keduanya **dibuktikan bisa gagal**, bukan dipercaya karena hijau:

- Menelan error enqueue di `Create` → `TestUnit_Create_EnqueueFailureAbortsTheLead` merah.
- Mengganti predikat `is_active` dengan `true` di `Enqueuer` → `TestEnqueuer_SkipsEndpoints/inactive` merah.

Keduanya dipulihkan; tidak ada yang di-commit dalam keadaan disabotase.

### Verifikasi manual (crm_be sungguhan + Postgres)

- Endpoint dibuat → `secret` tampil sekali; `GET` daftar & by-id tidak pernah membawanya lagi.
- Lead dibuat → satu baris `pending`, payload persis bentuk TD §5, `occurred_at` UTC `Z`.
- Status diubah `new → contacted` → `changes.status.{from,to}` benar, dan `data.lead.status` = `contacted`
  sementara snapshot `lead.created` **tetap** `new` — properti "dibekukan saat di-enqueue" terbukti hidup.
- **Rantai penuh**: ciphertext dibaca dari database → didekripsi → identik dengan secret yang ditampilkan
  → dipakai menghasilkan `X-Jualin-Signature` yang sah. Ini persis yang mustahil dilakukan sebelum `0009`.
- Endpoint dinonaktifkan → lead berikutnya **tidak** menambah baris.
- `grep` secret di seluruh log server → nihil (Aturan #26).
- `migrate up → down → up` bersih.

---

## #102 — Worker: klaim, kirim, retry, reaper, retensi

PR: [#108](https://github.com/Pravasta/jualin-crm/pull/108) · branch `feat/102-webhook-worker`

Infrastruktur async pertama di produk ini. Setelah issue ini, sebuah lead yang dibuat benar-benar
sampai ke server pelanggan, ditandatangani, dan bisa diverifikasi dari sisi mereka.

### Empat cacat ditemukan saat mereview rencana sendiri, sebelum menulis kode

Rencana awal sempat dinyatakan siap. Membacanya ulang sebagai reviewer — bukan sebagai penulisnya —
menemukan empat hal, satu di antaranya lubang keamanan. Semuanya ditemukan **tanpa** test, karena
tidak satu pun akan ditangkap test yang belum ditulis.

**1. Keep-alive melewati daftar tolak SSRF.** Rencananya menaruh cek deny-list di `DialContext`.
Tapi `http.Transport` hanya memanggilnya saat butuh koneksi **baru** — pengiriman kedua ke endpoint
yang sama memakai koneksi pool dan tidak pernah dicek lagi. Jendela DNS rebinding justru terbuka untuk
kasus paling umum: endpoint yang sering menerima event. TD §3.2 mensyaratkan validasi **tiap kirim**,
dan rencana itu tidak memenuhinya.

Diperbaiki dengan `DisableKeepAlives: true` — satu koneksi per pengiriman, deny-list dievaluasi setiap
kali. Dijaga `TestHTTPClient_NeverReusesConnections`, yang **menghitung koneksi yang diterima server**:
satu-satunya cara mengamati ini dari luar, karena status code-nya identik di kedua keadaan. Dibuktikan
bisa gagal — mengembalikan keep-alive membuat 3 request jadi 1 koneksi, dan test merah.

Pelajaran yang layak dibawa: menaruh pemeriksaan keamanan di sebuah hook tidak berarti apa-apa sebelum
memastikan hook itu dipanggil setiap kali. Pertanyaannya bukan "apakah ceknya benar" tapi **"apa yang
membuat ini berjalan pada request kedua?"**

**2. Kegagalan `Decrypt` tanpa perlakuan.** Rotasi `WEBHOOK_SECRET_ENC_KEY` membuat setiap secret tak
bisa didekripsi. Tanpa penanganan khusus jalur itu jatuh ke "transport error" dan seluruh antrian
berputar retry 6 jam sekali selamanya menuju kegagalan yang sama. Sekarang `failed` permanen.

**3. Hanya IP pertama yang di-dial.** Begitu resolusi diambil alih dari `net/http`, keandalan yang
biasanya gratis jadi tanggung jawab kita: host dengan beberapa A record gagal total kalau yang pertama
mati. Diperbaiki dengan iterasi seluruh alamat tervalidasi.

**4. D5 ambigu, dan bedanya besar.** *"5 percobaan"* di sebelah daftar **lima** jeda tidak konsisten —
lima percobaan hanya punya empat celah, jadi jeda `6j` mustahil terpakai. Dua bacaan yang sama-sama
cocok dengan acceptance criteria, tapi jendela retry totalnya 2j36m vs 8j36m. Itu keputusan produk yang
terlihat pelanggan, bukan detail implementasi, jadi **ditanyakan, bukan dipilih diam-diam**. Keputusan:
5 percobaan ulang setelah kiriman pertama (6 panggilan HTTP). Teks D5 diperbarui.

### Temuan yang membalik anggapan TD: apa yang sebenarnya menjamin exactly-once

TD §4.1 menyiratkan `FOR UPDATE SKIP LOCKED` yang membuat banyak instance aman. Saat menguji apakah
test konkurensi benar-benar bisa gagal, ternyata **tidak**: menghapus `SKIP LOCKED` tidak membuatnya
merah.

Sebabnya, tanpa klausa itu transaksi tidak melewati baris terkunci melainkan **memblokir**; saat kunci
lepas, PostgreSQL mengevaluasi ulang subquery terhadap versi baris terbaru — yang kini `delivering` dan
tidak lagi cocok `WHERE status = 'pending'`. Korektnes tetap terjaga, oleh predikat status.

Pembagian sebenarnya:

- **`WHERE status = 'pending'` + row lock → exactly-once**
- **`SKIP LOCKED` → liveness**: claimer tidak antre di belakang claimer lain

Keduanya kini diuji **terpisah**, dan yang kedua lewat test yang menahan kunci di transaksi lain lalu
memanggil `ClaimDue` dengan deadline pendek — tanpa `SKIP LOCKED` panggilan itu menggantung sampai
deadline lewat, dan test merah. Sebelumnya klaim di komentar test-ku sendiri salah; diperbaiki di sana,
dan dicatat di `docs/issues/102` supaya teks TD §4.1 ikut diperbaiki di #104. Ini penting bukan demi
ketelitian: orang berikutnya yang percaya `SKIP LOCKED` sudah cukup bisa menghapus predikat status dan
memecah jaminan yang sebenarnya.

### Graceful shutdown melampaui minimum TD §13 — dan TD-nya ikut dikoreksi

§13 aslinya menulis sisa batch *"ditinggal `delivering` dan diambil reaper setelah restart"*. Benar,
tapi mahal: baris yang sudah diklaim namun **belum dicoba sama sekali** menunggu ambang reaper 10 menit.
Pada instance tunggal, setiap deploy menunda sisa batch 10 menit tanpa alasan.

`Release` mengembalikannya ke `pending` tanpa menambah `attempt` — tidak ada yang dicoba, jadi tidak ada
yang perlu dicatat. Yang benar-benar sedang di udara saat batas waktu lewat **tetap** ditinggal
`delivering`: instance lain tidak boleh merebut pengiriman yang mungkin masih berjalan, dan di situ §13
yang asli tetap berlaku. `td.md` diperbarui (§4.4 baru + baris §13) di PR yang sama — bukan cuma dicatat
di notes, karena kalau tidak, dokumennya jadi salah.

### Yang lain

- **`ClaimDue` mengambil URL + secret lewat JOIN**, bukan lookup terpisah. Lookup kedua meninggalkan
  jendela di mana endpoint diedit atau dihapus antara klaim dan kirim. `ClaimedDelivery` adalah tipe
  terpisah dari `Delivery` supaya `secret_ciphertext` **tidak pernah** ada di struct yang di-serialize
  handler.
- **`http.ErrUseLastResponse`**, bukan error, dari `CheckRedirect`: `3xx` sampai ke pemetaan status
  sebagai respons biasa, jadi satu jalur untuk semua kode, bukan cabang khusus.
- **`MarkResult` dijaga `WHERE status = 'delivering'`** — kalau reaper sudah merebut baris ini, hasil
  kita dibuang, bukan menimpa milik pemilik barunya.
- **Urutan shutdown**: HTTP drain dulu (tidak ada lead baru masuk antrian), baru worker berhenti.
  Sempat ditulis `defer onDrained()`, yang membuatnya jalan setelah log `"shutdown complete"` dan
  dilewati sama sekali pada `os.Exit(1)`; diganti pemanggilan eksplisit.

### Verifikasi manual — penerima sungguhan, bukan log pengirim

Kriteria #1 hanya bisa dibuktikan oleh server penerima yang mencatat apa yang diterima. Ditulis satu
penerima kecil yang memverifikasi signature memakai **skema terdokumentasi, bukan kode kita** —
`hmac.New(sha256, secret)` atas `"<t>.<body>"`, disusun dari `td.md` §2.

```
[ok]       signature cocok=true (umur 0s) len(body)=716
           body: {"delivery_id":"01a062da-…","data":{"lead":{…}},"event":"lead.created",…}
[500]      → pending, attempt 1, next_attempt_at 55 detik lagi (backoff 1m)
[400]      → failed, attempt 0, tidak pernah diulang
[redirect] → failed, response_status 302, error "redirect not followed"
             0 request ke 169.254.169.254 di seluruh log
[status]   → changes {"status":{"from":"new","to":"contacted"}}
SIGTERM    → "webhook worker stopped" → 0 baris tertinggal 'delivering'
```

`umur 0s` di **setiap** retry membuktikan timestamp ditandatangani ulang saat kirim, bukan saat
enqueue — retry enam jam kemudian tetap berada dalam toleransi 5 menit penerima.

`go test -race ./...` bersih (satu data race ditemukan di test sendiri: `srv.Config.ConnState` diset
setelah `httptest.NewServer` mulai melayani — diperbaiki dengan `NewUnstartedServer`).
`golangci-lint run` 0 issues.

---

## #103 — Dashboard: /connect/webhook

PR: [#109](https://github.com/Pravasta/jualin-crm/pull/109) · branch `feat/103-dashboard-webhook`

Issue pertama Phase 7 yang menyentuh `crm_dashboard`, bukan Go. Setelah ini, seluruh siklus webhook
bisa dilakukan Owner/Admin **tanpa `curl`**: daftar endpoint, buat (dengan secret tampil sekali), ubah
event/URL, nonaktifkan, lihat riwayat pengiriman, kirim ulang yang gagal.

### `lib/webhooks.ts` — bentuk terverifikasi terhadap handler Go, bukan diasumsikan

`WebhookEndpoint` vs `CreatedWebhookEndpoint`: `secret` **hanya** ada di tipe kedua, dan hanya
`createWebhookEndpoint` mengembalikannya. Merender secret dari daftar adalah **type error**, bukan bug
runtime — pola `CreatedAPIKey` (#48). Dijaga `@ts-expect-error` di `webhooks.test.ts`: kalau baris itu
berhenti error, tipe daftar sudah menumbuhkan field yang membiarkan layar daftar merender kredensial.

`WEBHOOK_EVENTS` union (`lead.created`, `lead.status_changed`) — bukan `string`. Test membandingkannya
persis dengan `webhook.KnownEvents` di Go: tidak ada yang menghubungkan dua bahasa saat kompilasi, jadi
rename di salah satu sisi gagal di test, bukan sebagai langganan yang diam-diam mati.

`isWebhookUrlNotAllowed(err)` — `webhook_url_not_allowed` adalah 400 **polos**, bukan `validation_failed`,
jadi tidak ada `details[]` untuk menaruhnya di bawah field. Helper ini (bentuk `isLeadAlreadyConverted`)
dipakai dialog buat **dan** editor untuk mengarahkan pesan ke bawah field URL. Tinggal di `lib/`, bukan
di salah satu komponen, supaya keduanya tidak menyimpang. Pesannya selalu apa adanya dari backend —
kevaguannya (`"URL webhook tidak diizinkan."`) adalah keputusan keamanan TD §7, bukan celah untuk diisi.

### `canCloseCreateDialog` — penjaga tutup dialog ditarik keluar jadi fungsi murni

Tahap reveal adalah **satu-satunya** tempat di produk yang menampilkan kredensial yang tak bisa
dipulihkan. Penjaga tutupnya (`step !== "reveal" || confirmedSaved`) ditarik ke `lib/webhooks.ts`
bukan dibiarkan inline di JSX — karena cara lain membuktikannya adalah browser, dan codebase ini
sengaja menaruh test visual di luar cakupan (TD phase 3 §9). Alasan yang sama menaruh `canManageWebhooks`
dan `lib/nav.ts` di berkasnya sendiri. Diuji 3 kasus; komponen memanggilnya di `onOpenChange` — satu
titik yang dilewati X, Escape, dan klik backdrop.

### Kartu Webhook di `/connect` — deskripsinya diperbaiki, bukan cuma diaktifkan

Selama masih placeholder, deskripsinya berbunyi *"Terima event dari platform lain secara real-time"* —
itu **inbound** webhook (Phase 7.5), fitur yang sama sekali berbeda. Diganti jadi *"Kirim event ke
sistem Anda sendiri…"*. Kartu berhenti berbunyi *"belum tersedia"* — bukan *"terkunci oleh paket"*,
yang lahir di Phase 8 (ADR-012 §4) dan tidak ada sekarang.

### `formatDateTimeID` baru di `lib/date.ts`

Riwayat pengiriman butuh **jam**, bukan cuma tanggal: pengiriman berjarak detik dan retry-nya berjam
kemudian, jadi tanggal saja tidak membedakan dua baris. `formatDateID` yang ada tetap dipakai di
tempat lain.

### Verifikasi visual terhadap `crm_be` sungguhan (browser)

- Kartu Webhook aktif di `/connect`, `href="/connect/webhook"`, teks "belum tersedia" nihil
- Daftar: URL, `secret_prefix…`, badge event, status Aktif/Nonaktif, tanggal, "Kelola"
- Dialog buat: dua event pre-checked; URL privat/tak-teresolusi → **"URL webhook tidak diizinkan."
  muncul di bawah field URL**, bukan banner
- **Penjaga secret**: Escape tertahan, klik backdrop tertahan, tidak ada tombol X di tahap reveal,
  "Selesai" nonaktif sampai checkbox dicentang, lalu tutup + daftar refresh
- Editor: pengaturan (URL, event, keterangan, Aktif dengan penjelasan), riwayat, blok Hapus endpoint
  (border merah, mengarahkan ke "nonaktifkan saja")
- Riwayat: Waktu / Event / Status (Gagal merah + detail error apa adanya, mis.
  `transport error: webhook url not allowed: 169.254.169.254 is in a denied range`) / Percobaan ke-N
- **Kirim ulang**: `Gagal ke-5` → klik → `Menunggu ke-1` dengan detail error baru, di tabel yang sama
- **Kirim ulang pada baris non-`failed` → 409**: diverifikasi lewat API (`delivery_not_retryable`,
  *"Hanya pengiriman yang gagal yang bisa dikirim ulang."*), **bukan** visual — tombolnya hanya muncul
  untuk baris `failed`, jadi 409 hanya terjadi pada balapan worker antara render dan klik. Kode
  menanganinya: `retryWebhookDelivery` melempar, `DeliveryHistory` menangkap dan merender
  `setRetryError({ id, message })` inline per baris, bukan tombol yang diam
- **Manager**: `/connect/webhook` **dan** `/connect/webhook/:id` sama-sama menampilkan "Pengelolaan
  webhook tidak tersedia untuk role Anda." — **nol** panggilan `/v1/webhook-endpoints` di Network
  (hanya `/v1/me` + notifikasi/metrics dari layout)

`npm run typecheck && lint && test (121) && build` bersih.

---

## #104 — Dokumentasi verifikasi + penutup Phase 7

PR: [#110](https://github.com/Pravasta/jualin-crm/pull/110) · branch `feat/104-webhook-docs-and-phase7-close`

Penutup phase. Tidak menyentuh Go — halaman docs baru di `crm_dashboard`, satu ADR, empat dokumen
arsitektur, satu berkas testing flow, dan review 14 AC PRD terhadap bukti yang sudah ada.

### `/connect/webhook/docs` — halaman menghadap-penerima

Pola `/connect/api/docs` (#49), tapi dengan satu perbedaan yang menentukan: **contoh di halaman ini
benar-benar bisa dijalankan apa adanya.** Contoh `curl` di #49 butuh secret kami tertanam untuk
memanggil kami — halaman statis tidak bisa memegangnya (Aturan #21), jadi ia hanya menampilkan bentuk
dengan placeholder. Contoh **verifikasi** signature tidak butuh secret kami sama sekali: penerima
menempelkan secret yang **ia** simpan saat membuat endpoint, dan kodenya jalan. Itu yang membuat
AC #2 ("contoh disalin, dijalankan, benar-benar memvalidasi") jujur, bukan aspiratif.

`lib/webhook-docs.ts` memegang tiga contoh (Node/PHP/Python) sebagai konstanta string, satu builder
konsep. `webhook-docs.test.ts` (19 test) mengunci konstruksi yang benar dengan cara yang sama seperti
`signature_test.go`: sebuah fungsi `verifyAsDocumented` yang mengikuti langkah yang **dicetak halaman**
(HMAC atas `"<t>.<body mentah>"`, banding hex) diuji terhadap vektor yang ditandatangani cara
`signature.go` menandatangani — kalau contohnya menulis `"<body>.<t>"` atau body yang di-marshal
ulang, test merah. Plus: payload diubah satu byte → ditolak; `t` digeser → ditolak (membuktikan
timestamp benar-benar di dalam input HMAC); tidak ada contoh yang menyematkan secret yang terlihat
asli.

Tautan "Dokumentasi verifikasi" ditambahkan di `webhooks-screen.tsx` (di bawah deskripsi) dan
`webhook-editor.tsx` (tombol di header). Kartu `/connect` sudah aktif sejak #103 — tidak disentuh.

### ADR-013 — penyimpangan Aturan #20 diformalkan

Keputusan pemilik produk (3 Sep 2026): **lewat ADR baru**, bukan pengecualian ber-scope-phase.
[ADR-013](../../decisions/ADR-013-signing-secret-storage.md) menambah klausa bertingkat batas ke
Aturan #20 — kredensial yang **kita** pakai untuk menghasilkan bukti (arah kepercayaan terbalik)
disimpan terenkripsi reversibel dengan kunci di environment, **hanya bila** tidak ada alternatif hash
yang bisa memenuhi fungsinya. Isi Aturan #20 di `freeze.md` tidak diubah — hanya anotasi `> ⚠️`
penunjuk ke ADR-013 (mekanisme sama seperti ADR-011 terhadap Aturan #8). Batasnya tegas: inbound
webhook (Phase 7.5) kembali ke hash — pengirim yang memegang rahasianya, kita yang memverifikasi.

### Dokumen arsitektur (TD §15)

- **`api.md`** — bab baru *Webhook Keluar* (bentuk payload, signature, retry, at-least-once, SSRF
  ringkas, tabel endpoint, retensi); dua error code ke katalog (`webhook_url_not_allowed`,
  `delivery_not_retryable`); angka retry/interval/batch + biaya `DisableKeepAlives` yang belum diukur
  masuk daftar *Angka batasnya belum pernah diukur* yang sudah ada. Satu parentetis lama ("kredensial
  baru ... webhook Phase 7 ... masuk kelas rate limit pertama") diperbaiki jadi "webhook **inbound**
  Phase 7.5" — outbound tidak menerima traffic, jadi tidak menambah kelas rate limit apa pun.
- **`authentication.md`** — bagian *Signing secret webhook (`whsec_`)*, baris keempat kredensial dan
  yang pertama dengan arah kepercayaan terbalik: tabel banding lawan tiga kredensial masuk,
  penyimpanan terenkripsi (→ ADR-013), tampil sekali, tak pernah di-log, konsekuensi rotasi kunci.
- **`authorization.md`** — *Matriks (Phase 7)*: lima `Action` `webhook.*`, Owner/Admin saja.
- **`multi-tenancy.md`** — pengecualian **jenis baru**, seksi terpisah: sebuah **index**
  (`ix_webhook_deliveries_claim`) sengaja tidak berawalan `organization_id`. Ini Aturan **#16**
  (index), bukan Aturan #2 (composite FK) seperti empat pengecualian sebelumnya — worker infrastruktur
  lintas-org, `organization_id` adalah hasil klaim.
- **`td.md` §4.1** — blok koreksi: `WHERE status='pending'` + row lock yang menjamin exactly-once,
  `SKIP LOCKED` = liveness. Kewajiban dari `docs/issues/102`. **`td.md` §8** — "Empat `Action`" di
  sebelah daftar lima diperbaiki jadi "Lima".

### `docs/testing/flow/09-webhook.md` (baru)

Prosedur manusia: `receiver.py` stdlib yang memverifikasi signature dengan skema terdokumentasi →
daftarkan endpoint → buat lead → request sampai (AC #1) → contoh dari halaman docs memvalidasi
(AC #2) → payload diubah satu byte → ditolak (AC #2) → status change dengan `changes` → endpoint mati
→ retry + kirim ulang manual (AC #6/#10/#11) → SSRF URL privat ditolak (AC #4) → gerbang role. Tabel
urutan `README.md` bertambah baris 9; `06-checklist-akhir.md` bertambah bagian 9; baris usang di
`08-formulir-embed.md` ("kartu Webhook Belum tersedia") diperbaiki.

### Review 14 acceptance criteria PRD

Dicek satu per satu terhadap bukti nyata di `notes.md` #100–#103 dan test yang disebutkan — bukan
diasumsikan dari ingatan. AC #1 dan #2 (butuh server penerima sungguhan + stack jalan) awalnya
ditandai **langkah manusia berikutnya** lewat `09-webhook.md`; verifikasi itu **kemudian dijalankan**
pada sesi browser 3 September 2026 — lihat bagian *Verifikasi manual dijalankan* di bawah tabel.

| # | Kriteria | Bukti | Status |
|---|---|---|---|
| 1 | URL dari dashboard → lead → request sampai, diverifikasi dari server penerima nyata | #102 verifikasi manual + **sesi browser 3 Sep 2026**: lead #6 → `receiver.py` `[OK] signature_valid=True`, HTTP 200 (`09-webhook.md` §9.3) | ✅ |
| 2 | Payload signature bisa diverifikasi penerima; secret ditampilkan sekali | #101 rantai penuh ciphertext→decrypt→sign; #103 dialog reveal sekali + `@ts-expect-error`; #104 `webhook-docs.test.ts` `verifyAsDocumented` lulus terhadap vektor `signature.go`; **sesi browser**: contoh Python halaman docs = `receiver.py`, memvalidasi kiriman nyata (§9.4) | ✅ |
| 3 | Signature menolak payload diubah satu byte, dan replay di luar toleransi | `signature_test.go` (body 1 byte, `t` diubah); `webhook-docs.test.ts` (tamper ditolak, `t` digeser ditolak); toleransi 5 menit didokumentasikan ke penerima (`Sign` doc comment, halaman docs). §9.5 (tamper manual di `receiver.py`) dilewati di sesi browser — dikunci dua test di atas | ✅ (test) |
| 4 | URL privat/loopback/link-local ditolak saat disimpan **dan** saat dikirim (DNS berubah) | `safedial/denylist_test.go` (~35 alamat, IPv4+IPv6, `::ffff:` mapped), `validator_test.go` (resolve DNS nyata); #100 manual curl SSRF semua `400`; #102 saat kirim: "transport error ... denied range" + 0 request ke `169.254.169.254`. **Sesi browser tidak menguji ini** — butuh `WEBHOOK_ALLOW_PRIVATE_TARGETS=false` (mematikan receiver lokal) | ✅ (test + #100) |
| 5 | Redirect tidak pernah diikuti | `worker_test.go` `3xx`→`failed`; #102 manual "[redirect] → failed, response_status 302, redirect not followed, 0 request ke 169.254.169.254"; `CheckRedirect` mengembalikan `http.ErrUseLastResponse` (satu jalur, bukan cabang khusus) | ✅ |
| 6 | `5xx` dicoba ulang dengan jeda menaik; setelah percobaan terakhir → gagal permanen | `entity_test.go` `backoff()` terhadap jadwal persis D5; `worker_test.go` `5xx`→retry dengan `next_attempt_at` sesuai backoff, percobaan ke-6 → `failed`; #102 manual "[500] → pending, attempt 1, next 55 detik"; `retryOrFail` cek `next > MaxAttempts` | ✅ |
| 7 | `4xx` tidak dicoba ulang kecuali `429` | `worker_test.go` `4xx`→`failed` tanpa retry, `429`→retry; switch di `deliver()` | ✅ |
| 8 | Pengiriman tidak pernah di dalam transaksi database pemicunya (Aturan #32) | `enqueuer.go` terikat `db.Querier` (bukan `Store`), menulis baris antrian **di dalam** tx `lead`; HTTP terjadi di worker, **di luar** tx; `internal/lead` atomicity test (fake + rollback Postgres sungguhan `repository_atomicity_test.go`) | ✅ |
| 9 | Membuat lead tetap berhasil walau endpoint webhook mati total | Pengiriman HTTP asinkron di worker, tak pernah menyentuh jalur `lead.Create`; **sesi browser**: `receiver.py` dimatikan → ubah status lead **tetap berhasil** (badge & riwayat lead berubah), pengiriman ke endpoint mati jadi `Menunggu`/`Gagal` terpisah (§9.7) | ✅ |
| 10 | Owner melihat riwayat per endpoint: waktu, status, kode respons, percobaan ke-N | #103 `delivery-history.tsx` (kolom Waktu/Event/Status+HTTP N/Percobaan ke-N), diverifikasi visual di #103; `handler_test.go` list deliveries berpaginasi; `09-webhook.md` §9.3/§9.7 | ✅ |
| 11 | Kirim ulang manual untuk pengiriman gagal, hasilnya di riwayat yang sama | #103 tombol "Kirim ulang" (hanya baris `failed`) + refetch; `handler_test.go` retry `200` lalu retry kedua `409`; #103 visual "Gagal ke-5 → Menunggu ke-1 di tabel yang sama"; `09-webhook.md` §9.7 | ✅ |
| 12 | Dua instance bersamaan tidak mengirim ganda — dibuktikan di bawah konkurensi nyata | `worker_concurrency_test.go` — N goroutine paralel terhadap Postgres sungguhan, tiap baris diklaim **tepat sekali**; **dibuktikan bisa gagal** dan pembagian exactly-once/liveness diuji terpisah (#102, TD §4.1 diperbaiki #104) | ✅ |
| 13 | Endpoint org lain tidak terlihat/disunting — harness isolasi tenant + terbukti bisa gagal | `tenant_isolation_test.go` +5 kasus (#100: `GET/PATCH/DELETE /v1/webhook-endpoints/:id`, `…/deliveries`, `POST …/retry`) → semua `404` lintas org; **dibuktikan bisa gagal** (predikat `organization_id` dihapus dari `FindByID` → 3 subtest bocor `200`, dikembalikan); gerbang role UI di `09-webhook.md` §9.9 | ✅ |
| 14 | `webhook_deliveries` punya retensi tertulis — tidak tumbuh selamanya | PRD D8 + `td.md` §10 + `api.md` bab *Webhook Keluar → Retensi* (baru #104); `WEBHOOK_DELIVERY_RETENTION_DAYS=30`; `repository_test.go` `Purge` tidak pernah menghapus `pending`; throttle 1×/jam dari worker loop. **Poin terbuka diwarisi sadar**: pola retensi malas belum diuji di volume produksi (`docs/issues/047`, `102`) — pemicu peninjauan tercatat | ✅ |

### `docs/issues/` Phase 7 — ditinjau

- `101-*.md` dan `102-*.md` diperbarui: poin selesai dicentang dengan cara penyelesaiannya, poin
  terbuka dinyatakan ulang dengan **pemicu eksplisit** (rotasi kunci enkripsi → sebelum pelanggan
  produksi; `timestamp +07:00` vs Aturan #33 → perubahan API lintas klien berikutnya; ambang reaper →
  endpoint sah butuh >10 menit; `DisableKeepAlives` & retensi malas → traffic produksi, bareng #047).
- **`100-*.md` dan `103-*.md` sengaja tidak dibuat** — `notes.md` keduanya menyatakan "tidak ada
  penyimpangan berarti" / verifikasi bersih; skill `jualin-issue-log` melarang berkas kosong demi
  kelengkapan.

### Menyimpang dari TD

Tidak ada penyimpangan. Halaman docs mengikuti bentuk `/connect/api/docs` (#49) yang TD §2 dan §15
sebut. Satu tambahan di luar checklist issue: tombol "Dokumentasi verifikasi" di dua layar webhook —
tanpa itu halaman docs tidak bisa ditemukan tanpa mengetik URL, sama seperti #49 menautkan dari
`/connect/api`.

### Cek

`npm run typecheck && lint (0/0) && test (140, +19 dari `webhook-docs.test.ts`) && build` bersih.
Sisi Go tidak disentuh (perubahan hanya `.md`); `go build ./...` + `go test ./...` dijalankan tetap
sebagai konfirmasi — bersih, `internal/webhook` termasuk.

### Verifikasi manual dijalankan — sesi browser 3 September 2026 (setelah merge #110)

Pemilik produk menjalankan `09-webhook.md` terhadap `docker compose` + `crm_dashboard` + `receiver.py`
lokal, didampingi Claude lewat browser automation. Hasil:

| AC / langkah | Hasil |
|---|---|
| §9.1 kartu Webhook aktif | ✅ tiga kartu aktif, tak ada "belum tersedia"/"terkunci paket" |
| §9.2 secret sekali + penjaga | ✅ Escape & klik-backdrop tertahan; "Selesai" nonaktif sampai checkbox; daftar + DB hanya `secret_prefix` + `secret_ciphertext` 77 byte (12 nonce + 49 plaintext + 16 tag GCM), bukan raw |
| **AC #1** (§9.3) | ✅ lead #6 → worker kirim → `receiver.py`: `[OK] signature_valid=True within_tolerance=True`, HTTP 200; riwayat dashboard "Berhasil HTTP 200" dengan jam. Diverifikasi **dari sisi penerima**, bukan log pengirim |
| **AC #2** (§9.4) | ✅ contoh Python di `/connect/webhook/docs` = konstruksi `receiver.py` persis (`hmac.new(secret, f"{t}.".encode()+raw, sha256)`), yang sudah memvalidasi kiriman nyata. §9.5 (tamper 1 byte) dilewati manual — dikunci `webhook-docs.test.ts` + `signature_test.go` |
| §9.6 `lead.status_changed` | ✅ `changes.status:{from:"new",to:"contacted"}`, `data.lead.status="contacted"`, `version:2`; snapshot `lead.created` **tetap** `status:"new"`, `version:1` — dibekukan saat event. `occurred_at`/`created_at` `Z` (org timezone UTC — item `+07:00` `docs/issues/101` bergantung timezone, tak tereproduksi di sini) |
| **AC #6, #10, #11** (§9.7) | ✅ receiver dimatikan → `Menunggu` + `transport error: dial "host.docker.internal": ... network is unreachable` + `Percobaan ke-1`, `next_attempt_at` +1m (backoff pertama). Baris dipaksa `failed` lewat SQL (menunggu 8j tak praktis; ladder retry sudah di `worker_test.go`), lalu **"Kirim ulang"** → `Menunggu` → `Berhasil HTTP 200` di **baris & `delivery_id` yang sama** (`01a06812-…`, id tak berubah — dedup handle stabil). Retry non-`failed` → `409 delivery_not_retryable` (tombol hanya muncul untuk `failed`; `handler_test.go`) |
| §9.9 gerbang role (Manager) | ✅ `/connect/webhook`, `/webhook/:id`, `/webhook/docs` → "…tidak tersedia untuk role Anda"; **nol** panggilan `/v1/webhook-endpoints` (hanya `/v1/me` + notifikasi/metrics dari layout) |
| §9.8 SSRF saat disimpan | ⏭️ **tidak dijalankan** — butuh `WEBHOOK_ALLOW_PRIVATE_TARGETS=false` + restart api, yang mematikan receiver `host.docker.internal` (§9.2–§9.7 tak bisa bersamaan). Dikunci `internal/shared/safedial/denylist_test.go` (~35 alamat) + `curl` #100. **Bug di `09-webhook.md`** yang ditulis #104: setup meminta flag `true` lalu §9.8 mengharap penolakan — kontradiksi. Diperbaiki di PR follow-up ini (§9.8 jadi bagian terpisah dengan syarat env sendiri) |

Dengan ini seluruh 14 AC PRD terkonfirmasi: 12 dari test + verifikasi #100–#103, dan #1/#2/#6/#10/#11
plus gerbang role dari sesi browser di atas. Endpoint uji di-soft-delete setelah selesai; `receiver.py`
(berkas sementara) dihapus dari root repo.
