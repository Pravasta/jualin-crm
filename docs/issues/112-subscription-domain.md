# Issue #112 — checklist penutupan phase

> Checklist ringkas, **bukan** catatan status. Status pekerjaan tetap hidup di GitHub Issues (ADR-008) —
> berkas ini mengumpulkan poin yang perlu **dicek ulang saat #115** menutup Phase 8. Detail lengkap ada
> di `docs/phases/08-subscription/notes.md` bagian `## #112`.
>
> **Ditinjau di #115 (5 September 2026).** Status tiap poin di bawah. Ditulis retroaktif bersamaan
> dengan peninjauannya sendiri — #112 tidak menulis berkas ini saat merge, celah yang tertangkap saat
> #115 mengecek `docs/issues/*` Phase 8 dan menemukan nol berkas untuk tiga issue yang sudah selesai.

## Keputusan yang tidak eksplisit ditulis TD

- [ ] **`FindActiveByOrg` bisa mengembalikan nol baris — cabang yang belum pernah tersentuh data
      produksi.** `ErrNoActiveSubscription` diterjemahkan `ResolvePlan` jadi kode kosong + seluruh
      kanal tertutup (fail closed), bukan error. Hari ini tidak pernah terjadi: `CreateFree` selalu
      menulis `active`, dan tidak ada jalur yang mengubahnya. **Pemicu peninjauan: saat payment
      service mulai menulis `past_due`/`suspended`/`canceled`** — pastikan perilaku "kode kosong,
      semua kanal tertutup" masih yang diinginkan produk saat itu benar-benar terjadi, bukan cuma
      teori. Detail: `notes.md` `## #112`, `internal/subscription/errors.go`.

- [ ] **`plan.code` dikirim di `GET /v1/me` tapi tidak ditampilkan di layar mana pun.** PRD kriteria
      #3/D5 menulis kode itu "untuk ditampilkan (mis. 'Paket: Free')", tapi #114 (dashboard) tidak
      menambahkan tampilan apa pun untuknya — hanya `channels` yang dibaca. **Pemicu peninjauan: saat
      paket kedua lahir** — di situ menampilkan nama paket ("Free" vs "Pro") baru jadi informasi yang
      berarti bagi pemilik toko; hari ini hanya ada satu paket, jadi menampilkannya adalah informasi
      kosong (Aturan #27/#29).

## Tidak ada deviasi TD/ADR lain

`planChannels`, `channelsFor`, `ParseChannel`, dan bentuk `Repository`/`Usecase` mengikuti
`td.md` §1–§7 apa adanya — tidak ada penyimpangan yang perlu dicatat di luar dua poin di atas.
