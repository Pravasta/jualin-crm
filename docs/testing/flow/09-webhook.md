# 9 — Webhook Keluar

Prasyarat: [`01-registrasi-dan-autentikasi.md`](./01-registrasi-dan-autentikasi.md),
[`02-tim-dan-undangan.md`](./02-tim-dan-undangan.md), dan
[`03-lead-dan-pipeline.md`](./03-lead-dan-pipeline.md) selesai — butuh sesi Owner, satu Manager atau
Employee aktif (untuk gerbang role), dan minimal satu lead yang bisa diubah statusnya. **Tidak**
bergantung pada `07` (mobile) maupun `08` (formulir).

Menguji: **data berhenti jadi jalan satu arah** (Phase 7). Ini phase pertama di mana *Jualin yang
memanggil* — server kita menelepon URL yang pelanggan berikan. AC #1 dan #2 PRD hanya bisa
dibuktikan dengan **server penerima sungguhan** yang mencatat apa yang diterima dan **menjalankan
contoh verifikasi dari halaman docs** — bukan dengan membaca log pengirim.

## Yang perlu disiapkan

**1. `crm_be` harus mengizinkan target lokal.** `docker-compose.yml` sudah menyetel
`WEBHOOK_ALLOW_PRIVATE_TARGETS=true` untuk pengembangan — tanpanya, mendaftarkan
`http://host.docker.internal:9099` ditolak. Konfirmasi:

```bash
docker compose exec api printenv WEBHOOK_ALLOW_PRIVATE_TARGETS   # harus: true
```

Kalau `false`, set di `docker-compose.yml` service `api`, lalu `docker compose up -d api`.
`WEBHOOK_SECRET_ENC_KEY` juga harus terisi (default `.env` sudah) — tanpanya membuat endpoint gagal.

> **§9.8 (uji SSRF) adalah pengecualian.** Flag `true` di atas **mematikan daftar tolak IP** —
> itu memang gunanya (izinkan target privat saat dev). Jadi §9.8 dijalankan **terpisah**, di akhir,
> dengan `WEBHOOK_ALLOW_PRIVATE_TARGETS=false` dan `docker compose up -d api` — dan pada mode itu
> receiver `host.docker.internal` ikut ditolak, jadi §9.2–§9.7 tidak bisa jalan bersamaan.
> Kalau tidak ingin menukar env dua kali, §9.8 cukup diverifikasi lewat test otomatis
> (`internal/shared/safedial/denylist_test.go`, ~35 alamat) — tandai di checklist sebagai
> "diverifikasi lewat test, bukan manual".

**2. Server penerima.** Simpan ini sebagai `receiver.py` dan jalankan `python3 receiver.py` di
terminal terpisah. Ia mendengarkan di `:9099`, mencatat setiap kiriman, dan memverifikasi signature
**memakai skema terdokumentasi** — bukan kode Jualin:

```python
import hashlib
import hmac
import http.server
import json
import os
import time

SECRET = os.environ.get("WEBHOOK_SIGNING_SECRET", "")  # diisi di langkah 9.2
TOLERANCE = 300


class Handler(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        raw = self.rfile.read(int(self.headers.get("Content-Length", 0)))
        header = self.headers.get("X-Jualin-Signature", "")
        parts = dict(p.strip().split("=", 1) for p in header.split(",") if "=" in p)
        t, v1 = parts.get("t", ""), parts.get("v1", "")

        expected = hmac.new(SECRET.encode(), f"{t}.".encode() + raw, hashlib.sha256).hexdigest()
        valid = bool(t) and hmac.compare_digest(v1, expected)
        fresh = bool(t) and abs(time.time() - int(t)) <= TOLERANCE

        body = json.loads(raw)
        print(f"\n[{'OK' if valid and fresh else 'REJECT'}] "
              f"signature_valid={valid} within_tolerance={fresh}")
        print(f"  event={body['event']} delivery_id={body['delivery_id']}")
        print(f"  raw body: {raw.decode()}")

        # Answer 200 only for a valid signature — a receiver that accepts
        # anything is not verifying anything.
        self.send_response(200 if valid and fresh else 400)
        self.end_headers()

    def log_message(self, *args):
        pass


print("receiver listening on :9099")
http.server.HTTPServer(("", 9099), Handler).serve_forever()
```

## 9.1 Kartu Webhook aktif di Connect

1. Login sebagai `owner@test.local`. Menu **Connect**.

