# Issue #46 — checklist penutupan phase

> Checklist ringkas, **bukan** catatan status. Status pekerjaan tetap hidup di GitHub Issues
> (ADR-008) — berkas ini hanya mengumpulkan poin yang perlu **dicek ulang saat #49** (penutup Phase 4)
> menutup phase-nya, supaya tidak ada yang terlewat di antara PR-PR yang sudah lama merge. Detail lengkap
> tiap poin ada di `docs/phases/04-public-api/notes.md` bagian `## #46` dan PR
> [#51](https://github.com/Pravasta/jualin-crm/pull/51).

## Deviasi dari TD / ADR

- [ ] **ADR-004 masih menampilkan angka yang salah di dokumennya sendiri.** "secret: 32char" vs "entropi
      256-bit" tidak bisa benar bersamaan (32 karakter base64url ≈ 192 bit). Kode mengambil 256-bit (`entity.go`,
      `keyIDRawBytes`/`secretRawBytes`) dan mendokumentasikan alasannya di komentar — tapi **ADR-004
      sendiri belum diperbaiki**. Perlu diputuskan saat #49: revisi ADR-004 langsung, atau tambahkan
      catatan errata yang menunjuk ke kode sebagai sumber kebenaran.
- [ ] **ADR-004 "key_prefix 8 karakter pertama" tidak cocok dengan contohnya sendiri.** Contoh di ADR
      (`jln_live_a3f9…`) menunjukkan 4 karakter pertama `key_id`, bukan 8. Kode mengikuti contohnya
      (benar), teksnya belum diperbaiki. Sama seperti poin di atas — revisi atau errata.

## Keputusan yang perlu dicek ulang

- [ ] **Manager & Employee tidak punya akses sama sekali** ke `api_key.*` (bukan read-only seperti
      `membership.list`). Ini keputusan sadar (TD §9), bukan bug — pastikan masih relevan saat #49
      mengecek matriks RBAC final terhadap `authorization.md`.
- [ ] `expires_at` tetap `NULL` selamanya di seluruh Phase 4 — kolomnya ada, isinya tidak pernah diisi.
      Pastikan halaman dokumentasi integrasi (#49) tidak menyiratkan fitur masa berlaku yang belum ada.

## Bug ditemukan & sudah diperbaiki (tidak perlu tindakan lanjut, dicatat untuk arsip)

- [x] Fixture test dengan `membership_id` acak menabrak `fk_api_keys_created_by` (composite FK
      sungguhan) → `500`. Diperbaiki dengan men-seed membership asli sebelum PR #51 dibuka.
- [x] Test tabel per-role (`authz_test.go`) tidak pernah mendapat 3 baris `api_key.create/list/revoke`
      saat aksinya ditambahkan — ketahuan & dibackfill di #47 (PR #52), bukan di sini.
