# Phase 8 — Subscription · Implementation Notes

> Realitas implementasi, satu bagian per issue. Penyimpangan dari TD/PRD beserta alasannya,
> utang teknis, catatan untuk session berikutnya. Naratif lengkap — berbeda dari `docs/issues/11*`
> yang hanya checklist penutupan phase.

---

## #112 — Domain `subscription` penuh: peta kanal + `plan` di `GET /v1/me`

Implementasi mengikuti TD §1–§7 apa adanya. Satu keputusan yang TD tidak mengikat secara eksplisit,
dicatat di sini:

**Organization tanpa baris `subscriptions` berstatus `active`.** `FindActiveByOrg` memfilter
`WHERE status = 'active'`, jadi secara teori bisa mengembalikan nol baris — hari ini tidak pernah
terjadi (`CreateFree` selalu menulis `active`, dan belum ada jalur yang mengubahnya), tapi query-nya
sendiri sudah membuka kemungkinan itu. Repository mengembalikan sentinel `ErrNoActiveSubscription`
alih-alih `pgx.ErrNoRows` mentah. `ResolvePlan` (dipakai `GET /v1/me`) menerjemahkan sentinel itu
menjadi **kode kosong + seluruh kanal tertutup**, bukan error — konsisten dengan prinsip gagal
tertutup TD §1.1/§2, dan supaya satu baris billing yang hilang tidak menjatuhkan seluruh
`GET /v1/me` (dipanggil `SessionGate` di setiap layar). `RequireChannel` (#113) memperlakukan
sentinel yang sama sebagai kanal tertutup, bukan 500.

**Wiring `PlanResolver` lewat `auth.Repos.Plan`, bukan parameter `NewUsecase`.** Preseden
`lead.Repos.Webhook = webhook.NewEnqueuer(q)` — konsisten dengan pola bridging composition-root yang
sudah ada, dan tidak mengubah signature `auth.NewUsecase` yang dipakai 10+ call site test.

**`subscription.New(q)` tetap mengembalikan concrete type (bukan interface `Repository`).** Ia harus
memenuhi dua interface consumer-declared yang bentuknya berbeda sekaligus: `auth.SubscriptionRepository`
(`CreateFree` saja, dipakai `auth.Repos.Sub`) dan `subscription.Repository` milik paket ini sendiri
(`FindActiveByOrg` saja, dipakai `subscription.Usecase`). Interface-ke-interface di Go hanya
type-check bila method set sumbernya superset dari tujuan — dua interface yang salurannya saling
lepas tidak bisa saling menggantikan. Struct diganti nama dari `Repository` (exported) menjadi
`postgresRepository` (unexported) supaya nama `Repository` bisa dipakai untuk interface baru di
`port.go` tanpa tabrakan.

Tidak ada penyimpangan lain dari TD. Nol pemanggil `RequireChannel` per rencana (#113 yang memakainya).
