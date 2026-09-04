# Phase 8 — Subscription · PRD

> **Apa & kenapa.** Detail teknis di [`td.md`](./td.md).
> Sumber: [ADR-012](../../decisions/ADR-012-connect-surface-and-subscription-gating.md) §2–§4 (subscription sebagai gerbang kanal, dan apa yang sengaja **tidak** diputuskan di sana) · [`architecture/freeze.md`](../../architecture/freeze.md) bagian 8.4 (`plans`/`subscriptions`/`usage_counters`), amandemen **S1** (`subscriptions` minimal di Phase 1) · [`product/decisions.md`](../../product/decisions.md) §16 (Free Plan sejak registrasi), §24, §25 (pricing & limit sengaja ditunda) · TD Phase 4 §19 (`usage_counters` menghitung, `ratelimit` melindungi)

---

## Tujuan

**Batas paket menjadi nyata — mekanismenya, bukan angkanya.**

`subscriptions` sudah ada sejak `0002_identity.sql` (Phase 1, amandemen S1): setiap organization
mendapat baris `plan_code = 'free'` sejak detik registrasi. Sejak itu **tidak ada satu pun query yang
pernah membacanya kembali.** Kolomnya hidup; pembacanya tidak pernah lahir.

Sementara itu Phase 4, 6, dan 7 membangun tiga kanal Connect — API key, Formulir, Webhook — dan ADR-012
sudah memutuskan bahwa **subscription-lah yang menggerbangi ketiganya**. Kartu di `/connect` hari ini
ketiganya aktif tanpa syarat, karena gerbangnya memang belum ada.

Phase 8 menutup jarak itu: **`plan_code` mendapat pembaca pertamanya, dan gerbangnya ditegakkan di
usecase.**

> **Phase ini sengaja tidak menetapkan angka apa pun.** Tidak ada harga, tidak ada limit free tier,
> tidak ada peta "kanal X hanya untuk paket Y" yang mengunci sesuatu hari ini. ADR-012 §4 mengikat
> angka-angka itu ke *"setelah gate freeze (3–5 pengguna nyata) memberi data yang membuat angka itu
> bisa dipilih dengan jujur"* — dan gate itu belum terlewati. Yang ADR-012 nyatakan bisa dikerjakan
> sekarang adalah kalimatnya sendiri: ***"ADR ini menetapkan mekanismenya, bukan angkanya."***

### Kenapa mekanismenya dibangun sebelum angkanya

Bukan untuk mendahului keputusan produk, melainkan karena keduanya **berbeda jenis pekerjaan**:

| | Mekanisme (phase ini) | Angka (menyusul) |
|---|---|---|
| Bentuk | Kode: pembaca `plan_code`, peta kapabilitas, gerbang di usecase, keadaan "terkunci" di UI | Data: satu peta diisi ulang |
| Butuh pengguna nyata? | **Tidak** — bentuknya sudah ditetapkan ADR-012 | **Ya** — itu yang gate freeze minta |
| Biaya menundanya | Tiga kanal terus bertambah tanpa tempat untuk digerbangi; makin lama makin banyak titik yang harus disisipi belakangan | Nol — mengisi peta yang sudah ada |
| Biaya salah | Rendah, dan terlihat: gerbang yang salah menolak akan langsung ketahuan | **Tinggi, dan tidak terlihat**: angka tebakan bertahan karena terlanjur tertulis (ADR-012 menamai risiko ini sendiri) |

---

## Kebutuhan

