# Issue #124 — checklist penutupan phase

> Checklist ringkas, **bukan** catatan status. Status pekerjaan tetap hidup di GitHub Issues (ADR-008) —
> berkas ini mengumpulkan poin yang perlu **dicek ulang saat #126** menutup Phase 8.5. Detail lengkap ada
> di `docs/phases/08.5-paid-plans/notes.md` bagian `## #124`.

> **Ditinjau di #126 (5 September 2026).** Satu poin ditutup; sisanya dinyatakan ulang dengan pemicu
> eksplisit.

## Deviasi dari issue (bukan hanya TD) — ✅ ditutup di #125

- [x] **`Action` `subscription.read` tidak dibuat.** Issue menulis dua Action baru
      (`subscription.change` Owner, `subscription.read` Owner+Admin); hanya yang pertama dibuat —
      `subscription.read` tidak punya pemanggil nyata saat itu.
      **→ Ditutup:** #125 ternyata **memang** butuh endpoint terpisah — bukan untuk paket organization
      sendiri (`GET /v1/me` tetap cukup untuk itu, seperti dugaan #124), melainkan untuk **katalog
      paket lain** (`GET /v1/plans`, perbandingan tiga paket). `subscription.read` dibuat di #125 dan
      menggerbangi endpoint itu, Owner+Admin. Urutannya persis seperti yang direncanakan: Action lahir
      bersama pemanggil pertamanya, bukan mendahuluinya. Dicatat juga di `authorization.md`.

## Pemeriksaan yang diminta issue, hasilnya negatif

- [ ] **Jalur reaktivasi membership: tidak ada.** Issue meminta diperiksa eksplisit, bukan diasumsikan.
      Dikonfirmasi lewat pencarian menyeluruh (`grep -rn "[Rr]eactivat" internal/membership
      internal/invitation`) — nol hasil, **diperiksa ulang di #126, masih nol**. Titik pasang batas
      seat tetap tunggal (`invitation.Usecase.Create`). **Pemicu: membership mendapat jalur reaktivasi**
      — saat itu titik pasang kedua wajib ditambahkan di situ, atau organization bisa melewati batas
      seat dengan menonaktifkan lalu menghidupkan kembali anggota.

## Keputusan yang perlu dicek ulang

- [ ] **Perubahan `plan_code` dan penulisan `audit_logs` tidak atomik** (dua statement terpisah, tanpa
      transaksi bersama) — disengaja, Aturan #27, untuk aksi admin yang jarang dan token-gated.
      **Pemicu peninjauan: baris audit yang hilang benar-benar terjadi di produksi** (mis. pool
      kehabisan koneksi tepat di antara dua write) — kalau itu terjadi berulang, baru sepadan
      membungkus `internal/subscription` dengan `Store`/`InTx` penuh.

- [ ] **Test checkout mengunci tujuan ke `subscription.PlanPro`, bukan menerima `plan_code` bebas dari
      body.** Sesuai maksud tombolnya ("coba Pro"), tapi berarti test checkout tidak bisa dipakai
      mencoba paket Enterprise. **Pemicu peninjauan: kebutuhan uji-coba Enterprise sebelum pembayaran
      sungguhan ada** — kemungkinan besar tidak relevan karena Enterprise memang diarahkan ke kontak
      manual (prd), bukan self-serve.

## Tidak ada deviasi TD/ADR lain

Bentuk `PlanSeatQuota`, urutan gerbang (`authz` → batas seat → validasi role undangan), pola
"route tidak terdaftar saat nonaktif", dan guard boot `SUBSCRIPTION_TEST_CHECKOUT` mengikuti `td.md`
apa adanya.
