# Design Brief — Jualin CRM Owner Dashboard

> **Dokumen ini untuk desainer (Claude Design), bukan untuk implementor.**
>
> Sumbernya: [`prd.md`](./prd.md) (apa & kenapa) dan [`td.md`](./td.md) (bagaimana). Bila brief ini
> bertentangan dengan keduanya, **prd.md dan td.md yang menang** — laporkan pertentangannya, jangan
> diam-diam memilih salah satu (Aturan #30).
>
> Hasil desain akan diimplementasikan di `crm_dashboard/` (Next.js App Router + Tailwind v4 +
> shadcn/ui). Fondasi teknis, klien API, dan lima layar auth **sudah jadi** (issue #31) — desain
> untuk layar auth berarti **memperbaiki yang sudah ada**, bukan mendesain dari nol.

---

## 1. Yang diminta

Desain antarmuka untuk **±15 layar** dashboard CRM yang dipakai setiap hari oleh pemilik usaha kecil-menengah Indonesia dan timnya.

Prioritas usaha **tidak dibagi rata**. Freeze arsitektur menyatakan: *"Lead list & detail adalah produknya. Layar itu dibuka ratusan kali sehari; sisanya sesekali."* Dua layar itu layak mendapat porsi terbesar perhatian desain; layar admin dan settings cukup rapi dan jelas.

---

## 2. Produk dalam satu paragraf

Jualin CRM mencatat **lead** (seseorang menunjukkan minat), menugaskannya ke anggota tim, merekam apa yang terjadi padanya, dan mengubah yang berhasil menjadi **customer**. Alurnya: **Capture → Manage → Assign → Follow-up → Customer**. Produk SaaS multi-tenant berharga terjangkau — satu akun bisnis = satu *organization*, dan biaya infrastruktur per tenant harus tetap rendah.

**Bukan** produk ini: HRIS, akuntansi, inventory, payroll, invoice, email campaign, chat inbox WhatsApp/IG/FB, payment gateway. Bila sebuah ide desain menyentuh domain itu, hentikan dan tandai sebagai *scope discussion*.

---

## 3. Pengguna & konteks pemakaian

| Peran | Yang dilakukan di dashboard | Frekuensi |
|---|---|---|
| **Owner** | Semuanya. Pemilik usaha, sering bukan orang teknis. | Harian |
| **Admin** | Hampir semuanya kecuali beberapa aksi kepemilikan | Harian |
| **Manager** | Kelola lead & tim-nya; tidak bisa hapus lead, konversi, atau ubah customer | Harian |
| **Employee** | **Tidak memakai dashboard ini** — mereka dapat aplikasi mobile di Phase 5 | — |

**Konteks nyata:** dibuka di laptop di toko/kantor kecil, kadang di tablet. Koneksi tidak selalu cepat. Pengguna membuka daftar lead puluhan kali sehari untuk menjawab satu pertanyaan: *"mana yang belum ditangani?"*

**Desktop-first**, tetapi layar tidak boleh rusak di tablet. Mobile bukan target dashboard ini.

---

## 4. Yang sudah dikunci — jangan didesain ulang

Ini keputusan yang sudah diambil dan diimplementasikan. Mengubahnya berarti membongkar kode yang sudah jalan.

| Hal | Keputusan | Konsekuensi untuk desain |
|---|---|---|
| **Bahasa** | **Bahasa Indonesia saja**, tanpa library i18n | Tidak ada language switcher. Tidak ada teks Inggris di antarmuka — termasuk label tombol, header tabel, dan *empty state*. Nama field API tetap Inggris, tetapi itu tidak pernah terlihat pengguna. |
| **Design system** | **shadcn/ui** (style `base-nova`, base color `neutral`) + **Tailwind v4** | Desain harus bisa dibangun dari komponen shadcn/ui. Token warna sudah ada dalam format `oklch` (lihat §5). Primitive-nya `@base-ui/react`, ikon **lucide**. |
| **Radius** | `--radius: 0.625rem` | Bahasa bentuk sudah ditentukan; jangan usulkan sudut tajam atau pil penuh sebagai gaya global. |
| **Tema** | **Terang saja.** Dark mode di luar cakupan | Tidak perlu desain varian gelap. Token gelap ada di kode tetapi tidak dipakai. |
| **Font** | Geist Sans + Geist Mono — **lihat catatan di bawah** | Boleh diusulkan diganti, tetapi sertakan alasan — bukan preferensi. |
| **Navigasi antar-organization** | Pengguna memilih organization **saat login**, bukan lewat switcher di dalam aplikasi | Tidak ada dropdown "pindah workspace" di header. |

### 4.1 Token warna yang sudah terpasang

```
--background      oklch(1 0 0)            --foreground        oklch(0.145 0 0)
--card            oklch(1 0 0)            --card-foreground   oklch(0.145 0 0)
--primary         oklch(0.205 0 0)        --primary-foreground oklch(0.985 0 0)
--secondary       oklch(0.97 0 0)         --muted-foreground  oklch(0.556 0 0)
--muted           oklch(0.97 0 0)         --accent            oklch(0.97 0 0)
--destructive     oklch(0.577 0.245 27.325)
--border          oklch(0.922 0 0)        --input             oklch(0.922 0 0)
--ring            oklch(0.708 0 0)        --radius            0.625rem
```

Palet saat ini **netral abu-abu murni tanpa warna aksen** — itu default shadcn, bukan keputusan desain. **Menentukan warna aksen/brand adalah bagian dari pekerjaan ini.** Bila mengusulkan aksen, berikan nilai `oklch` untuk `--primary` dan `--ring`, dan pastikan kontrasnya lolos WCAG AA terhadap `--primary-foreground`.

### 4.2 Catatan font — ada yang belum benar

Geist Sans dan Geist Mono **dimuat** di root layout, tetapi Geist Sans **belum benar-benar diterapkan**: `globals.css` memetakan `--font-sans: var(--font-sans)` (menunjuk ke dirinya sendiri) sementara layout mendefinisikan `--font-geist-sans`. Akibatnya `font-sans` jatuh ke font bawaan browser. `--font-mono` sudah benar.

Ini bug satu baris dari hasil scaffold, sudah dilaporkan dan akan diperbaiki terpisah — **bukan** hal yang perlu diselesaikan lewat desain. Disebut di sini supaya tipografi tidak dinilai dari tampilan yang sekarang.

---

## 5. Prinsip desain untuk produk ini

1. **Alat kerja, bukan etalase.** Kepadatan informasi lebih berharga daripada ruang kosong yang lapang. Owner memindai puluhan baris lead, bukan mengagumi hero section.
2. **Jawab pertanyaan dalam hitungan detik.** Setiap layar utama punya satu pertanyaan yang harus terjawab tanpa scroll: daftar lead → *"mana yang belum ditangani?"*; detail lead → *"apa yang sudah terjadi pada orang ini?"*; home → *"bagaimana keadaan bisnis minggu ini?"*
3. **Yang berbahaya harus terasa berbeda.** Menonaktifkan anggota, menghapus lead, dan menimpa data orang lain bukan aksi rutin — perlakuannya harus berbeda dari tombol biasa (lihat §8).
4. **Ringan.** Sales membuka dashboard dari koneksi lambat. Hindari desain yang mewajibkan gambar besar, banyak font, atau animasi berat.
5. **Benar dulu, cantik kemudian.** PRD menyatakannya eksplisit: *"layar yang benar dan bisa dipakai lebih penting daripada layar yang cantik"*. Tetapi phase ini akan **didemokan ke calon pelanggan** — jadi ia tetap harus terlihat seperti produk yang layak dibayar, bukan prototipe internal.

---

## 6. Kosakata wajib

Konsistensi penamaan dijaga [`docs/product/glossary.md`](../../product/glossary.md). Ini **bukan preferensi gaya** — istilah yang salah di layar menjanjikan fitur yang tidak ada.

| ⛔ Jangan pakai di UI | ✅ Pakai | Alasan |
|---|---|---|
| "Workspace" | **Organization** | Satu istilah saja di seluruh produk |
| "Team" / "Tim saya" | "Semua lead" / "seluruh organisasi" | Entity `Team` **belum ada** — UI tidak boleh menjanjikannya |
| "Karyawan" / "Staff" sebagai entity | "Anggota" (organization) | Employee = anggota dengan role `employee`, bukan tabel tersendiri |
| "Pipeline" sebagai papan Kanban | Status lead | Tidak ada entity pipeline sampai Phase 9 |
| "Deal" / "Nilai deal" / "Revenue" | — (jangan tampilkan) | Deal belum ada. Angka uang **tidak boleh** muncul di manapun. |

Istilah yang dipakai di layar: **Lead · Customer · Activity · Task · Anggota · Undangan · Organization · Notifikasi**.

Lead ditampilkan ke pengguna dengan **nomor urut per organization** (`#1024`), bukan UUID.

---

## 7. Inventaris layar

Dikelompokkan per issue implementasi. Endpoint API-nya **sudah ada semua** — desain tidak boleh mengandaikan data yang tidak tersedia.

### 7.1 Auth — sudah diimplementasikan, butuh perbaikan visual

Lima layar berdiri sendiri, lebarnya sempit (kartu di tengah). Sudah berfungsi; yang dibutuhkan adalah kualitas visual dan kejelasan.

| Layar | Isi | Catatan |
|---|---|---|
| **Masuk** | Email, password, tautan "Lupa password?" dan "Daftar organization baru" | Juga menampung **pemilihan organization** — lihat §8.2 |
| **Daftar** | Nama organization, nama lengkap, email, password (min. 12 karakter) | Sukses → layar "cek email", bukan langsung masuk |
| **Verifikasi email** | Otomatis memproses token dari tautan email. Tiga keadaan: sedang memverifikasi / berhasil / gagal + form kirim ulang | |
| **Lupa password** | Satu field email → layar "cek email" | Selalu menampilkan hasil yang sama, apakah email terdaftar atau tidak (mencegah penebakan akun) |
| **Atur ulang password** | Password baru + konfirmasi, token dari tautan email | |

### 7.2 Daftar lead — layar terpenting

Menjawab *"mana yang belum ditangani?"*

**Data per baris:** nomor lead (`#1024`), nama, status, pemilik (nama anggota), sumber, tanggal masuk. Kontak (email/telepon) berguna tetapi opsional bila ruang tidak cukup.

**Filter yang harus muat semua sekaligus dan bisa dikombinasikan:**
- Status (multi-pilih, 8 nilai)
- Sumber (multi-pilih, 4 nilai)
- Pemilik (pilih anggota) — **termasuk opsi "tanpa pemilik aktif"**
- Periode masuk (rentang tanggal)
- Kata kunci (nama, email, telepon)

> **Filter "lead tanpa pemilik aktif" wajib terlihat permanen** — bukan tersembunyi di balik menu "filter lanjutan". Ini jaring pengaman: saat seorang anggota dinonaktifkan, lead-nya bisa tetap tercatat atas namanya dan **tidak muncul di daftar "belum ter-assign" siapa pun**. Kalau filter ini sulit ditemukan, lead menghilang diam-diam. Beri ia perlakuan yang membuatnya ditemukan tanpa dicari — misalnya chip/tab yang selalu tampil dengan jumlahnya.

**Juga di layar ini:** tombol buat lead baru, pagination dengan **jumlah total**, dan indikator filter aktif (pengguna harus bisa melihat dan menghapus filter yang sedang berlaku — filter juga tercermin di URL agar bisa dibagikan).

**Dua keadaan kosong yang berbeda:** "belum ada lead sama sekali" (ajakan membuat lead pertama) vs "tidak ada yang cocok dengan filter" (ajakan menghapus filter). Menyamakan keduanya membuat pengguna baru mengira produknya rusak.

### 7.3 Detail lead — layar terpenting kedua

Menjawab *"apa yang sudah terjadi pada orang ini, dan apa berikutnya?"* Hampir seluruh aksi tulis produk ada di sini.

| Bagian | Isi |
|---|---|
| **Header** | `#1024`, nama, kontak, sumber, pemilik, status |
| **Aksi status** | Ubah status — hanya transisi yang sah yang ditawarkan (§9.1). Memilih `lost` **mewajibkan** alasan (6 pilihan) |
| **Assignment** | Tugaskan ke anggota, atau lepaskan |
| **Timeline** | Riwayat activity, **terbaru di atas**. 10 tipe, masing-masing perlu tampil terbaca manusia — `status_changed` harus terbaca "Status: Baru → Dihubungi", bukan JSON |
| **Tambah catatan** | Hanya tiga tipe yang boleh dibuat pengguna: catatan, log telepon, WhatsApp dibuka |
| **Task** | Daftar task pada lead ini: buat, ubah, tandai selesai, hapus. Punya judul, deskripsi, jatuh tempo, penanggung jawab |
| **Konversi** | Ubah jadi customer — **hanya ditawarkan saat status `won`** |
| **Hapus lead** | Aksi merusak, perlakuan berbeda |

Setiap aksi tulis harus memperbarui timeline tanpa reload manual.

### 7.4 Tim — anggota, undangan, penonaktifan, notifikasi

| Bagian | Isi |
|---|---|
| **Daftar anggota** | Nama, email, role, tanggal bergabung. Ubah role. |
| **Undangan** | Undang lewat email + role. Daftar undangan tertunda, bisa dicabut. |
| **Halaman terima undangan** | Layar publik (dari tautan email), **dua cabang**: pengguna baru (isi nama + password) dan pengguna yang sudah punya akun |
| **Penonaktifan anggota** | **Alur tiga cabang** — lihat §8.1. Ini layar paling rawan disederhanakan secara salah. |
| **Notifikasi** | Daftar notifikasi, penanda belum dibaca, tandai satu / tandai semua. Diambil saat halaman dimuat — **tidak realtime**, dan tidak boleh didesain seolah-olah realtime. |

Beberapa aturan kepemilikan tercermin di UI: tidak bisa mengubah role sendiri; Admin tidak bisa menyentuh Owner; Owner terakhir tidak bisa menonaktifkan dirinya. Aksi yang tidak diizinkan **tidak ditawarkan**, bukan ditawarkan lalu ditolak.

### 7.5 Home metrik · Customer · Task lintas lead · Settings

**Home (metrik):**
- Lead masuk pada periode, jumlah per status, lead belum ter-assign, **conversion rate**
- Tabel performa per anggota: jumlah lead, waktu respons rata-rata, jumlah konversi
- Pemilih periode
- Angka harus bisa diklik menuju daftar lead yang sudah ter-filter (mis. "belum ter-assign" → daftar ter-filter)

> **Angka, bukan grafik.** Grafik tren dan deret waktu adalah Phase 9. Sparkline sederhana boleh diusulkan, tetapi bukan bagian yang diminta.
>
> **`conversion rate` bisa bernilai "belum ada data"**, dan itu **harus tampil berbeda dari 0%**. "Belum ada yang bisa dihitung" bukan "sudah dicoba, gagal semua". Ini butuh perlakuan visual, bukan sekadar teks.

**Customer:** daftar + pencarian + pagination; detail dengan tautan ke lead asalnya; ubah/hapus (hanya Owner/Admin — Manager melihat tanpa tombol itu).

**Task lintas lead:** daftar semua task dengan filter penanggung jawab, status, dan jatuh tempo. Bisa ditandai selesai langsung dari daftar. Task yang lewat jatuh tempo perlu terlihat.

**Settings:** profil organization dan pengguna. Sederhana.

### 7.6 Kerangka aplikasi

Belum ada dan dibutuhkan: **navigasi utama** yang menampung Home, Lead, Customer, Task, Tim, Settings — plus notifikasi dan identitas pengguna (nama, organization, keluar). Sidebar atau top bar adalah keputusan desain; sertakan alasannya.

---

## 8. Empat pola yang tidak boleh disederhanakan

Ini bukan preferensi. Masing-masing melindungi kegagalan yang **tidak terlihat** kalau salah.

### 8.1 Menonaktifkan anggota yang masih memegang lead terbuka

Backend menolak penonaktifan dan mengembalikan **jumlah lead terbuka** milik anggota itu.

**Dialog wajib memaksa pengguna memilih secara sadar** di antara tiga:
1. **Lepas assignment** — lead menjadi tanpa pemilik, masuk daftar "belum ter-assign"
2. **Pindahkan ke anggota lain** — dengan pemilih anggota
3. **Batal**

> ⛔ **Tidak boleh** disederhanakan menjadi *"Yakin ingin menonaktifkan? [Ya] [Batal]"*.
>
> Kenapa: lead yang tetap ter-assign ke orang yang tidak bisa login lagi **tidak muncul di daftar siapa pun** dan **tidak tertangkap filter "belum ter-assign"** — karena secara teknis ia masih punya pemilik. Itu persis kegagalan senyap yang aturan ini dibangun untuk mencegah. Menyembunyikan pilihan di balik satu tombol membuang seluruh alasannya ada.

Anggota **tanpa** lead terbuka boleh dinonaktifkan tanpa dialog tambahan.

### 8.2 Pengguna dengan lebih dari satu organization

Saat masuk, backend bisa menjawab *"pilih organization dulu"* beserta daftarnya. Layar masuk berubah menjadi pemilih organization, lalu melanjutkan. Saat ini diimplementasikan sebagai perubahan keadaan pada form masuk yang sama — desain boleh mengusulkan bentuk lain selama pengguna tidak perlu mengetik ulang email/password.

### 8.3 Data sudah diubah orang lain (konflik penyimpanan)

Bila dua orang menyunting lead atau task yang sama, penyimpanan kedua ditolak dan backend mengirim **keadaan terkini**.

Layar harus: memberi tahu bahwa data sudah berubah, memuat ulang keadaan terkini, dan **tidak pernah menimpa otomatis**. Pengguna memutuskan.

> Ini satu-satunya tempat di seluruh produk pengguna **harus** memilih secara sadar sebelum melanjutkan. Perlakuannya tidak boleh sama dengan notifikasi kesalahan biasa yang bisa diabaikan.

### 8.4 Kesalahan validasi per field

Kesalahan validasi datang dengan **nama field**-nya. Tampilkan **di bawah field yang bersangkutan**, bukan sebagai pesan global di atas form. Pesan global hanya untuk kesalahan yang bukan milik satu field (mis. email/password salah, terlalu banyak percobaan).

---

## 9. Nilai yang butuh perlakuan visual

Ini enumerasi lengkap dari database — tidak akan bertambah tanpa perubahan skema.

### 9.1 Status lead (8) — dan transisi yang sah

```
new → contacted → qualified → proposal → won
```
- Maju/mundur **satu langkah** di jalur utama
- **`lost`, `unqualified`, `spam`** bisa dicapai dari status non-final manapun
- **`unqualified` dan `spam` bersifat final** — tidak ada jalan keluar
- `lost` boleh diperbarui alasannya, dan boleh keluar kembali ke jalur utama

Label Indonesia belum ditetapkan — **usulkan**. `won`/`lost` perlu terbaca jelas; `unqualified` (tidak memenuhi syarat) berbeda dari `spam` (sampah), dan perbedaan itu harus terlihat karena **keduanya dikecualikan dari perhitungan conversion rate**.

Delapan status perlu bisa dibedakan sekilas dalam tabel padat, dan tetap terbaca oleh pengguna dengan gangguan penglihatan warna — **warna saja tidak cukup**.

### 9.2 Alasan `lost` (6)
`price` · `competitor` · `timing` · `no_response` · `not_interested` · `other`

### 9.3 Sumber lead (4)
`manual` · `api` · `form` · `webhook` — **metode capture**, bukan channel marketing.

### 9.4 Tipe activity (10) — timeline

Dibuat sistem otomatis: `lead_created` · `lead_assigned` · `lead_unassigned` · `status_changed` · `lead_converted` · `task_created` · `task_completed`

Dibuat pengguna: `note_added` · `call_logged` · `whatsapp_opened`

Timeline perlu membedakan **peristiwa sistem** dari **catatan manusia** — keduanya bercampur dalam satu aliran waktu, dan pengguna membaca timeline untuk mencari jejak manusia.

### 9.5 Role (4)
`owner` · `admin` · `manager` · `employee`

---

## 10. Keadaan yang wajib punya desain

Sering terlewat, dan justru itu yang dilihat pengguna baru di hari pertama.

| Keadaan | Catatan |
|---|---|
| **Kosong — belum ada data** | Berbeda dari kosong karena filter. Ajak membuat yang pertama. |
| **Kosong — tidak cocok filter** | Ajak menghapus filter. |
| **Memuat** | Daftar lead dan timeline paling sering terlihat memuat. |
| **Gagal memuat** | Termasuk kehilangan koneksi. |
| **Sesi berakhir** | Pengguna dikembalikan ke layar masuk. |
| **Tidak diizinkan** | Manager membuka aksi Owner/Admin — tombolnya tidak ditawarkan sejak awal. |
| **Terlalu banyak percobaan** | Login dan pengiriman email dibatasi; pesannya perlu tempat. |
| **Notifikasi belum dibaca** | Penanda jumlah. |

---

## 11. Di luar cakupan desain

Jangan desain, jangan sertakan sebagai "bonus":

- **Dark mode / tema / kustomisasi tampilan**
- **Grafik tren, deret waktu, laporan lanjutan** (Phase 9)
- **Angka uang** — revenue, nilai deal, target penjualan (Deal belum ada)
- **Papan Kanban / drag-and-drop pipeline** (entity pipeline tidak ada)
- **Realtime** — jangan desain sesuatu yang mengandaikan pembaruan langsung tanpa refresh
- **Layar untuk Employee** (Phase 5, aplikasi mobile terpisah)
- **Landing page / halaman marketing** (repository terpisah, belum dijadwalkan)
- **Manajemen API key & dokumentasi integrator** (Phase 4)
- **Export/import CSV**
- **Onboarding tour, empty-state ilustratif bergerak, dan sejenisnya** — bila terasa perlu, usulkan terpisah dengan alasan

---

## 12. Bentuk keluaran yang diharapkan

Agar bisa langsung diimplementasikan tanpa menebak:

1. **Token desain** — nilai `oklch` untuk warna yang diubah/ditambah, agar bisa masuk langsung ke `globals.css`. Sebutkan token shadcn mana yang diganti.
2. **Peta komponen** — untuk tiap elemen, sebutkan komponen shadcn/ui yang dipakai (`button`, `input`, `card`, `alert`, `table`, `select`, `dialog`, `badge`, `tabs`, …). Bila sebuah elemen tidak ada padanannya di shadcn/ui, tandai eksplisit sebagai komponen kustom.
3. **Desain layar** untuk seluruh §7, dengan keadaan §10 yang relevan.
4. **Empat pola §8** didesain eksplisit — terutama dialog penonaktifan tiga cabang dan konflik penyimpanan.
5. **Label Indonesia** untuk 8 status, 6 alasan `lost`, 4 sumber, 4 role, dan 10 tipe activity.
6. **Catatan aksesibilitas** — kontras dan cara membedakan status tanpa mengandalkan warna.

Bila sebuah layar terasa membutuhkan data yang tidak disebut di §7, **tanyakan** — kemungkinan besar data itu tidak ada di API, dan menambah endpoint di tengah phase UI berarti dua aplikasi berubah dalam satu PR tanpa direncanakan.

---

## 13. Referensi

| Dokumen | Isi |
|---|---|
| [`prd.md`](./prd.md) | Tujuan phase, 10 kebutuhan pengguna, 13 acceptance criteria, di luar cakupan |
| [`td.md`](./td.md) | §5 penanganan error, §7 endpoint yang tersedia, §8 peta layar |
| [`issues.md`](./issues.md) | Pembagian pekerjaan implementasi (#30–#35) |
| [`../../product/glossary.md`](../../product/glossary.md) | Kosakata mengikat — §6 di atas adalah ringkasannya |
| [`../../architecture/freeze.md`](../../architecture/freeze.md) | Bagian 3.2 (cakupan dashboard & metrik), 2.3 (jaring pengaman) |
