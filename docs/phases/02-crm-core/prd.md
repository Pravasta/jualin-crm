# Phase 2 — CRM Core · PRD

> **Apa & kenapa.** Detail teknis di [`td.md`](./td.md).
> Sumber: [`architecture/freeze.md`](../../architecture/freeze.md) bagian 4 (Phase 2), 2.3 (keputusan modeling), 2.4 (lead status), 2.5 (activity type), 8.4 (migration `0003`/`0004`), A3 (notification) · [ADR-006](../../decisions/ADR-006-lead-status-as-pipeline.md)

---

## Tujuan

Membangun **produk yang sebenarnya**: siklus hidup lead dari masuk sampai menjadi pelanggan.

Phase 0 menyiapkan lantai, Phase 1 menyiapkan dinding tenancy. Phase 2 adalah yang pertama menghasilkan sesuatu yang bisa disebut CRM — dan yang pertama menyimpan **data pelanggan milik pelanggan Anda**.

> Ini juga phase yang membuktikan bahwa fondasi Phase 1 benar. Harness isolasi tenant sampai sekarang hanya bisa menguji membership dan invitation; begitu `leads` ada, ia akhirnya bisa menguji apa yang sebenarnya dijaga — data bisnis, dan aturan "employee hanya melihat lead miliknya".

---

## Kebutuhan

| # | Sebagai… | Saya butuh… | Supaya… |
|---|---|---|---|
| 1 | Owner | Mencatat lead yang masuk lewat jalur apapun | Tidak ada calon pembeli yang hilang di WhatsApp atau catatan kertas |
| 2 | Owner | Menyebut lead dengan nomor yang bisa diucapkan (`#1024`) | Bisa membahasnya di telepon dan grup tanpa membacakan UUID |
| 3 | Owner | Menugaskan lead ke sales tertentu | Setiap lead punya penanggung jawab yang jelas |
| 4 | Employee | Melihat **hanya** lead yang ditugaskan ke saya | Tidak tenggelam dalam daftar seluruh organisasi |
| 5 | Employee | Diberi tahu saat ada lead baru untuk saya | Bisa merespons cepat, tanpa harus memantau layar |
| 6 | Siapa pun | Melihat riwayat lengkap apa yang sudah terjadi pada sebuah lead | Bisa melanjutkan pekerjaan orang lain tanpa bertanya |
| 7 | Employee | Mencatat tindak lanjut yang harus dilakukan, dengan tenggat | Tidak lupa menelepon balik |
| 8 | Owner | Mengubah status lead mengikuti kemajuan nyata | Bisa melihat funnel yang mencerminkan keadaan sebenarnya |
| 9 | Owner | Mengubah lead yang berhasil menjadi pelanggan | Relasi jangka panjang tercatat terpisah dari proses penjualannya |
| 10 | Owner | Menyaring lead (status, pemilik, sumber, periode, kata kunci) | Bisa menjawab "mana yang belum ditangani" dalam hitungan detik |
| 11 | Sistem pengirim (Phase 4) | Mengirim ulang request yang timeout tanpa membuat lead ganda | Retry tidak merusak data pelanggan |

---

## Acceptance Criteria

Phase 2 selesai bila **semuanya** terpenuhi:

| # | Kriteria |
|---|---|
| 1 | Lead bisa dibuat, dibaca, diubah, dan di-soft-delete lewat API internal, seluruhnya tenant-scoped |
| 2 | `lead_number` dialokasikan **berurutan per organization mulai dari 1**, tanpa lubang dan tanpa duplikat, **dibuktikan di bawah pembuatan bersamaan** |
| 3 | Perubahan bersamaan pada lead/task yang sama → tepat satu berhasil, sisanya **409** dengan keadaan terkini — tidak pernah menimpa diam-diam |
| 4 | Transisi status divalidasi di usecase: mundur satu langkah diizinkan, melompat maju ditolak, `lost` wajib disertai `lost_reason` |
| 5 | `spam` dan `unqualified` **dikecualikan** dari perhitungan konversi |
| 6 | Nomor telepon dinormalisasi ke E.164; nomor yang tidak bisa diurai **tetap diterima** dan tersimpan apa adanya |
| 7 | `Idempotency-Key` yang diulang mengembalikan **response asli**, bukan lead kedua dan bukan error |
| 8 | Setiap peristiwa penting pada lead menghasilkan activity **otomatis**, dalam transaksi yang sama — timeline tidak pernah bercerai dari kejadiannya |
| 9 | Activity **append-only**: tidak ada endpoint edit maupun hapus, dan tidak ada kolom `updated_at`/`deleted_at` |
| 10 | Assignment menghasilkan notification untuk employee yang ditugaskan, dalam transaksi yang sama |
| 11 | **Employee hanya bisa membaca lead yang di-assign kepadanya** — ditegakkan di **repository**, dan lead milik employee lain → **404**, bukan 403 |
| 12 | Konversi ke Customer adalah **aksi eksplisit**; masuk status `won` tidak mengonversi apapun secara otomatis |
| 13 | Menonaktifkan membership **menolak berjalan** bila masih ada lead terbuka miliknya, kecuali disertai keputusan eksplisit (reassign / lepas assignment) — kewajiban yang diteruskan Phase 1 |
| 14 | Harness isolasi tenant bertambah kasus `lead` dan **tetap terbukti bisa gagal** |

---

## Keputusan yang ditutup di phase ini

Empat keputusan yang `STATUS.md` tandai *"diputuskan sebelum Phase 2"* — ditutup di sini, bukan diserahkan ke tengah implementasi:

| # | Pertanyaan | Keputusan | Alasan |
|---|---|---|---|
| **B6** | Daftar `lost_reason` final | `price` · `competitor` · `timing` · `no_response` · `not_interested` · `other` | Menutup alasan kalah yang lazim, `other` sebagai katup pengaman. Teks bebas ditolak: *"kemahalan"*, *"harga"*, dan *"mahal"* akan menjadi tiga kategori berbeda dan laporan kalah-menang kehilangan artinya. |
| **B7** | Boleh mengubah status mundur? | **Ya, satu langkah.** Melompat maju ditolak. | Salah klik adalah kejadian normal; membuatnya permanen berarti butuh jalur koreksi tersendiri. Mundur bebas ditolak karena membuat funnel tidak bermakna — lead bisa "lahir kembali" berkali-kali. |
| **B8** | Task boleh berdiri tanpa Lead? | **Tidak** — `lead_id NOT NULL` | Melonggarkan `NOT NULL` nanti adalah satu `ALTER`; mengetatkannya berarti membersihkan baris yatim lebih dulu. Task pribadi tanpa lead belum ada di core flow. |
| **B9** | Konversi otomatis saat `won`? | **Tidak** — aksi eksplisit | Sudah ditetapkan freeze 2.4 aturan 5. `won` berarti kesepakatan tercapai; konversi berarti relasi pelanggan dibuat. Keduanya kejadian berbeda dan tidak selalu terjadi bersamaan. |

---

## Di luar cakupan

| Tidak dikerjakan | Ke |
|---|---|
| Dashboard UI apapun | Phase 3 |
| API Key, `POST /v1/leads` publik, dokumentasi integrator | Phase 4 |
| Mobile app, push notification, `device_tokens` | Phase 5 |
| Embedded form, `forms`, `source_form_id` | Phase 6 |
| Webhook masuk & keluar | Phase 7 |
| Penegakan limit / usage / quota | Phase 8 |
| Deal, Pipeline sebagai entity, automation, reports lanjutan | Phase 9 |
| Dedup bisnis (`possible_duplicate_of`) | Belum dijadwalkan — lihat catatan di bawah |
| Attribution channel (Website, Facebook, Referral, …) | Belum dijadwalkan — berbeda dimensi dari `source` |
| Round robin / assignment rule otomatis | Setelah assignment manual terbukti dipakai |
| Custom field, Contact, Team | Saat diminta |
| Import/export CSV | **Naikkan prioritas bila calon pelanggan datang dari spreadsheet** |

### Tiga hal yang sengaja tertahan

**Dedup bisnis.** `architecture_product_review.md` §4 menganjurkan menandai lead dengan telepon/email sama dalam 30 hari sebagai `possible_duplicate_of`. Yang dikerjakan Phase 2 hanya **idempotency teknis** (`Idempotency-Key`) — retry yang menghasilkan lead ganda. Keduanya sering dikira satu hal; keduanya berbeda. Dedup bisnis butuh keputusan produk yang belum diambil (ambang waktu, apakah ditampilkan sebagai peringatan atau blokir), dan menebaknya sekarang berarti membangun perilaku yang mungkin dibatalkan.

