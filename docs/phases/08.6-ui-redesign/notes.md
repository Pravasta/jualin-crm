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
