# ADR-015 — Tidak Ada Jalan Kembali ke `new`

> **Status:** ✅ Accepted — 20 September 2026
> **Berlaku sejak:** issue #139
> **Mengamandemen:** [ADR-006](./ADR-006-lead-status-as-pipeline.md) aturan turunan #3 · `freeze.md` 2.4 aturan #3 dan B7 · `docs/phases/02-crm-core/td.md` §5 · `docs/phases/02-crm-core/prd.md` B7 (**alasannya**, bukan hanya rumusannya — lihat *Yang dikorbankan*)
> **Tidak mengubah** arti status lain, transisi mundur lainnya, atau rencana perpindahan `proposal`/`won` ke Deal (ADR-006)

## Konteks

Lead berstatus **Dihubungi** menawarkan tombol **"→ Baru"**. Ditemukan pemilik produk saat uji manual `03-lead-dan-pipeline.md`, dan ternyata bukan bug kode: backend, dashboard, dan mobile menerapkan aturan yang sama persis, mengikuti teks yang mengikat:

> ADR-006 #3, `freeze.md` 2.4 #3: *"Mundur satu langkah diizinkan (`qualified → contacted`)"*
> `td.md` §5 (B7): *"`qualified → contacted` boleh, `proposal → new` tidak"*

Ketiganya memakai contoh yang **sama** — `qualified → contacted` — dan tidak satu pun memeriksa `contacted → new`. Aturannya ditulis umum, diilustrasikan dengan kasus yang masuk akal, dan kasus yang tidak masuk akal tidak pernah dilihat.

Ada selisih yang nyata di antara status yang bisa dimundurkan:

| Status | Sifatnya | Mundur ke sana masuk akal? |
|---|---|---|
| `contacted` · `qualified` · `proposal` | **tahapan kerja** — sesuatu yang sedang dikerjakan | Ya — wajar mengakui terlalu cepat menaikkan |
| `new` | **pernyataan riwayat** — ADR-006: *"Baru masuk, **belum disentuh**"* | Tidak — begitu sebuah lead dihubungi, "belum disentuh" tidak akan pernah benar lagi |

