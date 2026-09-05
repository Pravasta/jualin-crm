# Issue #125 — checklist penutupan phase

> Checklist ringkas, **bukan** catatan status. Status pekerjaan tetap hidup di GitHub Issues (ADR-008) —
> berkas ini mengumpulkan poin yang perlu **dicek ulang saat #126** menutup Phase 8.5. Detail lengkap ada
> di `docs/phases/08.5-paid-plans/notes.md` bagian `## #125`.

## Deviasi dari issue (bukan hanya TD)

- [ ] **#125 berlabel `dashboard`, tapi ikut menyentuh Go.** Konsekuensi langsung dari keputusan
      katalog di bawah — `GET /v1/plans` (endpoint baru) dan `authz.ActionSubscriptionRead` (Action
      yang #124 sengaja tolak buat) keduanya lahir di issue ini. **Dicek ulang di #126**: pastikan
      `docs/phases/08.5-paid-plans/td.md` §9 mencerminkan bahwa perbandingan paket punya endpoint
      sendiri, bukan hanya `GET /v1/me`.

## Keputusan yang perlu dicek ulang

- [ ] **Perbandingan tiga paket diambil dari `GET /v1/plans` (katalog backend), bukan tabel statis
      TypeScript.** Tiga opsi diajukan ke pemilik produk sebelum implementasi; opsi ini dipilih karena
      satu-satunya yang tidak melanggar Phase 8 kriteria #6 / prd 8.5 kriteria #9. **Pemicu
      peninjauan: tidak ada** — ini bentuk yang benar secara arsitektur, bukan kompromi sementara.

- [ ] **Tombol Enterprise belum ada — hanya teks "Hubungi kami untuk diskusi harga".** Nomor
      WhatsApp/email kontak Enterprise masih placeholder (prd 8.5, tabel *Angka provisional*).
      **Pemicu peninjauan: begitu pemilik produk memberi kontak sebenarnya** — satu baris di
      `planDisplay` (atau field kontak baru) diisi, tombol/tautan dirender.

- [ ] **Label harga Pro masih `"Segera"`, bukan angka.** Menunggu keputusan pemilik produk yang sama
      dengan `LimitsAreProvisional` (`docs/issues/122`). **Pemicu peninjauan: sama dengan #122/#124**
      — angka provisional final diisi (kuota Free/Pro, batas seat, harga Pro sekaligus).

- [ ] **`useSessionRefresh` ditambahkan ke `session-context.tsx` — satu-satunya pemanggilnya adalah
      layar ini**, untuk menampilkan plan baru seketika setelah test checkout. **Pemicu peninjauan:
      fitur lain butuh me-refresh session tanpa navigasi** — kalau muncul, ini sudah jadi abstraksi
      yang tepat untuk dipakai bersama, bukan disalin.

## Verifikasi manual — prosedur untuk pemilik produk

Sama pola #114/#123: dijalankan terhadap `crm_be` sungguhan (`docker compose up` + `make migrate-up`),
bukan hanya mock. Login sebagai Owner satu organization uji.

1. **Turunkan kuota lead** — via token admin:
   ```
   curl -X POST http://localhost:8080/internal/subscriptions/{organization_id}/plan \
     -H "Authorization: Bearer $SUBSCRIPTION_ADMIN_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"plan_code": "free"}'
   ```
   lalu buat lead lewat dashboard sampai `leads_this_month` mencapai 100 (atau turunkan langsung lewat
   SQL: `UPDATE leads SET created_at = now() WHERE organization_id = '...' LIMIT 100` pada baris yang
   sudah ada). Coba buat satu lead lagi dari dialog **Lead baru** — konfirmasi:
   - [ ] Pesan `403 plan_quota_exceeded` tampil **di bawah field**, bukan banner generik tanpa arah
   - [ ] Tautan **"Lihat paket & pemakaian"** muncul, mengarah ke `/subscription`
2. **Penuhi batas seat** — undang anggota sampai `seats_used` mencapai batas paket (2 untuk Free),
   lalu coba undang satu lagi lewat dialog **Undang anggota** — konfirmasi:
   - [ ] Pesan `403 plan_seat_limit_reached` tampil inline dengan tautan yang sama
3. **Buka `/subscription`** sebagai Owner — konfirmasi:
   - [ ] Paket aktif menampilkan nama yang benar ("Free"/"Pro"/"Enterprise")
   - [ ] Pemakaian lead & anggota menunjukkan angka yang sesuai basis data
   - [ ] Tiga kolom perbandingan tampil terurut Free → Pro → Enterprise
   - [ ] Kolom paket aktif ditandai "Paket Anda"
4. **Login sebagai Manager atau Employee**, buka `/subscription` — konfirmasi:
   - [ ] "Langganan tidak tersedia untuk role Anda" tampil
   - [ ] **Nol** panggilan `GET /v1/plans` di tab Network browser
5. **Jika `SUBSCRIPTION_TEST_CHECKOUT=true`**, klik "Coba Pro (test)" sebagai Owner di kolom Pro —
   konfirmasi:
   - [ ] Paket aktif berubah menjadi "Pro" **tanpa reload halaman**
   - [ ] Tombol tidak lagi muncul di kolom Pro (sudah jadi paket aktif)
6. **Login sebagai Admin** (bukan Owner) di kolom Pro — konfirmasi:
   - [ ] Tombol "Coba Pro (test)" **tidak muncul** sama sekali, meski `test_checkout_available: true`

## Tidak ada deviasi TD/ADR lain

Gerbang role (`canViewSubscription`/`canChangePlan`), bentuk `GET /v1/plans`, dan larangan peta
paket→kapabilitas versi TypeScript mengikuti `td.md` §7/§9 apa adanya.
