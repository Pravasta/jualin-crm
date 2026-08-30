# Issue #72 — checklist penutupan phase

> Checklist ringkas, **bukan** catatan status. Status pekerjaan tetap hidup di GitHub Issues
> (ADR-008) — berkas ini hanya mengumpulkan poin yang perlu **dicek ulang saat #73** (penutup Phase 5)
> menutup phase-nya. Detail lengkap tiap poin ada di `docs/phases/05-employee-mobile/notes.md` bagian
> `## #72`.

## Keputusan yang perlu dicek ulang

- [ ] **"Anggota tim lain" — istilah generik untuk aktor timeline yang bukan diri sendiri.** Employee
      tidak punya `ActionMembershipList`, jadi `actor_membership_id` tidak pernah bisa di-resolve ke
      nama. Mockup Claude Design hanya mengonfirmasi kasus diri-sendiri ("Ditugaskan ke Anda") — kasus
      "orang lain" tidak punya rujukan visual langsung, istilah "Anggota tim lain" dipilih sendiri.
      Tinjau ulang begitu ada mockup yang menunjukkan bentuk sebenarnya.
- [ ] **Kegagalan muat awal Detail Lead selalu layar error penuh, tidak pernah tampilan sebagian.**
      Bila `GET /v1/leads/{id}` sukses tapi `GET .../activities` gagal (atau sebaliknya), seluruh layar
      menampilkan `LeadDetailError`, bukan lead yang tampil dengan timeline kosong/error kecil. Pilihan
      sadar untuk kesederhanaan (Aturan #27) — tinjau ulang bila kegagalan parsial ternyata sering
      terjadi di pemakaian nyata dan terasa lebih mengganggu daripada tampilan sebagian.
- [ ] **Aksi WhatsApp/telepon yang dibatalkan sebelum handoff OS tidak menampilkan pesan apa pun** —
      hanya activity yang tidak dicatat (sesuai desain), tanpa toast "dibatalkan". Diasumsikan
      pembatalan adalah pilihan sadar pengguna yang tidak butuh konfirmasi tambahan. Tinjau ulang bila
      pengguna nyata bingung kenapa tidak ada activity baru setelah menekan tombol.

## Temuan — dicatat untuk arsip, sudah selesai

- **`LeadDetailResult` ditambahkan** — `LeadRepository.getLeadDetail` semula membuang `fromCache`/
  `fetchedAt` yang `cachedGet` sudah hitung, padahal design brief §10 minta banner cache di Detail Lead
  juga. Ditemukan sebelum dipakai di UI.
- **`LeadDetailError` diberi `leadId`** — tanpa itu, tombol "Coba lagi" setelah kegagalan refresh (dari
  state `Loaded` yang lead-nya sudah hilang begitu error) tidak tahu lead mana yang harus dimuat ulang.
  Ditemukan saat menulis `_ErrorView`.
- **`noteError` dipindah dari snackbar ke `errorText` inline** — draf pertama menyamakan perlakuannya
  dengan `statusError`/`externalActionError` (toast), padahal design brief §10 eksplisit minta
  kesalahan per-field untuk form catatan. Dikoreksi sebelum PR dibuka.
- **`CacheBanner` diekstrak ke `shared/widgets/`** — implementasi kedua yang nyata (Lead Saya #71,
  Detail Lead #72), Aturan #28.

---

**Belum ditinjau** — menunggu penutupan Phase 5 (#73).
