# Phase 8.5 — Paket Berbayar & Kuota · PRD

> **Apa & kenapa.** Detail teknis di [`td.md`](./td.md).
> Sumber: [ADR-012](../../decisions/ADR-012-connect-surface-and-subscription-gating.md) §4 (angka menunggu gate freeze — **dikoreksi** oleh ADR-014) · [ADR-014](../../decisions/ADR-014-provisional-pricing-before-gate.md) (harga provisional ditetapkan sebelum gate) · [`08-subscription/prd.md`](../08-subscription/prd.md) **D1** (`usage_counters` dievaluasi ulang saat angka mendarat) · [`architecture/freeze.md`](../../architecture/freeze.md) 8.4

---

## Dua hal yang harus dilaporkan sebelum apa pun (Aturan #30)

**1. Nomor phase ini menyimpang dari `freeze.md`.** Freeze memesan **Phase 9** untuk *"Deal, Pipeline
entity, reports lanjutan, automation"*. Pekerjaan ini bukan itu — ia lanjutan langsung Phase 8, jadi
disisipkan sebagai **8.5**, mengikuti preseden `04.5-hardening`, `04.6-email-delivery`, dan
`07.5-inbound-webhook`. Phase 9 tetap milik Deal & Pipeline; urutannya tidak diubah, hanya didahului.

**2. Menetapkan harga sekarang bertentangan dengan ADR-012 §4.** ADR itu mengikat angka ke *"setelah
gate freeze (3–5 pengguna nyata)"*, dan gate itu **belum terlewati**. Keputusan pemilik produk
(5 September 2026) adalah meluncur dengan harga provisional lebih dulu. Itu keputusan yang sah, tapi
mengubah premis ADR — jadi ia ditutup lewat **ADR-014**, bukan diselipkan ke dalam PRD ini.

---

## Tujuan

**Batas paket berhenti jadi mekanisme kosong.** Phase 8 membangun gerbangnya dan sengaja tidak mengisi
satu angka pun; hasilnya produk punya penegakan yang bekerja dengan **nol paket berbayar untuk
digerbangi**. Phase 8.5 mengisi angkanya, menambah dimensi yang Phase 8 tidak punya (**berapa
banyak**, bukan hanya **kanal apa**), dan memberi pelanggan layar untuk melihat serta menaikkan
paketnya.

Tiga paket: **Free**, **Pro** (berbayar, self-serve), **Enterprise** (berbayar, lewat percakapan).

> **Kenapa kuota dipasang sebelum peluncuran, bukan sesudah.** Kalau produk diluncurkan dengan Free
> tanpa batas lalu kuota dipasang belakangan, setiap pengguna yang sudah memakai **kehilangan
> sesuatu** — dan itu perilaku downgrade, yang Phase 8 keputusan D4 tolak justru karena jalurnya
> belum ada. Memasang kuota sejak hari pertama jauh lebih murah daripada menariknya kemudian.

---

## Kebutuhan

| # | Sebagai… | Saya butuh… | Supaya… |
|---|---|---|---|
| 1 | Pemilik produk | Paket berbayar yang **benar-benar membatasi sesuatu**, bukan sekadar label | Pelanggan tidak punya alasan naik paket kalau Free sudah tak terbatas |
| 2 | Owner | Melihat **berapa banyak yang sudah saya pakai bulan ini** terhadap batas saya | Batas yang baru terasa saat ditolak adalah batas yang membuat marah, bukan yang membuat upgrade |
| 3 | Owner | Tahu apa yang saya dapat kalau naik paket, tanpa harus bertanya | Pelanggan berharga murah harus bisa onboard sendiri — sama alasannya dengan halaman dokumentasi API (`freeze.md` Phase 4) |
| 4 | Pemilik produk | Bisa menaikkan paket pelanggan **sebelum** payment service tersambung | Pelanggan pertama akan bayar lewat transfer/WA jauh sebelum ada checkout otomatis |
| 5 | Pemilik sistem | Kuota ditegakkan **di server**, di seluruh jalur pembuatan lead | Kuota yang hanya dihitung di dashboard dilewati siapa pun yang memakai API key |
| 6 | Pemilik sistem | Angka paket hidup di **satu tempat**, di sebelah peta kanal Phase 8 | Dua tempat berbeda untuk "apa yang paket ini dapat" akan menyimpang |
| 7 | Pemilik produk | Enterprise **tidak** punya checkout sendiri | Harganya negosiasi; tombol beli untuk sesuatu yang harganya belum tentu adalah janji yang tidak bisa ditepati |

---

## Acceptance Criteria

Phase 8.5 selesai bila **semuanya** terpenuhi:

| # | Kriteria |
|---|---|
| 1 | Tiga paket ada di **satu peta**, di berkas yang sama dengan `planChannels` — menambah paket atau mengubah kuota adalah perubahan **satu baris**, bukan perubahan usecase manapun |
| 2 | Kuota lead bulanan ditegakkan di **ketiga** jalur pembuatan lead: dashboard/mobile (user), `POST /v1/leads` (API key), dan submit form publik — **dibuktikan lewat `curl`**, bukan lewat UI |
| 3 | Batas seat ditegakkan saat mengundang anggota — organization yang penuh menerima penolakan yang menyebut batasnya, bukan error generik |
| 4 | Paket yang tidak dikenal peta → diperlakukan sebagai **paling ketat**, bukan tanpa batas — gagal tertutup, konsisten dengan Phase 8 kriteria #8 |
| 5 | `GET /v1/me` membawa **pemakaian dan batas** yang sudah diselesaikan (`usage` + `limits`), bukan bahan mentah untuk dihitung klien — bentuk yang sama dengan `plan.channels` (Phase 8 D5) |
| 6 | Layar **Langganan** di dashboard menampilkan paket aktif, pemakaian terhadap batas, dan perbandingan ketiga paket. Enterprise mengarah ke percakapan, bukan checkout |
| 7 | Ada jalur yang benar-benar **mengubah `plan_code`** sebuah organization, tercatat di `audit_log`, dan **tidak bisa dipanggil pelanggan untuk menaikkan paketnya sendiri** |
| 8 | Kuota terlampaui **tidak pernah** membuat pengunjung situs pelanggan melihat apa pun tentang keadaan paket pelanggan (batas Phase 8 §11 tetap berlaku) |
| 9 | Angka provisional hidup **hanya** di satu peta Go dan satu berkas dokumen — tidak tersebar ke usecase, migration, atau TypeScript |
| 10 | `go test -race ./...` dan `npm run typecheck && lint && test && build` bersih |

---

## Keputusan yang ditutup di phase ini

