# Phase 4.5 — Hardening · Technical Design

> **Bagaimana.** Apa & kenapa di [`prd.md`](./prd.md).
> Hanya **delta** untuk phase ini — aturan yang sudah ada di [`freeze.md`](../../architecture/freeze.md) dirujuk, tidak diulang.
> Sumber: freeze Aturan #26, #34, #36 · [ADR-010](../../decisions/ADR-010-fail-fast-startup.md) · [`architecture/authentication.md`](../../architecture/authentication.md) bagian *Rate limiting*

---

## 1. Model kepercayaan jaringan

### 1.1 Keadaan sekarang

`cmd/api/main.go:85` memanggil `gin.New()` dan tidak pernah memanggil `SetTrustedProxies`. Nilai
bawaan Gin 1.12 (`gin.go:214–226`):

```go
ForwardedByClientIP: true,
RemoteIPHeaders:     []string{"X-Forwarded-For", "X-Real-IP"},
trustedProxies:      []string{"0.0.0.0/0", "::/0"},
```

`Context.ClientIP()` mengembalikan isi header **bila peer-nya tepercaya**, dan default di atas membuat
setiap peer tepercaya. Karena itu `ClientIP()` sekarang adalah **input dari klien**, bukan fakta
jaringan.

### 1.2 Konfigurasi

