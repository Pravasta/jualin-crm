# Issue #101 — checklist penutupan phase

> Checklist ringkas, **bukan** catatan status. Status pekerjaan tetap hidup di GitHub Issues (ADR-008) —
> berkas ini hanya mengumpulkan poin yang perlu **dicek ulang saat #104** menutup Phase 7. Detail lengkap
> tiap poin ada di `docs/phases/07-outbound-webhook/notes.md` bagian `## #101`.
>
> **Ditinjau di #104 (3 September 2026).** Status tiap poin di bawah.

## Deviasi dari TD / ADR

- [x] **Aturan #20 dilanggar secara sadar untuk `whsec_`** — signing secret disimpan **terenkripsi**
      (AES-256-GCM), bukan di-hash. **Diputuskan di #104: lewat ADR baru**
      ([ADR-013](../decisions/ADR-013-signing-secret-storage.md)) yang menambah klausa pengecualian
      bertingkat batas ke Aturan #20 — kredensial yang **kita** pakai untuk menghasilkan bukti, hanya
      bila tidak ada alternatif hash. `freeze.md` tetap tak disentuh (Aturan #30); rujukannya lewat
      `authentication.md`.

- [x] **`architecture/authentication.md` baris keempat untuk `whsec_`** — ditambahkan di #104: bagian
      *Signing secret webhook (`whsec_`)* dengan tabel "arah kepercayaan" yang membedakannya dari tiga
      kredensial masuk, penyimpanan terenkripsi, tampil sekali, tak pernah di-log, dan konsekuensi
      rotasi kunci.

## Keputusan yang perlu dicek ulang

- [x] **`delivery_id` tidak ikut di dalam payload yang di-enqueue.** **#102 menyuntikkannya** lewat
      splice byte (bukan marshal ulang), sebagai kunci pertama, dibuktikan lewat penerima sungguhan yang
      memverifikasi signature dengan skema terdokumentasi. Lihat `docs/issues/102-webhook-worker.md`
      bagian *Kontrak kabel dari #101 — dipenuhi*.

- [ ] **Rotasi `WEBHOOK_SECRET_ENC_KEY` belum punya jalur bertahap.** Merotasi kunci membuat seluruh
      secret tersimpan tidak bisa didekripsi; setiap endpoint harus dibuat ulang. Worker menandai
      pengiriman `failed` permanen (bukan retry selamanya) saat `Decrypt` gagal — jadi tidak ada
      kegagalan senyap, hanya kerja manual. **Pemicu peninjauan: sebelum ada pelanggan produksi
      pertama** (`docs/STATUS.md` bagian *Punya Lead Time* / *Berikutnya*). Bentuk penyelesaiannya bila
      dibutuhkan: kolom versi kunci, bukan perubahan skema `0009`. Dicatat juga di
      [ADR-013](../decisions/ADR-013-signing-secret-storage.md) bagian *Konsekuensi*.

- [ ] **Timestamp di `data.lead.created_at`/`updated_at` memakai offset `+07:00`, bukan `Z`.** Bukan
      regresi #101 — bentuk itu sudah dipakai API dashboard sejak Phase 2, dan webhook sengaja memakai
      `leadJSON` yang sama (TD §5). Tapi Aturan #33 berbunyi "ISO 8601 UTC `Z`", jadi salah satu keliru.
      Memperbaikinya mengubah response API yang dipakai dashboard **dan** mobile — **bukan pekerjaan
      Phase 7**. **Pemicu: saat ada perubahan API lintas klien terjadwal berikutnya** (atau lebih awal
      bila integrator webhook melaporkan masalah parsing). Keputusannya: revisi Aturan #33 lewat ADR,
      atau jadwalkan perubahan API. Tetap terbuka di `docs/STATUS.md` bagian *Utang Teknis*.

## Kontrak kabel untuk #102 — **dipenuhi & dibuktikan di #102**

Nilai literal yang #101 tetapkan dan #102 harus cocokkan persis. Semuanya gagal diam-diam — keempatnya
dicek lewat penerima sungguhan yang memverifikasi signature dengan skema terdokumentasi:

- [x] `webhook.SignatureHeader` = `X-Jualin-Signature`, isi `t=<unix>,v1=<hex>` — worker memakai
      `webhook.Sign` apa adanya, tidak menyusun ulang header.
- [x] Body yang ditandatangani adalah byte yang benar-benar dikirim (`delivery_id` disuntik lewat
      splice, bukan marshal ulang).
- [x] Worker mendekripsi `secret_ciphertext` lewat `crypter` yang sama sebelum menandatangani.
- [x] `delivery_id` disuntikkan saat kirim, sebagai kunci pertama.

## Bug ditemukan & sudah diperbaiki (dicatat untuk arsip)

- [x] **`0008` menyimpan signing secret sebagai SHA-256 hash**, menyalin pola `api_key` — sehingga worker
      #102 tidak akan pernah bisa menandatangani apa pun, karena HMAC butuh secret asli sebagai kunci.
      Ketahuan saat membaca ulang TD §2 sebelum menulis `signature.go`, bukan lewat test (tidak ada test
      yang bisa menangkapnya: seluruh #100 hijau, dan tetap hijau, karena tidak ada satu pun pemanggil
      yang membutuhkan secret itu kembali). Diperbaiki migration `0009` + `internal/shared/crypter`.

      **Kelas bugnya layak diingat, bukan hanya perbaikannya:** komentar `0008` sudah menyatakan dengan
      benar bahwa kredensial ini "yang PERTAMA dengan arah kepercayaan terbalik", lalu tetap menerapkan
      pola default di kolom tepat di bawahnya. Mengenali sebuah kasus itu berbeda **tidak** otomatis
      mencegah penerapan pola yang sudah dikenal. Pertanyaan yang seharusnya diajukan: "siapa yang
      membaca nilai ini nanti, dan dalam bentuk apa?" — bukan "kredensial lain disimpan bagaimana?".

      Regresinya sekarang dijaga `TestUnit_Create_SecretIsRecoverable` dan
      `TestHandler_Create_SecretShownOnceNeverAgain`, yang menguji properti "yang tersimpan bisa kembali
      menjadi secret yang ditampilkan" — bukan sekadar "tidak tersimpan sebagai plaintext", yang dulu
      juga dipenuhi oleh hash.
