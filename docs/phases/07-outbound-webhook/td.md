# Phase 7 — Outbound Webhook · TD

> **Bagaimana.** Apa & kenapa di [`prd.md`](./prd.md).
> Ini **delta** untuk phase ini. Aturan yang sudah ada di [`freeze.md`](../../architecture/freeze.md) tidak diulang, hanya dirujuk.

---

## 1. Schema — migration `0008_webhooks`

Nomor `0008` belum dipesan freeze 8.4 (tabelnya berhenti di `0007`). Dua tabel, satu migration.

```sql
CREATE TABLE webhook_endpoints (
    id               uuid PRIMARY KEY,
    organization_id  uuid NOT NULL REFERENCES organizations (id),
    url              text NOT NULL,
    secret_hash      text NOT NULL,
    secret_prefix    text NOT NULL,
    events           text[] NOT NULL,
    description      text NOT NULL DEFAULT '',
    is_active        boolean NOT NULL DEFAULT true,
    created_by_membership_id uuid,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    deleted_at       timestamptz,

    CONSTRAINT uq_webhook_endpoints_id_org UNIQUE (id, organization_id),
    CONSTRAINT ck_webhook_endpoints_url_scheme CHECK (url LIKE 'https://%' OR url LIKE 'http://%'),
    CONSTRAINT ck_webhook_endpoints_events_not_empty CHECK (cardinality(events) > 0),
    CONSTRAINT fk_webhook_endpoints_created_by
        FOREIGN KEY (created_by_membership_id, organization_id)
        REFERENCES memberships (id, organization_id)
);

CREATE INDEX ix_webhook_endpoints_org ON webhook_endpoints (organization_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE TABLE webhook_deliveries (
    id               uuid PRIMARY KEY,
    organization_id  uuid NOT NULL REFERENCES organizations (id),
    endpoint_id      uuid NOT NULL,
    event_type       text NOT NULL,
    payload          jsonb NOT NULL,
    status           text NOT NULL DEFAULT 'pending',
    attempt          integer NOT NULL DEFAULT 0,
    next_attempt_at  timestamptz NOT NULL DEFAULT now(),
    response_status  integer,
    error            text,
    delivering_since timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT uq_webhook_deliveries_id_org UNIQUE (id, organization_id),
    CONSTRAINT ck_webhook_deliveries_status
        CHECK (status IN ('pending', 'delivering', 'succeeded', 'failed')),
    CONSTRAINT fk_webhook_deliveries_endpoint
        FOREIGN KEY (endpoint_id, organization_id)
        REFERENCES webhook_endpoints (id, organization_id)
);

-- Riwayat per endpoint, dibaca dashboard (kriteria #10). Tenant-aware, berawalan
-- organization_id sesuai Aturan #16.
CREATE INDEX ix_webhook_deliveries_org ON webhook_deliveries
    (organization_id, endpoint_id, created_at DESC);

-- Antrian worker. SENGAJA TIDAK berawalan organization_id — lihat §1.2.
CREATE INDEX ix_webhook_deliveries_claim ON webhook_deliveries (next_attempt_at)
    WHERE status = 'pending';
```

`ck_webhook_endpoints_url_scheme` sengaja **tidak** memaksa `https://` di level database: pengembangan
lokal butuh `http://localhost` (§9, `WEBHOOK_ALLOW_PRIVATE_TARGETS`). Penegakan `https` di produksi
ada di usecase, bukan di `CHECK` — kalau di database, ia tidak bisa dilonggarkan per-environment tanpa
migration.

### 1.1 `payload` sebagai JSONB — alasan tertulis (Aturan #17)

Ia **snapshot event pada saat terjadi**, bukan data yang di-query. Payload harus dibekukan saat
di-enqueue, bukan dibangun ulang saat dikirim — kalau lead berubah status tiga kali dalam lima menit,
tiga pengiriman itu harus membawa tiga isi yang berbeda, bukan tiga salinan keadaan terakhir. Tidak
pernah ada query yang memfilter berdasarkan isinya; ia dibaca utuh saat mengirim dan saat menampilkan
riwayat.

