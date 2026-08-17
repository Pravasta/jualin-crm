# Delivery Workflow

> **Prosedur operasional pengerjaan Jualin CRM.** Keputusan dan alasannya di [ADR-008](./decisions/ADR-008-delivery-workflow.md).
>
> Dibaca di **awal setiap session pengerjaan**. Setiap langkah di bawah bersifat mengikat.

---

## Peta besar

```
PHASE
  ↓
PRD  (apa & kenapa)  ──┐
TD   (bagaimana)     ──┼──►  docs/phases/<NN>-<slug>/
ISSUES (pecahan)     ──┘         ↓
                            GitHub Issues
                                 ↓
              ┌──────────────────┴──────────────────┐
              │        SIKLUS PER ISSUE             │
              │  branch → kerjakan → PR → review    │
              │  → merge (manusia) → tutup (agent)  │
              └─────────────────────────────────────┘
```

**Satu issue = satu session = satu PR.** Tanpa pengecualian.

---

## Bagian 1 — Membuka sebuah Phase

Dilakukan **sekali** di awal setiap phase, sebelum ada issue apapun.

### 1.1 Tulis PRD

`docs/phases/<NN>-<slug>/prd.md` — **apa & kenapa**, tanpa detail teknis.

| Bagian | Isi |
|---|---|
| Tujuan | Satu paragraf: kondisi apa yang berubah setelah phase ini |
| Kebutuhan | Daftar kebutuhan pengguna, bukan daftar tabel |
| Acceptance criteria | Kondisi terukur untuk menyatakan phase selesai |
| Di luar cakupan | Yang **sengaja** tidak dikerjakan di phase ini, beserta ke phase mana |
| Dependensi | Phase / keputusan yang harus selesai lebih dulu |

### 1.2 Tulis TD

`docs/phases/<NN>-<slug>/td.md` — **bagaimana**.

| Bagian | Isi |
|---|---|
| Delta schema | Tabel & kolom baru, migration mana |
| Endpoint | Path, method, request/response, principal yang diterima |
| Alur penting | Sequence untuk operasi yang melibatkan >1 modul |
| Error baru | Tambahan katalog di `architecture/api.md` |
| Otorisasi | Role mana boleh apa |
| Rencana test | Termasuk kasus isolasi tenant |
| Risiko teknis | Yang diketahui sejak awal |

> **TD tidak mengulang `architecture/freeze.md`.** Ia hanya berisi *delta* untuk phase ini. Bila sebuah aturan sudah ada di freeze, TD cukup merujuknya.

### 1.3 Pecah menjadi issue

`docs/phases/<NN>-<slug>/issues.md` — indeks, **tanpa kolom status**.

```markdown
| # | Judul | Cakupan | TD |
|---|---|---|---|
| 12 | Lead core | model, CRUD, status, idempotency | §3.1–3.4 |
| 13 | Activity & Task | append-only timeline, task CRUD | §3.5 |
```

> **Status tidak pernah ditulis di docs.** Status hidup hanya di GitHub. Kolom status di dokumen akan basi dalam hitungan hari dan menyesatkan session berikutnya.

Lalu buat issue-nya di GitHub:

```bash
gh issue create \
  --title "Lead core" \
  --milestone "Phase 2 — CRM Core" \
  --label backend \
  --body-file /tmp/issue-body.md
```

Isi body issue mengikuti template `.github/ISSUE_TEMPLATE/task.md`: cakupan, acceptance criteria, tautan ke bagian TD, dan yang **tidak** termasuk.

Setelah issue dibuat, isi nomornya ke `issues.md`, commit lewat PR biasa.

---

## Bagian 2 — Siklus per Issue

### Fase 1 — Persiapan *(agent)*

```bash
git checkout main
git pull origin main
git switch -c <type>/<issue>-<slug>
```

**Branch selalu dari `main` yang baru di-pull.** Bukan dari branch lain, bukan dari main yang sudah basi.

**Format branch:**

```
feat/12-lead-core
fix/28-refresh-token-rotation
chore/1-foundation
docs/5-api-conventions
```

Tipe: `feat` · `fix` · `chore` · `docs` · `refactor` · `test`

### Fase 2 — Pengerjaan *(agent)*

1. Baca issue, TD terkait, dan kode yang berdekatan
2. Buat rencana implementasi → **tunggu persetujuan bila perubahan besar**
3. Implementasi + test
4. **Update dokumentasi di dalam branch ini**, bukan setelahnya:
   - `docs/phases/<NN>-<slug>/notes.md` — realitas implementasi, penyimpangan dari TD beserta alasannya, utang teknis
   - `docs/STATUS.md` — bila phase selesai atau ada utang teknis baru
   - `docs/architecture/*` — bila ada konvensi yang berubah
   - ADR baru bila ada keputusan arsitektur

```bash
git add -A
git commit    # format di bagian 3
git push -u origin <branch>
```

