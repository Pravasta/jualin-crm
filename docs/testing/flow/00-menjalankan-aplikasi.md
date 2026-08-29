# 0 — Menjalankan aplikasi

Tujuan: **backend** (`crm_be`) dan **dashboard** (`crm_dashboard`) menyala dari clone bersih, sampai
bisa dibuka di browser. Semua perintah `make`/`docker compose` dijalankan dari **akar repository**.

## 1. Siapkan environment backend

```bash
cp .env.example .env
```

Nilai default di `.env.example` sudah cukup untuk lokal — tidak perlu diubah kecuali Anda tahu kenapa.

## 2. Nyalakan PostgreSQL + API

```bash
make dev
```

Ini menjalankan `docker compose up --build`. Tunggu sampai log berhenti bergulir cepat dan terlihat
baris mirip:

```
api-1  | time=2026-08-29T15:23:53.922Z level=INFO msg="server starting" port=8080 env=development
```

**Biarkan terminal ini terbuka** — inilah tempat mencari "email" (LogMailer) di langkah-langkah
berikutnya. Buka **terminal baru** untuk langkah selanjutnya.

## 3. Jalankan migration

Container `api` **tidak** menjalankan migration otomatis (keputusan sadar — `docs/STATUS.md`'s Utang
Teknis). Dari terminal baru, di akar repo:

```bash
set -a && source .env && set +a
make migrate-up
```

Hasil yang diharapkan — baris terakhir mirip:

```
goose: successfully migrated database to version: 5
```

Bila muncul `connection refused`: postgres di langkah 2 belum siap sepenuhnya — tunggu beberapa detik
dan ulangi.

## 4. Pastikan backend benar-benar hidup

```bash
curl -s http://localhost:8080/health | jq
curl -s http://localhost:8080/health/ready | jq
```

Hasil yang diharapkan — keduanya `200`:

```json
{"data":{"status":"ok","version":"dev"}}
{"data":{"database":"reachable","status":"ok"}}
```

(Tidak punya `jq`? Hilangkan `| jq`, hasilnya tetap terbaca sebagai satu baris JSON.)

## 5. Siapkan environment dashboard

```bash
cp crm_dashboard/.env.example crm_dashboard/.env.local
```

Default-nya sudah menunjuk ke `http://localhost:8080` — cocok dengan backend di langkah 2.

## 6. Nyalakan dashboard

Di terminal baru (ketiga):

```bash
cd crm_dashboard
npm install
npm run dev
```

Tunggu sampai muncul:

```
▲ Next.js 16.3.3
- Local:   http://localhost:3000
✓ Ready in ...
```

## 7. Buka di browser

Buka **http://localhost:3000**.

**Hasil yang diharapkan:** otomatis diarahkan ke `/login` — belum ada sesi login (`SessionGate`
memanggil `GET /v1/me`, dapat `401`, redirect). Ini **bukan** bug — memang begitu perilakunya untuk
siapa pun yang belum login.

Kalau langkah ini lolos, lanjut ke [`01-registrasi-dan-autentikasi.md`](./01-registrasi-dan-autentikasi.md).

---

## 8. Cara mengambil tautan "email" dari log

Dipakai berulang di berkas-berkas berikutnya (verifikasi email, reset password, undangan) — belum ada
penyedia email sungguhan, jadi setiap tautan hanya tercatat di log `api`
(`internal/shared/mailer.LogMailer`).

Baris log-nya berbentuk seperti ini:

```
api-1  | time=... level=INFO msg="email (not sent — LogMailer)" to=owner@test.local subject="Verifikasi email Jualin CRM Anda" body="Klik tautan berikut untuk memverifikasi email Anda: http://localhost:3000/verify-email?token=WrnunLAJuz8v...\n\nTautan berlaku 24 jam."
```

⚠️ **Jangan salin manual dengan mata.** Di dalam `body=`, URL diikuti langsung oleh `\n\n` (dua
karakter literal, bukan baris baru) — ikut tersalin dan token jadi tidak valid. Pakai perintah ini,
yang sudah memotong tepat di ujung token:

```bash
docker compose logs api | grep -o 'http://localhost:3000/[^\\"]*' | tail -1
```

Hasilnya satu baris bersih siap tempel ke browser, misalnya:

```
http://localhost:3000/verify-email?token=WrnunLAJuz8vTf6wQtVpAx5MKRKostypB2g4SHRAXtE
```

`tail -1` mengambil **tautan terbaru**. Kalau sedang menguji beberapa email sekaligus (mis. beberapa
undangan berturut-turut), saring dulu per jenis:

```bash
docker compose logs api | grep -o 'http://localhost:3000/verify-email[^\\"]*'      | tail -1
docker compose logs api | grep -o 'http://localhost:3000/reset-password[^\\"]*'    | tail -1
docker compose logs api | grep -o 'http://localhost:3000/invitations/accept[^\\"]*' | tail -1
```

---

## Kalau ada yang tidak beres

| Gejala | Kemungkinan penyebab |
|---|---|
| `docker compose up` gagal, port sudah dipakai | Ada proses lain memakai `5432` atau `8080`. Matikan proses itu, atau ubah pemetaan port di `docker-compose.yml` (jangan commit perubahan ini). |
| `make migrate-up` → `connection refused` | Postgres container belum sehat. `docker compose ps` — pastikan `postgres` berstatus `healthy`, bukan cuma `running`. |
| `make migrate-up` → `DATABASE_URL required` | Lupa `source .env` di terminal yang sama sebelum `make migrate-up` — env var ini **tidak** otomatis terbaca dari file `.env`. |
| Dashboard menampilkan error jaringan terus-menerus | `crm_dashboard/.env.local` belum ada, atau `next dev` dinyalakan **sebelum** file itu dibuat — restart `npm run dev` setelah membuat/mengubah `.env.local` (Next.js hanya membaca env saat start). |
| `curl http://localhost:8080/health` → connection refused | `api` container belum selesai boot, atau gagal boot — cek log di terminal langkah 2 untuk pesan `failed to connect to database` atau error validasi config. |

## Mematikan semuanya

```bash
docker compose down          # hentikan postgres + api, data tetap ada
docker compose down -v       # sama, TAPI data postgres ikut terhapus — pipeline test harus diulang dari 0
```

Hentikan `npm run dev` dengan `Ctrl+C` di terminalnya.