### 1.2 `ix_webhook_deliveries_claim` — pengecualian tertulis atas Aturan #16

Aturan #16: *"Index tenant-aware selalu berawalan `organization_id`"*. Index ini **sengaja tidak**,
dan itu bukan kelalaian:

> Worker mengambil kerja **lintas seluruh organization** — ia infrastruktur, bukan pemanggil
> tenant-scoped. `organization_id` adalah **hasil** dari baris yang terambil, bukan input untuk
> mencarinya. Mengawali index dengan `organization_id` membuatnya tidak terpakai sama sekali oleh
> query klaim, yang predikatnya hanya `status` dan `next_attempt_at`.

Bentuknya sekelas pengecualian yang sudah ada tiga kali: `apikey.FindByKeyID`, `form.FindByPublicKey`,
dan `device_tokens.token` — semuanya lookup di mana organization adalah keluaran. Perbedaannya, di sini
yang dikecualikan **index**, bukan constraint unik. Ditambahkan ke `multi-tenancy.md` sebagai
pengecualian jenis baru, bukan diselipkan ke daftar yang sudah ada (§15).

Index riwayat (`ix_webhook_deliveries_org`) **tetap** berawalan `organization_id` — ia memang dibaca
dari jalur tenant-scoped.

---

## 2. Signature (keputusan D4)

```
signed_payload = "<timestamp>" + "." + <body mentah, byte demi byte>
signature      = HMAC-SHA256(secret, signed_payload)
header         = X-Jualin-Signature: t=<timestamp>,v1=<signature hex>
```

