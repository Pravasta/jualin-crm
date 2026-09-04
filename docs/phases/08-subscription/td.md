# Phase 8 — Subscription · TD

> **Bagaimana.** Apa & kenapa di [`prd.md`](./prd.md).
> Ini **delta** untuk phase ini. Aturan yang sudah ada di [`freeze.md`](../../architecture/freeze.md) tidak diulang, hanya dirujuk.

---

## 1. Schema — **tidak ada migration di phase ini**

Kolom yang dibutuhkan sudah ada sejak `0002_identity.sql` (Phase 1, amandemen S1):

```sql
CREATE TABLE subscriptions (
    id                    uuid PRIMARY KEY,
    organization_id       uuid NOT NULL REFERENCES organizations (id),
    plan_code             text NOT NULL DEFAULT 'free',   -- ← pembacanya lahir di phase ini
    status                text NOT NULL,                  -- ← ikut dibaca
    current_period_start  timestamptz,
    current_period_end    timestamptz,
    external_reference    text,                           -- ← titik sambung payment service, TIDAK disentuh
    ...
    CONSTRAINT ck_subscriptions_status CHECK (status IN ('active','past_due','suspended','canceled')),
    CONSTRAINT uq_subscriptions_id_org UNIQUE (id, organization_id)
);
CREATE UNIQUE INDEX uq_subscriptions_org_active ...;
```

**Phase kedua tanpa migration sama sekali**, setelah Phase 3. Itu bukan kebetulan melainkan hasil
langsung amandemen S1: *"Membuat baris subscription tanpa mesin penegakan adalah biaya yang mendekati
nol; menambahkan tabelnya belakangan berarti backfill untuk semua organization yang sudah ada."*
Phase 8 menagih hasil dari keputusan itu.

**Tidak ada `usage_counters` (prd D1) dan tidak ada `plans` (prd D2).** Keduanya penyimpangan tertulis
dari `freeze.md` 8.4, dicatat penuh di PRD beserta kewajiban evaluasi ulangnya.

### 1.1 `status` ikut menggerbangi, bukan hanya `plan_code`

`ck_subscriptions_status` sudah membatasi ke `active`/`past_due`/`suspended`/`canceled`. Gerbang
memperlakukan **hanya `active`** sebagai berlaku; tiga lainnya menutup seluruh kanal, apa pun
`plan_code`-nya.

Hari ini hanya `active` yang pernah ditulis (`CreateFree`), jadi cabang itu tak pernah terpakai — tapi
ia gratis untuk ditulis sekarang dan **mahal untuk diingat nanti**: saat payment service mulai
menuliskan `past_due`, gerbang yang hanya membaca `plan_code` akan diam-diam tetap membuka semuanya.

---

## 2. Paket & kanal — satu peta, satu tempat (prd D3)

```go
// internal/subscription/plan.go

type Channel string

const (
    ChannelAPIKey  Channel = "api_key"
    ChannelForm    Channel = "form"
    ChannelWebhook Channel = "webhook"
)

// Channels is the closed set every table-driven test iterates over —
// a hand-written list is what lets a new channel be forgotten.
var Channels = []Channel{ChannelAPIKey, ChannelForm, ChannelWebhook}

const PlanFree = "free"

// planChannels is THE map. Opening or closing a channel for a plan is a
// one-line change here and nowhere else (kriteria #2).
//
// Every channel is open on `free` today. That is not a placeholder: it is
// the honest state of the product until pricing exists (ADR-012 §4, prd
// kriteria #9). The mechanism is what ships now; the numbers replace this
// literal later.
var planChannels = map[string]map[Channel]bool{
    PlanFree: {ChannelAPIKey: true, ChannelForm: true, ChannelWebhook: true},
}
```

