# Issue #114 — checklist penutupan phase

> Checklist ringkas, **bukan** catatan status. Status pekerjaan tetap hidup di GitHub Issues (ADR-008) —
> berkas ini mengumpulkan poin yang perlu **dicek ulang saat #115** menutup Phase 8. Detail lengkap ada
> di `docs/phases/08-subscription/notes.md` bagian `## #114`.
>
> **Ditinjau di #115 (5 September 2026).** Status tiap poin di bawah.

## Keputusan yang dikonfirmasi eksplisit sebelum implementasi

- [x] **Layar tujuan (`/connect/api`, `/connect/form`, `/connect/webhook`) sengaja tidak digerbangi
      paket.** Tombol "Buat" tetap tampil apa pun `plan.channels`-nya; hanya balapan (`403
      plan_upgrade_required` dari klik setelah paket berubah) yang ditangani, sesuai TD §8 apa adanya.
      Satu-satunya jalan sampai ke layar tertutup adalah mengetik URL langsung — di situ backend
      (#113) yang menolak, penegakan sungguhan (ADR-012 §3). **Bukan celah** — diputuskan eksplisit
      sebelum implementasi, bukan luput. Tidak ada pemicu peninjauan: ini bentuk yang diinginkan
      selama belum ada jalur downgrade (D4 di TD Phase 8).

## Verifikasi manual — dijalankan

- [x] **Verifikasi browser AC #7 — kartu terkunci sungguhan, `planChannels` dibalik.** Keputusan
      eksplisit sebelum implementasi #114: diserahkan ke pemilik produk untuk dijalankan sendiri
      setelah PR naik, bukan dijalankan sesi itu. Prosedurnya ditulis di
      `docs/testing/flow/09-webhook.md` §9.10 (ditambahkan #115). **Dijalankan pemilik produk
      5 September 2026** — dilaporkan lewat sesi kerja, bukan diamati agent. Poin ini ditutup.

## Tidak ada deviasi TD/ADR lain

`lib/plan.ts`, bentuk kartu, dan penanganan balapan mengikuti `td.md` §8, §4 apa adanya.
