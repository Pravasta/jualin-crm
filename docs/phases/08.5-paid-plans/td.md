# Phase 8.5 — Paket Berbayar & Kuota · TD

> **Bagaimana.** Apa & kenapa di [`prd.md`](./prd.md).
> Ini **delta** untuk phase ini. Aturan yang sudah ada di [`freeze.md`](../../architecture/freeze.md) tidak diulang, hanya dirujuk.

---

## 1. Schema — **satu migration kecil, ditemukan saat #123 dikerjakan**

> ⚠️ **Koreksi atas klaim awal phase ini (Aturan #30).** Saat PRD/TD ditulis, bagian ini menyatakan
> *"phase ketiga tanpa migration sama sekali"*. Itu keliru: `notifications.type` punya
> `CONSTRAINT ck_notifications_type CHECK (type IN ('lead_assigned','task_assigned'))`
> (`0004_notifications.sql`), dan §5 di bawah butuh nilai ketiga untuk memberi tahu Owner saat kuota
> habis. Ketahuan saat #123 dikerjakan, sebelum kode ditulis — bukan setelah migration terlanjur
> dilewati. Klaim ini diperbaiki di sini, bukan didiamkan.

Kolom yang dibutuhkan **selain notifikasi** sudah ada seluruhnya, tanpa migration:

| Yang dibutuhkan | Sudah ada di |
|---|---|
| `subscriptions.plan_code` (`text`, tanpa `CHECK`) | `0002_identity.sql` — menerima `'pro'`/`'enterprise'` apa adanya |
| `subscriptions.status` | `0002` — dibaca sejak #112 |
| `leads.organization_id` + `created_at` + `deleted_at` | `0003_crm_core.sql` |
| Index `ix_leads_org_created (organization_id, created_at DESC) WHERE deleted_at IS NULL` | `0003` — **lihat §4.2, ada catatan penting** |
| `audit_log` | `0002` — perubahan paket dicatat di sini |

**`migrations/0010_notification_plan_quota.sql`** — satu baris: `ALTER TABLE notifications DROP
CONSTRAINT ck_notifications_type, ADD CONSTRAINT ck_notifications_type CHECK (type IN
('lead_assigned','task_assigned','plan_quota_exceeded'))`. `down` mengembalikan constraint lama —
aman hanya jika tidak ada baris `plan_quota_exceeded` yang sudah ditulis, sama seperti setiap
`down` di produk ini yang mengasumsikan rollback terjadi sebelum data baru terpakai secara luas.

**Tidak ada `usage_counters`** (prd D1). Kewajiban Phase 8 D1 (*"dievaluasi ulang bersamaan dengan
mendaratnya angka limit"*) dengan ini **terjawab, bukan ditunda lagi**: angkanya mendarat, evaluasinya
dilakukan, jawabannya tetap tidak — dengan alasan yang berbeda dari Phase 8. Dulu: tidak ada angka
untuk ditegakkan. Sekarang: `COUNT` atas `leads` memberi semantik yang sama tanpa tabel dan tanpa
jalur tulis di tiga usecase.

---

## 2. Peta limit — di berkas yang sama dengan peta kanal

```go
// internal/subscription/plan.go — di bawah planChannels

const (
    PlanFree       = "free"        // sudah ada sejak #112
    PlanPro        = "pro"         // BARU
    PlanEnterprise = "enterprise"  // BARU
)

// Limits is what a plan allows in QUANTITY — the dimension Phase 8
// deliberately did not have. Zero means unlimited, not "none": a plan
// that forgot to set a number must not silently stop a customer's
// business (§2.1).
type Limits struct {
    LeadsPerMonth int
    Seats         int
}

// planLimits is THE map for quantities, deliberately beside planChannels
// rather than merged into it: "which channel" and "how many" are
// different questions with different failure modes (§2.1), and merging
// them makes one struct answer both badly.
//
// ⚠️ ANGKA DI BAWAH PROVISIONAL — ditetapkan tanpa data pengguna nyata
// (ADR-014). Wajib ditinjau ulang setelah 3–5 pelanggan berbayar pertama.
var planLimits = map[string]Limits{
    PlanFree:       {LeadsPerMonth: 0 /* TODO */, Seats: 0 /* TODO */},
    PlanPro:        {LeadsPerMonth: 0 /* TODO */, Seats: 0 /* TODO */},
    PlanEnterprise: {LeadsPerMonth: 0, Seats: 0}, // tanpa batas — disengaja
}
```

`planChannels` bertambah dua baris untuk `pro` dan `enterprise`; isinya (kanal mana terbuka untuk
paket mana) ikut tabel angka provisional di `prd.md`.

### 2.1 Gagal tertutup di sini **tidak** berarti nol — dan itu bukan inkonsistensi

Phase 8 memutuskan: paket tak dikenal → **seluruh kanal tertutup**. Phase 8.5 memutuskan sebaliknya
untuk kuota: paket tak dikenal, atau `status != 'active'` → **limit paket paling ketat yang dikenal
(`free`)**, bukan nol.

Alasannya asimetri konsekuensi, dan harus ditulis supaya tidak "diperbaiki" jadi konsisten-secara-buta:

| Gagal tertutup pada… | Akibat bagi pelanggan | Layak? |
|---|---|---|
| **Kanal** (Phase 8) | Tidak bisa membuat API key / form / webhook **baru**. Yang sudah ada tetap jalan | Ya — mengganggu, tidak merusak |
| **Kuota, kalau nol** | Produk **berhenti menerima lead sama sekali**. Form di situs pelanggan mati, integrasi mati | **Tidak** — kegagalan billing tidak boleh menghapus fungsi inti |
| **Kuota, dibatasi ke `free`** | Pelanggan turun ke batas gratis sampai keadaannya beres | Ya — perilaku standar SaaS, dan bisa dipulihkan |

Nol tetap berarti **tanpa batas** di dalam `Limits` (dipakai `enterprise`) — bukan "tidak boleh apa
pun". Konstanta itu dibaca lewat satu helper (`allows(limit, used int) bool`), tidak pernah
dibandingkan langsung di call site, supaya arti "0" hidup di satu tempat.

---

## 3. Kuota **bukan** `PlanGate` — pertanyaannya berbeda

Godaan yang harus ditolak eksplisit: memakai ulang `PlanGate` (#113) untuk kuota. Ditolak karena
ketiga bedanya nyata:

| | `PlanGate` (Phase 8) | Kuota (phase ini) |
|---|---|---|
| Pertanyaan | "Paket ini membuka kanal X?" | "Organization ini sudah pakai berapa dari jatahnya?" |
| Butuh baca tabel bisnis? | Tidak — cukup baris `subscriptions` | **Ya** — `COUNT` atas `leads` / `memberships` |
| Konsumen | `apikey`, `form`, `webhook` | `lead`, `invitation` |
| Perilaku saat menolak | Selalu `403` | **Bergantung principal** (§5) |

Konsumen baru mendeklarasikan interface-nya sendiri (ADR-011), bentuk yang sama:

```go
// internal/lead/port.go
type PlanQuota interface {
    // AllowLead reports whether t's organization may create one more
    // lead this month. used is supplied by the caller because only
    // lead's own repository can count leads — the gate owns the LIMIT,
    // the domain owns the COUNT.
    AllowLead(ctx context.Context, t tenant.Context, used int) error
}

// internal/invitation/port.go
type PlanQuota interface {
    AllowSeat(ctx context.Context, t tenant.Context, used int) error
}
```

**Kenapa `used` diserahkan pemanggil, bukan dihitung di dalam gate.** Kalau gate yang menghitung, ia
harus mengimpor `internal/lead` untuk tahu cara menghitung lead — membalik arah dependensi yang
ADR-011 tetapkan. Yang dimiliki `subscription` adalah **batasnya**; yang dimiliki domain adalah
**angkanya**. Bridge di composition root sama seperti `planGate` (#113).

---

## 4. Kuota lead — satu titik penegakan untuk tiga jalur

### 4.1 Titik pasangnya cuma satu, dan itu bukan kebetulan

Ketiga jalur pembuatan lead bermuara di **`lead.Usecase.Create`**:

```
POST /v1/leads          (principal user, dashboard/mobile) ─┐
POST /v1/leads          (principal api_key)                 ├─► lead.Usecase.Create
POST /v1/forms/{pk}/submit → form.Usecase.Submit
      → LeadCreator (bridge cmd/api/form_store.go) ─────────┘
```

Diverifikasi sebelum TD ini ditulis, bukan diasumsikan: `cmd/api/form_store.go`'s
`leadCreatorAdapter.CreateFromForm` memanggil `a.usecase.Create(...)`, dan `lead.Usecase.Create`
sendiri sudah bercabang atas `t.PrincipalType` untuk `PrincipalAPIKey` dan `PrincipalPublicForm`
(#47, #87). **Satu `if` di satu fungsi menutup ketiga jalur** — dan principal-nya tersedia persis di
tempat yang membutuhkannya untuk keputusan §5.

Urutan di dalam `Create`, mengikat — kuota **setelah** authz, sebelum kerja:

```go
if err := authz.Require(t, authz.ActionLeadCreate); err != nil { return ... }   // 1. role/scope
used, err := repos.Lead.CountCreatedThisMonth(ctx, t)                            // 2. hitung
if err := u.quota.AllowLead(ctx, t, used); err != nil { ... }                     // 3. kuota
```

### 4.2 `CountCreatedThisMonth` menghitung **termasuk yang soft-deleted**

```sql
SELECT count(*) FROM leads
WHERE organization_id = $1 AND created_at >= $2   -- awal bulan, di timezone organization
-- deliberately NO "deleted_at IS NULL"
```

Menghapus lead **tidak mengembalikan kuota** — itu yang membuat `COUNT` setara penghitung (prd D1).
Kalau predikat `deleted_at IS NULL` ikut, tersedia jalur penyalahgunaan sederhana: buat sampai batas,
hapus, ulangi selamanya.

> ⚠️ **Index `ix_leads_org_created` punya `WHERE deleted_at IS NULL`** (`0003`), jadi query di atas
> **tidak** memakainya. Tiga pilihan, diputuskan saat issue dikerjakan dengan `EXPLAIN` sebagai bukti,
> bukan tebakan: (a) biarkan — jumlah lead per organization per bulan berada di orde ratusan–ribuan,
> seq scan atas partisi tenant kecil masih murah; (b) index parsial baru khusus penghitungan;
> (c) hitung dengan `deleted_at IS NULL` dan terima jalur penyalahgunaannya. **Rekomendasi (a)**, dan
> `EXPLAIN`-nya dicatat di `notes.md` — kalau ternyata mahal, (b) adalah satu migration kecil.

Awal bulan dihitung di **timezone organization** (`organizations.timezone`, Aturan #13), bukan UTC —
pelanggan di Asia/Jakarta yang jatahnya reset jam 07:00 tanggal 1 akan menganggapnya bug.

---

## 5. Saat kuota habis — perilakunya **bergantung siapa yang memanggil**

> ✅ **prd D3 ditutup 5 September 2026.** §5 di bawah adalah keputusan yang berlaku, bukan lagi rekomendasi.

| Principal | Jalur | Saat kuota habis |
|---|---|---|
| `user` | Dashboard / mobile | **Ditolak** — `403 plan_quota_exceeded`, pesan menyebut batas & ajakan naik paket |
| `api_key` | `POST /v1/leads` | **Ditolak** — `403 plan_quota_exceeded`. Integrator adalah sistem pelanggan sendiri; ia berhak tahu alasannya |
| `public_form` | `POST /v1/forms/{pk}/submit` | **Diterima** — lead tetap masuk, `201` seperti biasa |

**Kenapa form publik tidak ditolak.** Dua alasan yang keduanya mengikat:

1. **Pengunjung situs pelanggan bukan pihak dalam hubungan billing kita.** Menolak submit berarti
   orang yang sedang mencoba menghubungi sebuah toko melihat kegagalan karena tagihan toko itu —
   persis yang Phase 8 §11 dan kriteria #8 phase ini larang.
2. **Kehilangan lead sungguhan lebih mahal daripada kehilangan pendapatan overage.** Pelanggan yang
   kehilangan prospek karena form-nya mati akan berhenti berlangganan, bukan naik paket.

Yang menggantikan penolakan: **Owner diberi tahu**, dan kanal-kanal berbayar tetap tertutup untuk
mereka (mereka tidak bisa menambah form/API key/webhook baru, dan tidak bisa mengundang orang) sampai
naik paket. Tekanannya ada, tapi tidak dibayar oleh pengunjung.

**Pemberitahuannya memakai `internal/notification` yang sudah ada** (Phase 2) — satu notifikasi
in-app saat lead pertama melewati batas dalam satu bulan, **bukan** per lead (kalau tidak, melewati
batas 500 lead menghasilkan 500 notifikasi). Ambang "sudah pernah diberi tahu bulan ini" dibaca dari
notifikasi itu sendiri, bukan kolom baru.

### 5.1 Error code baru

| Status | Code | Kapan |
|---|---|---|
| `403` | `plan_quota_exceeded` | Kuota lead bulan berjalan habis, principal `user` atau `api_key` |
| `403` | `plan_seat_limit_reached` | Mengundang anggota melebihi batas seat paket |

Keduanya **menyebut angkanya** di `message` (mis. *"Paket Free dibatasi 100 lead per bulan."*) —
berbeda dari `plan_upgrade_required` (#113) yang sengaja kabur. Alasannya: di sana kevaguan
melindungi keadaan billing dari orang yang tidak berhak tahu; di sini yang menerima pesan **adalah**
pelanggannya sendiri, dan angka yang disembunyikan cuma bikin bingung.

---

## 6. Batas seat

Ditegakkan di **`invitation.Usecase.Create`** — titik satu-satunya di mana jumlah anggota bisa
bertambah (registrasi membuat organization + owner sekaligus, dan itu selalu 1 seat).

```
used = COUNT(memberships aktif) + COUNT(invitations pending)
```

**Undangan yang belum diterima ikut dihitung.** Tanpa itu, organization berbatas 2 seat bisa mengirim
lima undangan sekaligus dan berakhir dengan lima anggota — batasnya dilewati tanpa satu pun cek gagal.
Undangan yang **kedaluwarsa, dicabut, atau sudah diterima** tidak dihitung (yang diterima sudah jadi
membership, dan menghitung keduanya berarti menghitung dua kali).

Mengaktifkan kembali membership yang dinonaktifkan **juga** menambah seat — kalau `membership` punya
jalur reaktivasi, ia titik pasang kedua. Diperiksa saat issue dikerjakan; kalau jalurnya tidak ada,
dicatat sebagai tidak ada, bukan diasumsikan.

---

## 7. `GET /v1/me` — pemakaian ikut, sudah diselesaikan

```json
{
  "data": {
    "role": "owner",
    "plan": {
      "code": "free",
      "channels": { "api_key": true, "form": true, "webhook": true },
      "limits":   { "leads_per_month": 100, "seats": 2 },
      "usage":    { "leads_this_month": 37, "seats_used": 2 },
      "test_checkout_available": false
    }
  }
}
```

| Ketentuan | Alasan |
|---|---|
| `usage` dihitung server, bukan klien | Sama dengan `channels` (Phase 8 D5): dashboard merender jawaban, tidak pernah menghitung. Kalau klien menghitung, "berapa yang terpakai" punya dua implementasi yang pasti menyimpang |
| `limits: 0` berarti **tanpa batas** | Satu arti untuk nol di kedua sisi (§2.1); `lib/plan.ts` mendapat helper yang menyatakannya, bukan `if (limit === 0)` tersebar di komponen |
| `test_checkout_available` ikut di sini | Satu sumber kebenaran untuk "deployment ini mengizinkan checkout test", supaya flag frontend dan backend tidak bisa menyimpang (§8) |
| Tidak ada endpoint `GET /v1/subscription` baru | `SessionGate` sudah memanggil `/v1/me` di setiap layar (Aturan #27) |

**Biaya tambahannya dua `COUNT` per panggilan `/v1/me`** — dan `/v1/me` dipanggil di **setiap** layar
terproteksi. Itu nyata dan harus disebut: keduanya query terindeks atas satu organization, tapi kalau
`EXPLAIN` (§4.2) menunjukkan biayanya tidak sepele, opsi pertama adalah **menghitung `usage` hanya di
layar Langganan** (endpoint terpisah) dan membiarkan `/v1/me` hanya membawa `limits`. Diputuskan
dengan angka, bukan firasat.

---

## 8. Perubahan paket — dua jalur, keduanya bukan tombol bebas

| Jalur | Siapa | Kapan | Penjaga |
|---|---|---|---|
| `POST /internal/subscriptions/{organization_id}/plan` | Pemilik produk (di luar aplikasi) | Sekarang — pelanggan bayar transfer/WA | Bearer token `SUBSCRIPTION_ADMIN_TOKEN`, dibandingkan `subtle.ConstantTimeCompare` (Aturan #20). **Bukan** sesi user |
| `POST /v1/subscription/test-checkout` | Owner | Hanya saat `SUBSCRIPTION_TEST_CHECKOUT=true` | Route **tidak didaftarkan sama sekali** kalau flag mati → `404`, bukan `403` |

**`SUBSCRIPTION_TEST_CHECKOUT=true` + `APP_ENV=production` → boot gagal.** Pola persis
`WEBHOOK_ALLOW_PRIVATE_TARGETS` (#100) dan `CAPTCHA_PROVIDER=none` (#87): konfigurasi yang aman di dev
tapi fatal di produksi ditolak di `config.Validate()`, sebelum HTTP server menerima satu koneksi
(ADR-010).

Kenapa dua jalur, bukan satu yang di-flag: keduanya menjawab kebutuhan berbeda dan punya penjaga
berbeda. Menyatukannya berarti satu endpoint yang kadang butuh token admin dan kadang tidak —
percabangan otorisasi di dalam satu handler, persis yang paling mudah salah.

Keduanya:
- menulis `audit_log` (`subscription.plan_changed`, dengan `old_values`/`new_values`), aktor `NULL`
  untuk jalur internal (bukan tindakan membership manapun) — bentuk yang sudah dipakai #11/#22;
- **tidak** menyentuh `external_reference` — kolom itu tetap menunggu payment service (prd D6);
- memvalidasi `plan_code` tujuan terhadap `planLimits`/`planChannels`: paket yang tidak ada di peta
  **ditolak**, supaya salah ketik tidak diam-diam menurunkan pelanggan ke perilaku "paket tak dikenal"
  (§2.1).

---

## 9. Dashboard — layar **Langganan**

Item sidebar baru (`lib/nav.ts`), rute `/subscription`. Isi:

| Bagian | Isi |
|---|---|
| Paket aktif | `plan.code` ditampilkan sebagai nama ("Free"/"Pro"/"Enterprise") — **pembaca pertama `plan.code`**, menutup poin terbuka `docs/issues/112` |
| Pemakaian | Dua baris: lead bulan ini terhadap `limits.leads_per_month`, anggota terhadap `limits.seats`. Tanpa batas ditampilkan sebagai "tanpa batas", bukan "0" |
| Perbandingan paket | Tiga kolom dari satu sumber: apa yang backend kirim + tabel statis nama/harga. **Bukan** peta paket→kapabilitas versi TypeScript (Phase 8 kriteria #6 tetap berlaku) |
| Aksi | Pro → tombol test checkout **hanya bila** `plan.test_checkout_available`; Enterprise → tautan keluar (WhatsApp/email), bukan checkout (prd D4) |

> ⚠️ **Koreksi atas baris "Perbandingan paket" (Aturan #30, ditambahkan #126).** Baris itu
> mengandaikan angka paket lain sudah tersedia di klien dan tinggal digabung dengan "tabel statis
> nama/harga". **Keduanya keliru:** `GET /v1/me` hanya membawa paket organization yang sedang login,
> dan sebuah tabel statis nama/harga di TypeScript justru **melanggar** kriteria #9 phase ini (angka
> hidup hanya di satu peta Go + satu dokumen).
>
> Ditemukan sebelum kode #125 ditulis dan dibawa ke pemilik produk sebagai tiga opsi; yang dipilih:
> **backend mengirim katalog** lewat endpoint baru **`GET /v1/plans`** (digerbangi
> `subscription.read`, Owner+Admin), berisi nama, label harga, limit, dan kanal untuk **setiap** paket
> dalam urutan tampilan. Dashboard merender apa adanya dan tidak menyimpan satu angka pun. Bentuk
> lengkapnya di `architecture/api.md` bagian *`GET /v1/plans` — katalog, bukan keadaan organization*.
>
> Konsekuensi yang dicatat jujur: #125 berlabel `dashboard` tapi ikut menyentuh Go
> (`docs/issues/125`).

**Gerbang role:** Owner dan Admin bisa **melihat**; hanya **Owner** yang bisa memicu perubahan paket.
Alasan: tagihan urusan pemilik, tapi Admin yang mengelola operasional harus bisa melihat kenapa
undangan ditolak. Ditegakkan di usecase, bukan hanya di UI (Aturan #10) — `subscription.read`
menggerbangi `GET /v1/plans`, `subscription.change` menggerbangi test checkout.

Layar-layar yang sudah ada bertambah satu keadaan: **`403 plan_quota_exceeded` di form buat lead** dan
**`403 plan_seat_limit_reached` di dialog undang** ditampilkan inline dengan pesan backend apa adanya,
pola `plan_upgrade_required` (#114) — plus satu tautan ke `/subscription`.

---

## 10. Konfigurasi

| Env | Default | Catatan |
|---|---|---|
| `SUBSCRIPTION_ADMIN_TOKEN` | — (kosong) | Kosong → route `/internal/...` tidak didaftarkan. Min 32 byte kalau diisi |
| `SUBSCRIPTION_TEST_CHECKOUT` | `false` | `true` + `APP_ENV=production` → boot gagal |

**Tidak ada env untuk angka paket** — angka ada di `planLimits` (prd D7), bukan konfigurasi.

---

## 11. Otorisasi

**Satu `Action` baru:** `subscription.read` (Owner + Admin) untuk layar Langganan. Perubahan paket
lewat `/v1/subscription/test-checkout` memakai `subscription.change` (Owner saja).

Jalur `/internal/...` **tidak lewat `authz` sama sekali** — ia tidak punya principal user. Penjaganya
token, dan itu perbedaan yang harus ditulis di `authorization.md`: ini permukaan pertama di produk yang
terautentikasi **bukan** sebagai siapa pun di dalam sebuah organization.

Kuota **bukan** `Action` — sama alasannya dengan paket di Phase 8 §10: ia dimensi lain, bukan baris di
matriks role.

---

## 12. Rencana test

| Berkas | Menguji |
|---|---|
| `internal/subscription/plan_test.go` | Tabel atas **seluruh** paket × `Limits`; paket tak dikenal → limit `free` (**bukan** nol, §2.1); `status` non-`active` → sama; `0` berarti tanpa batas |
| `internal/lead/usecase_unit_test.go` | Kuota habis → `403 plan_quota_exceeded` untuk `user` & `api_key`; **`public_form` tetap `201`** (§5) — tiga principal, satu tabel |
| `internal/lead/repository_test.go` | Postgres asli: `CountCreatedThisMonth` **menghitung lead yang sudah di-soft-delete**, dan **tidak** menghitung bulan lalu maupun organization lain (isolasi tenant) |
| `internal/invitation/usecase_unit_test.go` | Seat penuh → `403 plan_seat_limit_reached`; **undangan pending ikut dihitung** |
| `internal/auth/handler_session_test.go` | `/v1/me` membawa `limits` + `usage` + `test_checkout_available`; angka `usage` berubah setelah lead dibuat |
| `cmd/api/plan_quota_test.go` | Wiring produksi asli (`newRouter`): organization di batas → `POST /v1/leads` (user & API key) `403`, submit form publik **`201`**, `GET` apa pun tetap `200`. **Terbukti bisa gagal**: batas dinaikkan → ketiganya `201` |
| `cmd/api/subscription_admin_test.go` | `/internal/...` tanpa token → `401`; token salah → `401`; token benar → paket berubah + baris `audit_log`; `SUBSCRIPTION_TEST_CHECKOUT=false` → `/v1/subscription/test-checkout` **`404`** |
| `internal/shared/config/config_test.go` | `SUBSCRIPTION_TEST_CHECKOUT=true` + `APP_ENV=production` → `Load` gagal |
| `crm_dashboard/src/lib/plan.test.ts` | `limits: 0` → "tanpa batas"; pemakaian melebihi batas tidak menghasilkan angka negatif atau >100% yang aneh |

### Verifikasi manual wajib

Bagian baru pada `docs/testing/flow/` yang sudah ada (bukan berkas baru, pola #115): turunkan kuota
sebuah organization ke angka kecil lewat `/internal/...`, lalu buktikan lewat `curl` bahwa jalur user
dan API key ditolak sementara **submit form publik tetap diterima** — itu satu-satunya cara melihat
keputusan §5 benar-benar terpasang, dan bukan sesuatu yang bisa dibaca dari UI.

---

## 13. Risiko teknis

| Risiko | Penanganan |
|---|---|
| **Kuota gagal terbuka** — paket tak dikenal jadi tanpa batas | §2.1: paket tak dikenal → limit `free`, diuji eksplisit. `0` hanya berarti tanpa batas bagi paket yang **memang** ada di peta |
| **Kuota gagal tertutup terlalu keras** — kegagalan billing menghentikan penerimaan lead | §2.1 memilih limit `free`, bukan nol. Diuji: `status='past_due'` tetap bisa membuat lead sampai batas free |
| **`COUNT` jadi mahal** | §4.2 — `EXPLAIN` dicatat sebagai bukti, bukan asumsi; jalan keluarnya satu index parsial |
| **`/v1/me` melambat** karena dua `COUNT` di setiap layar | §7 — kalau terukur mahal, `usage` pindah ke endpoint layar Langganan saja |
| **Test checkout bocor ke produksi** | §8 — boot gagal, dan route tidak didaftarkan; diuji di `config_test.go` |
| **Token admin bocor** | Bandingkan `subtle.ConstantTimeCompare`, tidak pernah di-log (Aturan #26), tidak pernah ada di klien (Aturan #23) |
| **Angka provisional bertahan selamanya** | ADR-014 ketentuan 1–3: ditandai di kode, wajib ditinjau setelah 3–5 pelanggan berbayar, dan `STATUS.md` mempertahankan barisnya sebagai terbuka |
| **Kuota terlampaui di bawah konkurensi** | prd D2 — diterima 1–2 baris, dicatat, tidak dikunci |

---

## 14. Yang harus disiapkan pemilik produk

> ✅ **Ditutup 5 September 2026 (#126).** Angkanya diisi: Free 100 lead / 2 seat, Pro 2.000 lead /
> 10 seat, Enterprise tanpa batas, Pro **Rp99.000/bulan**, kanal terbuka di semua paket. Tabel di
> `prd.md` bagian *Angka provisional* adalah cerminannya. Kewajiban meninjau ulang setelah 3–5
> pelanggan **berbayar** pertama tetap berlaku (ADR-014 ketentuan 2) dan hidup di `STATUS.md`.
>
> ⚠️ **Koreksi mekanisme (Aturan #30).** Paragraf di bawah menjanjikan nilai `TODO` yang "sengaja
> tidak masuk akal untuk produksi" dan **test yang sengaja gagal**. Yang dibangun #122 berbeda dan
> disengaja: `planLimits` diisi angka yang masuk akal sejak awal, ditandai konstanta
> `LimitsAreProvisional`, dan **boot produksi yang menolak jalan** selama konstanta itu `true`
> (ADR-010) — bukan test merah. Alasannya: CI merah yang "memang seharusnya merah" selama empat issue
> melatih orang mengabaikan warna merah, sementara boot yang gagal tidak bisa diabaikan. Sejak #126
> konstanta itu `false`, dan dua test **hijau** yang menjaganya:
> `TestLimitsAreNoLongerProvisional` dan `TestPlanDisplay_NoPlaceholderPriceLabels` — keduanya merah
> lagi kalau putaran angka berikutnya ditandai provisional dan lupa diselesaikan.

**Satu hal, dan ia memblokir rilis (bukan implementasi):** mengisi tabel angka provisional di
`prd.md` — kuota lead Free/Pro, batas seat, kanal per paket, dan label harga Pro. Sampai diisi,
`planLimits` memakai nilai `TODO` yang **sengaja tidak masuk akal untuk produksi**, dan test yang
mengunci "angka sudah diisi" gagal — supaya rilis dengan placeholder tidak mungkin terjadi diam-diam.

Nomor WhatsApp / alamat email untuk kartu Enterprise juga dibutuhkan (prd D4), dan itu satu string.
**Masih belum diisi per #126** — kartu Enterprise sementara menampilkan teks "Hubungi kami untuk
diskusi harga" **tanpa tombol** (tombol yang tidak menuju ke mana pun lebih buruk daripada tidak ada
tombol). Poin terbuka di `docs/issues/125`.

---

## 15. Yang berubah pada dokumentasi

| Berkas | Perubahan |
|---|---|
| `architecture/api.md` | Dua error code baru; bab *Gerbang Paket* bertambah bagian **Kuota** — apa yang dihitung, jalur mana yang ditolak dan mana yang tidak (§5), dan kenapa |
| `architecture/authorization.md` | `Action` baru; bagian baru tentang **permukaan `/internal/`** — terautentikasi token, bukan sebagai principal manapun (§11) |
| `architecture/multi-tenancy.md` | `CountCreatedThisMonth` adalah query lintas-baris pertama yang dipakai untuk **keputusan**, bukan tampilan — tetap tenant-scoped, dicatat |
| `docs/testing/flow/` | Bagian pada berkas yang ada: prosedur kuota §12 |
| `ADR-012` | ✅ **Sudah dilakukan saat phase dibuka** — anotasi penunjuk ke ADR-014 di §4 (mekanisme yang sama seperti Aturan #20 → ADR-013). Isinya tidak diubah. Dikerjakan di muka, bukan di penutup, karena ADR-014 sudah berlaku sejak issue pertama |
| `STATUS.md` | Baris Selesai; *Progress per Phase* + baris 8.5; *Keputusan Belum Diambil* — pemicu pricing berubah dari "gate freeze" ke "3–5 pelanggan berbayar pertama" |

`freeze.md` **tidak disentuh** — penyimpangan nomor phase dan urutan gate keduanya dicatat di `prd.md`
dan ADR-014 (Aturan #30).

---

## 16. Kewajiban yang diteruskan ke phase berikutnya

- **Saat payment service tersambung** (phase akhir): webhook pembayaran memanggil jalur perubahan paket
  yang **sudah ada** (§8) — bukan alur baru. `external_reference` mendapat pembacanya. Tombol test
  checkout dihapus bersamaan, bukan dibiarkan hidup di samping checkout sungguhan.
- **Setelah 3–5 pelanggan berbayar pertama** (ADR-014): tinjau ulang kuota Free, kuota Pro, harga Pro,
  dan batas seat **bersama-sama**.
- **Saat ada bukti kuota selain lead & seat jadi biaya nyata** (penyimpanan, webhook terkirim):
  `Limits` bertambah field, peta bertambah kolom — test tabel §12 otomatis mencakupnya.
- **Kalau `EXPLAIN` §4.2 menunjukkan `COUNT` mahal**: satu index parsial, dicatat di `notes.md` beserta
  angkanya — bukan diputuskan ulang dari nol.
