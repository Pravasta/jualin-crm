# Phase 6 — Connect & Embedded Form · Notes

One section per issue, appended as each is implemented.

---

## #85 — Migration `0007_forms`, domain `form`, CRUD kredensial

Issue pertama Phase 6, dan fondasi seluruh phase — kredensial keempat produk ini menerbitkan
(`public_key`, ADR-005/D3) beserta manajemennya sebagai principal user (Owner/Admin). `internal/form`
dibangun **murni mengikuti bentuk `internal/apikey`** (Phase 4 #46): `entity.go` → `port.go` →
`usecase.go` → `repository_postgres.go` → `handler_http.go`, tiap file ditulis dengan `apikey`'s
padanannya terbuka sebagai referensi baris-demi-baris.

### Cakupan sadar dipersempit — mengulang split #46/#47 apikey persis

TD §1–§4 sudah menetapkan Phase 6 bertingkat: issue ini **hanya** manajemen (create/list/get/update/
delete). `tenant.Context.FormID`, cabang ketiga `authz` untuk `PrincipalPublicForm`, dan
`Usecase.ResolvePublicKey` **sengaja tidak dibangun di sini** — itu #87. Buktinya bukan tebakan:
`apikey.Repository.FindByKeyID` sendiri sudah ada sejak #46 dengan nol pemanggil HTTP, dan baru
dipakai `ResolveAPIKey` di #47. `form.Repository.FindByPublicKey` mengikuti bentuk itu persis —
dibangun sekarang, diuji langsung di `repository_test.go` (termasuk bukti index-hit lewat `EXPLAIN`),
tapi belum dipanggil dari usecase manapun.

### Migration `0007_forms` — pengecualian unik-lintas-organization **keempat**

`uq_forms_public_key` tidak composite dengan `organization_id` — alasan tertulisnya identik dengan tiga
pendahulunya (`refresh_tokens.token_hash` 0002, `api_keys.key_id` 0005, `device_tokens.token` 0006):
lookup kredensial terjadi **sebelum** organization diketahui, organization justru **hasil** dari lookup
itu (Aturan #5). `fields jsonb NOT NULL` punya alasan tertulis Aturan #17 langsung di komentar migration
— konfigurasi tampilan enam field tetap (ADR-005), dibaca utuh, tidak pernah di-query sebagian.
`ALTER TABLE leads ADD COLUMN source_form_id` + FK komposit mengikuti pola `source_api_key_id` 0005
persis; `leads.source` sudah menerima `'form'` sejak 0003, tidak ada `ALTER` pada enum-nya.

### `Fields` — Go map dengan `Validate()`, bukan struct enam field tetap

`type Fields map[FieldKey]FieldConfig` dengan `FieldKey` sebagai enum Go tertutup (`AllFieldKeys`) —
dipilih daripada `FieldName, FieldEmail string` dst. langsung sebagai field struct, karena marshal/
unmarshal ke JSONB jadi satu baris (`json.Marshal(f.Fields)`) tanpa perlu memetakan manual enam nama
field ke enam kunci JSON. `Validate()` menolak kunci tidak dikenal, `Required` tanpa `Enabled`, dan
`Enabled` tanpa `Label` — dipanggil di `Usecase.Create`/`Update`, bukan di repository (ADR-011:
validasi milik usecase). `DefaultFields()` mengisi nama & telepon wajib dengan label Indonesia yang
masuk akal — form baru langsung bisa dipakai tanpa Owner harus mengisi enam label dari kosong.

### `generate()`/`parsePublicKey()` — sengaja tanpa hash, sengaja tanpa `ConstantTimeCompare`

TD §2 (D3) menjelaskan kenapa ini bukan kelalaian: `public_key` dirancang terbaca semua orang (tertanam
di HTML halaman pelanggan). Meng-hash-nya membuat lookup mustahil tanpa menyimpan nilai mentahnya juga.
Perlindungan endpoint ini bukan kerahasiaan kunci — itu §4/§6 TD (cabang otorisasi tertutup + lima
lapis anti-abuse), keduanya #87. `entity.go`'s doc comment mengutip alasan ini eksplisit supaya siapa
pun yang membaca kode tanpa membaca TD tidak salah kira ini bug.

### Bug ditemukan sebelum commit — parameter SQL tak terpakai di `Update`

Draf pertama `repository_postgres.go`'s `Update` melempar `nil` sebagai `$3` yang **tidak pernah
direferensikan** di query (placeholder melompat dari `$2` ke `$4`) — Postgres menolak dengan
`could not determine data type of parameter $3 (SQLSTATE 42P18)`, tertangkap oleh
`TestRepository_Update_COALESCE_PartialUpdate` sebelum sempat commit. Perbaikannya dua bagian:
menghapus parameter yang tak terpakai, dan menambah cast eksplisit (`$3::text`, `$4::jsonb`,
`$5::text[]`) pada tiap `COALESCE` — tanpa cast, `NULL` literal untuk kolom `jsonb`/`text[]` juga gagal
diinferensi tipenya oleh Postgres. Pelajaran yang sama ini berlaku untuk domain manapun berikutnya yang
menambah `COALESCE`-based partial update pada kolom non-`text`.

### Ketidaktepatan komentar ditemukan & diperbaiki — `Delete` TIDAK idempotent-sukses seperti `apikey.Revoke`

Draf pertama `Usecase.Delete`'s doc comment mengklaim perilakunya "sama seperti `apikey.Revoke`" —
panggilan kedua tetap sukses. Ini salah, dan ketahuan lewat test sendiri
(`TestUnit_Delete_Twice_SecondCallIs404`/`TestHandler_Delete_OwnerAllowed_SecondCallIs404`): `api_keys`
tidak punya `deleted_at` sama sekali, jadi baris yang di-revoke tetap ditemukan `FindByID`. `forms`
**soft-delete** — `FindByID` mengecualikan baris `deleted_at IS NOT NULL` dengan aturan yang sama
seperti mengecualikan cross-org (Rule #6: baris terhapus harus tidak bisa dibedakan dari baris yang
tidak pernah ada). Maka panggilan `Delete` **kedua** selalu 404, bukan 204 berulang — bentuk yang justru
sama seperti `customer.Usecase.Delete` (yang mencapai hasil sama lewat `RowsAffected() == 0` di
repository, bukan pre-check `FindByID`). Komentar diperbaiki sebelum commit; tidak ada perubahan
perilaku, hanya dokumentasi yang tadinya salah.

### `formJSON` — `public_key` selalu tampil utuh, di setiap response

Berbeda dari `apiKeyJSON` (raw secret hanya di response `create`, tidak pernah lagi setelahnya),
`public_key` bukan rahasia (D3) — muncul utuh di `create`, `list`, dan `get`, karena itu justru nilai
yang perlu Owner salin ulang kapan saja untuk snippet embed (#89), bukan sekali saja saat form dibuat.
Diverifikasi eksplisit lewat `TestHandler_Create_ResponseCarriesPublicKeyInFull`.

### `authz` — dua Action baru, bukan satu peta baru

`ActionFormCreate/List/Read/Update/Delete` ditambah ke peta `permissions` yang sudah ada — Owner/Admin
saja, sama sempit dengan `api_key.*` (Manager/Employee nol akses, bukan read-only), karena kredensial
yang bisa menyuntik lead ke organization bukan informasi tim biasa. Tidak ada peta baru dibuat; ini
persis pola yang sudah dipakai lima kali sebelumnya di file yang sama.

### Test

- `entity_test.go` (`package form`, akses langsung ke `generate`/`parsePublicKey` yang unexported) —
  format `public_key`, keunikan 200 generate, `parsePublicKey` menolak bentuk salah, `Fields.Validate()`
  empat kasus, bentuk `DefaultFields()`.
- `usecase_unit_test.go` (`package form_test`, `Store` palsu, tanpa Docker) — otorisasi per role untuk
  kelima operasi, validasi nama/fields, 404 cross-org, idempotency delete (404 pada panggilan kedua,
  **bukan** 204 berulang — lihat catatan di atas).
- `repository_test.go` (Postgres asli lewat `dbtest`) — round-trip create/find, cross-org 404,
  `FindByOrg` mengecualikan yang terhapus, COALESCE partial update (termasuk kasus "allowed_origins
  kosong-tapi-non-nil MENGOSONGKAN, nil TIDAK menyentuh"), `ck_forms_name_not_blank` &
  `fk_forms_created_by` ditolak database (bukan cuma usecase), `uq_forms_public_key` menolak duplikat
  lintas organization, `FindByPublicKey` index-hit lewat `EXPLAIN`.
- `handler_test.go` (Postgres asli, router HTTP penuh) — matriks role per endpoint, `public_key` utuh di
  setiap response, cross-org 404, delete kedua 404, audit log `form.created`/`form.deleted` tercatat.
- `cmd/api/tenant_isolation_test.go` — tiga kasus baru (`GET`/`PATCH`/`DELETE /v1/forms/{id}` lintas
  organization) ditambahkan ke tabel generik yang sudah ada, memakai `seedIsolationForm` (pola sama
  dengan `seedIsolationAPIKey`). **Dibuktikan bisa gagal**: `organization_id = $2` sengaja dihapus
  sementara dari `FindByID`'s query, dua dari tiga kasus baru merah (bukan 404, melainkan galat SQL 500
  karena parameter jadi tak terpakai — konsisten dengan bug §di atas), lalu dikembalikan — perubahan
  ini tidak pernah commit.

### Verifikasi manual end-to-end

Dijalankan lewat `docker compose up -d --build api` + migration nyata ke Postgres compose (bukan
testcontainer), lalu `curl` sebagai dua organization berbeda:

1. Register + login dua user (dua organization terpisah), email diverifikasi manual lewat SQL karena
   `mailpit` tidak ikut dijalankan sesi ini.
2. `POST /v1/forms` dengan cookie session + header `X-CSRF-Token` (wajib, Aturan #25) → `201`,
   `public_key` berbentuk `pk_...` asli, `fields` terisi `DefaultFields()`.
3. `GET /v1/forms`, `GET /v1/forms/{id}` → `public_key` sama persis dengan respons `create`.
4. `PATCH /v1/forms/{id}` mengubah `name` + `allowed_origins` → keduanya berubah, `fields` tidak
   tersentuh (COALESCE bekerja).
5. `GET /v1/forms/{id}` dengan organization LAIN → `404`, bukan `403` (Rule #6 sungguhan, bukan cuma
   test).
6. `DELETE /v1/forms/{id}` → `204`; panggilan kedua → `404` (bukan `204` — konsisten dengan §di atas).
7. `SELECT action FROM audit_logs` mengonfirmasi `form.created` dan `form.deleted` tercatat dengan
   waktu asli.

### Batas issue ini

Tidak ada: `POST /v1/forms/{public_key}/submit` (publik, #87), `GET /embed/{public_key}` (#88),
`tenant.Context.FormID`, cabang `authz` ketiga untuk `PrincipalPublicForm`, `Usecase.ResolvePublicKey`,
UI dashboard apa pun (`/connect`, manajemen form, #86/#89). `Repository.FindByPublicKey` dibangun dan
diuji penuh, tapi nol pemanggil di luar test — persis situasi `apikey.FindByKeyID` antara #46 dan #47.

---

## #86 — Dashboard: permukaan Connect + pindahkan pengelolaan API key

Paralel dengan #85 (tidak bergantung padanya) — murni `crm_dashboard`. Menaikkan `Connect` jadi menu
utama (ADR-012) dan memindahkan pengelolaan API key keluar dari `Pengaturan`, yang selama ini hanya
ditemukan oleh yang sudah tahu ia ada.

### Pemindahan folder — `git mv` utuh, bukan copy manual

`settings/api-keys/` (termasuk subfolder `docs/`) dipindah **satu perintah** ke `connect/api/` dengan
`git mv`, mempertahankan struktur nesting internal apa adanya. Ini penting karena satu-satunya impor
relatif di seluruh folder — `docs/api-docs-screen.tsx`'s `import { CreateAPIKeyDialog } from
"../create-api-key-dialog"` — hanya tetap valid bila jarak relatifnya (satu folder di bawah induk)
tidak berubah. TD §10 menyebut impor ini eksplisit sebagai hal yang "patah saat folder dipindah" —
benar untuk pemindahan naif (copy file satu-satu tanpa menjaga nesting), tapi `git mv` yang
mempertahankan struktur membuatnya tetap resolve tanpa perlu diubah. Diverifikasi lewat `typecheck`,
bukan diasumsikan.

### Tiga `router.push`, bukan lima — TD/issue's hitungan diperbaiki di sini, bukan didiamkan

TD §10 dan isu #86 sama-sama menulis "lima `router.push("/settings/api-keys…")`". Pencarian langsung
(`grep -rn "settings/api-keys" src/`) sebelum menyunting apa pun hanya menemukan **tiga**:
`api-keys-screen.tsx` (ke `/docs`), `docs/api-docs-screen.tsx` (kembali ke induk), dan
`settings-screen.tsx` (kartu "Integrasi API"). Dicatat di sini sebagai perbedaan dokumentasi-vs-kode
(CLAUDE.md §Source of truth: "laporkan, jangan diam-diam mengubah") — bukan kesalahan implementasi,
TD kemungkinan ditulis dari ingatan sebelum kode benar-benar dihitung ulang. Ketiganya diperbaiki ke
`/connect/api` atau `/connect/api/docs` sesuai tujuannya masing-masing.

### Kartu "Integrasi API" di `settings-screen.tsx` — dihapus, bukan cuma dialihkan

Cakupan tertulis issue hanya minta memperbaiki `router.push` yang patah, tapi mengubah tujuannya ke
`/connect/api` sambil MEMPERTAHANKAN kartu itu di Pengaturan akan menyisakan dua pintu masuk ke fitur
yang sama — bertentangan langsung dengan alasan tertulis ADR-012 sendiri ("Kenapa bukan tetap di
dalam Pengaturan": *Pengaturan* jarang disentuh setelah setup awal, *Connect* adalah tempat tesis
produk dibuktikan). Kartunya dihapus utuh berikut `canManageAPIKeys`/`useRouter` yang jadi tidak
terpakai — dicatat lewat komentar di `settings-screen.tsx` supaya pembaca berikutnya tahu ini
kesengajaan, bukan penghapusan yang lupa dikembalikan.

### `/connect` — tiga kartu, tanpa gerbang role di level kartu

`ConnectScreen` menampilkan ketiga kartu (API, Formulir, Webhook) ke **semua role tanpa terkecuali** —
konsisten dengan D6 diperluas dari nav item ke halaman arahnya sendiri: gerbang tetap di layar tujuan
(`/connect/api` sudah menampilkan "tidak tersedia untuk role Anda" untuk Manager/Employee, persis
seperti sebelum dipindah). Kartu Formulir mengarah ke `/connect/form` yang belum berisi apa pun
(#89's scope) — sesuai batas tertulis issue, bukan lupa dibangun. Kartu Webhook memakai `opacity-60`
+ badge "Belum tersedia", **bukan** "terkunci oleh paket" — TD §10 eksplisit soal ini: keadaan
terkunci-oleh-paket baru lahir Phase 8.

### Redirect permanen lewat `next.config.ts`, bukan halaman `page.tsx` yang memanggil `permanentRedirect()`

Dua opsi Next 16 dipertimbangkan (lihat `node_modules/next/dist/docs/01-app/02-guides/redirecting.md`,
dibaca dulu sebelum menulis kode — AGENTS.md wajib untuk versi Next non-standar ini): `redirects()` di
`next.config.ts` (redirect terjadi di layer routing, sebelum render apa pun) vs `permanentRedirect()`
dari `next/navigation` dipanggil di dalam halaman lama. Dipilih yang pertama — persis kasus yang
dokumennya sebut sebagai use case intended ("known ahead of time", jumlahnya cuma dua, tidak perlu
skala Proxy/bloom-filter). **Diverifikasi sungguhan**, bukan dipercaya dari baca kode: `npm run build`
lalu `npm run start` di port terpisah, `curl` langsung ke kedua URL lama —
`308 -> http://localhost:3100/connect/api` dan `308 -> .../connect/api/docs`, keduanya **308 Permanent
Redirect asli**, persis kriteria acceptance ("diverifikasi dengan membuka URL lamanya, bukan dengan
membaca kodenya").

### `nav.ts`/`nav.test.ts`/`app-shell.tsx`

`NAV_ITEMS` bertambah `{ href: "/connect", label: "Connect" }` antara Tim dan Pengaturan; `NAV_ICONS`
di `app-shell.tsx` bertambah `Plug` (lucide-react, diverifikasi ada di package sebelum dipakai).
`nav.test.ts`'s `toEqual([...])` diperbarui — **memang merah sebelum diperbaiki**, itu yang diinginkan
TD §10. Dua kasus baru ditambahkan mengikuti pola penjaga yang sudah ada: `isActive("/connecting-
something", "/connect")` harus `false` (tabrakan prefix, disebut eksplisit oleh TD §10), dan
`isActive("/connect/api", "/connect")` harus `true` (Connect tetap aktif di sub-halamannya). `pageTitle`
untuk `/connect/api` juga diuji — memastikan sort href-terpanjang-dulu tidak salah pilih.

### Verifikasi

- `npm run typecheck && lint && test && build` — bersih. (`.next/` sempat berisi tipe rute basi yang
  merujuk path lama sebelum dihapus manual sekali; ini artefak cache, bukan kode.)
- 85 test lolos (termasuk `nav.test.ts` yang diperbarui).
- `grep -rn "settings/api-keys" src/` setelah seluruh perubahan — kosong, tidak ada rujukan tersisa.
- Redirect 308 sungguhan dibuktikan lewat `curl` (lihat di atas), bukan diasumsikan dari config.

### Batas issue ini

Tidak ada: layar `/connect/form` (#89), gerbang `canManageForms` (#89), apa pun yang menyentuh
`crm_be` (issue ini murni `crm_dashboard`). Kartu Formulir sengaja mengarah ke rute kosong, sesuai
batas tertulis di issue.

---

## #87 — Endpoint submit publik + lima lapis anti-spam

Risiko keamanan tertinggi di phase ini — endpoint pertama yang menerima request dari browser orang
asing, dengan kredensial yang sengaja terbuka. Prasyarat #85 sudah terpenuhi. Cakupan penuh: principal
keempat (`tenant.PrincipalPublicForm`, akhirnya dipakai sejak Phase 1), cabang ketiga `authz.Require`,
paket baru `internal/shared/formtoken` dan `internal/shared/captcha`, cabang `PrincipalPublicForm` di
`lead.Usecase.Create`, dan `POST /v1/forms/{public_key}/submit` itu sendiri.

### Dua keputusan diajukan eksplisit sebelum menulis kode — TD punya celah nyata, bukan diasumsikan

**Content-Type body submit.** TD §7 bilang halaman embed tanpa CAPTCHA "murni HTML +
`<form method="post">`", tapi tidak pernah menyebutkan Content-Type endpoint submit secara eksplisit —
dan itu menentukan cara body di-parse serta bentuk `raw_payload` (kolom `jsonb`). Diajukan ke pemilik
produk: **`application/x-www-form-urlencoded`** dipilih — form native browser tanpa JavaScript,
konsisten dengan kalimat TD §7. Konsekuensinya: `raw_payload` **tidak** byte mentah body (beda dari
jalur API key, yang bodinya sudah JSON) — handler membangunnya dari field yang sudah di-parse,
mengecualikan tiga field protokol (honeypot, token, respons captcha), lalu di-marshal ulang jadi JSON
sebelum disimpan.

**Celah waktu honeypot vs CAPTCHA.** TD §5 menaruh honeypot di posisi 6, sebelum time-trap dan CAPTCHA
di posisi 7-8 — supaya bot tidak pernah dapat pesan error yang bisa dipelajari, dan supaya kuota
verifikasi CAPTCHA tidak terbuang ke request yang sudah diketahui bot. Konsekuensinya: submission
honeypot berhenti jauh lebih cepat daripada submission asli yang sampai memanggil Turnstile (bisa
sampai `CAPTCHA_TIMEOUT`) — celah waktu nyata, sementara AC eksplisit minta "tidak bisa dibedakan
termasuk waktu respons". Diajukan tiga opsi (ikuti urutan TD apa adanya + catat keterbatasan / delay
buatan di jalur honeypot / tetap jalankan time-trap+CAPTCHA di jalur honeypot lalu buang hasilnya).
**Opsi pertama dipilih** — TD §5 sudah punya alasan tertulis untuk urutan ini, menata ulang berarti
menyimpang dari keputusan TD tanpa otorisasi baru. Keterbatasan dicatat di `Usecase.Submit`'s doc
comment dan `api.md`, bukan didiamkan.

### `LeadCreator` — bridge primitif, mengulang arah `lead.ActivityRecorder`/`PushSender` terbalik

`lead.Usecase.Create` **memakai ulang apa adanya** (keputusan D5 TD), tapi `internal/form` tidak boleh
mengimpor `internal/lead` langsung — diverifikasi dulu sebelum menulis apa pun: `grep` seluruh
repository membuktikan **hanya** `cmd/api` yang pernah mengimpor `internal/lead` di luar paket itu
sendiri, tidak ada domain lain yang jadi pengecualian, termasuk `customer` (yang membaca tabel `leads`
lewat SQL repository sendiri, bukan lewat `lead.Usecase`). `LeadCreator` di `form/port.go` karena itu
memakai **primitif saja** (`name string`, `email, phone, company, notes *string`, `rawPayload []byte`)
— bentuk yang sama seperti `lead.ActivityRecorder`/`NotificationSender`/`PushSender`, hanya arahnya
terbalik. `leadCreatorAdapter` (composition root, `cmd/api/form_store.go`) menerjemahkannya ke
`lead.CreateLeadInput` dan memanggil `lead.Usecase.Create` yang **sama persis** dipakai dashboard/API
key — termasuk `authz.Require(ActionLeadCreate)`-nya, yang untuk `PrincipalPublicForm` otomatis
lewat `publicFormAllows`. `Usecase.Submit` sendiri **tidak pernah** memanggil `authz.Require` — ia
sepenuhnya bergantung pada gerbang di dalam `lead.Usecase.Create`.

### `lead.Usecase.Create` — cabang `PrincipalPublicForm`, bentuk sama dengan `PrincipalAPIKey`

`Source` dipaksa `"form"`, `AssignedToMembershipID` ditolak (`insufficient_scope`) bila body
mengirimkannya, `SourceFormID` diisi dari `t.FormID` — bukan dari `in`, alasan yang sama seperti
`SourceAPIKeyID` (Rule #5, dicatat eksplisit di `CreateLeadInput`'s doc comment). **Tanpa** panggilan
`maybeCleanupExpiredIdempotencyKeys` — TD §5 eksplisit forms tidak pernah kirim `Idempotency-Key`.
`leads.source_form_id` sendiri sudah ada sejak migration `0007` (#85) — tidak ada migration baru di
#87.

### `internal/shared/formtoken` — waktu terpotong ke detik, bukan bug, ketahuan lewat test sendiri

Token literal mengikuti TD §6: `base64url(issued_at_unix) + "." + base64url(HMAC-SHA256(...))` — unix
**detik**, bukan milidetik. Draf pertama test batas (`age == minAge - 1ms`, `age == maxAge` persis)
merah karena `issuedAt.Unix()` membulatkan ke bawah ke detik penuh sementara `verifyAt`'s offset test
dihitung dari `issuedAt` yang masih punya pecahan detik — perbedaan sampai ~1 detik antara usia
"sungguhan" dan usia yang dihitung ulang dari timestamp yang sudah dibulatkan. **Bukan bug
implementasi** — TD sendiri menulis literal "unix", jadi kelonggaran sampai ~1 detik di sekitar batas
2 detik memang karakteristik desain yang diterima. Diperbaiki di sisi test: `issuedAt :=
time.Now().Truncate(time.Second)` di setiap kasus batas, supaya model mental test cocok persis dengan
apa yang `issueAt`/`verifyAt` sungguhan simpan. `ErrInvalidToken` satu sentinel untuk tanda tangan
salah/terlalu cepat/kedaluwarsa/rusak — mencerminkan TD §9's satu kode `form_token_invalid`, sama
seperti `apikey.ResolveAPIKey` mengumpulkan semua alasan kegagalan jadi satu 401.

### `internal/shared/captcha` — gagal tertutup, bukan gagal terbuka, saat Cloudflare tak terjangkau

`NoopVerifier` (selalu lolos) dan `TurnstileVerifier` (HTTP POST ke `siteverify`, tanpa SDK — pola
sama seperti `push.FCMSender` memanggil HTTP v1 API FCM langsung, Rule #27). Keputusan sadar:
kegagalan JARINGAN ke Cloudflare (timeout, tak terjangkau) **menolak** submission, bukan meloloskannya
— berbeda dari `push`/`mailer` yang best-effort dan tidak pernah menggagalkan aksi pemicunya. CAPTCHA
di sini adalah gerbang, bukan efek samping: satu hiccup infrastruktur Cloudflare tidak boleh diam-diam
berubah jadi jendela spam terbuka (TD §6 — anti-spam adalah fitur ekonomi). `TURNSTILE_SECRET_KEY`
tidak pernah muncul di pesan error — diverifikasi lewat test yang mencari string secret di
`err.Error()`, bukan dipercaya dari baca kode (Aturan #26).

### Route publik `/v1/forms/:id/submit` — jebakan wildcard TD §8 sendiri, dua `r.Group` beda middleware

`:id` dipakai **dua arti berbeda** di paket yang sama: `/v1/forms/:id` (kelola, `authMW`) memperlakukannya
sebagai id baris; `/v1/forms/:id/submit` (publik, tanpa middleware apa pun) memperlakukannya sebagai
`public_key` — sengaja **tidak** `uuid.Parse`. Gin menuntut nama wildcard sama di segmen yang sama;
preseden persis `internal/task`/`internal/activity` berbagi `/v1/leads/:id` dari paket terpisah, di
sini dari `r.Group` terpisah dalam paket yang sama.

### `submit_count` — tidak diminta eksplisit oleh AC #87, tapi dinaikkan di sini karena #87 satu-satunya tempat yang bisa

Kolom `forms.submit_count` sudah ada sejak #85 dengan komentar "angka untuk ditampilkan di dashboard",
tapi tidak ada AC #87 yang secara literal minta menaikkannya. Diputuskan menaikkannya tetap di sini
karena `Usecase.Submit` adalah **satu-satunya** jalur yang pernah menghasilkan event submission —
kalau tidak dinaikkan di #87, tidak ada issue lain di phase ini yang akan pernah melakukannya, dan
kolomnya jadi permanen mati. Best-effort, **di luar** transaksi lead (tidak bisa berbagi transaksi
dengan `lead.Usecase.Create` yang buram di balik `LeadCreator`) — kegagalan menaikkan tidak pernah
menggagalkan response submission, karena lead-nya sudah benar-benar dibuat.

### Respons submit — `{"id": ...}` saja, bukan `leadJSON` penuh

TD §5 hanya menyatakan langkah 10 berakhir `201`, tidak menspesifikkan bentuk body. Diputuskan minimal
sadar: mengembalikan `leadJSON` penuh (seperti jalur dashboard/API key) akan membocorkan
`assigned_to_membership_id`, `created_by_membership_id`, `lead_number`, dan field internal lain ke
pengunjung anonim yang tidak seharusnya melihatnya sama sekali. `{"id": <uuid lead>}` cukup untuk
kebutuhan #88 nanti (indikator sukses), tanpa membocorkan apa pun.

### Backfill dari #85 — dua celah ditemukan sambil mengerjakan #87, diperbaiki di sini

1. **`authz_test.go` tidak pernah menyentuh Form actions.** `allActions` dan `TestRequire`'s kasus
   per-role masih nol entri `ActionForm*` — file punya komentar sendiri yang eksplisit menyebut ini
   "celah nyata, bukan hipotetis" untuk kasus `ActionAPIKey*` di #46, dan pola yang sama terulang
   untuk `ActionForm*` di #85. Diperbaiki di sini karena #87 toh menyentuh file yang sama untuk
   `PrincipalPublicForm`'s test sendiri.
2. **`multi-tenancy.md` tidak pernah menyebut pengecualian keempat.** TD §1 (#85) eksplisit menulis
   "multi-tenancy.md ditambah baris keempat (§15)" sebagai bagian cakupannya sendiri — tidak
   dilakukan. Ditambahkan sekarang (`## Pengecualian keempat`).

Keduanya dicatat di sini sebagai temuan, bukan diperbaiki diam-diam (Aturan #30).

### Test

- `internal/shared/formtoken/formtoken_test.go` — tanda tangan valid/tamper, batas `<2s`/`>30m`
  inklusif, token form lain ditolak, token rusak ditolak.
- `internal/shared/captcha/turnstile_internal_test.go` + `captcha_test.go` — bentuk request
  `siteverify`, token kosong tidak memanggil jaringan sama sekali, gagal tertutup saat Cloudflare
  tak terjangkau/respons rusak, timeout dihormati, secret/token tidak pernah di pesan error.
  `TestNoopVerifier_AlwaysSucceeds`.
- `internal/shared/authz/authz_test.go` — cabang `PrincipalPublicForm` diuji sebagai tabel atas
  **seluruh** `Action` (bukan daftar tulisan tangan), plus backfill Form actions di atas.
  `TestRequire_PublicFormPrincipal_NeverConsultsRoleMap`.
- `internal/lead/usecase_unit_test.go` — cabang `PrincipalPublicForm`: penugasan ditolak, source
  dipaksa, raw_payload tersimpan, tidak pernah memicu cleanup idempotency.
  `internal/lead/repository_postgres.go`/`entity.go` — `source_form_id` ikut round-trip (dibuktikan
  test repository real Postgres yang sudah ada, bukan test baru).
- `internal/form/usecase_unit_test.go` — 14 test `Submit`: 404 kunci tak dikenal/form terhapus,
  origin ditolak (termasuk allowlist kosong dan header hilang), honeypot sukses-palsu tanpa lead
  DAN urutan setelah origin, token salah/form lain, captcha gagal, field wajib (termasuk `product`
  yang tanpa kolom Lead), sukses penuh (`tenant.Context` yang diteruskan diperiksa persis), gagal di
  `leadCreator` melewatkan increment.
- `internal/form/handler_test.go` — 10 test HTTP: sukses 201 lewat body url-encoded sungguhan,
  field tak dikenal vs field protokol di `raw_payload`, 404/403/413, honeypot 201 tanpa lead di DB,
  rate limit IP dan per-form 429 dengan header, **rate limit sebelum body dibaca** dibuktikan
  langsung (body oversized + limit habis → 429, bukan 413).
- `cmd/api/public_form_api_test.go` — end-to-end dengan `form.Usecase` DAN `lead.Usecase` nyata
  tersambung lewat `leadCreatorAdapter` sungguhan: lead sungguhan muncul dengan `source`/
  `source_form_id` benar, `raw_payload` DB sungguhan, `submit_count` naik, penugasan tidak pernah
  bisa dikirim, origin pelanggan tidak pernah dapat header CORS tapi submission tetap sukses.
- `cmd/api/tenant_isolation_test.go` — kasus baru
  `TestTenantIsolation_FormSubmit_CreatesLeadOnlyInFormsOwnOrganization`: bentuknya beda dari tabel
  404 generik (tidak ada principal yang mencoba resource org lain — intinya submission form org B
  tidak pernah bocor terlihat dari sesi org A). **Dibuktikan bisa gagal**: filter
  `organization_id = $1` dihapus sementara dari `lead.FindAllByOrg`, test merah (org A melihat 1
  lead padahal seharusnya 0), dikembalikan, tidak pernah commit.
- `internal/shared/config/config_test.go` — `TestMain` menambah `FORM_TOKEN_SECRET` global (lewat
  `os.Setenv`, bukan `t.Setenv`, supaya test lain yang tidak tahu variabel ini ada tetap lolos) —
  ditemukan perlu setelah env.Parse (caarlos0/env) terbukti gagal-cepat pada field required
  **pertama** yang hilang menurut urutan deklarasi struct, bukan mengumpulkan semua. 4 test
  produksi (`TestLoad_ProductionMode` dkk.) diperbarui menambah `CAPTCHA_PROVIDER=turnstile` +
  kunci, mengikuti pola `PUSH_PROVIDER=fcm` yang sudah ada di situ. 12 test baru untuk
  `FORM_TOKEN_SECRET`/`CAPTCHA_PROVIDER`/`FORM_SUBMIT_RATE_LIMIT_*`.

### Verifikasi manual end-to-end — sungguhan, `docker compose` + `curl`

`docker compose up -d --build api` (boot sukses membuktikan validasi config baru tidak merusak
fail-fast yang sudah ada), form dibuat lewat dashboard API sungguhan, `allowed_origins` diset, token
time-trap dibangkitkan lewat `cmd/gentoken` sekali-pakai (dihapus setelah dipakai, tidak pernah
commit) memanggil `formtoken.Issue` dengan secret asli dari `docker-compose.yml`:

1. Submit sukses lewat `application/x-www-form-urlencoded` asli → `201`, lead sungguhan di DB dengan
   `source='form'`, `source_form_id` benar, `raw_payload` berisi field tak dikenal (`utm_source`)
   tapi **tidak** `form_token`; `forms.submit_count` naik ke 1.
2. Origin salah dan Origin tidak dikirim sama sekali → keduanya `403 origin_not_allowed` sungguhan.
3. `public_key` tak dikenal → `404 not_found` sungguhan.
4. Honeypot terisi → `201` dengan id yang terlihat asli, tapi jumlah lead di DB **tetap 1** — tidak
   bertambah.
5. Token rusak → `400 form_token_invalid` sungguhan.
6. Body 40KB → `413 payload_too_large` sungguhan.
7. 22 request beruntun dari IP yang sama (termasuk request-request verifikasi di atas, total tepat
   20 sebelum limit) → mulai request ke-21 `429`, header `Retry-After`/`X-RateLimit-*` sungguhan.

### Batas issue ini

Tidak ada: `GET /embed/{public_key}` dan `GET /embed.js` (#88), UI dashboard manajemen form (#89).
`docs/testing/flow/`'s prosedur manual "tempel snippet ke HTML kosong" (TD §12) — butuh halaman embed
(#88) untuk punya snippet yang bisa ditempel sama sekali; masuk ke issue penutup phase.

---

## #88 — Halaman embed (iframe) + CSP per-form

Kemampuan baru bagi `crm_be` — hari ini backend tidak pernah menyajikan HTML sama sekali (nol
`html/template`, nol `http.FileServer`, nol berkas `.html`). Karena itu issue tersendiri, bukan
tempelan pada #87. Prasyarat #85 sudah terpenuhi.

### Kontradiksi TD ditemukan sebelum menulis kode — diajukan, bukan diputuskan sepihak

Issue #88 AC eksplisit tulis *"Tanpa CAPTCHA, halaman murni HTML — nol JavaScript"*. Tapi D8 (auto-
resize, ditambahkan belakangan lewat PR #92) minta `ResizeObserver`+`postMessage` di halaman embed
**selalu**, tidak bersyarat pada CAPTCHA — TD §7's baris "JavaScript hanya bila turnstile" luput
diperbarui saat D8 masuk. Diajukan dua opsi ke pemilik produk: **D8 menang** dipilih — script auto-
resize (kecil, cuma miliknya sendiri) tetap ada tanpa CAPTCHA; yang tetap benar dari niat AC aslinya
adalah *tanpa CAPTCHA, tidak ada script pihak ketiga*, bukan *tidak ada JavaScript sama sekali*. TD §7
diperbaiki di tempat sebelum implementasi dimulai — lihat baris "JavaScript" di tabelnya, sekarang
mencatat sejarah koreksi ini secara eksplisit.

Konsekuensi kedua yang ikut ditemukan: TD §7's literal CSP (`default-src 'none'; style-src
'unsafe-inline'; form-action 'self'; frame-ancestors <allowlist>`) **tidak punya `script-src` sama
sekali** — dengan `default-src 'none'`, itu berarti SETIAP script (inline auto-resize milik sendiri,
maupun Turnstile) akan diblokir browser sendiri, bukan cuma "sebaiknya dihindari". Diperbaiki dengan
menambah `script-src 'nonce-<per-request>'` (plus `https://challenges.cloudflare.com` saat turnstile
aktif) — nonce dipilih daripada `'unsafe-inline'` karena tidak banyak kerja ekstra dan mengurangi
blast radius XSS secara nyata: satu-satunya script inline di halaman ini adalah auto-resize milik
sendiri, jadi nonce membuktikan itu satu-satunya yang boleh jalan, bukan "semua inline script boleh".

### Nonce base64 std vs URL-safe — celah nyata ditemukan oleh test sendiri

Draf pertama `generateNonce` memakai `base64.StdEncoding` — alfabetnya memuat `+`/`/`/`=`. Test
`TestHandler_Embed_NonceMatchesCSPAndScriptTags` (membandingkan nonce di header CSP dengan nonce di
atribut `<script nonce="...">`) merah: `html/template` meng-HTML-escape atribut, mengubah `+` jadi
`&#43;` di badan HTML, sementara header CSP tetap memuat `+` mentah. **Bukan bug fungsional** — browser
sungguhan mendekode entity HTML kembali ke `+` sebelum mencocokkan nonce, jadi ini akan tetap bekerja
di browser nyata. Diperbaiki tetap: `base64.RawURLEncoding` (alfabet `-`/`_`, tanpa `=`) tidak butuh
escaping apa pun, menghilangkan kerapuhan itu sepenuhnya alih-alih bergantung pada browser mendekode
dengan benar.

### `internal/form/template.go` — satu berkas `//go:embed`, mengikuti pola `migrations/embed.go`

`go:embed` tidak bisa menjangkau ke luar direktori berkas `.go`-nya — karena itu berkas ini di
`internal/form/` (satu level di atas `template/`), bukan di dalam `template/` itu sendiri, persis
alasan yang sudah ditulis `migrations/embed.go` untuk `*.sql`. Dua aset: `template/form.gohtml`
(`html/template`, **bukan** `text/template` — TD §7's satu-satunya jalur XSS di phase ini, escaping
kontekstual otomatis) dan `template/embed.js` (byte mentah, disajikan apa adanya, bukan template sama
sekali — D8's companion script tidak memuat data per-form apa pun).

### `AllowedOriginsJSON template.JS` — cara aman menyuntik array JSON ke `<script>`

`{{.AllowedOriginsJSON}}` dirender TANPA tanda kutip di sekitarnya (`var allowedOrigins =
{{.AllowedOriginsJSON}};`) — kalau nilainya `string` biasa, `html/template`'s escaper konteks-JS akan
memperlakukannya sebagai STRING LITERAL, bukan ekspresi array. `template.JS` memberi tahu
`html/template` nilainya sudah aman apa adanya, dilewati tanpa escaping ulang — idiom resminya untuk
kasus ini, benar dipakai di sini karena isinya sudah melalui `encoding/json.Marshal` di sisi Go
(`forms.allowed_origins`, data yang diset Owner/Admin lewat PATCH terautentikasi, bukan input
pengunjung anonim), bukan string bebas yang belum diverifikasi bentuknya.

### `orderedEnabledFields` — `AllFieldKeys`, bukan `range fields`

Field dirender dengan urutan yang SAMA setiap render — `range` langsung atas `Fields` (map) akan
mengacak urutan field antar request, karena Go tidak menjamin urutan iterasi map. `AllFieldKeys`
(urutan tetap dari `entity.go`) dipakai sebagai sumber urutan; `Fields`-nya sendiri hanya dikonsultasi
per key. Dibuktikan test `TestHandler_Embed_FieldOrderIsDeterministic` — 5 render berturut-turut,
urutan tetap sama.

### `embed.js` — `EMBED_ORIGIN` dari `document.currentScript.src`, iframe dicocokkan lewat `e.source`

TD §7's snippet literal untuk `embed.js` menyebut `EMBED_ORIGIN` dan `frame` sebagai variabel yang
sudah tersedia tanpa menjelaskan dari mana asalnya — dua celah kecil diselesaikan saat implementasi:
`EMBED_ORIGIN` diturunkan dari `document.currentScript.src` (origin skrip ini sendiri dimuat, valid
selama eksekusi sinkron awal skrip termasuk untuk skrip `async`); `frame` yang tepat dicari lewat
`document.querySelectorAll("iframe[data-jualin-form]")` dan dicocokkan `contentWindow === e.source` —
bukan asumsi "iframe pertama di halaman", supaya halaman dengan lebih dari satu form tertanam tetap
me-resize iframe yang benar.

### Respons embed — `httpx.WriteError` tetap dipakai untuk 404, bukan halaman 404 HTML terpisah

`public_key` tak dikenal mengembalikan envelope JSON error yang sama dengan endpoint API lain, bukan
halaman HTML 404 kustom — meski route ini menyajikan HTML untuk kasus sukses. Tautan embed yang rusak
adalah kasus tepi jarang; membangun bentuk 404 kedua khusus HTML untuk satu kasus itu saja tidak
sepadan (Aturan #27).

### Tidak ada kasus isolasi tenant baru di `tenant_isolation_test.go`

Berbeda dari #87 (yang menambah kasus baru untuk submit), #88 sengaja **tidak** menambah kasus baru —
scoping organization halaman embed 100% diwarisi dari `Usecase.ResolvePublicKey` yang sudah ada dan
sudah diuji isolasinya di #87 sendiri. Tidak ada jalur kode baru di #88 yang bisa membocorkan data
lintas organization; menambah test isolasi di sini hanya akan mengulang apa yang #87 sudah buktikan.

### Test

- `internal/form/handler_test.go` — 17 test embed baru: sukses 200 + `Content-Type`, 404 kunci tak
  dikenal, 404 form terhapus (identik dengan kunci tak dikenal), label `<script>` di-escape (AC
  literal), `frame-ancestors` dari `allowed_origins` DAN kosong→`'none'`, `X-Frame-Options` tidak
  pernah dikirim, `Cache-Control: no-store`, tanpa CAPTCHA tidak ada script Cloudflare (tapi auto-
  resize tetap ada), CAPTCHA aktif merender widget+script dengan site key benar, nonce CSP cocok
  persis dengan setiap atribut `nonce` di badan HTML, `allowedOrigins` JSON cocok dengan
  `forms.allowed_origins`, field disabled tidak dirender, urutan field deterministik lintas 5 render,
  `GET /embed.js` dengan `Cache-Control` benar + smoke-check empat pengaman D8 ada di byte yang
  sungguhan disajikan, `form_token` yang dirender adalah token time-trap sungguhan yang lolos
  `formtoken.Verify`.
- Sintaks JS diverifikasi lewat `node --check` — `embed.js` dan skrip auto-resize yang diekstrak dari
  `form.gohtml` (dengan placeholder templat diganti nilai dummy) — bukan dipercaya dari mata telanjang.

### Verifikasi manual end-to-end — sungguhan, `docker compose` + `curl` + browser nyata

`docker compose up -d --build` (boot sukses), form dibuat dengan nama DAN label field berisi
`<script>alert(1)</script>` lewat dashboard API sungguhan:

1. `GET /embed/{public_key}` sungguhan → `200`, header `Cache-Control: no-store` asli, `Content-
   Security-Policy` dengan `frame-ancestors`/`script-src` nonce asli, **tidak ada** header
   `X-Frame-Options` sama sekali.
2. Judul dan label yang berisi `<script>alert(1)</script>` muncul di HTML sebagai `&lt;script&gt;...`
   — bukan tag hidup — dibuktikan dari body response sungguhan, bukan dari baca kode.
3. Form dengan `allowed_origins` kosong → `frame-ancestors 'none'` sungguhan.
4. `public_key` tak dikenal → `404` sungguhan.
5. `GET /embed.js` → `200`, `Cache-Control: public, max-age=3600` asli.
6. **Verifikasi browser sungguhan untuk AC "bisa di-iframe dari allowlist, tidak bisa dari luar
   allowlist"**: dua server statis lokal dijalankan di port berbeda (`:9001` = origin yang dimasukkan
   ke `allowed_origins` form, `:9002` = origin di luar allowlist), masing-masing menyematkan iframe ke
   halaman embed yang sama. Kedua tab dibuka di browser sungguhan pemilik produk untuk konfirmasi
   visual — pemeriksaan header CSP di atas sudah membuktikan **apa yang server kirim**, tapi AC ini
   secara eksplisit minta "diverifikasi dari browser sungguhan, bukan dari membaca header", jadi
   konfirmasi visual tetap dicatat sebagai langkah terpisah, bukan diasumsikan otomatis lolos karena
   header-nya benar.

### Batas issue ini

Tidak ada: endpoint submit (#87, sudah ada), UI pengelolaan form (#89). Prosedur manual formal
"tempel snippet ke halaman HTML kosong" (TD §12) tetap masuk `docs/testing/flow/` di issue penutup
(#89) — verifikasi browser di atas memakai iframe manual sebagai pengganti sementara untuk AC #88
sendiri, bukan menggantikan prosedur resmi yang direncanakan.

## #89 — Dashboard: manajemen form + snippet embed — penutup Phase 6

Layar terakhir Phase 6, murni `crm_dashboard`. Backend sudah lengkap sejak #85 (CRUD) dan #88 (embed);
tidak ada perubahan Go. `NAV_ITEMS`/`pageTitle` untuk `/connect` sudah beres sejak #86 — #89 tidak
menyentuh `nav.ts`.

### Editor jadi route `/connect/form/[id]`, bukan dialog

Keputusan yang diambil sendiri (dilaporkan ke pemilik produk sebelum implementasi): editor per-form
adalah **route detail** seperti `/leads/[id]` dan `/customers/[id]`, bukan dialog seperti
`CreateAPIKeyDialog`. Isinya besar — nama, 6×(toggle enabled + toggle required + input label),
allowlist domain, dua kotak snippet dengan tombol salin, tombol nonaktifkan — dan dialog setinggi itu
harus scroll. TD §10 hanya menyebut satu route (`/connect/form`, "daftar & pengelolaan"); menambah
`[id]` konsisten dengan pola route detail yang sudah ada dua kali di codebase ini, bukan mekanisme
baru.

### Forms **tidak** punya optimistic locking — beda dari lead/task

`formJSON` tidak membawa `version`; `form.Usecase.Update` last-write-wins. Jadi editor **tidak**
membawa `version` bolak-balik dan **tidak** ada jalur `409 version_conflict` (Aturan #35 tidak berlaku
untuk `forms`). Dicatat eksplisit di doc comment `forms.ts` supaya session berikutnya tidak mencari
penanganan konflik yang memang tidak ada.

### `firstFieldConfigError` — cermin `Fields.Validate` Go, hanya untuk pesan inline

Backend menolak kombinasi tidak koheren (`required && !enabled`; `enabled && label==""`) dengan
`validation_failed` generik yang tidak menyebut field mana. `firstFieldConfigError` (`forms.ts`,
diuji terpisah di `forms.test.ts`) mengulang aturan yang sama **hanya** supaya editor bisa menaruh
pesan merah di bawah field yang salah, bukan sebagai banner buta. Backend tetap sumber kebenaran —
kalau helper ini meleset, backend tetap menolak. Toggle di UI juga menjaga kedua invariant tetap
koheren saat diklik (mematikan field ikut mematikan "wajib"; menandai "wajib" ikut menyalakan field).

### `NEXT_PUBLIC_EMBED_BASE_URL` — env var terpisah, bukan pakai ulang API base

Snippet embed menunjuk ke `GET /embed/{public_key}` + `GET /embed.js`, yang hari ini disajikan
`crm_be` sendiri (`http://localhost:8080`). TD §13 menyebut `EMBED_BASE_URL` sebagai "dipakai
dashboard membangun snippet". Dibuat env var dashboard sendiri (`NEXT_PUBLIC_EMBED_BASE_URL`, fallback
ke `NEXT_PUBLIC_API_BASE_URL`) daripada memakai ulang API base — ADR-005 D1 mewajibkan halaman embed
pindah ke hostname berbeda dari dashboard saat deployment; var terpisah membuat perpindahan itu
perubahan config, bukan perubahan kode. Dibaca **saat pemanggilan** (`embedBaseUrl()`), bukan saat
module load, supaya trailing-slash tertangani dan test bisa `vi.stubEnv` tanpa main import order.
`||` bukan `??` di rantai fallback — string kosong (bisa terjadi di sebagian setup deploy) harus jatuh
ke kandidat berikutnya, bukan dianggap nilai sah.

### `escapeHtmlAttribute` — nama form masuk ke `title="..."`

Nama form adalah teks yang diketik pelanggan; ia dirender ke atribut HTML `title=""` di snippet.
Di-escape (`&`, `<`, `>`, `"`) supaya nama ber-`"` tidak memecah tag — cermin apa yang `html/template`
lakukan server-side untuk label field di halaman embed (TD §16 risiko XSS). Varian JSX melewatkannya
sebagai string literal JS (`JSON.stringify`) karena React yang meng-escape saat render.

### Tiga varian snippet, satu kalimat penjelas

`autoResizeSnippet` (dianjurkan, + `embed.js`), `fixedHeightSnippet` (tanpa script pihak ketiga),
`jsxSnippet` (`style={{ border: 0 }}`, `height={620}`). Editor menampilkan dua yang pertama sebagai
kotak salin dan JSX di balik toggle — TD §10 minta "menjelaskan bedanya dalam satu kalimat, bukan
menyodorkan dua kotak tanpa keterangan". `height="620"` tetap ada di varian auto sebagai nilai awal
sebelum script sempat jalan (mencegah kedip).

### Gerbang role di level fetch

`canManageForms` (`form-permissions.ts`, di sebelah `canManageAPIKeys`) — Owner/Admin, cermin persis
matriks `authz.go` untuk `ActionForm*`. Gerbang ada **di atas** `useEffect` yang memanggil
`listForms`/`getForm` di kedua layar (`/connect/form` dan `/connect/form/[id]`), jadi Manager/Employee
yang mengetik URL langsung tidak menghasilkan satu pun panggilan `/v1/forms` — pola sama seperti
`canManageAPIKeys` di #48.

### Cakupan doc penutup phase — dicek satu per satu

- `api.md`, `authentication.md`, `authorization.md`, `multi-tenancy.md` — **sudah lengkap** untuk
  Phase 6, ditutup bertahap oleh #85 (matriks `form.*`, pengecualian unik keempat)/#87 (tiga error
  code, bab submit publik, backfill baris keempat `multi-tenancy.md`)/#88 (bab halaman embed). Diperiksa
  satu per satu di #89; tidak ada gap tersisa. Satu perbaikan kecil: `api.md` masih menyebut
  `/settings/api-keys/docs` (path lama sebelum #86) → diperbaiki ke `/connect/api/docs`.
- `docs/testing/flow/08-formulir-embed.md` — **baru**. Prosedur "tempel snippet ke halaman HTML
  kosong lalu isi dari browser" (AC #1 & #10). Sekalian: `02`/`05`/`06` masih menunjuk
  `/settings/api-keys` (debt #86, redirect tetap jalan tapi menyesatkan QA) → diperbaiki ke
  `/connect/api` karena berkasnya di folder yang sama yang sedang disunting (Aturan #30: dicatat, bukan
  diperbaiki diam-diam).
- `STATUS.md` — baris Selesai #89, Phase 6 di *Progress per Phase* jadi ✅, kunci Turnstile tetap di
  *Punya Lead Time* (sudah ada sejak #87; keterangannya diperbarui — tidak lagi menyebut #88/#89
  sebagai "tidak diblokir" karena keduanya sudah selesai).

### Test

- `form-permissions.test.ts` (2) — gerbang Owner/Admin.
- `form-snippet.test.ts` (14) — resolusi `embedBaseUrl` (prefer var khusus, fallback, strip trailing
  slash), `escapeHtmlAttribute`, ketiga varian snippet (URL + public_key benar, `data-jualin-form`,
  script pendamping, `height` awal, nama ter-escape di `title`, JSX pakai objek style).
- `forms.test.ts` (5) — `firstFieldConfigError` mencerminkan `Fields.Validate` (wajib-tapi-nonaktif,
  aktif-label-kosong, urutan kanonik).
- Total dashboard: 106 test lolos (85 → 106). Tidak ada test komponen React (`vitest.config.mts`
  sengaja hanya `.test.ts`).

### Verifikasi manual end-to-end — `docker compose` + browser sungguhan

`docker compose up -d --build`, register org baru + verifikasi lewat Mailpit + login sebagai Owner,
lalu:

1. **Buat form** lewat API dashboard sungguhan → 6 field ter-seed dengan label Indonesia default.
2. **PATCH** nama + label (`email` → "Alamat Email", `product` diaktifkan + label diubah) + `allowed_origins: ["http://localhost:9099"]` → response mencerminkan semuanya.
3. Snippet dari `autoResizeSnippet()` sungguhan (`NEXT_PUBLIC_EMBED_BASE_URL=http://localhost:8080`) ditempel ke `index.html` kosong, disajikan lewat `python3 -m http.server 9099`.
4. `GET /embed/{public_key}` (dengan `Referer` dari `:9099`) → 200, label yang diubah muncul, field `company` (nonaktif) **tidak** dirender, `form_token` time-trap asli ada di HTML.
5. **Submit dari browser origin** (`Origin: http://localhost:9099`, `application/x-www-form-urlencoded`, tunggu >2 detik) → `201`, `X-RateLimit-*` per-form asli.
6. `GET /v1/leads?source=form` → lead baru dengan `source: "form"`, `source_form_id` cocok, `assigned_to_membership_id`/`created_by_membership_id` **kosong**, `phone_e164` ter-normalisasi, catatan = isi field message.
7. `submit_count` form → `1`.
8. Submit dari `Origin: http://evil.example` → `403`.
9. **Nonaktifkan**: `DELETE` → `204`; `DELETE` kedua → `404`; `GET /embed/{public_key}` setelahnya → `404`; `GET /v1/forms` → daftar kosong; lead dari langkah 6 **tetap ada**.

`npm run typecheck && lint && test && build` bersih.

### Batas issue ini

Verifikasi anti-spam sungguhan terhadap Cloudflare Turnstile **tetap tertunda** (`CAPTCHA_PROVIDER=none`
sepanjang phase — akun Cloudflare belum diurus, `STATUS.md` *Punya Lead Time*). `TurnstileVerifier`
sendiri sudah diuji terhadap server palsu sejak #87; yang belum adalah round-trip ke Cloudflare asli.
Tidak memblokir penutupan Phase 6 — semua fungsi lain jalan penuh tanpanya.