**Attribution channel.** `source` menjawab *"lewat pintu mana lead masuk"* (`manual` · `api` · `form` · `webhook`), bukan *"dari kampanye mana"*. Menggabungkan keduanya ke satu enum mencampur dua dimensi dan membuat laporan mustahil dibaca (freeze 2.3).

**`source_api_key_id` dan `source_form_id`.** `leads` dibuat **tanpa** kedua kolom itu; keduanya menyusul bersama tabel tujuannya di `0005` dan `0007` (freeze 8.4). Nilai enum `api` dan `form` sudah diterima `leads.source` sejak `0003` — nilai enum boleh mendahului FK-nya, dan menambah kolom nullable ke tabel yang sudah ada bersifat instan.

---

## Dependensi

Phase 1 selesai — tenancy, `tenant.Context`, pola repository tenant-scoped, `Store`/`Repos`, `authn`, `authz`, audit log, dan harness isolasi tenant semuanya sudah ada.

**Satu kewajiban diwarisi dari Phase 1** ([`01-auth-organization/td.md`](../01-auth-organization/td.md) §17): penonaktifan membership harus menolak berjalan bila masih ada lead terbuka. Di Phase 1 aturan itu belum bisa ditegakkan karena `leads` belum ada — Phase 2 wajib menutupnya, dan ia masuk sebagai acceptance criterion #13, bukan sebagai catatan.

**Tidak ada keputusan yang memblokir.** B6–B9 ditutup di atas.

---

## Pembagian issue

Freeze memetakan Phase 2 sebagai **empat** session. Setelah melihat isinya — 5 tabel, dua migration, ~15 endpoint, alokasi nomor di bawah konkurensi, optimistic locking, idempotency, dan aturan visibilitas employee yang ditegakkan di repository — saya pecah menjadi **lima**, dengan memisahkan lapisan schema/konkurensi dari lapisan HTTP:

| Urut | Issue | Kenapa dipisah |
|---|---|---|
| 1 | Schema `0003` + repository lead + alokasi `lead_number` + optimistic locking | Dua mekanisme paling halus di seluruh phase ini — serialisasi nomor per organization dan deteksi konflik tulis — dan keduanya hanya bisa dipercaya bila diuji **di bawah konkurensi nyata**. Menumpuk HTTP di atasnya sebelum keduanya terbukti berarti mereview keduanya sekaligus. |
| 2 | Lead CRUD, transisi status, filter, pagination, idempotency, E.164 | Lapisan HTTP + aturan bisnis lead di atas fondasi yang sudah terbukti |
| 3 | Activity (append-only + auto-log) + Task | Timeline dan tindak lanjut; keduanya bergantung pada peristiwa lead dari #2 |
| 4 | Assignment + Notification (`0004`) + **penutupan kewajiban Phase 1** | Assignment memicu activity (#3) **dan** notification sekaligus; kewajiban "tolak nonaktifkan bila ada lead terbuka" jatuh di sini karena baru di sini semua bahannya ada |
| 5 | Customer + konversi + kasus `lead` pada harness isolasi | Konversi butuh lead di status `won` (#2) dan menghasilkan activity (#3). Harness ditutup terakhir supaya mencakup seluruh endpoint phase ini. |

Ini **penyimpangan dari peta session di freeze**, dicatat di sini agar terlihat — pola yang sama dipakai Phase 1 (3 → 4). Bila Anda lebih memilih empat PR, katakan saat review: #1 dan #2 tinggal digabung.

Rincian: [`issues.md`](./issues.md)

---

## Bukan tujuan phase ini

- **Bukan** membangun UI apapun — seluruh verifikasi lewat test dan `curl`
- **Bukan** membekukan bentuk API publik — itu Phase 4, dan justru phase inilah yang memvalidasi bentuknya lebih dulu lewat pemakaian internal
- **Bukan** mengoptimasi query — index dipasang mengikuti Aturan #16, tapi belum ada volume untuk diukur
- **Bukan** menyiapkan struktur untuk Deal, Form, atau Webhook yang "sudah pasti datang" (Aturan #27, #28)
