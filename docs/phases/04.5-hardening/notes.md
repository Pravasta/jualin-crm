# Phase 4.5 — Hardening · Notes

One section per issue, appended as each is implemented.

---

## #57 — Trusted proxy: tegakkan Aturan #34 yang selama ini bisa dilewati

### Keputusan implementasi

- **`TRUSTED_PROXIES` menerima IP maupun CIDR, bukan CIDR saja seperti tertulis di TD §1.1.** TD
  awalnya hanya menyebut "wajib mem-parse sebagai CIDR". Membaca `gin.Engine.prepareTrustedCIDRs`
  langsung sebelum menulis kode menunjukkan Gin sendiri menerima IP telanjang (auto-append `/32` atau
  `/128`) **atau** CIDR. Validasi (`validProxyEntry` di `config.go`) mengikuti persis apa yang Gin
  terima — `net.ParseIP` dulu, baru `net.ParseCIDR` — supaya nilai yang lolos validasi boot **dijamin**
  diterima `SetTrustedProxies`, bukan ditolak diam-diam di tempat lain. Diputuskan sebelum implementasi,
  bukan penyimpangan yang ditemukan setelah kode ditulis.
- **`none` tidak boleh bercampur dengan entri lain** (`TestLoad_TrustedProxiesRejectsNoneMixedWithCIDR`)
  — `none` berarti "tidak ada proxy sama sekali", jadi berdampingan dengan CIDR adalah kontradiksi,
  bukan daftar sebagian.
- **`SetTrustedProxies` dipanggil sebelum middleware apa pun**, termasuk `httpx.Logging` — logger
  sendiri membaca `ClientIP()` (lewat `httpx.RequestID`/`Logging`), jadi ia harus melihat konfigurasi
  yang benar sejak baris pertama `newRouter`, bukan setelah beberapa middleware terpasang.
- **Kegagalan `SetTrustedProxies` di-`panic`, bukan dikembalikan sebagai error dari `newRouter`.**
  `config.Validate()` sudah membuktikan setiap entri parseable sebelum `newRouter` pernah dipanggil —
  satu-satunya cara `SetTrustedProxies` masih bisa gagal di titik ini adalah Gin sendiri tidak sepakat
  dengan `net.ParseIP`/`net.ParseCIDR`, yang berarti bug di validasi ini, bukan kesalahan konfigurasi
  pengguna. ADR-010 melarang proses berjalan setengah siap; `panic` menghentikannya sebelum
  `ListenAndServe` dipanggil, konsisten dengan bagaimana kegagalan config/DB lain ditangani di `main()`.

### Test yang mencoba melewati Aturan #34

Enam test di `cmd/api/trusted_proxy_test.go`, terhadap router produksi sungguhan (`newRouter`, bukan
router tiruan) — mencakup seluruh empat titik panggil `c.ClientIP()` di `internal/auth` (register,
resend, forgot, login), bukan hanya satu endpoint wakil:

| Test | Membuktikan |
|---|---|
| `TestClientIP_Register_ForgedIPDoesNotBypassRateLimit` | `X-Forwarded-For` berbeda tiap request tidak melewati `registerLimit` (5/jam) |
| `TestClientIP_Resend_ForgedIPDoesNotBypassRateLimit` | Sama untuk `resendIPLimit` (10/jam), dengan email baru tiap request supaya sisi email tidak ikut menahan |
| `TestClientIP_ForgotPassword_ForgedIPDoesNotBypassRateLimit` | Sama untuk `forgotIPLimit` (10/jam) |
| `TestClientIP_Login_BackoffNotBypassableByForgedIP` | Backoff `LoginLimiter` ber-key IP tidak direset oleh header palsu + email baru |
| `TestClientIP_UntrustedPeerHeaderIgnoredEvenWithProxiesConfigured` | Peer di luar CIDR tepercaya tetap diabaikan headernya — mempercayai *sebagian* proxy bukan mempercayai *semua* koneksi |
| `TestClientIP_TrustedProxyHeaderIsHonored` (positif) | Proxy yang **memang** dikonfigurasi tepercaya tetap bisa meneruskan IP klien asli — enam klien berbeda dari satu proxy, keenamnya tidak digabung jadi satu key |

