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
