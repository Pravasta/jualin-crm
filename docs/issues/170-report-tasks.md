# Issue #170 — checklist penutupan phase

> Checklist ringkas, **bukan** catatan status. Status pekerjaan tetap hidup di GitHub Issues (ADR-008) —
> berkas ini mengumpulkan poin yang perlu **dicek ulang saat #174** menutup Phase 8.6. Detail lengkap ada
> di `docs/phases/08.6-ui-redesign/notes.md` bagian `## #170`.

## Keputusan yang perlu dicek ulang

- [ ] **Tugas tanpa penanggung jawab tidak tampil di blok "Tugas" Laporan.** `/v1/metrics/tasks` per anggota
      (TD §2.6). Tugas yang tidak ditugaskan, termasuk yang terlambat, tidak masuk baris mana pun. Terlihat nyata
      dengan data uji lokal: satu tugas terlambat hilang dari laporan.
      Dua pilihan, sengaja tidak diambil sendiri: (a) layar menyebut blok ini "per anggota" dan menautkan ke
      Tugas terfilter; (b) API menambah satu baris "Tanpa penanggung jawab" (`membership_id: null`).
      **Pemicu peninjauan:** #171. Bila layar memilih (a), poin ini ditutup di sana. Bila (b), itu perubahan
      API kecil yang harus diputuskan pemilik produk lebih dulu.

- [ ] **`overdue_count` adalah keadaan saat ini, tidak mengikuti periode** (TD §2.6, penyimpangan yang disengaja).
      **Pemicu peninjauan:** #171 wajib menuliskannya di layar. #174 memeriksa bahwa kalimat itu ada.
