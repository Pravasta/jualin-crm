# Phase 1 — Auth & Organization · Notes

> Realitas implementasi. Satu bagian per issue.

---

## #8 — Schema 0002, tenant context, pola repository, test katalog

### Menyimpang dari TD / freeze

| Yang berbeda | Alasan |
|---|---|
| `UNIQUE (id, organization_id)` ditambahkan ke `invitations`, `refresh_tokens`, `audit_logs` | **Freeze bagian 8.3 sendiri tidak menuliskannya secara eksplisit** untuk ketiga tabel ini — hanya `memberships` dan `subscriptions` yang punya baris `CONSTRAINT uq_..._id_org UNIQUE (id, organization_id)` tertulis di spesifikasi. Tapi Aturan #2 ("setiap tabel tenant-scoped punya `UNIQUE (id, organization_id)`") bersifat blanket, tanpa pengecualian. Ditambahkan untuk konsistensi dan langsung diuji lewat test katalog — bukan menyimpang dari freeze, melainkan mengisi kekosongan yang freeze sendiri tinggalkan. |
| `internal/membership` dan `internal/user` sebagai paket top-level, bukan `internal/organization/membership_*` | Rencana awal saya sebut `internal/organization/membership_repository.go`. Skill (`jualin-backend`) mencontohkan domain sejajar: "auth, organization, membership, lead" — bukan bersarang. Disesuaikan sebelum menulis kode, tidak ada refactor. |
| `Repository` (bukan `MembershipRepository`) sebagai nama tipe | Nama tipe di dalam paket `membership` tidak perlu mengulang nama paketnya — pemanggil menulis `membership.Repository`, bukan `membership.MembershipRepository`. Konvensi Go standar, tidak disebutkan eksplisit di skill tapi konsisten dengan idiom yang dipakai `httpx`, `logger`, dll. |
| `db.Querier` — interface baru di paket `db` | **Tidak ada di TD.** Repository perlu bekerja baik lewat `*pgxpool.Pool` (baca biasa) maupun `pgx.Tx` (di dalam `db.InTx`, dibutuhkan issue #9 untuk registrasi atomik). Tanpa ini, setiap repository harus punya dua constructor atau dua signature method. `Querier` adalah interface minimal yang dipenuhi keduanya — pola umum Go untuk kasus ini. |
| `TestMigrationRoundTrip` (Phase 0) diubah jadi dua test bertarget versi eksplisit | **Perbaikan wajib, bukan pilihan.** Test lama di issue #3 memanggil `goose.Down` lalu mengecek `set_updated_at` hilang — itu benar selama hanya ada satu migration. Begitu `0002` ada, `goose.Down` hanya membatalkan migration **terakhir** (0002), bukan 0001, sehingga assertion lama menjadi salah (fungsi dari 0001 tidak akan hilang, padahal test mengharapkannya hilang). Diganti `goose.UpTo`/`DownTo` bertarget versi eksplisit — `TestMigrationRoundTrip_0001Baseline` dan `TestMigrationRoundTrip_0002Identity`, masing-masing menguji migration-nya sendiri tanpa bergantung pada "yang mana yang paling akhir". Pola ini tidak perlu diubah lagi saat `0003` dst. ditambahkan. |

### Keputusan implementasi

- **`hasCompositeUnique` (test katalog) mencocokkan berdasarkan himpunan kolom, bukan nama constraint** — memeriksa apakah ada UNIQUE constraint mana pun yang kolomnya persis `{id, organization_id}` (diurutkan), bukan mencari nama constraint tertentu. Ini membuat test tidak rapuh terhadap penamaan constraint yang berbeda-beda di setiap tabel (`uq_memberships_id_org`, `uq_subscriptions_id_org`, dst.).
- **`FindActiveByUserID` di `membership.Repository` sengaja tidak menerima parameter organisasi** — satu-satunya method di paket ini yang menjelajah lintas organization. Didokumentasikan eksplisit di komentar method: ini bukan kelalaian, ini query yang justru dibutuhkan alur login (resolusi "organisasi mana saja yang saya punya membership aktif") sesuai ADR-007.
- **Test repository ditambahkan di luar rencana awal** — rencana hanya menyebut 5 test schema-level (round-trip, composite FK, partial unique, multi-membership, katalog). Ditambahkan `internal/membership/repository_test.go` dan `internal/user/repository_test.go` karena pola repository *itu sendiri* — bukan hanya constraint database di baliknya — adalah hal yang paling penting dibuktikan benar di issue ini. `TestRepository_FindByID_CrossTenant_ReturnsNotFound` adalah pembuktian di level Go bahwa `httpx.ErrNotFound` benar-benar dikembalikan (bukan 403) untuk membership tenant lain — melengkapi test SQL mentah yang membuktikan constraint database-nya.

### Verifikasi

```
go test -race -count=1 ./...    → semua paket PASS, dijalankan 2× berturut-turut
golangci-lint run                → 0 issues
gofmt -l .                       → bersih
go list -deps ./cmd/api | grep docker → kosong (binary produksi tetap bersih)
```

Lima kelas test freeze bagian 8.5, semuanya lolos:

| # | Test | Lokasi |
|---|---|---|
| 1 | Round-trip per migration | `TestMigrationRoundTrip_0001Baseline`, `TestMigrationRoundTrip_0002Identity` |
| 2 | Composite FK menolak referensi lintas tenant | `TestCompositeFK_RejectsCrossTenantMembershipReference` |
| 3 | Partial unique menolak duplikat aktif, mengizinkan setelah soft-delete | `TestMembershipPartialUnique_RejectsDuplicateActive_AllowsAfterSoftDelete` |
| 4 | Multi-membership diizinkan (penjaga ADR-007) | `TestMultiMembership_AllowedAcrossOrganizations` |
| 5 | Katalog — `organization_id` + `UNIQUE(id, organization_id)` | `TestCatalog_TenantScopedTablesHaveOrganizationID`, `TestCatalog_TenantScopedTablesHaveCompositeUniqueConstraint` |

**Kriteria "harness terbukti bisa gagal" (freeze) diverifikasi secara adversarial.** `subscriptions.organization_id` sengaja dihapus `NOT NULL`-nya di `migrations/0002_identity.sql`, dijalankan `go test -run TestCatalog_TenantScopedTablesHaveOrganizationID`:

```
catalog_test.go:51: table "subscriptions"'s organization_id must be NOT NULL, got nullable=YES
--- FAIL: TestCatalog_TenantScopedTablesHaveOrganizationID/subscriptions
```

Migration dikembalikan (diverifikasi `diff` identik dengan cadangan), test kembali hijau. Harness terbukti bukan sekadar hijau karena tidak menguji apapun.

### Utang teknis

- **Harness isolasi tenant lapis 4 (generik atas daftar route) belum dibangun** — sesuai rencana, itu memang cakupan issue #11, karena belum ada endpoint untuk ditembak. Yang dibangun di sini hanya lapis 1 (pola repository), 2 (composite FK), dan sebagian test katalog dari lapis "struktural".
- **Test yang memutar versi migration (`UpTo`/`DownTo`)** mengasumsikan seluruh test dalam paket `internal/shared/db` berjalan sekuensial (tanpa `t.Parallel()`). Ini benar sekarang dan diverifikasi lewat urutan log run manual, tapi **jangan tambahkan `t.Parallel()` ke paket ini** tanpa mempertimbangkan ulang — test migration memutar schema container yang dipakai bersama.

### Catatan untuk session berikutnya

- **`db.Querier`** dipakai ulang oleh setiap repository baru mulai issue #9. Constructor selalu `New(q db.Querier) *Repository`, bukan `New(pool *pgxpool.Pool)`.
- **Organization belum punya model/repository sendiri.** Issue #8 hanya membuat tabelnya lewat migration; `internal/organization` (bila dibutuhkan sebagai paket) menyusul di issue #9 saat registrasi benar-benar membuat baris organization.
- **Pola "satu contoh nyata lalu disalin"**: `membership.Repository` adalah rujukan untuk setiap repository tenant-scoped berikutnya. `user.Repository` adalah rujukan untuk setiap repository global berikutnya (belum ada yang lain sampai saat ini).

---

## #9 — Registrasi atomik, argon2id, dan verifikasi email

### Menyimpang dari TD

| Yang berbeda | Alasan |
|---|---|
| `httpx.DomainError` — mekanisme baru, tidak ada di TD | Domain error (`email_already_registered`, `invalid_token`) butuh status/code/message spesifik tanpa membuat `internal/shared/httpx` mengimpor paket domain (itu akan membalik arah dependensi: shared → domain). `DomainError{Status, Code, Message}` dikenali `MapError` lewat `errors.As`, generik untuk domain manapun. Dipakai lagi tanpa perubahan di #10 dan #11. |
| `internal/organization`, `internal/subscription`, `internal/auditlog` dibuat sebagai paket baru | Tidak eksplisit di rencana per-file, tapi sudah tersirat dari cakupan ("organization + membership owner + subscription free", "audit log"). Mengikuti pola yang sama seperti `membership`/`user` di #8: `organization.Repository` juga tanpa `tenant.Context` (ia tenant root — alasan berbeda dari `user`, tapi pengecualian sejenis, didokumentasikan eksplisit di komentar). |
| `internal/shared/ratelimit` — paket baru, tidak ada di TD sebagai nama paket | TD menyebut kebutuhan "rate limit per IP/per email" tanpa menentukan bentuk implementasinya. Dipilih fixed-window in-memory sesuai freeze ("tanpa Redis di MVP"), di balik interface `Limiter` agar bisa diganti nanti. Dipakai ulang di #10 (login) dan #11 (invitation). |
| Rate limit dicek di **handler**, bukan sebagai middleware generik | Dimensi "per email" untuk `resend` butuh body ter-parse — middleware generik gin berjalan sebelum body dibaca, sehingga tidak punya akses ke email. Dicek eksplisit di dalam masing-masing handler setelah `ShouldBindJSON`. |
| Angka rate limit (5/jam register, 3/jam+10/jam resend) adalah **default sementara**, bukan keputusan final | Freeze & `STATUS.md` sendiri mencatat "strategi rate limit final" sebagai keputusan terbuka hingga Phase 4/traffic nyata. Nilai di sini cukup untuk membuktikan mekanismenya aktif (acceptance criteria), bukan hasil tuning. |
| **Test isolasi tenant untuk endpoint baru — tidak berlaku langsung di #9** | Definition of Done template menyebutnya secara generik, tapi ketiga endpoint di issue ini (`register`, `verify-email`, `verify-email/resend`) **tidak terautentikasi** — `register` justru **membuat** tenant, dan `verify-email`/`resend` beroperasi pada `users`/`email_verification_tokens` yang global. Tidak ada boundary lintas-tenant untuk diuji di sini karena belum ada sesi yang membawa `organization_id`. Pengujian isolasi tenant yang sesungguhnya dimulai di #10 (token terikat satu organization) dan #11 (harness generik lapis 4). Yang paling dekat dengan "isolasi" di #9: setiap registrasi menghasilkan `organization_id` baru yang independen — diverifikasi implisit lewat `TestRegister_CreatesFourRowsAtomically` dan `TestRegister_DuplicateEmail_LeavesNothingBehind`. |

### Keputusan implementasi

- **Deteksi email duplikat lewat *unique violation* Postgres, bukan `SELECT` lalu `INSERT`.** Menghindari race check-then-act di bawah registrasi konkuren untuk email yang sama. `user.Repository.Create` memeriksa `pgconn.PgError.Code == "23505"` **dan** `ConstraintName == "users_email_key"` secara spesifik — bukan asal kode 23505 apapun — supaya *unique violation* dari constraint lain (mis. token hash) tidak salah diterjemahkan jadi `email_already_registered`.
- **Seluruh insert registrasi (organization, user, membership, subscription, token verifikasi, audit log) berada dalam satu `db.InTx`.** Token verifikasi sengaja dibuat **di dalam** transaksi yang sama, bukan setelah commit — supaya tidak ada state "user ada tapi tidak punya token untuk verifikasi" bila proses crash di antara keduanya. Hanya pengiriman email yang berada di luar transaksi (Aturan #32).
- **`VerifyEmail` menyamakan token hilang/kedaluwarsa/sudah dipakai** — `verificationRepository.FindValidByHash` memakai satu query (`used_at IS NULL AND expires_at > now()`) yang mengembalikan `httpx.ErrNotFound` untuk ketiganya, diterjemahkan seragam jadi `invalid_token`. Klien tidak bisa membedakan ketiga kasus lewat response.
- **Audit log `email.verified` mengasumsikan tepat satu membership aktif** pada saat verifikasi (`FindActiveByUserID`, ambil elemen pertama). Ini benar sepanjang Phase 1 karena user baru selalu lahir dengan satu membership dari registrasi. Akan perlu ditinjau ulang begitu #11 membuat multi-membership umum terjadi **sebelum** verifikasi selesai (mis. diundang ke organization kedua sebelum memverifikasi email organisasi pertama) — dicatat sebagai kondisi tepi untuk #11, bukan blocker sekarang karena skenario itu butuh urutan kejadian yang tidak mungkin di alur produk saat ini (undangan mensyaratkan akun yang sudah ada).
- **`ResendVerification` tidak pernah mengembalikan error ke handler** — signature-nya `func(ctx, email)` tanpa `error`, karena endpoint selalu 202 apapun yang terjadi secara internal (email tidak ditemukan, sudah terverifikasi, gagal generate token). Kegagalan internal dicatat ke logger, tidak pernah bocor ke response.
- **`looksLikeEmail` sengaja validasi minimal** (ada `@` dengan sesuatu di kedua sisi) — bukan regex RFC 5322 penuh. Validasi format yang lebih ketat tidak menambah keamanan (keunikan sudah ditegakkan database) dan berisiko menolak alamat sah yang tidak lazim. Konsisten Aturan #27.

### Verifikasi

```
go test -race -count=1 ./...    → semua paket PASS, 2× berturut-turut
golangci-lint run                → 0 issues (5 errcheck + 1 staticcheck ditemukan & diperbaiki)
gofmt -l .                       → bersih
go list -deps ./cmd/api | grep docker → kosong (binary produksi tetap bersih meski testcontainers dipakai luas di test)
```

19 test baru di `internal/auth`, mencakup seluruh acceptance criteria issue:

| Acceptance criteria | Test |
|---|---|
| Registrasi sukses → 4 baris + audit + email | `TestRegister_CreatesFourRowsAtomically` |
| Kegagalan di tengah transaksi → tidak ada baris tersimpan | `TestRegister_DuplicateEmail_LeavesNothingBehind` |
| `LogMailer` gagal → registrasi tetap commit | `TestRegister_MailerFailure_StillCommits` |
| Email sudah terdaftar → 409 | `TestRegister_DuplicateEmail_LeavesNothingBehind`, `TestHandler_Register_DuplicateEmail_Returns409` |
| Resend email tak dikenal → 202, tidak ada email terkirim | `TestResendVerification_UnknownEmail_SendsNothing` |
| Token sekali pakai, dipakai dua kali → `invalid_token` | `TestVerifyEmail_TokenUsedTwice_SecondAttemptFails` |
| Token kedaluwarsa ditolak | `TestVerifyEmail_ExpiredToken_Rejected` |
| Rate limit terbukti aktif | `TestHandler_Register_RateLimited`, `TestHandler_ResendVerification_RateLimitedPerEmail` |

Ditambah di luar checklist eksplisit: `TestPassword_HashIsNeverPlaintext` (jaring pengaman terhadap regresi paling merusak yang mungkin terjadi di paket ini), dan test HTTP-level untuk register/verify-email/resend di `handler_test.go`.

### Utang teknis

- **Peta `map[string]*bucket` di `ratelimit.FixedWindow` tidak pernah dibersihkan** — tumbuh tanpa batas seiring key baru (IP/email) muncul. Untuk volume MVP tidak masalah; perlu eviction (TTL sederhana atau `sync.Map` + goroutine pembersih berkala) sebelum traffic produksi nyata. Belum jadi masalah karena provider email sungguhan (dan karenanya traffic pendaftaran nyata) masih menunggu domain.
- **Angka rate limit belum final** (lihat Menyimpang dari TD) — perlu direvisi berdasarkan data nyata di Phase 4 atau lebih awal bila terbukti terlalu ketat/longgar saat pengujian manual dengan pengguna sungguhan.
- **HIBP (cek password bocor) belum diimplementasikan** — dicatat di TD §3 sebagai kandidat, sengaja tidak dikerjakan di sini (butuh panggilan HTTP eksternal, di luar cakupan issue).

### Catatan untuk session berikutnya

- **`httpx.DomainError` adalah titik ekstensi utama mulai sekarang** — #10 dan #11 akan menambah lebih banyak kode error (`invalid_credentials`, `email_not_verified`, `organization_selection_required`, dst.) lewat pola yang sama, bukan menambah sentinel baru di `httpx`.
- **`internal/shared/password`, `token`, `mailer`, `ratelimit` adalah dependensi bersama** — #10 (reset password, login) dan #11 (invitation) memakainya langsung, tidak menulis ulang.
- ~~`auth.Service` akan tumbuh~~ — **superseded oleh #15.** `Service` menjadi `Usecase`, bergantung pada `Store`, bukan `*pgxpool.Pool`. #10 menambah `Login`/`Refresh`/`Logout`/`ForgotPassword`/`ResetPassword` ke `auth.Usecase` yang sama, dan ke `auth.Repos`/`port.go` bila butuh repository baru (mis. `RefreshTokenRepository`).
- **`internal/organization.Repository` belum punya method `Update`** (mis. ubah timezone) — belum dibutuhkan siapapun. Tambahkan hanya saat ada pemakai nyata (Aturan #27).

---

## #15 — Refactor: layering per-paket, interface di sisi consumer, Unit of Work

### Menyimpang dari rencana issue

| Yang berbeda | Alasan |
|---|---|
| `membership`, `user`, `organization`, `subscription`, `auditlog` **tidak** mendapat `port.go`/`usecase.go`/`handler_http.go` — hanya `entity.go` + `repository_postgres.go` | Kelima paket ini adalah "repository murni": tidak punya business logic atau endpoint HTTP miliknya sendiri, hanya dikonsumsi lewat interface yang dideklarasikan `auth/port.go`. Memaksakan 5 berkas penuh pada paket yang tidak punya usecase adalah folder kosong yang terlihat seperti kemajuan (Aturan #27/#28) — bertentangan dengan semangat refactor ini sendiri. Pola 5 berkas berlaku untuk paket yang **punya** usecase & handler sendiri (`auth`, dan nanti `lead`, dll di Phase 2+). |
| Perakitan `Repos`/`Store` (`authStore`) ditaruh di **`cmd/api/auth_store.go`**, bukan `internal/app` | ADR-011 hanya menetapkan "bukan di `shared/`, bukan di paket domain manapun" — lokasi persisnya diputuskan saat implementasi. Karena baru ada **satu** composition site (`cmd/api`; `cmd/migrate` tidak butuh ini), membuat `internal/app` sekarang adalah abstraksi untuk kebutuhan yang belum ada (Aturan #27). Diekstrak ke `internal/app` **saat** composition site kedua muncul (mis. `cmd/worker`), bukan sebelumnya. |
| Test integrasi lama (`usecase_test.go`, sebelumnya `service_test.go`) mendapat `testStore` **sendiri**, menduplikasi ~20 baris dari `cmd/api/auth_store.go` | `internal/auth`'s production code sengaja tidak boleh mengimpor `user`/`organization`/`membership`/`subscription` konkret (itulah inti ADR-011) — jadi test package `auth_test` juga tidak mengimpor `cmd/api` (package `main`, dan secara arsitektural janggal untuk diimpor test paket domain). Duplikasi kecil ini adalah harga yang sepadan untuk menjaga batas itu tetap bersih di kedua sisi. |

### Keputusan implementasi

- **Deteksi "lepasnya business logic dari Docker" memakai `DOCKER_HOST` yang diarahkan ke socket tidak ada**, bukan benar-benar mematikan Docker Desktop pengguna — lebih aman dan tetap reversibel, membuktikan hal yang sama. Percobaan pertama (`DOCKER_HOST` + tanpa `-count=1`) sempat memberi hasil `(cached)` yang menyesatkan — Go test cache tidak otomatis tervalidasi ulang oleh perubahan env var sembarangan. Diperbaiki dengan `-count=1` untuk memaksa run baru.
- **Percobaan "negative control"** (menjalankan test **integrasi** dengan `DOCKER_HOST` yang sama untuk membuktikan ia justru gagal) **tidak berhasil sebagai pembuktian negatif** — testcontainers tetap menemukan Docker asli lewat mekanisme context Docker Desktop, bukan murni `DOCKER_HOST`. Ini tidak melemahkan klaim utama: pembuktian utama datang dari **inspeksi kode** (`grep testcontainers` di berkas unit test → kosong; `go list -deps` tidak menyertakan `internal/shared/db/dbtest`) dan **kecepatan run** (<1 detik vs ~9 detik test integrasi), bukan dari eksperimen `DOCKER_HOST` semata. Dicatat di sini secara jujur agar tidak terbaca sebagai bukti yang lebih kuat dari yang sebenarnya.
- **`VerificationTokenRepository`** (email_verification_tokens) **tetap punya interface DAN implementasi di dalam `internal/auth`** — berbeda dari `user`/`organization`/`membership`/`subscription` yang implementasinya di paket masing-masing. Alasan: tabel ini tidak punya consumer di luar `auth`, jadi tidak ada alasan memisahkannya ke paket sendiri hanya demi konsistensi kosmetik. `NewVerificationRepository` diekspor supaya composition root (`cmd/api`) tetap bisa mengonstruksinya tanpa mengetahui detail internal `auth`.
- **Kesalahan kecil terulang, ditemukan sendiri saat menulis**: sempat menambahkan import `internal/membership` + placeholder `var _ = membership.Membership{}` yang tidak perlu di `usecase.go` — Go ternyata tidak butuh import eksplisit untuk tipe yang hanya mengalir lewat inferensi (nilai balik `r.Member.FindActiveByUserID` tidak pernah dieja ulang sebagai `membership.Membership` secara literal di `usecase.go`). Dihapus sebelum commit — pengingat untuk lebih hati-hati pada import yang "terasa perlu" tapi sebenarnya tidak.

### Verifikasi

```
go build ./...                              → bersih
go vet ./...                                → bersih
golangci-lint run                           → 0 issues
gofmt -l .                                  → bersih
go test -race -count=1 ./...                → semua PASS (33 test lama, asersi tidak diubah)

DOCKER_HOST=unix:///tmp/nonexistent-docker.sock \
  go test ./internal/auth/... -run TestUnit -v -count=1
                                             → 7/7 PASS, 0.63–0.86 detik

go list -deps ./internal/shared/db | grep -E "internal/(auth|user|organization|membership|subscription|auditlog)$"
                                             → kosong
go list -deps ./cmd/api | grep -iE "testcontainers|docker"
                                             → kosong (binary produksi tetap bersih)
grep "jackc/pgx" internal/auth/{usecase,entity,port,handler_http}.go
                                             → kosong (pgx hanya di repository_postgres.go)
```

Seluruh 8 acceptance criteria issue #15 terpenuhi.

### Utang teknis

- Tidak ada item baru. Refactor murni, tanpa perubahan perilaku.

### Catatan untuk session berikutnya

- **Pola 5 berkas (`entity.go`/`port.go`/`usecase.go`/`repository_postgres.go`/`handler_http.go`) hanya untuk paket yang punya usecase & endpoint sendiri.** Paket "repository murni" seperti `membership`/`user`/`organization`/`subscription`/`auditlog` cukup `entity.go` + `repository_postgres.go` — **jangan** tambahkan `port.go` kosong ke paket-paket ini hanya demi keseragaman kosmetik. Tambahkan `port.go`/`usecase.go` pada paket itu **hanya** saat paket itu sendiri mendapat business logic atau endpoint HTTP miliknya (mis. `membership` kemungkinan mendapatkannya di #11 saat "ubah role"/"nonaktifkan" jadi endpoint tersendiri).
- **Composition root ada di `cmd/api/auth_store.go`.** Domain baru (mis. `lead` di Phase 2) yang butuh Unit of Work sendiri akan mendapat `cmd/api/<domain>_store.go` yang sama polanya — bukan satu `Store` raksasa yang menaungi semua domain.
- **Test integrasi lintas paket (`internal/auth`'s `usecase_test.go`) menduplikasi wiring `Repos` yang juga ada di `cmd/api/auth_store.go`.** Ini disengaja (lihat Menyimpang dari rencana). Bila polanya terasa berulang di banyak paket, pertimbangkan test helper bersama — tapi jangan taruh di `internal/shared` (aturan yang sama berlaku untuk test helper: jangan sampai `shared` mengimpor domain).

---

## #10 — Login, refresh rotation, logout, reset password, CSRF, GET /v1/me

### Menyimpang dari rencana issue

| Yang berbeda | Alasan |
|---|---|
| `cmd/migrate` **berhenti memakai `internal/shared/config.Config`**, dapat `migrateConfig` lokal (hanya `DATABASE_URL`) | Ditemukan lewat smoke test manual `docker compose up`, bukan direncanakan di awal: begitu `JWT_SECRET` jadi `required` di `config.Config`, `make migrate-up`/`migrate-status` ikut gagal minta `JWT_SECRET` — padahal migrate tidak pernah menyentuhnya. Berbagi satu `Config` lintas binary dengan kebutuhan berbeda adalah bug yang menunggu terjadi lagi di field `required` berikutnya; diperbaiki sekarang, bukan dicatat sebagai utang. |
| `MembershipRepository` (di `auth/port.go`) dapat method `FindByID` baru | Diperlukan `Refresh` untuk memuat ulang `role`/`user_id` sebelum menerbitkan access token baru — `refresh_tokens` hanya menyimpan `membership_id`+`organization_id` (TD §1), bukan keduanya. Implementasi konkretnya (`membership.Repository.FindByID`) sudah ada sejak #8; hanya diekspos lewat interface yang baru dipakai sekarang. |
| Rate limit login pakai tipe baru `LoginLimiter` (backoff progresif), **bukan** `ratelimit.FixedWindow` | TD eksplisit minta "backoff progresif, tanpa lockout permanen" untuk login — beda dari limit flat register/resend/forgot. `ratelimit.Limiter`'s `Allow(key) bool` tidak bisa menyatakan "naik saat gagal, reset saat sukses". Satu tipe konkret baru, bukan interface baru — `ratelimit.Limiter` tidak disentuh. |

### Keputusan implementasi

- **`internal/shared/accesstoken` (JWT) dipisah dari `internal/shared/token` (opaque)** — keduanya secara struktural berbeda (JWT bertanda tangan vs random+hash) dan TD memperlakukannya sebagai dua konsep berbeda. Refresh token dan password reset token tetap memakai `token.Generate()`/`.Hash()` yang sudah ada — tidak ada paket baru untuk itu.
- **`httpx.VerifyCSRF` generik**, tidak tahu domain — hanya membandingkan cookie `csrf_token` vs header `X-CSRF-Token`. Yang memutuskan *kapan* ia dipanggil bersifat lokal: `auth.AuthMiddleware` untuk endpoint di belakangnya, handler `refresh`/`logout` langsung untuk keduanya (karena determinasi "apakah kredensial ini dari cookie" berbeda tempat: middleware tahu dari token akses, refresh/logout tahu dari mana refresh token dibaca).
- **`organization_selection_required`** ditulis langsung lewat `c.JSON`, bukan lewat `httpx.WriteError` — `OrganizationSelectionError` membawa field `organizations` yang tidak bisa direpresentasikan `httpx.DomainError`. Didokumentasikan di `api.md` sebagai satu-satunya pengecualian sadar terhadap bentuk envelope error.
- **Deteksi reuse refresh token** tidak dapat kode error baru — mengikuti diagram alur TD §4 apa adanya, baik "tidak ditemukan/kedaluwarsa" maupun "sudah dirotasi/dicabut" sama-sama jadi `401 invalid_credentials`, supaya penyerang yang memakai token curian tidak bisa membedakan dari respons mana yang terjadi.
- **`FindByHashForUpdate`/`RevokeAllByUserID`** (di `RefreshTokenRepository`) sengaja **tidak** menerima `tenant.Context` — ditulis eksplisit di `port.go` sebagai pengecualian TD §8: yang pertama karena organization baru diketahui dari hasil lookup itu sendiri (bukan diketahui sebelumnya), yang kedua karena reset password sengaja mencabut sesi lintas organization.
- **`docker-compose.yml` dan `.env.example`** dapat `JWT_SECRET` dev-only + var Phase 1 lain (`ACCESS_TOKEN_TTL`, dst.) — tanpa ini `docker compose up` dari clone baru langsung gagal boot (Aturan #36 bekerja sesuai desain, tapi harus tetap zero-setup untuk dev).

### Verifikasi

```
go build ./...                              → bersih
go vet ./...                                → bersih
golangci-lint run                           → 0 issues
gofmt -l .                                  → bersih
go test -race -count=1 ./... (2x)           → semua PASS

DOCKER_HOST=unix:///tmp/nonexistent-docker.sock \
  go test ./internal/auth/... -run TestUnit -v -count=1
                                             → 20/20 PASS, tanpa Docker

go list -deps ./cmd/api | grep -iE "docker"
                                             → kosong

Manual smoke test (docker compose up --build, migrasi dijalankan manual):
  register → verify-email → login client=dashboard
    → Set-Cookie: access_token (HttpOnly), refresh_token (HttpOnly), csrf_token (bukan HttpOnly)
    → body TIDAK memuat token
  login client=mobile
    → TIDAK ada Set-Cookie sama sekali, token di body
  GET /v1/me dengan Authorization: Bearer → 200, profil benar
  POST /v1/auth/refresh dengan cookie TANPA X-CSRF-Token → 403 csrf_token_invalid
```

Seluruh acceptance criteria issue #10 (checklist & acceptance criteria di GitHub) terpenuhi, termasuk seluruh test keamanan TD §14 yang relevan untuk issue ini (reuse refresh → family dicabut; cookie tanpa CSRF → 403; bearer/body tanpa CSRF → berhasil; `COOKIE_SECURE=false`+`production` → boot gagal; reset password mencabut seluruh sesi).

### Utang teknis

- Tidak ada item baru dari issue ini sendiri. `docs/STATUS.md` bagian Utang Teknis tetap seperti sebelumnya (belum ada eviction `ratelimit.FixedWindow`, angka rate limit belum final).

### Catatan untuk session berikutnya

- **Jangan tambahkan field `required` baru ke `internal/shared/config.Config` tanpa memeriksa `cmd/migrate`.** Kejadian di atas (JWT_SECRET) kemungkinan besar akan terulang untuk field wajib berikutnya kalau tidak diperiksa — pertimbangkan pola `migrateConfig` yang sama untuk binary lain yang mungkin muncul (`cmd/worker`, dst.) bila mereka juga tidak butuh seluruh `Config`.
- **Issue #11 (RBAC, invitation, membership deactivation, harness isolasi tenant) adalah penutup Phase 1.** `authorization.md` dibuat di sana, bukan di sini — `authentication.md` (issue #10) sudah menyebutkan `AuthMiddleware` tapi belum ada RBAC di atasnya.
- **`AuthMiddleware` hanya melindungi `GET /v1/me` saat ini** — satu-satunya endpoint terautentikasi di Phase 1. Endpoint terautentikasi berikutnya (Phase 2 CRM Core) tinggal masuk grup `protected` yang sama di `RegisterRoutes`.
