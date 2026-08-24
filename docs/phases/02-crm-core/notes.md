# Phase 2 — CRM Core · Notes

One section per issue, appended as each is implemented.

---

## #19 — Schema 0003, repository lead, alokasi lead_number, optimistic locking

### Menyimpang dari rencana issue

| Yang berbeda | Alasan |
|---|---|
| `internal/lead` **tidak** mendapat `port.go` — hanya `entity.go` + `repository_postgres.go`, dengan `Repository` sebagai struct konkret (bukan interface) | Issue #19 checklist menyebut `port.go`, tapi itu tidak konsisten dengan konvensi yang sudah dua kali ditetapkan codebase ini (`user`, `organization`, dan `membership` sebelum #11): paket "repository murni" tanpa usecase mengekspor struct konkret; `port.go` (interface) baru muncul saat consumer sungguhan (`Usecase`) ada untuk mendeklarasikannya (ADR-011). #20 menambah `port.go` + `usecase.go` saat consumer itu ada, dan `Repository` berubah jadi interface — migrasi yang sama persis dengan yang `membership` alami di #11. |

### Keputusan implementasi

- **Klaim awal di komentar kode terbukti berlebihan, ditemukan sebelum commit.** Draf pertama `Create`'s doc comment menyatakan "callers MUST wrap Create in db.InTx, dibuktikan di repository_concurrency_test.go". Sebelum menulis ini di notes.md, saya uji klaim itu secara langsung: memodifikasi `createConcurrently` sementara untuk memanggil `Create` langsung ke pool (tanpa `db.InTx` sama sekali) dan menjalankan `TestConcurrency_LeadNumber_NoGapsNoDuplicates` lima kali dengan `-race`. **Tetap hijau seluruhnya.** Alasannya: `UPDATE … RETURNING` tunggal sudah atomik dan mengambil row lock terlepas dari apakah ia dibungkus transaksi eksplisit — yang dibuktikan test konkurensi hanyalah alokasi nomor tidak pernah tabrakan, bukan kebutuhan `db.InTx`.

  Yang **sungguh** bergantung pada `db.InTx` adalah skenario berbeda: `INSERT INTO leads` gagal **setelah** `next_lead_number` berhasil dialokasikan (mis. pelanggaran FK). Tanpa transaksi, alokasi itu sudah ter-commit sendiri — nomor itu hilang permanen, sebuah lubang. Di dalam `db.InTx`, kegagalan `INSERT` me-rollback **seluruh** percobaan termasuk alokasinya, sehingga percobaan berikutnya mendapat nomor yang sama, bukan nomor berikutnya.

  Ditulis test baru yang secara langsung membuktikan skenario itu — `TestCreate_FailedInsertInsideInTx_DoesNotBurnLeadNumber` (`repository_test.go`): buat lead pertama (nomor 1), coba `Create` kedua yang **sengaja** melanggar FK `assigned_to_membership_id` di dalam `db.InTx`, pastikan gagal, lalu `Create` ketiga (di luar transaksi manapun, seperti biasa) harus mendapat nomor **2**, bukan 3. Ini benar-benar membuktikan kebutuhan transaksi; tiga test konkurensi tidak.

  Komentar kode (`Create`'s doc comment dan `repository_concurrency_test.go`'s doc comment) ditulis ulang supaya tidak mengklaim lebih dari yang benar-benar dibuktikan test yang menyertainya — kesalahan kecil, ditangkap sebelum commit, tapi dicatat di sini karena persis jenis overclaim yang mudah lolos review kalau tidak diuji langsung.

- **`Update`'s field nullable (`Email`, `Phone`, `Company`, `Notes`) tidak bisa di-set ke `NULL` lewat `UpdateInput`** — `nil` berarti "jangan sentuh", direpresentasikan lewat `COALESCE($n, kolom)`. Tidak ada cara membedakan "jangan sentuh" dari "kosongkan" dengan `*string` tunggal. Disengaja untuk cakupan repository-only issue ini; `#20` menambah mekanisme tri-state (mis. `**string` atau flag terpisah) bila usecase-nya benar-benar butuh mengosongkan field, bukan ditebak sekarang.

