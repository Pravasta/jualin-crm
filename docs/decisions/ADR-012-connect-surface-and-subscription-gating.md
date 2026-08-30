# ADR-012 — "Connect" sebagai Permukaan Produk, dan Subscription sebagai Gerbangnya

> **Status:** ✅ Accepted — 30 Agustus 2026
> **Berlaku sejak:** Phase 6 (permukaan), Phase 8 (penegakan penuh)
> **Terkait:** [ADR-004](./ADR-004-api-key-format.md) · [ADR-005](./ADR-005-public-form-key.md) · Decisions §16, §24, §25 · Freeze §1.2, §3.4
> **Tidak mengubah freeze** — ADR ini menambah keputusan yang belum pernah diambil, bukan membatalkan yang sudah ada.

## Konteks

Sejak Phase 4 selesai, produk ini punya **empat pintu masuk lead** yang sudah dikunci di skema database (`ck_leads_source`):

```
source ∈ { manual, api, form, webhook }
```

Tiga di antaranya datang dari luar sistem, dan masing-masing sudah punya keputusan arsitekturnya sendiri:

| Pintu | Kredensial | Keputusan | Status |
|---|---|---|---|
| `api` | `jln_live_…` (rahasia) | ADR-004 | ✅ Phase 4 |
| `form` | `public_key` (publik) | ADR-005 | ⛔ Phase 6 |
| `webhook` | belum diputuskan | — | ⛔ Phase 7 |

**Yang tidak pernah diputuskan: di mana pelanggan menemukan dan memilih pintu-pintu ini.**

Hari ini `api` hidup sebagai sub-halaman `Pengaturan → API Keys`, ditemukan hanya oleh orang yang sudah tahu ia ada. Bila `form` dan `webhook` menyusul dengan pola yang sama, produk akan punya tiga fitur terpentingnya tersebar di tiga sudut menu konfigurasi.

Sekaligus: `subscriptions` sudah ada sejak `0002_identity.sql` dengan kolom `plan_code` dan `external_reference` — tetapi paket `internal/subscription` hanya punya **satu** operasi, `CreateFree`. Tidak ada satu pun query yang pernah membacanya kembali. Decisions §16 menetapkan alur `Free → Upgrade → Payment Service`, tanpa pernah menjawab *apa* yang sebenarnya dibatasi.

Dua pertanyaan itu saling terkait, dan itulah alasan keduanya diputuskan dalam satu ADR: **Connect adalah tempat batas paket menjadi terlihat oleh pelanggan.**

## Keputusan

### 1. `Connect` naik menjadi menu utama dashboard

```
Beranda · Lead · Customer · Tugas · Tim · Connect · Pengaturan
```

| Route | Isi | Phase |
|---|---|---|
| `/connect` | Daftar kanal + statusnya (aktif / terkunci / belum tersedia) | 6 |
| `/connect/api` | Pindahan dari `/settings/api-keys` beserta halaman dokumentasinya | 6 |
| `/connect/form` | Kelola form, salin script embed | 6 |
| `/connect/webhook` | Kelola endpoint, signature, log pengiriman | 7 |

**Kenapa bukan tetap di dalam Pengaturan.** *Pengaturan* adalah tempat konfigurasi akun dan organisasi — jarang disentuh setelah setup awal. *Connect* adalah permukaan tempat tesis produk ini dibuktikan: **capture layer bekerja dari luar** (freeze §"Tujuan Phase 4"). Menaruhnya dua level di dalam menu konfigurasi membuat fitur pembeda produk ini setara dengan mengganti nama organisasi.

**Rute lama tetap hidup.** `/settings/api-keys` di-*redirect* ke `/connect/api`, tidak dihapus — pelanggan Phase 4 yang sudah menyimpan tautannya tidak boleh menemukan 404 karena kita memindahkan menu.

### 2. Subscription menggerbangi kanal — CRM tahu **paket**, tidak pernah tahu **uang**

Batas ini tegas dan tidak boleh kabur:

| | CRM (repository ini) | Payment service (di luar repository) |
|---|---|---|
| Tahu `plan_code` organization | ✅ | — |
| Menentukan kanal mana boleh dipakai | ✅ | — |
| Menampilkan keadaan "terkunci" | ✅ | — |
| Mengarahkan pengguna keluar untuk upgrade | ✅ | — |
| Harga, checkout, kartu, invoice, refund | ❌ **tidak pernah** | ✅ |
| Sumber kebenaran status pembayaran | ❌ | ✅ |

`subscriptions.external_reference` — kolom yang sudah ada sejak Phase 1 — **adalah** titik sambung antara keduanya. CRM menyimpan rujukan itu dan membaca `plan_code`/`status` yang dihasilkannya; ia tidak pernah menghitung apa pun tentang uang.

> Aturan #27 dan batas produk di `CLAUDE.md` sudah menyatakan payment gateway di luar cakupan. ADR ini menegaskan bentuk konkretnya: **satu kolom rujukan, bukan integrasi.**

### 3. Penegakan di usecase, bukan di UI