Menyusul bentuk `CORS_ALLOWED_ORIGINS` (#30) dan `PUBLIC_API_RATE_LIMIT` (#47) — satu env, di-parse di
`internal/shared/config`, divalidasi saat boot:

```go
// TrustedProxies lists the CIDRs whose X-Forwarded-For header may be
// believed. The literal "none" means no proxy sits in front and the peer
// address is used directly. Required when AppEnv is production (Rule #36)
// because both wrong settings fail silently in opposite directions:
// trusting everyone makes every per-IP limit forgeable, trusting nobody
// while behind a load balancer collapses all users onto one bucket.
TrustedProxies []string `env:"TRUSTED_PROXIES" envSeparator:","`
```

Validasi di `Config.Validate()`, mengikuti pola baris 117–119 yang sudah ada:

```go
if c.AppEnv == "production" && len(c.TrustedProxies) == 0 {
    return fmt.Errorf("config invalid: TRUSTED_PROXIES must be set when APP_ENV=production " +
        "(use \"none\" when no reverse proxy sits in front of this process)")
}
```

Setiap entri selain `none` wajib mem-parse sebagai CIDR — divalidasi saat boot, bukan saat request
pertama:

```go
for _, p := range c.TrustedProxies {
    if p == "none" { continue }
    if _, _, err := net.ParseCIDR(p); err != nil {
        return fmt.Errorf("config invalid: TRUSTED_PROXIES entry %q is not a CIDR: %w", p, err)
    }
}
```

`none` bercampur dengan CIDR lain ditolak — ia berarti "tidak ada proxy", jadi tidak bisa berdampingan
dengan daftar proxy.

### 1.3 Pemasangan

Di `newRouter`, **sebelum** middleware apa pun — `httpx.Logging` sudah mencatat IP, dan ia harus
mencatat yang benar:

```go
r := gin.New()
if err := r.SetTrustedProxies(trustedProxyCIDRs(cfg.TrustedProxies)); err != nil { ... }
```

`trustedProxyCIDRs` mengembalikan `nil` bila daftarnya `["none"]`. `SetTrustedProxies(nil)` membuat
`isTrustedProxy` selalu `false` (`gin.go:415`), sehingga `ClientIP()` melewati seluruh
`RemoteIPHeaders` dan mengembalikan alamat peer — persis yang diinginkan saat tidak ada proxy.

`newRouter` sudah mengembalikan `*gin.Engine` tanpa error. Kesalahan CIDR sudah ditangkap di
`Config.Validate()` sebelum `newRouter` dipanggil, jadi `SetTrustedProxies` di sini tidak bisa gagal
karena input pengguna; error yang tersisa di-`panic` — konsisten dengan ADR-010 (boot tidak boleh
setengah jadi).

### 1.4 Nilai untuk lingkungan yang sudah ada

| Berkas | Nilai | Kenapa |
|---|---|---|
| `.env.example` | `TRUSTED_PROXIES=none` | Pengembangan lokal: proses diakses langsung |
| `docker-compose.yml` | `TRUSTED_PROXIES: none` | `api` diekspos langsung ke host, tidak ada proxy |
| Produksi | Ditentukan saat hosting dipilih | Belum ada — ini justru alasan kontraknya ditetapkan sekarang |

> **Catatan deployment.** Bila nanti ada load balancer, nilainya adalah CIDR **load balancer itu**,
> bukan `0.0.0.0/0`. Bila LB me-*replace* `X-Forwarded-For` (bukan menambahkan), spoofing sudah mati di
> LB — konfigurasi ini tetap wajib supaya `ClientIP()` membaca IP klien asli, bukan IP LB.

---

## 2. Eviction dua generasi (keputusan H2)

### 2.1 Kenapa bukan sweep

Sweep berkala harus memindai seluruh map sambil memegang lock. Justru pada kondisi yang membuat
eviction dibutuhkan — map berisi jutaan key hasil serangan — sweep menjadi pause multi-detik yang
menghentikan seluruh proses. Mekanisme pertahanannya berubah menjadi senjata penyerang.

Menukar dua map membebaskan satu generasi penuh sekaligus tanpa memindai satu key pun. Biayanya O(1)
selamanya, dan GC yang mengerjakan sisanya.

### 2.2 `ratelimit.FixedWindow`

```go
type FixedWindow struct {
	limit  int
	window time.Duration

	mu       sync.Mutex
	current  map[string]*bucket
	previous map[string]*bucket
	genStart time.Time
}
```

`Take` menukar generasi lebih dulu, lalu berperilaku persis seperti sekarang:

```go
func (f *FixedWindow) Take(key string) Result {
	now := time.Now()

	f.mu.Lock()
	defer f.mu.Unlock()

	f.rollLocked(now)

	b, ok := f.current[key]
	if !ok {
		// Carry the bucket forward when its window is still live, so a
		// generation swap never hands anyone a fresh budget mid-window.
		if pb, pok := f.previous[key]; pok && now.Sub(pb.windowStart) < f.window {
			b = &bucket{windowStart: pb.windowStart, count: pb.count}
		} else {
			b = &bucket{windowStart: now}
		}
		f.current[key] = b
	}
	if now.Sub(b.windowStart) >= f.window {
		b.windowStart = now
		b.count = 0
	}
	// … sisanya tidak berubah
}

func (f *FixedWindow) rollLocked(now time.Time) {
	if now.Sub(f.genStart) < f.window {
		return
	}
	if now.Sub(f.genStart) >= 2*f.window {
		f.previous = map[string]*bucket{} // both generations are dead
	} else {
		f.previous = f.current
	}
	f.current = map[string]*bucket{}
	f.genStart = now
}
```

**Semantik per-key tidak berubah.** Bucket yang jendelanya masih hidup ikut terbawa lengkap dengan
`windowStart` dan `count`-nya, jadi tidak ada key yang mendapat jatah baru lebih cepat daripada
sebelumnya. Ini yang membuat kriteria #7 (seluruh test lama lulus tanpa perubahan asersi) bisa
dipenuhi.

**Batas memori:** key yang terlihat dalam 2 × `window` terakhir. Tanpa traffic, tidak ada pertumbuhan —
`rollLocked` hanya berjalan saat ada panggilan.

`Allow` tidak disentuh sama sekali; ia tetap `Take(key).Allowed`.

### 2.3 `auth.LoginLimiter`

Bentuk yang sama, dengan interval generasi `loginBackoffCap` (5 menit) — keputusan H4:

```go
mu       sync.Mutex
current  map[string]*loginBackoffState
previous map[string]*loginBackoffState
genStart time.Time
```

Aturan carry-forward berbeda karena `LoginLimiter` tidak punya jendela tetap. Entry dibawa maju hanya
bila backoff-nya **masih berjalan**:

```go
if ps, pok := l.previous[key]; pok && now.Before(ps.nextAllowedAt) {
	s = ps
	l.current[key] = s
}
```

Entry yang `nextAllowedAt`-nya sudah lewat dilepas. Yang hilang bersamanya adalah hitungan `failures`,
sehingga penyerang yang menunggu lebih dari 2 × 5 menit mendapat tangga backoff dari awal lagi. Itu
berarti laju serangan berkelanjutannya turun dari "1 percobaan per 5 menit" menjadi "1 percobaan per
~10 menit lalu menaik lagi" — **lebih lambat**, bukan lebih cepat. Tidak ada yang diuntungkan.

`RecordSuccess` menghapus dari **kedua** generasi.

### 2.4 Yang sengaja tidak disentuh (keputusan H3)

| Map | Key | Kenapa dibiarkan |
|---|---|---|
| `apikey.Usecase.lastUsedThrottle` | `api_key_id` | Terbatas jumlah kunci yang ada. Hanya bertambah lewat jalur terautentikasi |
| `lead.Usecase.idempotencyCleanupThrottle` | `organization_id` | Terbatas jumlah tenant |

Komentar di kedua tempat diperbarui: rujukan "same accepted debt as `ratelimit.FixedWindow`" **tidak
lagi benar** setelah phase ini dan harus diganti dengan alasan yang sebenarnya (ber-key entity bisnis,
bukan input penyerang).

---

## 3. Rencana test

### 3.1 Kepercayaan proxy

| Test | Berkas | Membuktikan |
|---|---|---|
| `TestClientIP_SpoofedHeaderIgnoredWhenNoProxyTrusted` | `internal/shared/httpx/` | `X-Forwarded-For` palsu **tidak** mengubah `ClientIP()` — kriteria #1 |
| `TestClientIP_TrustedProxyForwardsRealClient` | idem | Di belakang CIDR tepercaya, IP klien asli yang terbaca — kriteria #3 |
| `TestClientIP_UntrustedPeerHeaderIgnored` | idem | Peer di luar CIDR tepercaya diabaikan headernya |
| `TestConfig_TrustedProxiesRequiredInProduction` | `internal/shared/config/` | Boot gagal tanpa `TRUSTED_PROXIES` di produksi — kriteria #2 |
| `TestConfig_TrustedProxiesRejectsNonCIDR` | idem | Nilai salah ketahuan saat boot, bukan saat request |

### 3.2 Aturan #34 — test yang mencoba melewatinya

Ini yang selama ini tidak ada. Untuk `verify-email/resend` dan `password/forgot`, **kedua sisi**:

| Test | Membuktikan |
|---|---|
| `TestHandler_Resend_RateLimitNotBypassableByForgedIP` | 10 request dengan `X-Forwarded-For` berbeda-beda dari satu peer → tetap `429` |
| `TestHandler_Resend_RateLimitNotBypassableByRotatingEmail` | Email berganti tiap request → limiter **IP** tetap menahan |
| `TestHandler_ForgotPassword_RateLimitNotBypassableByForgedIP` | idem untuk jalur reset password |
| `TestHandler_Login_BackoffNotBypassableByForgedIP` | Backoff login tidak direset dengan mengganti header |

Test lama (`TestHandler_Register_RateLimited` dan sekerabatnya) **tidak diubah**. Komentarnya di baris
125 diperbarui: ia bergantung pada `RemoteAddr` konstan, dan setelah phase ini alasannya bukan lagi
kebetulan `httptest` melainkan konsekuensi konfigurasi yang sudah ditegakkan.

### 3.3 Batas memori — diukur, bukan dibaca

Kriteria #5 menuntut bukti, bukan pembacaan kode. Bentuknya deterministik, bukan `runtime.MemStats`
(yang membuat test flaky karena GC):

| Test | Membuktikan |
|---|---|
| `TestFixedWindow_EvictsAcrossGenerations` | Setelah 2 × window dengan clock yang dimajukan, jumlah entry tidak tumbuh meski key unik terus masuk |
| `TestFixedWindow_LiveWindowSurvivesGenerationSwap` | Key yang jendelanya masih hidup **tidak** mendapat jatah baru saat generasi ditukar |
| `TestLoginLimiter_EvictsExpiredBackoff` | Entry yang backoff-nya lewat tidak tertahan melewati dua generasi |
| `TestLoginLimiter_ActiveBackoffSurvivesGenerationSwap` | Backoff yang masih berjalan tidak hilang |

Keempatnya butuh kontrol waktu. `FixedWindow` dan `LoginLimiter` mendapat field `now func() time.Time`
internal (default `time.Now`) — **bukan** interface baru dan bukan paket clock; satu field yang hanya
di-set dari test di paket yang sama.

> Ini satu-satunya penambahan permukaan yang phase ini lakukan pada kode produksi. Alternatifnya adalah
> `time.Sleep` di test dengan window sungguhan, yang berarti test 2 jam untuk limiter resend.

### 3.4 Terbukti bisa gagal (kriteria #8)

Prosedur yang sama seperti harness isolasi tenant (#8, #11, #23, #30, #46) — dijalankan dan
**dicatat hasilnya di `notes.md`**, bukan diklaim:

| Proteksi | Dilepas sementara | Harus merah |
|---|---|---|
| Trusted proxy | `SetTrustedProxies` dihapus dari `newRouter` | §3.1 dan §3.2 |
| Carry-forward generasi | `previous` diabaikan saat lookup | `…LiveWindowSurvivesGenerationSwap` |
| Roll generasi | `rollLocked` di-`return` di awal | `…EvictsAcrossGenerations` |

Perusakan **tidak pernah** ikut ter-commit.

---

## 4. Yang berubah pada dokumentasi

| Berkas | Perubahan |
|---|---|
| `architecture/authentication.md` | Bagian baru **Model kepercayaan jaringan** — apa yang dipercaya, dari siapa, kenapa. Bagian *Rate limiting* menyebut `TRUSTED_PROXIES` sebagai prasyarat penegakan Aturan #34 (kriteria #10) |
| `architecture/api.md` | Bagian *Rate limiting* menambah satu kalimat: header per-IP hanya berarti bila proxy dikonfigurasi |
| `docs/issues/047-public-lead-api.md` | **Koreksi** — pemisahan dua kelas map; poin `last_used_at` dipindahkan dari "butuh traffic" ke "diselesaikan di Phase 4.5" dengan catatan kenapa kategorisasi awalnya salah (kriteria #9). Dua poin lain tetap terbuka, tidak disentuh |
| `STATUS.md` | Baris Utang Teknis `ratelimit.FixedWindow` ditutup; catatan Phase 4 yang menyebut tiga map disesuaikan; Phase 4.5 masuk *Progress per Phase* |
| `.env.example`, `docker-compose.yml` | `TRUSTED_PROXIES` |

`freeze.md` **tidak disentuh** — ia 🔒 FROZEN, dan phase ini tidak mengubah satu pun aturannya. Aturan
#34 tidak berubah; yang berubah adalah implementasi yang selama ini tidak memenuhinya. Tidak ada ADR
baru: `TRUSTED_PROXIES` adalah konfigurasi deployment dengan bentuk yang sudah berpreseden
(`CORS_ALLOWED_ORIGINS`), bukan keputusan arsitektur.

---

## 5. Risiko teknis

| Risiko | Penanganan |
|---|---|
| Salah pasang `TRUSTED_PROXIES` di produksi → seluruh pengguna berbagi satu bucket, ditolak massal | Justru ini alasan ia wajib dan fail-fast. Gejalanya terlihat saat boot atau saat pengguna kedua, bukan saat traffic naik |
| Generasi ditukar tepat saat request → key kehilangan jatah | Tidak bisa terjadi: carry-forward menyalin `windowStart` dan `count` selama jendelanya hidup. Dikunci `…LiveWindowSurvivesGenerationSwap` |
| Field `now` bocor ke produksi | Tidak diekspor, hanya di-set dari test paket yang sama. Tidak ada konstruktor publik yang menerimanya |
| Memori masih bisa tumbuh dalam 2 generasi | Benar, dan itu batas yang disengaja. Setelah §1 ditegakkan, laju pembuatan key terikat pada IP nyata penyerang — bukan lagi tak terbatas |
| Perbaikan ini hilang di refactor mendatang | Kriteria #8 — proteksi yang tidak punya test yang bisa merah tidak dianggap ada |

---

## 6. Kewajiban yang diteruskan ke phase berikutnya

- **Saat hosting dipilih (sebelum deploy produksi pertama):** `TRUSTED_PROXIES` diisi CIDR load
  balancer sungguhan. Boot akan menolak berjalan tanpanya — itu memang mekanismenya, bukan gangguan.
- **Phase 5 (Mobile):** tidak terdampak. Mobile memakai user session; `login:ip:` yang sama berlaku,
  dan kini benar-benar mengikat.
- **Kapan pun traffic produksi nyata ada:** dua poin di `docs/issues/047` yang tetap terbuka
  (`PUBLIC_API_RATE_LIMIT=60`, retensi `idempotency_key` di volume tinggi) baru bisa diputuskan.
  Phase ini **tidak** menyentuh keduanya.
- **Bila rate limit terdistribusi dibutuhkan** (lebih dari satu instance): `ratelimit.Limiter` sudah
  interface. Eviction dua generasi adalah detail implementasi in-memory dan ikut hilang bersamanya —
  bukan sesuatu yang perlu diporting.
