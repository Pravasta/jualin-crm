# Jualin CRM — Architecture & Product Review

> **Status:** Draft untuk direview — belum ada implementasi
> **Tanggal:** 17 Agustus 2026
> **Sumber:** [Project Context & Brainstorming Directive](#) + [`crm_saas_brainstorming.md`](./crm_saas_brainstorming.md)
> **Menyalip:** [`technical_product_assessment.md`](./technical_product_assessment.md) (sebagian — lihat bagian 0)
> **Aturan tahap ini:** tidak ada production code, migration, endpoint, atau perubahan struktur project.

---

## Daftar Isi

- [0. Revisi terhadap Asesmen Sebelumnya](#0-revisi-terhadap-asesmen-sebelumnya)
- [1. Pemahaman Produk](#1-pemahaman-produk)
- [2. Product Boundary](#2-product-boundary)
- [3. Architecture Recommendation](#3-architecture-recommendation)
- [4. Domain Model](#4-domain-model)
- [5. Multi-Tenant Strategy](#5-multi-tenant-strategy)
- [6. Authentication & Authorization](#6-authentication--authorization)
- [7. Security Review](#7-security-review)
- [8. Documentation & Claude Code Knowledge System](#8-documentation--claude-code-knowledge-system)
- [9. MVP Boundary](#9-mvp-boundary)
- [10. Development Roadmap](#10-development-roadmap)
- [11. Architectural Risks](#11-architectural-risks)
- [12. Open Questions](#12-open-questions)
- [13. Recommended First Feature](#13-recommended-first-feature)

---

# 0. Revisi terhadap Asesmen Sebelumnya

Directive mengubah tiga premis yang menjadi dasar sebagian rekomendasi lama. Perubahannya dicatat di sini agar tidak ada rekomendasi yatim yang terbawa ke implementasi.

| # | Rekomendasi lama | Status | Alasan perubahan |
|---|---|---|---|
| 1 | Struktur modul `internal/platform/` vs `internal/crm/` + aturan dependensi | ❌ **Dibatalkan** | Directive §2 & §57: project standalone, jangan buat generic architecture untuk produk yang belum ada. Pemisahan itu **hanya** dibenarkan oleh rencana multi-produk dalam satu repo. Premis itu gugur → abstraksi ikut gugur. |
| 2 | `app.jualin.id` sebagai shell semua produk, cookie `Domain=.jualin.id` | ❌ **Dibatalkan** | Produk lain akan punya domain & repo sendiri. Cookie di-scope ke domain CRM saja. |
| 3 | Kolom `subscriptions.product = 'crm'` | ❌ **Dibatalkan** | Tidak ada produk kedua di database ini. |
| 4 | Pilih payment gateway (Xendit/Midtrans) | ❌ **Dibatalkan** | Directive §32: payment service sudah ada dan terpisah. CRM hanya menyimpan referensi eksternal. |
| 5 | `custom_fields JSONB` wajib sejak migration pertama | ⚠️ **Diturunkan ke opsional** | Directive §38 menolak JSONB tanpa alasan kuat, dan directive §51 menolak future-proofing yang tidak murah. Pada PostgreSQL modern, `ALTER TABLE ADD COLUMN` nullable itu instan. Jadi biayanya memang rendah untuk ditunda → **tunda**. |
| 6 | Composite FK ber-tenant, form public key, skema lookup API key, idempotency | ✅ **Tetap, diperkuat** | Semuanya dibenarkan oleh kebutuhan CRM itu sendiri, bukan oleh rencana multi-produk. |

**Catatan tentang #5 vs `raw_payload`:** dua-duanya JSONB, tapi tidak setara.

- `custom_fields` → bisa ditambahkan kapan saja tanpa kehilangan apapun. **Tunda.**
- `raw_payload` → kalau tidak disimpan sejak hari pertama, datanya **hilang permanen**. Tidak bisa di-backfill. **Simpan sejak awal.** Alasan bisnisnya konkret: setiap kali pelanggan bilang "lead saya tidak masuk", tanpa payload asli Anda tidak bisa membuktikan apapun.

---

# 1. Pemahaman Produk

## 1.1 Inti produk

Jualin CRM adalah **CRM SaaS multi-tenant standalone** untuk bisnis yang butuh CRM sederhana, mudah diintegrasikan, dan murah.

```
Capture Leads → Manage Leads → Assign → Employee Follow-up → Customer → Deal
```

Tiga permukaan aplikasi, satu backend:

| Permukaan | Teknologi | Pengguna | Fungsi |
|---|---|---|---|
| Landing Page | Next.js | Publik | Menjual produk |
| CRM Dashboard | Next.js | Owner, Admin, Manager | Mengoperasikan CRM |
| Employee App | Flutter | Employee | Mengeksekusi follow-up |
| Backend | Go monolith + PostgreSQL | — | Seluruh business logic + tenant isolation |

Lead masuk lewat empat jalur — **API, Embedded Form, Webhook, Manual** — dan semuanya menghasilkan entity `Lead` yang sama. Ini prinsip arsitektural terpenting di produk ini: integration layer hanyalah adapter tipis; core tidak boleh bercabang per sumber.

## 1.2 Konsekuensi strategi price-war terhadap arsitektur

Ini bagian yang paling sering diabaikan, dan di produk Anda ia menjadi **batasan rekayasa yang mengikat**, bukan sekadar catatan bisnis.

Strategi harga murah berarti **margin per tenant tipis**, sehingga biaya infrastruktur per tenant harus rendah dan dapat diprediksi. Implikasi konkretnya:

| Prinsip | Implikasi arsitektur |
|---|---|
| Biaya per tenant harus rendah | Shared database, shared schema. Schema-per-tenant atau database-per-tenant **tidak layak secara ekonomi** di model harga ini. |
| Komponen infra = biaya tetap + beban ops | **Tanpa Redis, tanpa message broker, tanpa search engine** di MVP. PostgreSQL menangani job queue, rate limit counter, dan pencarian. |
| Free tier adalah biaya, bukan pendapatan | Free tier + endpoint form publik = **permukaan abuse**. Spam bukan gangguan UX, ia langsung menjadi biaya storage dan compute yang Anda tanggung. Anti-spam adalah **fitur ekonomi**, bukan fitur keamanan. |
| Support mahal untuk produk murah | Produk harus bisa di-onboard sendiri tanpa bantuan. Kualitas halaman dokumentasi integrasi berdampak langsung ke biaya operasional. |
| Query lambat = butuh server lebih besar | Index dan pola query harus benar sejak awal. Di harga premium Anda bisa membeli jalan keluar dari query buruk; di harga murah tidak bisa. |

> **Kesimpulan:** "murah" bukan keputusan pricing yang terjadi setelah produk jadi. Ia adalah batasan desain yang berlaku sejak migration pertama.

## 1.3 Catatan jujur tentang strategi harga

Satu keberatan, disampaikan sekali lalu saya lanjutkan dengan asumsi keputusan Anda tetap:

Perang harga adalah moat yang lemah bila dijadikan **satu-satunya** pembeda — selalu ada yang bisa lebih murah, dan **gratis lebih murah daripada murah** (HubSpot Free memberi CRM + form capture tanpa batas kontak, selamanya).

Harga murah bekerja sebagai **pintu masuk**, bukan sebagai alasan bertahan. Yang membuat pelanggan tidak pindah biasanya:

1. **Mobile app yang benar-benar dipakai sales lapangan** — mobile app kompetitor global umumnya buruk. Ini pembeda paling nyata yang sudah ada di rencana Anda.
2. **Integrasi yang sudah terpasang** — begitu form dan API terpasang di website pelanggan, biaya pindah menjadi riil.
3. **Kedekatan lokal** — Bahasa, WhatsApp, invoice Rupiah, dukungan yang responsif.

Ketiganya sudah sejalan dengan rencana Anda. Rekomendasi saya hanya: **posisikan harga sebagai pintu masuk, dan mobile + integrasi sebagai alasan bertahan.** Ini tidak mengubah satu pun keputusan teknis di dokumen ini.

---

# 2. Product Boundary

## 2.1 Termasuk dalam project ini

| Area | Cakupan |
|---|---|
| **Identity & Tenancy** | User, Organization, Membership, Role, tenant context |
| **Lead Management** | Lead, status, source, notes, assignment |
| **Sales Execution** | Task, Activity, follow-up |
| **Conversion** | Customer, Deal (tahap lanjut) |
| **Lead Capture** | Direct API, Embedded Form, Inbound Webhook, manual entry |
| **Developer Surface** | API Key, API versioning, rate limiting, dokumentasi API |
| **Subscription** | Plan, limit, usage, status, **referensi** ke payment service eksternal |
| **Operational** | Audit log, notification, reports dasar |

## 2.2 Tidak termasuk — domain produk lain

HRIS · ERP · Accounting · Inventory · Payroll · Attendance · Purchasing · Manufacturing

Tidak ada tabel, kode, atau abstraksi untuk ini. Bila suatu saat dibutuhkan integrasi antar produk Jualin, itu menjadi keputusan arsitektur baru di project terpisah.

## 2.3 Batas yang akan benar-benar diuji

Daftar §2 directive mudah dipatuhi — tidak ada yang tergoda menambahkan payroll ke CRM. Yang berbahaya adalah permintaan yang **terdengar seperti CRM** tapi sebenarnya produk lain. Ini yang perlu diputuskan sebelum diminta, bukan saat pelanggan pertama memintanya.

| Permintaan | Putusan | Alasan |
|---|---|---|
| **Payment gateway / processing** | ⛔ Keluar | Sudah ada service terpisah (directive §32) |
| **Generate invoice dari Deal** | ⛔ Keluar | Ini Jualin Invoice. **Batas: CRM boleh menyimpan `deal.value`, tidak boleh menerbitkan dokumen invoice.** |
| **Quotation / penawaran ber-PDF** | ⛔ Keluar | Sangat sering diminta pengguna CRM di Indonesia — **antisipasi permintaan ini**. Jalan tengah: Activity bertipe `quotation_sent` + lampiran file. Bukan generator dokumen. |
| **Product / inventory catalog** | ⚠️ Batasi | Lead & Deal butuh referensi produk. **Batas: satu daftar nama produk sederhana milik organization. Tanpa stok, harga bertingkat, SKU, varian, atau gudang.** Melewati batas ini = membangun inventory. |
| **Email campaign / blast / nurture sequence** | ⛔ Keluar | Ini marketing automation, produk berbeda dengan ekonomi berbeda (biaya pengiriman, deliverability, compliance). Menambahkannya akan merusak struktur biaya price-war Anda. |
| **Omnichannel chat inbox (WA/IG/FB)** | ⛔ Keluar dari MVP | Godaan terbesar di pasar Indonesia dan **secara diam-diam adalah produk terbesar di daftar ini**. Batas MVP: deeplink `wa.me` + pencatatan Activity. Bukan inbox, bukan penyimpanan percakapan. |
| **Absensi / lokasi sales (GPS check-in)** | ⛔ Keluar | Ini HRIS/field-force management, meski dimintanya lewat mobile app CRM. |
| **Target & komisi sales** | ⚠️ Tunda | Berbatasan dengan payroll. Report performa per sales ✅; perhitungan komisi ⛔. |
| **Custom field pada Lead** | ✅ Masuk, tunda | Sah sebagai CRM. Ditunda karena bisa ditambahkan murah nanti (bagian 0). |

> **Aturan pengujian batas — turunan langsung dari directive §3:**
>
> *"Apakah fitur ini membantu customer menangkap, mengelola, mendistribusikan, atau mengkonversi lead?"*
>
> Tambahan dari saya: bila jawabannya ya **tapi** fitur itu memperkenalkan kelas biaya baru (pengiriman email massal, penyimpanan media, pemrosesan dokumen, penyimpanan percakapan), fitur itu **melanggar strategi price-war** meskipun lolos uji relevansi. Uji dua kali: relevansi domain **dan** struktur biaya.

## 2.4 Batas arsitektur di dalam project

- **Monolith.** Tidak ada microservices sampai ada bottleneck terukur.
- **Tanpa lapisan generik untuk produk masa depan.** Tidak ada "plugin system", "module registry", "generic entity engine".
- **Tanpa abstraksi multi-database, multi-provider, atau multi-tenant-strategy.** Satu PostgreSQL, satu strategi.

---

# 3. Architecture Recommendation

## 3.1 Prinsip pengarah

1. Satu binary, satu database, satu deployment unit.
2. Setiap komponen infrastruktur tambahan harus membayar sewanya — dalam biaya **dan** dalam beban ops.
3. Abstraksi hanya diperkenalkan setelah ada implementasi kedua yang nyata, bukan sebelumnya.

## 3.2 Go Backend

### Layering

```
HTTP Handler   → parsing, validasi bentuk, serialisasi response
     ↓
Service        → business logic, orkestrasi transaksi, otorisasi
     ↓
Repository     → akses database, selalu tenant-scoped
     ↓
PostgreSQL
```

Tiga lapis. **Tanpa** `usecase/` + `interactor/` + `entity/` + `gateway/` terpisah — clean architecture penuh menambah 3 file per operasi tanpa manfaat pada skala ini.

**Aturan lapis:**

| Aturan | Alasan |
|---|---|
| Handler **tidak boleh** memanggil repository langsung | Otorisasi & aturan bisnis tinggal di service; melewatinya = jalur yang tidak terlindungi |
| Repository **tidak boleh** berisi business logic | Agar bisa diuji terpisah dan agar SQL bisa dibaca sebagai satu kesatuan |
| Service **tidak boleh** tahu tentang HTTP | Agar bisa dipanggil dari worker/CLI tanpa membawa `*gin.Context` |
| Interface didefinisikan **di sisi consumer** | Idiom Go; menghindari paket `interfaces/` raksasa |

### Pilihan teknis

| Aspek | Rekomendasi | Alasan |
|---|---|---|
| Router | **Gin** (sesuai directive) | Ergonomis, matang. Catatan: Go 1.22+ stdlib sudah cukup — tapi ini bukan keputusan yang perlu diperdebatkan. |
| Akses DB | **sqlc** (atau `pgx` + query manual) | SQL eksplisit + tipe ter-generate. **Jangan GORM** — query implisit membuat tenant isolation sulit diaudit; Anda tidak bisa membaca satu file dan yakin `organization_id` selalu ada. |
| Migration | **goose** atau **golang-migrate** | Migration terversi = source of truth schema |
| Validasi | `go-playground/validator` untuk bentuk; aturan bisnis di service | Validasi bentuk ≠ aturan bisnis |
| Config | env var + struct ter-validasi saat boot | **Gagal cepat saat startup**, bukan saat request pertama |
| Logging | `log/slog` (stdlib), JSON di produksi | Tanpa dependensi, terstruktur |
| Error | Sentinel error domain + mapping terpusat ke HTTP | Handler tidak menentukan status code sendiri → response konsisten |
| Testing | `testcontainers` atau Postgres khusus test | Uji repository terhadap PostgreSQL asli, bukan mock. Mock DB tidak akan pernah menangkap bug isolasi tenant. |
| Background job | **Tabel `jobs` di PostgreSQL** + worker goroutine, `FOR UPDATE SKIP LOCKED` | Menghindari Redis. `SKIP LOCKED` membuat pola ini benar dan sederhana. |

### Struktur modul

Struktur §36 directive **sudah tepat** untuk project standalone. Satu penyesuaian:

```
internal/
  ├── auth/            ├── lead/           ├── form/
  ├── organization/    ├── customer/       ├── webhook/
  ├── user/            ├── task/           │
  ├── membership/      ├── activity/       ├── notification/
  ├── subscription/    ├── pipeline/       ├── auditlog/
  ├── apikey/          ├── deal/           │
  │                                        ├── middleware/
  └── platform/        ← shared teknis     ├── database/
      ├── tenant/         (bukan "platform ├── config/
      ├── httpx/           produk")        └── logger/
      └── errors/
```

**Penyesuaian yang saya sarankan:**

| Perubahan | Alasan |
|---|---|
| Hapus `role/` sebagai modul | Role adalah enum di membership, bukan domain. Modul tersendiri mengundang tabel RBAC dinamis yang tidak dibutuhkan. |
| Hapus `assignment/` sebagai modul | Assignment adalah operasi pada Lead, bukan domain terpisah. Letakkan di `lead/`. Modul terpisah akan menghasilkan service anemik yang hanya meneruskan panggilan. |
| Hapus `contact/` dari MVP | Ditunda (lihat bagian 4). |
| Hapus `storage/` sampai ada file upload | Buat saat dibutuhkan. |
| Buat modul **hanya saat fiturnya dikerjakan** | Folder kosong adalah utang dokumentasi. |

> **Peringatan:** kata `platform/` di atas berarti *utilitas teknis lintas modul* (tenant context, error, http helper) — **bukan** "platform produk" seperti pada rekomendasi lama. Bila kata ini berisiko disalahpahami lintas session, namai `shared/` atau `pkg/` saja.

## 3.3 PostgreSQL

| Keputusan | Rekomendasi | Alasan |
|---|---|---|
| Isolasi tenant | Shared DB, shared schema, `organization_id` di setiap tabel bisnis | Satu-satunya yang layak secara ekonomi di model price-war |
| Primary key | **UUIDv7** | Time-ordered → menghindari fragmentasi index yang ditimbulkan UUIDv4 pada tabel yang tumbuh cepat (`leads`, `activities`). Tetap aman diekspos publik, tidak seperti serial. |
| Index | `(organization_id, <kolom filter>)` — **`organization_id` selalu di depan** | Setiap query tenant-aware. Index tanpa prefix ini hampir tidak berguna. |
| Foreign key | Wajib, **composite ber-tenant** untuk FK antar entity bisnis | Lihat bagian 5 — ini pertahanan terpenting |
| Delete | `deleted_at` (soft) untuk entity bisnis; hard delete hanya lewat proses retensi | Pelanggan menghapus lead karena salah klik lebih sering daripada yang Anda kira |
| Waktu | `timestamptz`, simpan UTC, render di timezone organization | Simpan `organizations.timezone`, default `Asia/Jakarta` |
| Uang | `numeric(15,2)` + kolom `currency` | Jangan `float`. Jangan sampai perlu migrasi saat multi-currency muncul. |
| JSONB | Hanya dengan alasan tertulis | Sesuai directive §38. Alasan yang sah di MVP: `raw_payload` (bentuk tidak diketahui, milik pihak eksternal), `plan.entitlements` (daftar limit yang berubah tanpa migrasi), `auditlog.metadata` (heterogen secara definisi). |
| Enum | Kolom `text` + `CHECK` constraint, bukan tipe `ENUM` PostgreSQL | Menambah nilai ke tipe ENUM di PostgreSQL merepotkan; `CHECK` lebih mudah diubah lewat migration |

## 3.4 Next.js Dashboard

| Aspek | Rekomendasi |
|---|---|
| Rendering | App Router. Server Component untuk shell/halaman, Client Component untuk tabel & form interaktif. Jangan paksakan RSC ke UI interaktif. |
| Data fetching | TanStack Query di client untuk lead list/detail — butuh cache, pagination, optimistic update |
| Auth | Cookie `HttpOnly` di-set backend Go. Route protection di middleware. |
| Proxy | Panggil `api.<domain>` langsung; **jangan** bungkus setiap endpoint dengan Next.js API route. Satu hop lebih sedikit, satu tempat lebih sedikit untuk bug auth. Route Next.js hanya bila perlu manipulasi cookie. |
| UI kit | Tailwind + shadcn/ui — cepat, tanpa biaya lisensi, konsisten |
| Prioritas | **Halaman Lead list & detail adalah produknya.** Layar itu dibuka ratusan kali sehari; sisanya dibuka sesekali. Alokasikan usaha sesuai itu. |
| Empty state | Diperlakukan sebagai fitur. CRM baru itu kosong; halaman kosong yang mengarahkan (*"Pasang form Anda →"*) adalah bagian dari onboarding. Ini berdampak langsung ke biaya support. |

## 3.5 Flutter Mobile

| Aspek | Rekomendasi |
|---|---|
| State | Riverpod |
| Routing | go_router (perlu deeplink dari push notification) |
| HTTP | Dio + interceptor auto-refresh token |
| Token | `flutter_secure_storage` (Keychain/Keystore). **Jangan** SharedPreferences. |
| Offline | **Cache baca sejak awal** (Drift/Isar) — sales lapangan kehilangan sinyal di basement mall dan area pinggiran. Retrofit offline ke aplikasi online-only adalah penulisan ulang, bukan penambahan. |
| Antrian offline | Aksi tulis dengan id ter-generate client (UUIDv7) + idempotency key, agar sinkronisasi ulang tidak menduplikasi Activity |
| Push | FCM. Push adalah *best-effort* — daftar notifikasi in-app tetap menjadi source of truth. |
| Ukuran & waktu buka | Batasan nyata — pengguna memakai Android kelas menengah, bukan flagship |

> **Detail kecil berdampak besar:** tombol Call dan WhatsApp **harus otomatis membuat Activity**. Sales tidak akan pernah mengisi log secara manual. Tanpa auto-log, timeline kosong dan seluruh nilai reporting hilang — dan reporting adalah alasan owner membayar.

## 3.6 Direct API

```
POST /v1/leads
Authorization: Bearer jln_live_<key_id>_<secret>
Idempotency-Key: <uuid dari client>
Content-Type: application/json
```

| Aspek | Rekomendasi |
|---|---|
| Versioning | Prefix path `/v1/` sejak awal. Murah sekarang, mustahil ditambahkan setelah ada integrator. |
| Auth | API key milik organization (bagian 6) |
| Idempotency | `UNIQUE (organization_id, idempotency_key)`. Pengulangan mengembalikan **response asli**, bukan error. |
| Rate limit | Per API key. Balas header `X-RateLimit-*` + `Retry-After` **sejak versi pertama** — menambahkannya nanti membingungkan integrator yang sudah jalan. |
| Error | Bentuk konsisten `{error: {code, message, details}}`. `code` stabil dan machine-readable. |
| Validasi | Tolak field tak dikenal? **Tidak** — abaikan, agar client lama tidak rusak saat Anda menambah field. Simpan tetap di `raw_payload`. |
| Batas payload | Wajib (mis. 64KB) |

## 3.7 Embedded Form

**Keputusan paling penting:** endpoint form publik **tidak boleh** memakai API key.

Directive §28 sudah benar menyatakan *"Jangan menganggap Form ID sebagai secret."* Yang perlu ditegaskan sebagai konsekuensinya: karena form berjalan di browser pengunjung, **tidak ada satupun kredensial rahasia yang boleh hadir di sana**. Bila API key dipasang di sisi klien, siapapun bisa membacanya lewat DevTools dan memakainya untuk mengakses seluruh API organization tersebut.

| | Direct API | Embedded Form |
|---|---|---|
| Endpoint | `POST /v1/leads` | `POST /v1/forms/{public_key}/submit` |
| Kredensial | `jln_live_...` — **rahasia** | `public_key` — **publik, memang terekspos** |
| Kemampuan | Sesuai scope key | **Hanya submit ke form itu.** Tidak bisa membaca apapun. |
| Proteksi | Rate limit per key | Domain allowlist + rate limit per IP + honeypot + CAPTCHA |

> Pola yang sama dipakai Stripe: `pk_` publishable untuk browser, `sk_` secret untuk server. Dua kredensial, dua kemampuan.

### Mekanisme embed

**Rekomendasi MVP: iframe.**

| | iframe | inline script |
|---|---|---|
| Isolasi keamanan | ✅ Kuat — form terisolasi dari halaman host | ❌ Berjalan di konteks host |
| Risiko dari host yang tercemar | ✅ Rendah | ❌ Script host bisa membaca input |
| Fleksibilitas styling | ❌ Terbatas | ✅ Menyatu dengan situs |
| Kompleksitas implementasi | ✅ Rendah | ❌ Perlu isolasi CSS, shadow DOM |

Untuk MVP yang sederhana dan aman, iframe menang. Inline script bisa ditawarkan nanti sebagai opsi lanjutan.

### Field form

**MVP: field tetap** — name, email, phone, company, message, product — dengan toggle wajib/opsional dan label yang bisa diubah.

Form builder dinamis (owner menyusun field bebas) memerlukan schema field + mesin validasi + renderer + strategi penyimpanan. Itu fitur besar tersendiri, dan tidak diperlukan untuk membuktikan bahwa capture layer bekerja.

## 3.8 Webhook

### Inbound *(pasca-MVP)*

`POST /v1/webhooks/in/{endpoint_token}` — token panjang & acak. Verifikasi signature bila provider mendukung. Proteksi replay dengan toleransi waktu.

Secara jujur, inbound webhook ≈ Direct API dengan payload yang tidak Anda kendalikan. Nilai tambah sesungguhnya ada pada **field mapping** — dan itulah pekerjaan besarnya. Karena itu ia ditunda.

### Outbound *(pasca-MVP)*

Ini yang lebih berisiko bagi Anda, karena **server Anda** yang melakukan request:

| Risiko | Mitigasi |
|---|---|
| **SSRF** | Pelanggan mendaftarkan `http://169.254.169.254/...` (metadata cloud) atau `http://localhost:5432`. Blokir private IP range **setelah resolusi DNS**; tangani DNS rebinding (resolve → validasi → connect ke IP yang sudah divalidasi). Hanya HTTPS, port 443. |
| **Forgery** | HMAC-SHA256 atas `timestamp + body`, header `X-Jualin-Signature` |
| **Worker exhaustion** | Backoff eksponensial + jitter, batas percobaan, circuit breaker per endpoint |
| **Hang** | Timeout 5 detik, tanpa mengikuti redirect |

Model data: `webhook_endpoints` + `webhook_deliveries` (event, payload, attempt, status, response). Cukup, tanpa event bus.

## 3.9 Subscription

Karena payment service sudah terpisah, domain ini menjadi kecil:

```
plans                 → definisi limit (entitlements JSONB)
subscriptions         → organization → plan, status, periode,
                        external_reference (id di payment service)
usage_counters        → (organization_id, metric, period_start, value)
```

| Aspek | Rekomendasi |
|---|---|
| Limit | Di `plans.entitlements` (JSONB), **jangan hard-code di Go**. Mengubah limit = update data, bukan deploy. |
| Penegakan | Satu titik: middleware/service guard `CheckQuota(org, metric)`. Bukan `if` yang tersebar di banyak handler. |
| Counter | UPSERT atomik (`INSERT ... ON CONFLICT DO UPDATE SET value = value + 1`). Increment baca-lalu-tulis akan salah di bawah konkurensi. |
| Status | `active`, `past_due`, `suspended`, `canceled` — tentukan **sekarang** apa arti masing-masing terhadap akses |
| Integrasi payment | Hanya `external_reference` + endpoint penerima webhook status. **Tanpa** logika payment. |
| Kontrak | Definisikan kontrak integrasi ke payment service Anda **sebelum** Phase 8, bukan saat mengerjakannya |

---

# 4. Domain Model

## 4.1 Koreksi terpenting: Employee adalah Membership, bukan entity

Directive tidak konsisten pada titik ini: §14 (Core Domain) **tidak** menyebut Employee sebagai entity, tapi §11 menggambarkan `Employees` sebagai anak Organization, dan §13 menjadikan `Employee` sebagai role.

**Rekomendasi: Employee bukan tabel.**

```
User ──< Membership >── Organization
              │
              └── role: owner | admin | manager | employee
```

| Konsep | Definisi |
|---|---|
| **User** | Identitas login: email, password_hash, status verifikasi |
| **Membership** | Keanggotaan user di satu organization + role. **Inilah "employee".** |
| **Employee** | Membership dengan `role = 'employee'` |

**Assignment menunjuk `membership_id`.**

Kenapa bukan `user_id`? Karena satu user bisa berada di dua organization. Bila lead menunjuk `user_id`, isolasi tenant bergantung pada disiplin query di setiap tempat. Bila menunjuk `membership_id` — yang secara definisi terikat pada satu organization — isolasi menjadi **struktural dan bisa ditegakkan database** (lihat bagian 5).

Kalau Employee dijadikan tabel terpisah, pertanyaan berikut tidak akan punya jawaban bersih:

- Owner UMKM yang ikut menangani lead sendiri — perlu record Employee?
- Employee dipromosikan jadi Manager — recordnya pindah tabel? Lead lamanya ikut?
- Manager memegang lead sendiri — dia User atau Employee?

## 4.2 Entity MVP

```
┌─ IDENTITY & TENANCY ────────────────────────────────────────┐
│  User ──< Membership >── Organization                        │
│                              │                               │
│                              ├──< Subscription >── Plan      │
│                              │         └──< UsageCounter     │
│                              ├──< Invitation                 │
│                              ├──< ApiKey                     │
│                              └──< AuditLog                   │
└──────────────────────────────────────────────────────────────┘

┌─ CRM CORE ───────────────────────────────────────────────────┐
│  Form ──┐                                                    │
│  ApiKey ├──> LEAD ──assigned_to──> Membership                │
│  Manual ┘     │                                              │
│               ├──< Activity  (sudah terjadi, append-only)    │
│               ├──< Task      (harus dilakukan, mutable)      │
│               └──> Customer ──< Deal                         │
└──────────────────────────────────────────────────────────────┘
```

## 4.3 Relasi utama

| Relasi | Kardinalitas | Catatan |
|---|---|---|
| User ↔ Organization | many-to-many via Membership | Jangan `user.organization_id`. Schema mendukung banyak; UI MVP boleh asumsikan satu. |
| Membership → role | enum kolom | Bukan tabel. RBAC dinamis ditunda. |
| Organization → Subscription | 1 aktif + histori | Baris ber-status, agar riwayat plan terjaga |
| Organization → ApiKey | 1-N | Key **milik organization**, bukan user. Kalau pembuatnya keluar, integrasi tidak boleh mati. Simpan `created_by_membership_id` untuk audit saja. |
| Organization → Form | 1-N | Form punya `public_key` |
| Lead → Membership | N-1, nullable (`assigned_to`) | **Wajib composite FK** (bagian 5) |
| Lead → Activity | 1-N | Append-only, immutable |
| Lead → Task | 1-N | Mutable: `due_at`, `status`, `assignee` |
| Lead → Customer | 1-1 saat konversi | **Baris baru**, bukan mutasi. Lihat di bawah. |
| Customer → Deal | 1-N | Satu customer bisa membeli berkali-kali |

### ⚠️ Lead vs Customer

Godaan terbesar: Customer = Lead dengan `status='customer'`. **Jangan.**

- Satu Customer bisa berasal dari **beberapa** Lead (orang yang sama mengisi form 3× setahun).
- Lead adalah **peristiwa capture**; Customer adalah **relasi berkelanjutan**.
- Menggabungkannya merusak metrik konversi secara permanen — Anda tidak akan bisa menghitung *conversion rate* karena penyebut dan pembilangnya adalah baris yang sama.

**Putusan:** Lead tetap ada sebagai catatan historis; Customer adalah baris baru; dihubungkan `lead.converted_customer_id`.

## 4.4 Entity yang sebaiknya TIDAK dibuat di MVP

| Entity di §14 | Putusan | Alasan |
|---|---|---|
| **Role** | ⛔ Bukan tabel | Enum di membership. Tabel Role/Permission/RolePermission adalah RBAC dinamis — dibutuhkan saat pelanggan meminta custom role, dan itu sinyal enterprise yang belum ada. |
| **Assignment** | ⛔ Bukan tabel | `lead.assigned_to` (kondisi sekarang) + Activity bertipe `lead_assigned` (riwayat). Tabel terpisah baru dibutuhkan bila ada beberapa assignee aktif sekaligus — dan itu belum ada. |
| **Contact** | ⛔ Tunda | Relevan untuk B2B dengan banyak PIC per perusahaan. SMB Indonesia: Customer sudah cukup. |
| **Form Submission** | ⛔ Tunda | Submission yang valid **adalah** Lead (+ `raw_payload`). Tabel terpisah baru berguna saat Anda perlu menyimpan submission yang **ditolak** (spam) untuk analisis — kebutuhan pasca-MVP. |
| **Pipeline** | ⛔ Tunda | `lead.status` **adalah** pipeline di MVP. Lihat 4.5. |
| **Deal** | ⚠️ Versi minimal | Cukup: `customer_id`, `value`, `currency`, `status`, `closed_at`. Tanpa stage, tanpa produk berbaris, tanpa probability. Ini membuka seluruh reporting revenue dengan satu tabel. |

## 4.5 Lead lifecycle

Directive §16 mengusulkan `NEW → CONTACTED → QUALIFIED → PROPOSAL → WON | LOST`, dan menyatakan lifecycle belum dikunci.

**Rekomendasi MVP:**

```
NEW ──> CONTACTED ──> QUALIFIED ──> PROPOSAL ──> WON
 │           │             │            │
 └───────────┴─────────────┴────────────┴──────> LOST
 │
 └──> UNQUALIFIED   (bukan calon pembeli, bukan kegagalan sales)
 │
 └──> SPAM          (form publik akan menghasilkan ini)
```

**Empat catatan:**

1. **Tambahkan `SPAM` dan `UNQUALIFIED`.** Tanpa keduanya, spam dari form publik akan masuk ke metrik konversi dan merusak angka yang justru menjadi alasan owner membayar. Ini konsekuensi langsung dari punya form publik + free tier.
2. **Jangan tambahkan status `ASSIGNED`.** Assignment ortogonal terhadap status — lead bisa `NEW` dan sudah ter-assign, atau `QUALIFIED` dan belum. Mencampurnya menghasilkan state machine yang tidak konsisten.
3. **`lost_reason`** wajib saat `LOST` (enum: harga, kompetitor, timing, tidak responsif, lainnya). Ini satu kolom yang menghasilkan laporan paling berguna bagi owner.
4. **Status = pipeline di MVP.** Ini justru penyederhanaan yang tepat untuk positioning "CRM sederhana". Saat Deal masuk nanti, `PROPOSAL`/`WON` berpindah ke Deal dan daftar status Lead menyusut. **Catat rencana ini sebagai ADR sekarang**, supaya session mendatang tidak menganggapnya inkonsistensi.

**Transisi divalidasi di service layer**, bukan sekadar kolom bebas — kalau tidak, mobile app dan dashboard akan menghasilkan riwayat yang berbeda aturannya.

## 4.6 Field yang harus ada sejak migration pertama

| Field | Di | Alasan **sekarang** |
|---|---|---|
| `raw_payload JSONB` | leads | **Tidak bisa di-backfill.** Kalau tidak disimpan, hilang selamanya. Menyelamatkan setiap debugging integrasi pelanggan. |
| `phone_e164` | leads, customers | Normalisasi `08123…` → `+628123…` **saat ingest**. Tanpa ini dedup dan deeplink WhatsApp rusak; memperbaikinya nanti = backfill seluruh tabel. |
| `idempotency_key` | leads | Unik per org |
| `source` + `source_id` | leads | `api` / `form` / `webhook` / `manual` + id sumbernya |
| `deleted_at` | entity bisnis | Soft delete konsisten |
| `organizations.timezone` | organizations | Default `Asia/Jakarta`. Laporan "lead hari ini" salah tanpa ini. |

**Tidak** perlu di MVP: `custom_fields` (murah ditambahkan nanti — lihat bagian 0).

---

# 5. Multi-Tenant Strategy

Ini area di mana kegagalan bersifat fatal: satu kebocoran lintas tenant menghapus kredibilitas B2B secara permanen, dan tidak ada permintaan maaf yang memulihkannya.

**Strategi: shared database, shared schema, `organization_id` di setiap tabel bisnis.** Satu-satunya yang layak secara ekonomi di model harga Anda.

`organization_id` saja **tidak cukup** — itu hanya mengandalkan disiplin developer, dan disiplin gagal pada jam 2 pagi di sprint yang sibuk. Empat lapis:

## Lapis 1 — Repository yang tidak bisa lupa

Tidak boleh ada jalur query yang menerima org sebagai parameter opsional. Setiap method repository menerima `TenantContext` sebagai parameter **pertama** dan meng-inject `organization_id` ke WHERE clause **di dalam**, bukan menyerahkannya ke caller.

```go
// Konsep
repo.Leads.FindByID(ctx, tenant, id)   // org_id di-inject di dalam
```

**Dan tidak ada** method `FindByID(id)` tanpa tenant — kalau method itu tidak ada, tidak ada yang bisa memanggilnya secara salah.

> Prinsipnya: *make illegal states unrepresentable*, diterapkan pada tenancy. Ini bukan abstraksi tambahan — ini bentuk signature yang sama-sama harus ditulis, hanya dipilih yang aman.

**Konsekuensi:** ini alasan tambahan menolak ORM. Dengan sqlc/pgx, setiap query bisa dibaca dan diaudit sebagai teks.

## Lapis 2 — Composite foreign key

**Bagian yang hampir selalu terlewat.** Lapis 1 melindungi *pembacaan*. Yang tidak terlindungi: **referensi silang tenant**.

**Skenario nyata:**

```
Owner Org A → PATCH /leads/{lead_milik_A}
              { "assigned_to": "<membership_id milik Org B>" }
```

Query lead-nya tenant-scoped → lolos. FK ke `memberships(id)` valid → lolos.
**Hasil: data korup lintas tenant tanpa satupun error.**

Kalau `assigned_to` menunjuk `user_id`, kerusakannya lebih parah lagi: karyawan Org B melihat lead Org A di aplikasi mobilenya.

**Cegah di database:**

```sql
-- pada memberships
UNIQUE (id, organization_id)

-- pada leads
FOREIGN KEY (assigned_to, organization_id)
  REFERENCES memberships (id, organization_id)
```

Sekarang database **mustahil** menyimpan referensi lintas tenant, apapun bug di kode aplikasi.

Terapkan untuk **setiap** FK antar entity bisnis: `task → lead`, `activity → lead`, `deal → customer`, `lead → form`.

> Biayanya satu unique index tambahan per tabel. **Ini rasio manfaat-per-biaya tertinggi di seluruh sistem**, dan ia bekerja bahkan ketika kode aplikasi salah.

## Lapis 3 — Row Level Security *(ditunda, tapi jangan dihalangi)*

RLS PostgreSQL dengan `SET LOCAL app.current_org_id` + policy per tabel.

**Rekomendasi: tunda ke pasca-MVP.** Alasan jujur: RLS berinteraksi rumit dengan connection pooling dan menambah beban debugging nyata untuk tim kecil. Lapis 1, 2, dan 4 sudah memberi perlindungan sangat besar. Ini juga sejalan dengan directive §51.

**Yang harus dijaga sekarang agar RLS bisa masuk nanti tanpa nyeri:**

1. Nama kolom **konsisten `organization_id`** di semua tabel — tanpa pengecualian
2. **Semua** akses DB lewat satu transaction manager — satu tempat untuk menyisipkan `SET LOCAL` nanti

Dua hal itu gratis hari ini dan mahal diperbaiki nanti.

## Lapis 4 — Test suite isolasi tenant *(wajib)*

Directive §53 sudah menyebut tenant isolation test. Penajaman:

Jangan tulis test per endpoint secara manual — itu akan tertinggal begitu endpoint bertambah. Buat **satu test generik yang berjalan di atas daftar route**:

```
Untuk setiap endpoint tenant-scoped:
  buat Org A + data, Org B + data
  panggil endpoint dengan kredensial A terhadap resource id milik B
  harapkan 404
```

**Selalu 404, bukan 403** — 403 mengonfirmasi bahwa resource dengan id tersebut ada, dan itu kebocoran informasi tersendiri.

**Jadikan blocking di CI.** Endpoint baru otomatis ikut teruji tanpa ada yang perlu ingat.

## Tenant context

```
Request
  → Auth middleware (JWT | API key | form public key)
  → resolve principal
  → resolve organization_id   ← SELALU dari kredensial, TIDAK PERNAH dari body/query
  → cek status subscription
  → cek quota
  → rate limit
  → service
```

```go
type TenantContext struct {
    OrganizationID uuid.UUID
    PrincipalType  PrincipalType   // user | api_key | public_form | system
    MembershipID   *uuid.UUID      // nil bila API key
    UserID         *uuid.UUID
    Role           Role
    APIKeyID       *uuid.UUID
    RequestID      string
}
```

`PrincipalType` bukan hiasan: audit log harus bisa membedakan *"Budi mengubah lead"* dari *"integrasi website mengubah lead"*. Tanpa itu, audit trail Anda menyesatkan — lebih buruk daripada tidak ada.

---

# 6. Authentication & Authorization

## 6.1 Tiga jalur authentication

### Dashboard — Owner / Admin / Manager

| Aspek | Rekomendasi |
|---|---|
| Access token | JWT, TTL pendek (15 menit) |
| Refresh token | **Opaque, disimpan di DB** — bisa direvoke; JWT murni tidak bisa |
| Penyimpanan | Cookie `HttpOnly; Secure; SameSite=Lax`, di-scope ke domain CRM |
| **Jangan** | Simpan token di `localStorage` — satu XSS = semua token pelanggan tercuri. Cookie HttpOnly tidak terbaca JavaScript. |
| Konsekuensi | Memakai cookie ⇒ **wajib proteksi CSRF** (double-submit token, atau `SameSite=Strict` pada endpoint mutasi). Ini yang paling sering terlupakan saat memilih cookie. |

### Mobile — Employee

| Aspek | Rekomendasi |
|---|---|
| Token | Access JWT (1 jam, lebih panjang karena sering offline) + refresh opaque |
| Penyimpanan | `flutter_secure_storage` (Keychain / Keystore) |
| Rotasi | **Refresh rotation + reuse detection** — setiap refresh menerbitkan token baru & membatalkan yang lama. Bila token lama dipakai lagi → asumsikan pencurian, revoke seluruh family, paksa login ulang. |
| Device binding | Kaitkan refresh token ke `device_id` → push bisa dialamatkan, dan owner bisa logout jarak jauh. **Kasus nyata: sales resign sambil membawa daftar lead di HP-nya.** |

### External API

| Aspek | Rekomendasi |
|---|---|
| Format | `jln_live_<key_id>_<secret>` |
| Penyimpanan | `key_id` plaintext ter-index + `secret_hash` (SHA-256) |
| Verifikasi | Lookup by `key_id` → `subtle.ConstantTimeCompare` |
| Scope | Kolom ada sejak awal (`leads:write`, `leads:read`), meski MVP hanya memakai satu. Menambah scope ke key yang sudah beredar adalah breaking change. |
| Batas | **Tidak boleh** menyentuh endpoint dashboard — tidak bisa kelola employee, ubah subscription, atau membuat key lain. Ditegakkan di middleware, bukan per handler. |

### Public form

Tanpa authentication. Diproteksi oleh: `public_key` + domain allowlist + rate limit per IP + honeypot + CAPTCHA. Kemampuannya hanya membuat lead untuk form itu.

### Ringkasan

| Principal | Kredensial | Penyimpanan | Revocable | Lifetime |
|---|---|---|---|---|
| Dashboard user | JWT + refresh opaque | HttpOnly cookie | ✅ | 15m / 30d |
| Mobile employee | JWT + refresh opaque | Secure storage | ✅ + device | 1h / 90d |
| External system | API key | Milik pelanggan | ✅ instan | Sampai direvoke |
| Public form | Public key | Publik | ✅ | Sampai form dihapus |

## 6.2 Authorization — matriks permission

Otorisasi ditegakkan di **service layer**. UI yang menyembunyikan tombol bukan otorisasi.

| Resource | Owner | Admin | Manager | Employee |
|---|---|---|---|---|
| Organization settings | CRUD | Read | Read | Read |
| Membership: undang / hapus / ubah role | CRUD | CRU¹ | Read | — |
| Subscription & billing | CRUD | Read | — | — |
| API Key | CRUD | Read² | — | — |
| Form | CRUD | CRUD | Read | — |
| Webhook | CRUD | CRUD | — | — |
| Lead — semua di organization | CRUD | CRUD | Read | — |
| Lead — yang di-assign ke dirinya | CRUD | CRUD | CRUD | **RU** |
| Assign / reassign lead | ✅ | ✅ | ✅ | — |
| Customer | CRUD | CRUD | Read | Read³ |
| Deal | CRUD | CRUD | Read | Read³ |
| Task | CRUD | CRUD | CRUD | Own |
| Activity | Create + Read | Create + Read | Create + Read | Own leads |
| Audit log | Read | Read | — | — |
| Reports | Read | Read | Read | — |

¹ Admin tidak boleh mengubah/menghapus Owner, dan tidak boleh mengangkat siapapun menjadi Owner
² Admin melihat daftar & metadata key, tapi tidak bisa membuat atau merevoke
³ Terbatas pada customer/deal yang berasal dari lead miliknya

**Empat aturan yang harus ditulis eksplisit:**

1. **Employee hanya melihat lead yang di-assign kepadanya** — ditegakkan di **repository**, bukan hanya di UI. Ini permukaan kebocoran paling mungkin di mobile app.
2. **Owner terakhir tidak bisa menghapus atau menurunkan dirinya sendiri.** Organization tanpa owner adalah tenant yatim yang hanya bisa diperbaiki lewat akses database langsung.
3. **Tidak ada yang bisa mengubah role dirinya sendiri.**
4. **Manager melihat seluruh organization, bukan "tim"** — lihat 6.3.

## 6.3 ⚠️ Ambiguitas: Manager tanpa konsep Team

Directive §13 mendefinisikan Manager sebagai *"Team monitoring"*. Tapi tidak ada entity `Team` di §14, dan tidak ada di roadmap.

**Tanpa Team, "Manager" tidak punya definisi ruang lingkup.**

| Opsi | Konsekuensi |
|---|---|
| **A. Manager = akses baca seluruh organization** *(rekomendasi MVP)* | Sederhana, nol tabel tambahan, cukup untuk organisasi < 20 sales |
| **B. Tambahkan entity Team** | Tabel + UI + filter di setiap query + aturan keanggotaan. Fitur tersendiri. |

**Rekomendasi: Opsi A untuk MVP**, dan **jangan sebut kata "team" di UI** — gunakan "semua lead" atau "seluruh organisasi". Kalau UI menjanjikan team padahal tidak ada, pengguna akan melaporkannya sebagai bug.

Team menjadi relevan saat pelanggan punya beberapa divisi sales — itu sinyal untuk membangunnya, dan sinyal itu akan datang dari pelanggan, bukan dari tebakan.

---

# 7. Security Review

Diurutkan berdasarkan tingkat keparahan.

## 🔴 Kritis

### 1. API key tidak boleh berada di sisi klien

Endpoint form publik memakai `public_key` (publishable), **bukan** API key. Rinciannya di 3.7.

**Dampak bila salah:** setiap pemasangan form membocorkan kredensial penuh organization tersebut ke publik. Bukan risiko probabilistik — kepastian.

### 2. Skema penyimpanan & lookup API key

Menyimpan hash saja **tidak cukup** — ada jebakan implementasi. Bcrypt/argon2 di-salt, sehingga lookup mustahil: setiap request harus membandingkan terhadap **seluruh** baris. Pada 10.000 key, itu 10.000 operasi hash lambat per request. Sistem berhenti berfungsi.

**Yang benar:**

```
jln_live_<key_id>_<secret>
          │        └─ crypto/rand, 32 char
          └─ ter-index, plaintext, untuk lookup

1. Parse key_id
2. SELECT WHERE key_id = $1 AND revoked_at IS NULL   ← O(1)
3. subtle.ConstantTimeCompare(sha256(secret), secret_hash)
```

**Kenapa SHA-256, bukan argon2, khusus untuk API key?** argon2/bcrypt sengaja lambat untuk melawan brute-force pada *password* — yang entropinya rendah karena dipilih manusia. API key dari `crypto/rand` punya entropi 256-bit; brute-force mustahil terlepas dari kecepatan hash. Memakai argon2 di sini hanya menambah ratusan milidetik ke **setiap request API panas** tanpa manfaat keamanan.

> **Untuk password user, tetap argon2id.** Dua kasus berbeda, dua jawaban berbeda.

Catatan operasional: `last_used_at` di-update **async atau throttled**, jangan setiap request — itu menjadikan tabel api_keys write hotspot.

### 3. Kebocoran lintas tenant

Empat lapis di bagian 5. Composite FK (lapis 2) dan test generik (lapis 4) adalah yang paling saya tekankan di seluruh dokumen ini.

## 🟠 Tinggi

### 4. Idempotency & lead duplikat

**Tidak ada di directive.** Ini bug correctness yang **pasti terjadi**:

- Client retry karena timeout → lead ganda
- Visitor klik submit dua kali → lead ganda
- Webhook pihak ketiga mengirim ulang (semua provider melakukannya) → lead ganda

> Lead duplikat merusak kepercayaan lebih cepat daripada hampir semua bug lain — sales menelepon orang yang sama dua kali, dan **pelanggan akhir Anda yang menyaksikannya**.

Dua mekanisme, keduanya perlu:

| Mekanisme | Cara |
|---|---|
| **Idempotency teknis** | Header `Idempotency-Key`, `UNIQUE (organization_id, idempotency_key)`. Pengulangan → kembalikan **response asli**, bukan error. Retensi 24–48 jam. |
| **Dedup bisnis** | `phone_e164` atau `email` sama dalam 30 hari di org yang sama → **tandai `possible_duplicate_of`**, jangan tolak, jangan auto-merge. Auto-merge berbahaya: dua orang bisa berbagi nomor kantor. |

### 5. Abuse pada form publik — ini isu ekonomi, bukan hanya keamanan

Free tier + endpoint publik = magnet spam. Di model price-war, setiap lead spam adalah biaya yang **Anda** tanggung.

| Proteksi | Catatan |
|---|---|
| Domain allowlist per form | Verifikasi header `Origin`. *Jujur: `Origin` bisa dipalsukan non-browser — ini menghalangi penyalahgunaan biasa, bukan penyerang tertarget. Karena itu ia berpasangan, tidak berdiri sendiri.* |
| CAPTCHA (Cloudflare Turnstile) | Gratis, tanpa puzzle. **Wajib di free tier.** |
| Honeypot | Field tersembunyi; bila terisi → buang diam-diam. **Jangan** kembalikan error — bot akan belajar. |
| Rate limit per IP + per form | mis. 5/menit |
| Time-trap | Tolak submit < 2 detik setelah render |
| Batas payload | mis. 32KB |

### 6. Otorisasi mobile

Employee hanya boleh melihat lead miliknya. **Ditegakkan di repository.** Uji secara eksplisit: employee A meminta lead employee B → 404.

### 7. SSRF pada outbound webhook

Rinciannya di 3.8. Relevan saat webhook dibangun, **tapi catat sekarang** — ini jenis kerentanan yang mudah terlewat saat fiturnya dikerjakan di bawah tekanan waktu.

## 🟡 Sedang

| Risiko | Mitigasi |
|---|---|
| Password | argon2id. Cek terhadap daftar bocor (HIBP k-anonymity, gratis). Min 12 karakter; jangan paksa aturan komposisi rumit (NIST SP 800-63B). |
| Enumerasi email | Response register & forgot-password **identik** ada/tidaknya email |
| Brute-force login | Rate limit per IP **dan** per akun, backoff progresif. Jangan lock permanen — itu vektor DoS terhadap pengguna sah. |
| Token verifikasi & undangan | Sekali pakai, hash di DB, kedaluwarsa (24 jam / 7 hari), undangan terikat ke email spesifik |
| Privilege escalation | Aturan di 6.2 |
| PII / UU PDP | Data pelanggan Anda berisi PII pihak ketiga — Anda *processor*. Perlu: export, hapus permanen, retensi. |
| Penghapusan tenant | Soft delete → grace 30 hari → hard delete terjadwal. Bukan `CASCADE DELETE` langsung. |
| Logging | Jangan pernah log password, API key mentah, token, atau payload lead lengkap. Redaksi di logger, bukan di call site. |
| Embed | Sajikan dari domain terpisah, CSP ketat, `frame-ancestors` sesuai allowlist |
| Upload file | Bila ada: validasi magic bytes (bukan ekstensi), sajikan dari domain terpisah, jangan pernah dieksekusi |

---

# 8. Documentation & Claude Code Knowledge System

Ini bagian yang paling menentukan apakah "1 Feature = 1 Session" benar-benar berjalan.

## 8.1 Penilaian struktur usulan

| Elemen | Penilaian |
|---|---|
| `CLAUDE.md` | ✅ Tepat |
| `.claude/skills/` | ⚠️ Terlalu banyak skill di awal |
| `docs/product/` | ✅ Tepat |
| `docs/architecture/` | ✅ Tepat |
| `docs/features/` | ⚠️ Terlalu banyak file per feature |
| `docs/decisions/` | ✅ Tepat |
| **Project state ledger** | ❌ **Tidak ada — ini kekurangan terpenting** |

## 8.2 ❌ Yang hilang: berkas state project

Tujuan Anda (§61): *"Session berikutnya tidak perlu membaca seluruh history percakapan."*

Struktur usulan menyimpan **rencana** (requirements, domain, api) dan **hasil** (code), tapi tidak ada satu tempat pun yang menjawab pertanyaan pertama setiap session baru:

> **"Sekarang sudah sampai mana?"**

Tanpa itu, setiap session dibuka dengan menjelajahi kode untuk merekonstruksi status — persis pemborosan yang ingin Anda hindari, dan sumber kesalahan ketika rekonstruksinya keliru.

### Rekomendasi: `docs/STATUS.md`

Satu berkas, di-update di akhir **setiap** session. Ini adalah artefak terpenting dalam sistem dokumentasi Anda.

```markdown
# Project Status

Last updated: 2026-08-17 (Session 3 — API Key)

## Sudah Selesai
| Feature | Session | Status | Catatan |
|---|---|---|---|
| Foundation      | 1 | ✅ | Docker, migration, config, logger, error |
| Auth & Org      | 2 | ✅ | Register, verify, login, refresh, tenant ctx |
| API Key         | 3 | ✅ | Scope belum dipakai — hanya leads:write |

## Sedang Dikerjakan
(kosong)

## Berikutnya
Lead core — bergantung pada Auth & Org

## Utang Teknis
- [ ] Rate limit masih in-memory, tidak selamat dari restart
- [ ] last_used_at API key ditulis sinkron, perlu di-throttle
- [ ] Belum ada test isolasi tenant untuk endpoint API key

## Keputusan yang Belum Diambil
- Perilaku saat kuota lead habis (tolak vs terima+tandai)
- Aturan dedup lead final
```

Bagian **Utang Teknis** sama pentingnya dengan bagian Selesai: tanpa itu, kompromi yang diambil di session 3 akan terlupa sepenuhnya di session 8, lalu ditemukan kembali sebagai bug produksi.

## 8.3 `docs/features/` — terlalu berat

Usulan §45: 6 file per feature × ~11 feature = **66 berkas**.

Masalahnya bukan jumlahnya, melainkan konsekuensinya: dokumentasi yang mahal dipelihara akan **berhenti dipelihara**, lalu menjadi menyesatkan — dan dokumentasi yang menyesatkan lebih buruk daripada tidak ada, karena session berikutnya akan mempercayainya.

### Rekomendasi: 2 berkas per feature

```
docs/features/api-key/
  ├── spec.md      # requirements + domain + API + acceptance criteria
  └── notes.md     # keputusan implementasi, penyimpangan, utang teknis
```

| Berkas | Isi | Kapan ditulis |
|---|---|---|
| `spec.md` | Apa & kenapa: requirement, entity + relasi, endpoint + bentuk request/response, aturan otorisasi, acceptance criteria | **Sebelum** implementasi |
| `notes.md` | Apa yang benar-benar terjadi: keputusan yang diambil saat coding, penyimpangan dari spec **beserta alasannya**, utang teknis, jebakan untuk session berikutnya | **Sesudah** implementasi |

Pemisahan sebelum/sesudah ini penting: `spec.md` adalah kontrak, `notes.md` adalah realitas. Menggabungkannya menghasilkan dokumen yang tidak jelas menggambarkan rencana atau hasil.

Pecah menjadi lebih banyak berkas **hanya** bila satu feature benar-benar tumbuh besar (Lead kemungkinan akan). Mulai dari dua, mekar sesuai kebutuhan.

**Testing** tidak perlu berkas tersendiri — acceptance criteria di `spec.md` sudah mendefinisikan apa yang harus diuji, dan test itu sendiri ada di kode.

## 8.4 `.claude/skills/` — mulai dari satu

Usulan §42: 5 skill. Rekomendasi: **mulai dengan 1, tambah saat terbukti perlu.**

Alasannya: sampai backend berjalan, skill frontend/mobile/database/testing tidak akan pernah terpakai — dan skill yang ditulis sebelum ada kode adalah tebakan tentang konvensi yang belum terbentuk. Konvensi ditemukan saat menulis kode, bukan sebelumnya.

| Skill | Kapan dibuat |
|---|---|
| `jualin-backend` | **Sekarang** — konvensi Go, layering, pola repository tenant-scoped, penanganan error, pola testing |
| `jualin-frontend` | Saat mulai dashboard (Phase 3) |
| `jualin-mobile` | Saat mulai Flutter (Phase 5) |
| `jualin-database` | Kemungkinan tidak perlu terpisah — gabungkan ke `jualin-backend` kecuali sudah terbukti terlalu panjang |
| `jualin-testing` | Sama — gabungkan dulu |

**Isi skill = "bagaimana menulis kode di project ini"** (konvensi, pola, contoh konkret).
**Isi docs = "apa yang harus dibangun dan kenapa"** (requirement, keputusan).

Pemisahan itu sudah benar di directive §42. Pertahankan dengan ketat — begitu business requirement bocor ke skill, ia akan usang tanpa ada yang menyadarinya.

## 8.5 `CLAUDE.md` — jaga tetap pendek

**Target: di bawah 150 baris.** File ini masuk ke setiap session; setiap baris membayar sewa pada setiap tugas, termasuk yang tidak relevan dengannya.

Kerangka yang direkomendasikan:

```markdown
# Jualin CRM

## Apa ini
CRM SaaS multi-tenant standalone. Capture → Manage → Assign → Follow-up → Customer → Deal.
Strategi: harga terjangkau. Infrastruktur harus tetap murah.

## Di luar cakupan
HRIS, ERP, accounting, inventory, payroll, invoice generation, email campaign, chat inbox.
Produk lain = project terpisah. Jangan buat abstraksi untuk produk yang belum ada.

## Stack
Go + Gin + PostgreSQL + Docker (monolith) · Next.js (dashboard) · Flutter (mobile)

## Aturan yang tidak bisa dilanggar
1. Setiap tabel bisnis punya organization_id
2. Repository selalu tenant-scoped — tidak ada method tanpa TenantContext
3. organization_id selalu dari kredensial, tidak pernah dari input client
4. FK antar entity bisnis memakai composite key ber-tenant
5. Otorisasi di service layer, bukan di UI
6. API key & password di-hash. Raw key hanya ditampilkan sekali.
7. Feature dengan tenant boundary wajib punya test isolasi tenant
8. Resource tenant lain → 404, bukan 403

## Anti-overengineering
Bila solusi sederhana sudah cukup, pakai itu. Abstraksi hanya setelah ada
implementasi kedua yang nyata. Jangan bangun untuk kebutuhan yang belum ada.

## Layering
Handler → Service → Repository → PostgreSQL
Handler tidak memanggil repository. Repository tidak berisi business logic.

## Workflow session
1 feature = 1 session. Baca: CLAUDE.md → docs/STATUS.md → skill terkait →
docs/architecture terkait → docs/features/<feature>/spec.md → kode terkait.
Rencana dulu, tunggu persetujuan untuk perubahan besar, baru implementasi.
Akhir session: update docs/STATUS.md + docs/features/<feature>/notes.md.

## Source of truth
Kode & migration > CLAUDE.md > docs/architecture > docs/features > ADR > brainstorming
Bila dokumentasi bertentangan dengan kode: laporkan, jangan diam-diam mengubah salah satunya.
```

Detail feature **tidak** masuk sini. Yang masuk hanya aturan yang berlaku untuk **setiap** session.

## 8.6 Struktur akhir yang direkomendasikan

```
CLAUDE.md                      ← < 150 baris, aturan global

docs/
├── STATUS.md                  ← ⭐ state project, di-update tiap session
├── product/
│   ├── vision.md              ← positioning, ICP, strategi
│   ├── scope.md               ← in/out (bagian 2 dokumen ini)
│   ├── roadmap.md             ← fase & urutan
│   └── glossary.md            ← ⭐ definisi istilah — cegah drift lintas session
├── architecture/
│   ├── overview.md
│   ├── multi-tenancy.md       ← ⭐ tulis sebelum kode pertama
│   ├── authentication.md
│   ├── authorization.md       ← matriks permission
│   ├── database.md            ← konvensi, bukan salinan schema
│   └── api.md                 ← konvensi API: versioning, error, pagination
├── features/
│   └── <feature>/
│       ├── spec.md
│       └── notes.md
├── decisions/
│   └── ADR-XXX-*.md
└── brainstorming/             ← histori, read-only
    ├── crm_saas_brainstorming.md
    ├── technical_product_assessment.md
    └── architecture_product_review.md   ← dokumen ini

.claude/skills/
└── jualin-backend/
```

**Dua tambahan terhadap usulan Anda:**

| Tambahan | Kenapa |
|---|---|
| `docs/STATUS.md` | Satu-satunya jawaban atas "sudah sampai mana" — inti dari kontinuitas lintas session |
| `docs/product/glossary.md` | Mencegah drift penamaan. Tanpa ini, session 2 menulis `employee_id`, session 5 menulis `member_id`, session 8 menulis `staff_id` — semuanya merujuk hal yang sama, dan tidak ada yang menyadarinya sampai refactor menyakitkan. |

**Jangan buat semua folder sekarang.** Buat saat diisi. Folder kosong adalah utang dokumentasi yang terlihat seperti kemajuan.

## 8.7 Protokol session

**Awal session:**

```
1. CLAUDE.md
2. docs/STATUS.md                          ← sudah sampai mana
3. .claude/skills/jualin-backend           ← bagaimana menulis kode di sini
4. docs/architecture/<yang relevan>        ← multi-tenancy hampir selalu relevan
5. docs/features/<feature>/spec.md         ← bila sudah ada
6. Kode yang berdekatan                    ← pola nyata mengalahkan pola terdokumentasi
7. → Rencana implementasi → persetujuan → implementasi
```

**Akhir session — non-negotiable:**

```
1. Update docs/STATUS.md          (selesai, utang teknis, berikutnya)
2. Tulis docs/features/<f>/notes.md
3. ADR bila ada keputusan arsitektur signifikan
4. Update docs/architecture/* bila ada konvensi yang berubah
```

> Bila langkah akhir session dilewati sekali saja, sistem dokumentasi ini kehilangan nilainya — session berikutnya kembali membaca kode untuk merekonstruksi status, dan Anda membayar biaya penulisan dokumen tanpa mendapat manfaatnya.

## 8.8 ADR yang layak ditulis sekarang

Sesuai directive §47 — hanya untuk keputusan yang benar-benar penting:

| ADR | Isi |
|---|---|
| `ADR-001-monolith.md` | Monolith, kriteria kapan dievaluasi ulang |
| `ADR-002-multi-tenancy.md` | Shared schema + 4 lapis, kenapa RLS ditunda |
| `ADR-003-employee-as-membership.md` | Employee = Membership, bukan tabel |
| `ADR-004-api-key-format.md` | Format key, kenapa SHA-256 bukan argon2 |
| `ADR-005-public-form-key.md` | Pemisahan public key vs API key |
| `ADR-006-lead-status-as-pipeline.md` | Status = pipeline di MVP, rencana perpindahan saat Deal masuk |

Enam ADR ini menangkap keputusan yang paling mungkin dipertanyakan ulang oleh session mendatang — dan tanpa catatan alasannya, akan diubah tanpa sadar.

---

# 9. MVP Boundary

## 9.1 Wajib MVP

Definisi MVP — turunan dari core flow §49 directive:

> Owner mendaftar → mengundang sales → membuat API key → website mengirim lead →
> owner meng-assign → sales menindaklanjuti dari HP → lead menjadi customer.

### Backend

| Area | Cakupan |
|---|---|
| Foundation | Config, migration, logging, error, health check, Docker, CI |
| Auth & Org | Register, verifikasi email, login, refresh, forgot password, tenant context, RBAC |
| **Employee invitation** | ⚠️ Undang → set password → membership. **Tidak ada di roadmap directive** — lihat 10.2 |
| Lead | CRUD, status, source, notes, filter, pagination |
| Assignment | Manual |
| Activity | Append-only, auto-log pada perubahan status & assignment |
| Task | Dengan `due_at` |
| Customer | Konversi dari Lead |
| API Key | Create, list, revoke |
| Lead API | `POST /v1/leads`, idempotency, validasi, rate limit |
| Audit log | Aksi sensitif: auth, API key, role, penghapusan |
| Notification | Untuk assignment |

### Dashboard

Auth · onboarding · **lead list & detail** · assignment · employee + undangan · API key + halaman dokumentasi · metrik dasar · settings

### Mobile

Login · My Leads · My Tasks · lead detail · Call/WhatsApp dengan auto-Activity · update status · notes · push · cache offline baca

### Infrastruktur

VPS + Docker Compose · reverse proxy · **managed PostgreSQL** · backup harian **+ uji restore** · Sentry · CI

> **Catatan tentang managed PostgreSQL:** ini satu-satunya tempat di mana saya menyarankan membayar lebih meski strateginya price-war. Mengelola database produksi sendiri berarti Anda menangani backup, failover, patching, dan tuning. Biaya satu kali kehilangan data pelanggan jauh melebihi selisih harganya, dan pemulihannya tidak mungkin.

## 9.2 Bisa ditunda

| Fitur | Kapan |
|---|---|
| Embedded Form | Phase 6 — setelah API terbukti |
| Webhook (in & out) | Phase 7 |
| Subscription enforcement | Phase 8 |
| Integrasi payment service | Phase 8, setelah core stabil |
| Pipeline (kustom stage) | Phase 9 — `lead.status` sudah cukup |
| Deal | Phase 9 — atau versi minimal lebih awal (lihat 10.2) |
| Contact | Saat ada pelanggan B2B dengan banyak PIC |
| Round robin & assignment rule | Setelah manual assignment terbukti dipakai |
| Custom field | Saat diminta |
| Import/export CSV | **Naikkan prioritas bila calon pelanggan datang dari spreadsheet — dan mereka biasanya begitu** |
| Reports & analytics lanjutan | Setelah metrik dasar dipakai |
| Automation | Jauh kemudian |
| RBAC dinamis, Team, SSO | Sinyal enterprise — belum ada |
| Multiple API key environment | Simpan **format** `live`/`test` sejak awal (gratis); sandbox environment ditunda |

---

# 10. Development Roadmap

## 10.1 Penilaian roadmap directive

Roadmap §48 **lebih baik** daripada dokumen brainstorming pertama, dan saya mengadopsinya sebagai dasar. Urutan Phase 2 (CRM Core) → Phase 3 (Dashboard) → Phase 4 (API) khususnya tepat: Anda memvalidasi model domain lewat entry manual di UI **sebelum** membekukan API publik — dan API publik adalah permukaan yang paling mahal untuk diubah setelah ada integrator.

## 10.2 Amandemen yang saya rekomendasikan

| # | Amandemen | Alasan |
|---|---|---|
| 1 | **Pindahkan Employee Invitation ke Phase 1** | Saat ini tidak ada di fase manapun — hanya muncul di open questions §58. Padahal Phase 5 (mobile) mustahil tanpa employee yang bisa login. Ini blocker tersembunyi. |
| 2 | **Keluarkan Contact & Pipeline dari Phase 2** | Phase 2 mencantumkan Contact; Pipeline muncul di §14. Keduanya ditunda (bagian 4.4). Phase 2 sudah padat tanpa itu. |
| 3 | **Tambahkan Deal minimal di akhir Phase 3** | Tanpa nilai deal, dashboard tidak bisa menjawab *"berapa yang saya hasilkan bulan ini"* — pertanyaan yang membuat owner membayar. Cukup satu tabel: `customer_id`, `value`, `closed_at`. |
| 4 | **Idempotency masuk Phase 2, bukan Phase 4** | Ia menyentuh schema `leads`. Menambahkannya di Phase 4 berarti migrasi + membongkar jalur create yang sudah ada. |
| 5 | **Definisikan kontrak payment service sebelum Phase 8** | Kontrak integrasi ditulis lebih awal; implementasinya tetap di Phase 8. Menemukan ketidakcocokan kontrak saat Phase 8 akan mahal. |
| 6 | **Test isolasi tenant generik di Phase 1**, bukan menyusul | Kalau harness-nya ada sejak awal, setiap feature berikutnya otomatis terlindungi tanpa ada yang perlu ingat. |

## 10.3 Roadmap hasil amandemen

| Phase | Isi | Selesai bila |
|---|---|---|
| **0 — Foundation** | Go, Docker, PostgreSQL, config, migration, logging, error, health, CI | `docker compose up` jalan, migration jalan, CI hijau |
| **1 — Auth & Organization** | User, Organization, Membership, register, verifikasi email, login, refresh, **invitation**, tenant context, RBAC, **harness test isolasi** | Dua org bisa dibuat; test isolasi membuktikan tidak ada kebocoran silang |
| **2 — CRM Core** | Lead, status, source, Activity, Task, assignment manual, Customer, **idempotency** | Siklus lead lengkap berjalan lewat API internal |
| **3 — Owner Dashboard** | Auth UI, lead list & detail, employee, assignment, task, metrik dasar, **Deal minimal** | 🎯 **Demo pertama** — owner memakainya tanpa curl |
| **4 — Public API** | API key, autentikasi API, `POST /v1/leads`, validasi, rate limit, dokumentasi | 🎯 **Tesis produk terbukti** — curl dari luar → lead muncul di dashboard |
| **5 — Employee Mobile** | Flutter: login, My Leads, My Tasks, detail, activity, follow-up, push, cache offline | 🎯 **Produk menjadi nyata** — siklus penuh di HP |
| **6 — Embedded Form** | Form, public key, submit endpoint, domain allowlist, anti-spam, embed | Paste snippet → submit → lead masuk; domain tak terdaftar ditolak |
| **7 — Webhook** | Inbound, outbound, event, delivery, retry, log | |
| **8 — Subscription** | Plan, limit, usage, status, upgrade, integrasi payment service | Free tier benar-benar terbatas |
| **9 — Advanced** | Pipeline, Deal lengkap, reports, analytics, automation | Berdasarkan permintaan pelanggan nyata |

## 10.4 🚦 Gate setelah Phase 5

Setelah Phase 5 Anda punya siklus produk yang lengkap: capture → assign → follow-up → convert.

> **Rekomendasi kuat: cari 3–5 pengguna nyata sebelum melanjutkan ke Phase 6.**
>
> Apa yang mereka minta akan berbeda dari tebakan Anda, dan itu yang seharusnya menentukan urutan Phase 6–9. Ini juga sekaligus menguji tesis harga Anda dengan orang yang benar-benar membuka dompet.

Kegagalan paling umum untuk produk seperti ini bukan arsitektur yang buruk — melainkan **arsitektur yang bagus untuk produk yang belum ada yang membutuhkannya**.

## 10.5 Pemetaan ke session

Directive §61 memakai 1 feature = 1 session. Feature besar perlu dipecah, karena satu session yang terlalu besar menghasilkan review yang tidak menyeluruh:

| Session | Feature | Phase |
|---|---|---|
| 1 | Foundation | 0 |
| 2 | User, Organization, Membership, Registration | 1 |
| 3 | Email verification, Login, Refresh token, Tenant context | 1 |
| 4 | RBAC + Employee invitation + harness test isolasi | 1 |
| 5 | Lead core (model, CRUD, status, source, idempotency) | 2 |
| 6 | Activity + Task | 2 |
| 7 | Assignment + notification | 2 |
| 8 | Customer + Deal minimal | 2 |
| 9–12 | Dashboard (dipecah per area) | 3 |
| 13 | API Key | 4 |
| 14 | Public Lead API + rate limit + dokumentasi | 4 |
| 15+ | Mobile, Form, Webhook, Subscription | 5–8 |

---

# 11. Architectural Risks

## 🔴 R1 — Kebocoran lintas tenant

| | |
|---|---|
| **Dampak** | Fatal. Kepercayaan B2B tidak pulih setelah insiden seperti ini. |
| **Kemungkinan** | Sedang — meningkat seiring bertambahnya endpoint |
| **Titik terlemah** | Referensi silang tenant (bukan pembacaan) dan otorisasi mobile employee |
| **Mitigasi** | 4 lapis di bagian 5. **Composite FK + harness test generik adalah yang paling menentukan.** |
| **Kapan** | Phase 1, bukan menyusul |

## 🔴 R2 — Scope creep ke domain produk lain

| | |
|---|---|
| **Dampak** | Kehilangan fokus, biaya naik, positioning kabur |
| **Kemungkinan** | **Tinggi** — permintaan akan datang dari pelanggan nyata, dan terdengar masuk akal |
| **Bentuk nyata** | Quotation PDF, invoice, chat inbox WhatsApp, katalog produk penuh |
| **Mitigasi** | Batas eksplisit di 2.3, ditulis ke `docs/product/scope.md` **sebelum** permintaan pertama datang. Menolak lebih mudah bila batasnya sudah tertulis sebelum ada wajah yang memintanya. |

## 🟠 R3 — Biaya abuse merusak ekonomi price-war

| | |
|---|---|
| **Dampak** | Free tier menjadi beban biaya, bukan corong akuisisi |
| **Kemungkinan** | Tinggi begitu form publik hidup |
| **Mitigasi** | Anti-spam berlapis (7.5) sebagai bagian dari Phase 6, bukan tambalan setelahnya. Batasi juga retensi data free tier. |
| **Catatan** | Di model harga premium, spam adalah gangguan. Di model Anda, ia langsung menggerus margin. |

## 🟠 R4 — Scope Flutter di Phase 5

| | |
|---|---|
| **Dampak** | Mobile app adalah pembeda utama Anda, dan sekaligus fase terberat |
| **Kemungkinan** | Tinggi — offline + push + auth rotation adalah tiga hal yang mudah diremehkan |
| **Mitigasi** | Jangan kejar fitur; kejar keandalan. Login yang selalu bekerja + daftar lead yang muncul offline lebih bernilai daripada sepuluh layar tambahan. |

## 🟠 R5 — Dokumentasi menjadi usang

| | |
|---|---|
| **Dampak** | Menghancurkan seluruh premis 1 feature = 1 session; dokumentasi yang salah lebih berbahaya daripada tidak ada |
| **Kemungkinan** | **Tinggi** — ini kegagalan paling umum dari sistem dokumentasi yang ambisius |
| **Mitigasi** | Dokumentasi seminimal mungkin (8.3), `STATUS.md` sebagai satu berkas wajib, update akhir session yang tidak bisa ditawar |

## 🟡 R6 — Pengeluaran waktu sebelum ada pengguna

| | |
|---|---|
| **Dampak** | Membangun berbulan-bulan untuk produk yang belum tervalidasi |
| **Mitigasi** | Gate setelah Phase 5 (10.4). Kalau memungkinkan, ajak 2–3 calon pengguna sejak sekarang. |

## 🟡 R7 — Kontrak payment service tidak cocok

| | |
|---|---|
| **Dampak** | Phase 8 tersendat karena asumsi yang tidak sesuai |
| **Mitigasi** | Tulis kontrak integrasi (event, field, arah panggilan, penanganan kegagalan) sebelum Phase 8, meski implementasinya belakangan |

---

# 12. Open Questions

## 🔴 Harus dijawab sebelum migration pertama

| # | Pertanyaan | Rekomendasi |
|---|---|---|
| 1 | Employee = Membership, atau tabel terpisah? | **Membership** (4.1) |
| 2 | Satu user boleh di banyak organization? | **Ya di schema, tidak di UI MVP.** Membaliknya nanti = migrasi besar. |
| 3 | Lead → Customer: mutasi baris atau baris baru? | **Baris baru + link** (4.3) |
| 4 | Status lifecycle Lead final? | 4.5 — dengan tambahan `SPAM` & `UNQUALIFIED` |
| 5 | Assignment: kolom di Lead atau tabel sendiri? | **Kolom `assigned_to`** + Activity untuk riwayat (4.4) |
| 6 | Form Submission entity terpisah? | **Tidak** di MVP — submission valid **adalah** Lead + `raw_payload` |
| 7 | UUIDv7 atau serial? | **UUIDv7** (3.3) |
| 8 | Manager: lihat seluruh org atau butuh Team? | **Seluruh org, tanpa Team.** Jangan pakai kata "team" di UI. (6.3) |

## 🟠 Sebelum Phase 4 (Public API)

| # | Pertanyaan | Rekomendasi |
|---|---|---|
| 9 | Apa yang terjadi saat kuota lead habis? | **Jangan buang lead pelanggan** — kerusakan yang tidak bisa dibatalkan, dan mereka akan menyalahkan Anda. Usulan: terima sampai 2× limit dengan tanda `over_quota`, kunci fitur dashboard, peringatkan keras; tolak hanya di luar itu. **Keputusan bisnis — saya hanya bisa merekomendasikan.** |
| 10 | Owner dihitung dalam kuota employee? | **Tidak.** "2 employees" harus berarti 2 sales. Lebih jujur dan lebih mudah dijelaskan. |
| 11 | Aturan dedup lead? | `phone_e164` atau `email` sama dalam 30 hari → tandai, jangan tolak, jangan auto-merge |
| 12 | Scope API key di MVP? | Kolom ada, isi hanya `leads:write`. Menambah scope ke key yang beredar = breaking change. |
| 13 | Rate limit: angka & granularitas? | Token bucket per API key + per IP untuk endpoint publik. In-memory di MVP, dengan interface yang bisa diganti. Header `X-RateLimit-*` sejak versi pertama. |

## 🟡 Sebelum Phase 6–8

| # | Pertanyaan | Catatan |
|---|---|---|
| 14 | Email provider | Resend (DX terbaik) / Postmark (deliverability) / SES (termurah, paling ribet). **Siapkan SPF/DKIM/DMARC sejak hari pertama** — email verifikasi yang masuk spam membunuh funnel tanpa Anda sadari. |
| 15 | Embed: iframe atau inline script? | **iframe** untuk MVP (3.7) |
| 16 | Retensi data free tier | Pembatas yang bagus untuk konversi **sekaligus** pengendali biaya storage |
| 17 | Kontrak integrasi payment service | Event apa, field apa, arah panggilan, penanganan kegagalan |
| 18 | Bahasa UI | Indonesia / Inggris / dwibahasa — menentukan apakah i18n dibutuhkan sejak awal. Retrofit i18n menyakitkan. |
| 19 | Push provider | FCM default. iOS perlu APNs — pastikan biayanya dipahami. |
| 20 | Domain final & branding | Menentukan konfigurasi cookie, CORS, dan email sender |

## 🟢 Menyusul

Backup & DR detail · kebijakan penghapusan tenant · API v2 · developer portal · observability lanjutan · strategi deployment/CD

---

# 13. Recommended First Feature

## 13.1 Sebelum session pertama: bootstrap dokumentasi

Karena sistem kerja Anda menjadikan repository sebagai memori, dokumentasi harus ada **sebelum** kode pertama — kalau tidak, session 1 tidak punya aturan untuk diikuti dan session 2 tidak punya rujukan.

Ini bukan session pengembangan. Perkiraan setengah hari:

| Berkas | Isi |
|---|---|
| `CLAUDE.md` | Kerangka di 8.5 |
| `docs/STATUS.md` | Kosong, siap diisi |
| `docs/product/scope.md` | Bagian 2 dokumen ini |
| `docs/product/glossary.md` | Organization, Membership, Lead, Customer, Deal, Activity, Task, Assignment |
| `docs/architecture/overview.md` | Bagian 3 |
| `docs/architecture/multi-tenancy.md` | Bagian 5 — **yang paling penting** |
| `docs/decisions/ADR-001..003` | Monolith, multi-tenancy, employee-as-membership |
| `.claude/skills/jualin-backend/` | Konvensi Go, layering, pola repository tenant-scoped |

> Jawab dulu 8 pertanyaan 🔴 di bagian 12 — beberapa berkas di atas tidak bisa ditulis tanpa jawabannya.

## 13.2 Session 1 — Foundation

**Rekomendasi: Foundation, bukan Authentication.**

Auth adalah feature pertama yang bernilai bisnis, tapi mengerjakannya tanpa migration tooling, config, penanganan error, dan harness testing berarti Anda membangun semua itu **di tengah** feature auth — tercampur, tanpa review tersendiri, dan menjadi preseden pola untuk semua session berikutnya. Fondasi yang dibuat sambil lalu akan ditiru oleh 15 session sesudahnya.

### Cakupan Session 1

| Termasuk | Tidak termasuk |
|---|---|
| Struktur project Go + `cmd/api` | Business logic apapun |
| Docker Compose (app + PostgreSQL) | Endpoint domain |
| Config dari env, ter-validasi saat boot | Tabel selain migration awal |
| Structured logging + request ID | Auth |
| Penanganan & mapping error terpusat | |
| Setup migration + migration pertama (`organizations`, `users`) | |
| Health check | |
| Harness testing (database test, helper) | |
| CI: lint + test + build | |

### Acceptance criteria

1. `docker compose up` → API merespons `/health`
2. Migration up & down berjalan bersih
3. Config invalid → gagal saat startup dengan pesan jelas, bukan saat request pertama
4. Log terstruktur JSON dengan request ID
5. Error mengembalikan bentuk konsisten `{error: {code, message}}`
6. Test bisa berjalan terhadap PostgreSQL asli
7. CI hijau

### Kenapa ini batasan session yang baik

- Tanpa ketergantungan pada feature lain
- Bisa direview dalam satu kali duduk
- Menetapkan setiap pola yang akan ditiru 15 session berikutnya
- Menghasilkan `docs/STATUS.md` yang terisi — memulai siklus dokumentasi dengan benar

## 13.3 Session 2 dan seterusnya

| Session | Feature | Kenapa urutan ini |
|---|---|---|
| 2 | User, Organization, Membership, Registration | Semua tenant-scoped bergantung padanya |
| 3 | Email verification, Login, Refresh, **Tenant context** | Tenant context memblokir setiap feature berikutnya |
| 4 | RBAC + Employee invitation + **harness test isolasi** | Harness lebih awal ⇒ setiap feature setelahnya otomatis terlindungi |
| 5 | Lead core | Jantung produk |

---

## Ringkasan — apa yang perlu Anda putuskan

1. **Jawab 8 pertanyaan 🔴** di bagian 12 — memblokir migration pertama
2. **Konfirmasi batas produk di 2.3** — khususnya quotation, katalog produk, dan chat inbox
3. **Konfirmasi struktur dokumentasi di 8.6** — terutama penambahan `STATUS.md` dan penyederhanaan feature docs
4. **Konfirmasi amandemen roadmap di 10.2** — terutama Employee Invitation masuk Phase 1

Setelah keempatnya selesai: bootstrap dokumentasi (13.1), lalu Session 1 Foundation.

---

*Dokumen review. Setiap rekomendasi disertai alasan agar bisa ditolak secara sadar, bukan diikuti secara buta. Keputusan final ada pada pemilik produk.*