Menyembunyikan kartu "Formulir" di `/connect` adalah **kenyamanan**, bukan penegakan. Gerbang yang sesungguhnya berada di usecase — konsisten dengan Aturan otorisasi yang sudah berlaku sejak Phase 1 (*otorisasi di usecase, bukan di UI*).

Artinya: `POST /v1/forms` yang dipanggil langsung lewat `curl` oleh organization yang paketnya tidak mengizinkan form **tetap ditolak**, meski tombolnya tidak pernah muncul di layar.

### 4. Yang **sengaja tidak** diputuskan di sini

| Tidak diputuskan | Kenapa |
|---|---|
| Harga | Decisions §25 — *"Pricing final"* eksplisit ditunda |
| Kanal mana di paket mana | Decisions §25 — *"Exact Free Tier limit"* eksplisit ditunda |
| Nama paket selain `free` | Butuh data harga yang belum ada |
| Tabel `plans` | Aturan #28 — belum ada implementasi kedua yang nyata; `plan_code` sebagai `text` masih cukup |

**ADR ini menetapkan mekanismenya, bukan angkanya.** Peta "kanal X untuk paket Y" ditulis saat Phase 8, setelah gate freeze (3–5 pengguna nyata) memberi data yang membuat angka itu bisa dipilih dengan jujur — bukan ditebak sekarang lalu dipertahankan karena sudah terlanjur ditulis.

## Alasan

| | |
|---|---|
| **Capture layer adalah alasan produk ini dibeli** | Kalimat inti MVP dimulai dari *"website mengirim lead"*. Pintu masuknya layak jadi menu, bukan sub-halaman konfigurasi. |
| **Tiga kanal, satu pertanyaan pengguna** | Pelanggan tidak bertanya "di mana API key saya" — ia bertanya *"bagaimana cara menyambungkan website saya"*. Satu tempat menjawab itu; tiga sudut menu tidak. |
| **Batas paket harus terlihat sebelum dibutuhkan** | Pelanggan yang tidak pernah melihat bahwa Webhook ada tidak akan pernah meng-upgrade untuk mendapatkannya. Keadaan "terkunci" adalah bagian dari produk, bukan kegagalan. |
| **Uang di luar, kapabilitas di dalam** | Memisahkan keduanya membuat CRM ini tidak pernah menanggung beban kepatuhan pembayaran, dan payment service bisa diganti tanpa menyentuh repository ini. |

### Kenapa bukan menu terpisah per kanal

`API · Formulir · Webhook` sebagai tiga menu sejajar akan memenuhi sidebar dengan istilah teknis yang **tidak** berarti apa-apa bagi pemilik toko — dan dua di antaranya akan tampak sebagai menu mati bagi paket yang belum membukanya. Satu menu `Connect` dengan tiga kartu di dalamnya menjawab pertanyaan pengguna sekaligus memberi tempat yang wajar bagi keadaan terkunci.

## Konsekuensi

**Positif:** pelanggan menemukan cara menyambungkan sistemnya tanpa diajari · batas paket punya tempat tinggal yang alami · payment service bisa diganti tanpa menyentuh CRM · `subscriptions` akhirnya punya pembaca setelah menganggur sejak Phase 1

**Negatif:**

| Konsekuensi | Mitigasi |
|---|---|
| Rute `/settings/api-keys` berpindah setelah dirilis di Phase 4 | Redirect permanen, bukan penghapusan. Halaman dokumentasi integrasi ikut pindah bersamanya. |
| Gerbang paket menyentuh usecase yang sudah ada (`apikey`, nanti `form`, `webhook`) | Satu titik pembacaan `plan_code`, bukan pemeriksaan tersebar. Bentuk persisnya diputuskan saat Phase 8, bukan diimprovisasi per-kanal. |
| Keadaan "terkunci" harus didesain, bukan sekadar tombol mati | Masuk daftar keadaan wajib pada design brief phase yang bersangkutan — pola yang sama seperti design brief Phase 5 §10. |
| Godaan menetapkan harga lebih awal karena UI-nya sudah ada | Bagian *"Yang sengaja tidak diputuskan"* di atas adalah jawabannya. Kartu terkunci boleh eksis tanpa harga tertulis di sebelahnya. |

## Batas ADR ini

**Landing page tidak termasuk.** `crm_landing_page/` masih berstatus *belum terjadwal* (`CLAUDE.md`, ADR-009) dan belum punya PRD maupun phase. Alur *landing → register → dashboard* dan tempat harga ditampilkan adalah keputusan tersendiri yang membutuhkan dokumennya sendiri — ADR ini sengaja tidak mendahuluinya.

**Penegakan kuota tidak termasuk.** Batas *jumlah* (berapa form, berapa lead per bulan) adalah `usage_counters` di Phase 8 — mekanisme yang berbeda dari gerbang kapabilitas di sini. TD Phase 4 §19 sudah memperingatkan agar keduanya juga tidak tertukar dengan rate limit: *"`usage_counters` menghitung; `ratelimit` melindungi. Keduanya tidak saling menggantikan."*
