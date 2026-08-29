# Phase 4.5 — Hardening · PRD

> **Apa & kenapa.** Detail teknis di [`td.md`](./td.md).
> Sumber: [`architecture/freeze.md`](../../architecture/freeze.md) Aturan #34 (rate limit per email dan per IP), Aturan #26, bagian 7 (keputusan default — *Rate limit login: Phase 1, bukan Phase 4*) · [ADR-010](../../decisions/ADR-010-fail-fast-startup.md) (fail-fast saat boot) · [`architecture/authentication.md`](../../architecture/authentication.md) bagian *Rate limiting* · temuan penutupan Phase 4 di [`docs/issues/047-public-lead-api.md`](../../issues/047-public-lead-api.md)

---

## Kenapa phase ini ada

**Phase ini tidak direncanakan.** Ia lahir dari pemeriksaan ulang `docs/issues/046` dan `047` sebelum
Phase 5 dibuka — pemeriksaan yang memang gunanya untuk itu.

Penutupan Phase 4 (#49) mencatat tiga poin sengaja dibiarkan terbuka karena "butuh traffic produksi
nyata untuk diputuskan jujur". Dua di antaranya memang begitu. Yang ketiga — *"peta `last_used_at` tanpa
eviction, sama seperti `ratelimit.FixedWindow`'s bucket map"* — **salah dikategorikan**, dan menarik
benang itu menemukan sesuatu yang lebih besar.

Kesalahannya: empat map in-memory digabung jadi satu baris utang, lalu ditunda seluruhnya dengan alasan
yang sama. Padahal keempatnya terbelah dua kelas yang berbeda secara fundamental:

| Kelas | Map | Key | Batasnya |
|---|---|---|---|
| **Ber-key entity bisnis** | `apikey.lastUsedThrottle` | `api_key_id` | Jumlah kunci yang benar-benar ada |
| | `lead.idempotencyCleanupThrottle` | `organization_id` | Jumlah tenant |
| **Ber-key input penyerang** | `ratelimit.FixedWindow.buckets` | email, IP | **Tidak ada** |
| | `auth.LoginLimiter.state` | email, IP | **Tidak ada** |

Dua yang pertama memang butuh data traffic untuk di-tuning — penundaannya benar dan tetap berlaku.
Dua yang terakhir tidak butuh data apa pun untuk diketahui salah: map yang key-nya dipilih orang
asing yang belum terautentikasi tidak boleh tumbuh selamanya, dan itu bisa disimpulkan dari membaca
kodenya saja.

Menarik benang itu sampai ke akarnya menemukan bahwa **Aturan #34 secara faktual tidak pernah
ditegakkan**, sejak Phase 1, tanpa satu pun test menyadarinya.

---

## Tiga temuan

### A. `c.ClientIP()` dikendalikan penyerang — akar masalahnya

`cmd/api/main.go:85` membangun engine dengan `gin.New()`. `SetTrustedProxies` tidak dipanggil di mana
pun. Default Gin 1.12 adalah `trustedProxies: ["0.0.0.0/0", "::/0"]` dengan
`ForwardedByClientIP: true` dan `RemoteIPHeaders: ["X-Forwarded-For", "X-Real-IP"]` — artinya
**setiap pengirim request dipercaya sebagai proxy**, dan `ClientIP()` mengembalikan apa pun yang ada
di header.

Dibuktikan langsung: satu peer asli `203.0.113.9`, tiga nilai `X-Forwarded-For` berbeda, `ClientIP()`
melaporkan `1.2.3.4`, `5.6.7.8`, `2001:db8::dead`.

Akibatnya seluruh pembatasan per-IP di sistem ini bisa dilewati dengan satu header:

| Endpoint | Key | Nasibnya |
|---|---|---|
| `POST /v1/auth/register` | `ip:` | Dilewati |
| `POST /v1/auth/verify-email/resend` | `email:` + `ip:` | Dilewati **dua-duanya** |
| `POST /v1/auth/password/forgot` | `forgot:email:` + `forgot:ip:` | Dilewati **dua-duanya** |
| `POST /v1/auth/login` | `login:ip:` + `login:email:` | Backoff per-IP dilewati |
| `POST /v1/invitations` | `invite:org:` | **Aman** — ber-key organization |
| `POST /v1/leads` (API publik) | `publiclead:key:` | **Aman** — ber-key API key |

Sisi per-IP dilewati lewat header; sisi per-email dilewati cukup dengan mengganti alamat email tiap
request, karena setiap email baru mendapat jatah baru.

**Aturan #34 berbunyi:** *"Setiap endpoint yang memicu pengiriman email wajib dibatasi per alamat email
dan per IP."* Kedua sisinya bisa dilewati bersamaan. Aturannya ada, kodenya ada, penegakannya tidak ada.

Konsekuensi nyatanya bukan abstrak: **email verifikasi dan reset password bisa dikirim tanpa batas ke
alamat mana pun.** `STATUS.md` sendiri menyebut deliverability email sebagai pembunuh funnel registrasi
— *"email verifikasi yang masuk spam akan membunuh funnel registrasi tanpa menghasilkan satu pun
error"* — dan reputasi domain pengirim adalah item lead-time yang belum diurus. Penyerang, atau
pesaing, bisa membakarnya tanpa biaya.

### B. Memori tumbuh tanpa batas dan tidak bisa direklamasi

Diukur langsung pada `auth.LoginLimiter` dengan 500.000 email karangan:

```
entries retained : 500.000
heap growth      : 64,7 MB  (136 byte/entry)
```

Tidak ada satu pun yang pernah dibebaskan. `RecordSuccess` hanya menghapus entry saat login **berhasil**
— yang tidak akan pernah terjadi untuk akun yang tidak ada. `FixedWindow` sedikit lebih baik (bucket
di-*overwrite* saat jendelanya lewat) tapi tetap tidak pernah **dihapus**.

Digabung dengan temuan A: satu penyerang, satu koneksi, dua entry permanen per request, dan tidak
pernah menyentuh backoff sama sekali karena setiap key selalu baru.

### C. Test-nya memberi lampu hijau palsu

`internal/auth/handler_test.go:125` justru **bersandar** pada `RemoteAddr` yang konstan, dan
komentarnya menyebutkan itu terang-terangan:

> `httptest.NewRequest` defaults RemoteAddr to the same value across calls, so every request in this
> test shares one rate-limit key.

Test itu membuktikan limiter **menyala** saat key-nya sama. Ia tidak pernah membuktikan bahwa key-nya
**tidak bisa dipilih** penyerang. Kolom enforcement Aturan #34 di freeze berbunyi "Test" — dan test itu
akan lulus persis sama seandainya IP sepenuhnya bisa dipalsukan.

Tidak ada satu pun test di seluruh `crm_be` yang pernah mengirim `X-Forwarded-For`.

> Ini standar yang project ini terapkan dengan disiplin tinggi di tempat lain. Harness isolasi tenant
> **dibuktikan bisa gagal** secara adversarial di #8, #11, #23, #30, #46 — predikat tenant sengaja
> dirusak sementara untuk memastikan test-nya benar-benar merah. Rate limit tidak pernah mendapat
> perlakuan yang sama. Phase ini memberikannya.

---

## Seberapa mendesak

**Bukan keadaan darurat, tapi tidak boleh dibawa ke produksi.**

Belum ada yang ter-deploy — hosting masih terbuka di *Keputusan Belum Diambil*. Tidak ada pengguna
nyata yang sedang terpapar hari ini.

Tapi Phase 4 baru saja menambahkan pintu masuk publik **dan** halaman dokumentasi yang mengundang
developer pelanggan masuk lewat pintu itu. Profil paparan produk ini berubah kelas di Phase 4, dan
justru itulah momen yang tepat untuk memeriksa ulang asumsi jaringan yang dibuat di Phase 1 — saat
satu-satunya klien adalah `curl` kita sendiri.

Mengerjakannya sekarang juga lebih murah daripada nanti: `TRUSTED_PROXIES` adalah keputusan
**deployment**, dan hosting belum dipilih. Menetapkan kontraknya sebelum ada infrastruktur berarti
hosting dipilih untuk memenuhi kontrak, bukan kontrak ditambal untuk memenuhi hosting.

---

## Kebutuhan

| # | Sebagai… | Saya butuh… | Supaya… |
|---|---|---|---|
| 1 | Pemilik sistem | Rate limit per-IP yang benar-benar mengikat, bukan yang bisa dilewati satu header | Aturan #34 berarti sesuatu, bukan sekadar tertulis |
| 2 | Pemilik sistem | Domain pengirim email saya tidak bisa dibakar reputasinya oleh orang asing | Funnel registrasi tidak mati diam-diam sebelum produk sempat dijual |
| 3 | Pemilik sistem | Proses yang tidak bisa dihabiskan memorinya oleh orang yang belum login | Satu skrip iseng tidak menjatuhkan seluruh tenant |
| 4 | Pemilik sistem | Konfigurasi proxy yang **wajib** dinyatakan di produksi, bukan ditebak | Salah pasang di belakang load balancer ketahuan saat boot, bukan saat pengguna pertama diblokir |
| 5 | Developer berikutnya | Test yang gagal bila proteksinya dilepas | Perbaikan ini tidak diam-diam hilang di refactor tiga phase lagi |
| 6 | Developer berikutnya | Catatan utang yang membedakan "butuh data" dari "belum dikerjakan" | Poin yang sebenarnya bug tidak ikut tertunda selamanya di balik alasan yang salah |

---

## Acceptance Criteria

Phase 4.5 selesai bila **semuanya** terpenuhi:

| # | Kriteria |
|---|---|
| 1 | `X-Forwarded-For` palsu **tidak** lagi mengubah `ClientIP()` saat tidak ada proxy tepercaya dikonfigurasi — dibuktikan test, bukan dibaca dari kode |
| 2 | `TRUSTED_PROXIES` wajib dinyatakan saat `APP_ENV=production`; boot **gagal** bila tidak (Aturan #36, pola sama seperti `CORS_ALLOWED_ORIGINS` di #30) |
| 3 | Di belakang proxy tepercaya yang dikonfigurasi, `ClientIP()` mengembalikan IP klien asli — bukan IP proxy, dan bukan header dari peer tak tepercaya |
| 4 | Aturan #34 punya test yang **mencoba melewatinya** dan gagal melewatinya — untuk `resend` dan `password/forgot`, kedua sisi (email dan IP) |
| 5 | `ratelimit.FixedWindow` dan `auth.LoginLimiter` tidak lagi tumbuh tanpa batas — dibuktikan dengan pengukuran, bukan dengan pembacaan kode |
| 6 | Dua map ber-key entity bisnis (`apikey.lastUsedThrottle`, `lead.idempotencyCleanupThrottle`) **tidak disentuh** — dan alasannya tertulis |
| 7 | Seluruh test yang sudah ada tetap lulus **tanpa perubahan asersi** — perilaku rate limit yang benar tidak berubah, hanya yang salah |
| 8 | Setiap proteksi baru **terbukti bisa gagal**: dilepas sementara → test merah → dikembalikan (prosedur yang sama seperti harness isolasi tenant) |
| 9 | `docs/issues/047-public-lead-api.md` dikoreksi — pemisahan dua kelas map tercatat, poin yang memang butuh traffic tetap terbuka |
| 10 | `authentication.md` menyatakan model kepercayaan jaringan secara eksplisit: apa yang dipercaya, dari siapa, dan kenapa |

---

## Keputusan yang ditutup di phase ini

| # | Pertanyaan | Keputusan | Alasan |
|---|---|---|---|
| **H1** | Bagaimana proxy tepercaya dikonfigurasi? | Env `TRUSTED_PROXIES` berisi daftar CIDR. Nilai literal `none` berarti tidak ada proxy di depan. **Wajib** saat `APP_ENV=production` — tidak ada default diam-diam. | Dua kesalahan pasang punya gejala berlawanan dan dua-duanya buruk: percaya semua orang → limit bisa dipalsukan; tidak percaya siapa pun **padahal ada** load balancer → seluruh pengguna terlihat berasal dari satu IP dan berbagi satu jatah, sehingga pengguna keenam diblokir tanpa sebab. Tidak ada nilai default yang benar untuk keduanya, jadi ia harus jadi pilihan sadar. Fail-fast saat boot (ADR-010) mengubah kesalahan konfigurasi dari misteri produksi menjadi pesan error. |
| **H2** | Bagaimana map dibatasi? | **Dua generasi** (`current` + `previous`, ditukar tiap jendela), bukan sweep berkala dan bukan batas ukuran. | Sweep berkala harus menyapu seluruh map sambil memegang lock — dengan jutaan key hasil serangan, itu justru menjadi pause multi-detik yang menghentikan seluruh proses. Menukar dua map membebaskan seluruh generasi lama sekaligus tanpa memindai satu key pun: O(1) selamanya, tanpa goroutine latar, tanpa `jobs` (freeze bagian 6 aturan 4 melarang worker sampai ada kebutuhan async nyata — ini bukan). |
| **H3** | Map mana yang diperbaiki? | Hanya dua yang ber-key input penyerang. `apikey.lastUsedThrottle` dan `lead.idempotencyCleanupThrottle` **sengaja tidak disentuh**. | Keduanya ber-key `api_key_id` dan `organization_id` — terbatas oleh jumlah entity bisnis yang benar-benar ada, dan hanya bisa bertambah lewat jalur terautentikasi yang berbayar. Menambahkan eviction ke sana berarti membangun mekanisme untuk masalah yang belum ada (Aturan #27–#29). Penundaannya di `docs/issues/047` **tetap berlaku** — yang dikoreksi hanya alasan yang menggabungkannya dengan dua map lain. |
| **H4** | Apakah backoff `LoginLimiter` boleh hilang saat generasi ditukar? | Boleh, selama interval tukar ≥ `loginBackoffCap` (5 menit). | Entry yang `nextAllowedAt`-nya sudah lewat memang tidak berguna lagi — melepasnya identik dengan membiarkannya. Selama satu generasi hidup minimal selama backoff terpanjang, tidak ada penyerang yang mendapat jatah lebih cepat daripada tanpa eviction. |

---

## Di luar cakupan

| Tidak dikerjakan | Kenapa |
|---|---|
| Angka rate limit final (`PUBLIC_API_RATE_LIMIT=60`, register 5/jam, resend 3+10/jam) | Masih butuh traffic produksi nyata — penundaannya di `docs/issues/047` benar dan tetap berlaku. Phase ini memperbaiki **penegakan**, bukan **angka**. |
| Retensi `idempotency_key` di volume tinggi | Sama — ber-key `organization_id`, terbatas, butuh data nyata |
| Eviction untuk `apikey.lastUsedThrottle` & `lead.idempotencyCleanupThrottle` | Keputusan H3 — terbatas oleh entity bisnis |
| Rate limit terdistribusi (Redis, dsb.) | Freeze melarang infrastruktur di luar PostgreSQL untuk MVP. `ratelimit.Limiter` sudah interface — penggantinya bisa datang tanpa menyentuh satu call site pun |
| Test e2e graceful shutdown (#1), auto-migrate (#2), retensi `notifications` (#22) | Utang tidak berhubungan. Menggabungkannya ke sini hanya karena "sekalian" melanggar Aturan #27–#29 |
| Audit ulang seluruh permukaan auth Phase 1–4 | Cakupannya tidak bisa dibatasi di muka. Bila diinginkan, ia phase tersendiri dengan PRD-nya sendiri |
| CAPTCHA / proof-of-work di endpoint email | Menambah dependensi dan gesekan registrasi untuk masalah yang belum terbukti ada setelah #34 benar-benar ditegakkan |

---

## Dependensi

| Bergantung pada | Sifat |
|---|---|
| Phase 1 (#9, #10) | **Keras.** Seluruh kode yang diperbaiki lahir di sana |
| Phase 4 (#47) | **Lunak.** `publiclead:key:` tidak terdampak, tapi penutupan Phase 4-lah yang menemukan ini |
| Hosting & load balancer | **Tidak memblokir.** Justru sebaliknya — kontrak `TRUSTED_PROXIES` ditetapkan lebih dulu supaya hosting dipilih untuk memenuhinya |

**Tidak memblokir Phase 5.** Mobile memakai user session dan tidak menyentuh jalur ini. Phase 4.5
dikerjakan lebih dulu karena murah, terbatas, dan menyentuh kode yang akan lebih mahal diubah setelah
ada klien kedua yang menempel padanya.
