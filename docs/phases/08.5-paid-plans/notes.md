# Phase 8.5 — Paket Berbayar & Kuota · Implementation Notes

> Realitas implementasi, satu bagian per issue. Penyimpangan dari TD/PRD beserta alasannya,
> utang teknis, catatan untuk session berikutnya. Naratif lengkap — berbeda dari `docs/issues/12*`
> yang hanya checklist penutupan phase.

---

## #122 — Peta limit paket: kuota + seat, `limits`/`usage` di `GET /v1/me`

Mengikuti TD §2, §3, §7. Empat hal yang perlu dicatat — dua penyimpangan dari TD, satu pengukuran, dan
satu keputusan yang TD tidak eksplisit tutup.

### Penyimpangan 1 — penjaga angka provisional: boot gagal, bukan test merah

TD §14 menulis *"test yang mengunci 'angka sudah diisi' gagal — supaya rilis dengan placeholder tidak
mungkin terjadi diam-diam"*. **Tidak diikuti**, dan diganti dengan yang lebih kuat: konstanta
`subscription.LimitsAreProvisional` + penjaga di composition root — `APP_ENV=production` sementara
konstanta itu masih `true` → **proses berhenti sebelum HTTP server menerima satu koneksi** (ADR-010).

Alasannya: test yang sengaja merah membuat CI merah **sejak #122 sampai #126**, dan CI merah yang
"memang seharusnya merah" melatih orang mengabaikan warna merah — persis kebiasaan yang paling mahal
untuk dipulihkan. Boot yang menolak produksi tidak bisa diabaikan, tidak bisa lupa, dan bentuknya
sudah dipakai tiga kali di codebase ini (`WEBHOOK_ALLOW_PRIVATE_TARGETS` #100, `CAPTCHA_PROVIDER=none`
#87, dan `SUBSCRIPTION_TEST_CHECKOUT` yang menyusul di #124). Pesan errornya menyebut persis tiga
langkah untuk membereskannya.

### Penyimpangan 2 — `planChannels` untuk `pro`/`enterprise` diisi sama dengan `free`

TD §2 menyiratkan kanal ikut membedakan paket. **Untuk sekarang ketiganya membuka ketiga kanal.**
Menutup kanal yang hari ini `free` sudah buka berarti **mengambil sesuatu dari organization yang sudah
memakainya** — perilaku downgrade yang Phase 8 D4 tolak bangun justru karena jalurnya belum ada.
Kanal mana yang membedakan paket adalah bagian dari keputusan angka provisional yang masih ditunggu
dari pemilik produk; sampai itu diisi, **pembeda satu-satunya adalah `planLimits`**. Ditulis sebagai
komentar di `plan.go`, bukan hanya di sini.

### Pengukuran — `EXPLAIN` yang TD §4.2 minta, dijalankan

Query hitung tidak bisa memakai `ix_leads_org_created`: **keempat** index `leads` bersifat parsial
(`WHERE deleted_at IS NULL`, migration `0003`), sementara penghitung kuota justru **harus** menghitung
baris yang sudah di-soft-delete. Dijalankan terhadap Postgres asli, 2.000 lead milik organization uji
+ 2.000 milik organization lain, setelah `ANALYZE`:

```
Aggregate  (cost=160.83..160.84 rows=1) (actual time=1.735..1.735 rows=1)
  ->  Nested Loop  (actual time=0.329..1.621 rows=2000)
        ->  Index Scan using organizations_pkey on organizations  (actual time=0.007..0.007 rows=1)
        ->  Seq Scan on leads  (actual time=0.021..0.297 rows=2000)
              Filter: (organization_id = '…')
              Rows Removed by Filter: 2000
Buffers: shared hit=58        Execution Time: 1.762 ms
```

**Diputuskan (a) — biarkan, tanpa index baru.** 1,76 ms untuk 4.000 baris, 58 buffer hit, seluruhnya
dari cache. Seq scan di sini bukan tanda masalah: pada tabel sekecil itu planner memang memilihnya
terlepas dari index apa pun. Ambang untuk meninjau ulang: saat satu organization mendekati puluhan
ribu lead per bulan — di situ `(organization_id, created_at)` **non-parsial** adalah satu migration
kecil, bukan perubahan desain. Dicatat sebagai kewajiban di `docs/issues/122`.

### Keputusan yang TD tidak eksplisit tutup — `seats_used` menghitung undangan pending

TD §6 mewajibkan undangan pending ikut dihitung saat **penegakan** (#124), tapi tidak menyebut apakah
angka yang **ditampilkan** di `/v1/me` harus sama. Diputuskan: **sama**. Kalau layar bilang "2 dari 2
terpakai" lalu undangan ketiga ditolak, itu terbaca seperti bug; kalau layar bilang "3 dari 2", itu
jujur dan menjelaskan dirinya sendiri. Konsekuensinya `Me` menjumlahkan **dua** penghitung
(`membership.CountActive` + `invitation.CountPendingSeats`), dan #124 wajib menjumlahkan **dua yang
sama** — duplikasi aritmetika kecil yang disengaja, dicatat supaya tidak menyimpang.

`CountPendingSeats` predikatnya **lebih ketat** dari `FindByOrgPending` yang sudah ada: ia juga
membuang undangan kedaluwarsa. Undangan yang tidak mungkin diterima lagi tidak boleh menahan seat —
sementara `FindByOrgPending` sengaja tetap menampilkannya supaya layar bisa menandainya kedaluwarsa.

### Catatan lain

- **`ResolvePlan` mengembalikan `subscription.Limits`, bukan dua `int`.** `internal/auth` memang sudah
  mengimpor `internal/subscription` (untuk `*subscription.Subscription`), jadi tidak ada yang
  dihindari dengan meratakannya — sementara dua `int` posisional adalah call site di mana `leads` dan
  `seats` tertukar tanpa compiler protes.
- **`Unlimited = 0` dibaca lewat `allows()`, tidak pernah dibandingkan langsung.** Arti "0 = tanpa
  batas, bukan nol jatah" hidup di satu baris kode produksi, dan satu test yang menyatakannya.
- **`test_checkout_available` di-hardcode `false`** — field-nya dipasang sekarang supaya bentuk
  `/v1/me` stabil untuk #125, tapi endpoint dan flag config-nya milik #124. Dikunci test supaya tidak
  diam-diam jadi `true`.
- **Tiga `COUNT` per panggilan `/v1/me`**, dan `/v1/me` dipanggil di setiap layar terproteksi. Biaya
  ini disebut di TD §7 beserta jalan keluarnya (pindahkan `usage` ke endpoint layar Langganan) —
  belum diambil karena belum terukur mahal; angkanya di atas.
- **Nol pemanggil `allows()` di luar test.** Penegakan adalah #123/#124 — bentuk yang sama dengan
  #112's `RequireChannel`, `apikey.FindByKeyID` (#46), `form.FindByPublicKey` (#85).

`go test -race ./...` bersih, `golangci-lint run` 0 issues, tanpa migration.

---

## #123 — Kuota lead: satu titik penegakan untuk tiga jalur + `plan_quota_exceeded`

Mengikuti TD §3, §4, §5 setelah D3 ditutup (5 September 2026: form publik saat kuota habis **tetap
menerima**). Satu penyimpangan besar dari klaim awal phase ini, satu keputusan desain yang tidak
eksplisit ditulis TD, dan satu catatan verifikasi.

### Penyimpangan — TD §1 keliru: phase ini **butuh** satu migration

Ditemukan sebelum kode ditulis, dilaporkan ke pemilik produk (Aturan #30), bukan diam-diam
diselesaikan: `notifications.type` punya `CONSTRAINT ck_notifications_type CHECK (type IN
('lead_assigned','task_assigned'))` sejak `0004_notifications.sql`. TD §5 menjanjikan notifikasi Owner
lewat `internal/notification` yang sudah ada — tapi tipe ketiga butuh migration untuk masuk daftar
yang diizinkan. Tiga jalan diajukan ke pemilik produk; **migration kecil** dipilih.
**`migrations/0010_notification_plan_quota.sql`** menambah `'plan_quota_exceeded'` ke constraint.
`td.md` §1 dikoreksi di tempat — bukan dihapus diam-diam — supaya pembaca berikutnya tahu klaim
"tanpa migration" sempat salah dan kenapa.

### Bentuk implementasi

- **`lead.PlanQuota`** (consumer-declared, ADR-011) — `AllowLead(ctx, t, used) error`. `used` datang
  dari **pemanggil** (lead menghitung lewat `CountCreatedThisMonth` #122), bukan dihitung di dalam
  gate — gate memiliki **batas**, domain memiliki **angka**; kalau gate yang menghitung, ia harus
  mengimpor `internal/lead`, membalik arah dependensi.
- **`subscription.Usecase.RequireLeadQuota`** — bentuk sama `RequireChannel`, tapi **pesannya
  menyebut angka** (`"Paket Anda dibatasi 100 lead per bulan"`), sengaja beda dari `RequireChannel`
  yang kabur: audiens `RequireChannel` bisa jadi Manager yang tidak berhak tahu apa pun soal paket;
  audiens `RequireLeadQuota` selalu principal organization itu sendiri (`user`/`api_key`, **tidak
  pernah** `public_form`) bertanya soal jatahnya sendiri.
- **Titik pasang tunggal**: `lead.Usecase.Create`, diverifikasi (bukan diasumsikan) sebelum kode
  ditulis bahwa `cmd/api/form_store.go`'s `leadCreatorAdapter.CreateFromForm` memanggil
  `usecase.Create` yang sama dengan jalur `user`/`api_key`.
- **Urutan**: `authz.Require` → `CountCreatedThisMonth` → `AllowLead` — kuota dihitung untuk
  **ketiga** principal (murah, dan relevan untuk semuanya), tapi hanya **ditegakkan** untuk
  `user`/`api_key`; `public_form` yang gagal kuota tetap lanjut membuat lead, lalu memicu notifikasi.
- **`lead.QuotaNotifier.NotifyQuotaExceededOnce`** — bridge di composition root
  (`cmd/api/quota_notifier.go`) yang menggabungkan `membership.FindActiveOwnerIDs` (baru) +
  `notification.ExistsThisMonth` (baru) + `notification.Notifier.Notify` yang sudah ada. **Ambang
  dicek dulu, baru cari Owner** — kasus umum (sudah pernah diberi tahu bulan ini) membayar satu query,
  bukan satu query plus N insert yang dibatalkan.
- **Ambang organization-wide, bukan per-owner** — co-owner (diizinkan sejak `authorization.md`
  Aturan #4) berbagi satu ambang; menambah Owner kedua di tengah bulan tidak me-reset hitungannya.
  `ExistsThisMonth` memotong bulan di **timezone organization**, konsisten dengan
  `CountCreatedThisMonth` (#122) — "bulan ini" berarti hal yang sama di kedua sisi ambang.
- **Best-effort, tidak pernah gagal atas nama lead** — `_ = u.quotaNotifier.NotifyQuotaExceededOnce(...)`,
  pola persis `_ = push.PushToMembership(...)` yang sudah ada (Phase 5 #68): kegagalan mengirim
  notifikasi tidak boleh membatalkan atau menggagalkan lead yang pengunjung situs sedang tunggu.

### Keputusan yang TD tidak eksplisit tutup

**Notifikasi hanya untuk `public_form`, tidak untuk `user`/`api_key` yang ditolak.** TD §5 menulis
notifikasi sebagai *"pengganti penolakan di jalur publik"*, tapi tidak eksplisit melarang mengirim
notifikasi juga saat `user`/`api_key` ditolak. Diputuskan: **tidak** — `403` yang mereka terima
**adalah** notifikasinya; notifikasi in-app kedua untuk hal yang sudah mereka lihat sendiri hanya
kebisingan. Dikunci test (`TestUnit_Create_Quota_UserAndAPIKey_NeverNotify`).

### Verifikasi — dijalankan, bukan diasumsikan

`cmd/api/plan_quota_test.go` terhadap router produksi asli, peta `planLimits` asli:
- Tiga principal di satu organization, tepat di batas kuota — `user`/`api_key` `403
  plan_quota_exceeded`, `public_form` tetap `201` (`TestPlanQuota_AtLimit_ThreePrincipalsBehaveDifferently`)
- **Notifikasi sungguhan dibaca lewat `GET /v1/notifications`**, bukan fake yang diasumsikan terpanggil:
  dua submit form melewati kuota → **tepat satu** notifikasi `plan_quota_exceeded`
  (`TestPlanQuota_PublicForm_NotifiesOwnerOnceNotTwice`)
- `GET`/`PATCH` lead yang sudah ada tetap jalan saat kuota habis (D4-nya kuota)
- **Terbukti bisa gagal**: organization di batas → `403`; `plan_code` dinaikkan ke `enterprise`
  (tanpa batas) lewat SQL langsung → `201` — jalur kode yang sama membuktikan dua arah, tanpa
  menyabotase kode yang di-commit (`TestPlanQuota_ProvenToFail`)

`go test -race ./...` bersih, `golangci-lint run` 0 issues. Kuota terlampaui di bawah konkurensi
(1–2 baris) **tidak dikunci** — sesuai prd D2, diterima secara sadar, tidak diuji sebagai kegagalan.
