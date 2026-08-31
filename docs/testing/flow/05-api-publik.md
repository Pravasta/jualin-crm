# 5 — API Publik

Prasyarat: [`04-customer.md`](./04-customer.md) selesai. Login sebagai `owner@test.local`.

Menguji: bagian dari kalimat inti MVP yang paling sering dilewatkan — **"website mengirim lead"**
lewat API key, dari luar dashboard sama sekali. Ini juga satu-satunya berkas yang memakai terminal
(`curl`) sebagai "klien", bukan cuma browser.

## 5.1 Buat API key

1. Buka menu **Connect** → kartu **API** (atau langsung `/connect/api`).
2. Klik **+ Buat kunci baru**.
3. **Nama**: `Website Testing`, submit.

**Hasil yang diharapkan:** dialog berganti ke tahap **"Kunci Anda"**, menampilkan secret **lengkap**
satu kali — dengan tombol **Salin kunci** dan blok contoh `curl` siap pakai + tombol **Salin
perintah**.

4. Klik **Salin perintah** — ini menyalin curl yang **sungguhan bisa dijalankan** (bukan placeholder).
5. **Sebelum menutup dialog**: buka terminal, tempel, jalankan.

**Hasil yang diharapkan:** status `201`, dengan body memuat `"source":"api"` dan `source_api_key_id`
terisi:

```json
{"data":{"id":"01a04e20-...","lead_number":1,"name":"Budi Santoso","source":"api",
         "source_api_key_id":"01a04e20-...","status":"new","version":1, ...}}
```

Perhatikan `created_by_membership_id` bernilai `null` — lead dari API memang tidak dibuat oleh
anggota mana pun.

