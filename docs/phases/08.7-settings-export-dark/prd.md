# Phase 8.7 — Pengaturan Akun, Pengingat Tugas, Unduh Laporan, Mode Gelap · PRD

> **Apa & kenapa.** Detail teknis di [`td.md`](./td.md).
> Sumber: permintaan pemilik produk setelah Phase 8.6 ditutup (26 September 2026), dengan empat keputusan
> yang diambilnya hari itu (§*Keputusan*).

---

## Dua hal yang harus dilaporkan sebelum apa pun (Aturan #30)

**1. Tiga dari empat permintaan ini sebelumnya sengaja dikeluarkan.** Brief desain 8.6 §15 dan PRD 8.6
*Handoff vs sistem* menolak **ekspor laporan**, **preferensi notifikasi**, dan **tema/mode gelap** untuk putaran
itu. Pemilik produk kini memasukkannya secara sadar. Ini keputusan yang sah dan tidak bertentangan dengan
`scope.md`: *Operational → notification, reports dasar* termasuk cakupan. Yang tetap ditolak, dan tidak dibangun
di sini: **email berkala/ringkasan mingguan** (kelas biaya baru) dan **ganti email** (identitas login).

**2. Nomor phase 8.7 menyimpang dari `freeze.md`**, dengan alasan yang sama seperti 8.5 dan 8.6: ini lanjutan langsung
dari phase sebelumnya, bukan Deal & Pipeline (Phase 9), yang urutannya tidak digeser.

---

## Tujuan

Pengguna bisa **mengurus akunnya sendiri** (nama, password, organization), **mengatur notifikasi**, dan **tidak lagi
kelewatan tugas** karena Jualin mengingatkannya sebelum jatuh tempo, di dashboard dan di HP. Laporan bisa **dibawa
keluar**: diolah di spreadsheet atau dikirim/dicetak. Dashboard bisa dipakai dalam **mode gelap**.

---

## Kebutuhan

| # | Sebagai… | Saya butuh… | Supaya… |
|---|---|---|---|
| 1 | Owner/Admin/Manager | **Mengunduh Laporan** sebagai CSV dan mencetak/menyimpannya sebagai PDF, sesuai filter yang aktif | Saya bisa mengolah angka di spreadsheet dan mengirim ringkasan ke atasan/rekan |
| 2 | Siapa pun | **Mengubah nama** saya | Nama yang salah ketik tidak harus diperbaiki lewat admin |
| 3 | Siapa pun | **Mengganti password** dengan memasukkan password lama | Saya bisa mengamankan akun tanpa lewat "lupa password" |
| 4 | Owner/Admin | **Mengubah nama organization** | Rebranding tidak butuh bantuan |
| 5 | Anggota yang memegang tugas | **Diingatkan** sebelum tugas jatuh tempo, di lonceng dashboard **dan** push HP | Tugas tidak terlewat |
| 6 | Siapa pun | **Menyalakan/mematikan** push notifikasi dan pengingat tugas, dari dashboard **atau** dari aplikasi mobile | Notifikasi sesuai cara saya bekerja. Employee (yang tidak masuk dashboard) juga bisa |
| 7 | Owner/Admin/Manager | **Mode gelap** di dashboard: Terang / Gelap / Ikuti sistem | Nyaman dipakai malam atau di ruangan redup |

---

## Acceptance Criteria

| # | Kriteria |
|---|---|
| 1 | Laporan punya **Unduh CSV** (satu berkas, semua blok, angka sesuai filter aktif, dibuka rapi di Excel dan Google Sheets termasuk huruf Indonesia) dan **Cetak / PDF** (tata letak cetak A4 tanpa navigasi, dengan periode dan filter tertulis di kepala). **Tanpa** beban server dan tanpa library PDF |
| 2 | `PATCH` nama pengguna dan nama organization. Organization hanya Owner/Admin. Validasi per field. Perubahan tercatat di `audit_log` |
| 3 | **Ganti password**: wajib password lama (salah → ditolak dengan pesan umum, dihitung ke rate limit), password baru ≥ 12 karakter, disimpan argon2id (Aturan #20), **sesi lain dicabut** (sesi saat ini tetap). Tidak pernah dicatat di log (Aturan #26) |
| 4 | **Preferensi notifikasi per membership**: *push ke HP* dan *pengingat tugas jatuh tempo*. Default **menyala**. Bisa diubah dari dashboard **dan** mobile |
| 5 | **Pengingat jatuh tempo**: satu kali per tugas, dikirim saat tugas terbuka akan jatuh tempo dalam **24 jam**. Isinya notifikasi in-app + push (bila push menyala). **Aman untuk beberapa instance API** (tidak ada pengingat ganda). **Tanpa** broker, cron eksternal, atau email. Menghormati preferensi penerima |
| 6 | Push `lead_assigned`/`task_assigned` yang sudah ada **menghormati** preferensi push. Notifikasi in-app tetap tercatat (lonceng adalah riwayat) |
| 7 | **Mode gelap dashboard**: token `.dark` dirancang ulang (bukan bawaan shadcn), **setiap pasangan teks/latar dihitung** (≥ 4.5:1), warna status dan grafik punya varian gelap yang lolos, dan pilihan Terang/Gelap/Sistem tersimpan per perangkat **tanpa kedip** saat halaman dimuat |
| 8 | Setiap endpoint baru punya **test isolasi tenant** dan otorisasi per role. Seluruh test lama tetap lolos. Sapuan tata letak (guliran + simetri celah) lolos di **kedua** tema |

---

## Keputusan (pemilik produk, 26 September 2026)

| # | Pertanyaan | Keputusan |
|---|---|---|
| K1 | Format unduh Laporan | **CSV dan PDF** |
| K2 | Apa yang bisa diubah di profil | **Nama + password** (+ nama organization untuk Owner/Admin). **Email tidak**, karena email adalah identitas login dan menggantinya butuh alur verifikasi sendiri |
| K3 | Saluran pengingat | **In-app + push HP**. **Tanpa email** (kelas biaya baru, strategi biaya rendah) |
| K4 | "Ganti warna biar lebih hidup" | **Mode gelap** |

---

## Di luar cakupan

| Yang tidak dikerjakan | Alasan / ke mana |
|---|---|
| Ganti email | K2. Butuh verifikasi ulang, pembatalan sesi, dan rate limit tersendiri. Bila diminta: issue sendiri |
| Email pengingat / ringkasan mingguan | K3. Kelas biaya baru (`scope.md` lapis 2) |
| Mode gelap di **aplikasi mobile** | K4 ditujukan ke dashboard. Mobile punya skala kontras sendiri (≥7:1) yang harus dirancang ulang untuk latar gelap. Bila diminta: phase sendiri |
| Pilihan jam pengingat, pengingat berulang, pengingat untuk tugas yang sudah lewat | Satu pengingat "24 jam sebelum" dulu. Ditambah bila terbukti kurang (Aturan #27) |
| Ekspor Excel `.xlsx` asli, laporan terjadwal | CSV dibuka Excel. `.xlsx` butuh library. Laporan terjadwal berarti email |

---

## Dependensi

Phase 8.6 selesai. `notification` + `device` (push FCM) dan pola worker di dalam proses (`webhook.Worker`) sudah ada,
dan pengingat mengikuti pola worker itu.
