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

**Belum ditinjau** — menunggu penutupan Phase 5 (#73).
