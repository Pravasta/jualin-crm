---
name: jualin-issue-log
description: Konvensi mencatat temuan yang perlu ditinjau ulang nanti — deviasi TD/ADR/PRD, keputusan yang belum diukur, kelas bug yang mungkin berulang — ke docs/issues/ selama mengerjakan issue GitHub apa pun di repository ini. Gunakan setiap kali menemukan sesuatu di luar cakupan issue yang sedang dikerjakan yang layak ditinjau ulang sebelum phase ditutup.
---

# Jualin Issue Log — Mencatat Temuan Selama Pengerjaan

> Skill ini menjawab **"ke mana temuan yang bukan bagian dari issue yang sedang dikerjakan ini pergi"**.
> Bukan pengganti `docs/phases/<NN>-<slug>/notes.md` (naratif lengkap per issue, wajib diisi sesuai
> `docs/workflow.md`) atau `docs/STATUS.md` (ringkasan + Utang Teknis) — keduanya tetap wajib. Skill ini
> menambah satu tempat lagi, khusus untuk hal yang **perlu ditinjau ulang secara sadar** sebelum phase
> yang sedang berjalan ditutup.

---

## Kapan dipakai

Selama mengerjakan issue apa pun — bukan hanya saat menutup phase — begitu menemukan salah satu dari
ini **di luar cakupan issue yang sedang dikerjakan**:

- **Ketidakkonsistenan di ADR/TD/freeze** yang ditemukan saat membaca ulang, tapi memperbaiki dokumen
  sumbernya bukan bagian dari issue ini (kode sudah benar dan sudah mendokumentasikan alasannya di
  komentar, tapi dokumen sumber — ADR, TD — belum ikut diperbaiki).
- **Keputusan yang diambil sekarang tapi butuh ditinjau ulang** setelah ada data nyata — angka rate
  limit, ukuran batch, threshold yang eksplisit dicatat "bukan hasil pengukuran".
- **Bug nyata yang ditemukan lewat test**, sudah diperbaiki di PR yang sama, tapi kelasnya bisa berulang
  di paket lain (pola composite FK yang lupa di-seed di fixture test, dst.) — layak diarsipkan sebagai
  pengingat pola, bukan hanya catatan "sudah diperbaiki dan selesai".
- **Deviasi dari teks issue/PRD/TD sendiri** yang ternyata implementasinya benar dan teksnya yang
  keliru — perlu diverifikasi ulang terhadap **PRD**, bukan cuma TD atau checklist issue (lihat contoh
  di bawah), supaya penutup phase tidak salah mengikuti teks yang sudah diketahui salah.
- **Kontrak kabel antar-issue** — nilai literal yang issue ini tetapkan dan issue **berikutnya** harus
  cocokkan persis, di mana ketidakcocokannya gagal **diam-diam** alih-alih melempar error. Preseden:
  `docs/issues/087-form-submit-anti-spam.md` bagian *Kontrak kabel untuk #88* — #87 menetapkan nama
  field literal (`website` untuk honeypot, `form_token`, `cf-turnstile-response`), #88 merendernya, dan
  salah ketik satu nama saja akan membuat setiap submission gagal sebagai "field kosong", bukan sebagai
  kesalahan yang terlihat. Dicatat di #87, dicek dan dikonfirmasi di #88, ditutup dengan bukti di #98.
  **Layak diulang setiap kali dua issue berbagi kontrak yang tidak dijaga compiler** — payload webhook
  Phase 7 adalah kandidat berikutnya.

**Bukan untuk:**

- Hal yang sudah sepenuhnya selesai dan tidak perlu tindakan lanjut apa pun — itu cukup di `notes.md`.
- Status pekerjaan issue itu sendiri ("issue #46 sedang dikerjakan", "menunggu review") — itu GitHub
  Issues (ADR-008). Dokumen di repository ini **tidak pernah** punya kolom status.
- Setiap issue yang berjalan mulus tanpa temuan di luar cakupannya — **jangan buat berkas kosong demi
  kelengkapan**. Kebanyakan issue tidak butuh berkas sama sekali.

---

## Format

Satu berkas per **issue GitHub yang sedang dikerjakan saat temuan itu muncul** — bukan satu berkas per
temuan. Bila issue yang sama menemukan beberapa hal, semuanya masuk satu berkas, dikelompokkan per
kategori.

```
docs/issues/<NNN>-<slug>.md
```

