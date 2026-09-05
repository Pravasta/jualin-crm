# Issue #123 — checklist penutupan phase

> Checklist ringkas, **bukan** catatan status. Status pekerjaan tetap hidup di GitHub Issues (ADR-008) —
> berkas ini mengumpulkan poin yang perlu **dicek ulang saat #126** menutup Phase 8.5. Detail lengkap ada
> di `docs/phases/08.5-paid-plans/notes.md` bagian `## #123`.

> **Ditinjau di #126 (5 September 2026).** Satu poin ditutup; tiga sisanya dinyatakan ulang dengan
> pemicu eksplisit.

## Deviasi dari TD

- [x] **TD §1 salah — phase ini butuh migration.** Klaim "tanpa migration sama sekali" ditulis sebelum
      diperiksa bahwa `notifications.type` punya `CHECK` tertutup. **`migrations/0010`** menambah
      `plan_quota_exceeded` ke daftar yang diizinkan. `td.md` §1 dikoreksi di tempat, bukan dihapus.
      **→ Ditutup #126 sebagai poin #123, tapi pemeriksaannya menemukan hal lain** — lihat di bawah.

## ✅ Temuan #126 — `freeze.md` 8.4 tertinggal tiga migration (ditutup, follow-up #126)

- [x] Memeriksa konsistensi `td.md` §1 dengan `freeze.md` 8.4 (yang poin di atas minta) menemukan
      masalah yang **lebih besar dan bukan milik Phase 8.5**: tabel *Migration setelahnya* di
      `freeze.md` 8.4 berhenti di **`0007_forms` (Phase 6)**. Tiga migration yang sudah ada di
      repository tidak tercatat di sana:

      | Migration | Phase | Sejak |
      |---|---|---|
      | `0008_webhooks` | 7 | issue #100 |
      | `0009_webhook_secret_encrypted` | 7 | issue #101 |
      | `0010_notification_plan_quota` | 8.5 | issue #123 |

      **Tidak diperbaiki di PR ini, dan itu keputusan sadar.** `freeze.md` adalah dokumen beku —
      CLAUDE.md: *"Perubahan arsitektur hanya melalui ADR baru. Freeze tidak diubah tanpa catatan."*
      Menambal tabelnya diam-diam di dalam PR penutup phase adalah persis kebiasaan yang aturan itu
      cegah. **Dilaporkan, bukan diperbaiki sepihak** (Aturan #30).

      **→ Diputuskan pemilik produk 5 September 2026: tabel itu adalah rekaman rencana Phase 0–6.**
      Satu kalimat ditambahkan di atas tabelnya yang menyatakan ia berhenti di `0007_forms` dan bahwa
      daftar sesungguhnya adalah isi `crm_be/migrations/` — menyebut ketiga migration yang menyusul.
      Dipilih di atas "jadikan daftar hidup" justru karena selisih ini sudah terjadi **tiga kali**:
      dokumen yang wajib diperbarui tiap phase adalah dokumen yang akan menyimpang lagi. Sekarang
      hanya ada **satu** sumber kebenaran untuk daftar migration, dan `freeze.md` menunjuk ke sana.

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
