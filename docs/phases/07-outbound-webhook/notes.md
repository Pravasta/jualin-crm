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
