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

---

## #113 — Gerbang paket di usecase: tiga kanal + `plan_upgrade_required`

Implementasi mengikuti TD §3, §5, §11 apa adanya. Dua tambahan di luar TD (disetujui sebelum
implementasi), satu keputusan test yang tidak eksplisit ditulis TD:

**`subscription.ParseChannel` (baru, exported) dipakai di bridge, bukan konversi buta
`subscription.Channel(ch)`.** TD §7 sendiri menamai risikonya: literal salah ketik di salah satu dari
empat tempat yang menduplikasi `"api_key"`/`"form"`/`"webhook"` membuat kanal itu **selalu tertutup
tanpa error** — `planChannels` tidak punya kuncinya, dan perilaku gagal-tertutup `channelsFor`
membuatnya tak terbedakan dari penolakan yang jujur. `cmd/api/subscription_gate.go`'s `planGate`
memanggil `ParseChannel`; string tak dikenal di-log sebagai error dan dikembalikan sebagai error
buram — **bukan** 403 — karena itu bug pemasangan di codebase ini, bukan keadaan paket pelanggan.

**Error code `plan_upgrade_required` ditunda ke katalog `api.md` sampai #115** (keputusan pemilik
produk) — meski mulai PR ini kode itu benar-benar bisa muncul di response. `authorization.md`'s bagian
"dua pertanyaan" (role vs paket) juga ditunda ke #115, TD §15 sudah menetapkan itu tempatnya.

**Membuktikan gerbang menolak tanpa menyabotase peta.** AC #3/#4 minta bukti `403` lewat request
sungguhan terhadap wiring asli. Alih-alih membalik `planChannels[free][webhook]` di kode yang
di-commit (yang berarti peta tidak lagi jujur), `cmd/api/plan_gate_test.go` menutup paket lewat
**data**: `UPDATE subscriptions SET status = 'past_due'` untuk organization uji. Ini mengaktifkan
cabang gagal-tertutup TD §1.1 yang sudah ada — peta asli, bridge asli, repository asli, tidak ada
satu baris kode produksi yang disabotase. Prosedur AC #4 yang sesungguhnya (membalik satu entri peta,
menjalankan, mengembalikan) tetap dilakukan manual sebagai verifikasi terpisah — dicatat di bawah.

**Injeksi `PlanGate` lewat parameter `NewUsecase`, bukan `Repos`** — beda dari #112's `auth.Repos.Plan`.
TD §3.1 menulis `u.plan.RequireChannel(...)` sebagai field `Usecase`, dan gerbang dicek **sebelum**
`Store.InTx` dibuka (bukan bagian unit-of-work seperti `Repos`, yang dibangun ulang tiap transaksi).
Konsekuensinya: ~30 call site test di tiga paket (`apikey`/`form`/`webhook`) + 2 di `cmd/api` harus
disentuh — mekanis, ditambah `fakePlanGate`/`alwaysOpenPlanGate`/`alwaysClosedPlanGate` di tiap paket
test yang terkena.

**Verifikasi AC #4 dijalankan (bukan hanya diklaim), lalu dikembalikan — tidak pernah commit dalam
keadaan disabotase:**

```
1. internal/subscription/plan.go: planChannels[PlanFree][ChannelWebhook] diubah true → false
2. go test ./cmd/api/... -run TestPlanGate_OpenPlan_AllThreeChannelsCreatable -v
   → FAIL: "webhook: expected 201 on an open plan, got 403:
      {"error":{"code":"plan_upgrade_required", ...}}" — api_key dan form TIDAK ikut merah
3. Baris dikembalikan ke true; go test ./cmd/api/... -run TestPlanGate -v → PASS kelimanya
4. git diff internal/subscription/plan.go → kosong, sabotase tidak pernah masuk staging/commit
```

Verifikasi lewat `curl` terhadap server sungguhan dan kartu terkunci di dashboard **belum dilakukan** —
yang kedua memang menunggu #114 (dashboard belum menggerbangi apa pun). Prosedur `curl` end-to-end
untuk kedua-duanya masuk `docs/testing/flow/` sebagai kewajiban issue penutup (#115, TD §12 "Verifikasi
manual wajib").

Tidak ada penyimpangan lain dari TD. Titik pasang tetap persis tabel §3.4 (hanya `POST` ketiga kanal);
`GET`/`PATCH`/`DELETE`, `POST /v1/leads` (API key), dan `POST /v1/forms/{public_key}/submit`
diverifikasi eksplisit tetap terbuka lewat `cmd/api/plan_gate_test.go`. Tidak ada `Action` `authz` baru.
