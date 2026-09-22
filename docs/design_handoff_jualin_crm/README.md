# Handoff: Jualin CRM — Web Dashboard + Token Desain

## Overview
Prototipe interaktif untuk Jualin CRM: dashboard web (10 halaman) plus satu eksplorasi tampilan mobile Employee, dan dokumen token desain yang jadi acuan visual seluruhnya. Dibangun untuk memvalidasi alur (lead → customer, tugas, laporan, tim, integrasi, langganan) dan sistem visual sebelum diimplementasikan di codebase produksi.

## About the Design Files
File di `screens/` adalah **referensi desain berbasis HTML** — prototipe yang menunjukkan tampilan dan perilaku yang diinginkan, bukan kode produksi untuk disalin langsung. Tugasnya adalah **membangun ulang desain ini di environment codebase target** (React, Vue, Flutter/native, dst.) menggunakan pola dan library yang sudah ada di sana — atau, bila belum ada environment, pilih framework yang paling sesuai lalu implementasikan di sana.

Setiap file `.dc.html` adalah halaman mandiri (bisa dibuka langsung di browser). Navigasi antar halaman memakai tautan `<a href="Nama Halaman.dc.html">` relatif — bukan router SPA. State tiap halaman (filter, dialog, toggle) hidup sendiri-sendiri di masing-masing file; tidak ada state global yang dibagi antar halaman selain lewat parameter URL (`?status=`, `?owner=`, `?lead=`, `?id=`).

## Fidelity
**High-fidelity.** Warna, tipografi, spacing, radius, dan interaksi sudah final sesuai Token Desain — rekonstruksi sebaiknya pixel-perfect terhadap file-file ini.

## Screens / Views

1. **Token Desain Jualin.dc.html** — Referensi visual: palet warna (oklch + hex), 8 badge status lead dengan 3 opsi treatment, skala mobile kontras tinggi (≥7:1), tipografi (Plus Jakarta Sans + JetBrains Mono), spacing/radius/shadow, dan peta token shadcn↔Flutter Material 3.
2. **Beranda.dc.html** — Ringkasan: kartu KPI (lead masuk, belum ter-assign, conversion rate, menang), chip jumlah per status, performa anggota. Semua kartu/baris bisa diklik ke Daftar Lead terfilter.
3. **Daftar Lead.dc.html** — Tabel (desktop/tablet) / kartu (HP) dengan pencarian, filter status/sumber/pemilik/periode, chip "Tanpa pemilik aktif", dialog Lead baru dengan validasi + konfirmasi lunak saat kontak kosong, keadaan Normal/Memuat/Kosong.
4. **Detail Lead.dc.html** — Header lead, stepper status 5 tahap dengan aturan transisi (lanjut/kembali/tutup/buka kembali), alasan kalah wajib, catatan, timeline, penugasan, tugas, konversi ke Customer, hapus.
5. **Daftar Customer.dc.html** — Tabel/kartu customer hasil konversi, pencarian.
6. **Detail Customer.dc.html** — Profil customer, nilai kontrak, tautan balik ke lead asal, riwayat pembelian, catatan, timeline.
7. **Laporan.dc.html** — 8 blok: ringkasan, distribusi status, performa anggota, tren, sumber lead, alasan kalah, waktu respons, tugas selesai/lewat jatuh tempo. Kartu berlabel [Tersedia]/[API baru]. Keadaan Normal/Data sedikit/Kosong.
8. **Tugas.dc.html** — Semua tugas lintas lead/customer, tab Semua/Tugas saya/Terlambat, checkbox selesai, tautan ke entitas asal.
9. **Tim.dc.html** — Daftar anggota, badge peran (Owner/Admin/Sales), tautan ke lead per anggota, dialog undang anggota.
10. **Connect.dc.html** — 6 kartu integrasi (WhatsApp, Formulir, API/Webhook, Instagram, Marketplace, Google Sheets) dengan status Terhubung/Putuskan, dan kunci fitur per tier paket.
11. **Langganan.dc.html** — 3 kotak paket (Starter/Tim/Bisnis) dengan daftar fitur dan tombol upgrade/downgrade langsung, bar pemakaian kuota, riwayat pembayaran.
12. **Pengaturan.dc.html** — Tab Profil bisnis, Notifikasi (toggle), API & Webhook (buat/cabut kunci), Keamanan.
13. **Mobile Employee.dc.html** — Eksplorasi terpisah: bezel Android, 2 layar (Daftar lead / Detail lead) dengan CTA Telepon/WhatsApp fungsional, dibangun di atas skala kontras mobile (≥7:1) dari Token Desain.

