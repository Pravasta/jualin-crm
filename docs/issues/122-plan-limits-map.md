# Issue #122 — checklist penutupan phase

> Checklist ringkas, **bukan** catatan status. Status pekerjaan tetap hidup di GitHub Issues (ADR-008) —
> berkas ini mengumpulkan poin yang perlu **dicek ulang saat #126** menutup Phase 8.5. Detail lengkap ada
> di `docs/phases/08.5-paid-plans/notes.md` bagian `## #122`.

## Deviasi dari TD

- [ ] **Penjaga angka provisional diganti bentuknya: boot gagal, bukan test merah.** TD §14 menulis
      "test yang mengunci angka sudah diisi gagal"; diganti `subscription.LimitsAreProvisional` +
      penjaga `APP_ENV=production` di composition root. Alasan: CI merah selama empat issue melatih
      orang mengabaikan merah. **Dicek ulang di #126**: konstanta wajib jadi `false` bersamaan dengan
      angka sungguhan, dan TD §14 diperbarui supaya tidak menyesatkan pembaca berikutnya.

- [ ] **`planChannels` untuk `pro`/`enterprise` sama dengan `free`** (ketiga kanal terbuka). Menutup
      kanal yang `free` sudah buka = downgrade terhadap organization yang sudah memakainya.
      **Dicek ulang saat angka provisional diisi**: kalau pembeda paket ternyata mencakup kanal (bukan
      hanya kuota), keputusan downgrade untuk organization yang sudah ada harus dijawab lebih dulu —
      Phase 8 D4 belum pernah menjawabnya.

## Keputusan yang perlu dicek ulang

- [ ] **`COUNT` kuota tidak bisa memakai index apa pun.** Keempat index `leads` parsial
      (`WHERE deleted_at IS NULL`); penghitung kuota justru harus menghitung yang soft-deleted.
      `EXPLAIN` dijalankan #122: **1,76 ms / 58 buffer hit pada 4.000 baris** (2.000 milik org uji),
      seluruhnya dari cache → diputuskan **biarkan, tanpa index baru**. **Pemicu peninjauan: satu
      organization mendekati puluhan ribu lead per bulan.** Jalan keluarnya satu index non-parsial
      `(organization_id, created_at)` — satu migration kecil, bukan perubahan desain.

- [ ] **Aritmetika seat terduplikasi di dua tempat.** `seats_used` yang ditampilkan (`auth.Me`) dan
      yang ditegakkan (#124) sama-sama menjumlahkan `membership.CountActive` +
      `invitation.CountPendingSeats`. Disengaja — angka yang ditampilkan harus sama dengan angka yang
      menolak — tapi tidak ada yang memaksa keduanya tetap sama. **Dicek ulang di #124**: kalau
      penjumlahannya berbeda, itu bug yang terlihat seperti keanehan produk. Pertimbangkan satu helper
      bersama bila #124 menemukan tempat yang wajar untuknya.

- [ ] **Tiga `COUNT` per panggilan `GET /v1/me`**, dipanggil `SessionGate` di setiap layar
      terproteksi. Belum terukur mahal (angka di atas untuk satu di antaranya). **Pemicu peninjauan:
      latensi `/v1/me` terlihat di produksi.** Jalan keluar yang sudah ditulis TD §7: `limits` tetap
      di `/v1/me`, `usage` pindah ke endpoint layar Langganan.

## Tidak ada deviasi TD/ADR lain

Bentuk `Limits`, `limitsFor` yang gagal-tertutup ke `free` (bukan nol), dan `allows()` sebagai
satu-satunya pembaca arti `0` mengikuti `td.md` §2, §2.1, §7 apa adanya.