| # | Pertanyaan | Keputusan | Alasan |
|---|---|---|---|
| **D1** | Tabel `usage_counters` dibangun sekarang? | **Tidak — dan ini jawaban final atas kewajiban Phase 8 D1.** Kuota dihitung `COUNT` atas `leads` bulan berjalan, **termasuk yang soft-deleted**. | Phase 8 D1 mewajibkan evaluasi ulang "saat angka mendarat"; angkanya sekarang mendarat, dan jawabannya tetap tidak. `leads` sudah punya `organization_id` + `created_at` + index `ix_leads_org_created`; menghitungnya satu query. Menyertakan baris `deleted_at IS NOT NULL` membuat semantiknya identik dengan penghitung (menghapus lead **tidak** mengembalikan kuota) **tanpa** tabel baru dan tanpa jalur tulis di tiga usecase. Aturan #27/#28: tabel yang tidak menambah kemampuan apa pun adalah biaya tanpa hasil. |
| **D2** | Kuota bisa terlampaui di bawah konkurensi? | **Ya, 1–2 baris, dan itu diterima.** Tidak ada penguncian di jalur pembuatan lead. | Dua request bersamaan di ambang batas bisa sama-sama lolos cek. Konsekuensi komersialnya nol (101 lead di paket 100); biaya menghilangkannya adalah lock pada **setiap** pembuatan lead — kontensi di jalur terpanas produk, demi ketepatan yang tidak ada yang bayar. Dicatat, bukan ditemukan belakangan. |
| **D3** | Apa yang terjadi saat kuota habis di **form publik**? | ⚠️ **Butuh konfirmasi pemilik produk — rekomendasi: lead tetap diterima, Owner diberi tahu.** | Ini satu-satunya keputusan phase ini yang belum ditutup, dan yang paling berdampak. Menolak submit berarti form di situs pelanggan **berhenti bekerja** — pelanggan kehilangan lead sungguhan, dan pengunjung melihat error tentang tagihan orang lain (melanggar kriteria #8). Menerimanya berarti kuota tidak tertegak di jalur bervolume tertinggi. Rekomendasi: terima, tandai, beri tahu Owner, dan tegakkan keras hanya di jalur **terautentikasi** (dashboard/mobile/API key) — pelanggan yang melewati batas tetap dapat lead-nya, tapi tidak bisa menambah kanal atau mengundang orang sampai naik paket. |
| **D4** | Enterprise punya checkout? | **Tidak.** Kartunya mengarah ke percakapan (WhatsApp/email), bukan pembayaran. | Kebutuhan #7. Harga enterprise adalah negosiasi; tombol beli untuk angka yang belum tentu adalah janji yang tidak bisa ditepati. Bentuk yang sama dengan kartu terkunci Phase 8 D6 — satu tautan keluar, bukan alur baru. |
| **D5** | Bagaimana `plan_code` berubah sebelum payment tersambung? | **Endpoint internal ber-token**, terpisah dari sesi user, tercatat di `audit_log`. **Bukan** tombol yang bisa diklik Owner untuk menaikkan paketnya sendiri. | Kriteria #7. Tombol "test beli" yang tersedia untuk Owner di produksi berarti setiap pelanggan bisa memberi dirinya Pro gratis — itu bukan fitur, itu lubang. Tombol test tetap boleh ada, tapi **hanya** saat `SUBSCRIPTION_TEST_CHECKOUT=true`, yang ditolak saat `APP_ENV=production` (pola persis `WEBHOOK_ALLOW_PRIVATE_TARGETS`, #100). |
| **D6** | Integrasi payment service dikerjakan di phase ini? | **Tidak** — ditunda ke phase akhir sesuai keputusan pemilik produk (5 Sep 2026). | Service-nya sudah ada, tapi kontrak integrasinya belum ditulis. Yang dibangun sekarang adalah **tempat sambungnya**: `external_reference` sudah ada sejak `0002`, dan endpoint perubahan paket D5 adalah bentuk yang sama yang kelak dipanggil webhook pembayaran — bukan alur berbeda yang harus ditulis ulang. |
| **D7** | Angka provisional ditulis di mana? | **Satu peta Go** (`planLimits`, di sebelah `planChannels`) + satu tabel di dokumen ini. Tidak di database, tidak di env, tidak di TypeScript. | Kriteria #9, dan alasan yang sama dengan Phase 8 D3: angka ini kebijakan produk, bukan data pelanggan. Sebagai literal Go ia ikut review, ikut test, ikut deploy. |

---

## Angka provisional — **belum diisi**

Bentuk tabelnya ditetapkan phase ini; isinya keputusan pemilik produk sebelum rilis. Sampai diisi,
`planLimits` memakai nilai sementara yang **ditandai jelas di kode** sebagai belum final.

| | Free | Pro | Enterprise |
|---|---|---|---|
| Lead / bulan | _(?)_ | _(?)_ | _(?)_ atau tanpa batas |
| Seat (anggota aktif) | _(?)_ | _(?)_ | _(?)_ atau tanpa batas |
| Kanal | Formulir _(?)_ | ketiganya _(?)_ | ketiganya |
| Harga | Rp0 | _(?)_ /bulan | negosiasi |

**Yang mengunci angka ini bukan kode, melainkan keputusan.** ADR-014 mencatat bahwa angka pertama
ditetapkan tanpa data pengguna, dan **wajib ditinjau ulang setelah 3–5 pelanggan pertama** — kewajiban
itu tidak hilang hanya karena gate-nya dilewati.

---

## Di luar cakupan

| Tidak dikerjakan | Ke |
|---|---|
| **Integrasi payment service** — checkout, kartu, invoice, refund, webhook pembayaran | Phase akhir (D6). ADR-012 §2 tetap: seluruhnya di service terpisah, bukan di repository ini |
| **Alur downgrade otomatis** — apa yang terjadi pada form/API key/webhook yang sudah ada saat paket turun | Belum ada yang bisa menurunkan paket kecuali tindakan manual pemilik produk. Phase 8 D4 tetap berlaku: menutup resource yang sudah jalan butuh keputusan produk tersendiri |
| **Proration, trial, kupon, invoice** | Seluruhnya urusan payment service |
| **Notifikasi otomatis saat mendekati kuota** (mis. email di 80%) | Layar Langganan menampilkan pemakaian; pemberitahuan aktif menunggu bukti bahwa layarnya saja tidak cukup (Aturan #29) |
| **Kuota selain lead dan seat** (penyimpanan, webhook terkirim, panggilan API) | Saat ada bukti salah satunya jadi biaya nyata per tenant |
| **Panel admin lengkap** | D5 hanya membangun satu endpoint berubah-paket, bukan permukaan admin |

---

## Dependensi

Phase 8 selesai (#112–#115). Yang dipakai phase ini sudah ada seluruhnya:

| Yang dipakai | Dari | Catatan |
|---|---|---|
| `planChannels` + `channelsFor` + `ParseChannel` | Phase 8 (#112) | `planLimits` tinggal di berkas yang sama, dibaca lewat resolver yang sama |
| `PlanGate` di `apikey`/`form`/`webhook` | Phase 8 (#113) | Kuota **tidak** memakai interface ini — ia pertanyaan berbeda ("berapa banyak", bukan "kanal apa") dan hidup di `lead`/`membership`. Lihat `td.md` §3 |
| `plan` di `GET /v1/me` + `lib/plan.ts` | Phase 8 (#112, #114) | Bertambah `limits` + `usage`; dashboard tetap tidak pernah menghitung apa pun sendiri |
| `audit_log` | Phase 1 | Perubahan paket dicatat di sini, sama seperti perubahan role |
| Pola env ditolak-di-produksi | Phase 7 (#100, `WEBHOOK_ALLOW_PRIVATE_TARGETS`) | Dipakai ulang untuk `SUBSCRIPTION_TEST_CHECKOUT` |
| `ix_leads_org_created` | Phase 2 (`0003`) | Index yang membuat `COUNT` bulanan D1 murah — bukan index baru |

**Satu poin terbuka yang wajib dijawab sebelum TD final:** keputusan **D3** (form publik saat kuota
habis). Seluruh §5 TD bergantung padanya.
