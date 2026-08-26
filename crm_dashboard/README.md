# crm_dashboard

Dashboard CRM untuk Owner, Admin, dan Manager. Next.js (App Router) + TypeScript + Tailwind +
shadcn/ui.

Dimulai di **Phase 3 — Owner Dashboard** (issue #31). Cakupan: auth UI · lead list & detail ·
assignment · employee management + undangan + penonaktifan · filter "lead tanpa pemilik aktif" ·
task · konversi ke customer · metrik dasar · settings.

Lihat `docs/architecture/freeze.md` bagian 4 (Phase 3) dan
`docs/phases/03-owner-dashboard/{prd,td}.md`.

## Menjalankan lokal

```bash
npm install
npm run dev
```

Butuh `crm_be` jalan di lokal (lihat `docker-compose.yml` di akar repo) dan `.env.local` berisi
`NEXT_PUBLIC_API_BASE_URL` (contoh di `.env.example`).

## Test

```bash
npm run test
```

## Konvensi

Ikuti `CLAUDE.md` di akar repository — struktur monorepo (ADR-009), bahasa UI Indonesia tanpa
library i18n (keputusan C1), arsitektur klien→API langsung dengan CORS (keputusan C2), shadcn/ui
di-copy ke repo bukan dependensi runtime (keputusan C3).
