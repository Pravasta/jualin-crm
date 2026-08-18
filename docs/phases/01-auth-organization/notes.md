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
