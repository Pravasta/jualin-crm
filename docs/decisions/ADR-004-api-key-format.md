# ADR-004 — Format & Hashing API Key

> **Status:** ✅ Accepted — 17 Agustus 2026
> **Berlaku sejak:** Phase 4

## Konteks

API key dipakai sistem eksternal pelanggan untuk mengirim lead ke Jualin CRM. Ia harus tidak bisa ditebak, bisa direvoke, dan **tidak pernah disimpan plaintext**.

Refleks yang umum adalah menyimpannya dengan bcrypt atau argon2 — sama seperti password. **Refleks itu salah di sini**, dan salahnya bukan hanya soal performa.

## Masalah dengan bcrypt/argon2 untuk API key

bcrypt dan argon2 **di-salt per baris**. Konsekuensinya: tidak ada cara melakukan lookup.

Setiap request harus membandingkan kredensial masuk terhadap **seluruh baris** di tabel:

```
10.000 API key  ×  1 operasi argon2 (~100ms)  =  sistem berhenti berfungsi
```

Ini bukan masalah optimasi. Skema ini tidak bisa bekerja sama sekali begitu jumlah key bertambah.

## Keputusan

### Format

```
jln_live_<key_id:12char>_<secret:43char>
    │         │                │
    │         │                └─ crypto/rand, entropi 256-bit
    │         └─ pencari — disimpan plaintext, ter-index
    └─ environment: live | test
```

> **Diperbaiki saat Phase 4 ditutup (issue #49).** Baris di atas sebelumnya menulis `<secret:32char>`
> pada kalimat yang sama dengan "entropi 256-bit" — dua hal itu tidak bisa benar bersamaan (32 karakter
> base64url ≈ 192 bit; 256 bit = 32 byte = 43 karakter base64url). Kode (`internal/apikey/entity.go`,
> issue #46) mengambil 256-bit sejak awal — argumen "kenapa SHA-256 aman" di bawah bersandar penuh pada
> angka itu — sehingga teks di sinilah yang diperbaiki mengikuti kode, bukan sebaliknya. Dicatat sebagai
> koreksi, bukan didiamkan (Aturan #30).

### Penyimpanan

| Kolom | Isi |
|---|---|
| `key_id` | Plaintext, **UNIQUE, ter-index** — ini yang dicari |
| `secret_hash` | **SHA-256** dari bagian secret |
| `key_prefix` | `jln_live_` + 4 karakter pertama `key_id`, untuk ditampilkan di dashboard (`jln_live_a3f9…`) — teks ini sebelumnya menulis "8 karakter pertama", yang tidak cocok dengan contohnya sendiri; diperbaiki saat Phase 4 ditutup (issue #49) mengikuti kode (`internal/apikey/entity.go`) |
| `name`, `scopes`, `created_by_membership_id` | Metadata |
| `created_at`, `last_used_at`, `revoked_at`, `expires_at` | Lifecycle |

### Verifikasi

```
1. Parse key_id dari kredensial
2. SELECT ... WHERE key_id = $1 AND revoked_at IS NULL     ← index hit, O(1)
3. subtle.ConstantTimeCompare(sha256(secret), secret_hash)
```

## Kenapa SHA-256 aman di sini, padahal tidak untuk password

Perbedaannya ada pada **sumber entropi**, bukan pada nilai datanya.

| | Password | API key |
|---|---|---|
| Dibuat oleh | Manusia | `crypto/rand` |
| Entropi | Rendah — bisa ditebak dari dictionary | **256-bit** |
| Ancaman | Brute-force / rainbow table | Tidak relevan |
| Hash lambat membantu? | ✅ Ya — memperlambat penebakan | ❌ Tidak — ruangnya sudah mustahil ditembus |

argon2 sengaja lambat untuk melawan penebakan pada rahasia berentropi rendah. Pada rahasia 256-bit dari `crypto/rand`, brute-force mustahil **terlepas dari kecepatan hash**.

> Memakai argon2 di sini hanya menambah ratusan milidetik ke **setiap request API panas** tanpa manfaat keamanan nyata.

**Untuk password user, tetap argon2id.** Dua kasus berbeda, dua jawaban berbeda. Ini bukan inkonsistensi.

## Aturan operasional

| # | Aturan |
|---|---|
| 1 | Raw secret ditampilkan **sekali** saat dibuat. Setelah itu tidak bisa dilihat lagi oleh siapapun. |
| 2 | API key **milik organization**, bukan user. Bila pembuatnya keluar, integrasi tidak boleh mati. `created_by_membership_id` hanya untuk audit. |
| 3 | `last_used_at` di-update **async atau throttled**, bukan setiap request — kalau tidak, tabel ini menjadi write hotspot. |
| 4 | Kolom `scopes` ada sejak awal meski MVP hanya memakai `leads:write`. Menambah scope ke key yang sudah beredar adalah **breaking change**. |
| 5 | API key **tidak pernah** hadir di sisi klien — browser maupun aplikasi mobile yang didistribusikan (Aturan #23, #24). |

## Konsekuensi

**Positif:** lookup O(1) · perbandingan constant-time · revoke instan · prefix bisa ditampilkan tanpa membocorkan secret

**Negatif:** `key_id` tersimpan plaintext — tapi ia memang bukan rahasia, hanya pencari

**Risiko yang harus dijaga:** SHA-256 di sini akan terlihat seperti kesalahan keamanan bagi yang membaca sekilas. **Itulah alasan ADR ini ditulis.** Jangan diubah tanpa membaca bagian "Kenapa SHA-256 aman di sini".
