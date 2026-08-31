# Issue #71 — checklist penutupan phase

> Checklist ringkas, **bukan** catatan status. Status pekerjaan tetap hidup di GitHub Issues
> (ADR-008) — berkas ini hanya mengumpulkan poin yang perlu **dicek ulang saat #73** (penutup Phase 5)
> menutup phase-nya, supaya tidak ada yang terlewat di antara PR-PR yang sudah lama merge. Detail lengkap
> tiap poin ada di `docs/phases/05-employee-mobile/notes.md` bagian `## #71`.

## Keputusan yang perlu dicek ulang

- [ ] **Filter status bottom sheet multi-select dari mockup desain tidak dibangun.** Mockup Lead Saya
      menunjukkan dua bentuk berkontradiksi untuk filter status: chip horizontal single-select (state
      "Normal") dan bottom sheet checkbox multi-select (state "Filter status") — tidak jelas triggernya
      apa, dan keduanya tidak konsisten satu sama lain. Dipilih chip horizontal saja (backend `status`
      query param tetap menerima CSV kalau nanti multi-select benar-benar dibutuhkan). Tinjau ulang bila
      pemakaian nyata menunjukkan single-select terasa kurang.
- [ ] **Tidak ada paginasi UI** — hanya halaman pertama `GET /v1/leads` yang dimuat. `meta.total`
      tersimpan di `LeadListResult` tapi tidak ditampilkan atau dipakai untuk scroll-tak-berhingga. Untuk
      Employee dengan lead melebihi satu halaman, lead lama tidak akan terlihat. Tinjau ulang begitu ada
      Employee dengan volume lead tinggi di penggunaan nyata.
- [ ] **Field pencarian tidak punya rujukan mockup** — tidak satu pun state desain Lead Saya
      menampilkan kotak pencarian, meski cakupan issue #71 sendiri memintanya. Dibangun mengikuti token
      tema tanpa acuan visual langsung. Tinjau ulang saat hasil desain berikutnya (jika ada) benar-benar
      menunjukkan bentuknya.

## Temuan — dicatat untuk arsip, sudah selesai

- **`ApiClient.sendListEnvelope` ditambahkan** — `send()` (dari #69) membuang `meta`, cukup untuk
  endpoint auth tapi tidak cukup untuk My Leads yang butuh `meta.total`. Ditemukan sebelum sempat jadi
  bug diam-diam (total selalu salah setelah baca dari cache).
- **`runApiCall()` diekstrak dari `AuthRepositoryImpl`** ke `core/network/` — dua implementasi nyata
  (auth, leads) butuh pemetaan `ApiError`/`SessionExpiredException` → `Failure` yang identik (Aturan
  #28). `AuthRepositoryImpl` ditulis ulang memakainya, perilaku tidak berubah (dibuktikan test lama
  tetap lolos).
- **`AuthBloc` mendapat event `AuthSessionInvalidated`** — fitur di luar auth (My Leads adalah yang
  pertama) sekarang punya jalan memberi tahu "sesi berakhir" tanpa `AuthBloc` perlu tahu fitur itu ada.

---

**Ditinjau 31 Agustus 2026 di #98, bukan di #73 seperti direncanakan** (alasan sama seperti `069`).

**Hasil — ketiganya tetap terbuka; semuanya menunggu pemakaian nyata yang belum pernah terjadi
(aplikasi belum pernah dipakai di HP sungguhan oleh pengguna sungguhan).** Satu dinaikkan
kejelasannya:

- **Tanpa paginasi** adalah yang paling berisiko dari ketiganya, dan pemicunya bisa dinyatakan
  konkret alih-alih "volume tinggi": `GET /v1/leads` memakai `per_page` default **25**
  (`meta.per_page`, dibuktikan di verifikasi #89). Jadi ambangnya bukan abstrak — **Employee dengan
  lebih dari 25 lead ter-assign tidak akan pernah melihat yang ke-26 dan seterusnya**, tanpa pesan
  apa pun bahwa ada yang disembunyikan. Itu angka yang mudah dicapai satu tenaga penjualan aktif dalam
  hitungan minggu. Tetap tidak diperbaiki di #98 (issue ini memutuskan, bukan mengimplementasi), tapi
  kalau salah satu dari ketiga poin ini naik jadi issue lebih dulu, ini yang pertama.
- Filter multi-select dan kotak pencarian tanpa mockup: keduanya menunggu masukan desain/pemakaian,
  tidak ada yang bisa diputuskan lebih jujur sekarang.