| Aspek | Ketentuan |
|---|---|
| Secret | 32 byte `crypto/rand` → base64url. Format `whsec_<43 karakter>` |
| Penyimpanan | **SHA-256 hash**, sama seperti API key (Aturan #20). `secret_prefix` (8 karakter pertama) disimpan terpisah untuk ditampilkan di daftar |
| Tampil | **Sekali**, saat endpoint dibuat (Aturan #21) |
| Toleransi | 5 menit — dinyatakan di dokumentasi untuk penerima, bukan ditegakkan pengirim |

**Kenapa timestamp ikut ditandatangani.** Kalau `t` berada di luar `signed_payload`, penyerang yang
menangkap satu request sah bisa mengubah `t` ke waktu sekarang dan memutarnya ulang selamanya —
signature-nya tetap cocok karena ia hanya menutupi body. Menyatukan keduanya membuat perubahan `t`
merusak signature. Ini kesalahan yang sering terjadi pada implementasi buatan sendiri, dan alasan
bentuk `t=…,v1=…` dipilih apa adanya dari pola yang sudah teruji luas.

**`whsec_` sengaja tidak menyerupai `jln_live_` maupun `pk_`.** Tiga kredensial dengan aturan
berlawanan tidak boleh terlihat mirip — dan yang ini arahnya terbalik dari keduanya: ia bukti **kita**
kepada pihak lain, bukan sebaliknya.

Dokumentasi verifikasi untuk penerima (contoh Node/PHP/Python) masuk `/connect/webhook/docs`, mengikuti
preseden `/connect/api/docs` (#49).

---

## 3. Pertahanan SSRF (keputusan D6) — bagian paling berbahaya di phase ini

Pelanggan mengetik URL; **server kita** yang memanggilnya. Tanpa pertahanan, endpoint ini adalah
proxy request yang dikendalikan pelanggan ke dalam jaringan kita sendiri.

### 3.1 Daftar tolak

Ditolak bila **IP hasil resolusi** masuk salah satu:

| Rentang | Kenapa |
|---|---|
| `127.0.0.0/8`, `::1` | Loopback — service internal di host yang sama |
| `10/8`, `172.16/12`, `192.168/16`, `fc00::/7` | Jaringan privat |
| `169.254.0.0/16`, `fe80::/10` | **Link-local — metadata cloud (`169.254.169.254`)**, kredensial instance |
| `100.64.0.0/10` | CGNAT |
| `0.0.0.0/8`, `::` | Unspecified |
| Multicast, broadcast, reserved | Tidak pernah tujuan HTTP yang sah |

### 3.2 Divalidasi dua kali, dan yang divalidasi adalah IP

```
Saat disimpan   → parse URL → resolve DNS → tiap IP dicek daftar tolak → tolak 400 bila ada yang kena
Saat dikirim    → resolve ulang → cek ulang → dial IP YANG SUDAH DIVALIDASI, bukan hostname
```

**Kenapa dua kali.** Validasi hanya saat disimpan bisa dilewati **DNS rebinding**: hostname yang saat
disimpan menunjuk IP publik, diarahkan ulang pemiliknya ke `127.0.0.1` setelah tersimpan. Setiap
pengiriman berikutnya lolos tanpa pernah dicek lagi.

**Kenapa dial IP, bukan hostname.** Kalau kita memvalidasi hostname lalu menyerahkan hostname itu ke
`http.Client`, ada dua resolusi DNS terpisah — satu untuk cek, satu untuk koneksi — dan penyerang bisa
membuat keduanya menjawab berbeda (TOCTOU). Resolusi dilakukan **sekali**, dan koneksi memakai IP hasil
resolusi itu (lewat `DialContext` kustom); header `Host` tetap hostname aslinya supaya TLS dan
virtual-host di sisi penerima tetap benar.

### 3.3 Redirect tidak pernah diikuti

`http.Client.CheckRedirect` selalu mengembalikan error. Redirect adalah jalur bypass paling langsung:
URL publik yang sah membalas `302` ke `http://169.254.169.254/`, dan seluruh validasi di atas terlewati
karena ia terjadi setelahnya.

Menolaknya sepenuhnya menghapus kelas bypass itu tanpa kerja tambahan (Aturan #27). Konsekuensi yang
diterima sadar: pelanggan yang URL-nya me-redirect harus mendaftarkan URL tujuan langsung. `3xx`
diperlakukan sebagai kegagalan **permanen**, bukan dicoba ulang — mengulanginya akan menghasilkan
redirect yang sama.

### 3.4 `WEBHOOK_ALLOW_PRIVATE_TARGETS`

Pengembangan lokal harus bisa mengirim ke `http://localhost:9099`. Satu env var melonggarkannya,
**ditolak saat boot** ketika `APP_ENV=production` (Aturan #36) — bentuk persis `CAPTCHA_PROVIDER=none`
(Phase 6), `MAIL_PROVIDER` (4.6), dan `PUSH_PROVIDER` (5). Pola yang sama untuk alasan yang sama:
seluruh phase bisa dikerjakan dan diuji tanpa layanan pihak ketiga, sementara produksi tidak bisa
berjalan setengah siap.

---

## 4. Worker (keputusan D2)

Goroutine di dalam binary `api`, dimulai setelah HTTP server siap, dihentikan pada graceful shutdown
(#1). Bukan proses, bukan deployable baru, bukan broker.

### 4.1 Klaim

```sql
SELECT id, organization_id, endpoint_id, event_type, payload, attempt
FROM webhook_deliveries
WHERE status = 'pending' AND next_attempt_at <= now()
ORDER BY next_attempt_at
LIMIT $1
FOR UPDATE SKIP LOCKED;
```

lalu, di transaksi yang sama:

```sql
UPDATE webhook_deliveries
SET status = 'delivering', delivering_since = now(), updated_at = now()
WHERE id = ANY($1);
```

**Transaksi ditutup sebelum HTTP apa pun terjadi** (Aturan #32). Baris sudah bertanda `delivering`, jadi
instance lain tidak akan mengambilnya.

`SKIP LOCKED` yang membuat banyak instance aman **tanpa leader election** — Postgres yang menjamin dua
transaksi tidak mengunci baris yang sama, bukan koordinasi buatan kita. Ini yang memenuhi kriteria #12,
dan ia **wajib diuji di bawah konkurensi nyata** (§12), bukan diasumsikan benar karena klausanya
tertulis.

### 4.2 Reaper — baris `delivering` yang menggantung

Crash tepat setelah transaksi klaim commit meninggalkan baris `delivering` selamanya: tidak ada yang
mengambilnya (bukan `pending`), tidak ada yang menyelesaikannya.

```sql
UPDATE webhook_deliveries
SET status = 'pending', next_attempt_at = now(), delivering_since = NULL
WHERE status = 'delivering' AND delivering_since < now() - interval '10 minutes';
```

Dijalankan di awal tiap putaran worker. Ambang 10 menit jauh di atas `WEBHOOK_DELIVERY_TIMEOUT` (10
detik), jadi ia tidak akan pernah merebut pengiriman yang benar-benar sedang berjalan.

**Konsekuensi yang diterima sadar:** crash antara "HTTP terkirim" dan "hasil tercatat" menghasilkan
pengiriman **ganda**. Ini `at-least-once`, bukan `exactly-once`, dan itu keputusan — `exactly-once`
lintas jaringan tidak bisa dicapai tanpa kerja sama penerima. Karena itu setiap payload membawa
`delivery_id` yang stabil lintas percobaan, supaya penerima bisa melakukan deduplikasi sendiri; ini
**didokumentasikan ke penerima**, bukan disembunyikan.

### 4.3 Hasil

| Respons | Perlakuan |
|---|---|
| `2xx` | `succeeded`, selesai |
| `429` | Dicoba ulang — server penerima yang minta |
| `4xx` lain | `failed` **permanen**, tidak dicoba ulang (D5: "permintaanmu salah" tidak berubah dengan diulang) |
| `3xx` | `failed` permanen (§3.3) |
| `5xx`, timeout, DNS gagal, koneksi ditolak | Dicoba ulang sampai `WEBHOOK_MAX_ATTEMPTS` |

Jeda: **1m → 5m → 30m → 2j → 6j** (D5). Setelah percobaan kelima → `failed`.

Body respons **tidak disimpan** — hanya `response_status` dan `error` singkat. Body penerima bisa
berisi apa saja, termasuk data sensitif mereka sendiri, dan kita tidak punya alasan menyimpannya
(Aturan #26 semangatnya sama).

---

## 5. Alur pemicu (keputusan D3)

```
lead.Usecase.Create
  └─ Store.InTx
       ├─ lead.Create
       ├─ activity.Record("lead_created")        ← sudah ada sejak #21
       └─ webhook.Enqueue("lead.created", ...)   ← BARU, di DALAM transaksi
     (commit)
  … worker mengambilnya nanti, HTTP di LUAR transaksi
```

`Enqueue` menulis **satu baris `pending` per endpoint aktif** yang melanggan event itu. Ia murni
operasi database, jadi ia **wajib** di dalam transaksi pemicunya: lead yang commit tanpa baris
pengiriman berarti event hilang selamanya, dan baris pengiriman yang commit tanpa lead berarti kita
mengirim event tentang sesuatu yang tidak ada.

Ini **tidak** melanggar Aturan #32 — yang dilarang aturan itu adalah **efek samping eksternal** (HTTP,
email) di dalam transaksi. Menulis baris antrian adalah operasi database biasa; justru pemisahan
inilah yang membuat Aturan #32 bisa dipenuhi tanpa kehilangan event.

### Bridge — `lead` tidak pernah mengimpor `internal/webhook`

Interface dideklarasikan **konsumen** (ADR-011), primitif saja, persis bentuk `ActivityRecorder` (#21)
dan `LeadCreator` (#87):

```go
// internal/lead/port.go
type WebhookEnqueuer interface {
    Enqueue(ctx context.Context, t tenant.Context, eventType string, payload []byte) error
}
```

Dijembatani di composition root (`cmd/api/`), bukan lewat impor langsung.

### Payload

```json
{
  "delivery_id": "01a0…",
  "event": "lead.created",
  "occurred_at": "2026-08-31T06:20:17.531933Z",
  "organization_id": "01a0…",
  "data": { "lead": { … bentuk leadJSON … } }
}
```

`data.lead` memakai **`leadJSON` yang sama** dengan API dashboard — satu bentuk lead di seluruh produk,
bukan bentuk kedua yang harus dijaga tetap sama selamanya. `delivery_id` stabil lintas percobaan (§4.2).

`lead.status_changed` menambahkan `"changes": {"status": {"from": "new", "to": "contacted"}}` — bentuk
yang sama dengan `activities.metadata` untuk `status_changed` (#21), bukan bentuk ketiga.

---

## 6. Endpoint

| Method | Path | Principal | Otorisasi |
|---|---|---|---|
| `POST` | `/v1/webhook-endpoints` | user | `webhook.create` — Owner/Admin |
| `GET` | `/v1/webhook-endpoints` | user | `webhook.list` — Owner/Admin |
| `GET` | `/v1/webhook-endpoints/:id` | user | `webhook.read` — Owner/Admin |
| `PATCH` | `/v1/webhook-endpoints/:id` | user | `webhook.update` — Owner/Admin |
| `DELETE` | `/v1/webhook-endpoints/:id` | user | `webhook.delete` — Owner/Admin (soft delete) |
| `GET` | `/v1/webhook-endpoints/:id/deliveries` | user | `webhook.read` — berpaginasi |
| `POST` | `/v1/webhook-deliveries/:id/retry` | user | `webhook.update` |

Path `webhook-endpoints` (bukan `webhooks`) supaya `/v1/webhook-deliveries/…` di sebelahnya tidak
ambigu, dan supaya tidak ada tabrakan wildcard seperti yang Phase 6 §8 hadapi.

**Kirim ulang manual** (kriteria #11) menyetel baris kembali ke `pending` dengan `next_attempt_at =
now()` dan `attempt = 0`. Hanya sah untuk status `failed` — `409` untuk status lain, supaya menekan
tombol dua kali tidak menggandakan pengiriman yang sedang berjalan.

---

## 7. Error code baru

| Status | Code | Kapan |
|---|---|---|
| `400` | `webhook_url_not_allowed` | URL menunjuk alamat privat/loopback/link-local, skema bukan http(s), atau DNS tidak bisa diresolusi |
| `409` | `delivery_not_retryable` | Kirim ulang dipanggil untuk pengiriman yang statusnya bukan `failed` |

`404 not_found`, `400 validation_failed`, `403 forbidden` dipakai ulang apa adanya.

`webhook_url_not_allowed` sengaja **satu kode untuk semua alasan penolakan URL** — membedakan "privat"
dari "tidak bisa diresolusi" memberi pelanggan alat memetakan jaringan internal kita lewat pesan error.
Pesannya generik; alasan detailnya masuk log server, bukan response.

---

## 8. Otorisasi

Owner/Admin penuh; Manager dan Employee **tidak punya akses sama sekali** — sama seperti `api_key`
(#46) dan `form` (#85), dan untuk alasan yang sama: endpoint webhook adalah kredensial yang mengalirkan
data organization ke luar.

Empat `Action` baru: `webhook.create`, `webhook.list`, `webhook.read`, `webhook.update`,
`webhook.delete`. Ditambahkan ke tabel per-role di `authz_test.go` **dan** ke `allActions` — celah yang
sama sudah terjadi dua kali (`ActionAPIKey*` di #46, `ActionForm*` di #85, keduanya di-backfill
belakangan). Jangan ketiga kalinya.

Principal `api_key` dan `public_form` **tidak** mendapat satu pun dari kelimanya — deny-by-absence di
`apiKeyScopeFor`/`publicFormAllows` menanganinya otomatis, dan tabel-atas-seluruh-`Action` yang sudah
ada membuktikannya tanpa baris baru.

---

## 9. Konfigurasi

```
WEBHOOK_WORKER_ENABLED=true          # false untuk instance yang hanya melayani HTTP
WEBHOOK_WORKER_INTERVAL=10s
WEBHOOK_WORKER_BATCH=20
WEBHOOK_DELIVERY_TIMEOUT=10s
WEBHOOK_MAX_ATTEMPTS=5
WEBHOOK_ALLOW_PRIVATE_TARGETS=false  # ditolak saat APP_ENV=production (Aturan #36)
WEBHOOK_DELIVERY_RETENTION_DAYS=30
```

`WEBHOOK_MAX_ATTEMPTS`, interval, dan batch **bukan hasil pengukuran** — masuk daftar bersama di
`api.md` bagian *Angka batasnya belum pernah diukur*, bukan keraguan terpisah yang menunggu sendiri
(keputusan #98).

---

## 10. Retensi (keputusan D8)

`webhook_deliveries` adalah tabel dengan pertumbuhan tercepat di produk: satu baris per lead × per
endpoint × per percobaan.

```sql
DELETE FROM webhook_deliveries
WHERE status IN ('succeeded', 'failed')
  AND created_at < now() - ($1 || ' days')::interval;
```

Dijalankan **malas, tanpa scheduler**, di-throttle 1×/jam dari dalam putaran worker — pola persis
retensi `idempotency_key` (#47). Baris `pending`/`delivering` tidak pernah dihapus berapa pun umurnya.

Aturan #18 (*"Activity & audit log tidak pernah dihapus"*) **tidak** berlaku di sini: riwayat pengiriman
adalah alat diagnosis, bukan catatan audit. Yang bersifat audit — endpoint dibuat/dihapus — tetap masuk
`audit_log` seperti biasa dan tidak pernah dihapus.

> **Poin terbuka yang diwarisi sadar:** pola retensi malas ini belum pernah diuji di volume produksi
> (`docs/issues/047-public-lead-api.md`). Kalau polanya bermasalah, ia bermasalah di dua tempat
> sekarang. Dicatat, bukan didiamkan.

---

## 11. Paket baru

```
crm_be/internal/webhook/
    entity.go               Endpoint, Delivery, generateSecret(), backoff()
    port.go                 Repository, DeliveryRepository, Repos, Store
    usecase.go              CRUD endpoint + Enqueue + Retry
    repository_postgres.go  ClaimDue (tanpa tenant.Context — §1.2), Reap, Purge
    handler_http.go         7 endpoint
    worker.go               loop, klaim, kirim, catat hasil
    signature.go            Sign(secret, ts, body)

crm_be/internal/shared/safedial/
    safedial.go             daftar tolak IP + DialContext yang mem-pin IP tervalidasi

crm_be/cmd/api/webhook_store.go   newWebhookStore(pool) webhook.Store
```

`safedial` dipisah dari `internal/webhook` karena ia **bukan** logika domain webhook — ia primitif
jaringan yang akan dipakai lagi oleh inbound webhook (Phase 7.5) dan setiap panggilan keluar
berikutnya. Satu-satunya paket `shared/` baru di phase ini.

`crm_dashboard/src/lib/webhooks.ts` dan `webhook-permissions.ts`, mengikuti `forms.ts`/
`form-permissions.ts`.

---

## 12. Rencana test

| Berkas | Menguji |
|---|---|
| `internal/shared/safedial/safedial_test.go` | **Tabel atas seluruh rentang §3.1** — tiap CIDR ditolak, alamat publik lolos. IPv4 **dan** IPv6. Ini test paling penting di phase ini |
| `internal/webhook/signature_test.go` | Signature cocok terhadap vektor yang dihitung tangan; body diubah satu byte → tidak cocok; `t` diubah → tidak cocok (membuktikan timestamp benar-benar ditandatangani) |
| `internal/webhook/entity_test.go` | Format `whsec_`, panjang, keacakan; `backoff(attempt)` menghasilkan jeda D5 persis |
| `internal/webhook/usecase_unit_test.go` | CRUD; `Enqueue` menulis satu baris per endpoint aktif yang melanggan; endpoint nonaktif/tidak melanggan **tidak** dapat baris; retry menolak status non-`failed` |
| `internal/webhook/repository_test.go` | Postgres asli: `ClaimDue` memakai index (`EXPLAIN`), `Reap` hanya menyentuh yang melewati ambang, `Purge` tidak pernah menghapus `pending` |
| **`internal/webhook/worker_concurrency_test.go`** | **Kriteria #12 — konkurensi nyata**: N goroutine menjalankan `ClaimDue` bersamaan terhadap Postgres sungguhan → tiap baris diklaim **tepat sekali**. Pola yang sama seperti alokasi `lead_number` (#19) dan idempotency (#20) — bukan test berurutan yang hijau meski logikanya salah |
| `internal/webhook/worker_test.go` | `2xx`→`succeeded`; `5xx`→dicoba ulang dengan `next_attempt_at` sesuai backoff; `4xx`→`failed` tanpa retry; `429`→dicoba ulang; `3xx`→`failed`; timeout→dicoba ulang. Terhadap `httptest.Server` sungguhan |
| `internal/webhook/handler_test.go` | URL privat ditolak `400` saat disimpan; kirim ulang manual; riwayat berpaginasi |
| `internal/lead/…` | `Create`/`UpdateStatus` memanggil `Enqueue` **di dalam** transaksi yang sama — dibuktikan lewat fake **dan** lewat rollback Postgres sungguhan (pola `repository_atomicity_test.go` #21): transaksi gagal → tidak ada baris pengiriman tersisa |
| `internal/shared/authz/authz_test.go` | Lima `Action` baru masuk `allActions` **dan** tabel per-role. Jangan ulangi celah #46/#85 |
| `cmd/api/tenant_isolation_test.go` | **Kasus baru**: `GET/PATCH/DELETE /v1/webhook-endpoints/:id` dan `GET …/deliveries` lintas org → `404`. **Tetap harus terbukti bisa gagal** |
| `crm_dashboard/src/lib/webhook-permissions.test.ts` | Gerbang role |

### Verifikasi manual wajib

Kriteria #1 hanya bisa dibuktikan dengan **server penerima sungguhan** yang mencatat apa yang diterima —
bukan dengan membaca log pengirim. Prosedurnya masuk `docs/testing/flow/` sebagai berkas baru saat issue
penutup, termasuk **verifikasi signature dari sisi penerima** dengan secret yang benar-benar disalin
dari dialog.

---

## 13. Risiko teknis

| Risiko | Penanganan |
|---|---|
| **SSRF** — pelanggan mengendalikan tujuan panggilan server kita | §3, tiga lapis: daftar tolak, validasi dua kali atas IP hasil resolusi, redirect ditolak total. Diuji sebagai tabel atas seluruh rentang, bukan beberapa contoh |
| **DNS rebinding / TOCTOU** | Resolusi **sekali**, koneksi ke IP hasil resolusi itu lewat `DialContext` kustom (§3.2) — bukan validasi hostname lalu menyerahkan hostname ke `http.Client` |
| **Pengiriman ganda antar-instance** | `FOR UPDATE SKIP LOCKED` (§4.1) + test konkurensi nyata (§12). Bukan leader election, bukan kunci aplikasi |
| **Baris `delivering` menggantung setelah crash** | Reaper berambang waktu (§4.2). Konsekuensinya `at-least-once` — diakui terbuka dan didokumentasikan ke penerima lewat `delivery_id` yang stabil |
| **Endpoint lambat menahan worker** | `WEBHOOK_DELIVERY_TIMEOUT` 10 detik, batch terbatas. Satu endpoint lambat memperlambat batch-nya, tidak menghentikan antrian |
| **Secret bocor lewat log** | Aturan #26. Secret tidak pernah masuk log, termasuk pada jalur gagal — diuji dengan mencari string secret di keluaran, bukan dipercaya dari baca kode (pola `TURNSTILE_SECRET_KEY` #87) |
| **`webhook_deliveries` tumbuh tak terkendali** | Retensi §10. Dicatat sebagai tabel dengan pertumbuhan tercepat di produk |
| **Worker menyulitkan graceful shutdown** | Worker berhenti pada sinyal yang sama dengan HTTP server (#1); pengiriman yang sedang berjalan diberi waktu selesai, sisanya ditinggal `delivering` dan diambil reaper setelah restart |

---

## 14. Yang harus disiapkan pemilik produk

**Tidak ada pihak ketiga** — berbeda dari Phase 6 (Turnstile) dan Phase 5 (Firebase). Yang dibutuhkan
untuk verifikasi manual hanyalah satu URL penerima, dan itu bisa berupa server lokal sederhana
(`WEBHOOK_ALLOW_PRIVATE_TARGETS=true` saat pengembangan) atau layanan penampung publik gratis.

**Tidak memblokir issue manapun.**

---

## 15. Yang berubah pada dokumentasi

| Berkas | Perubahan |
|---|---|
| `architecture/api.md` | Dua error code baru (§7); bab *Webhook Keluar* — bentuk payload, signature, cara verifikasi, kebijakan retry. Angka retry masuk daftar *Angka batasnya belum pernah diukur* yang sudah ada |
| `architecture/authentication.md` | Baris **keempat** tabel kredensial — dan yang pertama dengan arah kepercayaan terbalik (kita membuktikan diri ke pihak lain) |
| `architecture/authorization.md` | Matriks Phase 7 (`webhook.*`) |
| `architecture/multi-tenancy.md` | Pengecualian jenis **baru**: index yang sengaja tidak berawalan `organization_id` (§1.2) — bukan ditambahkan ke daftar pengecualian unik yang sudah ada empat |
| `docs/testing/flow/` | Berkas baru: mendaftarkan endpoint, menerima pengiriman sungguhan, memverifikasi signature dari sisi penerima |
| `STATUS.md` | Baris Selesai; Phase 7 di *Progress per Phase* |

`freeze.md` **tidak disentuh.** Penyimpangan D1 dari ketentuan #4 dicatat di `prd.md` sebagai keputusan
phase beserta kewajiban evaluasi ulangnya — bukan diselipkan seolah freeze memang mengizinkannya
(Aturan #30).

---

## 16. Kewajiban yang diteruskan ke phase berikutnya

- **Phase 7.5 (Inbound webhook)**: kredensial **kelima**; jangan disatukan dengan `whsec_`. Bentuk
  payload sudah diputuskan **tetap, sama seperti API Phase 4** (`prd.md` *Di luar cakupan*).
  `safedial` sudah ada dan dipakai ulang.
- **Phase 8 (Subscription)**: kartu Webhook di `/connect` berhenti berbunyi "belum tersedia" di phase
  ini. Keadaan "terkunci oleh paket" lahir di Phase 8, bukan sekarang (ADR-012 §4).
- **Saat konsumen async kedua lahir**: evaluasi ulang tabel `jobs` generik dengan dua implementasi
  nyata di tangan (`prd.md` D1).
- **Saat ada traffic produksi**: angka retry, interval, dan batch ikut peninjauan bersama angka rate
  limit di `api.md`.
