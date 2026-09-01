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
