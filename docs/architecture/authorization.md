# Authorization

> Sumber: `docs/brainstorming/architecture_product_review.md` §6.2 ("Authorization — matriks
> permission") · `docs/phases/01-auth-organization/td.md` §9 · Issue #11.
> Dibuat saat issue #11 dikerjakan, sesuai TD §16.

**Koreksi rujukan:** issue #11 dan TD §9 menyebut "freeze bagian 6.2" sebagai sumber matriks ini.
Bagian 6.2 di `freeze.md` sebenarnya adalah kerangka `docs/STATUS.md` — tidak terkait. Matriks yang
dimaksud ada di `docs/brainstorming/architecture_product_review.md` §6.2, dan itulah yang dipakai di
sini. Dicatat sebagai penyimpangan dokumentasi (Aturan #30), bukan diam-diam diperbaiki di sumbernya.

---

## Otorisasi ditegakkan di service layer, bukan UI

UI yang menyembunyikan tombol bukan otorisasi. Setiap usecase yang mengubah atau membaca resource
sensitif memanggil `authz.Require` sebagai baris pertama, sebelum menyentuh repository apapun.

```go
func (u *Usecase) Deactivate(ctx context.Context, t tenant.Context, targetID uuid.UUID) error {
    if err := authz.Require(t, authz.ActionMembershipDeactivate); err != nil {
        return err
    }
    // ...
}
```

---

## Matriks (Phase 1)

Matriks lengkap ada di `architecture_product_review.md` §6.2 dan mencakup resource yang saat itu belum
ada (Lead, Customer, Deal, dst — kosong sampai tabelnya ada). Lead/Activity/Task/Customer terwujud di
Phase 2, lihat bagian "Matriks (Phase 2)" di bawah; Deal masih menunggu (pasca-Phase 5). Baris yang
**nyata di Phase 1**:

| Resource | Owner | Admin | Manager | Employee |
|---|---|---|---|---|
| Membership: undang / hapus / ubah role | CRUD | CRU¹ | Read | — |

¹ Admin tidak boleh mengubah/menghapus Owner, dan tidak boleh mengangkat siapapun menjadi Owner.

Diterjemahkan ke `internal/shared/authz`'s `Action` enum:

| Action | Owner | Admin | Manager | Employee |
|---|---|---|---|---|
| `membership.list` | ✅ | ✅ | ✅ | — |
| `membership.update_role` | ✅ | ✅¹ | — | — |
| `membership.deactivate` | ✅ | ✅¹ | — | — |
| `invitation.create` | ✅ | ✅ | — | — |
| `invitation.list` | ✅ | ✅ | — | — |
| `invitation.revoke` | ✅ | ✅ | — | — |

Ini adalah **satu-satunya** yang `authz.Require` bisa jawab — "apakah role ini boleh melakukan kelas
aksi ini sama sekali". Baris ¹ (Admin dibatasi terhadap Owner) **tidak** ditegakkan di sini — lihat
bagian berikutnya.

---

## Matriks (Phase 2) — realisasi Aturan #1

Ditambahkan issue #20–#23. Baris `Lead`/`Activity`/`Task`/`Customer` yang disebut "belum ada" di bagian
Phase 1 sekarang **nyata**:

| Resource | Owner | Admin | Manager | Employee |
|---|---|---|---|---|
| Lead: buat / baca / ubah | CRU | CRU | CRU | RU¹ |
| Lead: hapus | ✅ | ✅ | — | — |
| Lead: assign | ✅ | ✅ | ✅ | — |
| Lead: convert ke Customer | ✅ | ✅ | — | — |
| Activity: buat (tipe pengguna) / baca | CR | CR | CR | CR¹ |
| Task: buat / ubah / selesaikan | CRU | CRU | CRU | CRU¹ |
| Task: hapus | ✅ | ✅ | ✅ | — |
| Customer: baca | ✅ | ✅ | ✅ | ✅¹ |
| Customer: ubah / hapus | ✅ | ✅ | — | — |

¹ Dibatasi repository ke lead yang di-assign kepadanya, dan turunannya: activity, task, customer dari
lead itu (`converted_from_lead_id`-nya). Lihat bagian berikutnya.

Diterjemahkan ke `internal/shared/authz`'s `Action` enum:

| Action | Owner | Admin | Manager | Employee |
|---|---|---|---|---|
| `lead.create` | ✅ | ✅ | ✅ | — |
| `lead.read` | ✅ | ✅ | ✅ | ✅¹ |
| `lead.update` | ✅ | ✅ | ✅ | ✅¹ |
| `lead.delete` | ✅ | ✅ | — | — |
| `lead.assign` | ✅ | ✅ | ✅ | — |
| `lead.convert` | ✅ | ✅ | — | — |
| `activity.create` | ✅ | ✅ | ✅ | ✅¹ |
| `activity.list` | ✅ | ✅ | ✅ | ✅¹ |
| `task.create` | ✅ | ✅ | ✅ | ✅¹ |
| `task.read` | ✅ | ✅ | ✅ | ✅¹ |
| `task.update` | ✅ | ✅ | ✅ | ✅¹ |
| `task.complete` | ✅ | ✅ | ✅ | ✅¹ |
| `task.delete` | ✅ | ✅ | ✅ | — |
| `customer.read` | ✅ | ✅ | ✅ | ✅¹ |
| `customer.update` | ✅ | ✅ | — | — |
| `customer.delete` | ✅ | ✅ | — | — |

Dua asimetri yang layak dicatat, bukan kebetulan: `lead.assign` memberi Manager akses yang `lead.delete`
tidak (Manager bisa memindahkan kepemilikan lead tapi tidak menghapusnya); `customer.update`/`delete`
lebih ketat daripada `lead.update`/`delete` yang setara — Manager kehilangan akses tulis begitu sebuah
lead selesai dikonversi.

---

## Matriks (Phase 3) — issue #30

| Action | Owner | Admin | Manager | Employee |
|---|---|---|---|---|
| `metrics.read` | ✅ | ✅ | ✅ | — |

Employee tidak dapat: dashboard bukan alatnya (Employee dapat mobile di Phase 5), dan agregat lintas
organization adalah informasi manajemen. Konsekuensinya `internal/metrics` **tidak** punya cabang
`isEmployee` di query-nya sama sekali — tidak seperti `lead`/`task`/`customer`, Employee tidak pernah
sampai ke repository ini (`docs/phases/03-owner-dashboard/td.md` §2.4).

---

## Matriks (Phase 4) — issue #46

| Action | Owner | Admin | Manager | Employee |
|---|---|---|---|---|
| `api_key.create` | ✅ | ✅ | — | — |
| `api_key.list` | ✅ | ✅ | — | — |
| `api_key.revoke` | ✅ | ✅ | — | — |

Manager tidak dapat **sama sekali** — bukan read-only seperti `membership.list` yang Manager punya.
Kredensial yang bisa memasukkan lead ke organization, dan daftar integrasi mana yang hidup, bukan
informasi level-baca biasa (TD phase 4 §9).

---

## Matriks (Phase 5) — issue #68/#73

| Action | Owner | Admin | Manager | Employee |
|---|---|---|---|---|
| `device_token.register` | ✅ | ✅ | ✅ | ✅ |
| `device_token.delete` | ✅ | ✅ | ✅ | ✅ |

Berbeda dari `api_key.*` di atas — sengaja terbuka untuk **semua** role, bukan Owner/Admin saja.
Mendaftarkan device token berarti mendaftarkan **HP pemanggil sendiri** untuk push, bukan kapabilitas
yang menjangkau data orang lain; Employee sudah pasti memakainya (mobile, Phase 5), tapi tidak ada
alasan keamanan menutup pintu bagi Owner/Admin/Manager yang kelak juga memasang aplikasinya
(`internal/shared/authz/authz.go`'s doc comment pada `ActionDeviceTokenRegister`).

---

## Matriks (Phase 6) — issue #85

| Action | Owner | Admin | Manager | Employee |
|---|---|---|---|---|
| `form.create` | ✅ | ✅ | — | — |
| `form.list` | ✅ | ✅ | — | — |
| `form.read` | ✅ | ✅ | — | — |
| `form.update` | ✅ | ✅ | — | — |
| `form.delete` | ✅ | ✅ | — | — |

Bentuk sama persis dengan `api_key.*` (Phase 4) — Manager tidak dapat **sama sekali**, bukan read-only.
`public_key` adalah kredensial yang bisa memasukkan lead ke organization, sekelas API key dalam hal itu
(`internal/shared/authz/authz.go`'s doc comment pada `ActionFormCreate`).

---

## Matriks (Phase 7) — issue #100

| Action | Owner | Admin | Manager | Employee |
|---|---|---|---|---|
| `webhook.create` | ✅ | ✅ | — | — |
| `webhook.list` | ✅ | ✅ | — | — |
| `webhook.read` | ✅ | ✅ | — | — |
| `webhook.update` | ✅ | ✅ | — | — |
| `webhook.delete` | ✅ | ✅ | — | — |

Bentuk sama persis dengan `api_key.*` (Phase 4) dan `form.*` (Phase 6) — Manager dan Employee tidak
dapat **sama sekali**, bukan read-only. Endpoint webhook adalah instruksi tetap untuk mengalirkan
data lead organization ke alamat yang dipilih seseorang; siapa pun yang bisa menambahkannya bisa
diam-diam mengarahkan setiap lead baru ke tempat lain (`internal/shared/authz/authz.go`'s doc comment
pada `ActionWebhookCreate`).

`webhook.read` juga menggerbangi `GET /v1/webhook-endpoints/:id/deliveries` (riwayat pengiriman) dan
`webhook.update` menggerbangi `POST /v1/webhook-deliveries/:id/retry` (kirim ulang manual) — tidak ada
`Action` terpisah untuk keduanya.

Kelima `Action` ditambahkan ke `authz_test.go`'s `allActions` **dan** tabel per-role di PR yang sama
yang mendefinisikannya (#100) — celah yang sama sudah terjadi dua kali (`ActionAPIKey*` #46,
`ActionForm*` #85, keduanya di-backfill belakangan) dan tidak terulang ketiga kalinya. Tidak ada peta
gaya `apiKeyScopeFor`/`publicFormAllows` untuk webhook: endpoint webhook tidak pernah memanggil balik
ke sistem (ia hanya menerima pengiriman), jadi tidak ada principal "webhook" — kelima `Action` itu
seluruh permukaannya, dan tabel-atas-seluruh-`Action` yang sudah ada membuktikan principal
`api_key`/`public_form` tidak dapat satu pun tanpa baris baru.

---

## Otorisasi berbasis scope — principal tanpa role (Phase 4, issue #47)

Matriks role di atas menjawab pertanyaan **"role apa boleh action apa"** — tapi principal `api_key`
tidak punya role sama sekali (`tenant.Context.Role` kosong untuknya). Menjawab "apa yang boleh dilakukan
API key" **bukan** perluasan dari matriks role — itu pertanyaan yang berbeda, dijawab peta terpisah:

```go
// internal/shared/authz/authz.go
var apiKeyScopeFor = map[Action]string{
    ActionLeadCreate: "leads:write",
}

func Require(t tenant.Context, action Action) error {
    if t.PrincipalType == tenant.PrincipalAPIKey {
        scope, ok := apiKeyScopeFor[action]
        if !ok || !slices.Contains(t.Scopes, scope) {
            return InsufficientScopeError()   // 403 insufficient_scope
        }
        return nil
    }
    if permissions[t.Role][action] {          // jalur role, tidak berubah
        return nil
    }
    return forbiddenError()                   // 403 forbidden
}
```

`Require` bercabang di `t.PrincipalType` **sebelum** menyentuh `permissions[t.Role]` sama sekali — API
key tidak pernah jatuh ke jalur role dengan `Role` kosong dan mendapat jawaban kebetulan (baik
kebetulan diizinkan maupun kebetulan ditolak); ia dicek terhadap peta scope-nya sendiri, titik.

**Kenapa peta terpisah, bukan `Role("api_key")` kelima.** Menambahkan API key sebagai baris kelima di
matriks role akan membuatnya terlihat seperti role — dan dokumen ini sudah menyatakan role adalah enum
tertutup berisi empat (freeze 1.3). Peta terpisah membuat "apa yang bisa dilakukan API key" dijawab
dengan membaca **satu baris**, bukan menyaring lima kolom di setiap tabel matriks di atas.

**Kenapa peta ini isinya cuma satu baris.** `apiKeyScopeFor` hanya berisi `lead.create → leads:write` —
setiap `Action` lain (termasuk `api_key.create/list/revoke` di atas) ditolak karena **tidak ada** di
peta ini, bukan karena ada baris eksplisit yang menolaknya. Menambah kemampuan API key di masa depan
berarti menambah baris di sini, bukan mengaudit ulang setiap handler yang ada (freeze 5.1, Aturan #24).
Dikunci test yang mengulang **seluruh** `Action` yang terdaftar di paket ini (26 saat ini) terhadap
principal `api_key` — bukan daftar tulis tangan, supaya `Action` baru phase berikutnya otomatis ikut
tertutup tanpa ada yang perlu mengingatnya.

**Matriks role di bagian atas dokumen ini karena itu tidak lagi menggambarkan seluruh sistem
otorisasi** — ia menjawab untuk `PrincipalUser`; bagian ini menjawab untuk `PrincipalAPIKey`. Keduanya
harus dibaca bersama untuk tahu "siapa boleh apa" secara lengkap.

---

## Otorisasi tanpa role — principal form (Phase 6, issue #87)

`Require` bercabang **ketiga kalinya** — `PrincipalPublicForm`, bentuk yang sama persis dengan
`PrincipalAPIKey` di atas, bukan digabung ke peta yang sama:

```go
// internal/shared/authz/authz.go
var publicFormAllows = map[Action]bool{
    ActionLeadCreate: true,
}

func Require(t tenant.Context, action Action) error {
    if t.PrincipalType == tenant.PrincipalAPIKey { /* ... */ }
    if t.PrincipalType == tenant.PrincipalPublicForm {
        if !publicFormAllows[action] {
            return forbiddenError()   // 403 forbidden — BUKAN insufficient_scope
        }
        return nil
    }
    if permissions[t.Role][action] { /* ... */ }
}
```

**Kenapa peta terpisah dari `apiKeyScopeFor`, bukan digabung.** Keduanya menjawab pertanyaan yang
berbeda: API key punya *scope yang bisa dipilih pelanggan* (hari ini cuma satu, tapi bentuknya
extensible); form punya *satu kemampuan tetap* yang tidak pernah bertambah tanpa ADR-005 berubah.
Menggabungkannya berarti suatu hari seseorang menambah scope ke API key dan tanpa sadar memberikannya
juga ke setiap form yang terpasang di situs pelanggan.

**Kenapa `forbiddenError()`, bukan `InsufficientScopeError()`.** `public_key` tidak punya konsep scope
sama sekali — ia tidak seperti API key yang *bisa* diberi scope lebih luas tapi tidak diberi. Kode
`forbidden` (bukan `insufficient_scope`) mencerminkan itu: bukan "kredensial ini kurang izin", tapi
"kredensial jenis ini tidak pernah punya jalan menuju aksi ini sama sekali".

Dikunci test yang sama bentuknya dengan `apiKeyScopeFor` — mengulang **seluruh** `Action` yang
terdaftar di paket ini terhadap principal `public_form`, bukan daftar tulis tangan
(`TestRequire_PublicFormPrincipal_OnlyLeadCreateAllowed`).

---

## Dua pertanyaan berbeda yang harus dilewati sebuah `POST` kanal (Phase 8, issue #112–#113)

Sejak Phase 8, `Usecase.Create` di `apikey`, `form`, dan `webhook` masing-masing menjawab **dua**
pertanyaan yang bentuknya sama sekali berbeda, berurutan:

```go
func (u *Usecase) Create(ctx context.Context, t tenant.Context, in CreateInput) (...) {
    if err := authz.Require(t, authz.ActionWebhookCreate); err != nil {
        return err                        // 1. "role ini boleh?"  → 403 forbidden
    }
    if err := u.plan.RequireChannel(ctx, t, "webhook"); err != nil {
        return err                        // 2. "paket ini membuka?" → 403 plan_upgrade_required
    }
    // ...
}
```

| Pertanyaan | Dijawab oleh | Tabelnya |
|---|---|---|
| *"Role ini boleh melakukan aksi ini?"* | `internal/shared/authz` — matriks di atas | Statis, per-role, milik seluruh produk |
| *"Paket organization ini membuka kanal ini?"* | `internal/subscription.RequireChannel` | `planChannels`, milik satu organization, bisa berubah kapan saja |

**Kenapa urutannya mengikat, dan tidak boleh dibalik.** Manager yang memanggil `POST /v1/forms` harus
menerima `403 forbidden` — bukan `plan_upgrade_required`. Kode kedua membocorkan keadaan paket
organization kepada orang yang **memang tidak berhak** mengelola kanal itu sama sekali, apa pun
paketnya. Semangatnya sama dengan Aturan #6 (404 alih-alih 403 untuk tenant lain): jangan menjawab
pertanyaan yang penanyanya tidak berhak ajukan. Dibuktikan test, bukan komentar di kode — fake
`PlanGate` yang menolak **sekaligus** dengan role yang authz tolak harus menghasilkan `forbidden`,
karena satu-satunya cara mengamati urutan dari luar adalah lewat kasus di mana keduanya menolak.

**Kenapa paket bukan baris tambahan di matriks `Action` di atas.** Paket adalah dimensi **kedua**,
bukan perluasan dimensi role. Membaca paket sendiri (`plan.channels` di `GET /v1/me`) bukan aksi
ber-otorisasi terpisah — setiap principal user yang terautentikasi berhak melihat paketnya sendiri,
jadi tidak ada `Action` baru untuknya. Menambahkan paket sebagai kolom baru di `permissions[role][action]`
akan salah secara kategori: satu sel di matriks itu menjawab *"siapa boleh"*, sedangkan paket
menjawab *"organization mana yang boleh"* — pertanyaan yang berbeda sumbu.

`internal/apikey`, `internal/form`, dan `internal/webhook` masing-masing mendeklarasikan **`PlanGate`
miliknya sendiri** (ADR-011) — bentuk sama dengan `ActivityRecorder`/`LeadCreator`/`WebhookEnqueuer`:
```go
type PlanGate interface {
    RequireChannel(ctx context.Context, t tenant.Context, ch string) error
}
```
`ch` bertipe `string`, bukan `subscription.Channel` — mengimpor tipe itu berarti mengimpor
`internal/subscription`, yang justru dihindari pola ini. Nilainya (`"api_key"`, `"form"`, `"webhook"`)
adalah kontrak kabel dengan `subscription.Channels`, dikunci `cmd/api/plan_gate_test.go` — bukan
diasumsikan cocok.

---

## Empat aturan yang harus ditulis eksplisit

Sumber: `architecture_product_review.md` §6.2. Tiga dari empat bergantung pada relasi actor-vs-target,
bukan sekadar role actor — karena itu **tidak** bisa direpresentasikan sebagai map role→action, dan
ditegakkan langsung di `internal/membership.Usecase` (`UpdateRole`/`Deactivate`), di atas gerbang
`authz.Require` yang kasar.

| # | Aturan | Ditegakkan di | Kode error |
|---|---|---|---|
| 1 | Employee hanya melihat resource miliknya | Repository — `lead.FindByID`/`FindAllByOrg`/`Update`/`UpdateStatus`/`UpdateAssignment`/`Delete`, dan turunannya (`activity`, `task`, `customer`) lewat `EXISTS (SELECT 1 FROM leads ...)` terhadap lead yang sama — **terwujud sejak #20–#23** | 404 `not_found` (Aturan #6 — bukan 403) |
| 2 | Owner terakhir tidak bisa menghapus atau menurunkan dirinya sendiri | `membership.Usecase.Deactivate` — cek `CountActiveOwners` di dalam `Store.InTx` yang sama dengan write, mencegah race baca-lalu-tulis | 409 `last_owner_cannot_be_removed` |
| 3 | Tidak ada yang bisa mengubah role dirinya sendiri — **tanpa kecuali**, termasuk Owner | `membership.Usecase.UpdateRole` | 403 `forbidden` |
| 4 | Admin tidak bisa menyentuh Owner, tidak bisa mengangkat Owner | `membership.Usecase.UpdateRole` & `Deactivate` | 403 `forbidden` |

**Catatan Aturan #4:** Owner yang mengangkat membership lain menjadi Owner (co-owner) **diizinkan** —
matriks hanya membatasi Admin, dan schema tidak punya `UNIQUE` pada jumlah owner. Lihat
`internal/membership/usecase.go`'s `UpdateRole` untuk logikanya persis.

**Catatan Aturan #1:** belum berlaku secara harfiah di Phase 1 — `internal/membership`/`internal/invitation`
memberi Employee akses **nol** ke keduanya (bukan "hanya miliknya" — tidak ada sama sekali). Terwujud di
Phase 2: setiap query `lead` yang menyentuh satu baris menerapkan `(NOT $isEmployee OR
assigned_to_membership_id = $membershipID)`; `activity`/`task`/`customer` mewarisi batas yang sama lewat
`EXISTS` terhadap lead yang sama, bukan kolom kepemilikan masing-masing (`customer` bahkan tidak punya
kolom assignee — visibilitasnya murni lewat `converted_from_lead_id`). Dibuktikan di dua lapis: test per
domain (mis. `internal/lead/handler_test.go`'s `TestHandler_Get_Employee_OtherPersonsLead_Returns404`,
dan padanannya di `task`/`activity`/`customer`) dan harness isolasi tenant generik
(`cmd/api/tenant_isolation_test.go`) yang sejak #23 mencakup `lead`/`task`/`activity`/`customer` — lihat
`multi-tenancy.md` lapis 4 untuk kasus #1/#4 dan `docs/phases/02-crm-core/notes.md`'s `## #23` untuk
prosedur "harness terbukti bisa gagal".

---

## `internal/shared/authz` vs relationship rules — kenapa dipisah

`authz.Require` sengaja **tidak** tahu tentang "siapa target-nya" atau "apakah actor menargetkan
dirinya sendiri" — itu bukan pertanyaan tentang role, itu pertanyaan tentang state dua baris database.
Memaksakan keduanya ke dalam satu fungsi generik berarti `authz` harus mulai menerima parameter
resource-specific (`targetRole`, `isSelf`, dst.) yang berbeda-beda per domain — persis abstraksi
prematur yang Aturan #27 larang. Pemisahan ini bukan kompromi, melainkan batas yang disengaja: `authz`
menjawab pertanyaan yang sama di setiap domain masa depan (Lead, dst.); relationship rules selalu
spesifik pada domain yang mendefinisikannya.

---

## RBAC vs identitas — jangan tertukar

Cabang kedua penerimaan undangan (`invitation.Usecase.acceptExistingUser`, TD §6.1/B4) **bukan** kasus
RBAC — tidak ada pertanyaan "role apa yang boleh". Pertanyaannya adalah identitas: apakah user yang
login sekarang persis user yang alamat emailnya diundang. Ditegakkan lewat perbandingan `UserID`
langsung di usecase, bukan lewat `authz.Require`. Dua mekanisme berbeda untuk dua pertanyaan berbeda;
lihat `docs/architecture/authentication.md` untuk sesi/identitas dan dokumen ini untuk role.
