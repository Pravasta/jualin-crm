# Issue #159 — checklist penutupan phase

> Checklist ringkas, **bukan** catatan status. Status pekerjaan tetap hidup di GitHub Issues (ADR-008) —
> berkas ini mengumpulkan poin yang perlu **dicek ulang saat #174** menutup Phase 8.6. Detail lengkap ada
> di `docs/phases/08.6-ui-redesign/notes.md` bagian `## #159`.

## Kelas bug yang mungkin berulang

- [x] **Warna abu-abu dingin ditulis langsung di layar, melewati token, dan sebagian gagal AA.**
      Token #159 menghangatkan seluruh netral (hue 60), tetapi layar yang menulis `oklch(… 0 0)`
      sebagai inline style tidak ikut berubah, dan **tiga nilainya di bawah 4.5:1** di atas putih:

      | Nilai | Rasio | Di mana | Diperbaiki di |
      |---|---|---|---|
      | `oklch(0.65 0 0)` | **3.23:1** | teks "tanpa pemilik" `leads-list.tsx`; tugas selesai `task-list.tsx`, `lead-detail.tsx` | #161, #162, #165 |
      | `oklch(0.6 0 0)` | **3.95:1** (3.51:1 di atas `0.96`) | ikon & jatuh tempo timeline/tugas `lead-detail.tsx` | #162 |
      | `oklch(0.922 0 0)` | — (tepi) | tepi chip/opsi di `leads-list`, `task-list`, `lost-reason-dialog`, `deactivate-member-dialog`, `lead-detail` | #161, #162, #165 |

      Ini bukan regresi dari #159 — nilainya sudah begitu di `main`. Tapi setiap layar yang dirombak
      **wajib** mengganti nilai mentah ini dengan token (`text-muted-foreground`, `border-border`,
      `var(--accent-strong)`), bukan menyalin ulang angkanya. Teks yang dicoret (tugas selesai) tetap
      teks: 4.5:1 berlaku.
      **Kemajuan:** `leads-list.tsx` bersih sejak **#161** (tersisa hanya bayangan FAB, dekoratif);
      `leads/[id]/*` bersih sejak **#162**; `task-list.tsx` dan `deactivate-member-dialog.tsx` sejak **#165**.
      **→ Ditutup di #165 (25 Sep 2026):** `grep -rn "oklch(0\.[0-9]* 0 0)"` di `src/app/(protected)` dan
      `src/app/(auth)` kosong. Sisa `oklch(` mentah di layar hanya bayangan FAB (dekoratif) dan satu kata di
      komentar. Nilai abu-abu di `globals.css` adalah **definisi** token (`--chart-*` dan blok `.dark`), bukan
      pemakaian yang melewati token, jadi bukan bagian dari temuan ini. `--chart-*` akan ditinjau di #171 saat
      grafik Laporan dibuat.
      **Pemicu peninjauan (tetap berlaku untuk #166–#168):** #174 — `grep -rn "oklch(0\.[0-9]* 0 0)" crm_dashboard/src/app/\(protected\) crm_dashboard/src/app/\(auth\)` harus
      kosong, atau setiap sisanya punya alasan tertulis.

## Deviasi dari handoff

- [ ] **Klaim token sheet "≥4.6:1 di atas latar tintnya sendiri" tidak benar untuk tint 85%.**
      Warna status Opsi B (hex handoff) mencapai 5.03–5.27:1 di atas **putih**, tetapi hanya
      4.11–4.29:1 di atas tint 85% yang dipakai badge lama. `STATUS_META.background` dinaikkan ke
      **94% putih** (terendah 4.65:1, `won`). Dijaga oleh `labels.test.ts`, yang **menghitung** rasio
      dari nilai di kode. **Pemicu peninjauan:** bila putaran desain berikutnya kembali menulis rasio
      kontras, bandingkan dengan hasil test, bukan sebaliknya.
