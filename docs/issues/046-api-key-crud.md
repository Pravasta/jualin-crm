# Issue #46 — checklist penutupan phase

> Checklist ringkas, **bukan** catatan status. Status pekerjaan tetap hidup di GitHub Issues
> (ADR-008) — berkas ini hanya mengumpulkan poin yang perlu **dicek ulang saat #49** (penutup Phase 4)
> menutup phase-nya, supaya tidak ada yang terlewat di antara PR-PR yang sudah lama merge. Detail lengkap
> tiap poin ada di `docs/phases/04-public-api/notes.md` bagian `## #46` dan PR
> [#51](https://github.com/Pravasta/jualin-crm/pull/51).

## Deviasi dari TD / ADR

- [x] **ADR-004 masih menampilkan angka yang salah di dokumennya sendiri.** "secret: 32char" vs "entropi
      256-bit" tidak bisa benar bersamaan (32 karakter base64url ≈ 192 bit). **Diperbaiki di #49** —
      ADR-004 direvisi langsung mengikuti kode (`<secret:43char>`), dengan catatan koreksi di badan ADR
      itu sendiri (Aturan #30: dicatat, bukan didiamkan).
- [x] **ADR-004 "key_prefix 8 karakter pertama" tidak cocok dengan contohnya sendiri.** **Diperbaiki di
      #49** — teks direvisi jadi "4 karakter pertama `key_id`", cocok dengan contoh (`jln_live_a3f9…`)
      dan kode.

## Keputusan yang perlu dicek ulang

- [x] **Manager & Employee tidak punya akses sama sekali** ke `api_key.*` (bukan read-only seperti
      `membership.list`). **Ditinjau ulang di #49** terhadap `authorization.md`'s Matriks (Phase 4) yang
      baru ditulis — masih tepat, tidak berubah.
- [x] `expires_at` tetap `NULL` selamanya di seluruh Phase 4 — kolomnya ada, isinya tidak pernah diisi.
      **Ditinjau ulang di #49** — halaman dokumentasi integrasi (`/settings/api-keys/docs`) tidak
      menyebut masa berlaku sama sekali, tidak ada yang perlu diperbaiki.

## Bug ditemukan & sudah diperbaiki (tidak perlu tindakan lanjut, dicatat untuk arsip)

- [x] Fixture test dengan `membership_id` acak menabrak `fk_api_keys_created_by` (composite FK
      sungguhan) → `500`. Diperbaiki dengan men-seed membership asli sebelum PR #51 dibuka.
- [x] Test tabel per-role (`authz_test.go`) tidak pernah mendapat 3 baris `api_key.create/list/revoke`
      saat aksinya ditambahkan — ketahuan & dibackfill di #47 (PR #52), bukan di sini.

---

**Ditinjau penuh saat penutupan Phase 4 (#49).** Seluruh poin di atas sudah diputuskan — tidak ada yang
menunggu keputusan lagi dari berkas ini.
