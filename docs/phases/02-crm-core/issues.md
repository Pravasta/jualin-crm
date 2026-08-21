# Phase 2 — CRM Core · Issues

> Indeks pekerjaan. **Tanpa kolom status** — status hidup di GitHub ([ADR-008](../../decisions/ADR-008-delivery-workflow.md)).
>
> Status terkini: `gh issue list --milestone "Phase 2 — CRM Core"`

**Milestone:** [Phase 2 — CRM Core](https://github.com/Pravasta/jualin-crm/milestone/3)

---

## Daftar

| # | Judul | Cakupan | TD |
|---|---|---|---|
| [19](https://github.com/Pravasta/jualin-crm/issues/19) | Schema 0003, repository lead, alokasi `lead_number`, optimistic locking | 4 tabel + `ALTER organizations`, repository lead + visibilitas employee, serialisasi nomor, `version` | §1, §3, §4, §9 |
| [20](https://github.com/Pravasta/jualin-crm/issues/20) | Lead CRUD, transisi status, filter, pagination, idempotency, E.164 | Usecase + HTTP lead, validasi transisi, `Idempotency-Key`, `phone.ToE164` | §4, §5, §6, §7, §8, §8.1, §9 |
| [21](https://github.com/Pravasta/jualin-crm/issues/21) | Activity append-only + auto-log, dan Task | Timeline otomatis dalam transaksi yang sama, task `lead_id NOT NULL` | §1.3, §1.4, §8, §10 |
| [22](https://github.com/Pravasta/jualin-crm/issues/22) | Assignment, notification (`0004`), penutupan kewajiban penonaktifan membership | Assignment + notification atomik, **kewajiban warisan Phase 1** | §2, §8, §11, §13 |
| [23](https://github.com/Pravasta/jualin-crm/issues/23) | Customer, konversi dari lead, kasus `lead` pada harness isolasi | Konversi eksplisit (B9), CRUD customer, **penutup phase** | §1.2, §8, §12, §16, §18 |

---

## Urutan

Kelimanya **berurutan**, tidak bisa diparalelkan:

```
#19 schema & konkurensi ──► #20 lead ──► #21 activity & task ──► #22 assignment & notification ──► #23 customer & harness
```

| Dependensi | Alasan |
|---|---|
| #20 → #19 | Butuh tabel, repository, alokasi nomor, dan optimistic locking |
| #21 → #20 | Activity otomatis dipicu peristiwa lead; task menempel pada lead |
| #22 → #21 | Assignment menghasilkan activity **dan** notification sekaligus |
| #23 → #21 | Konversi menghasilkan activity `lead_converted` |

**Harness isolasi sengaja ditutup di #23**, bukan dimulai di #19 — supaya ia mencakup seluruh endpoint phase ini sekaligus, bukan ditambal empat kali.

---

## Batas per issue

| Issue | Berhenti di |
|---|---|
| #19 | **Tidak ada endpoint sama sekali.** Hanya schema, repository, dan test konkurensi. |
| #20 | Lead berdiri sendiri — belum ada timeline, belum ada task, belum bisa di-assign. |
| #21 | Task ada dan lead punya timeline, tapi assignment belum menghasilkan notification. |
| #22 | Belum ada customer — lead `won` berhenti di situ. |
| #23 | Phase 2 tutup. Dashboard, API publik, dan mobile **tidak** disentuh. |

Yang di luar batas ini ada di [`prd.md`](./prd.md) bagian *Di luar cakupan*, dan bersifat mengikat.

---

## Dua issue yang perlu perhatian ekstra saat review

**#19 — tidak menghasilkan perilaku yang bisa dilihat.** Nilainya seluruhnya ada di test konkurensi: N goroutine membuat lead bersamaan, dan `lead_number` harus 1..N tanpa lubang. Test berurutan akan hijau bahkan bila locking-nya dihapus total — jadi yang layak direview di PR itu adalah **apakah test-nya benar-benar konkuren**, bukan apakah kodenya rapi.

**#22 — menutup kewajiban yang sudah menunggu sejak Phase 1.** Penonaktifan membership dengan lead terbuka **harus menolak secara default**. Diam-diam melepas assignment saat seseorang resign adalah persis kegagalan senyap yang aturan ini ada untuk mencegahnya (freeze 2.3): lead tetap ter-assign ke orang yang tidak bisa login lagi, tidak muncul di "My Leads" siapapun, dan tidak tertangkap filter "belum ter-assign".

---

## Setelah kelimanya selesai

Phase 2 tutup bila **14 acceptance criteria** di [`prd.md`](./prd.md) terpenuhi. Lalu:

1. `authorization.md` diperbarui — matriks Lead/Task/Activity/Customer jadi nyata, aturan #1 terwujud
2. `multi-tenancy.md` — tabel lapis 4: kasus #1 dan #4 dari "belum ada kasus nyata" menjadi ✅
3. `docs/STATUS.md` — Phase 2 ✅, utang teknis (retensi `idempotency_key`) dicatat
4. Buka **Phase 3 — Owner Dashboard**: PRD + TD, pecah issue

> Phase 3 adalah **demo pertama** (freeze bagian 4) — produk bisa dipakai tanpa `curl`. Ia juga phase pertama di luar `crm_be`, sehingga PRD-nya perlu membahas hal yang belum pernah dibahas: setup Next.js, penyimpanan token di sisi client, dan bentuk error yang ditampilkan ke pengguna.
