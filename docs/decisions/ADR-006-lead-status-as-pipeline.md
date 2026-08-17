# ADR-006 — Lead Status adalah Pipeline di MVP

> **Status:** ✅ Accepted — 17 Agustus 2026

## Konteks

Dokumen produk mencantumkan **Sales Pipeline** sebagai bagian dari scope, sekaligus menempatkan *advanced pipeline* di Phase 9. Dua pernyataan itu terlihat bertentangan.

Sebenarnya ada dua konsep berbeda dengan nama mirip:

| Istilah | Arti |
|---|---|
| **Pipeline (konsep)** | Urutan tahapan yang dilalui sebuah lead |
| **Pipeline (entity)** | Stage yang bisa dikonfigurasi per organization — tabel, UI, drag & drop |

## Keputusan

**Di MVP, `leads.status` adalah pipeline-nya.** Tidak ada tabel `pipelines` atau `pipeline_stages`.

```
                    ┌──────────────────────────────────► spam
                    │
new ──► contacted ──► qualified ──► proposal ──► won
 │          │             │             │
 └──────────┴─────────────┴─────────────┴────────────► lost
 │
 └──────────────────────────────────────────────────► unqualified
```

| Status | Arti |
|---|---|
| `new` | Baru masuk, belum disentuh |
| `contacted` | Sudah dihubungi |
| `qualified` | Terverifikasi sebagai calon pembeli |
| `proposal` | Penawaran sudah diberikan |
| `won` | Berhasil — memicu konversi ke Customer |
| `lost` | Gagal — **wajib** `lost_reason` |
| `unqualified` | Bukan calon pembeli (salah sasaran, iseng) |
| `spam` | Sampah dari form/API publik |

## Alasan

**Penyederhanaan ini justru tepat untuk positioning produk.** Jualin CRM menjual kesederhanaan; pipeline yang bisa dikonfigurasi adalah fitur yang membuat CRM terasa berat, dan belum ada pelanggan yang memintanya.

Satu kolom enum memberi 90% nilai pipeline dengan 5% biayanya.

## Empat aturan turunan

### 1. `spam` dan `unqualified` wajib ada sejak awal

Keduanya **dikecualikan dari seluruh metrik konversi**.

Ini konsekuensi langsung dari punya form publik dan free tier: tanpa pemisahan ini, sampah akan masuk ke angka conversion rate — dan angka itulah alasan owner membayar. Metrik yang tercemar merusak kepercayaan pada seluruh produk.

### 2. Tidak ada status `assigned`

Assignment **ortogonal** terhadap status. Lead bisa `new` dan sudah ter-assign, atau `qualified` dan belum. Mencampurnya menghasilkan state machine yang tidak konsisten.

### 3. Transisi divalidasi di service layer

Bukan sekadar kolom bebas. Mundur satu langkah diizinkan (`qualified → contacted`); melompat dari `new` ke `won` tidak.

Tanpa validasi terpusat, mobile app dan dashboard akan menghasilkan riwayat dengan aturan yang berbeda.

### 4. `won` tidak otomatis membuat Customer

`won` berarti kesepakatan tercapai. Konversi ke Customer adalah **aksi eksplisit** — ia membuat relasi pelanggan, dan itu keputusan pengguna, bukan efek samping.

## Rencana perpindahan — bukan inkonsistensi

Ketika Deal dibangun (pasca-Phase 5), `proposal` dan `won` **berpindah ke Deal**, dan daftar status Lead menyusut:

```
new → contacted → qualified → converted | lost | unqualified | spam
```

Alasannya: `proposal` dan `won` sebenarnya menggambarkan tahapan **transaksi**, bukan tahapan **prospek**. Selama Deal belum ada, keduanya menumpang di Lead sebagai jembatan.

> **Ini rencana yang disengaja.** ADR ini ditulis justru agar session mendatang tidak membacanya sebagai inkonsistensi lalu "memperbaikinya" ke arah yang salah.

Migrasinya nanti: `won` → buat Deal + set lead `converted`. Bisa di-backfill karena `converted_customer_id` dan `converted_at` sudah ada sejak awal.

## Konsekuensi

**Positif:** nol tabel tambahan · UI sederhana · reporting langsung bisa dibuat · perpindahan ke Deal sudah dipetakan

**Negatif:** organization tidak bisa menyesuaikan tahapan · perpindahan nanti menyentuh data yang sudah ada

**Kapan dievaluasi ulang:** saat ada pelanggan yang benar-benar meminta tahapan berbeda — bukan saat kita membayangkan mereka akan meminta.
