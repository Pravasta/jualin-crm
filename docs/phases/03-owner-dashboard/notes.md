# Phase 3 — Owner Dashboard · Notes

One section per issue, appended as each is implemented.

---

## #30 — CORS + endpoint metrik

### Keputusan implementasi

- **`internal/metrics` benar-benar tidak punya `Store`/`InTx`**, sesuai TD §2 — hanya `entity.go` +
  `port.go` (interface `Repository`, tanpa `Store`) + `usecase.go` (menerima `Repository` langsung,
  bukan `Store`) + `repository_postgres.go` + `handler_http.go`. Tidak ada `cmd/api/metrics_store.go`:
  composition root memanggil `metrics.New(pool)` langsung karena `*pgxpool.Pool` sudah memenuhi
  `db.Querier`, sama seperti `customer.New(pool)` dipakai di test — tidak ada wrapper yang perlu ditulis
  untuk paket yang tidak pernah membuka transaksi.

- **Query dibangun dengan closure `arg()` yang sama seperti `internal/lead`/`internal/customer`**
  (`WHERE ... created_at >= $2`), bukan pola `$2::timestamptz IS NULL OR ...` yang sempat ada di draf
  TD §1.1's contoh kode — mengikuti konvensi yang sudah dipakai `lead.FindAllByOrg` untuk
  `created_from`/`created_to`, bukan menciptakan pola baru untuk hal yang sama.

- **`avg_response_seconds` memakai `avg()` Postgres apa adanya**, tanpa `FILTER` atau `COALESCE` untuk
  menangani lead yang belum tersentuh — `avg()` sudah mengabaikan `NULL` secara native, dan
  `MIN(activities.created_at WHERE type <> 'lead_created') - leads.created_at` menghasilkan `NULL`
  persis saat lead itu belum pernah disentuh activity non-`lead_created`. Ini yang membuat "dikecualikan
  dari rata-rata, bukan dihitung nol" (TD §2.3) benar tanpa cabang logic tambahan di Go maupun SQL.
  Dibuktikan lewat `TestRepository_Employees_AvgResponseSeconds_ExcludesUntouchedLeadsAndLeadCreated`,
  yang menguji **dua** klaim sekaligus: `lead_created` tidak ikut terhitung (lead yang sama juga punya
  activity `lead_created` di waktu pembuatan — kalau ini ikut terhitung, hasilnya akan ~0 detik, bukan
  ~10 menit), dan lead kedua yang sama sekali tidak tersentuh tidak menarik rata-rata turun ke arah nol.

- **`Employees` mengagregasi seluruh membership aktif di organization, bukan hanya `role = 'employee'`.**
  Assignment lead (`leads.assigned_to_membership_id`) tidak dibatasi role — Owner/Admin/Manager juga bisa
  jadi assignee. Field responsnya tetap `membership_id`/`full_name` (generik), bukan menyaring ke satu
  role tertentu sebelum agregasi.

- **Harness isolasi tenant (`cmd/api/tenant_isolation_test.go`) mendapat test terpisah, bukan entri di
  slice `isolationCase` yang ada.** Kasus itu berbentuk "resource by-id milik org lain → 404"; endpoint
  metrik tidak punya `:id` sama sekali — kebocorannya berbentuk agregat (angka yang salah), bukan baris
  yang salah. `TestTenantIsolation_MetricsAggregate_ScopedToOrganization` ditulis sebagai test terpisah
  di file yang sama, dan **benar-benar dibuktikan bisa gagal**: mengubah predikat
  `organization_id = $1` di `metrics.postgresRepository.Summary` sementara jadi
  `(organization_id = $1 OR true)` membuat test merah — `total_new` organisasi A terbaca 3, bukan 1,
  ikut menghitung dua lead milik organisasi B. Perubahan itu tidak pernah di-commit; lihat komentar di
  test itu sendiri untuk detail.

### Menyimpang dari rencana issue

Tidak ada penyimpangan dari checklist issue #30 atau TD §1–§2 — implementasi mengikuti keduanya persis.

### Verifikasi

```
go build ./...                    → bersih
go vet ./...                      → bersih
gofmt -l .                        → bersih
go test ./... (real Postgres via testcontainers, semua paket)  → semua PASS, termasuk 33 test lama
                                     tanpa perubahan asersi

go test ./internal/shared/config/...    → 11/11 PASS (termasuk 3 test CORS_ALLOWED_ORIGINS baru)
go test ./internal/shared/httpx/... -run TestCORS  → 5/5 PASS
go test ./internal/shared/authz/...     → PASS (matrix bertambah metrics.read × 4 role)
go test ./internal/metrics/...          → 18/18 PASS
  TestUnit_*        (6)  — fake Repository, tanpa Docker
  TestRepository_*  (7)  — Postgres asli: by_status/unassigned/range filter, conversion_rate
                            (denominator excludes spam/unqualified, nil saat 0), avg_response_seconds
                            (excludes lead_created & untouched lead), scoping per organization (×2)
  TestHandler_*     (5)  — HTTP end-to-end: 200 Owner, 403 Employee (×2 endpoint), 401 tanpa kredensial

go test ./cmd/api/... -run TestTenantIsolation  → PASS, termasuk
  TestTenantIsolation_MetricsAggregate_ScopedToOrganization (dibuktikan bisa gagal, lihat di atas)
```

Seluruh acceptance criteria issue #30 terpenuhi.

### Utang teknis

- Tidak ada item baru dari issue ini.

### Catatan untuk session berikutnya

- **#31 dan seterusnya (`crm_dashboard`) sekarang bisa mulai** — CORS dan kedua endpoint metrik yang
  memblokirnya sudah ada. `CORS_ALLOWED_ORIGINS=http://localhost:3000` sudah ditambahkan ke
  `.env.example` dan `docker-compose.yml`; dashboard lokal di port 3000 akan langsung bisa memanggil API
  ini tanpa konfigurasi tambahan.
- `GET /v1/metrics/employees` **tidak dipaginasi** — TD §2.1 tidak menyebutnya, dan jumlah membership per
  organization di volume MVP kecil. Bila ini berubah, itu percakapan produk baru, bukan diam-diam
  ditambah paginasi.
