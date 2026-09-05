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

---

## #125 — Dashboard: layar Langganan

Mengikuti TD §7, §9. Satu keputusan yang mengubah bentuk pekerjaan sebelum kode ditulis — dilaporkan
ke pemilik produk lewat `AskUserQuestion`, bukan diputuskan sepihak — dan satu penyimpangan langsung
dari isi issue (bukan hanya TD) yang jadi konsekuensinya.

### Temuan sebelum implementasi — perbandingan tiga paket butuh data yang belum dikirim siapa pun

`GET /v1/me` hanya membawa paket **organization yang sedang login** — angka paket *lain* (berapa
lead/seat yang didapat kalau naik ke Pro) hanya hidup di peta Go, dan harga tidak punya rumah sama
sekali. Tiga jalan diajukan: (a) backend mengirim katalog lewat endpoint baru, (b) layar tanpa angka
konkret untuk paket lain, (c) tunda perbandingan ke issue terpisah. **(a) dipilih** — satu-satunya
opsi yang memenuhi Phase 8 kriteria #6 dan prd 8.5 kriteria #9 (angka paket **tidak pernah** hidup dua
kali, sekali di Go dan sekali di TypeScript) sekaligus benar-benar menjawab kebutuhan prd 8.5 #3
("tahu apa yang saya dapat kalau naik paket, tanpa bertanya"). Konsekuensinya: **#125, berlabel
`dashboard`, ikut menyentuh Go** — penyimpangan langsung dari cakupan issue, dicatat di sini dan di
`docs/issues/125`.

### Penyimpangan — `authz.ActionSubscriptionRead` yang #124 tolak buat, sekarang dibuat

#124 sengaja tidak membuat Action ini karena nol pemanggil nyata saat itu. `GET /v1/plans` adalah
pemanggil nyata pertama: ia mengembalikan angka paket **lain**, bukan cuma milik organization sendiri
(yang `GET /v1/me` sudah buka untuk semua principal tanpa Action apa pun) — jadi penjagaannya memang
baru sekarang jadi relevan. **Owner *dan* Admin**, beda dari `ActionSubscriptionChange` yang Owner-only
— TD §9 eksplisit memberi Admin hak melihat gambaran tagihan meski hanya Owner yang bisa bertindak.

### Bentuk implementasi — katalog paket (Go)

- **`subscription.Catalog()`** — fungsi murni, bukan method `Usecase`, karena ia tidak butuh
  `tenant.Context` sama sekali: katalog menjelaskan apa yang **ditawarkan** tiap paket, sama untuk
  organization mana pun yang bertanya. Dibangun dari **empat** peta di `plan.go`
  (`planChannels`/`planLimits`/`planDisplay`/`planOrder` — dua terakhir baru) sehingga tidak ada
  angka yang di-hardcode ulang di dalamnya.
- **`planDisplay`/`planOrder`** — nama tampilan dan urutan kolom, rumah yang sama prd D7 tetapkan untuk
  angka provisional ("satu peta Go + satu tabel dokumen"). Label harga (`"Rp0"`/`"Segera"`/`"Negosiasi"`)
  sengaja bukan angka pasti — Pro masih `_(?)_` di prd, dan `LimitsAreProvisional = true` yang sudah
  ada **sudah menahan boot produksi** selama itu benar, jadi placeholder ini tidak mungkin diam-diam
  sampai ke pelanggan produksi.
