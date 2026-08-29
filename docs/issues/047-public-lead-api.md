# Issue #47 — checklist penutupan phase

> Checklist ringkas, **bukan** catatan status. Status pekerjaan tetap hidup di GitHub Issues
> (ADR-008) — berkas ini hanya mengumpulkan poin yang perlu **dicek ulang saat #49** (penutup Phase 4)
> menutup phase-nya. Detail lengkap tiap poin ada di `docs/phases/04-public-api/notes.md` bagian `## #47`
> dan PR [#52](https://github.com/Pravasta/jualin-crm/pull/52).

## Deviasi dari teks issue / TD — sudah diverifikasi benar, catat alasannya di dokumentasi #49

- [x] **Empat endpoint non-`POST /v1/leads` menghasilkan `401`, bukan `403` seperti tertulis literal di
      checklist acceptance criteria issue #47.** Sudah diverifikasi **tidak** bertentangan dengan PRD
      Phase 4 — acceptance criterion PRD #5 hanya mensyaratkan *"API key tidak bisa memanggil satu pun
      endpoint aplikasi pengguna... karena otorisasi memang tidak punya jalan untuk mengizinkannya"*,
      tanpa menyebut kode status tertentu; `401` (ditolak di autentikasi, sebelum authz sama sekali
      dikonsultasi) justru bentuk yang **lebih kuat** dari "tidak punya jalan" dibanding `403`.
      **Diselesaikan di #49**: `authentication.md` bagian "API key" sekarang menyatakan `401` sebagai
      perilaku sesungguhnya secara eksplisit; halaman dokumentasi integrasi tidak menyebut `403` untuk
      endpoint selain `POST /v1/leads` sama sekali (tidak perlu — integrator yang mengikuti halaman itu
      tidak akan pernah memanggil endpoint lain).
- [x] **`CreateLeadInput` tidak mendapat field `SourceAPIKeyID` seperti tertulis di TD §5.**
      `source_api_key_id` diturunkan langsung dari `t.APIKeyID` di usecase — dicatat sebagai penyimpangan
      sadar (analog Aturan #5), bukan celah. **Ditinjau ulang di #49**, tetap berlaku; TD §19 (kewajiban
      Phase 6) sekarang menunjuk balik ke catatan ini supaya `source_form_id` tidak mengulang pola field
      yang sama di masa depan.

## Keputusan yang tetap dibawa maju — butuh traffic nyata untuk diputuskan, bukan bisa diselesaikan sekarang

Ketiga poin ini **sengaja dibiarkan terbuka** saat Phase 4 ditutup (#49) — bukan terlewat. Tidak ada
cara jujur menutupnya tanpa data traffic produksi sungguhan, yang belum ada.

> **Ditinjau ulang 29 Agustus 2026, sebelum Phase 5 dibuka.** Dua poin pertama tetap berlaku. Poin
> ketiga **tidak** — ia salah dikategorikan dan sebenarnya bug, bukan tuning. Koreksinya di poin itu
> sendiri. Pelajarannya dicatat di sini karena mudah terulang: *"pola kodenya sama"* bukan alasan yang
> cukup untuk menyimpulkan *"risikonya sama"* — yang menentukan adalah **siapa yang mengendalikan
> key-nya**.

- [ ] **`PUBLIC_API_RATE_LIMIT=60`/menit bukan hasil pengukuran** (TD §6, keputusan D4) — dua orde
      besaran di atas dugaan traffic SMB normal. Perlu ditinjau ulang begitu integrator sungguhan
      (bukan simulasi) mulai mengirim lead — bisa jadi terlalu longgar atau terlalu ketat.
- [ ] **Retensi `idempotency_key` 48 jam, penghapusan malas tanpa scheduler** — throttle 1×/organization/
      jam. Untuk organization dengan traffic API sangat tinggi (>1 request/jam terus-menerus), sweep ini
      akan berjalan sesering itu; belum pernah diuji di bawah volume produksi nyata.
- [x] ~~**`last_used_at` di-throttle 5 menit/kunci, peta in-memory tanpa eviction** — sama seperti
      `ratelimit.FixedWindow`'s bucket map (utang sejak #9).~~ **Dikoreksi 29 Agustus 2026 — poin ini
      salah dikategorikan.** Menyamakan `lastUsedThrottle` dengan `ratelimit.FixedWindow` menggabungkan
      dua hal yang berbeda kelas: yang pertama ber-key `api_key_id` (terbatas jumlah kunci, hanya
      bertambah lewat jalur terautentikasi), yang kedua ber-key email/IP — **input penyerang yang belum
      terautentikasi, tanpa batas**. Karena disamakan, keduanya ikut ditunda dengan alasan "butuh
      traffic produksi", padahal hanya yang pertama yang begitu.
      Menelusuri kesalahan ini menemukan bahwa **Aturan #34 tidak pernah benar-benar ditegakkan**
      (`ClientIP()` bisa dipalsukan lewat `X-Forwarded-For`). Keduanya sekarang ditangani terpisah di
      **Phase 4.5 — Hardening** (#57, #58): `FixedWindow` & `LoginLimiter` mendapat eviction;
      `lastUsedThrottle` **sengaja dibiarkan** (keputusan H3). Lihat `docs/phases/04.5-hardening/prd.md`.

## Bug ditemukan & sudah diperbaiki (tidak perlu tindakan lanjut, dicatat untuk arsip)

- [x] `leadJSON` tidak pernah menyertakan `source_api_key_id` di response — acceptance criterion PRD #1
      eksplisit menyebut field ini harus terisi. Ketahuan dari test HTTP pertama, diperbaiki sebelum PR
      #52 dibuka.
- [x] Fixture test (`internal/lead`, `cmd/api`) dengan `APIKeyID` acak menabrak `fk_leads_source_api_key`
      (composite FK sungguhan) → `500`. Kelas bug yang sama seperti #46's `fk_api_keys_created_by`.
      Diperbaiki dengan men-seed baris `api_keys` sungguhan sebelum PR #52 dibuka.

---

**Ditinjau saat penutupan Phase 4 (#49).** Dua bagian pertama selesai.

**Ditinjau ulang 29 Agustus 2026 sebelum Phase 5 dibuka** — dan pemeriksaan itu berbuah. Dari tiga poin
"dibawa maju", **dua tetap terbuka** (angka rate limit, retensi `idempotency_key` di volume tinggi;
keduanya betul-betul butuh integrator produksi nyata) dan **satu ternyata bug yang menyamar sebagai
tuning**, yang menelusurinya menemukan Aturan #34 tidak pernah ditegakkan → **Phase 4.5 — Hardening**
(#57, #58).

> Berkas ini bekerja sebagaimana mestinya. Poin yang ditunda dengan alasan salah tidak akan pernah
> ketahuan bila tidak ada tempat yang memaksa membacanya lagi.
