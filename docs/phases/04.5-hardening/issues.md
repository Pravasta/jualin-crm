# Phase 4.5 — Hardening · Issues

> Indeks pekerjaan. **Tanpa kolom status** — status hidup di GitHub ([ADR-008](../../decisions/ADR-008-delivery-workflow.md)).
>
> Status terkini: `gh issue list --milestone "Phase 4.5 — Hardening"`

**Milestone:** [Phase 4.5 — Hardening](https://github.com/Pravasta/jualin-crm/milestone/7)

---

## Daftar

| # | Judul | Aplikasi | Cakupan | TD |
|---|---|---|---|---|
| [57](https://github.com/Pravasta/jualin-crm/issues/57) | Trusted proxy: tegakkan Aturan #34 yang selama ini bisa dilewati | `crm_be` | `TRUSTED_PROXIES` + validasi boot, `SetTrustedProxies` di `newRouter`, test anti-spoofing untuk `resend`/`forgot`/`login`. **Akar masalahnya** | §1, §3.1, §3.2 |
| [58](https://github.com/Pravasta/jualin-crm/issues/58) | Eviction map ber-key penyerang + koreksi kategorisasi utang | `crm_be` | Dua generasi di `FixedWindow` & `LoginLimiter`, koreksi `docs/issues/047`, dokumentasi arsitektur. **Penutup phase** | §2, §3.3, §4 |

---

## Urutan

```
#57 trusted proxy ──► #58 eviction + penutup
```

| Dependensi | Sifat |
|---|---|
| #58 → #57 | **Lunak, tapi urutannya penting.** Keduanya bisa dikerjakan terpisah, tapi #57 lebih dulu karena ia yang mengubah laju pertumbuhan map dari *tak terbatas* menjadi *terikat pada IP nyata penyerang*. Mengerjakan #58 lebih dulu berarti membangun batas untuk pertumbuhan yang masih tak terbatas — batasnya tetap terlampaui, hanya lebih lambat. |

**Tidak ada pekerjaan paralel.** Phase ini dua issue, berurutan.

---

## Batas per issue

| Issue | Berhenti di |
|---|---|
| #57 | `ClientIP()` tidak lagi bisa dipalsukan dan Aturan #34 punya test yang mencoba melewatinya. Map **masih** tumbuh tanpa batas — hanya lebih lambat. |
| #58 | Phase 4.5 tutup. Angka rate limit, retensi `idempotency_key`, dan utang tak berhubungan (#1, #2, #22) **tidak** disentuh. |

Yang di luar batas ini ada di [`prd.md`](./prd.md) bagian *Di luar cakupan*, dan bersifat mengikat.

---

## Kenapa hanya dua issue

Phase ini sengaja kecil. Ia bukan phase fitur — ia memperbaiki satu kelas cacat yang ditemukan saat
memeriksa `docs/issues/046` & `047` sebelum Phase 5 dibuka, dan cakupannya dikunci di dua temuan yang
**tidak** butuh data traffic untuk diketahui salah.

Dua poin lain di `docs/issues/047` (angka `PUBLIC_API_RATE_LIMIT`, retensi `idempotency_key` di volume
tinggi) tetap terbuka dan tetap menunggu integrator produksi sungguhan. Menariknya ke sini hanya karena
"sekalian sedang menyentuh rate limit" akan mengulangi persis kesalahan yang phase ini koreksi:
menggabungkan hal-hal yang kebetulan bertetangga di kode, lalu memperlakukan mereka sebagai satu
keputusan.
