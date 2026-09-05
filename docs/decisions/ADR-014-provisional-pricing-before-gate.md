# ADR-014 — Harga Provisional Ditetapkan Sebelum Gate Freeze

> **Status:** ✅ Accepted — 5 September 2026
> **Berlaku sejak:** Phase 8.5 (`docs/phases/08.5-paid-plans/`)
> **Terkait:** [ADR-012](./ADR-012-connect-surface-and-subscription-gating.md) §4 · Freeze §3 (🚦 Gate setelah Phase 5), 8.4 · `docs/phases/08-subscription/prd.md` D1 · `docs/STATUS.md` *Keputusan Belum Diambil*
> **Tidak membatalkan gate freeze** — ADR ini mengubah **urutannya**, bukan kewajibannya. Kewajiban meninjau ulang angka setelah pengguna nyata tetap berlaku, dan sekarang punya pemicu tertulis.

## Konteks

Dua dokumen mengikat angka harga dan limit ke pengguna nyata:

> `freeze.md` 🚦 *"Gate setelah Phase 5: cari 3–5 pengguna nyata sebelum Phase 6. Apa yang mereka minta
> akan berbeda dari tebakan kita… Sekaligus menguji tesis harga Anda dengan orang yang benar-benar
> membuka dompet."*

> [ADR-012](./ADR-012-connect-surface-and-subscription-gating.md) §4: angka ditetapkan *"setelah gate
> freeze (3–5 pengguna nyata) memberi data yang membuat angka itu bisa dipilih dengan jujur"* — dan
> ADR itu menamai risikonya sendiri: **angka tebakan bertahan karena terlanjur tertulis.**

Gate itu **belum terlewati**. Phase 6, 7, dan 8 dikerjakan melewatinya secara sadar, tercatat di
`STATUS.md`. Phase 8 menghormati batasnya dengan tidak menulis satu angka pun — ia membangun
mekanismenya saja, dan hasilnya adalah gerbang paket yang bekerja dengan **nol paket berbayar untuk
digerbangi**.

### Kenapa urutan itu sekarang menghalangi

Untuk mendapatkan 3–5 pengguna yang **benar-benar membuka dompet** — persis yang gate minta — produk
harus lebih dulu punya sesuatu untuk dibeli. Tanpa paket berbayar, yang bisa didatangkan hanyalah
pengguna gratis, dan mereka **tidak menguji tesis harga sama sekali**.

Urutan yang tertulis melingkar: angka menunggu pelanggan berbayar, pelanggan berbayar menunggu angka.

Ada juga alasan kedua yang tidak bisa ditunda tanpa biaya: **kuota yang dipasang setelah orang memakai
produk adalah pengambilan sesuatu yang sudah dimiliki.** Meluncur dengan Free tanpa batas lalu
membatasinya kemudian membutuhkan jalur downgrade yang belum ada — persis yang Phase 8 keputusan D4
tolak bangun karena transisinya belum bisa terjadi.

## Keputusan

**Angka harga, kuota, dan batas seat ditetapkan sebelum gate freeze, sebagai angka provisional.**

Tiga ketentuan mengikat:

1. **Ditandai provisional di kode dan dokumen** — peta `planLimits` (Go) dan tabel angka di
   `08.5-paid-plans/prd.md` keduanya menyatakan angkanya belum pernah diuji ke pengguna nyata.
2. **Wajib ditinjau ulang setelah 3–5 pelanggan berbayar pertama** — bukan setelah 3–5 *pengguna*.
   Yang gate minta adalah orang yang membuka dompet; angka pertama justru alat untuk mendapatkannya,
   bukan pengganti datanya.
3. **Peninjauan itu punya tempat yang menunggunya**: `STATUS.md` bagian *Keputusan Belum Diambil*
   mempertahankan baris pricing/limit/retensi sebagai **terbuka**, dengan pemicu baru "setelah 3–5
   pelanggan berbayar pertama" menggantikan "setelah gate freeze".

**Yang tidak berubah:** ADR-012 §2 (CRM tidak pernah tahu tentang uang — harga, checkout, invoice,
refund seluruhnya di payment service) dan ADR-012 §3 (batas ditegakkan di server, bukan di UI).
Angka yang ADR ini izinkan adalah **kuota dan batas**, ditambah satu label harga untuk ditampilkan —
bukan perhitungan uang apa pun di dalam CRM.

## Alternatif yang ditolak

| Alternatif | Kenapa ditolak |
|---|---|
| **Tunggu gate freeze apa adanya** | Melingkar (lihat Konteks). Menunggu pelanggan berbayar sebelum ada yang bisa dibeli tidak akan pernah selesai sendiri. |
| **Luncurkan gratis dulu, pasang harga & kuota belakangan** | Membutuhkan downgrade terhadap pengguna yang sudah memakai — jalur yang tidak ada, dan pengalaman produk yang buruk. Phase 8 D4 menolak membangun penegakan untuk transisi yang belum bisa terjadi; ini kebalikannya: menciptakan transisi yang menyakitkan tanpa perlu. |
| **Tetapkan angka, lalu perlakukan sebagai final** | Persis risiko yang ADR-012 §4 namai. Ketentuan 1–3 di atas ada untuk mencegahnya. |
| **Bangun `usage_counters` sekalian** | Ditolak terpisah di `08.5-paid-plans/prd.md` D1 — `COUNT` atas `leads` (termasuk soft-deleted) memberi semantik yang sama tanpa tabel dan tanpa jalur tulis baru. Kewajiban Phase 8 D1 dengan demikian terjawab, bukan ditunda lagi. |

## Konsekuensi

**Yang menjadi mungkin.** Produk bisa diluncurkan dengan sesuatu untuk dijual, sehingga gate freeze
bisa dilewati dengan data yang benar — orang yang membayar, bukan orang yang mencoba.

**Yang menjadi risiko, dan harus dibaca jujur.** Angka pertama hampir pasti salah. Tiga bentuk
salahnya, dan bagaimana masing-masing ditangani:

| Bentuk salah | Penanganan |
|---|---|
| **Kuota Free terlalu longgar** — tidak ada yang perlu naik paket | Menaikkan harga/menurunkan kuota bagi pengguna yang sudah ada = downgrade. Karena itu kuota Free sebaiknya **ketat sejak awal**; melonggarkannya kemudian tidak menyakiti siapa pun. |
| **Kuota Free terlalu ketat** — orang pergi sebelum melihat nilainya | Melonggarkan aman dan bisa dilakukan kapan saja lewat satu baris peta. |
| **Harga Pro salah** | Hanya menyentuh label yang ditampilkan + apa yang ditagih payment service; tidak ada perhitungan uang di CRM yang ikut berubah (ADR-012 §2). |

**Kewajiban yang lahir dari ADR ini** — dicatat supaya tidak hilang bersama euforia peluncuran:
setelah 3–5 pelanggan berbayar pertama, tinjau ulang **keempatnya bersama-sama** — kuota Free, kuota
Pro, harga Pro, dan batas seat — karena keempatnya satu keputusan produk, bukan empat yang terpisah.
