# Project Status

> **Ledger state project.** Dibaca di **awal setiap session**, diperbarui di **akhir setiap session**.
> Ini satu-satunya jawaban atas pertanyaan *"sekarang sudah sampai mana?"* — jangan merekonstruksinya dari kode.

**Last updated:** 26 Agustus 2026 — Issue #34 selesai (tim: role, undang anggota, nonaktifkan, notifikasi)
**Phase sekarang:** Phase 3 — Owner Dashboard (6/7 issue selesai — #40 ditambahkan setelah hasil desain masuk)

---

## Selesai

| Item | Session | Phase | Catatan |
|---|---|---|---|
| Brainstorming & architecture review | — | — | `docs/brainstorming/` — arsip |
| Product & architecture decisions | — | — | `docs/product/decisions.md` |
| ADR-007 — kardinalitas User → Organization | — | — | Accepted |
| Architecture freeze + 6 amandemen | — | — | `docs/architecture/freeze.md` — 🔒 FROZEN |
| Bootstrap documentation | — | — | CLAUDE.md, STATUS.md, product/, architecture/, ADR-001..006, skill backend |
| Delivery workflow + setup repository | — | — | ADR-008, `docs/workflow.md`, template GitHub, git init, label & milestone |
| Restrukturisasi monorepo | — | — | ADR-009 — `crm_be/`, `crm_dashboard/`, `crm_landing_page/`, `crm_employee/` |
| **Issue #1 — Project skeleton** | — | 0 | PR [#5](https://github.com/Pravasta/jualin-crm/pull/5). Config ter-validasi saat boot, logger + request_id, error envelope, `/health`, graceful shutdown, CI. |
| **Issue #2 — Database, Docker Compose, migration** | — | 0 | `db.InTx`, `cmd/migrate` (goose), `0001_baseline`, Dockerfile + `docker-compose.yml` di akar, `/health/ready`. Diverifikasi end-to-end: `docker compose up` tanpa langkah manual, migration up/down bersih, `/health/ready` degradasi & pulih otomatis saat DB mati/hidup. |
| **ADR-010 — Fail-fast startup** | — | — | Muncul dari review PR #6, dikonfirmasi pemilik produk sebagai prinsip umum (bukan hanya DB). Aturan #36 di `CLAUDE.md`. |
| **Issue #3 — Test harness PostgreSQL asli** | — | 0 | `internal/shared/db/dbtest` (subpaket terpisah dari `db` produksi — testcontainers tidak ikut ter-link ke binary). Mengotomasi `db.InTx`, migration round-trip, `/health/ready` yang tadinya manual. **Phase 0 selesai.** |
| **Issue #8 — Schema 0002, tenant context, pola repository, test katalog** | — | 1 | 9 tabel identity, `internal/shared/tenant`, `db.Querier`, `internal/membership` + `internal/user` sebagai contoh repository tenant-scoped/global. Test katalog **diverifikasi bisa gagal** secara adversarial (lihat notes.md). |
| **Issue #9 — Registrasi atomik, argon2id, verifikasi email** | — | 1 | `POST /v1/auth/{register,verify-email,verify-email/resend}`. `httpx.DomainError` (mekanisme error domain generik, baru), `internal/{organization,subscription,auditlog,auth}`, `internal/shared/{password,token,mailer,ratelimit}`. Registrasi 6-insert atomik dalam satu `db.InTx`; email di luar transaksi (Aturan #32). Rate limit **dibuktikan aktif** di test HTTP. |
| **ADR-011 — Layering per-paket + Unit of Work** | — | — | Diminta pemilik produk di tengah Phase 1, diprioritaskan sebelum #10. Merevisi Aturan #8, menegakkan Aturan #11 yang sejak awal dilanggar. |
| **Issue #15 — Refactor: layering, interface, Unit of Work** | — | 1 | `internal/auth` dipecah jadi `entity/port/usecase/repository_postgres/handler_http.go`. `Store`/`Repos` (Unit of Work) menggantikan `db.InTx` langsung di usecase. Composition root di `cmd/api/auth_store.go`. **7 unit test baru lolos tanpa Docker** (dibuktikan lewat `DOCKER_HOST` tak valid + inspeksi `go list -deps`). 33 test lama lolos tanpa perubahan asersi. |
| **Issue #10 — Login, refresh rotation, logout, reset password, CSRF, GET /v1/me** | — | 1 | `POST /v1/auth/{login,refresh,logout,password/forgot,password/reset}`, `GET /v1/me`. Access token JWT (`internal/shared/accesstoken`) + refresh token opaque dengan rotasi & deteksi penggunaan ulang (`SELECT ... FOR UPDATE`, revoke seluruh `family_id`). Dashboard = cookie `HttpOnly`; Mobile = body — dibuktikan lewat test HTTP & smoke test manual. CSRF double-submit (`httpx.VerifyCSRF`) untuk request cookie non-GET. `LoginLimiter` (backoff progresif). `docs/architecture/authentication.md` baru. **12 test integrasi + 13 unit test baru** (plus 5 test `LoginLimiter`, 4 test `accesstoken`), semuanya lolos tanpa perubahan asersi lama. |
| **Issue #11 — RBAC, invitation, penonaktifan membership, harness isolasi tenant** | — | 1 | `internal/shared/{authn,authz}` (session middleware diekstrak dari `auth`, RBAC baru). `internal/membership` naik jadi domain penuh (`port/usecase/handler_http.go`). `internal/invitation` baru — undangan dua cabang (B4), keduanya diuji termasuk test keamanan wajib. `POST/GET/DELETE /v1/invitations`, `GET/PATCH/DELETE /v1/memberships`. Penonaktifan membership mencabut refresh token dalam transaksi yang sama — **dibuktikan lewat smoke test end-to-end**. `docs/architecture/authorization.md` baru. **Harness isolasi tenant (`cmd/api/tenant_isolation_test.go`) dibangun & dibuktikan bisa gagal** (lapis 4, blocking CI). Satu bug nyata (email tidak terverifikasi otomatis saat accept undangan) ditemukan lewat smoke test manual, diperbaiki, dan mendapat test regresi di kedua level. **Phase 1 selesai.** |
| **Issue #19 — Schema 0003, repository lead, alokasi `lead_number`, optimistic locking** | — | 2 | Migration `0003_crm_core` (`leads`, `customers`, `activities`, `tasks` + `organizations.next_lead_number`). `internal/lead` — repository murni (`entity.go`+`repository_postgres.go`, tanpa `port.go`/usecase — pola `membership` pra-#11). Alokasi `lead_number` berurutan per organization dan optimistic locking `version`, **dibuktikan di bawah konkurensi nyata** (20 goroutine bersamaan). Visibilitas employee ditegakkan di repository. Klaim awal soal kebutuhan `db.InTx` sempat berlebihan di komentar kode — diuji langsung sebelum commit, ternyata test konkurensi tidak membuktikannya; ditulis test terpisah (`TestCreate_FailedInsertInsideInTx_DoesNotBurnLeadNumber`) yang benar-benar membuktikan. Test katalog (dari #8) otomatis mencakup keempat tabel baru **tanpa perubahan**. |
| **Issue #20 — Lead CRUD, transisi status, filter, pagination, idempotency, E.164** | — | 2 | `internal/lead` naik jadi domain penuh (`port/usecase/handler_http.go`, `Repository` jadi interface — migrasi yang sama seperti `membership` di #11). `POST/GET/PATCH/DELETE /v1/leads`, `PATCH /v1/leads/{id}/status`. `internal/shared/phone.ToE164` (Indonesia-first). Idempotency-Key dideteksi lewat unique-violation database, **dibuktikan dengan 10 POST bersamaan → tepat 1 lead**. Optimistic locking `409` membawa keadaan terkini di body. Dua `Action` RBAC baru (`lead.create/read/update/delete`). **Bug nyata ditemukan lewat test yang salah pakai** (assignee tak valid → 500 diam-diam) — diperbaiki jadi `400` bersih + test regresi. Satu penyimpangan TD didokumentasikan (keluar dari `lost` belum bisa "satu langkah tepat" tanpa riwayat dari #21). 34 test baru di `internal/lead`, semuanya lolos tanpa perubahan asersi lama. |
| **Issue #21 — Activity append-only + auto-log, dan Task** | — | 2 | `internal/activity` dan `internal/task` baru — dua domain penuh. `ActivityRecorder` dideklarasikan konsumen (`lead`, `task`), dijembatani `activity.NewRecorder(q)` di composition root — pola `auth.RefreshTokenRevoker` (#11). `lead_created`, `status_changed` (`metadata={from,to}`), `task_created`, `task_completed` ditulis **di dalam `Store.InTx` yang sama** dengan pemicunya — `lead.Usecase.UpdateStatus` kini juga dibungkus `InTx`. `GET/POST /v1/leads/{id}/activities` (append-only — **tidak ada** `PATCH`/`DELETE`, diverifikasi lewat daftar route sungguhan). Tipe sistem dari client → `422`. `internal/task`: visibilitas employee lewat kepemilikan **lead**, bukan assignee task sendiri — dibuktikan test khusus. jsonb pertama yang benar-benar ditulis di codebase ini (`activities.metadata`), diverifikasi round-trip langsung terhadap Postgres asli. Atomisitas dibuktikan dua lapis: fake (unit) dan transaksi Postgres sungguhan (`repository_atomicity_test.go`, membuktikan rollback nyata + `lead_number` tidak "terbakar"). Satu bug desain (visibilitas `task.FindAllByLead`) ketahuan dan diperbaiki sebelum commit. Enam `Action` RBAC baru. Smoke test manual end-to-end lolos seluruh acceptance criteria. |
| **Issue #22 — Assignment, notification (`0004`), penutupan kewajiban penonaktifan membership** | — | 2 | Migration `0004_notifications`. `internal/notification` baru — satu-satunya resource di codebase ini tanpa akses lebih luas untuk Owner/Admin, selalu di-scope ke `t.MembershipID`. `PATCH /v1/leads/{id}/assignment` — activity `lead_assigned`/`lead_unassigned` **dan** notification (kecuali assign ke diri sendiri) dalam satu `Store.InTx`. **Menutup kewajiban warisan Phase 1**: `DELETE /v1/memberships/{id}` menolak default (`409 membership_has_open_leads`) bila masih ada lead terbuka; `?on_open_leads=unassign\|reassign` sebagai jalan keluar, atomik dengan pencabutan refresh token yang sudah ada sejak #11 — **dibuktikan lewat test Postgres sungguhan**, bukan hanya fake (`internal/membership/handler_test.go`, baru). `internal/lead.OpenLeadRepository` (bridge terpisah, bukan bagian `Repository`) dipakai `internal/membership` lewat interface lokalnya sendiri — `membership` tetap tidak pernah mengimpor `lead`. Satu batu sandungan arsitektural nyata: sentinel error domain (`lead.ErrAssigneeNotFound`) tidak bisa dikenali lintas paket tanpa melanggar ADR-011 — diselesaikan dengan mengembalikan `*httpx.ValidationError` langsung dari bridge method itu sendiri. Satu `Action` RBAC baru (`lead.assign`). Smoke test manual end-to-end lolos, termasuk verifikasi refresh token benar-benar mati via `/v1/auth/refresh`. |
| **Issue #23 — Customer, konversi dari lead, kasus `lead` pada harness isolasi tenant** | — | 2 | `internal/customer` baru — `POST /v1/leads/{id}/convert` (satu `INSERT ... SELECT ... FROM leads WHERE status='won'`, menyalin field lead ke customer baru dalam satu statement, bukan lewat bridge Go — deviasi sadar dari asumsi awal bahwa konversi akan menambah field ke `lead.Repos`, yang ternyata tidak diperlukan). `uq_customers_org_lead` menegakkan konversi tunggal (`409 lead_already_converted`); lead bukan `won` → `422` (memakai ulang `invalid_status_transition`, tidak ada kode baru). Lead **tidak pernah** berubah oleh konversi atau oleh edit customer setelahnya — dibuktikan langsung, bukan diasumsikan. **Penutup Phase 2**: harness isolasi tenant (`cmd/api/tenant_isolation_test.go`) bertambah entri `lead`/`task`/`customer`/`activity` ke slice `[]isolationCase` yang sama sejak #11 (13 subtest, semua `404`), **terbukti bisa gagal** diulang untuk `lead` (predikat tenant-scoping dihapus sementara → kebocoran data nyata 200, bukan sekadar 500). `docs/architecture/authorization.md` matriks Phase 2 ditulis lengkap (Aturan #1 "belum berlaku" → terwujud); `multi-tenancy.md` lapis 4 kasus #1 & #4 → ✅; `api.md` menambah `lead_already_converted` plus membackfill dua kode yang luput dicatat di #21/#22. Empat `Action` RBAC baru. Smoke test manual end-to-end lolos seluruh acceptance criteria Phase 2. **Phase 2 selesai.** |
| **Issue #30 — CORS + endpoint metrik** | — | 3 | Pembuka Phase 3, murni Go, **tidak ada UI**. `internal/shared/httpx/cors.go` baru — origin di-echo eksplisit (tidak pernah `*`), `OPTIONS` → `204` tanpa menyentuh handler, dipasang sebelum route manapun dan sebelum `authn.Middleware`. `CORS_ALLOWED_ORIGINS` di `internal/shared/config`, wajib non-kosong saat `APP_ENV=production` (Aturan #36). `internal/metrics` baru — **read-only, sengaja tanpa `Store`/`InTx`** (TD §2, penyimpangan sadar dari bentuk lima berkas). `GET /v1/metrics/summary` (`total_new`, `by_status`, `unassigned`, `conversion_rate`) dan `GET /v1/metrics/employees` (per membership: `lead_count`, `avg_response_seconds`, `converted_count`). `conversion_rate` mengecualikan `spam`/`unqualified` dari **penyebut** (menegakkan acceptance criterion #5 Phase 2 yang sampai sekarang hanya "kedua status ada") dan mengirim `null` (bukan `0`) saat penyebut nol. `avg_response_seconds` mengecualikan lead yang belum tersentuh dari rata-rata lewat perilaku native `avg()` mengabaikan `NULL` — tidak ada cabang logic tambahan, dibuktikan lewat test yang menaruh activity `lead_created` di lead yang sama untuk memastikan tipe itu benar-benar tidak ikut terhitung. `ActionMetricsRead` baru (Owner/Admin/Manager, bukan Employee). Harness isolasi tenant bertambah `TestTenantIsolation_MetricsAggregate_ScopedToOrganization` (bentuk terpisah dari slice `isolationCase` karena endpoint agregat tidak punya `:id`) — **terbukti bisa gagal** (predikat tenant di-tautologi-kan → kebocoran nyata 3 vs 1). 18 test baru di `internal/metrics` (unit + Postgres asli + HTTP), semuanya lolos tanpa perubahan asersi lama. |
| **Issue #31 — Setup Next.js, klien API, sesi, auth UI** | — | 3 | `crm_dashboard/` dari `README.md` saja menjadi aplikasi utuh — Next.js App Router + TypeScript + Tailwind v4 + shadcn/ui (`components.json` dengan `registries: {}` kosong, tanpa registry privat). `src/lib/api-client.ts`: `credentials:'include'`, `X-CSRF-Token` di setiap non-GET, dan **refresh single-flight** — satu `refreshPromise` modul-level yang ditetapkan sinkron sebelum `await` apa pun, sehingga request paralel yang 401 bersamaan memakai ulang Promise yang sama, bukan masing-masing memanggil refresh sendiri. Route group `(auth)` vs `(protected)` — layar protected memanggil `GET /v1/me` di layout-nya (`SessionGate`), **tanpa** `middleware.ts` (tidak bisa baca token `HttpOnly`). 5 layar auth (login, register, verifikasi email, lupa/reset password) + pilih organization (`409 organization_selection_required`, ADR-007) ditangani inline di form login, bukan route terpisah. Test runner **Vitest** (dikonfirmasi pemilik produk) — 6 test di `api-client.test.ts`, termasuk **test konkurensi genuine** (refresh sengaja ditahan lewat `deferred()` sampai 6 panggilan paralel semuanya mencapai titik single-flight) yang membuktikan tepat 1 panggilan `/v1/auth/refresh`, diulang 5× tanpa flaky. Kontrak backend (`src/lib/auth.ts`) dibangun dari pembacaan literal `crm_be/internal/auth/*.go`, **diverifikasi end-to-end lewat `curl`** terhadap `crm_be` sungguhan (register → verify → login → cookie flags persis benar → CSRF terbukti aktif → logout) — bukan diasumsikan dari baca kode saja. `.github/workflows/ci-dashboard.yml` baru (`paths: crm_dashboard/**`). **Phase 3 sekarang punya UI** — #32 (daftar lead) bisa mulai. |
| **Issue #40 — Fondasi desain: token warna, label Indonesia, app shell** | — | 3 | **Issue di luar rencana awal**, dibuka setelah hasil Claude Design masuk: desainnya mencakup ~15 layar (seluruh #32–#35) sementara token, label, dan kerangka aplikasi dipakai bersama dan tidak dimiliki satu pun dari mereka (design brief §7.6 sudah menandainya "belum ada dan dibutuhkan"). Hasil desain dibaca lewat `DesignSync` dari project `5ac090ad`. **Aksen desain gagal WCAG AA dan diperbaiki, bukan disalin apa adanya**: `oklch(0.58 0.19 41)` dipakai sebagai latar tombol dengan teks putih 14px = **4.45:1**, di bawah ambang 4.5:1 — diturunkan ke `oklch(0.56 0.19 41)` (**4.83:1**, `#D14400`→`#CA3C00`, tak terlihat mata). **Lima dari delapan badge status juga gagal**, terburuk `proposal` **3.14:1** → 4.55:1; diperbaiki dengan menurunkan *lightness* saja sehingga hue/chroma desain dan tampilan badge tetap sebagaimana digambar. Dua token aksen dipisah karena tugasnya berlawanan: `--primary` (0.56) untuk putih-di-atas-warna, `--accent-strong` (0.48, 7.04:1) untuk warna-di-atas-putih. `src/lib/labels.ts` baru — 8 status + 6 alasan kalah + 4 sumber + 4 role, satu tempat supaya tidak ada layar yang menuliskan ulang peta yang sama. Label nav desain ("Home"/"Task"/"Settings") **diterjemahkan** jadi Beranda/Tugas/Pengaturan (acceptance criterion #12); "Lead"/"Customer" tetap karena keduanya istilah `glossary.md` — dikunci test. Logika nav (`isActive`/`pageTitle`) dipisah ke `lib/nav.ts` agar bisa diuji tanpa merender React — jebakannya nyata (prefix-match naif membuat `/` aktif di setiap halaman). **Bug font sejak #31 diperbaiki**: `--font-sans` menunjuk dirinya sendiri sehingga Geist tidak pernah diterapkan; dibuktikan dari CSS hasil build. 9 test baru (15 total). Lima halaman placeholder bertanggal, masing-masing menyebut issue penggantinya. |
| **Issue #32 — Daftar lead: filter, pencarian, pagination** | — | 3 | Layar traffic tertinggi di seluruh produk (freeze 3.2). URL adalah sumber kebenaran filter (`useSearchParams` + `router.replace`); setiap perubahan filter mereset `page` ke 1. Kata kunci di-debounce 300ms; `AbortController` di setiap fetch (leads, memberships, metrics) mencegah respons lambat untuk filter yang ditinggalkan menimpa yang baru. **Bug nyata ditemukan saat verifikasi terhadap `crm_be` sungguhan**: `GET /v1/metrics/summary`'s `by_status` adalah Go map — status dengan nol lead **dihilangkan dari JSON**, bukan dikirim `0`; kode naif (`summary?.by_status[status] ?? "…"`) akan menampilkan "…" **selamanya** untuk status yang belum pernah dipakai, tak bisa dibedakan dari sedang memuat. Diperbaiki dengan fungsi murni `statusCount()` di `lib/metrics.ts` yang memisahkan "summary belum dimuat" dari "key tidak ada di summary yang sudah dimuat" — dikunci test yang membangun ulang response asli sebagai fixture. `loading` adalah *derived state* (`loadedKey !== requestKey`), bukan `setState` sinkron di effect — ESLint rule `react-hooks/set-state-in-effect` (baru di toolchain React 19) menolaknya. Hitungan chip status/tanpa-pemilik dari `GET /v1/metrics/summary`, di-scope periode saja (bukan dipersempit sumber/pemilik/kata kunci — satu request untuk delapan angka, didokumentasikan sebagai simplifikasi sadar). Kolom Pemilik diselesaikan di klien: `leadJSON` backend hanya kirim `assigned_to_membership_id`, di-lookup lewat `GET /v1/memberships` yang diambil sekali. Baris tabel **sengaja tidak bisa diklik** — issue eksplisit "detail lead belum ada di sini" (#33). `source` selalu `"manual"` saat buat lead dari layar ini. Badge nav "Lead" (kosong sejak #40) tersambung ke jumlah lead tanpa pemilik aktif. Logika murni dipisah & diuji mengikuti pola `lib/nav.ts`: `lib/lead-filters.ts`, `lib/date.ts`, `buildQuery` di `lib/leads.ts`. 18 test baru (33 total). Diverifikasi lewat `curl` terhadap `crm_be` sungguhan dengan 5 lead nyata (status/assignment/pencarian semua menghasilkan `meta.total` yang benar) — bukan hanya lewat mock. |
| **Issue #33 — Detail lead: timeline, activity, task, status, assignment, konversi** | — | 3 | Layar dengan hampir seluruh aksi tulis produk. **Logika transisi status desain adalah simplifikasi prototipe yang TIDAK diikuti** — `lib/lead-status.ts`'s `isValidStatusTransition` ditulis ulang sebagai port baris-demi-baris dari `validateStatusTransition` Go (backend membolehkan keluar dari `lost` ke seluruh 5 status jalur utama, bukan cuma "Baru" seperti mockup), diuji terhadap matriks lengkap 8×8 yang ditulis tangan dari TD, terpisah dari implementasinya. Satu penyempitan dipertahankan sadar: UI tetap hanya menawarkan "→ Buka kembali ke Baru" untuk `lost` (bukan kelima opsi yang backend benarkan) — sah karena acceptance criterion melarang menawarkan yang *tidak sah*, bukan mewajibkan menawarkan *semua* yang sah; dikunci test. Bentuk metadata activity untuk 10 tipe diverifikasi terhadap `crm_be` sungguhan lewat siklus hidup lead penuh — `lead_created` metadata ternyata `null` (bukan berisi source seperti dugaan), `task_completed` tidak membawa `title` (beda dari `task_created`) — keduanya dikunci test di `activity-text.test.ts` supaya asumsi salah tidak lolos diam-diam. **Dua bagian tidak ada di mockup sama sekali**, ditambahkan karena checklist mewajibkan: form edit field umum (`edit-lead-dialog.tsx`) dan form buat task (`new-task-dialog.tsx` — kode sumber desain sendiri membuat task hardcode tanpa form, ditandai "visual only"). `ConflictDialog` satu tempat untuk status/assignment/edit — tombol "Muat ulang" memicu refetch penuh, tidak pernah menerapkan `error.current` otomatis (Aturan #35). `EditLeadDialog` di-*remount* lewat `key={lead.id}-${lead.version}`, bukan `useEffect`+`setState` (pola yang sama seperti `loading` di #32 untuk lolos `react-hooks/set-state-in-effect`). Tombol Konversi hilang berdasar `activities.some(type==='lead_converted')`, bukan cuma `status==='won'` (Lead tidak punya flag "sudah dikonversi"). Hapus lead & Konversi disembunyikan untuk Manager (RBAC Owner/Admin saja). Dropdown penugasan **tidak** memfilter Employee (beda dari mockup) — Employee justru target assignment paling wajar. 18 test baru (54 total). Diverifikasi lewat `curl` terhadap `crm_be` sungguhan — status lengkap `new→...→won`, `lost` tanpa alasan ditolak `400`, convert dua kali → `409 lead_already_converted`, dan **`version` basi memicu `409 version_conflict` dengan `error.current` berbentuk persis `Lead`** — dibuktikan langsung, bukan diasumsikan. |
| **Issue #34 — Tim: role, undang anggota, nonaktifkan, notifikasi** | — | 3 | **Logika permission mockup salah untuk Owner-vs-Owner, TIDAK diikuti apa adanya** — mockup memblokir ganti-role/nonaktifkan untuk baris manapun `role==='owner'`, termasuk saat actor-nya sendiri Owner; `docs/architecture/authorization.md` eksplisit membolehkan Owner mempromosikan Owner lain (co-owner). `lib/team-permissions.ts` ditulis ulang memeriksa `actor.role==='admin' && row.role==='owner'` secara spesifik, dikunci test, **dibuktikan lagi terhadap `crm_be` sungguhan**: Owner promosikan Admin jadi co-Owner → `200`; Admin biasa coba ubah role Owner → `403`. Deaktivasi tiga-cabang (freeze 2.3 #3) **selalu mencoba dulu** `DELETE` tanpa parameter — dialog hanya terbuka setelah `409 membership_has_open_leads` sungguhan, tidak pernah spekulatif (draf pertama salah, membuka dialog langsung di `onClick`, diperbaiki sebelum verifikasi). Role Employee **hilang dari `<select>` undang** di mockup (hanya admin/manager) padahal `ck_invitations_role` membolehkannya — diperbaiki, dibuktikan `POST /v1/invitations {role:"employee"}` → `201`. **Rute terima undangan salah diletakkan di `/invite`** — dibaca ulang dari `internal/invitation/usecase.go:274`, link email sungguhan adalah `/invitations/accept` (konvensi sama seperti `/verify-email`/`/reset-password`), dipindah sebelum commit. Dua cabang terima undangan (`user_exists`): user baru → form nama+password → `/login`; user ada → tombol tunggal, kasus "belum login" ditangani otomatis oleh penanganan `401` `apiFetch` yang sudah ada sejak #31, tanpa cabang kode baru — dibuktikan `401 authentication_required` saat dicoba tanpa sesi. `canManageTeam` menjaga seluruh permukaan admin sekaligus termasuk fetch `listInvitations` itu sendiri (Manager tidak punya `ActionInvitationList` sama sekali, bukan hanya disembunyikan). Notification bell fetch-on-mount + `refreshKey`; `title` notifikasi (kalimat Indonesia lengkap dari backend) ditampilkan apa adanya, tidak direkonstruksi. 10 test baru (64 total). Diverifikasi lewat `curl` terhadap `crm_be` sungguhan — seluruh 4 aturan RBAC relationship, ketiga cabang deaktivasi (reject→409+count, unassign, reassign), kedua cabang accept undangan, dan notifikasi assign→list→mark-read→mark-all-read. |

---

## Sedang Dikerjakan

_(kosong)_

---

## Berikutnya

**Issue #35 — Customer, task, settings, home** (`crm_dashboard`)

- Cakupan & acceptance: [issue #35](https://github.com/Pravasta/jualin-crm/issues/35)
- TD: `docs/phases/03-owner-dashboard/td.md` (bagian sisa layar)
- `apiFetchList`/`buildQuery` (dari #32) adalah pola siap pakai untuk `/customers` dan `/tasks` — keduanya
  endpoint list berpaginasi dengan bentuk yang sama seperti `/leads`.
- `ConflictDialog`/`versionConflictCurrent<T>` (dari #33) generik, dipakai ulang langsung untuk task.
- Pola fetch-on-mount+`refreshKey` (`notification-bell.tsx`, #34) dan pola "coba dulu, cabang hanya pada
  error tertentu" (`handleDeactivateClick`, #34) bisa dipakai ulang kalau layar ini butuh bentuk serupa.
- Desain layar ini ada di project `5ac090ad` (`DesignSync` → `get_file`) — baca ulang penuh sebelum
  implementasi, jangan andalkan potongan lama. Riwayat sesi-sesi sebelumnya menemukan simplifikasi
  prototipe yang menyimpang dari backend sungguhan di **setiap** layar sejauh ini (#32 by_status map,
  #33 transisi status `lost`, #34 permission Owner-vs-Owner) — perlakukan mockup sebagai referensi visual,
  bukan sumber logika, dan verifikasi tiap keputusan terhadap `crm_be` yang jalan.
- Berurutan: #30 ✅ → #31 ✅ → #40 ✅ → #32 ✅ → #33 ✅ → #34 ✅ → #35. Rincian di
  `docs/phases/03-owner-dashboard/issues.md`.

---

## Utang Teknis

| Item | Dari | Catatan |
|---|---|---|
| Tidak ada test end-to-end otomatis untuk graceful shutdown | Issue #1 | Diverifikasi manual (build binary + SIGINT). Otomatisasi butuh test yang menjalankan binary sungguhan dan mengirim sinyal OS — `go run` tidak meneruskan sinyal ke child process, jadi tidak bisa diuji lewat itu. Belum ada issue yang mencakup ini; angkat saat menyentuh area shutdown lagi. |
| Tidak ada auto-migrate saat container `api` start | Issue #2 | `make migrate-up` dijalankan manual. Sengaja dipisah dari entrypoint `api` — migration dan serving punya kelas kegagalan berbeda. |
| `ratelimit.FixedWindow` tidak pernah membersihkan key lama | Issue #9 | Map tumbuh tanpa batas seiring IP/email baru muncul. Tidak masalah di volume MVP; perlu eviction sebelum traffic produksi nyata. |
| Angka rate limit (register 5/jam, resend 3/jam+10/jam) belum final | Issue #9 | Cukup untuk membuktikan mekanisme aktif, bukan hasil tuning. Freeze mencatat "strategi rate limit final" sebagai keputusan terbuka hingga Phase 4. |
| `leads.idempotency_key` tidak punya retensi | Issue #20, TD §7 | Disimpan selamanya — key yang dipakai ulang setahun kemudian mengembalikan lead lama. Tidak berbahaya (tidak pernah membuat duplikat), tetapi salah. Baru relevan saat Phase 4 (API publik) membuat integrator sungguhan mulai mengirim key. |
| `notifications` tidak punya retensi | Issue #22, TD §2 | Sama seperti `idempotency_key` — tidak ada scheduler di Phase 2 untuk membersihkan notifikasi lama. Tidak mendesak di volume MVP. |

> ~~Test otomatis `db.InTx` dan migration round-trip~~ — selesai di issue #3.

> Bagian ini sama pentingnya dengan bagian Selesai. Kompromi yang diambil di session 3 akan terlupa di session 12 lalu ditemukan kembali sebagai bug produksi.

---

## Keputusan Belum Diambil

Tidak ada yang memblokir. Semuanya diputuskan saat fitur terkait dikerjakan.

| Kode | Keputusan | Diputuskan sebelum |
|---|---|---|
| — | Email provider (Resend / Postmark / SES) | Phase 1 — ⏳ *lead time, lihat bawah* |
| — | Domain final & branding | Phase 1 — ⏳ *lead time, lihat bawah* |
| — | Hosting & managed PostgreSQL | Phase 0 akhir |
| — | Retensi data free tier | Phase 8 |
| — | Push provider detail | Phase 5 — ⏳ *lead time, lihat bawah* |
| — | Pricing final & limit free tier | Phase 8 |
| — | Kontrak integrasi payment service | Sebelum Phase 8 |

Rekomendasi untuk masing-masing ada di `docs/architecture/freeze.md` bagian 7 dan `docs/brainstorming/architecture_product_review.md` bagian 12.

> **B6–B9 ditutup** saat PRD Phase 2 dibuka (21 Agustus 2026), seluruhnya mengikuti rekomendasi freeze: `lost_reason` 6 nilai · mundur satu langkah · `tasks.lead_id NOT NULL` · konversi ke Customer adalah aksi eksplisit. Alasan tiap keputusan ada di `docs/phases/02-crm-core/prd.md` bagian *Keputusan yang ditutup di phase ini*.

> **C1–C3 ditutup** saat PRD Phase 3 dibuka (24 Agustus 2026): **Bahasa UI Indonesia saja, tanpa i18n** (C1 — satu-satunya dari ketiganya yang memang tercatat di tabel ini) · **browser → Go langsung dengan CORS**, bukan BFF (C2) · **shadcn/ui + Tailwind** (C3). C2 dan C3 ternyata belum pernah diputuskan di dokumen manapun; keduanya ditutup di muka, bukan di tengah implementasi. Alasan tiap keputusan ada di `docs/phases/03-owner-dashboard/prd.md`.

---

## ⏳ Punya Lead Time — mulai lebih awal dari phase-nya

> Yang membuat ketiganya berisiko **bukan** pekerjaan kodingnya, melainkan **menunggu pihak lain**.
> Kalau baru diurus di hari pertama phase-nya, phase itu berhenti sebelum dimulai.

| Hal | Kode dipakai di | Mulai diurus | Kenapa |
|---|---|---|---|
| **Domain + email sender** (SPF/DKIM/DMARC) | Phase 1 | **Sekarang** | Verifikasi email menggerbangi login (keputusan B3), jadi ia jalur kritis Phase 1. Propagasi DNS dan pemanasan reputasi pengirim butuh waktu — dan email verifikasi yang masuk spam akan membunuh funnel registrasi **tanpa menghasilkan satu pun error**. |
| **Apple Developer Program** | Phase 5 | Sebelum Phase 3 | Enrollment bisa berhari-hari sampai berminggu (verifikasi identitas / D-U-N-S untuk organisasi). Tanpa ini, build iOS dan APNs tidak bisa jalan sama sekali. |
| **Firebase project (FCM)** | Phase 5 | Bersama Apple Developer | Pembuatannya cepat, tapi konfigurasi sisi iOS bergantung pada APNs key dari akun Apple di atas. |

**Tidak ada yang memblokir Phase 0.** Dicatat di sini justru supaya tidak tersadar terlambat.

### Status

- [ ] Domain final dipilih & dibeli
- [ ] Email provider dipilih (Resend / Postmark / SES)
- [ ] SPF, DKIM, DMARC terpasang & terverifikasi
- [ ] Apple Developer Program terdaftar
- [ ] Firebase project dibuat

> Domain juga menentukan konfigurasi cookie (`Secure`, `SameSite`, scope), CORS, dan alamat pengirim email — semuanya disentuh di Phase 1.

---

## Progress per Phase

> **Status per issue tidak dicatat di sini** — ia hidup di GitHub Issues (ADR-008).
> Dokumen ini hanya melacak level phase.

| Phase | Nama | PRD | TD | Issues | Selesai |
|---|---|---|---|---|---|
| 0 | Foundation | ✅ | ✅ | ✅ #1–#3 | ✅ |
| 1 | Auth & Organization | ✅ | ✅ | ✅ #8–#11, #15 | ✅ |
| 2 | CRM Core | ✅ | ✅ | ✅ #19–#23 | ✅ |
| 3 | Owner Dashboard | ✅ | ✅ | ✅ #30–#35, #40 | ⬜ |
| 4 | Public API | ⬜ | ⬜ | ⬜ | ⬜ |
| 5 | Employee Mobile | ⬜ | ⬜ | ⬜ | ⬜ |

Pekerjaan yang sedang berjalan: `gh issue list --state open`
