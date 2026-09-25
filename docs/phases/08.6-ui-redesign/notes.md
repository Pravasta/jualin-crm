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