- **Test key-set yang sudah ada** (`TestPlanLimits_EveryPlanHasEntry`, sejak #122) diperluas jadi
  `TestPlanCatalog_AllFourCollectionsAgree` — paket yang ditambahkan ke satu peta dan lupa di peta lain
  sekarang gagal di keempat kombinasinya, bukan cuma dua.
- **`Usecase.ListPlans`** — satu-satunya method di paket ini yang **tidak** memanggil
  `FindActiveByOrg` sama sekali (dikunci `TestUnit_ListPlans_NeverTouchesRepository`): pertanyaan
  "apa yang ditawarkan tiap paket" tidak butuh baris subscription organization mana pun, beda dari
  `ResolvePlan`/`RequireChannel`/`RequireLeadQuota`/`RequireSeatLimit` yang keempatnya membaca itu
  lebih dulu.
- **`internal/subscription/handler_http.go`** (berkas baru) — `GET /v1/plans`, endpoint HTTP pertama
  yang dimiliki paket ini secara langsung; setiap entry point lain sebelumnya dipanggil dari usecase
  domain lain lewat bridge `PlanGate` (ADR-011).

### Bentuk implementasi — dashboard

- **`session.plan` sudah cukup untuk "Paket Aktif" dan "Pemakaian"** — `GET /v1/me` sudah membawa
  `limits`/`usage`/`test_checkout_available` sejak #122/#124, jadi bagian ini nol fetch tambahan.
  Hanya kolom perbandingan yang memanggil `GET /v1/plans` — apa yang ditawarkan paket **lain** memang
  tidak ada di `session.plan`.
- **`lib/plan.ts`** bertambah `planDisplayName` (pembaca pertama `plan.code`, menutup poin terbuka
  `docs/issues/112`, fallback ke kode mentah untuk nilai tak dikenal alih-alih crash),
  `formatLimit`/`formatUsage`/`isUnlimitedLimit` (satu-satunya tempat arti "0 = tanpa batas" dibaca di
  TypeScript, mirip `allows()` di Go), dan `usageRatio` yang **di-clamp `[0, 1]`** — pemakaian yang
  melewati batas (lead dari form publik setelah kuota habis, #123 D3) tidak pernah membuat bar >100%
  atau angka negatif (AC eksplisit, dikunci test).
- **`lib/session-context.tsx` bertambah `useSessionRefresh`** — satu-satunya pemanggilnya adalah
  layar ini: setelah test checkout mengubah `plan.code`, layar harus menunjukkan paket baru **seketika**,
  bukan menunggu navigasi berikutnya (SessionGate hanya memanggil `GET /v1/me` sekali saat mount).
  `useSession()` sendiri tidak berubah bentuk — setiap pemanggil lama tetap jalan tanpa sentuhan.
- **Aksi di kolom paket**: tombol test checkout muncul **hanya** bila `test_checkout_available` **dan**
  `canChangePlan(role)` (Owner) **dan** bukan paket yang sedang aktif. Enterprise **tidak pernah**
  punya tombol — teks *"Hubungi kami untuk diskusi harga"* saja, karena tujuan kontaknya (nomor
  WhatsApp/email) masih placeholder menunggu pemilik produk (keputusan eksplisit sebelum implementasi:
  tombol yang tidak menuju ke mana pun dilarang AC, bukan didisable). Dicatat sebagai poin terbuka di
  `docs/issues/125`.
- **Dua layar lama bertambah keadaan inline**: `new-lead-dialog.tsx` (`plan_quota_exceeded`) dan
  `invite-member-dialog.tsx` (`plan_seat_limit_reached`) — `FormErrorBanner` menampilkan pesan backend
  apa adanya (sudah ada), ditambah satu tautan "Lihat paket & pemakaian" ke `/subscription` yang
  **hanya muncul untuk kode error itu**, bukan untuk error lain yang dialog ini bisa tampilkan.
- **Item sidebar "Langganan" visible untuk SEMUA role** — pola yang sama "Connect" pakai sejak #86:
  gerbang sungguhan (`canViewSubscription`) ada di dalam layar, bukan dengan menyembunyikan item nav.
  Manager/Employee bisa membuka `/subscription` dan melihat "tidak tersedia untuk role Anda" dengan
  **nol** panggilan `GET /v1/plans` — dikunci di dua lapis: `lib/subscription-permissions.test.ts`
  (murni) dan `cmd/api/plan_catalog_test.go` (`403` sungguhan bila gerbang backend saja yang tersisa).

### Verifikasi — dijalankan, bukan diasumsikan (backend); manual (dashboard, diserahkan ke pemilik produk)

`cmd/api/plan_catalog_test.go` terhadap router produksi asli: Owner/Admin `200` tiga paket berurutan
(Free/Pro/Enterprise) dengan nama & label harga terisi; Manager/Employee `403`. `go test -race ./...`
bersih, `golangci-lint run` 0 issues. `npm run typecheck && lint && test && build` bersih — 162 test.

**Verifikasi manual terhadap `crm_be` sungguhan (AC eksplisit) diserahkan ke pemilik produk**, sama
pola #114/#123: turunkan kuota organization lewat `/internal/subscriptions/{id}/plan` atau SQL
langsung, konfirmasi `403 plan_quota_exceeded`/`plan_seat_limit_reached` tampil inline dengan tautan
di layar asalnya, dan layar `/subscription` menunjukkan pemakaian serta perbandingan paket yang benar
di browser. Prosedur lengkap: `docs/issues/125-dashboard-subscription-screen.md`.

---

## #126 — Dokumentasi + penutup Phase 8.5

Penutup phase. Angka provisional diisi pemilik produk, dokumentasi arsitektur disusul, seluruh 10 AC
PRD dicek terhadap bukti nyata, dan satu temuan di luar cakupan dilaporkan tanpa diperbaiki sepihak.

### Angka final — diisi 5 September 2026

| | Free | Pro | Enterprise |
|---|---|---|---|
| Lead / bulan | 100 | 2.000 | tanpa batas |
| Seat | 2 | 10 | tanpa batas |
| Kanal | ketiganya | ketiganya | ketiganya |
| Harga | Rp0 | **Rp99.000/bulan** | negosiasi |

`LimitsAreProvisional` → **`false`**. Penjaga bootnya **tetap terpasang**, bukan dihapus: putaran angka
berikutnya tinggal menyetelnya `true` lagi dan produksi berhenti boot sampai angkanya diputuskan.

**Kanal tidak membedakan paket, dan itu jawaban — bukan kolom yang lupa diisi.** Komentar di
`plan.go` yang dulu berbunyi "menunggu keputusan pemilik produk" diganti dengan alasan keputusannya:
menutup kanal yang Free sudah buka adalah downgrade tanpa jalur, sementara kanal **keempat** bisa
lahir Pro-only tanpa biaya itu. Perbedaan itu penting untuk pembaca berikutnya yang tergoda
"mengambil kembali" salah satu dari tiga kanal yang ada.

**Dua test hijau menggantikan test merah yang TD §14 janjikan** — `TestLimitsAreNoLongerProvisional`
dan `TestPlanDisplay_NoPlaceholderPriceLabels`. Keduanya **dibuktikan bisa gagal**, bukan diasumsikan:
konstanta dibalik ke `true` dan label harga dikembalikan ke `"Segera"`, keduanya merah dengan pesan
yang tepat, lalu dikembalikan dan `git diff` dikonfirmasi hanya memuat perubahan #126 yang disengaja.

### Review 10 AC PRD — terhadap bukti, bukan ingatan

| # | Kriteria | Status | Bukti |
|---|---|---|---|
| 1 | Tiga paket di satu peta, satu berkas dengan `planChannels`; ubah kuota = satu baris | ✅ | `plan.go`: `planLimits`/`planChannels`/`planDisplay`/`planOrder` bersebelahan. `grep` dijalankan #126: **nol** angka kuota di usecase, migration, atau TypeScript produksi |
| 2 | Kuota ditegakkan di **ketiga** jalur, dibuktikan lewat `curl` | ✅\* | `cmd/api/plan_quota_test.go` — request HTTP mentah ke router produksi (setara `curl` di lapis protokol). Prosedur `curl` literal untuk pemilik produk: `09-webhook.md` §9.11 |
| 3 | Batas seat menolak dengan menyebut batasnya | ✅ | `plan_seat_limit_reached` memuat angkanya; `TestUnit_Create_SeatFull_*`, `TestUnit_Create_SeatUsed_SumsActiveAndPending` |
| 4 | Paket tak dikenal → paling ketat, bukan tanpa batas | ✅ | `TestLimitsFor_UnknownPlan_FallsBackToFreeNotZero`, `TestLimitsFor_NonActiveStatus_FallsBackToFree` |
| 5 | `GET /v1/me` membawa `usage` + `limits` yang sudah diselesaikan | ✅ | `auth/handler_http.go`; dibaca sungguhan oleh `leadQuotaFor()` di `plan_quota_test.go` |
| 6 | Layar Langganan: paket aktif, pemakaian, perbandingan; Enterprise ke percakapan | ✅ | #125; `subscription-screen.tsx`, `plan_catalog_test.go`. Verifikasi browser → pemilik produk (§9.11 langkah 6) |
| 7 | Ada jalur yang mengubah `plan_code`, tercatat `audit_log`, **tidak** bisa dipakai pelanggan menaikkan paketnya sendiri | ✅ | #124; `subscription_admin_test.go` (10 test). Test checkout mati total di produksi lewat penjaga boot |
| 8 | Kuota terlampaui **tidak pernah** memperlihatkan keadaan paket ke pengunjung situs pelanggan | ✅ | **Diperkuat di #126**: `assertNoBillingLeak` memeriksa body respons form publik terhadap 11 kata (plan/paket/kuota/limit/tagihan/…) — sebelumnya hanya status `201` yang dicek sementara komentarnya mengklaim lebih |
| 9 | Angka hidup **hanya** di satu peta Go + satu dokumen | ✅ | `grep` dijalankan #126 (bukan diasumsikan): nihil di `internal/*` selain `plan.go`, nihil di `migrations/`, nihil di TypeScript produksi. Fixture test dashboard yang kebetulan memakai 100/2 **diganti** jadi 7/3 supaya tidak terbaca sebagai salinan kedua |
| 10 | `go test -race ./...` dan `npm run typecheck && lint && test && build` bersih | ✅ | Dijalankan ulang di #126: 39 paket Go hijau, `golangci-lint` 0 issues, 162 test dashboard, build sukses |

\* AC #2 ✅ **secara mekanisme** — jalur `curl` literal terhadap `docker compose` yang menyala adalah
langkah pemilik produk, prosedurnya tertulis lengkap. Preseden yang sama dipakai Phase 7 (#104) dan
Phase 8 (#115); tidak diklaim selesai sebagai pengamatan agent.

### Temuan di luar cakupan — `freeze.md` 8.4 tertinggal tiga migration

Memeriksa poin terbuka `docs/issues/123` (konsistensi `td.md` §1 dengan `freeze.md` 8.4) menemukan hal
yang lebih besar: tabel *Migration setelahnya* di `freeze.md` berhenti di **`0007_forms` (Phase 6)**.
`0008_webhooks`, `0009_webhook_secret_encrypted`, dan `0010_notification_plan_quota` tidak tercatat di
sana.

**Dilaporkan, tidak diperbaiki sepihak.** `freeze.md` adalah dokumen beku — menambal tabelnya diam-diam
di dalam PR penutup phase adalah persis kebiasaan yang CLAUDE.md cegah (*"Freeze tidak diubah tanpa
catatan"*). Keputusannya milik pemilik produk: tabel itu daftar hidup (→ PR kecil tersendiri) atau
rekaman rencana Phase 0–6 yang memang berhenti di sana (→ satu kalimat yang menyatakannya). Tercatat
di `docs/issues/123` dengan pemicunya.

### Poin terbuka `docs/issues/*` — seluruhnya ditinjau

| Berkas | Ditutup | Tetap terbuka (dengan pemicu) |
|---|---|---|
| `122` | penjaga provisional (boot, bukan test merah) · `planChannels` sama di tiga paket · aritmetika seat terduplikasi | `COUNT` tanpa index · tiga `COUNT` per `/v1/me` |
| `123` | `td.md` §1 dikoreksi | notifikasi hanya `public_form` · ambang organization-wide · kuota terlampaui di bawah konkurensi · **`freeze.md` 8.4 (baru)** |
| `124` | `subscription.read` (dibuat di #125) | reaktivasi membership · audit non-atomik · test checkout terkunci ke Pro |
| `125` | #125 menyentuh Go (didokumentasikan) · label harga Pro | **kontak Enterprise** · `useSessionRefresh` satu pemanggil |

Satu-satunya baris tabel *Angka provisional* yang **masih kosong** setelah phase ini: kontak Enterprise
(WhatsApp/email). Kartunya sengaja tanpa tombol sampai itu ada.

### Kewajiban yang diteruskan (TD §16) — tercatat, dengan pemicu masing-masing

| Kewajiban | Pemicu |
|---|---|
| Payment service tersambung | Webhook pembayaran memanggil jalur perubahan paket yang **sudah ada** (§8), bukan alur baru. `external_reference` mendapat pembacanya. **Tombol test checkout dihapus bersamaan** — bukan dibiarkan hidup di samping checkout sungguhan |
| Tinjau ulang angka | **Setelah 3–5 pelanggan _berbayar_ pertama** (ADR-014 ketentuan 2) — keempatnya bersama: kuota Free, kuota Pro, harga Pro, batas seat. Hidup di `STATUS.md` *Keputusan Belum Diambil* |
| Kuota dimensi baru (penyimpanan, webhook terkirim) | Ada bukti dimensi itu jadi biaya nyata → `Limits` bertambah field, peta bertambah kolom, test tabel otomatis mencakupnya |
| Index untuk `COUNT` kuota | Satu organization mendekati puluhan ribu lead/bulan → satu index non-parsial `(organization_id, created_at)`, angka `EXPLAIN` #122 sebagai titik banding |
| Kontak Enterprise | Pemilik produk memberi nomor/alamat → satu baris di `planDisplay`, tautan dirender |

Tanpa kode produksi baru selain angka final + dua test penjaga + satu asersi yang diperkuat (AC #8).
`go test -race ./...` bersih (39 paket), `golangci-lint run` 0 issues,
`npm run typecheck && lint && test && build` bersih (162 test).

---

## Follow-up pasca-#126 — kontak Enterprise & `freeze.md` 8.4

Dua poin terbuka yang tersisa saat phase ditutup, dijawab pemilik produk 5 September 2026 dan
dikerjakan sebagai satu PR kecil (bukan phase baru).

### Kontak Enterprise — env, bukan literal di `planDisplay`

Jawabannya "WhatsApp", tapi **bentuk penyimpanannya berbeda dari yang `docs/issues/125` perkirakan**
("satu baris di `planDisplay`"). Dua alasan yang keduanya baru terlihat setelah nomornya diberikan:

1. **Nomor yang dipakai hari ini nomor pribadi, dan pemiliknya sendiri menyatakan akan menggantinya
   dengan nomor Jualin.** Sebuah literal di kode akan tetap hidup di git history lama setelah
   penggantinya ada — dan repository ini private, tapi "private" bukan "terhapus".
2. **Bentuk URL penuh, bukan nomor telepon.** Kalau kelak pindah ke email, itu jadi perubahan env,
   bukan perubahan kode — CRM tidak perlu tahu mediumnya WhatsApp atau bukan; ia merender tautan.

`ENTERPRISE_CONTACT_URL` divalidasi saat boot dan **hanya menerima `https://` atau `mailto:`**. Itu
bukan kerapian melainkan batas keamanan: nilainya dirender langsung ke dalam `href` di layar
Langganan, dan `javascript:` di sana berarti eksekusi di sesi penonton. Ditolak saat boot lebih baik
daripada berharap dashboard meng-escape-nya dengan benar selamanya.

**Kosong tetap sah**, dan artinya kartu Enterprise dirender **tanpa tautan** — bukan tautan mati. Di
sisi kabel, `contact_url` **dihilangkan** dari JSON (bukan dikirim string kosong) supaya pengecekan
di klien adalah "ada tautan atau tidak", bukan "string ini kosong atau tidak" — kondisi yang cepat
atau lambat merender `href=""`.

Konfigurasinya duduk di **`subscription.HandlerConfig`**, bukan di `planDisplay`, dengan alasan yang
sama `auth.MeConfig` ada: usecase tidak boleh belajar membaca config (ADR-011). Ada alasan kedua yang
khusus di sini — katalog di `plan.go` adalah fungsi murni dari kebijakan produk, identik di setiap
deployment; sebuah URL kontak tidak: ia berbeda per deployment dan berubah tanpa paketnya berubah.

### `freeze.md` 8.4 — dinyatakan sebagai rekaman rencana, bukan dijadikan daftar hidup

Dari dua jalan keluar yang ditawarkan, pemilik produk memilih **menambahkan satu kalimat** yang
menyatakan tabel itu berhenti di Phase 6 dan bahwa daftar migration yang sesungguhnya adalah isi
`crm_be/migrations/`.

Pilihan ini lebih baik daripada "perbarui isinya" justru karena **selisih yang sama sudah terjadi tiga
kali berturut-turut** (`0008`, `0009`, `0010`). Dokumen yang wajib diperbarui setiap phase adalah
dokumen yang akan menyimpang lagi; yang dilakukan sekarang menghapus kewajiban itu sekaligus, dan
menyisakan **satu** sumber kebenaran untuk daftar migration.

Ini juga satu-satunya sentuhan ke `freeze.md` sepanjang Phase 8.5 — dilakukan hanya setelah
ditanyakan dan disetujui, bukan ditambal saat ditemukan.

### Sekalian: dua env dari #124 yang belum pernah masuk `.env.example`

`SUBSCRIPTION_ADMIN_TOKEN` dan `SUBSCRIPTION_TEST_CHECKOUT` ada di `config.go` sejak #124 tapi tidak
pernah didokumentasikan di `.env.example` — celah kecil yang tertangkap saat menambahkan env ketiga di
berkas yang sama. Ketiganya sekarang tercatat beserta arti "kosong berarti route tidak didaftarkan".

`go test -race ./...` bersih, `golangci-lint run` 0 issues,
`npm run typecheck && lint && test && build` bersih (162 test).