### Verifikasi — terbukti bisa gagal

Prosedur sama seperti harness isolasi tenant (#8, #11, #23, #30, #46). `r.SetTrustedProxies(...)` di
`cmd/api/main.go`'s `newRouter` dibungkus `if false { ... }` sementara, lalu
`go test ./cmd/api/... -run TestClientIP -v` dijalankan ulang:

```
--- FAIL: TestClientIP_Register_ForgedIPDoesNotBypassRateLimit (1.92s)
    got 201 pada percobaan ke-6 — seharusnya 429
--- FAIL: TestClientIP_Resend_ForgedIPDoesNotBypassRateLimit (0.03s)
    got 202 pada percobaan ke-11 — seharusnya 429
--- FAIL: TestClientIP_ForgotPassword_ForgedIPDoesNotBypassRateLimit (0.03s)
    got 202 pada percobaan ke-11 — seharusnya 429
--- FAIL: TestClientIP_Login_BackoffNotBypassableByForgedIP (0.02s)
    got 401 invalid_credentials pada percobaan ke-2 — seharusnya 429
--- FAIL: TestClientIP_UntrustedPeerHeaderIgnoredEvenWithProxiesConfigured (0.20s)
    got 201 pada percobaan ke-6 — seharusnya 429
--- PASS: TestClientIP_TrustedProxyHeaderIsHonored (0.20s)
    (diharapkan tetap lulus — default Gin yang rusak KEBETULAN juga
    mempercayai proxy manapun, termasuk yang sengaja dikonfigurasi di
    sini; test ini membuktikan perilaku positif, bukan pembatasan)
```

Lima dari enam test merah persis seperti yang diprediksi TD §3.4 — satu-satunya yang tetap hijau adalah
test positif yang memang tidak menguji pembatasan. Perubahan dikembalikan (`git diff` bersih) segera
setelah run ini; **tidak pernah ter-commit**.

### Test lama — tidak diubah, satu komentar diperbarui

`internal/auth/handler_test.go`'s `TestHandler_Register_RateLimited` (dari #9) **tidak diubah
asersinya**. Komentar baris 125 diperbarui untuk jujur soal apa yang test itu benar-benar buktikan:
`newTestRouter` di file yang sama membangun `gin.New()` tanpa `SetTrustedProxies` sama sekali (router
tiruan `internal/auth`, bukan `cmd/api`'s `newRouter`) — jadi test itu terus bergantung pada
`RemoteAddr` yang kebetulan konstan, bukan pada konfigurasi yang ditegakkan. Bukti bahwa key-nya
**tidak bisa dipalsukan** kini hidup di `cmd/api/trusted_proxy_test.go`, terhadap router produksi
sungguhan — komentar itu sekarang menunjuk ke sana secara eksplisit.

### Verifikasi

- `go test -race ./...` — seluruh paket lulus, termasuk `config` (7 test baru) dan `cmd/api` (6 test
  baru), tanpa perubahan asersi pada test lama manapun.
- `go build ./...`, `go vet ./...` — bersih.
- Dokumentasi: `authentication.md` mendapat bagian baru *Model kepercayaan jaringan* (diagram alur
  `ClientIP()`, tabel dua kesalahan konfigurasi dan gejalanya masing-masing); `api.md`'s bagian *Rate
  limiting* menambah satu paragraf yang menunjuk ke sana. `.env.example` dan `docker-compose.yml`
  mendapat `TRUSTED_PROXIES=none` — `api` diekspos langsung ke host di kedua lingkungan itu, tidak ada
  proxy di depannya.

### Batas — bukan penutup phase

Issue ini menutup akar masalahnya (Aturan #34 kini benar-benar ditegakkan), tapi **belum** menyentuh
eviction map (`ratelimit.FixedWindow`, `auth.LoginLimiter` masih tumbuh tanpa batas — hanya lebih
lambat sekarang, karena laju pertumbuhan key terikat pada IP nyata penyerang). Itu #58, penutup phase.

---

## #58 — Eviction map ber-key penyerang + koreksi kategorisasi utang, penutup Phase 4.5

### Keputusan implementasi

- **Dua generasi (`current`/`previous`), bukan sweep berkala** — keputusan H2 diikuti persis seperti
  ditulis di TD §2.1: memindai map besar sambil memegang lock justru mengubah pertahanan jadi bagian
  dari serangan. Ditukar setiap kurang lebih satu `window` (`FixedWindow`) atau satu `loginBackoffCap`
  (`LoginLimiter`, keputusan H4) — dua generasi sekaligus di memori setiap saat, tidak pernah lebih.
- **Carry-forward lewat lookup, bukan filter saat swap.** Baik `FixedWindow.Take` maupun
  `LoginLimiter.getLocked` memindahkan entry dari `previous` ke `current` **hanya saat entry itu
  diakses lagi** dan masih hidup (`FixedWindow`: `windowStart` belum lewat `window`; `LoginLimiter`:
  `nextAllowedAt` belum lewat). Entry yang tidak pernah diakses lagi setelah demosi cukup dibiarkan —
  ia akan lenyap otomatis di swap berikutnya tanpa perlu langkah pembersihan terpisah. Ini membuat
  kedua map memenuhi kriteria #7 (test lama lulus tanpa perubahan asersi): key mana pun yang masih
  hidup berperilaku **identik** dengan sebelum eviction ada, karena `windowStart`/`count` (atau
  `failures`/`nextAllowedAt`) disalin apa adanya, bukan direset.
- **`now func() time.Time` unexported, di-set hanya dari test paket yang sama** — satu-satunya
  penambahan permukaan ke kode produksi (TD §3.3). `NewFixedWindow`/`NewLoginLimiter` publik keduanya
  tetap tidak berubah tanda tangannya; tidak ada konstruktor baru yang menerima clock, jadi tidak ada
  jalan bagi produksi untuk memakai clock palsu. Empat test butuh kontrol waktu presisi
  (`limiter_internal_test.go`, `login_limiter_internal_test.go`, keduanya `package ratelimit`/`package
  auth` bukan `_test` — deviasi yang sama seperti `internal/apikey/entity_test.go` sejak #46) — alternatifnya
  `time.Sleep` dengan window sungguhan, yang berarti test menunggu jam sungguhan untuk lolos.
- **`Len()` ditambahkan ke kedua tipe**, hanya untuk kebutuhan test membuktikan batas memori lewat
  pengukuran (kriteria #5), bukan API introspeksi umum — tidak ada pemanggil produksi.
- **Dua map ber-key entity bisnis (`apikey.lastUsedThrottle`, `lead.idempotencyCleanupThrottle`)
  sengaja tidak disentuh** (keputusan H3) — komentarnya diperbarui di kedua tempat untuk berhenti
  menyamakan diri dengan `ratelimit.FixedWindow`; sekarang menjelaskan alasan sebenarnya (ber-key
  entity bisnis yang terbatas jumlahnya, bukan input penyerang tanpa batas) dan menunjuk balik ke #58.

### Test yang membuktikan batas memori — diukur, bukan dibaca

| Test | Membuktikan |
|---|---|
| `TestFixedWindow_EvictsAcrossGenerations` | 20 generasi × 50 key unik (1000 total) tetap terlacak ≤100 sepanjang waktu |
| `TestFixedWindow_LiveWindowSurvivesGenerationSwap` | Key yang jendelanya masih hidup melewati swap dengan `Remaining`/`ResetAt` **identik** seolah tidak ada swap sama sekali |
| `TestLoginLimiter_EvictsExpiredBackoff` | Sama untuk `LoginLimiter` — 20×50 key tetap terlacak ≤100 |
| `TestLoginLimiter_ActiveBackoffSurvivesGenerationSwap` | Backoff yang masih aktif (di-dorong ke batas `loginBackoffCap` lewat 10 kegagalan berturut) tetap menahan permintaan setelah swap |

### Verifikasi — terbukti bisa gagal

Prosedur sama seperti #57 dan harness isolasi tenant. Dua titik dirusak sementara di **masing-masing**
tipe, dijalankan, dicatat, dikembalikan — total empat run terpisah, `git diff` bersih setelah setiap
pengembalian:

```
FixedWindow — rollLocked dipaksa `return` di awal (generasi tidak pernah ditukar):
--- FAIL: TestFixedWindow_EvictsAcrossGenerations
    peaked at 1000 — seharusnya ≤100

FixedWindow — carry-forward previous→current dipaksa mati (kondisi `false &&`):
--- FAIL: TestFixedWindow_LiveWindowSurvivesGenerationSwap
    Remaining got 2 (seharusnya 1), ResetAt bergeser +31s — window K ter-reset oleh swap

LoginLimiter — rollLocked dipaksa `return` di awal:
--- FAIL: TestLoginLimiter_EvictsExpiredBackoff
    peaked at 1000 — seharusnya ≤100

LoginLimiter — carry-forward previous→current dipaksa mati:
--- FAIL: TestLoginLimiter_ActiveBackoffSurvivesGenerationSwap
    got allowed — backoff aktif hilang begitu saja saat swap
```

Keempat kali merah persis seperti diprediksi. Tidak satu pun perusakan ter-commit.

### Verifikasi

- `go test -race ./...` — seluruh paket lulus, termasuk 4 test baru di `ratelimit` dan 2 test baru di
  `auth`, tanpa perubahan asersi pada test lama manapun di kedua paket.
- `go build ./...`, `go vet ./...`, `gofmt -l .` — bersih.
- Dokumentasi: `internal/apikey/usecase.go` dan `internal/lead/usecase.go`'s komentar diperbarui
  (dijelaskan di atas). `docs/issues/047-public-lead-api.md` sudah dikoreksi sejak PR pembuka phase
  (#59) — ditinjau ulang di sini, tidak perlu perubahan lagi.

### Seluruh 10 acceptance criteria PRD Phase 4.5

| # | Kriteria | Bukti |
|---|---|---|
| 1 | `X-Forwarded-For` palsu tidak mengubah `ClientIP()` tanpa proxy tepercaya | `TestClientIP_Register/Resend/ForgotPassword_ForgedIPDoesNotBypassRateLimit`, `TestClientIP_Login_BackoffNotBypassableByForgedIP` (#57) |
| 2 | `TRUSTED_PROXIES` wajib saat produksi, boot gagal tanpanya | `TestLoad_ProductionRequiresTrustedProxies` (#57) |
| 3 | Proxy tepercaya meneruskan IP klien asli | `TestClientIP_TrustedProxyHeaderIsHonored` (#57) |
| 4 | Test yang mencoba melewati Aturan #34, kedua sisi | `TestClientIP_Resend_*`, `TestClientIP_ForgotPassword_*` (#57) |
| 5 | Map tidak tumbuh tanpa batas — diukur | `TestFixedWindow_EvictsAcrossGenerations`, `TestLoginLimiter_EvictsExpiredBackoff` (#58) |
| 6 | Dua map ber-key entity tidak disentuh, alasan tertulis | `internal/apikey/usecase.go`, `internal/lead/usecase.go` komentar (#58) |
| 7 | Seluruh test lama lulus tanpa perubahan asersi | `go test -race ./...` bersih di #57 dan #58, nol asersi diubah |
| 8 | Setiap proteksi terbukti bisa gagal | Prosedur adversarial #57 (5 kasus) dan #58 (4 kasus), transkrip di atas |
| 9 | `docs/issues/047` dikoreksi | Dikoreksi di PR pembuka phase (#59), sebelum #57/#58 dikerjakan |
| 10 | `authentication.md` menyatakan model kepercayaan jaringan | Bagian *Model kepercayaan jaringan* (#57) |

**Seluruh 10 kriteria terpenuhi. Phase 4.5 — Hardening selesai.**
