# Phase 5 — Employee Mobile · Issues

> Indeks pekerjaan. **Tanpa kolom status** — status hidup di GitHub ([ADR-008](../../decisions/ADR-008-delivery-workflow.md)).
>
> Status terkini: `gh issue list --milestone "Phase 5 — Employee Mobile"`

**Milestone:** [Phase 5 — Employee Mobile](https://github.com/Pravasta/jualin-crm/milestone/6)

---

## Daftar

| # | Judul | Aplikasi | Cakupan | TD |
|---|---|---|---|---|
| [68](https://github.com/Pravasta/jualin-crm/issues/68) | Backend mobile: `device_tokens`, pengiriman FCM, hook setelah commit | `crm_be` | Migration `0006`, `internal/device`, `internal/shared/push`, config `PUSH_*`, kasus isolasi tenant baru. **Tanpa UI** | §8, §9, §13 |
| [69](https://github.com/Pravasta/jualin-crm/issues/69) | Fondasi Flutter: FVM, scaffold, `ApiClient` single-flight, secure storage, login + biometric | `crm_employee` | Pin 3.44.0, struktur `lib/`, klien API, sesi, login, biometric, CI | §2–§6, §13 |
| [70](https://github.com/Pravasta/jualin-crm/issues/70) | Fondasi desain mobile: tema, token, labels, kerangka navigasi | `crm_employee` | Token dari hasil Claude Design (dicek WCAG AA), `labels.dart`, navigasi, login ditata ulang | §3, §11 |
| [71](https://github.com/Pravasta/jualin-crm/issues/71) | My Leads + cache baca offline | `crm_employee` | Layar terpenting. Mekanisme cache lahir di sini bersama pemakai pertamanya | §7 |
| [72](https://github.com/Pravasta/jualin-crm/issues/72) | Detail lead: timeline, telepon/WhatsApp auto-Activity, ubah status, catatan | `crm_employee` | Seluruh aksi tulis mobile | §11 |
| [73](https://github.com/Pravasta/jualin-crm/issues/73) | My Tasks, FCM klien, deeplink | `crm_employee`, `crm_be`, docs | Push sisi klien, tiga keadaan deeplink, dokumentasi. **Penutup phase** | §10, §12.1, §15 |

---

## Urutan

```
#68 backend (tanpa UI) ─────────────────────────────────┐
                                                        ├──► #71 My Leads ──► #72 Detail ──► #73 penutup
#69 fondasi Flutter ──► #70 fondasi desain ─────────────┘
        ▲                        ▲
        │                        └── menunggu hasil Claude Design
        └── tidak menunggu desain
```

| Dependensi | Sifat |
|---|---|
| #69 → #68 | **Bukan prasyarat.** Keduanya boleh paralel — #68 murni Go, #69 murni Flutter, tidak bersinggungan |
| #70 → #69 | **Keras.** Tema butuh aplikasi yang sudah ada |
| #70 → hasil desain | **Keras.** Ini satu-satunya issue yang benar-benar diblokir `design-brief.md` |
| #71 → #70 | **Keras.** Membangun layar sebelum temanya ada berarti menatanya dua kali |
| #72 → #71 | **Keras.** Detail dibuka dari daftar |
| #73 → #68, #72 | **Keras.** Push klien butuh backend-nya (#68) dan layar tujuan deeplink (#72) |

**#68 dan #69 boleh dikerjakan paralel** — dan sebaiknya begitu: keduanya tidak menunggu desain, jadi
pekerjaan tetap jalan sementara Claude Design berjalan.

---

## Batas per issue

| Issue | Berhenti di |
|---|---|
| #68 | Backend siap menerima token & mengirim push. **Belum ada aplikasi** yang mendaftarkannya — diuji `curl` + token dummy |
| #69 | Aplikasi bisa login + biometric. **Tanpa** tema, navigasi, atau daftar lead. Tampilan sengaja seadanya |
| #70 | Tema, navigasi, dan istilah siap dipakai. **Belum ada** layar isi — hanya placeholder bertanggal |
| #71 | Daftar lead jalan, termasuk offline. Menekan lead **belum** membuka detail |
| #72 | Seluruh aksi tulis jalan. **Belum ada** push maupun My Tasks |
| #73 | Phase 5 tutup, GATE terbuka. **Tidak** rilis Play Store, **tidak** mengaktifkan iOS |

Yang di luar batas ini ada di [`prd.md`](./prd.md) bagian *Di luar cakupan*, dan bersifat mengikat.

---

## Kenapa enam issue, dan kenapa fondasi desain punya issue sendiri

Phase ini yang terbesar sejauh ini — `crm_employee/` mulai dari satu berkas `README.md`. Enam issue
membuat tiap PR tetap sebesar satu sesi.

**Issue fondasi desain (#70) sengaja ada sejak phase dibuka.** Di Phase 3 kebutuhan yang sama —
token, label, kerangka aplikasi — tidak dimiliki issue manapun, sehingga muncul sebagai **#40 di luar
rencana** begitu hasil desain masuk. Pelajaran itu diterapkan langsung di sini, bukan diulang.

**#68 sengaja tidak menunggu apa pun.** Ia murni Go dan tanpa UI, jadi ia bisa jalan sejak hari
pertama sementara `design-brief.md` dikerjakan desainer. Pola yang sama seperti #30 membuka Phase 3
dan #46 membuka Phase 4.