> **Dokumentasi ikut di dalam PR.** Tidak ada commit ke `main` di luar PR — termasuk update `STATUS.md`.

### Fase 3 — Serah terima *(agent)*

```bash
gh pr create --base main \
  --title "<judul>" \
  --body-file /tmp/pr-body.md
```

Lalu **laporkan URL PR dan berhenti.**

| ⛔ Agent tidak pernah menjalankan |
|---|
| `gh pr merge` |
| `git merge` ke `main` |
| `git push origin main` |
| `git push --force` ke branch manapun yang sudah di-PR |

### Fase 4 — Review *(manusia)*

Anda melakukan:

1. Review PR
2. Merge
3. Hapus branch remote

**Bila ada permintaan perubahan:** kembali ke Fase 2 pada **branch yang sama**. Jangan buat branch atau PR baru.

### Fase 5 — Penutupan *(agent, setelah Anda bilang "sudah saya merge")*

```bash
# 1. Verifikasi — jangan percaya begitu saja
gh pr view <pr> --json state,mergedAt,mergeCommit

# 2. Sinkron & bersihkan lokal
git checkout main
git pull origin main
git branch -d <branch>        # -d, BUKAN -D
git remote prune origin

# 3. Tutup issue
gh issue close <issue> --comment "Selesai lewat #<pr>."
```

**Kenapa `-d`, bukan `-D`:** `-d` menolak menghapus branch yang commit-nya belum masuk `main`. Bila verifikasi luput, git sendiri yang menahan — pekerjaan tidak hilang. `-D` menghapus paksa dan membuang jaring pengaman itu.

**Bila verifikasi menunjukkan PR belum ter-merge:** laporkan, jangan hapus apapun.

---

## Bagian 3 — Konvensi

### Commit — Conventional Commits, Bahasa Inggris

```
<type>(<scope>): <subject>

[body opsional]

Refs #<issue>
```

```
feat(lead): add lead creation endpoint
fix(auth): reject refresh token after family revocation
chore(ci): add golangci-lint step
docs(architecture): document api error catalog
test(tenant): add cross-tenant isolation harness
```

- `type`: `feat` · `fix` · `chore` · `docs` · `refactor` · `test`
- `scope`: nama modul domain (`lead`, `auth`, `membership`, `tenant`, `ci`)
- `subject`: imperatif, huruf kecil, tanpa titik

### Issue & PR — Bahasa Indonesia

Judul PR **sama** dengan judul issue, agar mudah ditelusuri.

### ⚠️ PR memakai `Refs #N`, bukan `Closes #N`

Ini bukan preferensi gaya — ini **wajib**.

> GitHub **otomatis menutup** issue saat PR yang memuat `Closes` / `Fixes` / `Resolves` ter-merge.
>
> Itu merampas Fase 5 dari agent: issue tertutup sebelum ada yang membersihkan branch lokal, memverifikasi merge, atau memperbarui catatan. Alur yang Anda rancang menjadi rusak diam-diam.

Gunakan **`Refs #N`** di body PR dan di footer commit. Penutupan issue selalu manual, di Fase 5.

---

## Bagian 4 — Label & Milestone

| Milestone | = Phase | `Phase 0 — Foundation`, `Phase 1 — Auth & Organization`, … |
|---|---|---|

| Label | Untuk |
|---|---|
| `backend` | Go |
| `dashboard` | Next.js |
| `mobile` | Flutter |
| `infra` | Docker, CI, deployment |
| `docs` | Dokumentasi saja |
| `security` | Menyentuh auth, tenancy, atau kredensial |
| `blocked` | Menunggu keputusan atau issue lain |

Label sengaja sedikit. Ditambah hanya bila ada kebutuhan penyaringan yang nyata.

---

## Bagian 5 — Struktur dokumentasi phase

```
docs/phases/
└── 00-foundation/
    ├── prd.md      apa & kenapa
    ├── td.md       bagaimana
    ├── issues.md   indeks issue (tanpa status)
    └── notes.md    realitas implementasi, ditambah per issue
```

`notes.md` ditambah **satu bagian per issue**:

```markdown
## #12 — Lead core

**Menyimpang dari TD:** ...  **Alasan:** ...
**Utang teknis:** ...
**Catatan untuk session berikutnya:** ...
```

---

## Ringkasan pembagian tanggung jawab

| Langkah | Siapa |
|---|---|
| Tulis PRD, TD, pecah issue | Agent (dengan persetujuan Anda) |
| Buat branch | Agent |
| Implementasi + test + dokumentasi | Agent |
| Buka PR | Agent |
| **Review** | **Anda** |
| **Merge** | **Anda** |
| **Hapus branch remote** | **Anda** |
| Konfirmasi "sudah saya merge" | **Anda** |
| Verifikasi merge, hapus branch lokal, tutup issue | Agent |

**Batas yang tidak boleh dilewati:** agent berhenti total setelah membuka PR, dan tidak melanjutkan apapun sampai Anda menyatakan sudah merge.