Kalau dialog belum ditutup dan Anda ragu sudah menyalin dengan benar, ulangi salin — begitu dialog
ditutup, secret **tidak bisa dilihat lagi selamanya** (sengaja, Aturan #21).

6. **Masih sebelum dialog ditutup** — klik **Salin kunci**, lalu simpan ke variabel shell untuk
   dipakai di seluruh sisa berkas ini:

```bash
export JUALIN_KEY="jln_live_....."   # tempel secret lengkap dari dialog
```

7. **Sekarang** tutup dialog (klik konfirmasi bahwa kunci sudah disimpan). Cek daftar API key —
   kunci baru muncul dengan **prefix** saja (mis. `jln_live_kSa6`), dan field `secret` **sama sekali
   tidak ada** di response daftar (bukan dikosongkan — memang tidak ada field-nya).

## 5.2 Konfirmasi lead muncul di dashboard

Buka `/leads`, cari lead dengan nama `Budi Santoso` (dari contoh curl bawaan).

**Hasil yang diharapkan:** ada, dengan **sumber = API** (bukan Manual), dan kalau dibuka detailnya,
menampilkan kunci mana yang mengirimnya (nama kunci `Website Testing` atau ID-nya).

## 5.3 Idempotency — kirim ulang tidak membuat duplikat

Memakai `$JUALIN_KEY` yang sudah di-`export` di §5.1 langkah 6.

```bash
curl -s -X POST http://localhost:8080/v1/leads \
  -H "Authorization: Bearer $JUALIN_KEY" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: test-idem-001" \
  -d '{"name": "Lead Idempotent", "email": "idem@test.local"}'
```

Jalankan **persis perintah yang sama** (`Idempotency-Key` sama) **dua kali berturut-turut**.

**Hasil yang diharapkan:** respons kedua punya **`id` yang sama persis** dengan respons pertama —
bukan lead baru, bukan error. Cek di `/leads`: hanya **satu** `Lead Idempotent`, bukan dua.

## 5.4 Scope — kunci tidak bisa melakukan hal lain

```bash
curl -s -o /dev/null -w "%{http_code}\n" -X POST http://localhost:8080/v1/leads \
  -H "Authorization: Bearer $JUALIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "Lead Nakal", "assigned_to_membership_id": "00000000-0000-0000-0000-000000000000"}'
```

**Hasil yang diharapkan:** `403` dengan
`{"error":{"code":"insufficient_scope","message":"Kredensial API tidak memiliki scope untuk field ini."}}`
— API key hanya boleh `leads:write` polos, tidak boleh menentukan penugasan.

Coba juga panggil endpoint **lain** dengan kunci yang sama, mis.:

```bash
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/v1/leads \
  -H "Authorization: Bearer $JUALIN_KEY"
```

**Hasil yang diharapkan:** `401 authentication_required` — bukan `403`. API key ditolak di
**autentikasi** untuk route selain `POST /v1/leads`, sebelum otorisasi sempat dikonsultasi sama
sekali (lihat `architecture/authentication.md` bagian *API key*).

## 5.5 Validasi field

```bash
curl -s -w "\n%{http_code}\n" -X POST http://localhost:8080/v1/leads \
  -H "Authorization: Bearer $JUALIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"email": "tanpa-nama@test.local"}'
```

**Hasil yang diharapkan:** `400` dengan `details` yang menyebut field mana yang salah — bukan pesan
generik:

```json
{"error":{"code":"validation_failed","message":"Permintaan tidak valid.",
          "details":[{"field":"name","code":"required"}]}}
```

## 5.6 Rate limit header

```bash
curl -s -D - -o /dev/null -X POST http://localhost:8080/v1/leads \
  -H "Authorization: Bearer $JUALIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "Cek Header"}'
```

**Hasil yang diharapkan:** ketiga header ada, mis.

```
X-Ratelimit-Limit: 60
X-Ratelimit-Remaining: 54
X-Ratelimit-Reset: 1788017229
```

Nilai `Remaining` harus **berkurang** setiap kali dijalankan ulang (`Limit` mengikuti
`PUBLIC_API_RATE_LIMIT`, default 60/menit per kunci).

*(Opsional, butuh waktu:* jalankan perintah ini berulang cepat sampai `Remaining` mencapai `0` —
permintaan berikutnya harus `429` dengan header `Retry-After`.)

## 5.7 Halaman dokumentasi integrasi

1. Di `/connect/api`, klik **Dokumentasi integrasi**.

**Hasil yang diharapkan:** halaman referensi menampilkan `key_prefix` kunci `Website Testing` (bukan
secret lengkap — sudah tidak bisa ditampilkan lagi sejak §5.1), contoh curl dengan placeholder
`<secret_anda>`, tabel field, katalog error, penjelasan `Idempotency-Key`, rate limit, dan peringatan
kenapa kunci **tidak boleh** dipakai dari kode sisi browser.

## 5.8 Cabut kunci

1. Kembali ke `/connect/api`, klik **Cabut** pada kunci `Website Testing`, konfirmasi.

**Hasil yang diharapkan:** status kunci berubah jadi tercabut (`revoked_at` terisi), tapi baris
kuncinya **tetap terlihat** di daftar (tidak dihapus — supaya Owner tahu ia pernah ada). Revoke juga
idempoten: mencabut kunci yang sama dua kali tidak menghasilkan error.

2. Segera coba kirim lead lagi dengan kunci yang sama:

```bash
curl -s -w "\n%{http_code}\n" -X POST http://localhost:8080/v1/leads \
  -H "Authorization: Bearer $JUALIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "Lead Setelah Revoke"}'
```

**Hasil yang diharapkan:** `401` **seketika** — tanpa jeda cache, tanpa masa tenggang:

```json
{"error":{"code":"invalid_api_key",
          "message":"Kredensial API tidak valid, sudah kedaluwarsa, atau sudah dicabut."}}
```

Perhatikan pesannya sengaja **tidak** membedakan "kunci tidak pernah ada", "kedaluwarsa", dan "sudah
dicabut" — orang luar tidak boleh bisa menyimpulkan kunci mana yang pernah valid.

---

Selesai di sini: seluruh jalur "website mengirim lead" dari kalimat inti MVP sudah diuji ujung ke
ujung, dari luar dashboard sama sekali. Lanjut ke
[`06-checklist-akhir.md`](./06-checklist-akhir.md) untuk rekap.