**Hasil yang diharapkan:** kartu **Webhook** aktif (bisa diklik), deskripsinya *"Kirim event ke
sistem Anda sendiri…"*. **Tidak** berbunyi *"Belum tersedia"* dan **tidak** berbunyi *"terkunci oleh
paket"* — yang terakhir baru lahir di Phase 8.

2. Klik kartu → halaman `/connect/webhook`, daftar endpoint kosong.

## 9.2 Daftarkan endpoint, simpan secret

1. Klik **+ Tambah endpoint**.
2. **URL tujuan**: `http://host.docker.internal:9099/webhook/jualin`
   *(di Linux tanpa Docker Desktop: pakai `http://172.17.0.1:9099/...` atau IP host di jaringan docker).*
3. Kedua event tercentang. **Tambah endpoint**.

**Hasil yang diharapkan:** dialog berpindah ke tahap **"Signing secret Anda"**, menampilkan secret
`whsec_...` **lengkap satu kali**, dengan tombol **Salin secret**. Tombol **Selesai** nonaktif
sampai checkbox "Saya sudah menyimpan…" dicentang.

4. **Salin secret.** Di terminal server penerima, hentikan (`Ctrl+C`), lalu jalankan ulang dengan
   secret terpasang:

```bash
WEBHOOK_SIGNING_SECRET="whsec_....." python3 receiver.py   # tempel secret lengkap
```

5. Centang checkbox, klik **Selesai**. Daftar endpoint kini menampilkan URL, `whsec_xxxx…` (prefix
   saja), badge dua event, status **Aktif**.

6. Coba buka detail endpoint → daftar riwayat pengiriman kosong (*"Belum ada pengiriman"*). Konfirmasi
   response API **tidak** membawa `secret`:

```bash
# ambil cookie sesi dari DevTools, atau lewat curl login — lihat 05-api-publik.md
curl -s http://localhost:8080/v1/webhook-endpoints -b "$COOKIES" | grep -o secret
# harus: hanya "secret_prefix", TIDAK ada "secret" polos
```

## 9.3 Buat lead → request sungguhan sampai ke penerima  ← **AC #1**

1. Buka `/leads` → **+ Tambah lead**. Nama `Webhook Test Satu`, submit.

**Hasil yang diharapkan (di terminal `receiver.py`):** dalam ±10 detik (interval worker), sebuah
baris muncul:

```
[OK] signature_valid=True within_tolerance=True
  event=lead.created delivery_id=0192...
  raw body: {"delivery_id":"0192...","event":"lead.created","occurred_at":"...Z",...}
```

**Ini kriteria #1**: request sungguhan sampai, diverifikasi dari sisi penerima, bukan dari log
pengirim.

2. Di dashboard, buka detail endpoint → **Riwayat pengiriman**: satu baris, **Berhasil**, HTTP 200,
   Percobaan `—` (kiriman pertama), waktu dengan jam.

3. Perhatikan `occurred_at` di body berakhiran `Z` (UTC), dan `data.lead` berisi bentuk lead yang
   sama seperti yang dashboard tampilkan (`lead_number`, `status`, dst.).

## 9.4 Contoh verifikasi dari halaman docs benar-benar memvalidasi  ← **AC #2 (bagian 1)**

1. Di `/connect/webhook`, klik **Dokumentasi verifikasi** (atau buka `/connect/webhook/docs`).

**Hasil yang diharapkan:** halaman menampilkan bentuk payload, tabel header, **tiga tab bahasa**
(Node.js / PHP / Python) berisi contoh verifikasi, tabel kebijakan retry, dan penjelasan
`delivery_id` + `at-least-once`.

2. Pilih tab **Python**. Contoh di halaman itu adalah **konstruksi yang sama** dengan `receiver.py`
   Anda (`hmac.new(secret, f"{t}.".encode() + raw, sha256)`). Anda sudah membuktikannya bekerja di
   9.3 — signature `receiver.py` `valid=True`.

3. *(Opsional, bukti lebih kuat)* salin **persis** contoh Node.js atau PHP dari halaman, jalankan di
   `:9098`, daftarkan endpoint kedua ke port itu, buat lead lagi → contoh yang disalin apa adanya
   memvalidasi `200`.

## 9.5 Payload diubah satu byte → contoh yang sama menolak  ← **AC #2 (bagian 2)**