- **`ck_leads_lost_requires_reason` dan `uq_customers_org_lead` ditegakkan database**, persis seperti TD — bukan hanya usecase (yang belum ada di issue ini). Terverifikasi lewat schema migration, bukan test Go (belum ada usecase untuk memicunya lewat kode aplikasi).

### Verifikasi

```
go build ./...                              → bersih
go vet ./...                                → bersih
golangci-lint run                           → 0 issues
gofmt -l .                                  → bersih
go test -race -count=1 ./... (2x)           → semua PASS

go test ./internal/lead/... -race -count=1  → 10/10 PASS
  TestConcurrency_LeadNumber_NoGapsNoDuplicates          — 20 goroutine, {1..20} tepat
  TestConcurrency_LeadNumber_IndependentPerOrganization  — dua org paralel, {1..10} masing-masing
  TestConcurrency_VersionConflict_ExactlyOneWins         — 20 update bersamaan, tepat 1 sukses
  TestCreate_FailedInsertInsideInTx_DoesNotBurnLeadNumber — lihat di atas

goose up/down round-trip (0003)             → bersih, tabel 0002 & fungsi 0001 tetap ada setelah rollback 0003
TestCatalog_TenantScopedTables*             → leads/customers/activities/tasks otomatis lolos TANPA perubahan test (dari #8)

go list -deps ./cmd/api | grep docker       → kosong
```

Seluruh acceptance criteria issue #19 terpenuhi.

### Utang teknis

- Tidak ada item baru dari issue ini sendiri.

### Catatan untuk session berikutnya

- **`internal/lead` mengikuti pola `entity.go`+`repository_postgres.go` murni.** #20 menambah `port.go` (interface `Repository` + `Store`/`Repos`) dan `usecase.go` — pada titik itu, struct `Repository` di `repository_postgres.go` berganti nama jadi `postgresRepository` (unexported), persis migrasi `membership` di #11. Jangan kaget saat rename itu terjadi — sudah direncanakan, bukan refactor darurat.
- **`Update` di repository ini hanya field umum** (`name`, `email`, `phone`, `company`, `notes`). Transisi status (`PATCH /status`) dan assignment (`PATCH /assignment`) di TD adalah endpoint terpisah dengan aturan sendiri — `#20` kemungkinan menambah method `Repository` baru untuk itu (`UpdateStatus`, atau usecase memanggil `Update` plus logic tambahan), bukan memperluas `UpdateInput` yang ada.
- **Uji klaim atomisitas secara langsung sebelum menuliskannya di komentar kode**, bukan berasumsi dari desain. Pengalaman di atas (klaim `db.InTx` yang ternyata tidak dibuktikan test konkurensi) adalah contoh konkret kenapa ini penting — desain yang *terlihat* benar bisa salah soal *test mana* yang sebenarnya membuktikannya.

---

## #20 — Lead CRUD, transisi status, filter, pagination, idempotency, E.164

### Menyimpang dari rencana issue

