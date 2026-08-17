# ADR-008 — Delivery Workflow: PRD → TD → Issue → Branch → PR

> **Status:** ✅ Accepted — 17 Agustus 2026
> **Prosedur operasional:** [`docs/workflow.md`](../workflow.md)
> **Mengubah:** `architecture/freeze.md` bagian 6 — `docs/features/` diganti `docs/phases/`

## Konteks

Freeze menetapkan "1 feature = 1 session" dan struktur dokumentasi `docs/features/<f>/{spec,notes}.md`, tetapi tidak menetapkan **bagaimana pekerjaan mengalir dari rencana ke kode yang ter-merge**.

Pengerjaan akan melibatkan junior programmer dan agent. Tanpa alur yang tegas, tiga hal akan terjadi: pekerjaan langsung masuk `main`, dokumentasi tertinggal di belakang kode, dan tidak ada titik review yang konsisten.

## Keputusan

### 1. Alur per phase

```
PHASE → PRD → TD → Issues → (siklus per issue) → Phase selesai
```

PRD (apa & kenapa) dan TD (bagaimana) ditulis **sekali per phase**, bukan per issue. TD hanya memuat *delta* — ia tidak mengulang freeze.

### 2. Satu issue = satu session = satu PR

Konsisten dengan aturan "1 feature = 1 session" yang sudah ada. Issue besar dipecah menjadi checklist **di dalam body issue**, bukan menjadi issue terpisah.

Alasan: PR yang memetakan tepat satu issue membuat review punya batas yang jelas, dan riwayat `main` terbaca sebagai urutan keputusan.

### 3. `docs/phases/` menggantikan `docs/features/`

```
docs/phases/<NN>-<slug>/{prd,td,issues,notes}.md
```

Unit kerja sekarang adalah **issue di dalam phase**, bukan "feature" yang berdiri sendiri. Folder per-feature akan memecah PRD dan TD yang sebenarnya berlaku untuk satu phase utuh.

`notes.md` satu per phase, ditambah satu bagian per issue — bukan satu file per issue. Lebih sedikit berkas, lebih kecil kemungkinan terbengkalai.

### 4. Status hidup di GitHub, bukan di docs

`issues.md` adalah **indeks tanpa kolom status**.

Alasan: status berubah setiap hari. Kolom status di dokumen akan basi dalam hitungan hari dan menyesatkan session berikutnya — persis kegagalan yang ingin dicegah oleh seluruh sistem dokumentasi ini.

| Artefak | Sumber kebenaran |
|---|---|
| Definisi pekerjaan (scope, acceptance) | `docs/phases/*` |
| Status pekerjaan (open/closed/assigned) | GitHub Issues |
| Riwayat keputusan implementasi | `docs/phases/*/notes.md` |

### 5. Agent berhenti total setelah membuka PR

Agent **tidak pernah** menjalankan `gh pr merge`, `git merge` ke main, atau `git push origin main`.

Merge, penghapusan branch remote, dan keputusan akhir sepenuhnya di tangan pemilik repository. Agent melanjutkan hanya setelah menerima konfirmasi eksplisit.

### 6. PR memakai `Refs #N`, bukan `Closes #N`

**Ini konsekuensi teknis, bukan preferensi gaya.**

GitHub otomatis menutup issue saat PR yang memuat `Closes` / `Fixes` / `Resolves` ter-merge. Itu akan menutup issue **sebelum** agent sempat memverifikasi merge, membersihkan branch lokal, dan mencatat penutupan — merusak Fase 5 secara diam-diam.

Penutupan issue selalu manual oleh agent, setelah verifikasi.

### 7. Dokumentasi ikut di dalam PR

`notes.md`, `STATUS.md`, dan perubahan `architecture/*` di-commit **di branch yang sama** dengan kodenya.

Alasan: dokumentasi yang di-commit setelah merge tidak pernah ikut direview, dan menuntut commit langsung ke `main` — yang dilarang oleh keputusan #5.

### 8. `git branch -d`, tidak pernah `-D`

`-d` menolak menghapus branch yang commit-nya belum masuk `main`. Bila verifikasi merge luput, git sendiri yang menahan dan pekerjaan tidak hilang. `-D` membuang jaring pengaman itu.

## Konsekuensi

**Positif:** setiap perubahan melewati review · riwayat `main` bersih dan terbaca · dokumentasi tidak bisa tertinggal karena ia bagian dari PR · pembagian tanggung jawab manusia/agent tidak ambigu

**Negatif:** setiap pekerjaan menanggung overhead branch + PR + tunggu review · phase tidak bisa dimulai sebelum PRD & TD ditulis

**Mitigasi:** issue berukuran satu session menjaga siklus tetap pendek. PRD & TD per phase, bukan per issue, sehingga biaya persiapan dibayar sekali.

**Risiko yang harus dijaga:** akan ada godaan melewati PR untuk perubahan "kecil" — satu typo, satu baris di STATUS.md. **Jangan.** Pengecualian pertama akan menjadi pengecualian berikutnya, dan `main` berhenti merefleksikan apa yang pernah direview.

## Kapan dievaluasi ulang

Bila ternyata ada kelas perubahan yang benar-benar tidak layak melewati PR — misalnya perbaikan typo dokumentasi yang sering — buat aturan **eksplisit dan sempit** untuknya lewat ADR baru. Jangan melonggarkannya secara diam-diam per kasus.
