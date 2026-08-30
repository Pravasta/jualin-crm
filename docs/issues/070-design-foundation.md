# Issue #70 — checklist penutupan phase

> Checklist ringkas, **bukan** catatan status. Status pekerjaan tetap hidup di GitHub Issues
> (ADR-008) — berkas ini hanya mengumpulkan poin yang perlu **dicek ulang saat #73** (penutup Phase 5)
> menutup phase-nya, supaya tidak ada yang terlewat di antara PR-PR yang sudah lama merge. Detail lengkap
> tiap poin ada di `docs/phases/05-employee-mobile/notes.md` bagian `## #70`.

## Keputusan yang perlu dicek ulang

- [ ] **State "backoff" login (percobaan berlebih) tidak punya hitung mundur langsung** seperti
      digambar desain ("Coba lagi dalam 04:37") — butuh `Retry-After` mengalir dari rate limiter
      `crm_be` sampai `ApiClient`/`ApiError`, keduanya belum pernah membaca header respons. Kegagalan
      rate-limit tetap tampil (pesan asli backend, banner bertema), hanya tanpa jam berjalan. Tinjau
      ulang saat #73 (atau lebih awal bila UX-nya terasa kurang saat dipakai sungguhan): apakah worth
      menambah dukungan header di `ApiClient`, atau pesan statis sudah cukup untuk MVP.
- [ ] **Gerbang biometric tidak menampilkan nama/organization pengguna** seperti di desain — aplikasi
      tidak menyimpan profil pengguna secara lokal. Tinjau ulang bila #71/#72 pernah membangun cache
      profil untuk alasan lain (misalnya untuk header offline) — kalau iya, pertimbangkan dipakai juga
      di sini.

## Temuan — kontras diverifikasi ulang, tiga angka desain sendiri meleset

Bukan yang perlu ditinjau ulang (sudah final, seluruhnya lolos AA) — dicatat untuk arsip:
`--primary` (klaim desain 4.83:1, dihitung ulang 5.05:1), `--warning` di atas tint-nya (klaim 7.6:1,
dihitung ulang 6.65:1), `--success` di atas tint-nya (klaim 7.0:1, dihitung ulang 7.16:1). Nilai yang
dipakai di `crm_employee/lib/shared/theme.dart` adalah hasil hitung ulang sesi ini (formula luminansi
relatif WCAG 2.1), bukan angka yang dicetak project desain. Detail lengkap tabel perbandingan di
`notes.md`'s `## #70`.

---

**Belum ditinjau** — menunggu penutupan Phase 5 (#73).
