# Phase 8.5 — Paket Berbayar & Kuota · Implementation Notes

> Realitas implementasi, satu bagian per issue. Penyimpangan dari TD/PRD beserta alasannya,
> utang teknis, catatan untuk session berikutnya. Naratif lengkap — berbeda dari `docs/issues/12*`
> yang hanya checklist penutupan phase.

---

## #122 — Peta limit paket: kuota + seat, `limits`/`usage` di `GET /v1/me`

Mengikuti TD §2, §3, §7. Empat hal yang perlu dicatat — dua penyimpangan dari TD, satu pengukuran, dan
satu keputusan yang TD tidak eksplisit tutup.

### Penyimpangan 1 — penjaga angka provisional: boot gagal, bukan test merah

TD §14 menulis *"test yang mengunci 'angka sudah diisi' gagal — supaya rilis dengan placeholder tidak
mungkin terjadi diam-diam"*. **Tidak diikuti**, dan diganti dengan yang lebih kuat: konstanta
`subscription.LimitsAreProvisional` + penjaga di composition root — `APP_ENV=production` sementara
konstanta itu masih `true` → **proses berhenti sebelum HTTP server menerima satu koneksi** (ADR-010).

Alasannya: test yang sengaja merah membuat CI merah **sejak #122 sampai #126**, dan CI merah yang
"memang seharusnya merah" melatih orang mengabaikan warna merah — persis kebiasaan yang paling mahal
untuk dipulihkan. Boot yang menolak produksi tidak bisa diabaikan, tidak bisa lupa, dan bentuknya
sudah dipakai tiga kali di codebase ini (`WEBHOOK_ALLOW_PRIVATE_TARGETS` #100, `CAPTCHA_PROVIDER=none`
#87, dan `SUBSCRIPTION_TEST_CHECKOUT` yang menyusul di #124). Pesan errornya menyebut persis tiga
langkah untuk membereskannya.

### Penyimpangan 2 — `planChannels` untuk `pro`/`enterprise` diisi sama dengan `free`

TD §2 menyiratkan kanal ikut membedakan paket. **Untuk sekarang ketiganya membuka ketiga kanal.**
Menutup kanal yang hari ini `free` sudah buka berarti **mengambil sesuatu dari organization yang sudah
memakainya** — perilaku downgrade yang Phase 8 D4 tolak bangun justru karena jalurnya belum ada.
Kanal mana yang membedakan paket adalah bagian dari keputusan angka provisional yang masih ditunggu
dari pemilik produk; sampai itu diisi, **pembeda satu-satunya adalah `planLimits`**. Ditulis sebagai
komentar di `plan.go`, bukan hanya di sini.

### Pengukuran — `EXPLAIN` yang TD §4.2 minta, dijalankan

Query hitung tidak bisa memakai `ix_leads_org_created`: **keempat** index `leads` bersifat parsial
(`WHERE deleted_at IS NULL`, migration `0003`), sementara penghitung kuota justru **harus** menghitung
baris yang sudah di-soft-delete. Dijalankan terhadap Postgres asli, 2.000 lead milik organization uji
+ 2.000 milik organization lain, setelah `ANALYZE`:

```
Aggregate  (cost=160.83..160.84 rows=1) (actual time=1.735..1.735 rows=1)
  ->  Nested Loop  (actual time=0.329..1.621 rows=2000)
        ->  Index Scan using organizations_pkey on organizations  (actual time=0.007..0.007 rows=1)
        ->  Seq Scan on leads  (actual time=0.021..0.297 rows=2000)
              Filter: (organization_id = '…')
              Rows Removed by Filter: 2000
Buffers: shared hit=58        Execution Time: 1.762 ms
```

**Diputuskan (a) — biarkan, tanpa index baru.** 1,76 ms untuk 4.000 baris, 58 buffer hit, seluruhnya
dari cache. Seq scan di sini bukan tanda masalah: pada tabel sekecil itu planner memang memilihnya
terlepas dari index apa pun. Ambang untuk meninjau ulang: saat satu organization mendekati puluhan
ribu lead per bulan — di situ `(organization_id, created_at)` **non-parsial** adalah satu migration
kecil, bukan perubahan desain. Dicatat sebagai kewajiban di `docs/issues/122`.

### Keputusan yang TD tidak eksplisit tutup — `seats_used` menghitung undangan pending

TD §6 mewajibkan undangan pending ikut dihitung saat **penegakan** (#124), tapi tidak menyebut apakah
angka yang **ditampilkan** di `/v1/me` harus sama. Diputuskan: **sama**. Kalau layar bilang "2 dari 2
terpakai" lalu undangan ketiga ditolak, itu terbaca seperti bug; kalau layar bilang "3 dari 2", itu
jujur dan menjelaskan dirinya sendiri. Konsekuensinya `Me` menjumlahkan **dua** penghitung
(`membership.CountActive` + `invitation.CountPendingSeats`), dan #124 wajib menjumlahkan **dua yang
sama** — duplikasi aritmetika kecil yang disengaja, dicatat supaya tidak menyimpang.

`CountPendingSeats` predikatnya **lebih ketat** dari `FindByOrgPending` yang sudah ada: ia juga
membuang undangan kedaluwarsa. Undangan yang tidak mungkin diterima lagi tidak boleh menahan seat —
sementara `FindByOrgPending` sengaja tetap menampilkannya supaya layar bisa menandainya kedaluwarsa.

### Catatan lain

- **`ResolvePlan` mengembalikan `subscription.Limits`, bukan dua `int`.** `internal/auth` memang sudah
  mengimpor `internal/subscription` (untuk `*subscription.Subscription`), jadi tidak ada yang
  dihindari dengan meratakannya — sementara dua `int` posisional adalah call site di mana `leads` dan
  `seats` tertukar tanpa compiler protes.
- **`Unlimited = 0` dibaca lewat `allows()`, tidak pernah dibandingkan langsung.** Arti "0 = tanpa
  batas, bukan nol jatah" hidup di satu baris kode produksi, dan satu test yang menyatakannya.
- **`test_checkout_available` di-hardcode `false`** — field-nya dipasang sekarang supaya bentuk
  `/v1/me` stabil untuk #125, tapi endpoint dan flag config-nya milik #124. Dikunci test supaya tidak
  diam-diam jadi `true`.
- **Tiga `COUNT` per panggilan `/v1/me`**, dan `/v1/me` dipanggil di setiap layar terproteksi. Biaya
  ini disebut di TD §7 beserta jalan keluarnya (pindahkan `usage` ke endpoint layar Langganan) —
  belum diambil karena belum terukur mahal; angkanya di atas.
- **Nol pemanggil `allows()` di luar test.** Penegakan adalah #123/#124 — bentuk yang sama dengan
  #112's `RequireChannel`, `apikey.FindByKeyID` (#46), `form.FindByPublicKey` (#85).

`go test -race ./...` bersih, `golangci-lint run` 0 issues, tanpa migration.