| # | Sebagai… | Saya butuh… | Supaya… |
|---|---|---|---|
| 1 | Pemilik sistem | Kanal yang tidak termasuk paket sebuah organization **ditolak di server**, bukan sekadar disembunyikan tombolnya | Menyembunyikan kartu adalah kenyamanan; siapa pun yang bisa memanggil `curl` bisa melewatinya. ADR-012 §3 menyatakan ini eksplisit |
| 2 | Owner | **Melihat** kanal yang belum saya punya, bukan tidak tahu ia ada | Pelanggan yang tidak pernah melihat bahwa Webhook ada tidak akan pernah meng-upgrade untuk mendapatkannya (ADR-012, *Alasan*) |
| 3 | Owner | Tahu paket apa yang sedang saya pakai | Hari ini tidak ada satu pun layar yang bisa menjawabnya — `plan_code` tidak pernah keluar dari database |
| 4 | Pemilik sistem | Peta "paket apa membuka kanal apa" hidup di **satu tempat** | Peta yang tersebar akan menyimpang; dan bila dashboard menyalinnya ke TypeScript, ada dua sumber kebenaran untuk satu keputusan (kesalahan yang #33 harus koreksi pada `lib/lead-status.ts`) |
| 5 | Pemilik sistem | Membuka atau menutup satu kanal untuk satu paket **tanpa menyentuh usecase manapun** | Kalau mengubah paket berarti menyunting `apikey`, `form`, dan `webhook` satu per satu, angka yang menyusul akan mahal untuk dipasang |
| 6 | Pemilik produk | CRM ini **tidak pernah** tahu tentang uang | Batas ADR-012 §2 — harga, checkout, kartu, invoice, refund seluruhnya di payment service. CRM menyimpan `external_reference` dan membaca `plan_code`; tidak menghitung apa pun tentang uang |

---

## Acceptance Criteria

Phase 8 selesai bila **semuanya** terpenuhi:

| # | Kriteria |
|---|---|
| 1 | `plan_code` punya **pembaca nyata** — `GET /v1/me` mengembalikan paket aktif organization beserta kanal yang dibukanya |
| 2 | Peta paket → kapabilitas ada di **satu tempat**, dan menambah/mengurangi satu kanal untuk satu paket adalah perubahan **satu baris** di peta itu — bukan perubahan di usecase manapun |
| 3 | `POST /v1/api-keys`, `POST /v1/forms`, dan `POST /v1/webhook-endpoints` menolak dengan **`403 plan_upgrade_required`** bila paket organization tidak membuka kanal itu — **dibuktikan lewat `curl`**, bukan hanya lewat UI yang menyembunyikan tombol |
| 4 | Gerbang itu **terbukti bisa gagal**: membalik satu entri peta jadi `false` membuat endpoint yang tadinya `201` menjadi `403`, dan kartu Connect-nya menjadi terkunci — dibuktikan dengan menjalankannya, lalu dikembalikan |
| 5 | Kartu Connect punya keadaan **"terkunci oleh paket"** yang nyata dan **terlihat** — bukan disembunyikan, dan bukan lagi teks *"belum tersedia"* yang berbohong tentang alasannya |
| 6 | Dashboard **tidak memuat peta paket→kanal versinya sendiri** — ia merender apa yang backend kirim. Ditegakkan dengan tidak pernah mengirim `plan_code` mentah sebagai dasar keputusan UI |
| 7 | Layar Connect menangani `403 plan_upgrade_required` yang datang dari balapan (paket berubah antara render dan klik) tanpa menjadi tombol yang diam |
| 8 | Organization yang paketnya tidak dikenal peta **ditolak**, bukan diizinkan — gerbang gagal tertutup, bukan gagal terbuka |
| 9 | Tidak ada satu pun angka harga, limit, atau kuota di dalam kode maupun dokumen phase ini |
| 10 | `go test -race ./...` dan `npm run typecheck && lint && test && build` bersih |

---

## Keputusan yang ditutup di phase ini

Enam keputusan, ditutup di muka daripada di tengah implementasi.

| # | Pertanyaan | Keputusan | Alasan |
|---|---|---|---|
| **D1** | `usage_counters` dibangun sekarang? | **Tidak.** Phase ini hanya gerbang kapabilitas. | Lihat *Penyimpangan tertulis dari freeze* di bawah — satu-satunya keputusan phase ini yang menyimpang dari freeze, dan karena itu ditulis terpisah. |
| **D2** | Tabel `plans`? | **Tidak.** `plan_code` sebagai `text` tetap cukup. | ADR-012 §4 sudah memutuskan ini: *"Aturan #28 — belum ada implementasi kedua yang nyata"*. Hari ini hanya ada satu paket (`free`). Tabel untuk satu baris adalah abstraksi sebelum implementasi kedua. `freeze.md` 8.4 mendaftarkan `plans` untuk Phase 8; ADR-012 lebih baru dan lebih spesifik, dan ADR-lah yang diikuti (Aturan #30 — dilaporkan di sini, bukan diam-diam dipilih). |
| **D3** | Di mana peta paket→kapabilitas hidup? | **Peta Go di satu paket**, bentuk yang sama dengan `authz.apiKeyScopeFor`/`publicFormAllows`. Hari ini `free` membuka ketiga kanal. | Peta yang dibaca dengan **satu baris** adalah syarat kriteria #2. Bentuk ini sudah dipakai dua kali di codebase ini untuk pertanyaan yang sebentuk ("principal jenis ini boleh apa"), dan keduanya dikunci test yang mengulang seluruh himpunan — bukan daftar tulis tangan. |
| **D4** | Gerbang di **membuat** resource, atau juga di **memakai** yang sudah ada? | **Hanya saat membuat** (`POST`). Form, API key, dan endpoint webhook yang sudah ada tetap bekerja. | Menutup resource yang sudah jalan adalah perilaku **downgrade**, dan hari ini tidak ada jalur downgrade sama sekali — hanya `free` yang eksis. Membangun penegakan untuk transisi yang belum bisa terjadi adalah Aturan #29. Saat angka mendarat, pertanyaan "apa yang terjadi pada form yang sudah ada saat paket turun" dijawab bersama pertanyaan retensi — keduanya butuh keputusan produk yang sama. |
| **D5** | Dashboard menerima `plan_code`, atau kapabilitas yang sudah diselesaikan? | **Kapabilitas yang sudah diselesaikan.** `GET /v1/me` mengirim kanal mana yang terbuka, bukan kode paketnya sebagai dasar keputusan. | Kalau dashboard menerima `plan_code`, ia harus menyalin peta D3 ke TypeScript untuk tahu artinya — dua sumber kebenaran untuk satu keputusan. Persis kesalahan yang #33 harus koreksi: `lib/lead-status.ts` sempat memakai logika transisi versi mockup dan ditulis ulang sebagai port baris-demi-baris dari Go. Di sini biayanya bisa **dihindari sepenuhnya** dengan mengirim jawabannya, bukan bahannya. |
| **D6** | Kartu terkunci mengarahkan ke mana? | **Tidak ke mana-mana — belum.** Kartu menyatakan kanal itu tidak termasuk paket saat ini, tanpa tombol upgrade dan tanpa harga. | ADR-012 §2 menempatkan checkout di payment service, dan service itu **belum ada kontraknya** (`freeze.md`: *"Kontrak integrasi payment service — sebelum Phase 8"*, masih terbuka). Tombol "Upgrade" yang tidak menuju ke mana pun lebih buruk daripada tidak ada tombol. Menampilkan harga akan melanggar kriteria #9. Saat payment service punya kontrak, yang ditambahkan adalah satu tautan keluar — bukan perubahan bentuk kartunya. |

### Penyimpangan tertulis dari freeze — D1

`freeze.md` 8.4 mendaftarkan `usage_counters` sebagai tabel Phase 8, dan bagian 3 menempatkan
*"Penegakan limit, usage, upgrade, payment"* di phase ini.

Phase 8 **tidak** membuat tabel itu, dan alasannya perlu berdiri terbuka:

| | |
|---|---|
| **Tidak ada angka untuk ditegakkan** | Kuota adalah *"berapa banyak"*. Phase ini sengaja tidak menetapkan satu pun angka (kriteria #9, ADR-012 §4). Penghitung tanpa pembanding tidak menegakkan apa pun — ia hanya menulis baris. |
| **Aturan #28 dan #29 mengikat** | *"Abstraksi hanya setelah ada implementasi kedua yang nyata"*, *"jangan buat tabel untuk kebutuhan yang belum ada"*. `usage_counters` hari ini adalah tabel dengan nol pembaca dan jalur tulis di banyak usecase — schema mati yang menyentuh banyak tempat. |
| **Preseden yang sama persis sudah ada** | Phase 7 **D1**: `freeze.md` bagian 5 ketentuan #4 menyebut tabel `jobs`, dan Phase 7 menolak membuatnya karena baru ada **satu** konsumen async yang nyata. Alasannya identik, dan hasilnya dicatat sebagai keputusan phase — bukan diselipkan seolah freeze memang mengizinkannya. |
| **Maksud freeze tetap dipenuhi** | Yang bagian 3 lindungi adalah *"batas paket harus ditegakkan, bukan sekadar ditampilkan"*. Phase ini memang menegakkannya — di usecase, gagal tertutup, terbukti bisa gagal. Yang berbeda hanya **dimensi** yang digerbangi: kapabilitas (kanal apa), bukan kuota (berapa banyak). |

**Kewajiban yang ikut lahir dari keputusan ini** — dicatat supaya tidak hilang: `usage_counters`
dievaluasi ulang **bersamaan** dengan mendaratnya angka limit, bukan terpisah. Saat itu pertanyaannya
sudah punya jawaban yang jujur: metrik mana yang benar-benar dibatasi, dan pada angka berapa. Phase 8
tidak menutup pintu itu; ia hanya menolak membukanya sebelum ada yang lewat.

---

## Di luar cakupan

| Tidak dikerjakan | Ke |
|---|---|
| **Harga, limit free tier, peta kanal-per-paket yang mengunci sesuatu** | Setelah gate freeze — 3–5 pengguna nyata (ADR-012 §4). Ini bukan penundaan teknis; ini syarat yang ADR-nya sendiri tetapkan |
| **`usage_counters` dan penegakan kuota** | Bersama mendaratnya angka limit (D1) |
| **Tabel `plans`** | Saat ada paket kedua yang nyata (D2) |
| **Integrasi payment service** — checkout, kartu, invoice, refund, webhook pembayaran | **Selamanya di luar repository ini** (ADR-012 §2, `CLAUDE.md` *Di luar cakupan*). Yang akan ditambahkan kelak hanyalah satu tautan keluar dan pembacaan `external_reference` |
| **Alur upgrade & downgrade** | Butuh kontrak payment service, yang belum ada. D4 dan D6 keduanya berhenti di batas ini |
| **Retensi data free tier** | Keputusan produk yang belum diambil (`STATUS.md` *Keputusan Belum Diambil*); jatuh tempo bersama angka limit |
| **Dashboard analytics pemakaian** (grafik request per hari) | PRD Phase 4 sudah menempatkannya bersama usage counter — ikut D1 |
| **Menggerbangi resource yang sudah ada saat paket turun** | D4 — tidak ada jalur downgrade hari ini |

---

## Dependensi

Phase 1–7 selesai. Yang dipakai phase ini sudah ada seluruhnya:

| Yang dipakai | Dari | Catatan |
|---|---|---|
| Tabel `subscriptions` + baris `free` per organization | Phase 1 (`0002`, amandemen S1) | **Tidak ada migration baru di phase ini** — kolom yang dibutuhkan (`plan_code`, `status`) sudah ada sejak awal. S1 menyebut biayanya "mendekati nol"; phase ini yang menagih hasilnya |
| `subscription.Repository.CreateFree` | Phase 1 | Satu-satunya method yang pernah ada; phase ini menambah pembacanya |
| Pola interface consumer-declared (`ActivityRecorder`, `LeadCreator`, `WebhookEnqueuer`) | Phase 2, 6, 7 | `apikey`/`form`/`webhook` memanggil gerbang lewat interface yang **mereka sendiri** deklarasikan — tidak satu pun mengimpor `internal/subscription` (ADR-011) |
| Pola peta kapabilitas satu-tempat (`authz.apiKeyScopeFor`, `publicFormAllows`) | Phase 4, 6 | D3 memakai bentuk yang sama, termasuk test yang mengulang seluruh himpunan alih-alih daftar tulis tangan |
| Kartu Connect + `canManageAPIKeys`/`canManageForms`/`canManageWebhooks` | Phase 6, 7 | Keadaan "terkunci" menempel pada kerangka yang sudah ada; gerbang **paket** berdiri di samping gerbang **role**, bukan menggantikannya |
| `GET /v1/me` + `SessionGate` | Phase 1, 3 | Kapabilitas ikut di response yang dashboard **sudah** panggil di setiap layar terproteksi — tanpa request tambahan (D5) |

**Dua poin terbuka yang wajib dibaca sebelum TD final:**

- [`docs/issues/047-public-lead-api.md`](../../issues/047-public-lead-api.md) — retensi dan angka rate
  limit yang belum pernah diukur. Phase ini **tidak** menambah angka ke daftar itu (kriteria #9), tapi
  peninjauan bersama yang dijanjikan `api.md` akan jatuh tempo pada momen yang sama dengan angka limit
  Phase 8.
- `freeze.md` — *"Kontrak integrasi payment service: **sebelum** Phase 8"*. Kontrak itu **belum ada**,
  dan phase ini berjalan tanpanya secara sadar: D6 berhenti tepat di batas yang membutuhkannya. Yang
  tidak boleh terjadi adalah menebak bentuk kontraknya lalu membangun ke arah tebakan itu.
