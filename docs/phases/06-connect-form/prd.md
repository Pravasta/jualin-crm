# Phase 6 — Connect & Embedded Form · PRD

> **Apa & kenapa.** Detail teknis di [`td.md`](./td.md).
> Sumber: [`architecture/freeze.md`](../../architecture/freeze.md) bagian 1.2 (`forms`), 3.4 (Embedded Form → Phase 6), 5.1 (Aturan #23), 8.4 (migration `0007`) · [ADR-005](../../decisions/ADR-005-public-form-key.md) (kenapa `public_key` **bukan** API key, dan seluruh proteksi anti-spam) · [ADR-012](../../decisions/ADR-012-connect-surface-and-subscription-gating.md) (Connect sebagai permukaan produk) · [ADR-004](../../decisions/ADR-004-api-key-format.md) (format kredensial yang **tidak** ditiru di sini)

---

## Tujuan

**Capture layer bisa dipakai tanpa developer.**

Phase 4 membuktikan lead bisa masuk sendiri dari luar — tetapi hanya bila pelanggan punya orang yang
bisa menulis `POST /v1/leads` dengan header `Authorization`. Untuk pemilik toko yang websitenya dibuat
sekali oleh vendor tiga tahun lalu, itu sama saja dengan tidak bisa.

Phase 6 menutup jarak itu: **salin satu potong HTML, tempel di halaman, selesai.**

Sekaligus phase ini memberi capture layer **rumah** di dalam produk. Hari ini API key hidup sebagai
sub-halaman `Pengaturan → API Keys`, ditemukan hanya oleh orang yang sudah tahu ia ada. ADR-012
menaikkannya jadi menu `Connect` — satu tempat yang menjawab pertanyaan yang sebenarnya diajukan
pelanggan: *"bagaimana cara menyambungkan website saya?"*, bukan *"di mana API key saya?"*.

> Ini phase kedua yang menghadapi **klien yang bukan kita**, dan yang pertama menghadapi **browser orang
> asing**. Bedanya tajam: integrator Phase 4 membaca dokumentasi dan memegang kredensial rahasia;
> pengunjung Phase 6 tidak membaca apa pun, dan kredensialnya **sengaja terbuka untuk semua orang**.
> Itulah kenapa ADR-005 memisahkannya dari API key sejak Agustus, jauh sebelum ada kode.

---

## Kebutuhan

| # | Sebagai… | Saya butuh… | Supaya… |
|---|---|---|---|
| 1 | Pemilik toko | Memasang form penangkap lead di website saya **tanpa developer** | Website saya dibuat vendor tiga tahun lalu; menyewa orang untuk satu form tidak masuk akal |
| 2 | Owner | Satu tempat yang menjawab *"bagaimana menyambungkan sistem saya"* | Saya tidak tahu bedanya "API key" dan "webhook" — saya cuma ingin lead masuk |
| 3 | Owner | Form saya hanya bekerja di domain saya sendiri | Supaya orang lain tidak bisa menyalin formnya dan mengirimi saya sampah atas nama saya |
| 4 | Owner | Mengganti label field mengikuti bahasa bisnis saya | "Product" tidak berarti apa-apa bagi pelanggan salon; "Layanan yang diminati" berarti |
| 5 | Owner | Mematikan satu form tanpa mengganggu form atau integrasi lain | Kampanye selesai; formnya tidak perlu hidup selamanya |
| 6 | Pengunjung website pelanggan | Mengisi form tanpa puzzle yang menyebalkan | Saya cuma ingin bertanya harga, bukan memilih gambar lampu lalu lintas |
| 7 | Pemilik sistem | Spam tidak pernah masuk sebagai lead | Di model harga terjangkau, **setiap lead spam adalah biaya yang ditanggung Jualin** — storage, compute, dan reputasi |
| 8 | Pemilik sistem | Kredensial form yang jelas-jelas terekspos **tidak bisa membaca apa pun** | Form hidup di browser siapa saja; kebocorannya otomatis pada setiap pemasangan, bukan risiko probabilistik |

---

## Acceptance Criteria

Phase 6 selesai bila **semuanya** terpenuhi:

| # | Kriteria |
|---|---|
| 1 | Form ditempel di halaman HTML statis **di luar jaringan kita** → diisi → lead muncul di dashboard dengan `source = form` dan penanda form mana yang mengirimnya |
| 2 | `public_key` yang diambil dari DevTools **tidak bisa** membaca lead, mengubah data, atau memanggil endpoint lain — bukan karena setiap handler mengeceknya, melainkan karena otorisasi memang tidak punya jalan untuk mengizinkannya (Aturan #23, ADR-005) |
| 3 | Submit dari domain di luar allowlist form ditolak |
| 4 | Honeypot terisi → submission dibuang **diam-diam**, dengan respons yang **tidak bisa dibedakan** dari sukses. Bot tidak boleh bisa belajar |
| 5 | Submit lebih cepat dari 2 detik setelah form dirender ditolak |
| 6 | Payload melebihi 32KB ditolak `413` |
| 7 | Rate limit aktif **per IP dan per form**; setiap response membawa `X-RateLimit-*`, `429` membawa `Retry-After` |
| 8 | `CAPTCHA_PROVIDER=none` cukup untuk mengembangkan seluruh phase; `none` **ditolak saat boot** ketika `APP_ENV=production` (Aturan #36) |
| 9 | Owner membuat form, mengubah label field, mengatur allowlist, dan menonaktifkan form **dari dashboard**, tanpa `curl` |
| 10 | Snippet embed bisa **disalin dan langsung bekerja** — diverifikasi dengan menempelkannya ke halaman kosong, bukan dengan membacanya |
| 11 | `Connect` menjadi menu; pengelolaan API key pindah ke sana; **tautan lama `/settings/api-keys` tetap hidup** (redirect, bukan 404) |
| 12 | Halaman embed mengirim `Content-Security-Policy` dengan `frame-ancestors` sesuai allowlist **form itu**, bukan daftar global |
| 13 | Field yang tidak dikenal tetap tersimpan apa adanya di `raw_payload` — sama seperti jalur API |
| 14 | Harness isolasi tenant bertambah kasus untuk kredensial baru, dan **tetap terbukti bisa gagal** |

---

## Keputusan yang ditutup di phase ini

Tujuh keputusan yang jatuh tempo di sini, ditutup di muka daripada di tengah implementasi. Alasan
teknis lengkap di [`td.md`](./td.md); yang di bawah adalah **apa** dan **kenapa** dalam kalimat produk.

| # | Pertanyaan | Keputusan | Alasan |
|---|---|---|---|
| **D1** | Halaman form yang di-iframe disajikan dari mana? | **Backend Go**, `GET /embed/{public_key}`, memakai `html/template` + `embed.FS`. | Lihat catatan *"Penyimpangan tertulis dari ADR-005"* di bawah — ini satu-satunya keputusan phase ini yang menyimpang dari ADR yang sudah Accepted, dan karena itu ditulis terpisah. |
| **D2** | CAPTCHA | `CAPTCHA_PROVIDER=none\|turnstile`. `none` = lewati verifikasi, ditolak saat `APP_ENV=production`. | Preseden persis `MAIL_PROVIDER` (Phase 4.6) dan `PUSH_PROVIDER` (Phase 5): satu env var membuat seluruh phase bisa dikerjakan tanpa akun pihak ketiga, sementara produksi tetap tidak bisa berjalan setengah siap. Kunci Turnstile punya lead time — ia tidak boleh memblokir empat dari lima issue. |
| **D3** | Format `public_key` | `pk_` + 22 karakter base64url acak. Disimpan **plaintext**, tidak di-hash. | ADR-005 sudah menetapkan ia bukan rahasia — meng-hash-nya akan menyiratkan sebaliknya kepada siapa pun yang membaca skema, dan lookup-nya butuh nilai mentah. Prefix `pk_` sengaja **tidak** menyerupai `jln_live_`: dua kredensial dengan aturan yang berlawanan tidak boleh terlihat mirip, baik oleh manusia yang menyalinnya maupun oleh parser yang memilahnya. |
| **D4** | Angka rate limit submit | Per IP **dan** per form, keduanya lewat env, default konservatif. | Dua sumbu berbeda: per-IP menghalangi satu bot; per-form menghalangi botnet yang menyerang satu pelanggan. Satu saja tidak cukup. Angkanya **bukan hasil pengukuran** — sama jujurnya dengan `PUBLIC_API_RATE_LIMIT=60`, dan header `X-RateLimit-*` ada supaya perubahannya tidak mengejutkan. |
| **D5** | Bagaimana lead dibuat? | Memakai ulang `lead.Usecase.Create` apa adanya. `source` dipaksa `'form'` dan `source_form_id` diambil **dari principal**, tidak pernah dari body. | Persis pola jalur API key Phase 4. Menulis jalur pembuatan lead kedua berarti dua tempat yang harus tetap sama selamanya — termasuk penomoran lead, `raw_payload`, dan notifikasi assignment. |
| **D6** | Siapa yang melihat menu `Connect`? | **Semua role**; gerbangnya di dalam layar, bukan di sidebar. | Mengikuti `/settings` hari ini. Nav sadar-role **belum pernah ada** di dashboard ini — menciptakannya untuk satu menu berarti menambah mekanisme baru demi kerapian yang tidak diminta siapa pun (Aturan #27). |
| **D7** | Time-trap 2 detik | Token HMAC **stateless** yang ditanam di halaman embed, memuat id form + waktu terbit; divalidasi umurnya (>2 detik, <30 menit). | Tanpa token, waktu render hanya bisa dititipkan ke field tersembunyi — yang bisa dipalsukan bot dengan satu baris. HMAC membuatnya tidak bisa dikarang tanpa menyimpan state apa pun. **Ini time-trap, bukan anti-replay**: token yang sah masih bisa dipakai berulang dalam jendela 30 menit. Perlindungan terhadap pengulangan datang dari rate limit, bukan dari sini. |
| **D8** | Tinggi iframe | iframe **tetap** wadah form; ditambah **script pendamping opsional** (`embed.js`) yang menyesuaikan tinggi lewat `postMessage`. Tanpa script, iframe tetap bekerja dengan tinggi tetap. | `height` mati punya dua akibat yang pasti: form pendek meninggalkan ruang kosong, form panjang terpotong atau ber-scrollbar sendiri. Lihat catatan *"Kenapa script pendamping tidak membatalkan ADR-005"* di bawah — ini bukan kembali ke *inline script* yang ADR-005 tolak. |

### Penyimpangan tertulis dari ADR-005 — D1

ADR-005 menulis halaman form *"disajikan dari **domain terpisah** (`cdn.…`)"*. Kalimat itu menyiratkan
host statis atau CDN. Phase 6 **tidak** melakukannya, dan alasannya perlu berdiri terbuka:

| | |
|---|---|
| **Maksud ADR tetap dipenuhi** | Yang sebenarnya dilindungi ADR-005 adalah **isolasi origin**, CSP ketat, dan `frame-ancestors` yang mengikuti allowlist per-form. Ketiganya ditegakkan handler Go, dan `frame-ancestors` per-form justru **lebih tepat** dari host statis yang hanya bisa mengirim satu kebijakan untuk semua form. |
| **Domain terpisah tetap bisa** | `cdn.jualin.id` diarahkan ke service yang sama saat deploy. Domain terpisah adalah keputusan **DNS**, bukan keputusan aplikasi — dan memisahkan aplikasinya sekarang tidak membuat domainnya lebih terpisah. |
| **Belum ada deployment sama sekali** | Menambah aplikasi keempat berarti menambah deployable dan workflow CI untuk sesuatu yang, hari ini, belum punya satu pun tempat berjalan. Aturan #27. |

**Kewajiban yang ikut lahir dari keputusan ini** — dicatat supaya tidak hilang: saat deployment
akhirnya dikerjakan, halaman embed **wajib** disajikan dari hostname yang berbeda dari dashboard.
Menyajikannya dari origin yang sama dengan dashboard membatalkan isolasi yang ADR-005 lindungi.

### Kenapa script pendamping tidak membatalkan ADR-005 — D8

ADR-005 menolak *inline script* dengan satu alasan spesifik: **"script host bisa membaca input"**.
Yang membuat itu benar adalah **di mana form-nya hidup** — pada inline script, form disuntikkan ke DOM
halaman pelanggan, sehingga script lain di halaman itu (termasuk plugin pihak ketiga yang tidak
dikendalikan siapa pun) bisa membaca apa yang diketik pengunjung.

D8 **tidak** memindahkan form ke DOM host:

| | Inline script (ditolak ADR-005) | D8 — iframe + `embed.js` |
|---|---|---|
| Form hidup di | DOM halaman host | **Di dalam iframe**, origin terpisah |
| Halaman host bisa membaca input? | ✅ Ya | ❌ **Tidak** — dihalangi browser, bukan oleh janji kita |
| Yang dikerjakan script host | Merender & mengelola form | **Hanya menerima satu angka: tinggi** |
| Tanpa script | Tidak ada form sama sekali | Form tetap jalan, tinggi tetap |

Isolasi lintas-origin adalah properti yang ditegakkan browser. Selama form berada di dalam iframe,
halaman host tidak punya cara membaca isinya — ada atau tidak ada `embed.js`.

**Syarat yang mengikat** (tanpa ini, D8 justru membuka permukaan serangan baru):

1. `postMessage` **tidak pernah** `targetOrigin: '*'` — selalu origin yang dituju secara eksplisit
2. Penerima **wajib** memverifikasi `event.origin`; pesan dari origin lain diabaikan
3. Tinggi dibatasi rentang wajar — pesan jahat tidak boleh bisa membuat iframe raksasa yang menutupi
   halaman pelanggan (*clickjacking* terbalik)
4. `embed.js` **hanya** mengenal satu jenis pesan. Ia tidak menerima perintah lain, tidak mengevaluasi
   apa pun yang dikirim iframe

---

## Di luar cakupan

| Tidak dikerjakan | Ke |
|---|---|
| Webhook masuk & keluar | Phase 7 — kanal ketiga di `Connect`, mekanisme yang sama sekali berbeda |
| Penegakan paket, keadaan "terkunci", `usage_counters` | Phase 8 — ADR-012 §4 sengaja menunda angkanya sampai ada data pengguna nyata |
| Landing page & alur *landing → register → dashboard* | Belum terjadwal — ADR-012 §Batas |
| Form builder dinamis (field yang bisa ditambah pelanggan) | ADR-005: field tetap dengan label yang bisa diubah. Builder butuh schema field + mesin validasi + renderer + strategi penyimpanan — fitur besar tersendiri, dan tidak diperlukan untuk membuktikan capture layer bekerja |
| Tabel `form_submissions` | **Tidak pernah.** Freeze 1.3 menolaknya: submission yang valid **adalah** Lead + `raw_payload`. Ia baru dipertimbangkan bila kita perlu menyimpan submission yang **ditolak** |
| Embed lewat inline script (menyatu dengan CSS situs) | ADR-005 memilih iframe untuk MVP; styling terbatas adalah harga yang wajar untuk isolasi. Inline script bisa ditawarkan nanti sebagai opsi lanjutan |
| Staging / deployment untuk QA | Terpisah dari phase manapun — dan tetap jadi prasyarat sebelum Tim QA bisa bekerja |

---

## Dependensi

Phase 1–5 selesai. Yang dipakai phase ini sudah ada seluruhnya:

| Yang dipakai | Dari | Catatan |
|---|---|---|
| `tenant.PrincipalPublicForm` | Phase 1 | **Sudah ada, nol pemakaian** — konstanta ini ditulis di `tenant.go` sejak Phase 1 dan tidak pernah disentuh siapa pun. Phase 6 adalah momen itu, persis seperti `APIKeyID` yang menunggu Phase 4 |
| `authz.Require` yang bercabang pada `PrincipalType` | Phase 4 | Phase 6 menambah cabang **ketiga**, bukan menyisipkan baris ke peta API key. Dua gerbang yang sudah ada tidak berubah |
| `apikey.Repository.FindByKeyID` — pengecualian tertulis tanpa `tenant.Context` | Phase 4 | `FindByPublicKey` bentuknya identik dengan alasan yang sama (Aturan #5): organization adalah **hasil** lookup, bukan input |
| `leads.source` menerima `'form'`, `raw_payload jsonb` | Phase 2 | Dibuat di `0003` **sebelum** ada yang memakainya, persis untuk phase ini — tidak ada `ALTER` pada enum |
| `lead.Usecase.Create` | Phase 2 | Dipakai ulang apa adanya (D5) — termasuk penomoran lead dan notifikasi assignment |
| `ratelimit.FixedWindow.Take` + `httpx.SetRateLimitHeaders` | Phase 1, 4, 4.5 | Sudah lengkap dengan `Result{Limit, Remaining, ResetAt}` dan eviction dua generasi. Phase 6 hanya menambah key baru |
| `crm_dashboard` — sesi, klien API, app shell, `canManageAPIKeys` | Phase 3, 4 | Layar baru menempel pada kerangka yang sudah ada; `canManageForms` lahir di sebelahnya |

**Satu kewajiban diwarisi ADR-012**: memindahkan pengelolaan API key ke `/connect/api` **tanpa
mematikan tautan lama**. Pelanggan Phase 4 yang sudah menyimpan `/settings/api-keys` tidak boleh
menemukan 404 karena kita merapikan menu.