| Ketentuan | Alasan |
|---|---|
| Paket yang **tidak ada di peta** → seluruh kanal tertutup | Gagal **tertutup**, bukan gagal terbuka (kriteria #8). `plan_code` adalah `text` tanpa `CHECK` — nilai tak terduga bisa masuk dari luar CRM (payment service menulis `external_reference` dan paket), dan default-nya harus menolak |
| `status != 'active'` → seluruh kanal tertutup | §1.1 |
| Bentuknya cermin `authz.apiKeyScopeFor` / `publicFormAllows` | Pertanyaan yang sebentuk ("principal/paket jenis ini boleh apa"), dijawab dengan membaca **satu baris**. Keduanya sudah dikunci test yang mengulang seluruh himpunan `Action`; peta ini mendapat perlakuan yang sama atas `Channels` |

**Kenapa bukan kolom di database.** Peta ini adalah *kebijakan produk*, bukan data pelanggan. Menaruhnya
di tabel berarti mengubah paket = migration atau panel admin yang belum ada — dan tabel `plans` sudah
ditolak D2. Sebagai literal Go ia ikut review kode, ikut test, dan ikut deploy; ketiganya justru yang
diinginkan untuk keputusan yang jarang berubah dan berdampak luas.

---

## 3. Gerbang — di usecase, setelah `authz` (prd D4)

### 3.1 Bentuk

```go
// internal/subscription/usecase.go
func (u *Usecase) RequireChannel(ctx context.Context, t tenant.Context, ch Channel) error
```

Mengembalikan `nil` bila terbuka, dan **`*httpx.DomainError` (403 `plan_upgrade_required`)** bila
tidak — bukan sentinel error.

**Kenapa error HTTP dikembalikan langsung dari bridge.** Preseden #22: sentinel error domain
(`lead.ErrAssigneeNotFound`) tidak bisa dikenali lintas paket tanpa melanggar ADR-011, dan
diselesaikan dengan mengembalikan `*httpx.ValidationError` langsung dari bridge method itu sendiri.
Bentuk yang sama dipakai di sini: `apikey`/`form`/`webhook` cukup `return err` tanpa perlu tahu paket
mana yang memutuskan atau kenapa.

### 3.2 Interface dideklarasikan konsumen (ADR-011)

Tiga paket domain mendeklarasikan interface-nya sendiri, primitif saja — persis bentuk
`lead.ActivityRecorder` (#21), `form.LeadCreator` (#87), `lead.WebhookEnqueuer` (#101):

```go
// internal/apikey/port.go  (dan padanannya di form/, webhook/)
type PlanGate interface {
    RequireChannel(ctx context.Context, t tenant.Context, ch string) error
}
```

`ch` bertipe `string` di sisi konsumen, bukan `subscription.Channel` — kalau tipenya diimpor, paket
itu mengimpor `internal/subscription`, yang justru dihindari interface ini. Nilai literalnya
(`"api_key"`, `"form"`, `"webhook"`) adalah **kontrak kabel** antar-issue; lihat §7.

Dijembatani di composition root (`cmd/api/subscription_gate.go`), bukan lewat impor langsung — bentuk
yang sama dengan `activity.NewRecorder(q)` dan `webhook.NewEnqueuer(q)`.

### 3.3 Urutan di dalam usecase — mengikat

```go
func (u *Usecase) Create(ctx context.Context, t tenant.Context, in CreateInput) (...) {
    if err := authz.Require(t, authz.ActionAPIKeyCreate); err != nil {
        return err                        // 1. role  → 403 forbidden
    }
    if err := u.plan.RequireChannel(ctx, t, "api_key"); err != nil {
        return err                        // 2. paket → 403 plan_upgrade_required
    }
    // 3. validasi field, lalu kerja
}
```

**Role lebih dulu, paket kedua — dan urutannya tidak boleh dibalik.** Manager yang memanggil
`POST /v1/forms` harus menerima `403 forbidden`, bukan `403 plan_upgrade_required`: yang kedua
membocorkan keadaan paket organization kepada orang yang memang tidak berhak mengelola kanal itu sama
sekali. Semangatnya sama dengan Aturan #6 (404 alih-alih 403 untuk tenant lain) — jangan menjawab
pertanyaan yang penanyanya tidak berhak ajukan.

### 3.4 Titik pasang — hanya `POST` (prd D4)

| Endpoint | Digerbangi | Tidak digerbangi |
|---|---|---|
| `POST /v1/api-keys` | ✅ | |
| `POST /v1/forms` | ✅ | |
| `POST /v1/webhook-endpoints` | ✅ | |
| `GET`/`PATCH`/`DELETE` ketiganya | | ✅ resource yang sudah ada tetap dikelola |
| `POST /v1/leads` (jalur API key) | | ✅ kunci yang sudah terbit tetap bekerja |
| `POST /v1/forms/{public_key}/submit` | | ✅ form yang sudah tertanam tetap menerima |
| Pengiriman webhook keluar (worker) | | ✅ endpoint yang sudah ada tetap terkirim |

Baris-baris "tidak digerbangi" adalah konsekuensi langsung D4: menutup resource yang sudah jalan
adalah perilaku **downgrade**, dan tidak ada jalur downgrade hari ini. Menambahkannya nanti berarti
menambah titik pasang, bukan mengubah mekanisme.

---

## 4. `GET /v1/me` membawa kapabilitas yang sudah diselesaikan (prd D5)

```json
{
  "data": {
    "user_id": "…", "email": "…", "full_name": "…",
    "organization_id": "…", "organization_name": "…",
    "membership_id": "…", "role": "owner",
    "plan": {
      "code": "free",
      "channels": { "api_key": true, "form": true, "webhook": true }
    }
  }
}
```

| Ketentuan | Alasan |
|---|---|
| `channels` adalah **jawaban**, bukan bahan | Dashboard merender `channels.webhook` langsung. Tidak ada peta paket→kanal versi TypeScript — itu sumber kebenaran kedua yang pasti menyimpang (prd D5, kesalahan `lib/lead-status.ts` di #33) |
| `code` tetap dikirim | Untuk **ditampilkan** ("Paket: Free"), bukan untuk diputuskan. Kriteria #6 melarang keputusan UI berdasarkan `code` |
| Tidak ada endpoint `GET /v1/subscription` baru | `SessionGate` sudah memanggil `/v1/me` di **setiap** layar terproteksi (#31). Endpoint kedua berarti request kedua untuk data yang sudah di tangan — Aturan #27 |
| `status`, `current_period_*`, `external_reference` **tidak** dikirim | Belum ada layar yang memerlukannya, dan `external_reference` adalah rujukan internal ke payment service yang tidak punya urusan di klien |

`internal/auth` mendeklarasikan interface consumer-nya sendiri untuk ini — bentuk yang sama dengan
`auth.RefreshTokenRevoker` (#11):

```go
// internal/auth/port.go
type PlanResolver interface {
    ResolvePlan(ctx context.Context, t tenant.Context) (code string, channels map[string]bool, err error)
}
```

---

## 5. Error code baru

| Status | Code | Kapan |
|---|---|---|
| `403` | `plan_upgrade_required` | Paket organization tidak membuka kanal yang diminta, atau `status != 'active'` |

Satu kode untuk kedua sebab — sama seperti `webhook_url_not_allowed` (Phase 7 §7) dan
`invalid_api_key` (Phase 4): membedakan *"paketmu tidak mencakup ini"* dari *"tagihanmu tertunggak"*
di response HTTP tidak berguna bagi klien dan membocorkan keadaan penagihan ke permukaan yang salah.
Alasan detailnya masuk log server, bukan response.

`403 forbidden` (role) tetap terpisah dan **selalu didahulukan** (§3.3).

---

## 6. Paket & berkas

```
crm_be/internal/subscription/
    entity.go               Subscription (ada) + Plan, Channel, Channels
    plan.go                 planChannels + resolve — BARU, jantung phase ini
    port.go                 Repository (consumer-declared) — BARU
    usecase.go              ResolvePlan + RequireChannel — BARU
    repository_postgres.go  CreateFree (ada) + FindActiveByOrg — BARU

crm_be/internal/{apikey,form,webhook}/port.go   + PlanGate (masing-masing miliknya sendiri)
crm_be/internal/auth/port.go                    + PlanResolver
crm_be/cmd/api/subscription_gate.go             bridge di composition root

crm_dashboard/src/lib/session-context.tsx       + plan pada tipe sesi
crm_dashboard/src/lib/plan.ts                   helper baca channels (BUKAN peta paket)
```

**Tidak ada `Store`/`InTx` di `internal/subscription`.** Phase ini hanya **membaca** — preseden
`internal/metrics` (#30, TD Phase 3 §2), yang juga sengaja tanpa Store. `CreateFree` yang sudah ada
tetap dipanggil sebagai repository polos dari dalam transaksi registrasi `internal/auth`, tidak
berubah.

---

## 7. Kontrak kabel antar-issue

Nilai literal yang gagal **diam-diam** kalau tidak dicocokkan persis — dicatat di sini, dicek di
`docs/issues/` saat issue-nya dikerjakan (pola `docs/issues/087`, `101`):

- `"api_key"`, `"form"`, `"webhook"` — string kanal, dipakai di **empat** tempat: `planChannels`,
  ketiga `PlanGate` call site, kunci JSON `plan.channels` di `/v1/me`, dan pembacaan di TypeScript.
  Salah ketik di salah satunya menghasilkan kanal yang **selalu tertutup** (peta tidak punya kuncinya)
  atau **selalu terbuka** (dashboard membaca `undefined` lalu memperlakukannya sebagai apa pun) —
  keduanya tanpa error.
- Karena itu §12 mewajibkan satu test yang membandingkan **himpunan kunci** `plan.channels` di response
  `/v1/me` dengan `subscription.Channels`, dan satu di TypeScript yang membandingkan literalnya dengan
  daftar yang sama — bentuk yang sudah dipakai `WEBHOOK_EVENTS` vs `webhook.KnownEvents` (#103).

---

## 8. Dashboard — keadaan "terkunci oleh paket" (prd D6)

Kartu Connect punya **tiga** keadaan sekarang, dan ketiganya harus dibedakan dengan jujur:

| Keadaan | Kapan | Tampilan |
|---|---|---|
| **Aktif** | `channels[x] === true` | Seperti sekarang — bisa diklik |
| **Terkunci oleh paket** | `channels[x] === false` | **Terlihat, redup, tidak bisa diklik**, dengan penjelasan bahwa kanal ini tidak termasuk paket saat ini. **Tanpa tombol upgrade, tanpa harga** (D6) |
| **Belum tersedia** | Kanal yang produknya memang belum punya | Sudah tidak dipakai satu pun kanal sejak #103 — dipertahankan untuk kanal berikutnya (inbound webhook), **jangan dihapus dan jangan dipakai untuk terkunci** |

> Perbedaan "terkunci" vs "belum tersedia" bukan kosmetik. `06/td.md` §416 sudah menuliskannya:
> *"keadaan terkunci-oleh-paket baru lahir di Phase 8. Menyatakannya terkunci sekarang akan berbohong."*
> Kebalikannya berlaku sekarang: menyatakan kanal berbayar sebagai "belum tersedia" juga berbohong.

**Gerbang paket berdiri di samping gerbang role, bukan menggantikannya.** `canManageAPIKeys`/
`canManageForms`/`canManageWebhooks` tetap apa adanya. Manager yang membuka `/connect/webhook` tetap
melihat *"tidak tersedia untuk role Anda"* — bukan keadaan terkunci paket, karena paket bukan
alasannya (§3.3 di UI).

Layar tujuan (`/connect/api`, `/connect/form`, `/connect/webhook`) juga menangani
`403 plan_upgrade_required` yang datang dari **balapan** — paket berubah antara render dan klik.
Ditangani inline seperti `delivery_not_retryable` di #103: pesan backend apa adanya, bukan tombol yang
diam (kriteria #7).

---

## 9. Konfigurasi

**Tidak ada env var baru.** Peta paket adalah literal Go (§2), bukan konfigurasi — dan tidak ada
angka apa pun di phase ini (kriteria #9).

---

## 10. Otorisasi

**Tidak ada `Action` baru.** Membaca paket sendiri bukan aksi ber-otorisasi terpisah: ia ikut di
`GET /v1/me`, yang sudah terbuka untuk setiap principal user terautentikasi. Gerbang paket adalah
dimensi **kedua** di samping `authz` (§3.3), bukan perluasan matriksnya.

`authorization.md` tetap diperbarui — dengan satu bagian yang menjelaskan bahwa sejak Phase 8 ada
**dua** pertanyaan berbeda yang harus dilewati sebuah `POST` kanal: *"role ini boleh?"* (`authz`) dan
*"paket ini membuka?"* (`subscription`), dan keduanya tidak boleh tertukar.

---

## 11. Principal non-user

`api_key` dan `public_form` **tidak** melewati gerbang paket, dan itu disengaja:

| Principal | Kenapa tidak digerbangi |
|---|---|
| `api_key` (`POST /v1/leads`) | Kunci yang sudah terbit adalah resource yang sudah ada — D4. Menutupnya adalah downgrade |
| `public_form` (`POST /v1/forms/{public_key}/submit`) | Sama; ditambah: pengunjung situs pelanggan tidak boleh menerima error tentang paket pelanggan |

Yang digerbangi adalah **penerbitan** kredensialnya (`POST /v1/api-keys`, `POST /v1/forms`), bukan
pemakaiannya. Bila kelak kuota mendarat, di situlah `usage_counters` masuk — dan itu mekanisme berbeda
(TD Phase 4 §19: *"`usage_counters` menghitung; `ratelimit` melindungi"* — dan gerbang kapabilitas
tidak melakukan keduanya).

---

## 12. Rencana test

| Berkas | Menguji |
|---|---|
| `internal/subscription/plan_test.go` | **Tabel atas seluruh `Channels` × seluruh paket yang dikenal** — bukan daftar tulis tangan, supaya kanal baru phase berikutnya otomatis ikut. Plus: paket tak dikenal → **semua tertutup**; `status` non-`active` → semua tertutup (§1.1) |
| `internal/subscription/usecase_unit_test.go` | `RequireChannel` mengembalikan `nil` vs `*httpx.DomainError` 403 `plan_upgrade_required`; `ResolvePlan` mengembalikan himpunan kunci = `Channels` |
| `internal/subscription/repository_test.go` | Postgres asli: `FindActiveByOrg` mengambil baris yang benar, dan **tidak** melihat organization lain |
| `internal/{apikey,form,webhook}/usecase_unit_test.go` | Gerbang dipanggil, dan **urutannya benar**: fake yang menolak role **dan** paket sekaligus harus menghasilkan `forbidden`, bukan `plan_upgrade_required` (§3.3) — ini yang membuktikan urutannya, bukan komentar di kode |
| `internal/{apikey,form,webhook}/handler_test.go` | `POST` → `403 plan_upgrade_required` saat kanal tertutup; `GET`/`PATCH`/`DELETE` **tetap** bekerja (D4) |
| `internal/auth/handler_test.go` | `GET /v1/me` membawa `plan.code` dan `plan.channels`; **himpunan kunci `channels` = `subscription.Channels`** (kontrak kabel §7) |
| `cmd/api/plan_gate_test.go` | **Terbukti bisa gagal**: satu entri `planChannels[free][webhook]` dibalik jadi `false` → `POST /v1/webhook-endpoints` yang tadinya `201` menjadi `403`, sementara `GET`-nya tetap `200`. Dijalankan, lalu dikembalikan — tidak pernah commit dalam keadaan disabotase (prosedur harness isolasi tenant #11/#23) |
| `crm_dashboard/src/lib/plan.test.ts` | Literal `"api_key"`/`"form"`/`"webhook"` dibandingkan dengan daftar yang sama (§7); `channels` yang tidak punya kunci → diperlakukan **tertutup**, bukan terbuka |

### Verifikasi manual wajib

Kriteria #3 mensyaratkan penolakan dibuktikan **lewat `curl`**, bukan lewat UI yang menyembunyikan
tombol — ADR-012 §3 menyatakan justru itu bedanya kenyamanan dan penegakan. Prosedurnya masuk
`docs/testing/flow/` saat issue penutup, termasuk membalik satu entri peta lalu memanggil endpoint
langsung.

---

## 13. Risiko teknis

| Risiko | Penanganan |
|---|---|
| **Gerbang gagal terbuka** — paket tak dikenal diperlakukan sebagai "boleh semua" | Peta mengembalikan `false` untuk paket yang tidak ada (§2), diuji eksplisit. `plan_code` adalah `text` tanpa `CHECK`; nilai tak terduga bisa datang dari luar CRM |
| **Peta kedua lahir di TypeScript** | Dicegah secara struktural: `/v1/me` mengirim kapabilitas yang sudah diselesaikan, jadi tidak ada yang perlu disalin (D5). Dikunci test kontrak kabel §7 |
| **Urutan role-vs-paket tertukar**, membocorkan keadaan paket ke role yang tidak berhak | §3.3, diuji dengan fake yang menolak keduanya sekaligus — satu-satunya cara membedakan urutan dari luar |
| **Gerbang dipasang di tempat yang salah** dan menutup resource yang sudah jalan | Tabel §3.4 mengikat; test D4 memastikan `GET`/`PATCH`/`DELETE` dan jalur publik tetap `200` |
| **Angka menyusup lebih awal** — harga atau limit ditulis "sementara" lalu bertahan | Kriteria #9, dan §9 menghapus tempatnya (tidak ada env var, tidak ada kolom). ADR-012 §4 menamai risiko ini sendiri |
| **`status` diabaikan** dan `past_due` tetap membuka semuanya | §1.1 — ditulis sekarang meski belum pernah terpakai, karena ia mahal untuk diingat nanti |

---

## 14. Yang harus disiapkan pemilik produk

**Tidak ada.** Tidak ada pihak ketiga, tidak ada akun, tidak ada angka yang harus diputuskan sebelum
phase ini bisa dikerjakan — itu justru bentuk yang dipilih (prd, *Kenapa mekanismenya dibangun sebelum
angkanya*).

**Yang tetap terbuka dan jatuh tempo bersama-sama**, di luar phase ini: harga, limit free tier, retensi
data free tier, peta kanal-per-paket yang sesungguhnya, dan kontrak integrasi payment service.
Kelimanya butuh data dari pengguna nyata (gate freeze), bukan keputusan yang bisa dipercepat dengan
menulis kode.

**Tidak memblokir issue manapun.**

---

## 15. Yang berubah pada dokumentasi

| Berkas | Perubahan |
|---|---|
| `architecture/api.md` | Error code baru `plan_upgrade_required`; bab singkat *Gerbang Paket* — apa yang digerbangi, apa yang sengaja tidak (§3.4), dan bahwa CRM tidak pernah tahu tentang uang |
| `architecture/authorization.md` | Bagian baru: **dua pertanyaan berbeda** yang harus dilewati sebuah `POST` kanal (role vs paket), urutannya, dan kenapa paket **bukan** baris di matriks `Action` (§10) |
| `architecture/authentication.md` | Tidak disentuh — subscription bukan kredensial dan bukan jalur autentikasi |
| `docs/testing/flow/` | Bagian pada berkas yang ada (bukan berkas baru): membalik satu entri peta, lalu membuktikan `403` lewat `curl` dan kartu terkunci di browser |
| `STATUS.md` | Baris Selesai; Phase 8 di *Progress per Phase*; `Keputusan Belum Diambil` diperbarui — pricing/limit/retensi **tetap** terbuka, dan sekarang punya tempat yang menunggunya |

`freeze.md` **tidak disentuh.** Penyimpangan D1 (`usage_counters`) dan D2 (`plans`) dicatat di
`prd.md` sebagai keputusan phase beserta kewajiban evaluasi ulangnya — bukan diselipkan seolah freeze
memang mengizinkannya (Aturan #30). ADR baru **tidak diperlukan**: mekanismenya sudah ditetapkan
ADR-012, dan phase ini mengimplementasikannya, bukan mengubahnya.

---

## 16. Kewajiban yang diteruskan ke phase berikutnya

- **Saat angka mendarat** (setelah gate freeze): isi `planChannels` dengan paket sesungguhnya —
  perubahan pada satu literal, bukan pada usecase manapun. Bersamaan dengan itu, evaluasi ulang
  `usage_counters` (prd D1) dan jawab pertanyaan downgrade (prd D4).
- **Saat payment service punya kontrak**: kartu terkunci mendapat satu tautan keluar (prd D6), dan
  `external_reference` mendapat pembacanya. Bentuk kartunya tidak berubah.
- **Saat kanal keempat lahir** (inbound webhook Phase 7.5, dst.): tambahkan satu konstanta ke
  `Channels` dan satu entri ke `planChannels`. Test tabel §12 otomatis mencakupnya — itu memang
  alasan `Channels` ada sebagai slice, bukan daftar tulis tangan di tiap test.
