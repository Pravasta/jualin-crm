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

---

## #124 — Batas seat + dua jalur perubahan paket (token internal & test checkout)

Mengikuti TD §6, §8, §10, §11. Satu pemeriksaan yang issue minta secara eksplisit dicatat (bukan
diasumsikan), satu penyimpangan dari isi issue (bukan sekadar TD) yang perlu disorot, dan satu
keputusan desain yang tidak eksplisit ditutup TD.

### Pemeriksaan yang diminta issue — jalur reaktivasi membership: **tidak ada**

Issue eksplisit meminta: *"Periksa apakah ada jalur reaktivasi membership; kalau ada, itu titik pasang
kedua. Kalau tidak ada, catat bahwa tidak ada — jangan diasumsikan."* Diperiksa lewat pencarian
menyeluruh (`grep -rn "[Rr]eactivat" internal/membership internal/invitation`) — nol hasil. Satu-
satunya jalan jumlah anggota aktif bertambah tetap `invitation.Usecase.Create` diikuti penerimaan
undangan; tidak ada endpoint atau method yang menghidupkan kembali membership yang sudah nonaktif.
Titik pasang tunggal (`invitation.Usecase.Create`) sudah benar tanpa titik kedua.

### Penyimpangan — `Action` baru hanya `subscription.change`, **`subscription.read` tidak dibuat**