Di terminal `receiver.py`, ubah sementara satu baris untuk merusak body **setelah** signature dihitung
Jualin:

```python
        raw = self.rfile.read(int(self.headers.get("Content-Length", 0)))
        raw = raw.replace(b'"name":"', b'"name":"X', 1)   # ← sisipkan satu byte
```

Restart `receiver.py`, buat lead `Webhook Test Dua`.

**Hasil yang diharapkan:**

```
[REJECT] signature_valid=False within_tolerance=True
```

Contoh verifikasi yang sama yang tadi menerima payload asli **menolak** payload yang diubah satu
byte. Kembalikan `receiver.py` seperti semula setelah ini.

## 9.6 Ubah status lead → `lead.status_changed` dengan `changes`

1. Buka detail `Webhook Test Satu`, ubah status **Baru → Dihubungi**.

**Hasil yang diharapkan (di `receiver.py`):**

```
[OK] ... event=lead.status_changed delivery_id=0192...
  raw body: {..."changes":{"status":{"from":"new","to":"contacted"}},"data":{"lead":{..."status":"contacted"...
```

`changes.status.{from,to}` benar, dan `data.lead.status` = `contacted`. Snapshot `lead.created` yang
lama **tetap** `new` di riwayat — payload dibekukan saat event terjadi.

## 9.7 Endpoint mati → gagal, retry, kirim ulang manual  ← **AC #6, #10, #11**

1. Hentikan `receiver.py` (`Ctrl+C`).
2. Ubah status lead lagi (**Dihubungi → Penawaran**).
3. Tunggu ±1 menit, buka **Riwayat pengiriman** endpoint.

**Hasil yang diharapkan:** baris terbaru **Menunggu** dengan Percobaan `ke-1`, detail error apa adanya
(`transport error: dial "host.docker.internal": ... network is unreachable` / `connection refused`),
`next_attempt_at` ≈ +1 menit (backoff pertama). Percobaan bertambah tiap retry dengan jeda menaik
(1m → 5m → 30m → 2j → 6j). Setelah 5 percobaan ulang → **Gagal** permanen.

> Menunggu 8 jam untuk melihat status **Gagal** permanen tidak praktis. Untuk menguji tombol
> **Kirim ulang** (yang hanya muncul pada baris `failed`), paksa satu baris ke `failed` lewat SQL —
> ini sah, mekanik retry+backoff sudah terbukti di langkah 3 dan di `internal/webhook/worker_test.go`:
>
> ```sql
> UPDATE webhook_deliveries SET status='failed', attempt=5, response_status=NULL,
>   error='transport error: connection refused (retries habis)'
> WHERE id='<id baris yang Menunggu>';
> ```

4. Jalankan ulang `receiver.py`. Refresh halaman — baris kini **Gagal** (merah) dengan tombol
   **Kirim ulang**. Klik.

**Hasil yang diharapkan:** baris berpindah ke **Menunggu** lalu **Berhasil HTTP 200** dalam satu
interval — **di baris yang sama**, bukan baris baru. `receiver.py` mencatat kirimannya, dan
`delivery_id`-nya **sama** dengan percobaan yang gagal (cek DB: `id` baris tidak berubah — dedup
handle stabil lintas percobaan).

5. Klik **Kirim ulang** lagi pada baris yang kini **Berhasil** — *(kalau tombolnya masih terlihat
   sebelum refresh)* atau lewat API:

```bash
curl -s -X POST http://localhost:8080/v1/webhook-deliveries/<id>/retry -b "$COOKIES"
# harus: 409 {"error":{"code":"delivery_not_retryable",...}}
```

## 9.8 SSRF — URL privat ditolak saat disimpan  ← **AC #4**

> **Butuh `WEBHOOK_ALLOW_PRIVATE_TARGETS=false`.** Dengan flag `true` (mode receiver di atas), daftar
> tolak IP dimatikan dan URL privat **diterima** — itu perilaku yang benar. Jalankan bagian ini
> terpisah:
>
> ```bash
> # set WEBHOOK_ALLOW_PRIVATE_TARGETS=false di docker-compose.yml service api
> docker compose up -d api
> ```
>
> Selesai §9.8, kembalikan ke `true` bila masih ingin menjalankan §9.2–§9.7.

Coba tambah endpoint dengan tiap URL berikut:

| URL | Hasil |
|---|---|
| `http://169.254.169.254/latest/meta-data/` | **"URL webhook tidak diizinkan."** di bawah field URL |
| `http://127.0.0.1:9099/x` | sama |
| `http://10.1.2.3/x` | sama |
| `ftp://example.com/x` | sama |
| `https://example.com/webhook` | **diterima** (`201`) — hapus lagi setelah ini |

Pesannya **tidak** membedakan alasan — itu keputusan keamanan (pesan spesifik = alat memetakan
jaringan internal kita).

**Alternatif tanpa menukar env:** AC #4 juga ditegakkan oleh `internal/shared/safedial/denylist_test.go`
(tabel ~35 alamat di tiap rentang, IPv4 + IPv6 + `::ffff:` mapped) dan verifikasi `curl` di issue #100.
Kalau §9.8 dilewati manual, tandai di checklist "diverifikasi lewat test otomatis".

## 9.9 Gerbang role — Manager/Employee tidak punya akses

1. Logout, login sebagai Manager atau Employee (dari `02`).
2. Buka `/connect/webhook`, `/connect/webhook/<id>`, dan `/connect/webhook/docs` langsung.

**Hasil yang diharapkan:** ketiganya menampilkan *"...tidak tersedia untuk role Anda."* Buka DevTools
→ Network: **nol** panggilan ke `/v1/webhook-endpoints` (hanya `/v1/me` + notifikasi/metrics dari
layout).

## 9.10 Gerbang paket (Phase 8, issue #112–#114) — kanal ditutup terbukti bisa gagal

