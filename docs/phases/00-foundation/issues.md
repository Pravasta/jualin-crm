# Phase 0 — Foundation · Issues

> Indeks pekerjaan. **Tanpa kolom status** — status hidup di GitHub ([ADR-008](../../decisions/ADR-008-delivery-workflow.md)).
>
> Status terkini: `gh issue list --milestone "Phase 0 — Foundation"`

**Milestone:** [Phase 0 — Foundation](https://github.com/Pravasta/jualin-crm/milestone/1)

---

## Daftar

| # | Judul | Cakupan | TD |
|---|---|---|---|
| [1](https://github.com/Pravasta/jualin-crm/issues/1) | Project skeleton: config, logging, error, health, CI | Go module, struktur paket, config ter-validasi saat boot, slog + request ID, error envelope, `/health`, Makefile, CI | §2–§7, §12–§14 |
| [2](https://github.com/Pravasta/jualin-crm/issues/2) | Database, Docker Compose, dan migration tooling | Dockerfile, compose + PostgreSQL 17, pgxpool, `db.InTx`, goose, `0001_baseline`, `/health/ready` | §8–§10 |
| [3](https://github.com/Pravasta/jualin-crm/issues/3) | Test harness terhadap PostgreSQL asli | testcontainers, `NewTestDB`, test migration round-trip, config, error mapping, health | §11, §12, §15 |

---

## Urutan

Ketiganya **berurutan**, tidak bisa diparalelkan:

```
#1 skeleton ──► #2 database ──► #3 test harness
```

| Dependensi | Alasan |
|---|---|
| #2 → #1 | Butuh config, logger, dan kerangka HTTP |
| #3 → #2 | Butuh database dan migration untuk diuji |

**CI sengaja masuk di #1**, bukan di akhir, supaya PR #2 dan #3 sudah tergerbang lint/test/build sejak awal.

---

## Cakupan per issue — batas yang mengikat

| Issue | Berhenti di |
|---|---|
| #1 | Belum menyentuh database sama sekali. `/health` tidak melakukan ping. |
| #2 | Belum ada tabel domain. `0001` **hanya** fungsi `set_updated_at()`. |
| #3 | Belum ada harness isolasi tenant — belum ada tabel tenant-scoped untuk diuji. |

Yang di luar batas ini ada di [`prd.md`](./prd.md) bagian *Di luar cakupan*, dan bersifat mengikat.

---

## Setelah ketiganya selesai

Phase 0 dinyatakan tutup bila **7 acceptance criteria** di [`prd.md`](./prd.md) terpenuhi. Lalu:

1. `docs/STATUS.md` — Phase 0 ✅, utang teknis dicatat
2. `docs/architecture/overview.md` dibuat (TD §16)
3. Buka **Phase 1 — Auth & Organization**: tulis PRD + TD, pecah issue