- `<NNN>` — nomor issue GitHub, **zero-padded 3 digit** (`046`, bukan `46`) supaya terurut sequential
  secara leksikografis di direktori.
- `<slug>` — kebab-case pendek dari judul issue, sama pola dengan nama branch (`type/<issue>-<slug>`).

### Templat

```markdown
# Issue #<N> — checklist penutupan phase

> Checklist ringkas, **bukan** catatan status. Status pekerjaan tetap hidup di GitHub Issues
> (ADR-008) — berkas ini hanya mengumpulkan poin yang perlu **dicek ulang saat #<issue penutup phase>**
> menutup phase-nya. Detail lengkap tiap poin ada di `docs/phases/<NN>-<slug>/notes.md` bagian `## #<N>`
> dan PR [#<PR>](https://github.com/Pravasta/jualin-crm/pull/<PR>).

## Deviasi dari TD / ADR

- [ ] <satu kalimat masalah> — <satu-dua kalimat kenapa belum ditindaklanjuti sekarang, dan apa yang
      perlu diputuskan: revisi dokumen sumber, atau catatan errata>

## Keputusan yang perlu dicek ulang

- [ ] <keputusan yang diambil> — <kondisi nyata yang membuatnya perlu ditinjau ulang>

## Bug ditemukan & sudah diperbaiki (tidak perlu tindakan lanjut, dicatat untuk arsip)

- [x] <bug, dan bagaimana ia ketahuan> — <sudah diperbaiki di PR mana>
```

Ketiga bagian **opsional** — tulis hanya bagian yang benar-benar ada isinya. Jangan memaksakan entri
demi mengisi templat; satu bagian terisi dengan satu entri jujur lebih berguna daripada tiga bagian
yang dipaksakan.

**Contoh nyata**: `docs/issues/046-api-key-crud.md` (ADR-004 yang belum diperbaiki, keputusan RBAC yang
perlu dicek ulang saat matriks final ditinjau) dan `docs/issues/047-public-lead-api.md` (deviasi 401
vs. teks "403" di acceptance criteria issue, diverifikasi ulang terhadap PRD Phase 4 — bukan cuma
TD — dan ternyata tidak bertentangan; angka rate limit yang belum diukur).

---

## Menyambungkan ke penutup phase

Tanpa disambungkan, berkas ini terlupakan begitu PR-nya merge. Dua tempat wajib diperbarui begitu
berkas baru dibuat, atau entri baru ditambahkan ke berkas yang sudah ada:

1. `docs/phases/<NN>-<slug>/issues.md` — bagian penutupan phase (biasanya "Setelah semuanya selesai")
   — tambah langkah eksplisit "cek `docs/issues/<NNN>-*.md`" **sebelum** daftar acceptance criteria PRD,
   bukan menggantikannya.
2. `docs/STATUS.md` bagian **Berikutnya**, di baris issue penutup phase — pointer yang sama.

Pola yang sama harus diulang setiap kali membuka issue baru di `docs/issues/` — jangan andalkan
seseorang menemukan folder itu sendiri tanpa penunjuk.

---

## Yang tidak boleh terjadi

- **Jangan tulis status pekerjaan di sini.** Pelanggaran langsung ADR-008. Tulis dalam bentuk lampau
  ("ditemukan", "belum diperbaiki"), bukan status hidup yang bisa basi.
- **Jangan duplikasi isi `notes.md`.** `notes.md` bercerita lengkap kenapa sebuah keputusan diambil;
  berkas di `docs/issues/` hanya menunjuk ke sana (`notes.md` bagian `## #<N>`) dan menyebutkan satu
  kalimat "apa yang masih perlu dicek", bukan menceritakan ulang alasannya.
- **Jangan buat berkas untuk issue yang tidak menemukan apa pun di luar cakupannya sendiri.**

---

## Sebelum menutup PR issue yang sedang dikerjakan

- [ ] Setiap temuan di luar cakupan issue ini sudah masuk `docs/issues/<NNN>-<slug>.md` (buat baru bila
      belum ada berkas untuk issue ini, tambah entri bila sudah ada)
- [ ] Pointer ke berkas itu sudah ada di `issues.md` dan `STATUS.md` milik phase penutup yang relevan
- [ ] Tidak ada entri yang sebenarnya cuma duplikat `notes.md`
- [ ] Tidak ada entri yang sebenarnya status pekerjaan, bukan temuan untuk ditinjau ulang
