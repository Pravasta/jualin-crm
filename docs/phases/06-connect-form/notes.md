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