Issue menulis dua Action baru: `subscription.change` (Owner) **dan** `subscription.read`
(Owner+Admin). Hanya yang pertama dibuat. Alasan: `subscription.read` tidak punya pemanggil nyata —
layar Langganan (#125) sepenuhnya dilayani `GET /v1/me` yang sudah terbuka sejak #112 untuk seluruh
role (data paket bukan rahasia dari anggota organization sendiri), dan tidak ada endpoint baca-detail-
paket terpisah yang direncanakan TD manapun. Menambah `Action` dengan **nol pemanggil permanen**
melanggar Aturan #27/#28 secara langsung — folder kosong dan Action tak terpakai adalah utang yang
sama bentuknya. Diputuskan untuk tidak membuatnya sekarang; kalau kelak ada endpoint yang benar-benar
butuh membedakan "boleh lihat" dari "boleh ubah", menambah satu `Action` adalah perubahan kecil, bukan
migrasi arsitektur. `subscription.change` sendirian sudah menjadi **`Action` Owner-only pertama** di
seluruh codebase ini — setiap `Action` sebelumnya, Admin selalu mewarisi apa pun yang Owner punya;
`authz_test.go` diperbarui eksplisit untuk kasus ini (satu baris komentar menjelaskan kenapa Admin
`false` di sini).

### Bentuk implementasi — batas seat

- **`used = membership.CountActive + invitation.CountPendingSeats`**, dijumlahkan di
  `invitation.Usecase.Create` sendiri (bukan di dalam gate) — pola identik #123's
  `lead.PlanQuota`: gate memiliki batas, domain memiliki angka. `CountActive` dan `CountPendingSeats`
  sama-sama sudah ada sejak #122, tapi **keduanya belum pernah punya test repository langsung** —
  celah yang baru terasa sekarang karena #124 adalah pemanggil nyata pertama yang bergantung pada
  ketepatan predikatnya (terutama "tidak menghitung yang kedaluwarsa"). Ditutup di issue ini:
  `internal/membership/repository_test.go` (+3 test) dan `internal/invitation/repository_test.go`
  (berkas baru, +3 test).
- **Urutan tetap `authz.Require` → batas seat → validasi role undangan** — role check (Owner tidak
  bisa diundang lewat jalur ini) datang **setelah** batas seat, bukan sebelum: mengetahui organization
  penuh lebih berguna bagi si pengundang daripada mengetahui role targetnya salah, dan keduanya sama-
  sama `4xx` jadi urutan tidak mengubah keamanan — hanya pesan mana yang dilihat lebih dulu.
- **`invitation.PlanSeatQuota`** (consumer-declared, ADR-011) — `AllowSeat(ctx, t, used) error`, dan
  `subscription.Usecase.RequireSeatLimit` bentuknya identik `RequireLeadQuota` #123, pesan menyebut
  angka (`"Paket Anda dibatasi %d anggota"`).

### Bentuk implementasi — dua jalur perubahan paket

- **`subscription.Usecase.AdminChangePlan`** — satu method dipakai **kedua** jalur (admin token dan
  test checkout), bukan dua usecase terpisah: keduanya sama-sama "ubah `plan_code` organization,
  validasi dulu, kembalikan yang lama untuk audit". Test checkout mengunci tujuannya ke
  `subscription.PlanPro` (hardcoded) — bukan menerima `plan_code` dari body, karena tombolnya memang
  cuma "coba Pro", bukan pemilih paket bebas.
- **Route tidak terdaftar sama sekali saat nonaktif** — pola yang sama `WEBHOOK_ALLOW_PRIVATE_TARGETS`
  #100 pakai: `if adminToken == "" { return }` / `if !enabled { return }` sebelum `r.Group(...)`,
  supaya fitur mati menghasilkan `404` (rute benar-benar tidak ada), bukan `401`/`403` dari middleware
  yang menjaga rute yang tak seorang pun bisa lewati.
- **`subscriptionAdminAuth`** — `subtle.ConstantTimeCompare` atas header `Authorization: Bearer
  <token>`, pola yang sama seperti kredensial API key (#46) meski jenis principalnya beda:
  `tenant.PrincipalSystem` dipakai untuk pertama kalinya di codebase ini — permukaan pertama yang
  terautentikasi bukan sebagai `user`, `api_key`, maupun `public_form`.
- **`auditlog.Repository.RecordChange`** — penulis pertama untuk kolom `old_values`/`new_values`/
  `entity_type`/`entity_id` di `audit_logs`, yang sudah ada sejak `0002_identity.sql` tapi belum
  pernah ditulis satu pun baris sampai sekarang.
- **Guard boot `SUBSCRIPTION_TEST_CHECKOUT=true` + `APP_ENV=production` → gagal** — pola identik
  `WEBHOOK_ALLOW_PRIVATE_TARGETS`/`CAPTCHA_PROVIDER=none`. `SUBSCRIPTION_ADMIN_TOKEN` opsional, tapi
  kalau diisi wajib ≥32 byte — pola identik `FormTokenSecret`/`WebhookSecretEncKey`.

### Keputusan yang TD tidak eksplisit tutup — perubahan paket + audit log **tidak atomik**

`AdminChangePlan` menulis `subscriptions.plan_code`, lalu `recordPlanChangeAudit` menulis
`audit_logs` — **dua statement terpisah, tanpa transaksi bersama**. Dipertimbangkan membungkus
`internal/subscription` dengan `Store`/`Repos`/`InTx` penuh (menyentuh 11 call site
`subscription.NewUsecase(repo)` yang sudah ada), tapi diputuskan tidak sepadan untuk aksi admin yang
jarang terjadi, dipicu manual, dan sudah dijaga token — Aturan #27. Perubahan paket adalah efek yang
penting; kegagalan menulis baris audit setelahnya (secara praktik nyaris tidak pernah terjadi pada
`INSERT` satu baris ke tabel yang sama) tidak boleh membatalkan perubahan paket yang sudah berhasil.
Ditulis sebagai komentar di `recordPlanChangeAudit`, bukan hanya di sini.

### Verifikasi — dijalankan, bukan diasumsikan

`cmd/api/subscription_admin_test.go` terhadap router produksi asli:
- Tanpa token/token salah → `401`; token benar → `200` + `plan_code` berubah di database + **tepat
  satu** baris `audit_logs` (`action = 'subscription.plan_changed'`) — dibaca langsung dari Postgres,
  bukan diasumsikan dari respons HTTP
- `plan_code` tak dikenal → `400` di kedua jalur
- Isolasi tenant: mengubah organization A tidak menyentuh `plan_code` organization B
- `SUBSCRIPTION_ADMIN_TOKEN=""` / `SUBSCRIPTION_TEST_CHECKOUT=false` → route benar-benar tidak
  terdaftar (`404`, bukan `401`/`403`)
- Test checkout: Owner → `200` + upgrade ke `pro`; Admin → `403` (Action Owner-only pertama)
- **Token tidak pernah muncul di log** — dibaca dari buffer log sungguhan setelah percobaan gagal dan
  berhasil, bukan dipercaya dari membaca kode (`TestSubscriptionAdmin_TokenNeverLogged`)

`go test -race ./...` bersih, `golangci-lint run` 0 issues. Tanpa migration baru.
