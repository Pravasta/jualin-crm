# Phase 8.6 — Redesain UI · Notes

> Realitas implementasi, per issue. Penyimpangan dari TD beserta alasannya, utang teknis, dan catatan untuk
> session berikutnya. Status tidak ditulis di sini (ADR-008).

---

## #159 — Token desain & fondasi visual dashboard

**Yang berubah:** `globals.css` (`:root`), `layout.tsx` (font), `lib/labels.ts` (`STATUS_META`: warna,
tint, `shape`), komponen baru `components/status-badge.tsx`, test baru `lib/labels.test.ts`.

### Tabel kontras — dihitung, bukan disalin

Rumus luminansi relatif WCAG 2.1; oklch → sRGB lewat matriks OKLab standar. Angka yang sama dihitung
ulang oleh `labels.test.ts` setiap kali test jalan, jadi tabel ini tidak bisa diam-diam basi.

| Teks | Latar | Rasio | Token sheet |
|---|---|---|---|
| `--foreground` 0.22 0.012 60 | `--background` 0.995 0.003 70 | **17.09** | 17.1 ✓ |
| `--foreground` | `--card` putih | **17.34** | — |
| `--foreground` | `--muted` 0.965 0.006 60 | **15.65** | — |
| `--muted-foreground` 0.505 0.012 60 | putih / `--background` | **5.89 / 5.80** | 5.8 ✓ |
| `--muted-foreground` | `--muted` / `--secondary` | **5.32 / 5.08** | — |
| `--secondary-foreground` 0.30 0.012 60 | `--secondary` 0.95 0.012 60 | **11.79** | 11.8 ✓ |
| `--primary-foreground` **putih** | `--primary` 0.56 0.19 41 | **5.05** | 5.05 ✓ |
| *(sebelumnya `0.985 0 0`)* | `--primary` | *4.83* | — |
| `--accent-strong` 0.48 0.17 41 | putih / `--background` / `--accent-tint` | **7.04 / 6.94 / 6.16** | 7.04 ✓ |
| putih | `--destructive` 0.505 0.21 27 | **6.47** | 6.47 ✓ |
| `--input` (tepi kontrol) | putih | 1.75 | 1.75 ✓ (dekoratif, bukan teks) |

**Status lead (Opsi B, dashboard)** — warna hex dari handoff:

| Status | Warna | di atas putih | di atas tint 85% (lama) | di atas tint **94%** (dipakai) |
|---|---|---|---|---|
| Baru | `#006cd3` | 5.14 | 4.20 ✗ | 4.75 |
| Dihubungi | `#8156c0` | 5.22 | 4.25 ✗ | 4.81 |
| Memenuhi Syarat | `#007b7d` | 5.09 | 4.15 ✗ | 4.69 |
| Penawaran | `#a05d00` | 5.17 | 4.22 ✗ | 4.77 |
| Menang | `#1d802b` | 5.03 | 4.11 ✗ | **4.65** (terendah) |
| Kalah | `#ce2930` | 5.27 | 4.29 ✗ | 4.86 |
| Tidak Memenuhi Syarat | `#6d6d6d` | 5.17 | 4.21 ✗ | 4.77 |
| Spam | `#7f6964` | 5.11 | 4.17 ✗ | 4.72 |

