# Jualin CRM

CRM SaaS **multi-tenant standalone**. Capture → Manage → Assign → Follow-up → Customer → Deal.

Strategi: harga terjangkau. **Biaya infrastruktur per tenant harus tetap rendah** — ini batasan rekayasa, bukan catatan bisnis.

## Di luar cakupan

HRIS · ERP · accounting · inventory · payroll · invoice generation · email campaign · chat inbox (WA/IG/FB) · payment gateway (service terpisah sudah ada).

Produk Jualin lain = repository terpisah. **Jangan buat abstraksi untuk produk yang belum ada.**

Batas yang akan benar-benar diuji ada di `docs/product/scope.md`. Fitur yang menyentuh domain di luar scope → **flag sebagai scope discussion**, jangan diimplementasikan.

## Stack

Go + Gin + PostgreSQL + Docker (monolith) · Next.js (dashboard) · Flutter (mobile)

**sqlc/pgx — bukan ORM.** Tanpa Redis. Tanpa message broker. Tanpa search engine.

## Layering

```
Handler → Service → Repository → PostgreSQL
```

- Handler **tidak** memanggil repository
- Repository **tidak** berisi business logic
- Service **tidak** tahu tentang HTTP
- Otorisasi di **service layer**, bukan di UI
- Interface didefinisikan di sisi consumer

## Aturan yang tidak boleh dilanggar

Ringkasan. Lengkapnya — 35 aturan — di `docs/architecture/freeze.md` bagian 5. **Penomoran di bawah mengikuti freeze**, jangan dinomori ulang.

### Tenancy (#1–#7)

1. Setiap tabel bisnis punya `organization_id` — kecuali `users` dan tabel token global
2. Setiap tabel tenant-scoped punya `UNIQUE (id, organization_id)`
3. FK ke tabel tenant-scoped **selalu composite**: `(x_id, organization_id)`
4. Repository tidak punya method tanpa `TenantContext` sebagai parameter pertama
5. `organization_id` **selalu** dari principal terautentikasi — **tidak pernah** dari body/query/header
6. Resource milik tenant lain → **404**, bukan 403
7. Feature dengan tenant boundary **wajib** punya test isolasi

### Data (#12–#19)

12. PK **UUIDv7**, di-generate aplikasi, tanpa `DEFAULT` di database
13. `timestamptz`, simpan UTC, render di `organizations.timezone`
14. Uang `numeric(15,2)` + kolom `currency`. Tidak pernah float.
15. Enum = `text` + `CHECK`, bukan tipe `ENUM` PostgreSQL
16. Index tenant-aware **selalu** berawalan `organization_id`
17. JSONB hanya dengan alasan tertulis di migration
18. Soft delete (`deleted_at`) untuk entity bisnis. Activity & audit log tidak pernah dihapus.
19. Email lowercase, ditegakkan `CHECK (email = lower(email))`

### Keamanan (#20–#26)

20. Password **argon2id**. API key **SHA-256** + `subtle.ConstantTimeCompare`.
21. Raw secret hanya ditampilkan **sekali**; database menyimpan hash
23. API key **tidak pernah** hadir di sisi klien
24. **User App ≠ API Key** — dashboard & mobile memakai user session, bukan API key
25. Token dashboard di cookie `HttpOnly` — **tidak pernah** `localStorage` ⇒ proteksi CSRF wajib
26. Jangan pernah log: password, raw API key, token, payload lead lengkap

### API, transaksi & konkurensi (#32–#35)

32. Efek samping eksternal (email, HTTP) **tidak pernah** di dalam transaksi database
33. Payload JSON: `snake_case`, ISO 8601 UTC `Z`, envelope `{data, meta}`, error `{code, message}`
34. Endpoint pengirim email wajib rate-limited **per alamat email dan per IP**
35. `leads` & `tasks` memakai optimistic locking (`version`) — konflik → **409**, jangan menimpa

## Anti-overengineering (#27–#29)

- Bila solusi sederhana sudah cukup, **pakai itu**
- Abstraksi hanya setelah ada implementasi kedua yang **nyata**
- Jangan buat tabel, modul, atau folder untuk kebutuhan yang belum ada
- Buat modul **hanya saat fiturnya dikerjakan** — folder kosong adalah utang yang terlihat seperti kemajuan

## Workflow — 1 issue = 1 session = 1 PR

Prosedur lengkap: `docs/workflow.md` · Alasan: `docs/decisions/ADR-008-delivery-workflow.md`

**Awal session:**

```
CLAUDE.md → docs/STATUS.md → docs/workflow.md → skill →
docs/architecture/* yang relevan → docs/phases/<NN>-<slug>/{prd,td}.md →
issue GitHub → kode berdekatan → rencana → persetujuan → implementasi
```

`docs/architecture/multi-tenancy.md` hampir selalu relevan.

**Siklus pengerjaan:**

```
1. git checkout main && git pull && git switch -c <type>/<issue>-<slug>
2. Implementasi + test + update notes.md/STATUS.md  ← dokumentasi DI DALAM PR
3. Commit (Conventional Commits, Inggris) → push
4. gh pr create --base main   dengan "Refs #N"  ← BUKAN "Closes #N"
5. BERHENTI. Laporkan URL PR.
   ── manusia: review → merge → hapus branch remote ──
6. Setelah dikonfirmasi merge: verifikasi → git branch -d → gh issue close
```

**⛔ Agent tidak pernah:** `gh pr merge` · `git merge` ke main · `git push origin main` · menutup issue sebelum dikonfirmasi merge

**Kenapa `Refs`, bukan `Closes`:** kata kunci penutup membuat GitHub menutup issue otomatis saat merge — sebelum branch lokal dibersihkan dan merge diverifikasi.

**Jangan baca seluruh dokumentasi** bila tidak relevan dengan issue yang sedang dikerjakan.

## Source of truth (#30)

```
Kode & migration
  > docs/architecture/freeze.md
  > docs/architecture/*
  > docs/phases/*
  > docs/decisions/ADR-*
  > docs/brainstorming/*   (arsip, bukan acuan)
```

**Status pekerjaan hidup di GitHub Issues, bukan di docs.** Dokumen tidak pernah punya kolom status.

Bila dokumentasi bertentangan dengan kode: **laporkan**. Jangan diam-diam mengubah salah satunya.

**Perubahan arsitektur hanya melalui ADR baru.** Freeze tidak diubah tanpa catatan.