| Yang berbeda | Alasan |
|---|---|
| Keluar dari status `lost` mengizinkan pindah ke **status jalur utama manapun**, bukan spesifik "satu langkah kembali ke status sebelum kalah" seperti bunyi literal TD §5 | `leads` hanya menyimpan status **saat ini** — tidak ada memori status sebelumnya. Riwayat itu baru tersedia lewat `activities` (#21). Didokumentasikan sebagai pendekatan sementara di `usecase.go`'s `validateStatusTransition`; tidak ada acceptance criteria issue #20 yang menguji kasus spesifik ini. Akan disempurnakan begitu `activities` memberi riwayat yang bisa dipakai, bukan ditebak dari nol. |

### Keputusan implementasi

- **Migrasi `internal/lead` dari repository murni ke domain penuh** persis seperti direncanakan di notes.md `## #19`: `Repository` (struct konkret) → interface, implementasi berganti nama `postgresRepository`, `port.go`+`usecase.go` baru. Tidak ada kejutan — sudah dicatat sebelum dikerjakan.
- **Bug nyata ditemukan lewat test yang salah pakai, bukan lewat desain**: saat menulis `TestHandler_Get_Employee_OtherPersonsLead_Returns404`, lead di-assign ke `assigned_to_membership_id` acak yang **tidak** merujuk baris `memberships` sungguhan. `INSERT` melanggar FK composite `fk_leads_assignee` — dan pelanggaran itu **tidak tertangani**, jatuh ke `fmt.Errorf` generik yang dipetakan `internal_error` (500) oleh `httpx.MapError`. Test berikutnya yang memakai id kosong hasil parse gagal itu justru gagal dengan cara membingungkan (301 redirect ke `/v1/leads`, gara-gara path menjadi `/v1/leads/` kosong). Diperbaiki dua arah: (1) test itu sendiri diperbaiki untuk menyeed membership sungguhan, (2) **gap produksi yang terungkap ditutup**: `Create` sekarang mendeteksi pelanggaran FK 23503 pada `fk_leads_assignee` dan mengembalikan `ErrAssigneeNotFound`, dipetakan usecase menjadi `400 validation_failed` yang jelas. Test regresi `TestHandler_Create_InvalidAssignee_Returns400NotInternalError` memastikan ini tidak kembali menjadi 500 diam-diam.
- **Deteksi idempotency key duplikat lewat unique-violation database** (`uq_leads_org_idempotency`, kode `23505`), persis pola `user.ErrEmailTaken` — bukan `SELECT`-lalu-`INSERT`, yang justru race persis di bawah kondisi retry bersamaan yang menjadi alasan idempotency key ada. Dibuktikan dengan test HTTP 10 request `POST` bersamaan memakai key yang sama: tepat 1 `201`, sisanya `200`, seluruhnya merujuk lead yang sama, dan hanya 1 baris di database.
- **`FindAllByOrg` membangun `WHERE` secara manual** (bukan query-builder library — "sqlc/pgx bukan ORM") — daftar kondisi + slice argumen dirakit sekali, dipakai bersama oleh query `count(*)` dan query berpaginasi supaya keduanya tidak pernah berbeda filter.
- **Visibilitas employee di `FindAllByOrg` diterapkan tanpa syarat** — filter `assigned_to` dari query param employee **diabaikan**, diganti paksa dengan `assigned_to_membership_id = t.MembershipID`. Employee yang mencoba memfilter ke lead orang lain mendapat daftar kosong, bukan error — aman by construction, sama seperti `FindByID`/`Update` sejak #19.

### Verifikasi

```
go build ./...                              → bersih
go vet ./...                                → bersih
golangci-lint run                           → 0 issues (setelah perbaikan G101 + 2x S1016)
gofmt -l .                                  → bersih
go test -race -count=1 ./... (2x)           → semua PASS

go test ./internal/lead/... -race -count=1  → 34/34 PASS
go test ./internal/shared/phone/... -count=1 → semua PASS

DOCKER_HOST=unix:///tmp/nonexistent-docker.sock \
  go test ./internal/... -run 'TestUnit|TestRequire|TestToE164' -count=1
                                             → PASS tanpa Docker (lead, authz, phone, + semua paket lama)

go list -deps ./cmd/api | grep docker       → kosong
```

**Test konkurensi idempotency (wajib di Definition of Done)**: 10 request `POST /v1/leads` bersamaan dengan `Idempotency-Key` sama → tepat 1× `201`, 9× `200`, seluruhnya mengembalikan `id` yang sama, dan `SELECT count(*)` di database mengonfirmasi tepat 1 baris.

**Smoke test manual** (`docker compose up --build`): register+verify+login owner → `POST /v1/leads` dengan `Idempotency-Key` → `201`, `lead_number=1`, `phone_e164` ternormalisasi `+6281234567890`, email lower-case → replay key yang sama dengan body berbeda → `200`, lead **sama persis**, nama tidak berubah → `PATCH /status` `new→won` → `422` → `PATCH /status` `new→contacted` → `200` → `PATCH` field umum dengan `version` basi → `409` dengan `error.current` memuat status terkini (`contacted`, `version=2`) → `GET /v1/leads?status=contacted` → daftar & `meta.total` benar.

Seluruh acceptance criteria issue #20 terpenuhi.

### Utang teknis

- Tidak ada item baru dari issue ini sendiri. `docs/STATUS.md` bagian Utang Teknis tetap seperti sebelumnya.

### Catatan untuk session berikutnya

- **`internal/lead/port.go`'s `Repos{Lead Repository}` punya satu field secara sengaja.** #21 menambah field `Activity` saat menulis `activities` perlu atomik dengan perubahan lead (`status_changed`, `lead_assigned`, dst) — pada titik itu, `UpdateStatus`/assignment di usecase perlu dibungkus `store.InTx` juga (saat ini hanya `Create` yang InTx, karena `Create` sendiri butuh atomisitas internal, bukan karena ada tabel lain yang ditulis).
- **Pola deteksi pelanggaran constraint database (unique ATAU foreign key) sebagai sinyal, bukan `SELECT` dulu**, sekarang punya dua contoh di paket ini (`ErrIdempotencyKeyExists`, `ErrAssigneeNotFound`) selain `user.ErrEmailTaken` di Phase 1. Pola yang sama berlaku untuk constraint baru yang muncul di #21/#22/#23 (mis. `uq_customers_org_lead` saat konversi).
- **Manual smoke test menemukan bug produksi untuk ketiga kalinya berturut-turut** (setelah #11 dan sesi ini sendiri, lewat jalur test bukan `docker compose` kali ini) — pola yang konsisten cukup kuat untuk terus dipertahankan di issue berikutnya, bukan dianggap kebetulan.

---

## #21 — Activity append-only + auto-log, dan Task

### Keputusan implementasi

- **`ActivityRecorder` dideklarasikan konsumen (`lead`, `task`), bukan diimpor dari `internal/activity`** — persis pola `auth.RefreshTokenRevoker` di #11. Kedua interface lokal identik bentuknya (`Record(ctx, t, leadID, activityType, actorMembershipID, metadata)`), sengaja diduplikasi tiga baris, bukan ditarik ke paket bersama untuk dua titik pakai. `internal/activity` mengekspor `Recorder` + `NewRecorder(q)` sebagai jembatan yang dipasang di composition root (`activity.NewRecorder(q)` dipakai baik oleh `lead.Repos.Activity` maupun `task.Repos.Activity`).
- **Visibilitas lead untuk `activity`/`task` tidak butuh interface lintas paket** — cukup `WHERE EXISTS (SELECT 1 FROM leads ...)` langsung di query masing-masing paket, karena `activity`/`task`/`lead` berbagi satu database Postgres yang sama. Menghindari interface `LeadVisibility` yang tidak diminta issue manapun.
- **`internal/task`'s visibilitas employee lewat kepemilikan LEAD, bukan `tasks.assigned_to_membership_id`** — TD §9 eksplisit: "task pada lead miliknya". Task boleh di-assign ke kolega di lead yang Anda miliki, dan Anda tetap harus melihatnya; task yang di-assign ke Anda di lead orang lain tetap harus **tidak** terlihat. Dibuktikan lewat `TestRepository_Employee_VisibilityIsThroughLeadAssignment` yang sengaja menguji ketiga kombinasi (pemilik lead, assignee task yang bukan pemilik lead, orang lain sama sekali).
- **`task.FindAllByLead` memeriksa visibilitas lead secara eksplisit** (404 bila tidak terlihat), **berbeda** dari `task.FindAllByOrg` yang membiarkan scoping employee diam-diam mempersempit hasil ke kosong. Desain awal memakai `buildTaskWhere` yang sama untuk keduanya — salah, karena itu membuat `GET /v1/leads/{id}/tasks` pada lead orang lain mengembalikan `200` dengan daftar kosong, bukan `404` seperti bunyi acceptance criteria ("Employee membaca activity/task pada lead orang lain → 404"). Ketahuan lewat test integrasi sendiri sebelum commit, diperbaiki dengan helper `leadVisible` terpisah — pola yang sama `activity.FindAllByLead` sudah pakai.
- **`authz.ActionTaskRead` ditambah di luar tabel TD §9 yang literal** — TD hanya menyebut `task.create/update/complete` dan `task.delete`, tidak ada baris baca eksplisit. Menambah action baca sendiri (permission identik keempat role, sama seperti `activity.list` mendampingi `activity.create`) lebih jelas ketimbang memakai `ActionTaskCreate` sebagai gerbang baca — nama aksi yang salah mengaburkan niat saat dibaca ulang nanti.
- **Bug ditemukan lewat test unit (fake), bukan lewat desain**: `TestUnit_UpdateStatus_RecordsStatusChangedActivityWithFromTo` awalnya gagal — `metadata.from` terisi status **baru**, bukan status lama. Penyebabnya alias pointer di fake test double: `fakeLeadRepo.FindByID` dan `fakeLeadRepo.UpdateStatus` sama-sama mengembalikan pointer ke struct yang sama di map, sehingga membaca `current.Status` **setelah** `UpdateStatus` dipanggil sudah melihat nilai yang sudah berubah. Postgres asli tidak akan pernah berperilaku begini (`scanLead` selalu mengalokasikan struct baru), tapi `usecase.go` diperbaiki supaya tidak bergantung pada asumsi itu sama sekali — `fromStatus` ditangkap ke variabel lokal segera setelah `FindByID`, sebelum `UpdateStatus` dipanggil.
- **`lead.Usecase.UpdateStatus` sekarang berjalan di dalam `store.InTx`** — sebelumnya (#20) memanggil `store.Repos()` langsung. Perubahan ini wajib supaya `status_changed` atomik dengan perubahan status yang memicunya (TD §10), persis seperti dicatat sebagai "catatan untuk session berikutnya" di bagian `## #20` di atas.
- **jsonb pertama yang benar-benar ditulis di codebase ini** (`activities.metadata`) — `lead.RawPayload` sejak #19 tulus-tembus tapi tidak pernah benar-benar diisi pemanggil manapun. Klaim "pgx menerima `[]byte` hasil `json.Marshal` untuk kolom jsonb" diverifikasi langsung lewat `TestRepository_Create_MetadataRoundTrips` sebelum dipakai di tempat lain — insert `{"from":"new","to":"contacted"}`, baca lewat `RETURNING` **dan** lewat `SELECT` terpisah, keduanya di-unmarshal dan dicocokkan.

### Verifikasi

```
go build ./...                              → bersih
go vet ./...                                → bersih
golangci-lint run                           → 0 issues (setelah 1x perbaikan S1016)
gofmt -l .                                  → bersih
go test -race -count=1 ./... (2x)           → semua PASS

DOCKER_HOST=unix:///nonexistent/docker.sock \
  go test ./internal/... -run TestUnit -count=1
                                             → PASS tanpa Docker (activity, task, lead, + semua paket lama)

go list -deps ./cmd/api | grep docker       → kosong
```

**Test atomisitas (wajib di Definition of Done)**: dibuktikan dua lapis — `usecase_unit_test.go` (fake `ActivityRecorder` yang gagal, membuktikan kegagalan menular sebagai error dari seluruh operasi) **dan** `repository_atomicity_test.go` di `internal/lead` (transaksi Postgres sungguhan: `Activity.Record` sengaja gagal setelah `Lead.Create` sukses, baris `leads` dipastikan **tidak ada** setelahnya, dan `lead_number` yang teralokasi tidak "terbakar" — percobaan berikutnya tetap dapat nomor 1).

**Smoke test manual** (`docker compose up --build`): register+verify+login owner → `POST /v1/leads` → `201`, `GET /v1/leads/{id}/activities` → tepat satu `lead_created` dengan `actor_membership_id` = owner → `PATCH /status` `new→contacted` → `GET activities` → `status_changed` dengan `metadata={from:new,to:contacted}`, urutan terbaru dulu → `POST activities {"type":"status_changed"}` → `422 invalid_activity_type` → `POST activities {"type":"note_added"}` → `201` → `PATCH` ke `/v1/leads/{id}/activities/{id}` → `404` (route memang tidak terdaftar) → `POST /v1/leads/{id}/tasks` → `201` → `POST /v1/tasks/{id}/complete` → `200`, `status=done`, `completed_at`/`completed_by` terisi → `GET activities` menunjukkan `task_created` dan `task_completed` → `PATCH` task dengan `version` basi → `409` → `GET /v1/tasks` dan `GET /v1/leads/{id}/tasks` keduanya mengembalikan task yang sama → `DELETE /v1/tasks/{id}` → `204`.

Seluruh acceptance criteria issue #21 terpenuhi. Kasus 404 employee (activity & task) diverifikasi otomatis lewat test Postgres (`TestHandler_Get_Employee_OtherPersonsLead_Returns404`, `TestHandler_Employee_ReadTaskOnAnotherEmployeesLead_Returns404`), tidak diulang manual — sudah terbukti lewat jalur HTTP sungguhan di test integrasi.

### Utang teknis

- Tidak ada item baru dari issue ini sendiri.

### Catatan untuk session berikutnya

- **`internal/lead/port.go`'s `Repos{Lead, Activity}` sekarang dua field** — #22 kemungkinan menambah lagi saat assignment perlu menulis `notifications` sekaligus (`lead_assigned`/`lead_unassigned` + notification, satu transaksi, TD §11).
- **Pola visibilitas-eksplisit-lalu-404 vs filter-diam-diam** sekarang punya preseden jelas: gunakan yang pertama untuk "satu resource spesifik milik satu lead" (`FindByID`, `FindAllByLead`), gunakan yang kedua untuk "daftar lintas-lead yang boleh menyempit" (`FindAllByOrg`). Jangan pakai `buildXWhere` yang sama untuk keduanya tanpa mikir ulang — itu persis kesalahan yang tertangkap di atas.
- **`docs/architecture/authorization.md`'s matrix belum diperbarui** — tetap ditunda ke #23 seperti sudah dicatat di TD §9, sekarang dengan `activity.*`/`task.*` (termasuk `task.read` yang tidak ada di TD literal) sebagai baris tambahan yang perlu masuk saat itu.
- **`internal/task`'s `Complete` tidak menolak menyelesaikan task yang sudah `done`** — pada `version` yang benar, memanggil `complete` lagi hanya menimpa ulang `completed_at`/`completed_by` tanpa error. Tidak diuji acceptance criteria manapun; disebutkan di sini supaya bukan kejutan bila suatu saat dianggap perlu perilaku berbeda.

---

## #22 — Assignment, notification (`0004`), penutupan kewajiban penonaktifan membership

### Keputusan implementasi

- **`internal/notification` baru, paket domain penuh** — satu-satunya resource di codebase ini di mana **Owner/Admin pun tidak dapat akses lebih luas** dari role lain: setiap query di-scope tanpa syarat ke `recipient_membership_id = t.MembershipID`, tanpa cabang `isEmployee`. Tidak ada `authz.Require` di `notification.Usecase` sama sekali — tidak ada pertanyaan berbasis role untuk dijawab, sama seperti `GET /v1/me`.
- **`Repository` milik `notification.Usecase` sengaja tanpa `Create`** — tidak ada `POST /v1/notifications` di TD §8. Pembuatan notifikasi hanya lewat `Notifier` (bridge lintas paket), dipasang di `lead.Repos.Notification` lewat `notification.NewNotifier(q)` — pola yang sama `activity.Recorder` pakai, instansi ketiga dari pola `auth.RefreshTokenRevoker` di #11.
- **`internal/lead` mendapat `OpenLeadRepository` (bridge terpisah, BUKAN bagian dari `Repository`)** — `CountOpen`/`UnassignOpen`/`ReassignOpen`, dipakai eksklusif oleh `membership.Usecase.Deactivate` lewat interface lokal `membership` sendiri (`OpenLeadRepository`, bentuk identik, dideklarasikan independen). `internal/membership` tetap tidak pernah mengimpor `internal/lead` — persis seperti tidak pernah mengimpor `internal/auth` untuk `RefreshTokenRepository`.
- **Batu sandungan nyata: sentinel error lintas paket tidak bisa dikenali tanpa impor domain** — rencana awal `ReassignOpen` mengembalikan `lead.ErrAssigneeNotFound` saat `reassign_to` tidak valid (pelanggaran `fk_leads_assignee`), persis pola `Create`/`UpdateAssignment`. Tapi `membership.Usecase` **tidak bisa** melakukan `errors.Is` terhadap sentinel itu tanpa mengimpor `internal/lead` — melanggar ADR-011 tepat di titik yang seharusnya dicegah oleh interface bridge. Diperbaiki dengan mengembalikan `*httpx.ValidationError` **langsung dari `ReassignOpen`** (bukan sentinel domain `lead`) — `internal/shared/httpx` adalah satu-satunya paket netral yang **kedua sisi** sudah impor, dan `httpx.MapError` sudah tahu memetakannya ke `400` tanpa kode tambahan apa pun di `membership`. Pelajaran: method yang HANYA melayani sebagai bridge lintas paket (bukan dipakai usecase paket asalnya sendiri) adalah lapis yang tepat untuk menghasilkan sinyal yang sudah bisa dikenali pemanggil — beda dengan `Create`/`UpdateAssignment` yang sentinel-nya diterjemahkan oleh usecase **di paket yang sama**.
- **`fromAssignee := current.AssignedToMembershipID` ditangkap sebelum `UpdateAssignment` dipanggil** — disiplin yang sama persis dengan `fromStatus` di `UpdateStatus` (#21), sekarang diterapkan proaktif di `UpdateAssignment` sejak awal (bukan ditemukan lewat test yang gagal seperti sebelumnya) karena polanya sudah dikenal.
- **Assign ke diri sendiri tetap menulis activity `lead_assigned`, hanya notifikasi yang dilewati** — TD §11 eksplisit hanya menyebut notifikasi ("memberi tahu... menambah bising"), bukan activity. Timeline lead tetap mencatat siapa yang mengambil alih, meski aktor dan penerima adalah orang yang sama.
- **`task_assigned` sengaja tidak dipicu di issue ini** — nilai enum `ck_notifications_type` sudah menyediakannya (keputusan skema TD §2), tapi tidak ada acceptance criteria issue #22 yang mensyaratkan notifikasi saat task di-assign. Dicatat eksplisit sebagai keputusan cakupan, bukan kelalaian — TD §2 sendiri menyebut ini "perluasan kecil yang disengaja", disiapkan untuk dipakai nanti.

### Verifikasi

```
go build ./...                              → bersih
go vet ./...                                → bersih
golangci-lint run                           → 0 issues
gofmt -l .                                  → bersih
go test -race -count=1 ./... (2x)           → semua PASS

DOCKER_HOST=unix:///nonexistent/docker.sock \
  go test ./internal/... -run TestUnit -count=1
                                             → PASS tanpa Docker (notification, lead, membership, + semua paket lama)

go list -deps ./cmd/api | grep docker       → kosong
```

**Test atomisitas (wajib di Definition of Done)**: `internal/membership/handler_test.go` (baru) membuktikan klaim "semuanya atau tidak sama sekali" terhadap transaksi Postgres sungguhan, bukan hanya fake — `on_open_leads=unassign` dengan dua lead terbuka: kedua lead terlepas, dua activity `lead_unassigned` tercatat, membership nonaktif, **dan** refresh token yang sudah ada sebelumnya benar-benar `revoked_at`-nya terisi, semuanya diverifikasi lewat `SELECT` langsung setelah satu panggilan HTTP. Jalur `reject` dibuktikan sebaliknya: `409` dengan jumlah lead terbuka yang benar (mengecualikan lead berstatus `won`), membership **tetap aktif**, refresh token **tetap** tidak revoked — transaksi benar-benar batal, bukan hanya response error yang dikembalikan lebih dulu.

**Smoke test manual** (`docker compose up --build`): register+verify+login owner → undang & terima undangan seorang employee (kolega) → `POST /v1/leads` → `PATCH /assignment` ke kolega → `GET activities` menunjukkan `lead_assigned` dengan `metadata={from:null,to:<kolega>}` → `GET /v1/notifications?unread=true` (sebagai kolega) → tepat satu notifikasi `lead_assigned` dengan title `"Lead #1 ditugaskan kepada Anda"` → `POST .../read` → `204`, hilang dari daftar unread → `PATCH /assignment` ke diri sendiri (owner) → **tidak ada** notifikasi baru → assign ulang lead ke kolega → `DELETE /v1/memberships/{kolega}` tanpa parameter → `409 membership_has_open_leads` dengan `open_lead_count: 1` → `DELETE ...?on_open_leads=unassign` → `204`, lead kembali `assigned_to_membership_id: null` → refresh token kolega (dengan cookie asli + header CSRF yang benar) → `401 invalid_credentials`, membuktikan sesi benar-benar mati. Catatan: `GET /v1/me` dengan access token JWT kolega yang **belum kedaluwarsa** tetap `200` setelah deactivation — ini bukan bug, access token JWT stateless tidak bisa dicabut instan, hanya refresh token opaque yang dicabut; ini konsisten dengan desain sejak #10.

Seluruh acceptance criteria issue #22 terpenuhi. Jalur `reassign` diverifikasi otomatis lewat test Postgres (`TestHandler_Deactivate_OnOpenLeadsReassign_MovesLeadsAndLogsActivity`), tidak diulang manual.

### Utang teknis

- Tidak ada item baru dari issue ini sendiri.

### Catatan untuk session berikutnya

- **`internal/lead/port.go`'s `Repos{Lead, Activity, Notification}` sekarang tiga field.** #23 (konversi ke Customer) kemungkinan menambah field lagi untuk menulis `customers` + activity `lead_converted` dalam satu transaksi.
- **Pola "sentinel domain diterjemahkan di usecase paket sendiri, tapi bridge lintas paket menerjemahkan langsung ke `httpx`"** sekarang punya satu contoh konkret (`lead.ReassignOpen`). Berlaku untuk bridge serupa yang akan datang — kapan pun sebuah method HANYA melayani sebagai jembatan `OpenLeadRepository`-style, jangan berasumsi errors.Is sentinel domain bisa dikenali pemanggil.
- **`docs/architecture/authorization.md`'s matrix masih belum diperbarui** — tetap ditunda ke #23 (penutup Phase 2), sekarang bertambah `lead.assign` dan seluruh baris `activity.*`/`task.*` dari #21 yang juga belum masuk.
- **Retensi `notifications` belum ada** — sama seperti `idempotency_key` (dicatat di `## #20`), TD tidak menyebut kebijakan retensi untuk notifikasi lama, dan Phase 2 tidak punya scheduler untuk membersihkannya. Tidak mendesak di volume MVP; dicatat di sini agar tidak terlupa saat volume bertambah.