### Layout responsif (berlaku di semua halaman dashboard)
- **Desktop (≥1024px)**: sidebar kiri 232px penuh label, konten max-width 1180–1280px.
- **Tablet (768–1023px)**: sidebar 68px ikon-saja, grid ringkasan turun ke 2 kolom.
- **Mobile (<768px)**: sidebar disembunyikan, header atas + bottom nav 5 item + FAB (di Daftar Lead), kartu menggantikan tabel.
- Breakpoint dideteksi lewat `window.innerWidth` di setiap komponen (lihat method `componentDidMount`/`renderVals` masing-masing file).

## Interactions & Behavior
- Semua tombol, chip, filter, toggle, dan dialog di prototipe ini **berfungsi** (state React lokal) — bukan sekadar visual statis.
- Navigasi antar halaman lewat `<a href>` biasa; beberapa tautan membawa query param yang dibaca `componentDidMount` untuk mem-prefilter halaman tujuan (contoh: `Daftar Lead.dc.html?status=won&owner=Budi%20Santoso`).
- Toast konfirmasi (mis. "Catatan ditambahkan") muncul di pojok bawah, auto-hilang ~2.2–2.4 detik.
- Dialog modal (Lead baru, Undang anggota) punya validasi field wajib dan state konfirmasi lunak.
- Tidak ada backend — semua data adalah array/objek statis di dalam masing-masing file (lihat konstanta di atas `class Component extends DCLogic`).

## State Management
Setiap halaman punya `state` React lokal di kelas `Component` masing-masing file (pola `DCLogic`/React class component minus `render()`). Contoh state penting:
- Daftar Lead: `statusFilter` (Set), `unassignedActive`, `sourceFilter`, `ownerFilter`, `keyword`, `dialogOpen`, `softConfirm`.
- Detail Lead: `overrideStatus` (map lead→status), `closeMenuOpen`, `lostReasonOpen`.
- Langganan: `planKey` (paket aktif), diubah langsung oleh tombol Upgrade/Downgrade di kartu paket.
- Connect: `overrides` (map integrasi→connected boolean).

Untuk implementasi produksi, state ini semestinya dipindah ke server/API sungguhan (fetch lead, update status, dst.) — struktur data dummy di tiap file bisa dijadikan acuan bentuk model.

## Design Tokens
Rujukan lengkap ada di `screens/Token Desain Jualin.dc.html` (interaktif, nilai bisa diklik untuk disalin). Ringkasan:
- **Primary (amber)**: `oklch(0.56 0.19 41)` `#ca3c00`; hover `oklch(0.50 0.185 41)` `#b32900`; accent-strong `oklch(0.48 0.17 41)` `#a72b00`.
- **Netral hangat (hue 60)**: background `#fffdfb`, card `#ffffff`, foreground `#1f1915`, muted-foreground `#6a635e`, border `#e2ddd9`, border-strong `#c9c3be`.
- **Destructive**: `oklch(0.505 0.21 27)` `#c0000f`.
- **8 status lead** (masing-masing punya fg + bg tint, kontras ≥4.6:1 dashboard / ≥7:1 mobile) — nilai penuh di §02–§04 Token Desain.
- **Tipografi**: Plus Jakarta Sans (400–800) untuk UI, JetBrains Mono untuk nomor lead/API key/kode.
- **Radius**: `0.5rem` dasar; badge 4–6px, kartu 8–12px, chip/pill 999px.
- **Shadow**: netral hangat (rgba dari `#1f1915`), bukan hitam murni; flat untuk kartu/tabel.

## Assets
Tidak ada aset gambar — semua ikon adalah inline SVG (lucide-style, digambar manual di tiap file). Font dimuat dari Google Fonts (`Plus Jakarta Sans`, `JetBrains Mono`).

`android-frame.jsx` adalah komponen bezel Android generik (starter component) yang dipakai oleh `Mobile Employee.dc.html` — bukan bagian dari desain final, hanya alat bantu pratinjau ukuran/jarak sentuh.

## Files
Semua file desain ada di folder `screens/` ini:
- `Token Desain Jualin.dc.html`
- `Beranda.dc.html`, `Daftar Lead.dc.html`, `Detail Lead.dc.html`
- `Daftar Customer.dc.html`, `Detail Customer.dc.html`
- `Laporan.dc.html`, `Tugas.dc.html`, `Tim.dc.html`
- `Connect.dc.html`, `Langganan.dc.html`, `Pengaturan.dc.html`
- `Mobile Employee.dc.html` (+ `android-frame.jsx` pendukungnya)
- `support.js` — runtime kecil yang dipakai semua file `.dc.html` di atas (jangan dihapus, wajib ada di folder yang sama saat file dibuka di browser).

Setiap `.dc.html` bisa dibuka langsung di browser untuk melihat perilakunya sebelum diimplementasikan ulang.