**Menyimpang dari handoff:** token sheet menyatakan warna status "≥4.6:1 di atas latar tintnya sendiri".
Untuk tint 85% itu **tidak benar** (4.11–4.29). **Alasan tint 94%:** badge Opsi B sendiri menggambar di
atas putih dan lolos, tetapi `background` masih dipakai pita "Lead ditutup" dan chip filter aktif, jadi
tint-nya harus lolos juga. Ini kali ketiga angka desainer meleset (#40, #70).

### Keputusan lain

- **`--primary-foreground` jadi putih murni**, dari `0.985 0 0`: 4.83 → 5.05:1, sesuai token map handoff.
- **Font:** Plus Jakarta Sans (400–800) + JetBrains Mono (400, 500) lewat `next/font/google`. Variabel
  `--font-plus-jakarta-sans`/`--font-jetbrains-mono`; `@theme inline` menunjuk keduanya, tidak ke dirinya
  sendiri (bug #40). Dicek di build produksi: kelas variabel keduanya terpasang di `<html>` dan CSS
  menyajikan `font-family: Plus Jakarta Sans`.
- **Token sidebar** diisi nilai hangat yang sama, meski belum dipakai komponen mana pun. Nilai bawaan
  shadcn (abu-abu dingin, ungu di `.dark`) akan jadi jebakan saat #160 memakainya.
- **`.dark` tidak disentuh**, karena dark mode di luar cakupan (PRD).

**Menyimpang dari issue:** issue menyebut *"tidak menyentuh satu layar pun"*. Dua badge status (tabel lead
dan header detail lead) **diganti `<StatusBadge>`**. Alasannya: komponen yang dibuat tanpa pemakai
melanggar Aturan #27, dan badge status *adalah* keluaran utama issue ini. Tata letak kedua layar tidak
berubah. Chip filter aktif di daftar lead juga beralih ke `meta.background`: tint lamanya (82%) akan
gagal AA dengan warna baru.

**Belum diverifikasi terhadap `crm_be` sungguhan:** Docker tidak jalan di sesi ini. Yang terverifikasi
hanya typecheck, lint, 243 test, build, dan HTML/CSS build produksi. Tampilan badge dengan data sungguhan
dicek saat review PR.

**Utang teknis:** layar masih menulis abu-abu dingin langsung sebagai inline style, sebagian di bawah AA
(`oklch(0.65 0 0)` 3.23:1). Rinciannya dan pembagian per issue ada di
[`docs/issues/159-design-tokens.md`](../../issues/159-design-tokens.md).

**Catatan untuk session berikutnya:** #160 bisa langsung memakai token `--sidebar-*`. Setiap layar yang
dirombak mengganti `oklch(… 0 0)` mentah dengan token, bukan menyalin angkanya.

---

## #160 — Kerangka aplikasi responsif

**Yang berubah:** `components/app-shell.tsx` (tiga tata letak), `lib/nav.ts` (`BOTTOM_NAV_HREFS`,
`MORE_NAV_ITEMS`, `isMoreActive` + test), `components/ui/sheet.tsx` (baru),
`components/notification-bell.tsx` (lebar panel dibatasi lebar layar).

| Lebar | Navigasi |
|---|---|
| ≥1024 (`lg:`) | Sidebar 232 px (`w-58`), label penuh, nama organization, identitas di bawah |
| 768–1023 (`md:`) | Sidebar 68 px (`w-17`), ikon saja. Label lewat `title` + `aria-label`, jumlah "tanpa pemilik aktif" menempel di sudut ikon Lead |
| <768 | Tanpa sidebar. Header (logo + judul + lonceng), **bilah bawah tetap** Beranda · Lead · Tugas · Lainnya, dan sheet "Lainnya" berisi Customer, Tim, Connect, Langganan, Pengaturan, identitas (nama · organization · role), Keluar |

- **Breakpoint dari CSS**, bukan `window.innerWidth` (TD §4.1): ketiga tata letak ada di HTML pertama, dan
  yang tidak cocok disembunyikan `display:none`. Tidak ada lompatan tata letak saat hydrate, dan server tidak
  perlu mengukur apa pun.
- **Bilah bawah berisi 4 slot, bukan 5.** Laporan masuk di #171 bersama rutenya (komentar di issue ini).
  Slotnya tetap di semua halaman: prototipe mengganti slot kelima per halaman (Connect/Paket/Atur), dan
  navigasi yang berpindah tempat bukan navigasi.
- **Target sentuh:** item bilah bawah `min-h-14` (56 px), item sheet `min-h-12` (48 px). Keduanya di atas
  44 px.
- **Safe area iPhone:** bilah bawah dan sheet menambahkan `env(safe-area-inset-bottom)`. `main` memberi ruang
  5.5rem + safe area di HP, supaya baris terakhir halaman tidak tertutup bilah.
- `main` beralih dari `bg-muted/30` ke `bg-background` (putih hangat token #159), supaya kartu putih terbaca
  sebagai permukaan tersendiri, sesuai handoff.
- Kotak logo memakai `bg-primary` (token), bukan gradasi oklch mentah.

**Menyimpang dari TD:** tidak ada. **Menyimpang dari issue:** item Laporan tidak dipasang (lihat di atas;
dicatat di komentar issue dan `issues.md`).

**`shadcn add sheet` tidak dipakai.** CLI-nya berhenti di prompt untuk menimpa `components/ui/button.tsx`
(yang sudah kita ubah), dan sebelum itu **sudah menambahkan paket npm tak dikenal bernama `cn`** ke
`package.json`. Keduanya dibatalkan (`git checkout`, `npm uninstall`). `sheet.tsx` ditulis tangan di atas
primitive `@base-ui/react/dialog` yang sama dengan `dialog.tsx`, dan hanya sisi bawah yang dibuat, sesuai
Aturan #27. **Untuk issue berikutnya:** jangan jalankan `shadcn add` tanpa memeriksa `git diff package.json`
sesudahnya.

**Verifikasi:** typecheck, lint (0/0), 247 test, build. Stack lokal dijalankan (`docker compose up`,
migration di versi 10). **Pengecekan visual di 360/390/820/1440 px belum dilakukan oleh agent**, karena
ekstensi Chrome tidak tersambung di sesi ini. Pengecekannya diserahkan ke review PR.

**Catatan untuk session berikutnya:** guliran horizontal di HP sekarang hanya bisa berasal dari **isi**
halaman (tabel lebar di daftar lead, customer, tugas, dan anggota), bukan dari kerangka. Itu wilayah
#161–#167. Dialog lama (`components/ui/dialog.tsx`) belum berubah jadi sheet di HP, dan itu juga
wilayah layar masing-masing.

---

## #161 — Daftar Lead

**Yang berubah:** `leads/leads-list.tsx` (tata letak tiga lebar), `components/ui/dialog.tsx` (sheet dari bawah di
HP), `components/ui/input.tsx` dan `button.tsx` (44 px di HP), `components/no-contact-badge.tsx` (gaya handoff),
`lib/lead-filters.ts` (`sheetFilterCount`) dan `lib/lead-contact.ts` (`primaryContact`), masing-masing dengan test.

| Lebar | Daftar | Filter |
|---|---|---|
| <768 | Kartu (nama, `#nomor`, badge, "Belum ada kontak", pemilik · sumber · tanggal), tombol + mengambang di atas bilah bawah | Chip status **digeser ke samping**. Sumber/pemilik/tanggal di sheet **Filter**, dengan jumlah filter tersembunyi di tombolnya |
| 768–1023 | Tabel: Nama (dengan `#nomor` di bawahnya) · Status · Pemilik · Masuk | Chip status membungkus; sumber/pemilik/tanggal sebaris di bawahnya |
| 1024–1279 | + Sumber | sama |
| ≥1280 | + Nomor sebagai kolom sendiri · Kontak | sama |

**Kolom bertambah menurut lebar**, bukan satu tabel yang menyusut. `table-fixed` dengan lebar tetap untuk kolom kecil.
Pada percobaan pertama, Nama tinggal ~120 px di 1024. Status diberi 12.5rem karena "Tidak Memenuhi Syarat"
adalah badge terlebar dan tidak boleh terpotong.

**Filter tidak berubah artinya.** Parameter URL dan query sama persis. Yang berubah hanya tempatnya: definisi JSX
filter sekunder dipakai dua kali (sebaris dan di sheet), jadi keduanya tidak bisa menyimpang. Tombol di sheet
berlabel **"Tampilkan N lead"**, bukan "Terapkan": filter sudah berlaku saat dipilih, dan tombol ini hanya
menutup sheet. Label "Terapkan" akan menjanjikan langkah yang tidak ada.

**Menyimpang dari handoff:**
- **Periode tetap rentang tanggal** (dua `<input type="date">`), bukan preset "7/30 hari, bulan ini" seperti di
  prototipe. Preset mengubah perilaku filter yang sudah berjalan, dan itu bukan pekerjaan redesain.
- **Sumber tetap multi-pilih (chip)**, bukan `<select>` tunggal seperti di prototipe. Brief §8.4 meminta
  multi-pilih, dan itu yang sudah berjalan.
- Pemilik mendapat opsi **"Tanpa pemilik aktif"** di dropdown-nya juga, dengan parameter `assigned_to=none` yang sama
  seperti chip, supaya sheet HP bisa memilihnya tanpa menggulir chip.
- Judul halaman "Lead" + subjudul dari prototipe **tidak** diulang di isi halaman, karena judulnya sudah ada di header
  kerangka (#160).

**Menyimpang dari issue, keduanya global dan hanya CSS:**
- **`DialogContent` menjadi sheet dari bawah di bawah 768 px** untuk *semua* dialog di aplikasi, bukan hanya
  "Lead baru". TD §4.3 meminta ini di setiap layar. Melakukannya sekali di komponen dasar berarti 16 dialog lain
  ikut benar tanpa disentuh, dan tidak ada dialog yang perlu tahu lebar layar.
- **`Input` dan `Button` ukuran `default` jadi 44 px di bawah 768 px** (target sentuh brief §9.2). Keduanya tetap
  32 px mulai 768 px. Input memang sudah 16 px di HP (di bawah 16 px, iOS Safari melakukan zoom saat fokus). Override
  14 px buatan saya sendiri di kotak pencarian dan filter sempat melanggarnya, dan sudah dikembalikan ke `text-base`.

**Test yang diubah:** `no-contact-badge.test.ts` sebelumnya mengunci kelas `bg-muted`. Maksud test itu adalah
"warna netral, bukan warna error", jadi kini ia menerima token netral (`bg-secondary` atau `bg-muted`) dan tetap
menolak `destructive/red/amber`.

**Temuan #159 untuk layar ini selesai:** teks "tanpa pemilik" `oklch(0.65 0 0)` (3.23:1) diganti
`text-accent-strong` tebal (7.04:1), mengikuti handoff. Tidak ada lagi abu-abu mentah di `leads-list.tsx`.

**Verifikasi:** typecheck, lint (0/0), **252 test**, build. **Diverifikasi visual terhadap `crm_be` sungguhan**
lewat Chrome headless yang dikendalikan CDP (sesi dari login API akun uji lokal, 7 lead contoh dengan status
berbeda-beda):
- `/leads` di 360, 390, 820, 1024, 1440: **`scrollWidth` = lebar layar di kelimanya**, tanpa elemen yang meluap
- sheet Filter dan dialog Lead baru di 360/390 (sheet dari bawah, target 44 px), dialog di 1024 (tetap di tengah)
- sapuan cepat halaman lain di 360/820: tidak ada guliran horizontal halaman. **Tabel di Beranda dan Tim masih
  terpotong di dalam kartunya di 360 px**, dan itu wilayah #163 dan #165.

**Catatan untuk session berikutnya:** skrip CDP (`shoot.mjs`) ada di scratchpad sesi ini, tidak di repo. Bila
dipakai lagi, polanya: `curl` login → cookie jar → `Network.setCookie` → `Emulation.setDeviceMetricsOverride`
→ ukur `document.documentElement.scrollWidth`. Tombol `position: fixed` punya `offsetParent === null`, jadi pakai
`getClientRects().length` untuk memeriksa keterlihatannya.

---

## #162 — Detail Lead

**Yang berubah:** `leads/[id]/lead-detail.tsx` (render), `lead-status-panel.tsx` (tombol & label stepper),
`lost-reason-dialog.tsx` dan `conflict-dialog.tsx` (token), `new-task-dialog.tsx` dan `lib/activity-text.ts`
("Task" → "Tugas"). **Tidak ada logika yang berubah**: pemanggilan API, `version`, penanganan 409, aturan transisi
(`lib/lead-pipeline.ts`, `lib/lead-status.ts`), dan izin role sama persis.

| Lebar | Susunan |
|---|---|
| <1024 | Satu kolom, dengan urutan dari brief §9.2: header + area status → tambah catatan → timeline → penugasan → tugas → konversi / hapus |
| ≥1024 | Kolom utama + kolom samping 300 px (penugasan, tugas, konversi, hapus) |

- **Header:** `#nomor` monospace, nama 20/24 px, badge status, tombol **Ubah** (dulu tautan kecil). Detail lead
  jadi daftar `dl` berlabel (Email, Telepon, Perusahaan, Sumber, Masuk). **Nilainya membungkus, tidak pernah
  dipotong.** Pada percobaan pertama, telepon terpotong jadi "0857-9999-00…" di 1024 px. Jumlah kolom mengikuti
  lebar kolom utama: 2 kolom di 1024, 3 di 1280, dan 5 mulai 1400.
- **Stepper di 360 px:** tiap tahap hanya ~60 px, sedangkan "Penawaran" dalam semibold 11.5 px ~62 px. Satu kata
  tidak bisa dipatahkan, jadi label meluap ke tetangganya. Kini 10.5 px di bawah 640 px, dengan
  `overflow-wrap:anywhere` sebagai pengaman terakhir. **Diukur**, bukan dilihat: di 360, 390, 820, dan 1024 px tidak
  ada label terlihat yang `scrollWidth > clientWidth`. Yang terukur hanya span `sr-only` "(selesai)", yang memang
  1 px.
- **Tombol status:** Lanjutkan/Buka kembali kini tombol primary solid (putih di atas primary, 5.05:1).
  Kembali/Tutup tetap netral. 44 px di HP.
- **Tambah catatan:** tiga tipe berlabel **Catatan · Telepon · WhatsApp** dengan ikon lucide, sesuai brief §8.5.
  Dulu labelnya "📝 Catatan / 📞 Log telepon / 💬 WhatsApp dibuka". Emoji tampil berbeda per OS dan tidak bisa
  mengikuti token. Tipe yang dikirim ke API tidak berubah.
- **Timeline:** jejak manusia = ikon di atas `accent-tint`, teks `foreground` tebal-sedang. Peristiwa sistem = titik
  netral, teks `muted-foreground` (5.32:1 di atas `--muted`, lolos AA). Nama penulis pindah ke baris waktu.
- **Tugas:** checkbox + judul dibungkus `label` (target sentuh 44 px di HP). Selesai tetap **satu arah**.
  Terlambat = `text-destructive` tebal. Tombol hapus 36 px di HP.
- **Konversi:** latar `STATUS_META.won.color` (putih 5.03:1), dulu hijau oklch mentah. **Hapus lead:** outline
  `destructive`, terpisah dari tombol rutin (brief §5.1 prinsip 3).

**Kosakata:** "Task" di layar diganti **"Tugas"** (kartu, dialog "Tugas baru", timeline "Tugas dibuat: …" /
"Tugas selesai", pesan konflik). Brief §6 menetapkan "Tugas" untuk layar, dan menu sudah bernama "Tugas" sejak Phase 3.
`glossary.md` tetap memakai **Task** untuk entity, jadi nama tipe dan variabel di kode tidak diubah.
`activity-text.test.ts` ikut diperbarui (dua string).

**Menyimpang dari handoff:**
- Label jatuh tempo tugas masih tanggal biasa ("26 Sep 2026" / "· Terlambat"), belum label kalender ("besok",
  "Terlambat 2 hari"). Label kalender dimiliki layar Tugas (#165, brief §8.7). Membuat dua versi di dua issue akan
  menghasilkan dua implementasi.
- Tugas dan penugasan **tidak** dibuat bisa dilipat di HP. Brief menyebutnya "bisa", bukan wajib, dan keduanya
  pendek.

**Temuan #159 untuk layar ini selesai:** `lead-detail.tsx`, `lost-reason-dialog.tsx`, dan `conflict-dialog.tsx`
bersih dari oklch mentah (abu-abu `0.6`/`0.65` yang gagal AA sudah hilang).

**Verifikasi:** typecheck, lint (0/0), 252 test, build. **Visual terhadap `crm_be` sungguhan** (Chrome headless
+ CDP): lead Penawaran dengan timeline & dua tugas (satu terlambat), lead Menang yang bisa dikonversi, lead Kalah,
dan lead tanpa kontak, di 360/390/820/1024/1280/1440. `scrollWidth` = lebar layar di semuanya, dan tidak ada
`dd`/`h1` yang terpotong.

---

## #163 — Beranda

**Yang berubah:** `home-screen.tsx` (render), `lib/metrics.ts` (`formatAvgResponseSeconds` di bawah satu menit, dengan test).

- **Empat kartu KPI**, 2 kolom di HP/tablet dan 4 mulai 1024: Lead masuk · Belum ter-assign · Conversion rate · Lead
  Menang, dengan urutan dan angka besar 28 px sesuai handoff. Tiga kartu yang merujuk satu daftar lead adalah
  tautan. Conversion rate tidak, karena ia rasio dan tidak ada satu daftar yang cocok.
- **Conversion rate** kini diberi keterangan **"Tanpa Spam & Tidak Memenuhi Syarat"** di kartunya. Brief §10.1
  meminta penjelasan ini di Laporan karena pengguna akan bertanya kenapa angkanya beda dari hitungan sendiri.
  Pertanyaan yang sama muncul di Beranda. "Belum ada data" tetap tampil berbeda dari 0%.
- **Belum ter-assign > 0** kini `text-accent-strong` (7.04:1), mengikuti handoff. Dulu merah Kalah: tanpa pemilik
  adalah peringatan, bukan kegagalan.
- **Jumlah per status** jadi kartu sendiri dengan chip bertepi warna status (pasangan yang sama dengan badge, 5.03–5.27:1).
- **Performa per anggota:** tabel mulai 768 px, kartu di HP. **Setiap baris kini membuka daftar lead anggota itu**
  (`assigned_to=<membership_id>` + rentang periode yang sama). Brief §8.3: "setiap angka bisa diklik". Dulu baris
  ini tidak bisa diklik.
- Memuat = skeleton per kartu dan per baris, bukan "…".
- **"< 1 menit"**, bukan **"0 menit"**. Data sungguhan menunjukkan anggota yang menyentuh lead 20 detik setelah
  masuk tertulis "0 menit", yang terbaca seperti "langsung". Di bawah 60 detik kini tertulis apa adanya.

**Menyimpang dari brief/handoff:** **pintasan ke Laporan belum dipasang**, karena rutenya lahir di #171. Tambahan
cakupan itu dicatat sebagai komentar di #171, dengan alasan yang sama seperti item navigasi Laporan.

**Verifikasi:** typecheck, lint, **253 test**, build. Visual terhadap `crm_be` sungguhan di 360/820/1024/1440:
`scrollWidth` = lebar layar di semuanya. Tabel performa yang di #161 masih terpotong di 360 px kini menjadi kartu.
**Catatan:** sesi uji kedaluwarsa di tengah jalan (halaman diarahkan ke Masuk), jadi login ulang lewat API.
Pengecekan visual yang melaporkan `scrollWidth` bersih tetap harus membaca **teks halamannya**, bukan hanya
angkanya: halaman Masuk juga tidak meluap.

---

## #164 — Customer

**Yang berubah:** `customers/customer-list.tsx` dan `customers/[id]/customer-detail.tsx` (render saja; query,
pencarian, pagination, izin, dan dialog ubah/hapus tidak berubah).

- **Daftar:** pencarian berikon, tabel mulai 768 px (Customer · Perusahaan [≥1024] · Kontak · Customer sejak), kartu di
  HP, skeleton saat memuat, dua keadaan kosong. Kosong karena pencarian kini punya tombol **Hapus pencarian**. Nama
  di tabel adalah `Link`, sama seperti daftar lead (#161), jadi keyboard dan pembaca layar tetap bisa masuk.
- **Detail:** pola header yang sama dengan Detail Lead (#162): judul, **Ubah** sebagai tombol (Owner/Admin saja),
  fakta dalam `dl` yang **membungkus, tidak memotong**, catatan, lalu **"Berasal dari lead #N Nama"** sebagai tautan.
  **Hapus customer** berada di luar kartu dengan outline destructive.

**Yang tidak dibangun, dan kenapa (PRD §Handoff vs sistem):** handoff menampilkan **nilai kontrak (Rp)**, **riwayat
pembelian/order**, **form catatan**, dan **timeline** customer. Keempatnya dummy: `customers` tidak punya kolom
nilai, tidak ada entity order, dan aktivitas di sistem ini hanya tertaut ke lead (`activities.lead_id NOT NULL`).
Angka uang di luar cakupan (brief §6). Kolom "Pemilik" di daftar handoff juga tidak ada di Customer
(`converted_by_membership_id` adalah **siapa yang mengonversi**, bukan pemilik), jadi tidak ditampilkan dengan label
yang salah.

**Verifikasi:** typecheck, lint, **253 test**, build. Visual terhadap `crm_be` sungguhan: lead Menang dikonversi
lewat API menjadi customer pertama, lalu daftar di 360/820/1440 dan detail di 360/1440 diperiksa. `scrollWidth` = lebar
layar di semuanya.

---

## #165 — Tugas & Tim

**Yang berubah:** `tasks/task-list.tsx`, `team/team-screen.tsx`, `team/deactivate-member-dialog.tsx`, file baru
`lib/due-label.ts` (+ test), dan `leads/[id]/lead-detail.tsx` (memakai label jatuh tempo yang sama).

### Tugas
- **Label jatuh tempo kalender** (brief §8.7): *Jatuh tempo hari ini / besok / dalam 3 hari / 12 Okt / Terlambat 2
  hari*. `lib/due-label.ts` adalah **port aturan-demi-aturan** dari `crm_employee/lib/shared/due_label.dart`
  (#153), dan **ke-11 kasus test-nya disalin**, termasuk batas tengah malam (dihitung per hari kalender lokal,
  bukan blok 24 jam) dan invarian "tertulis Terlambat tepat ketika `isOverdue`". Satu tugas kini terbaca sama di
  dashboard dan di HP. Detail Lead (#162) ikut memakainya.
- **Terlambat menonjol dua cara:** teks `destructive` tebal **dan** garis tepi kiri merah pada barisnya, supaya
  tetap terbaca meski warnanya hilang.
- **"Tugas saya"** menjadi opsi pertama di pemilih penanggung jawab (`assigned_to` = membership sendiri). Ini
  pengganti tab "Tugas saya" di handoff, tanpa parameter baru. Tab **"Terlambat"** dari handoff tidak dibangun:
  `due_before` menerima tanggal (akhir hari), jadi "terlambat" yang tepat per menit tidak bisa dinyatakan lewat
  filter yang ada. Menambah parameter backend di luar cakupan redesain.
- Dua keadaan kosong: **Belum ada tugas** vs **Tidak ada tugas yang cocok**. Dulu hanya ada satu pesan untuk
  keduanya.
- Baris menampilkan **"Buka lead"**. Tugas hanya membawa `lead_id`, tidak membawa nomor/nama lead. Mengambil tiap
  lead untuk satu label adalah N permintaan tambahan per halaman.
- "task" → "tugas" di seluruh teks layar ini.

### Tim
- Tabel mulai 768 px (Anggota dengan avatar · Role · Bergabung [≥1024] · **Lihat lead** · aksi), kartu di HP. **Lihat
  lead** membuka `/leads?assigned_to=<membership_id>` (brief §8.8 via handoff).
- **Role:** badge teks untuk yang tidak bisa diubah (Owner beraksen, lainnya netral). Dropdown hanya untuk yang
  boleh diubah, dengan aturan `team-permissions.ts` yang sama. Empat role saja; "Sales" di handoff adalah dummy.
- Kontrol role dan tombol Nonaktifkan dibuat **sekali per baris** lalu diletakkan di tabel atau kartu, jadi
  kedua tata letak tidak mungkin menawarkan aksi berbeda ke role yang sama.

### Dialog nonaktifkan tiga cabang
- **Perilakunya tidak berubah:** tetap dibuka **hanya** setelah percobaan nonaktifkan biasa dijawab
  `409 membership_has_open_leads`, jumlahnya dari body error, dan **Nonaktifkan** mati sampai satu cabang dipilih.
- **Diperbaiki:** `<select>` "Pindahkan ke" dulu berada **di dalam `<button>`**. Itu HTML tidak valid (konten
  interaktif di dalam tombol), dan keyboard serta pembaca layar tidak sepakat soal perilakunya. Kini kedua pilihan
  adalah `role="radio"` dalam `radiogroup`, dan pemilih anggota muncul **di bawah** pilihannya.
- "Lepas assignment" → **"Lepas penugasan"** (brief §12.1).

**Temuan #159 ditutup:** tidak ada lagi `oklch(… 0 0)` mentah di layar mana pun (`(protected)` dan `(auth)`).
Rinciannya di `docs/issues/159-design-tokens.md`.

**Verifikasi:** typecheck, lint, **264 test** (+11 `due-label`), build. Visual terhadap `crm_be` sungguhan:
- anggota kedua **diundang dan menerima undangan lewat API** (token dari Mailpit), lalu diberi dua lead terbuka dan
  satu tugas
- klik **Nonaktifkan** menghasilkan **409 sungguhan** dari backend, dan dialog tampil dengan "2 lead terbuka", di
  1024 (tengah) dan 360 (sheet dari bawah)
- `/tasks` (satu tugas terlambat 5 hari, satu hari ini, satu besok) dan `/team` di 360/820/1440: `scrollWidth` =
  lebar layar

**Catatan untuk session berikutnya (skrip uji CDP):** cookie `csrf_token` **tidak boleh** dipasang sebagai
HttpOnly. Dashboard membacanya dari `document.cookie` untuk header `X-CSRF-Token`. Pada percobaan pertama semua
aksi tulis di layar gagal "Token CSRF tidak valid". Itu kesalahan skrip, bukan aplikasi.

---

## #166 — Connect

**Yang berubah:** halaman induk, tiga layar kanal (`api/`, `form/`, `webhook/`), detail formulir & webhook,
riwayat pengiriman, dua halaman dokumentasi. File baru `connect/connect-ui.tsx`: `BackLink`, `SectionHeader`,
`NotForRole`, `ListSkeleton`, `EmptyCard`, `tableHeadRow`. **Tidak ada logika yang berubah**: izin per role
(`canManageAPIKeys`/`canManageForms`/`canManageWebhooks`), gerbang paket (`channelCardState`), alur kredensial
satu kali, dan kirim ulang pengiriman sama persis.

- **Tiga kartu kanal, bukan enam.** WhatsApp Business, Instagram DM, Marketplace, dan Google Sheets di handoff adalah
  dummy, dan chat inbox ⛔ scope. Kartu terkunci menampilkan gembok + **"Terkunci oleh paket"**, **tanpa nama paket
  dan tanpa tombol upgrade** (brief §8.9, D6). Kartu yang bisa dibuka punya kaki **"Kelola →"**.
- **API key / Formulir / Webhook:** tabel mulai 768 px (kolom sekunder bergabung di 1024), **kartu di HP**. Status
  Aktif/Dicabut/Nonaktif tampil sebagai badge teks. Di HP, kartu API key punya tombol **Cabut** selebar kartu.
- **Riwayat pengiriman webhook = satu daftar di semua lebar, bukan tabel.** Setiap baris membawa pesan error dari
  server ("connection refused", "cannot resolve …"), dan pesan itulah nilai utama layar ini (komentar #103). Kolom
  tabel akan memotongnya atau memaksa geser samping. Status tampil sebagai badge: Berhasil (pasangan Menang), Gagal
  (destructive), Menunggu/Sedang dikirim (netral). "Kirim ulang" tetap hanya untuk yang gagal.
- **Tabel referensi di dokumentasi** (field yang diterima, kode error) boleh digeser di dalam kotaknya sendiri
  (`overflow-x-auto`, `min-w-[480px]`), jadi **halaman** tidak pernah ikut bergeser. Brief §9.2 melarang guliran
  halaman, bukan tabel referensi yang dibaca developer.
- **`connect-ui.tsx` sengaja lokal di `connect/`.** Empat layar Connect menggambar tombol kembali, judul, kotak
  "tidak tersedia untuk role", teks memuat, dan kosong masing-masing dengan cara yang sedikit berbeda. Bagian lain
  aplikasi punya bentuk sendiri, jadi belum ada pemanggil kedua di luar Connect (Aturan #28).
- Header detail webhook bertumpuk di HP. Pada percobaan pertama, URL panjang terhimpit di samping tombol
  "Dokumentasi verifikasi".

**Temuan #159:** Connect memang sudah memakai token sejak Phase 6–7. Tidak ada `oklch` mentah yang perlu diganti.

**Verifikasi:** typecheck, lint, 264 test, build. Visual terhadap `crm_be` sungguhan:
- formulir dan endpoint webhook dibuat lewat API. Webhook sengaja ke domain `.invalid`, lalu sebuah lead dibuat untuk
  memicu event, sehingga riwayat berisi **pengiriman sungguhan** (Menunggu, percobaan ke-1, pesan "cannot resolve …")
- **delapan layar** (induk, 3 kanal, 2 detail, 2 dokumentasi) di 360 dan 1024: `scrollWidth` = lebar layar di
  keenam belas kombinasi
- **kredensial satu kali:** API key dibuat **lewat UI** di 390 px. Dialog tampil sebagai sheet, dengan kunci utuh,
  Salin kunci, contoh `curl`, peringatan "tidak akan ditampilkan lagi", dan **Selesai mati sampai kotak "Saya sudah
  menyimpan…" dicentang**

**Belum diverifikasi:** status **Gagal** dengan tombol **Kirim ulang**. Pengiriman baru mencapai "gagal" setelah
seluruh jadwal retry habis (berjam-jam). Tampilannya mengikuti kode yang sama dengan status lain, tetapi belum
terlihat dengan data sungguhan.

---

## #167 — Langganan & Pengaturan

**Yang berubah:** `subscription/subscription-screen.tsx`, `settings/settings-screen.tsx`, `lib/plan.ts`
(`usageLevel`, pengelompokan digit Indonesia), masing-masing dengan test.

### Langganan
- **Paket Anda** jadi kartu dengan nama paket besar dan dua bar pemakaian (2 kolom mulai 768 px). Bar kini punya
  `role="progressbar"` dengan nilai. Mulai **80%** bar berubah `destructive` dan tertulis **"Hampir mencapai
  batas"**. Di **100%** tertulis **"Batas tercapai"**. Kondisi batas dinyatakan dengan kata, bukan warna saja
  (`usageLevel`, ambang dari handoff). Bar tetap tidak pernah melebihi 100% (`usageRatio`, #125).
- **Perbandingan paket:** tiga kartu (1 kolom di HP, 3 mulai 768 px). Paket aktif bertepi primary + "Paket Anda".
  Harga besar dari `price_label`. Isinya **hanya dari `GET /v1/plans`**: batas lead, batas anggota, dan **ketiga kanal**
  (✓ / –). Kanal ditampilkan karena ada di katalog. Kuota fiktif tidak ditambahkan.
- **"2.000", bukan "2000".** `formatLimit`/`formatUsage` memakai `toLocaleString("id-ID")`, dikunci ke id-ID supaya
  tidak menjadi "2,000" di browser berbahasa Inggris. Test ditambahkan.
- Tombol **Coba Pro (test)** tetap hanya untuk Owner saat `test_checkout_available`. Enterprise tetap tanpa tombol
  beli, dan tautan kontaknya kini tombol outline 44 px di HP.

**Yang tidak dibangun (PRD §Handoff vs sistem):** paket Starter/Tim/Bisnis dan harganya, tombol
Upgrade/**Downgrade** di tiap kartu (jalur downgrade tidak ada, Phase 8 D4), **riwayat pembayaran + unduh invoice**
(⛔ invoice, payment service terpisah), bar "Penyimpanan file" (tidak ada penyimpanan file; kelas biaya baru), dan
fitur "Peran & izin kustom".

### Pengaturan
- Tetap **baca saja**: dua kartu (Organization, Profil Anda) dengan fakta dalam `dl` yang membungkus, plus satu
  kalimat "Data di halaman ini hanya bisa dibaca".
- **Tidak dibangun:** tab Notifikasi (preferensi tidak ada; "ringkasan mingguan via email" adalah kelas biaya
  baru), tab API & Webhook (sudah di Connect, ADR-012), tab Keamanan, dan zona waktu/email bisnis yang bisa diubah.
- **Zona waktu organization tidak ditampilkan.** Datanya nyata (`organizations.timezone`), tetapi `/v1/me` tidak
  membawanya. Menampilkannya berarti perubahan API, bukan redesain. Dicatat di sini, tidak diam-diam ditambahkan.

**Verifikasi:** typecheck, lint, **268 test**, build. Visual terhadap `crm_be` sungguhan (paket Free, 8/100 lead,
**2/2 anggota → "Batas tercapai"**) di 360 dan 1024: `scrollWidth` = lebar layar. **Belum terlihat:** tombol
**Coba Pro (test)**, karena `test_checkout_available` mati di lingkungan lokal ini.

---

## #168 — Layar auth

**Yang berubah:** `(auth)/layout.tsx` dan `(auth)/login/page.tsx`. Keenam layar (Masuk, Daftar, Verifikasi email,
Lupa password, Atur ulang password, Terima undangan) sudah dibangun dari `Card`/`Input`/`Button` shadcn, jadi token
#159 dan ukuran sentuh 44 px #161 sudah mereka warisi. Yang tersisa adalah kerangkanya.

- **Merek di atas kartu** (kotak "J" primary + "Jualin CRM"), di atas latar putih hangat token.
- **Di HP kartu mulai dekat bagian atas**, tidak di tengah persis. Keyboard layar yang muncul tidak mendorong tombol
  kirim keluar pandangan. Mulai 768 px kartu kembali di tengah.
- **Judul kartu (20 px tebal) dan tautan (`accent-strong` tebal, 7.04:1) diatur sekali di layout** lewat varian
  turunan (`[&_[data-slot=card-title]]`, `[&_a]`), bukan diulang di enam berkas. Keenamnya memakai `CardTitle` dan
  `<Link>` biasa.
- **Login:** pemilih organization (ADR-007) setinggi 44 px di HP, dan dua tautan bawah boleh membungkus.

**Tidak berubah:** seluruh alur, termasuk pemilih organization tanpa mengetik ulang, penolakan Employee lewat
`dashboard_not_available_for_role` yang tampil apa adanya (sudah dijaga `api-client.test.ts`), kesalahan per field,
password minimal 12, dua cabang terima undangan, dan tiga keadaan verifikasi.

**Verifikasi:** typecheck, lint, 268 test, build. Visual tanpa sesi di 360 dan 1024: `/login`, `/register`,
`/forgot-password`, `/verify-email` (token palsu → keadaan gagal), `/invitations/accept` (token palsu → keadaan
tidak valid). `scrollWidth` = lebar layar. **Alur yang butuh password tidak dijalankan di browser** oleh agent;
logikanya tidak disentuh.

**Temuan #159, pengecekan ulang:** `grep "oklch(0\.[0-9]* 0 0)"` di `(protected)` dan `(auth)` tetap kosong.

---

## #169 — Laporan API: tren, sumber, alasan kalah + filter anggota & sumber

**Yang berubah (`crm_be/internal/metrics`, tanpa migration):** `entity.go` (`Filter.Assignee`/`Source`, `Trend`,
`SourceMetric`, `LostReasonMetric`, `conversionRate`), `port.go` (3 method), `repository_postgres.go` (3 query +
`leadFilterConditions`), `usecase.go` (validasi rentang tren, pemilihan bucket), `handler_http.go` (3 rute + parsing
filter). Test baru: `repository_reports_test.go`, `handler_reports_test.go`, dan tambahan di `usecase_unit_test.go`.

- **Satu definisi conversion rate.** Rumus `won ÷ (total − spam − unqualified)` diekstrak ke `conversionRate()` dan
  dipakai `/summary` **dan** `/sources`. Dua tempat menghitung rasio yang sama dengan cara berbeda adalah bug yang
  menunggu dilaporkan pelanggan (TD §2.3).
- **Filter bersama** di satu fungsi `leadFilterConditions` (dulu `leadRangeConditions`), jadi kelima endpoint
  memfilter dengan cara yang identik. Nilai tak valid diabaikan, sama dengan aturan lama. `source` divalidasi terhadap
  daftar yang sama dengan `CHECK` database.
- **Filter anggota di `/employees` juga mempersempit barisnya.** Tanpa itu, anggota lain tetap tampil dengan angka
  nol, yang terbaca sebagai "mereka tidak punya apa-apa", bukan "tidak ditampilkan".
- **Tren:** `generate_series` atas bucket di `organizations.timezone`, lalu `LEFT JOIN` lead. Hari kosong ikut
  terkirim, dan minggu mulai Senin. Validasi rentang ada di **usecase**, bukan handler: aturan "maksimal 366 hari"
  adalah batas perlindungan query, bukan bentuk request.
- Kode detail validasi memakai kosakata yang sudah ada (`required`, `invalid_value`). Tidak ada kode baru.

**Menyimpang dari TD:** `converted_count` → **`won_count`** (sudah dikoreksi di tempat, `td.md` §2.3). Alasannya:
yang dihitung adalah status Menang, sama dengan `/summary`.

**Untuk #171 (layar Laporan):** `from`/`to` adalah instan, dan bucket adalah hari organization yang **disentuh**
rentang. Terbukti lewat `curl`: `from=20 Sep 00:00Z` s/d `to=26 Sep 23:59Z` pada organization WIB mengembalikan
**8** titik (20–27 Sep), karena `26 Sep 23:59Z` = `27 Sep 06:59 WIB`. Layar harus mengirim batas periode dalam waktu
organization, bukan UTC. Dicatat di `api.md`.

**Verifikasi:**
- `go test -race ./...` seluruh backend lolos. `golangci-lint`: 0 issues.
- Unit (tanpa Docker): `from`/`to` wajib, rentang mundur dan > 366 hari ditolak **tanpa** menyentuh repository,
  bucket 0/45 hari → day dan 46/366 → week, Employee ditolak di ketiganya, Owner/Admin/Manager diizinkan.
- Repository (Postgres asli): hari kosong ikut terkirim; **zona waktu Asia/Jayapura** (16:00Z 15 Jan terhitung 16 Jan);
  minggu mulai Senin; 4 sumber/6 alasan termasuk nol; spam keluar dari penyebut; conversion `nil` bila penyebut nol;
  filter anggota dan sumber di `/summary` dan `/employees`; **isolasi tenant** untuk ketiga query baru, dan UUID
  anggota dari tenant lain menghasilkan nol.
- **Test zona waktu diuji mutasi:** pengelompokan diganti sementara ke `AT TIME ZONE 'UTC'`, test gagal dengan pesan
  "bucketed in UTC", lalu kode dikembalikan.
- Handler: tanpa rentang → `400 validation_failed` dengan dua detail; tanggal keluar sebagai `YYYY-MM-DD`; `source`
  tak dikenal diabaikan; Employee → `403` di ketiganya.
- `curl` terhadap container API yang dibangun ulang, dengan data uji lokal: ketiga endpoint menjawab sesuai bentuk di
  `api.md`.

---

## #170 — Laporan API: waktu respons & tugas

**Yang berubah (`crm_be/internal/metrics`, tanpa migration):** `ResponseTimes` dan `Tasks` di entity, port, repository,
usecase, dan handler. Test baru di `repository_response_tasks_test.go`, plus tambahan di `handler_reports_test.go`
dan `usecase_unit_test.go`.

- **Waktu respons memakai subquery sentuhan pertama yang sama dengan `/employees`** (`MIN(activities.created_at)
  WHERE type <> 'lead_created'`). Satu arti "respons" di seluruh paket. Median memakai `percentile_cont(0.5)`, yang
  mengabaikan NULL, jadi median hanya atas lead yang sudah disentuh dan `NULL` bila tidak ada.
- **`never_touched` bucket sendiri**, bukan dilebur ke "> 1 hari". Menyembunyikannya membuat layar memuji tim yang
  mendiamkan lead (TD §2.5).
- **Batas bucket inklusif di bawah:** tepat 1 jam → `1h_4h`, tepat 24 jam → `gt_24h`, sesuai rencana test TD §7.
- **Tugas:** `completed_count` mengikuti rentang (`completed_at`). `overdue_count` adalah **snapshot `now()`**,
  penyimpangan yang disengaja karena tidak ada riwayat keterlambatan (TD §2.6). Tugas yang dihapus dan tugas pada
  **lead yang dihapus** tidak dihitung, karena tugas seperti itu tidak bisa dibuka dari mana pun. Poin kedua ini
  tambahan terhadap TD, yang hanya menyebut `tasks.deleted_at`. Filter `source` lewat lead tugas, dan `assigned_to`
  mempersempit baris seperti `/employees`.

**Menyimpang dari TD:** hanya tambahan "lead yang dihapus tidak dihitung" di atas. Nama field sesuai TD.

**Untuk #171 (terlihat lewat `curl` dengan data uji):** tugas **tanpa penanggung jawab** tidak muncul di `/tasks`,
karena bentuknya per anggota (TD §2.6). Tugas "Telepon ulang…" yang terlambat dan tidak ditugaskan ke siapa pun hilang
dari blok ini. Layar Laporan harus menyebut bahwa blok ini "per anggota", atau pemilik produk memutuskan baris
"Tanpa penanggung jawab". Belum dibangun, karena itu keputusan tampilan, bukan kekurangan API. Dicatat di
`docs/issues/170-report-tasks.md`.

**Verifikasi:**
- `go test -race ./...`: 31 paket lolos. `golangci-lint`: 0 issues.
- Repository (Postgres asli): lima batas bucket termasuk tepat 1 j/4 j/24 j; `lead_created` saja tidak dihitung sebagai
  sentuhan; median 9000 s atas empat lead tersentuh; median `nil` bila tak ada; tugas selesai di dalam/luar rentang,
  terlambat, tanpa jatuh tempo, terhapus, dan pada lead terhapus; filter sumber/anggota; **isolasi tenant** untuk
  kedua query, dan UUID anggota dari tenant lain menghasilkan nol baris.
- **Batas bucket diuji mutasi:** `secs < 3600` diganti sementara `<= 3600`, dan test gagal (lead tepat 1 jam terhitung
  dua kali).
- Unit: Employee ditolak dan Owner/Admin/Manager diizinkan di keduanya. Handler: bentuk respons dan 403 Employee.