`free` membuka ketiga kanal Connect hari ini (`internal/subscription/plan.go`), jadi membuktikan
gerbangnya benar-benar menolak butuh membalik satu entri **sementara**, dijalankan lalu dikembalikan
— prosedur yang sama dengan harness isolasi tenant (#11/#23). **Jangan pernah commit dalam keadaan
dibalik.**

1. Di `crm_be/internal/subscription/plan.go`, ubah `planChannels[PlanFree][ChannelWebhook]` dari
   `true` menjadi `false`.
2. `docker compose restart api` (atau restart proses `go run ./cmd/api` bila jalan lokal).
3. Lewat `curl` (bukan lewat UI yang menyembunyikan tombol — ADR-012 §3 mensyaratkan ini eksplisit):

```bash
curl -s -X POST http://localhost:8080/v1/webhook-endpoints -b "$COOKIES" \
  -H "Content-Type: application/json" -H "X-CSRF-Token: $CSRF" \
  -d '{"url":"https://example.com/hook","events":["lead.created"]}'
# harus: 403 {"error":{"code":"plan_upgrade_required",...}}

curl -s -X POST http://localhost:8080/v1/api-keys -b "$COOKIES" \
  -H "Content-Type: application/json" -H "X-CSRF-Token: $CSRF" \
  -d '{"name":"Masih Terbuka"}'
# harus: 201 — hanya kanal webhook yang dibalik, api_key/form tidak ikut tertutup

curl -s http://localhost:8080/v1/webhook-endpoints -b "$COOKIES"
# harus: 200 — GET tidak digerbangi (D4), resource yang sudah ada tetap terkelola
```

4. Di browser, buka `/connect`. Kartu **Webhook** harus redup, tidak bisa diklik, dengan badge
   "Terkunci oleh paket" — kartu **API** dan **Formulir** tetap normal.
5. Kembalikan `planChannels[PlanFree][ChannelWebhook]` ke `true`, restart lagi, ulangi langkah 3 baris
   pertama — harus kembali `201`.

Detail keputusan desain gerbang ini ada di `architecture/api.md` bagian *Gerbang Paket* dan
`architecture/authorization.md` bagian *Dua pertanyaan berbeda*.

## 9.11 Kuota lead & batas seat (Phase 8.5, issue #122–#126) — penolakan **dan** penerimaan

Bagian ini membuktikan hal yang tidak bisa dilihat dari UI mana pun: saat kuota habis, **dua jalur
ditolak dan satu jalur sengaja tetap diterima** (`architecture/api.md` bagian *Kuota*). Kalau ketiganya
ditolak, formulir di situs pelanggan berhenti bekerja — dan itu justru kegagalan yang keputusan D3
dibangun untuk mencegah.

Kuota Free sungguhan adalah **100 lead/bulan**; membuatnya dengan tangan tidak masuk akal. Jadi
batasnya diturunkan sementara — pola yang sama dengan §9.10, dan **jangan pernah commit dalam keadaan
diturunkan.**

### Persiapan

1. Di `crm_be/internal/subscription/plan.go`, ubah baris `PlanFree` di `planLimits` sementara:
   ```go
   PlanFree: {LeadsPerMonth: 2, Seats: 1},
   ```
2. Pastikan token admin terisi (`.env` / `docker-compose.yml` service `api`), minimal 32 byte:
   ```bash
   export ADMIN_TOKEN="token-uji-minimal-32-byte-supaya-lolos-validasi"
   ```
   Boot **gagal** kalau token diisi tapi lebih pendek dari 32 byte — itu memang perilakunya.
3. `docker compose up -d api` (restart dengan env baru), lalu siapkan API key baru (§5.8 sudah
   mencabut yang lama):
   ```bash
   # buat lewat /connect/api di browser, salin secret-nya
   export JUALIN_KEY="jln_live_....."
   export ORG_ID=$(curl -s http://localhost:8080/v1/me -b "$COOKIES" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["organization_id"])')
   ```

### Langkah 1 — pastikan organization ada di paket `free`, lewat permukaan `/internal/`

```bash
curl -s -w "\n%{http_code}\n" -X POST \
  "http://localhost:8080/internal/subscriptions/$ORG_ID/plan" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"plan_code":"free"}'
# harus: 200 {"data":{"organization_id":"...","plan_code":"free"}}
```

Sekalian buktikan penjaganya nyata — **tanpa token** dan **dengan token salah** harus sama-sama `401`,
dan tidak satu pun boleh menyentuh data:

```bash
curl -s -o /dev/null -w "%{http_code}\n" -X POST \
  "http://localhost:8080/internal/subscriptions/$ORG_ID/plan" \
  -H "Content-Type: application/json" -d '{"plan_code":"pro"}'          # harus: 401

curl -s -o /dev/null -w "%{http_code}\n" -X POST \
  "http://localhost:8080/internal/subscriptions/$ORG_ID/plan" \
  -H "Authorization: Bearer token-salah-tapi-panjangnya-cukup-32byte" \
  -H "Content-Type: application/json" -d '{"plan_code":"pro"}'          # harus: 401
```

### Langkah 2 — habiskan kuota, lalu buktikan **dua jalur ditolak**

Buat lead sampai melewati 2 (batas sementara). Lewat dashboard (principal `user`):

```bash
for i in 1 2 3; do
  curl -s -o /dev/null -w "user   #$i → %{http_code}\n" -X POST http://localhost:8080/v1/leads \
    -b "$COOKIES" -H "Content-Type: application/json" -H "X-CSRF-Token: $CSRF" \
    -d "{\"name\":\"Kuota User $i\"}"
done
# harus: 201, 201, lalu 403
```

Lewat API key (principal `api_key`) — kuota berlaku sama:

```bash
curl -s -X POST http://localhost:8080/v1/leads \
  -H "Authorization: Bearer $JUALIN_KEY" -H "Content-Type: application/json" \
  -d '{"name":"Kuota API Key"}'
# harus: 403 {"error":{"code":"plan_quota_exceeded",
#          "message":"Paket Anda dibatasi 2 lead per bulan. Sudah tercapai untuk bulan ini."}}
```

Perhatikan pesannya **menyebut angkanya** — beda sengaja dari `plan_upgrade_required` yang kabur:
penanya di sini adalah organization itu sendiri, bertanya soal jatahnya sendiri.

### Langkah 3 — jalur yang **sengaja tetap diterima**: formulir publik ← inti bagian ini

Pakai `public_key` formulir dari [`08-formulir-embed.md`](./08-formulir-embed.md) (`/connect/form`,
buka formulir uji, salin public key):

```bash
export FORM_KEY="....."   # public_key formulir dari §8

# ambil form_token dari halaman embed (time-trap, §8) lalu submit:
curl -s -w "\n%{http_code}\n" -X POST \
  "http://localhost:8080/v1/forms/$FORM_KEY/submit" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "name=Pengunjung Saat Kuota Habis" \
  --data-urlencode "email=pengunjung@example.com" \
  --data-urlencode "form_token=$FORM_TOKEN"
# harus: 201 — BUKAN 403
```

**Ini baris terpenting di seluruh bagian ini.** Pengunjung situs pelanggan tidak pernah melihat
keadaan penagihan pelanggan; lead-nya tetap masuk.

Lalu konfirmasi Owner benar-benar diberi tahu, **sekali saja**:

```bash
# submit sekali lagi lewat form (ambil form_token baru dulu)
curl -s http://localhost:8080/v1/notifications -b "$COOKIES" | grep -c plan_quota_exceeded
# harus: 1 — dua submit melewati kuota, tetap SATU notifikasi bulan ini
```

### Langkah 4 — naikkan paket lewat `/internal/`, penolakan hilang

```bash
curl -s -o /dev/null -w "%{http_code}\n" -X POST \
  "http://localhost:8080/internal/subscriptions/$ORG_ID/plan" \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" \
  -d '{"plan_code":"pro"}'                                              # harus: 200

curl -s -o /dev/null -w "%{http_code}\n" -X POST http://localhost:8080/v1/leads \
  -H "Authorization: Bearer $JUALIN_KEY" -H "Content-Type: application/json" \
  -d '{"name":"Setelah Naik Paket"}'                                    # harus: 201
```

Jalur kode yang **sama persis** memberi `403` lalu `201` — hanya barisan `plan_code` di database yang
berubah. Itu bukti gerbangnya benar-benar membaca paket, bukan kebetulan hijau.

Cek juga `plan_code` lama tercatat di audit:

```bash
docker compose exec db psql -U jualin -d jualin -c \
  "SELECT action, old_values, new_values, actor_membership_id FROM audit_logs WHERE action='subscription.plan_changed' ORDER BY created_at DESC LIMIT 2;"
# harus: old_values {"plan_code":"free"} → new_values {"plan_code":"pro"},
#        actor_membership_id NULL (tidak ada membership yang melakukannya)
```

### Langkah 5 — batas seat

Kembalikan ke `free` lewat `/internal/` (batas sementara: 1 seat, dan Owner sudah memakainya), lalu:

```bash
curl -s -X POST http://localhost:8080/v1/invitations -b "$COOKIES" \
  -H "Content-Type: application/json" -H "X-CSRF-Token: $CSRF" \
  -d '{"email":"orang-baru@test.local","role":"employee"}'
# harus: 403 {"error":{"code":"plan_seat_limit_reached",
#          "message":"Paket Anda dibatasi 1 anggota. Sudah tercapai batasnya."}}
```

Di browser, buka `/team` → **Undang anggota**, kirim undangan yang sama: pesan yang **sama persis**
tampil di dalam dialog, dengan tautan **"Lihat paket & pemakaian"** menuju `/subscription`.

### Langkah 6 — layar Langganan

Buka `/subscription` sebagai Owner:

- Paket aktif tampil sebagai nama (**Free**), bukan kode mentah
- Pemakaian lead & anggota menunjukkan angka yang sesuai — dan **boleh melebihi batas** (formulir
  publik tadi menambah lead setelah kuota habis); bar tidak boleh melewati 100%
- Tiga kolom paket tampil terurut **Free → Pro → Enterprise**, dengan harga `Rp0` /
  `Rp99.000/bulan` / `Negosiasi`
- Kolom Enterprise **tidak punya tombol beli** — teks "Hubungi kami untuk diskusi harga"

Lalu login sebagai **Manager** atau **Employee**, buka `/subscription`:

- "Langganan tidak tersedia untuk role Anda"
- Tab **Network** browser: **nol** panggilan ke `/v1/plans`

### Langkah 7 — kembalikan

1. Kembalikan `planLimits[PlanFree]` ke `{LeadsPerMonth: 100, Seats: 2}`.
2. Kembalikan paket organization ke `free` lewat `/internal/` bila masih di `pro`.
3. `docker compose up -d api`, lalu `git diff` — **harus kosong** di `plan.go`.

## 9.12 Bersihkan

1. Login lagi sebagai Owner. Hapus endpoint uji (blok merah **Hapus endpoint** di detail, atau
   nonaktifkan saja bila ingin menyimpan riwayat).
2. Hentikan `receiver.py`.

---

Selesai: seluruh jalur webhook keluar terbukti ujung ke ujung — endpoint didaftarkan dari dashboard,
lead sungguhan memicu request yang sampai ke penerima nyata, signature diverifikasi dari sisi
penerima dengan contoh dari halaman docs, dan payload yang diubah satu byte ditolak.
Kembali ke [`06-checklist-akhir.md`](./06-checklist-akhir.md) untuk rekap.
