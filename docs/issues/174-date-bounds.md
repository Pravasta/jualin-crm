# Issue #174 — temuan penutupan Phase 8.6

> Checklist ringkas, **bukan** catatan status (ADR-008). Ditemukan saat menulis prosedur uji
> `docs/testing/flow/10-redesain-dan-laporan.md` §10.4 di penutupan phase.

## Kelas bug yang mungkin berulang

- [ ] **Batas tanggal daftar lead (hari UTC) ≠ batas periode Laporan (hari lokal).**
      `crm_dashboard/src/lib/date.ts`: `dateInputToStartOfDayUTC("2026-09-01")` → `2026-09-01T00:00:00.000Z`,
      yang di WIB adalah **07:00**. Laporan (#171, `lib/report-period.ts`) memakai tengah malam **lokal**. Setiap tautan
      "buka daftar ini" dari Laporan (dan dari Beranda sejak Phase 3, yang malah memakai rentang UTC mundur dari
      *sekarang*) bisa menghasilkan daftar yang **selisih di tepi**: lead yang masuk 00:00–06:59 WIB pada hari pertama
      terhitung di Laporan tapi tidak di daftar, dan sebaliknya untuk hari setelah hari terakhir.
      **Bukan regresi Phase 8.6.** Daftar lead sudah begini sejak #32. Phase 8.6 membuat selisihnya terlihat, karena kini
      ada dua layar yang seharusnya menunjukkan angka yang sama.
      **Perbaikan yang diusulkan (satu issue kecil):** `date.ts` mengubah tanggal input menjadi batas hari **lokal**
      (sama dengan `report-period.ts`), dan Beranda memakai `presetRange` alih-alih `periodToRange`. Tambahkan test yang
      memakai zona waktu non-UTC. Organization dengan zona waktu berbeda dari browser tetap memakai browser sebagai
      pengganti, keterbatasan yang sama dengan #171, sampai `/v1/me` membawa `organizations.timezone`.
      **Pemicu:** keluhan pertama "angka Laporan tidak sama dengan daftarnya", atau issue perbaikan berikutnya di
      dashboard, mana yang lebih dulu.
