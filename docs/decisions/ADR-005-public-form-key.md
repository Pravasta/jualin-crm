# ADR-005 — Public Form Key Terpisah dari API Key

> **Status:** ✅ Accepted — 17 Agustus 2026
> **Berlaku sejak:** Phase 6
> **Terkait:** [ADR-004](./ADR-004-api-key-format.md) · Aturan #23, #24

## Konteks

Dokumen brainstorming awal mengusulkan embedded form memakai:

```http
POST /v1/forms/{form_id}/submit
Authorization: Bearer {api_key}
```

Untuk **server-to-server** bentuk ini benar. Untuk **embedded form** ia adalah kerentanan serius.

Embedded form berjalan di browser pengunjung website pelanggan. Apapun kredensial yang dipakainya **harus hadir di sisi klien** — bisa dibaca siapa saja lewat DevTools.

Bila kredensial itu adalah API key, siapapun yang membuka halaman pelanggan bisa mengambilnya dan memakainya untuk mengakses **seluruh API organization tersebut**: membaca semua lead, mengubah data, apapun yang diizinkan scope-nya.

> Ini bukan risiko probabilistik. Ini kebocoran otomatis pada **setiap pemasangan form**.

## Keputusan

Dua kredensial, dua kemampuan, dua endpoint.

| | Server-to-server | Embedded form |
|---|---|---|
| Endpoint | `POST /v1/leads` | `POST /v1/forms/{public_key}/submit` |
| Kredensial | `jln_live_...` — **rahasia** | `public_key` — **publik, memang terekspos** |
| Disimpan | Hash di database | Plaintext (bukan rahasia) |
| Kemampuan | Full API sesuai scope | **Hanya submit ke form itu** |
| Bisa membaca data? | Sesuai scope | **Tidak pernah** |
| Proteksi | Rate limit per key | Domain allowlist + rate limit IP + honeypot + CAPTCHA |

Pola yang sama dipakai Stripe: `pk_` publishable untuk browser, `sk_` secret untuk server.

### Kemampuan `public_key` — daftar tertutup

Hanya boleh: **membuat lead untuk form tersebut.**

Tidak boleh, tanpa pengecualian: membaca lead · mengubah lead · membaca customer · mengelola employee · mengelola API key · apapun yang lain.

## Proteksi endpoint form publik

Form publik tanpa autentikasi adalah magnet spam. Di model harga terjangkau, **setiap lead spam adalah biaya yang ditanggung Jualin** — storage, compute, dan reputasi domain pengirim.

Anti-spam di sini adalah **fitur ekonomi**, bukan sekadar fitur keamanan.

| Proteksi | Catatan |
|---|---|
| **Domain allowlist per form** | Verifikasi header `Origin`. *Jujur: `Origin` bisa dipalsukan oleh non-browser — ini menghalangi penyalahgunaan biasa, bukan penyerang tertarget. Karena itu ia berpasangan, tidak berdiri sendiri.* |
| **CAPTCHA** | Cloudflare Turnstile (gratis, tanpa puzzle). **Wajib di free tier.** |
| **Honeypot** | Field tersembunyi; bila terisi → buang **diam-diam**. Jangan kembalikan error — bot akan belajar. |
| **Rate limit** | Per IP dan per form |
| **Time-trap** | Tolak submit < 2 detik setelah form render |
| **Batas payload** | 32KB |

## Mekanisme embed: iframe

| | iframe ✅ | inline script |
|---|---|---|
| Isolasi keamanan | Kuat — terisolasi dari halaman host | Berjalan di konteks host |
| Risiko dari host tercemar | Rendah | Script host bisa membaca input |
| Fleksibilitas styling | Terbatas | Menyatu dengan situs |
| Kompleksitas | Rendah | Perlu isolasi CSS / shadow DOM |

**iframe untuk MVP.** Styling terbatas adalah harga yang wajar untuk isolasi. Inline script bisa ditawarkan nanti sebagai opsi lanjutan.

Disajikan dari **domain terpisah** (`cdn.…`) dengan CSP ketat dan `frame-ancestors` sesuai allowlist form.

## Field form — tetap, bukan builder

MVP memakai field tetap: `name`, `email`, `phone`, `company`, `message`, `product` — dengan toggle wajib/opsional dan label yang bisa diubah.

Form builder dinamis memerlukan schema field + mesin validasi + renderer + strategi penyimpanan. Itu fitur besar tersendiri, dan tidak diperlukan untuk membuktikan bahwa capture layer bekerja.

## Konsekuensi

**Positif:** kebocoran kredensial dari embedded form menjadi mustahil · pelanggan bisa memasang form tanpa memahami keamanan API key · revoke form tidak mempengaruhi integrasi API

**Negatif:** dua konsep kredensial yang harus dijelaskan di dokumentasi · endpoint submit publik menanggung beban anti-abuse tersendiri

**Risiko yang harus dijaga:** akan ada godaan menyatukan keduanya demi "kesederhanaan". **Jangan.** Kesederhanaan itu dibayar dengan kebocoran otomatis pada setiap pemasangan form.
