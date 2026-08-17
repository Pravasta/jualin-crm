# Product Scope

> Sumber: `docs/product/decisions.md` §9 + `docs/architecture/freeze.md` bagian 2–3
> Gunakan dokumen ini untuk menjawab *"apakah fitur ini masuk?"* tanpa membuka freeze.

---

## Uji dua lapis untuk setiap fitur baru

**Lapis 1 — relevansi domain:**

> Apakah fitur ini membantu customer **menangkap**, **mengelola**, **mendistribusikan**, atau **mengkonversi** lead?

**Lapis 2 — struktur biaya:**

> Apakah fitur ini memperkenalkan **kelas biaya baru**? (pengiriman email massal, penyimpanan media, pemrosesan dokumen, penyimpanan percakapan)

Bila lolos lapis 1 **tapi gagal lapis 2**, fitur itu **melanggar strategi price-war** meskipun relevan secara domain. Dua-duanya harus lolos.

---

## Termasuk

| Area | Cakupan |
|---|---|
| Identity & tenancy | User, Organization, Membership, Role, tenant context |
| Lead management | Lead, status, source, notes, assignment |
| Sales execution | Task, Activity, follow-up |
| Conversion | Customer, Deal (tahap lanjut) |
| Lead capture | Direct API, Embedded Form, Inbound Webhook, manual entry |
| Developer surface | API Key, versioning, rate limiting, dokumentasi API |
| Subscription | Plan, limit, usage, status, **referensi** ke payment service eksternal |
| Operational | Audit log, notification, reports dasar |

---

## Tidak termasuk — domain produk lain

```
HRIS · ERP · Accounting · Inventory · Payroll · Attendance ·
Purchasing · Manufacturing · Invoice generation
```

Tidak ada tabel, kode, atau abstraksi untuk ini. Produk Jualin lain akan memiliki repository, backend, database, dan domain sendiri.

---

## ⚠️ Batas yang akan benar-benar diuji

Daftar di atas mudah dipatuhi — tidak ada yang tergoda menambahkan payroll ke CRM. Yang berbahaya adalah permintaan yang **terdengar seperti CRM**.

Putusannya ditetapkan **sebelum** permintaan pertama datang, karena menolak jauh lebih mudah ketika batasnya sudah tertulis sebelum ada wajah yang memintanya.

| Permintaan | Putusan | Alasan |
|---|---|---|
| **Payment gateway / processing** | ⛔ Keluar | Service terpisah sudah ada (decisions §32) |
| **Generate invoice dari Deal** | ⛔ Keluar | Ini Jualin Invoice. **Batas: CRM boleh menyimpan `deal.value`, tidak boleh menerbitkan dokumen invoice.** |
| **Quotation / penawaran ber-PDF** | ⛔ Keluar | Sangat sering diminta pengguna CRM di Indonesia — **antisipasi permintaan ini**. Jalan tengah: Activity bertipe `quotation_sent` + lampiran file. Bukan generator dokumen. |
| **Product / inventory catalog** | ⚠️ Batasi | **Batas: satu daftar nama produk sederhana milik organization.** Tanpa stok, harga bertingkat, SKU, varian, atau gudang. Melewati batas ini = membangun inventory. |
| **Email campaign / blast / nurture** | ⛔ Keluar | Marketing automation — produk berbeda dengan ekonomi berbeda. Merusak struktur biaya price-war. |
| **Omnichannel chat inbox (WA/IG/FB)** | ⛔ Keluar | Godaan terbesar di pasar Indonesia dan **secara diam-diam produk terbesar di daftar ini**. Batas: deeplink `wa.me` + pencatatan Activity. Bukan inbox, bukan penyimpanan percakapan. |
| **Absensi / GPS check-in sales** | ⛔ Keluar | HRIS / field-force management, meski dimintanya lewat mobile app CRM |
| **Target & komisi sales** | ⚠️ Tunda | Report performa per sales ✅ · perhitungan komisi ⛔ (berbatasan payroll) |
| **Custom field pada Lead** | ✅ Masuk, tunda | Sah sebagai CRM. Ditunda karena murah ditambahkan nanti. |

**Bila sebuah permintaan menyentuh baris ⛔ atau ⚠️:** flag sebagai *scope discussion* (Aturan #29). Jangan implementasikan, jangan juga tolak sendiri — bawa ke pemilik produk.

---

## MVP vs ditunda

### Wajib MVP (Phase 0–5)

Identity & tenancy · invitation · penonaktifan membership · RBAC · audit log · subscription minimal · Lead (CRUD, status, source, idempotency, `lead_number`, `version`) · assignment manual · Activity · Task · Customer + konversi · notification in-app · API Key · Public Lead API · Dashboard · Mobile app

### Ditunda

| Fitur | Ke |
|---|---|
| Embedded Form | Phase 6 |
| Webhook (inbound & outbound) | Phase 7 |
| Penegakan limit, usage, upgrade, integrasi payment | Phase 8 |
| Deal, Pipeline entity, reports lanjutan, automation | Phase 9 |
| Organization switcher | Saat ada kebutuhan nyata (ADR-007) |
| Round robin, assignment rule | Setelah manual terbukti dipakai |
| Custom field, Contact, Team, RBAC dinamis, SSO | Saat diminta |
| Import/export CSV | **Naikkan prioritas bila calon pelanggan datang dari spreadsheet — dan biasanya begitu** |

---

## Ekspektasi yang sudah disepakati

**Dashboard Phase 3 tidak punya angka revenue.** Deal ditunda, sehingga metrik yang tersedia hanya: lead masuk per periode, lead per status, lead belum ter-assign, conversion rate (mengecualikan `spam` & `unqualified`), dan performa per employee.

Dicatat agar tidak muncul sebagai kejutan saat Phase 3 dikerjakan.
