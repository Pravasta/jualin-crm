# Phase 1 — Auth & Organization · Issues

> Indeks pekerjaan. **Tanpa kolom status** — status hidup di GitHub ([ADR-008](../../decisions/ADR-008-delivery-workflow.md)).
>
> Status terkini: `gh issue list --milestone "Phase 1 — Auth & Organization"`

**Milestone:** [Phase 1 — Auth & Organization](https://github.com/Pravasta/jualin-crm/milestone/2)

---

## Daftar

| # | Judul | Cakupan | TD |
|---|---|---|---|
| [8](https://github.com/Pravasta/jualin-crm/issues/8) | Schema 0002, tenant context, pola repository, test katalog | 9 tabel + composite FK, `tenant.Context`, repository tenant-scoped, test katalog | §1, §7, §8, §14 |
| [9](https://github.com/Pravasta/jualin-crm/issues/9) | Registrasi atomik, argon2id, dan verifikasi email | Registrasi satu transaksi, password hashing, token verifikasi, `Mailer`/`LogMailer` | §2, §3, §6, §10, §11, §12 |
| [10](https://github.com/Pravasta/jualin-crm/issues/10) | Login, refresh rotation, logout, reset password, CSRF | Access JWT + refresh opaque, rotasi + deteksi reuse, cookie vs bearer, CSRF | §2, §4, §5, §6, §11, §12, §13 |
| [11](https://github.com/Pravasta/jualin-crm/issues/11) | RBAC, invitation, penonaktifan membership, harness isolasi tenant | Otorisasi 4 role, undangan dua cabang, cabut sesi, **harness lapis 4** | §6, §6.1, §9, §12, §14, §17 |

---

## Urutan

Keempatnya **berurutan**, tidak bisa diparalelkan:

```
#8 schema & pola ──► #9 registrasi ──► #10 sesi ──► #11 otorisasi & harness
```

| Dependensi | Alasan |
|---|---|
| #9 → #8 | Butuh tabel, `tenant.Context`, dan pola repository |
| #10 → #9 | Login butuh user yang bisa didaftarkan dan diverifikasi |
| #11 → #10 | Harness isolasi butuh endpoint terautentikasi untuk ditembak |

**Test katalog sengaja masuk di #8**, bukan di akhir — begitu ia ada, setiap tabel baru di #9–#11 (dan seluruh Phase 2+) otomatis ikut ditegakkan tanpa ada yang perlu mengingatnya.

---

## Batas per issue

| Issue | Berhenti di |
|---|---|
| #8 | **Tidak ada endpoint sama sekali.** Hanya schema, pola, dan test struktural. |
| #9 | Belum ada login — user terdaftar dan terverifikasi, tapi belum bisa masuk. |
| #10 | Belum ada otorisasi per-role — semua user terautentikasi setara. |
| #11 | Aturan "reassign lead terbuka" **tidak** dikerjakan — tabel `leads` baru ada di Phase 2 (TD §17). |

Yang di luar batas ini ada di [`prd.md`](./prd.md) bagian *Di luar cakupan*, dan bersifat mengikat.

---

## Setelah keempatnya selesai

Phase 1 tutup bila **11 acceptance criteria** di [`prd.md`](./prd.md) terpenuhi. Lalu:

1. `architecture/authentication.md` dan `authorization.md` dibuat (TD §16)
2. `docs/STATUS.md` — Phase 1 ✅, utang teknis dicatat
3. Buka **Phase 2 — CRM Core**: PRD + TD, pecah issue

---

## Catatan lead time

Provider email sungguhan **tidak** memblokir Phase 1 — `LogMailer` menutup seluruh alur untuk development dan test. Tapi domain + SPF/DKIM/DMARC adalah item lead-time di [`STATUS.md`](../../STATUS.md) yang perlu jalan paralel, karena verifikasi email menggerbangi login di produksi.
