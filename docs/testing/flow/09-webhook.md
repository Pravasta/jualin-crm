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

**Hasil yang diharapkan:** baris terbaru **Gagal** (atau **Menunggu** dengan Percobaan `ke-1`/`ke-2`),
dengan detail error apa adanya (`connection refused` / `transport error: ...`). Percobaan bertambah
setiap retry dengan jeda menaik (1m, 5m, …). Setelah 5 percobaan ulang → **Gagal** permanen.

4. Jalankan ulang `receiver.py`. Pada baris yang **Gagal**, klik **Kirim ulang**.

**Hasil yang diharapkan:** baris berpindah ke **Menunggu** lalu **Berhasil** dalam satu interval —
**di tabel yang sama**, bukan baris baru. `receiver.py` mencatat kirimannya, `delivery_id` **sama**
dengan percobaan yang gagal (dedup handle stabil).

5. Klik **Kirim ulang** lagi pada baris yang kini **Berhasil** — *(kalau tombolnya masih terlihat
   sebelum refresh)* atau lewat API:

```bash
curl -s -X POST http://localhost:8080/v1/webhook-deliveries/<id>/retry -b "$COOKIES"
# harus: 409 {"error":{"code":"delivery_not_retryable",...}}
```

## 9.8 SSRF — URL privat ditolak saat disimpan  ← **AC #4**

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

## 9.9 Gerbang role — Manager/Employee tidak punya akses

1. Logout, login sebagai Manager atau Employee (dari `02`).
2. Buka `/connect/webhook`, `/connect/webhook/<id>`, dan `/connect/webhook/docs` langsung.

**Hasil yang diharapkan:** ketiganya menampilkan *"...tidak tersedia untuk role Anda."* Buka DevTools
→ Network: **nol** panggilan ke `/v1/webhook-endpoints` (hanya `/v1/me` + notifikasi/metrics dari
layout).

## 9.10 Bersihkan

1. Login lagi sebagai Owner. Hapus endpoint uji (blok merah **Hapus endpoint** di detail, atau
   nonaktifkan saja bila ingin menyimpan riwayat).
2. Hentikan `receiver.py`.

---

Selesai: seluruh jalur webhook keluar terbukti ujung ke ujung — endpoint didaftarkan dari dashboard,
lead sungguhan memicu request yang sampai ke penerima nyata, signature diverifikasi dari sisi
penerima dengan contoh dari halaman docs, dan payload yang diubah satu byte ditolak.
Kembali ke [`06-checklist-akhir.md`](./06-checklist-akhir.md) untuk rekap.
