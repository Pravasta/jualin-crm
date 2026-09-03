# ADR-013 — Penyimpanan Signing Secret (`whsec_`): Enkripsi, Bukan Hash

> **Status:** ✅ Accepted — 3 September 2026
> **Berlaku sejak:** Phase 7 (migration `0009`, issue #101)
> **Terkait:** [ADR-004](./ADR-004-api-key-format.md) · [ADR-005](./ADR-005-public-form-key.md) · Freeze §5 Aturan #20, #21, #26 · `docs/phases/07-outbound-webhook/td.md` §2.1 · `docs/issues/101-webhook-signature-enqueue.md`
> **Tidak membatalkan Aturan #20** — ADR ini menambahkan satu klausa pengecualian yang tegas batasnya. Aturan #20 apa adanya tetap berlaku untuk ketiga kredensial yang sudah ada.

## Konteks

Aturan #20 (freeze §5): *"Password: **argon2id**. API key: **SHA-256** + `subtle.ConstantTimeCompare`."*
Semangatnya, dijelaskan di [ADR-004](./ADR-004-api-key-format.md): kredensial tidak pernah
disimpan dalam bentuk yang bisa dikembalikan — kebocoran database tidak boleh menghasilkan
kredensial yang bisa dipakai.

Sampai Phase 6, ketiga kredensial produk ini cocok dengan aturan itu karena ketiganya punya
**arah kepercayaan yang sama**: pihak lain memegang rahasianya, mengirimkannya kepada kita, dan
**kita yang memverifikasi**.

| Kredensial | Siapa pegang rahasianya | Siapa memverifikasi | Perlu dibaca ulang? |
|---|---|---|---|
| `api_key` (`jln_live_…`) | integrator | **kita** | tidak → hash (SHA-256) |
| `public_key` form (`pk_…`) | halaman web pelanggan | **kita** | tidak → hash (SHA-256) |
| `refresh_token` | browser/perangkat pengguna | **kita** | tidak → hash (SHA-256) |

Untuk ketiganya, hash **cukup** dan hash **lebih baik**: kita hanya perlu tahu apakah nilai yang
masuk cocok, tidak pernah perlu nilai aslinya kembali.

### Phase 7 memperkenalkan kredensial dengan arah terbalik

Outbound webhook (`docs/phases/07-outbound-webhook/`) adalah phase pertama di mana **kita yang
memanggil** sistem lain. Pelanggan memberi kita URL; server kita meneleponnya; dan setiap kiriman
membawa tanda tangan supaya **penerima** bisa memastikan kiriman itu benar-benar dari kita:

```
X-Jualin-Signature: t=<unix>,v1=HMAC-SHA256(secret, "<t>.<body>")
                                            ↑ butuh secret ASLI sebagai kunci HMAC
```

HMAC-SHA256 memakai `secret` sebagai **kunci**. SHA-256 searah — tidak ada jalur apa pun yang bisa
mengembalikan secret dari kolom hash, dari `secret_prefix` (8 dari 49 karakter), maupun dari
pelanggan (`createRequest` tidak punya field `secret`). **Worker yang menyimpan hash tidak akan
pernah bisa menandatangani apa pun.**

Migration `0008` (issue #100) mengimplementasikan kolom ini sebagai `secret_hash`, menyalin pola
`api_key` dengan setia — dan itu **salah secara fatal**, ketahuan saat membaca ulang TD §2 sebelum
menulis `signature.go` (bukan lewat test: seluruh #100 hijau justru karena belum ada pemanggil yang
membutuhkan secret itu kembali). Dikoreksi migration `0009`: `secret_hash` → `secret_ciphertext bytea`.

**Kelas kesalahannya layak dicatat, bukan hanya perbaikannya:** komentar `0008` sudah menyatakan
dengan benar bahwa ini kredensial "yang pertama dengan arah kepercayaan terbalik", lalu tetap
menerapkan pola default di kolom tepat di bawahnya. Mengenali sebuah kasus itu berbeda **tidak**
otomatis mencegah penerapan pola yang sudah dikenal.

## Keputusan

### 1. Signing secret `whsec_` disimpan **terenkripsi**, bukan di-hash

`whsec_<43 karakter base64url>`, 32 byte `crypto/rand`. Disimpan **terenkripsi AES-256-GCM**
(`internal/shared/crypter`), kunci dari `WEBHOOK_SECRET_ENC_KEY`. `secret_prefix` (8 karakter
pertama) disimpan plaintext terpisah untuk ditampilkan di daftar.

- **AES-256-GCM, bukan sekadar "reversible"**: GCM authenticated — satu byte ciphertext berubah →
  dekripsi **gagal**, bukan menghasilkan plaintext yang berubah diam-diam.
- **`internal/shared/crypter` adalah satu-satunya tempat di codebase yang mengenkripsi**, bukan
  meng-hash. Dibuat khusus untuk kasus ini.

### 2. Klausa pengecualian ke Aturan #20 — batasnya tegas

> Aturan #20 mensyaratkan hash **untuk kredensial yang kita verifikasi**. Kredensial yang **kita
> pakai untuk menghasilkan bukti** (menandatangani, mengenkripsi ke pihak lain) — yaitu kredensial
> dengan arah kepercayaan terbalik — disimpan **terenkripsi reversibel** dengan kunci yang hidup di
> **environment**, bukan di database.

Syarat yang tidak boleh dilanggar oleh pengecualian ini:

| Syarat | Kenapa |
|---|---|
| Kunci enkripsi di environment, **tidak pernah** di database | Dump database saja tidak boleh cukup untuk memalsukan tanda tangan |
| Aturan #21 tetap berlaku — raw secret tampil **sekali**, saat dibuat | Bentuk penyimpanan tidak mengubah ini |
| Aturan #26 tetap berlaku — secret tidak pernah masuk log, termasuk jalur gagal | Bentuk penyimpanan tidak mengubah ini |
| Hanya berlaku bila **tidak ada** alternatif hash yang bisa memenuhi fungsinya | Bukan pintu keluar dari Aturan #20 secara umum |

### 3. Yang dilepas, disadari

*"Kebocoran database saja tidak cukup untuk memalsukan"* — properti yang hash berikan — **tidak bisa
dipertahankan** untuk kredensial yang harus kita pakai sendiri. Itu bukan pilihan desain, melainkan
sifat dari arah kepercayaan yang terbalik. Mitigasinya: kunci enkripsi di environment, sehingga dump
database saja tidak cukup.

### 4. `whsec_` sengaja tidak menyerupai `jln_live_` maupun `pk_`

Empat kredensial dengan aturan berlawanan tidak boleh terlihat mirip. `whsec_` mengikuti konvensi
prefix yang sudah dikenal integrator (Stripe memakai bentuk yang sama), dan arahnya terbalik dari
dua kredensial `jln_*`/`pk_*` sebelumnya.

## Konsekuensi

**Positif:**

- Worker bisa menandatangani — fungsi inti Phase 7 mungkin.
- Isi Aturan #20 di `freeze.md` tidak diubah — hanya diberi anotasi penunjuk `> ⚠️` ke ADR ini
  (mekanisme yang sama seperti [ADR-011](./ADR-011-layered-packages-and-unit-of-work.md) terhadap
  Aturan #8). Perubahan arsitekturnya ada di ADR ini, sebagaimana Aturan #30 mensyaratkan.
- Batas pengecualiannya tertulis, jadi kredensial kelima (inbound webhook Phase 7.5) tidak otomatis
  mewarisinya — inbound webhook **kita yang memverifikasi**, jadi ia kembali ke hash.

**Negatif:**

| Konsekuensi | Mitigasi |
|---|---|
| Merotasi `WEBHOOK_SECRET_ENC_KEY` membuat **seluruh** secret tersimpan tidak bisa didekripsi | Belum ada mekanisme rotasi bertahap. Bila dibutuhkan sebelum produksi: kolom versi kunci, bukan perubahan skema `0009`. Dicatat di `docs/issues/101`. Worker memperlakukan `Decrypt` gagal sebagai kegagalan **permanen** (`failed`), bukan retry selamanya. |
| Satu kelas kredensial kini menyimpang dari Aturan #20 apa adanya | Klausa §2 di atas, dan baris keempat tabel kredensial di `authentication.md`. Aturan #20 di `freeze.md` merujuk ke ADR ini lewat `authentication.md`, tidak diubah langsung. |
| `crypter` adalah primitif kripto baru di codebase | Satu implementasi, satu consumer (`internal/webhook`). AES-256-GCM dari `crypto/cipher` stdlib, bukan library pihak ketiga. Test membalik setiap bit di setiap posisi. |

## Batas ADR ini

**Inbound webhook (Phase 7.5) tidak termasuk.** Kredensial verifikasi inbound adalah rahasia yang
**pengirim** pegang dan **kita** verifikasi — arah kepercayaan yang sama seperti tiga kredensial
pertama. Ia di-hash, mengikuti Aturan #20 apa adanya. `docs/phases/07-outbound-webhook/td.md` §16
sudah menandai ini.

**Rotasi kunci bertahap tidak termasuk.** Bentuknya (kolom versi kunci) sudah diketahui bila
dibutuhkan, tapi belum ada pelanggan dan belum ada yang memintanya (Aturan #27, #28).
