# Issue #122 — checklist penutupan phase

> Checklist ringkas, **bukan** catatan status. Status pekerjaan tetap hidup di GitHub Issues (ADR-008) —
> berkas ini mengumpulkan poin yang perlu **dicek ulang saat #126** menutup Phase 8.5. Detail lengkap ada
> di `docs/phases/08.5-paid-plans/notes.md` bagian `## #122`.

> **Ditinjau di #126 (5 September 2026).** Dua poin di bawah **ditutup**; tiga sisanya dinyatakan
> ulang dengan pemicu eksplisit. Tidak ada yang ditinggalkan tanpa pemicu.

## Deviasi dari TD — ✅ keduanya ditutup di #126

- [x] **Penjaga angka provisional diganti bentuknya: boot gagal, bukan test merah.** TD §14 menulis
      "test yang mengunci angka sudah diisi gagal"; diganti `subscription.LimitsAreProvisional` +
      penjaga `APP_ENV=production` di composition root. Alasan: CI merah selama empat issue melatih
      orang mengabaikan merah.
      **→ Ditutup #126:** konstanta jadi `false` bersamaan dengan angka sungguhan, dan **TD §14
      dikoreksi di tempat** (dengan callout ⚠️, bukan dihapus) supaya pembaca berikutnya tahu
      mekanismenya berbeda dari yang dijanjikan dan kenapa. Dua test hijau menggantikan test merah
      yang dijanjikan: `TestLimitsAreNoLongerProvisional`, `TestPlanDisplay_NoPlaceholderPriceLabels`
      — keduanya **dibuktikan bisa gagal** (dibalik sementara, merah, dikembalikan, `git diff` kosong).

- [x] **`planChannels` untuk `pro`/`enterprise` sama dengan `free`** (ketiga kanal terbuka). Menutup
      kanal yang `free` sudah buka = downgrade terhadap organization yang sudah memakainya.
      **→ Ditutup #126:** pemilik produk memutuskan kanal **tidak** membedakan paket — pembedanya
      kuota dan seat. Ini sekarang jawaban final, bukan keadaan sementara; komentar di `plan.go`
      diperbarui dari "menunggu keputusan" jadi "sudah diputuskan". Kanal **keempat**, kalau kelak
      dibangun, bisa lahir Pro-only sejak hari pertama tanpa mengambil apa pun dari siapa pun — itu
      bentuk yang harus dituju, bukan mengambil kembali salah satu dari tiga yang ada.

## Keputusan yang perlu dicek ulang

- [x] **Aritmetika seat terduplikasi di dua tempat — diverifikasi konsisten di #124.** `seats_used`
      yang ditampilkan (`auth.Me`) dan yang ditegakkan (`invitation.Usecase.Create`) sama-sama
      menjumlahkan `membership.CountActive` + `invitation.CountPendingSeats`. **→ Ditinjau #124/#126:**
      keduanya cocok, dan #124 sekalian menutup celah test yang membuat ini rapuh — kedua penghitung
      itu sebelumnya **tidak punya test repository sama sekali**; sekarang punya, termasuk isolasi
      tenant. Helper bersama **sengaja tidak dibuat**: dua pemanggil dengan dua alasan berbeda
      (menampilkan vs menolak) belum cukup untuk sebuah abstraksi (Aturan #28). **Pemicu peninjauan:
      pemanggil ketiga muncul.**

- [ ] **`COUNT` kuota tidak bisa memakai index apa pun.** Keempat index `leads` parsial
      (`WHERE deleted_at IS NULL`); penghitung kuota justru harus menghitung yang soft-deleted.
      `EXPLAIN` dijalankan #122: **1,76 ms / 58 buffer hit pada 4.000 baris** (2.000 milik org uji),
      seluruhnya dari cache → diputuskan **biarkan, tanpa index baru**. **Pemicu peninjauan: satu
      organization mendekati puluhan ribu lead per bulan.** Jalan keluarnya satu index non-parsial
      `(organization_id, created_at)` — satu migration kecil, bukan perubahan desain.

- [ ] **Tiga `COUNT` per panggilan `GET /v1/me`**, dipanggil `SessionGate` di setiap layar
      terproteksi. Belum terukur mahal (angka di atas untuk satu di antaranya). **Pemicu peninjauan:
      latensi `/v1/me` terlihat di produksi.** Jalan keluar yang sudah ditulis TD §7: `limits` tetap
      di `/v1/me`, `usage` pindah ke endpoint layar Langganan.

## Tidak ada deviasi TD/ADR lain

Bentuk `Limits`, `limitsFor` yang gagal-tertutup ke `free` (bukan nol), dan `allows()` sebagai
satu-satunya pembaca arti `0` mengikuti `td.md` §2, §2.1, §7 apa adanya.