Akibat yang terlihat: lead bisa berstatus `new` sementara timeline-nya memuat `status_changed`. Activity **append-only** (#21) sehingga jejak itu tidak ikut mundur — dua sumber di layar yang sama saling bertentangan. Chip hitungan status di dashboard (`by_status`) juga menghitungnya sebagai Baru lagi.

Pertentangan yang sama ada di jalur **keluar dari `lost`**: satu-satunya tombol buka-kembali yang ditawarkan dashboard dan mobile adalah "→ Baru". Lead yang pernah kalah lalu dibuka kembali sebagai "belum disentuh" adalah kontradiksi yang persis sama, lewat pintu lain.

## Keputusan

**`new` tidak dapat dicapai dari status mana pun.** Sebuah lead hanya pernah *mulai* di `new` — saat dibuat — dan tidak pernah kembali.

| | Sebelum | Sesudah |
|---|---|---|
| `contacted → new` | ✅ | ❌ `422 invalid_status_transition` |
| `lost → new` | ✅ | ❌ `422 invalid_status_transition` |
| `lost → contacted` · `qualified` · `proposal` · `won` | ✅ | ✅ (tidak berubah) |
| Mundur lain (`qualified → contacted`, `proposal → qualified`, `won → proposal`) | ✅ | ✅ (tidak berubah) |

**Tombol buka-kembali di UI menjadi "→ Buka kembali ke Dihubungi"**, di dashboard dan mobile: tahap paling awal yang jujur bagi lead yang pernah kalah. Backend tetap membolehkan `lost` ke semua status jalur utama selain `new` lewat API — aproksimasi yang sudah tertulis sejak #20 (riwayat sebelum kalah tidak tersimpan), tidak diubah di sini. UI sengaja menawarkan satu pilihan agar tidak menjadi tembok tombol untuk kasus yang jarang.

Aturannya ditegakkan di **usecase** (`validateStatusTransition`), sehingga mobile dan API ikut — bukan sekadar tombol yang disembunyikan. Aturan itu ada di **tiga salinan** yang harus tetap identik (Go, `crm_dashboard/src/lib/lead-status.ts`, `crm_employee/lib/features/leads/domain/lead_status.dart`), masing-masing dengan tes matriks yang ditulis tangan, terpisah dari implementasinya.

Nol migration. Lead yang **sudah** berstatus `new` (termasuk yang sempat dimundurkan sebelum aturan ini) tidak disentuh dan tetap bisa maju seperti biasa.

## Yang dikorbankan — dan kenapa ini mengamandemen B7, bukan hanya memperluasnya

`prd.md` Phase 2 B7 memutuskan mundur satu langkah dengan alasan:

> *"Salah klik adalah kejadian normal; membuatnya permanen berarti butuh jalur koreksi tersendiri."*

Alasan itu **berlaku persis** untuk kasus yang ditutup di sini: `new → contacted` yang salah klik tidak bisa lagi dibatalkan, karena `contacted → new` adalah satu-satunya cara membatalkannya. Ini bukan efek samping yang terlewat — ini harga yang dipilih dengan sadar. Tombol "Dihubungi" dan "Memenuhi Syarat" bersebelahan, jadi salah klik nyata; lead yang salah dinaikkan tetap berstatus Dihubungi, dan pengguna maju atau menutupnya (`lost`/`unqualified`/`spam`) dari sana.

Keputusannya: kejujuran timeline dan makna `new` lebih berharga daripada satu jalur koreksi untuk salah klik tunggal. Kalau ternyata pengguna nyata sering terjebak di sini, jawaban yang benar adalah **jalur koreksi yang eksplisit** (mis. membatalkan perubahan status terakhir dan menuliskannya di timeline), bukan membuka kembali `contacted → new` — karena yang kedua menghidupkan lagi pertentangan yang ditutup ADR ini.

## Alternatif yang ditolak

1. **Biarkan apa adanya.** Mempertahankan satu jalur koreksi, tetapi `new` tetap bisa berbohong tentang riwayat, dan pertentangan lewat jalur `lost` tetap ada.
2. **Larang hanya `contacted → new`, biarkan `lost → new`.** Perubahan terkecil, tapi hanya menyelesaikan setengah masalah — pertentangan yang sama tetap hidup lewat pintu `lost`.
3. **Ubah arti `new` menjadi tahapan kerja.** Nol perubahan kode, tetapi mengubah ADR-006, `freeze.md` 2.4, dan `labels` di dashboard **dan** mobile, dan menghapus satu-satunya status yang bermakna "belum ada yang menyentuh ini" — status yang justru berguna untuk membedakan lead yang menunggu ditangani.

## Konsekuensi

**Positif:** `new` kembali berarti satu hal saja · timeline dan status tidak lagi saling bertentangan · chip dan filter "Baru" di dashboard menunjukkan lead yang benar-benar belum ditangani.

**Negatif:** salah klik `new → contacted` tidak bisa dibatalkan · tiga salinan aturan yang harus dijaga identik (sudah demikian sebelum ADR ini; ADR ini tidak menambahnya, tetapi mengingatkannya).

**Terus terang, yang tidak diubah:** `total_new` di `GET /v1/metrics/summary` **tidak** terpengaruh — isinya total lead pada periode, bukan hitungan status `new`; namanya saja yang menyesatkan. `by_status` dan `conversion_rate` juga tidak terpengaruh secara struktural, hanya isinya yang kini lebih jujur.

**Kapan dievaluasi ulang:** bila pengguna nyata melaporkan terjebak setelah salah klik — dan jawabannya jalur koreksi yang eksplisit, bukan mencabut ADR ini (lihat *Yang dikorbankan*).
