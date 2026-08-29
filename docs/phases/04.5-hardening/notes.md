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
