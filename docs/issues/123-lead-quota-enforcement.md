# Issue #123 — checklist penutupan phase

> Checklist ringkas, **bukan** catatan status. Status pekerjaan tetap hidup di GitHub Issues (ADR-008) —
> berkas ini mengumpulkan poin yang perlu **dicek ulang saat #126** menutup Phase 8.5. Detail lengkap ada
> di `docs/phases/08.5-paid-plans/notes.md` bagian `## #123`.

## Deviasi dari TD

- [ ] **TD §1 salah — phase ini butuh migration.** Klaim "tanpa migration sama sekali" ditulis sebelum
      diperiksa bahwa `notifications.type` punya `CHECK` tertutup. **`migrations/0010`** menambah
      `plan_quota_exceeded` ke daftar yang diizinkan. `td.md` §1 dikoreksi di tempat, bukan dihapus.
      **Dicek ulang di #126**: pastikan `td.md` §1 yang sudah dikoreksi ini konsisten dengan
      `docs/architecture/freeze.md` bagian 8.4 (migration terbaru yang tercatat di sana).

## Keputusan yang perlu dicek ulang

- [ ] **Notifikasi hanya untuk jalur `public_form`.** `user`/`api_key` yang ditolak `403
      plan_quota_exceeded` tidak menerima notifikasi in-app kedua — `403`-nya sendiri sudah jadi
      pemberitahuan. Keputusan ini **tidak eksplisit ditulis TD §5**, disimpulkan saat implementasi.
      **Pemicu peninjauan: keluhan pengguna bahwa mereka tidak sadar kuotanya habis** (mis. integrator
      yang memantau lewat webhook, bukan UI) — kalau itu terjadi, `403` saja tidak cukup terlihat.

- [ ] **Ambang "sudah diberi tahu bulan ini" organization-wide, bukan per-Owner.** Co-owner berbagi
      satu ambang; Owner kedua yang ditambahkan pertengahan bulan tidak mendapat notifikasinya sendiri
      kalau Owner pertama sudah diberi tahu. **Pemicu peninjauan: organization dengan >1 Owner
      melaporkan salah satu tidak pernah melihat notifikasi kuota** — kemungkinan besar bukan bug,
      tapi perlu dikonfirmasi sebagai bentuk yang diinginkan, bukan diasumsikan.

- [ ] **Kuota terlampaui 1–2 baris di bawah konkurensi diterima secara sadar** (prd D2) — tidak ada
      lock di jalur pembuatan lead. **Pemicu peninjauan: tidak ada** — ini keputusan permanen kecuali
      biayanya berubah drastis (mis. pelanggan mulai mengeksploitasi race untuk melewati kuota jauh
      lebih dari 1–2 baris, yang butuh volume request bersamaan tidak wajar).

## Tidak ada deviasi TD/ADR lain

Urutan gerbang (`authz` → hitung → kuota), bentuk `PlanQuota`/`QuotaNotifier`, dan perilaku
per-principal (§5) mengikuti `td.md` apa adanya setelah D3 ditutup.
