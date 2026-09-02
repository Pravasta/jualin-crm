# Issue #101 — checklist penutupan phase

> Checklist ringkas, **bukan** catatan status. Status pekerjaan tetap hidup di GitHub Issues (ADR-008) —
> berkas ini hanya mengumpulkan poin yang perlu **dicek ulang saat #104** menutup Phase 7. Detail lengkap
> tiap poin ada di `docs/phases/07-outbound-webhook/notes.md` bagian `## #101`.

## Deviasi dari TD / ADR

- [ ] **Aturan #20 dilanggar secara sadar untuk `whsec_`** — signing secret disimpan **terenkripsi**
      (AES-256-GCM), bukan di-hash. Alasannya sudah ditulis penuh di `td.md` §2.1 dan di komentar
      migration `0009`, tapi **`freeze.md` bagian 5 #20 sendiri belum menyebut pengecualian ini**.
      Aturan #20 berbunyi "API key SHA-256" tanpa syarat, dan sekarang ada satu kredensial yang tidak
      mengikutinya. Perlu diputuskan saat #104: tambahkan klausa ke #20 lewat ADR baru (Aturan #30:
      perubahan arsitektur hanya lewat ADR), atau biarkan sebagai pengecualian ber-scope-phase yang
      dicatat di `authentication.md` saja. **Jangan diam-diam mengubah freeze.**

- [ ] **`architecture/authentication.md` belum punya baris keempat** untuk `whsec_`. TD §15 sudah
      mendaftarkannya sebagai perubahan dokumentasi yang harus terjadi di phase ini; #101 belum
      mengerjakannya karena tabel itu baru bermakna lengkap setelah worker #102 benar-benar mengirim.
      Cek di #104 bahwa baris itu ada, termasuk kolom "arah kepercayaan" yang membedakannya dari tiga
      lainnya.

## Keputusan yang perlu dicek ulang

- [ ] **`delivery_id` tidak ikut di dalam payload yang di-enqueue.** TD §5 menggambarkan payload dengan
      `delivery_id` di dalamnya; implementasinya menyimpan snapshot **tanpa** field itu dan menyerahkan
      penyuntikannya ke worker saat kirim (`delivery_id` = `webhook_deliveries.id`, stabil lintas
      percobaan sesuai TD §4.2). Alasannya: satu snapshot dipakai bersama oleh semua endpoint yang
      melanggan, sedangkan `delivery_id` berbeda per baris. **#102 wajib benar-benar menyuntikkannya** —
      kalau tidak, penerima kehilangan satu-satunya alat deduplikasi yang kita janjikan untuk model
      `at-least-once`. Ini gagal **diam-diam**: payload tetap valid JSON, hanya kehilangan satu field.

- [ ] **Rotasi `WEBHOOK_SECRET_ENC_KEY` belum punya jalur bertahap.** Merotasi kunci membuat seluruh
      secret tersimpan tidak bisa didekripsi dan setiap endpoint harus dibuat ulang. Dapat diterima
      sekarang (belum ada pelanggan), tapi perlu ditinjau sebelum produksi. Bentuk penyelesaiannya kalau
      dibutuhkan: kolom versi kunci, bukan perubahan skema `0009`.

- [ ] **Timestamp di dalam payload memakai offset lokal, bukan `Z`.** `occurred_at` yang dibangun #101
      benar (`...Z`, UTC), tapi `data.lead.created_at` / `updated_at` yang berasal dari `Lead.Fields()`
      dirender sebagai `+07:00`. Ini **bukan regresi #101** — bentuk itu sudah dipakai API dashboard
      sejak Phase 2 dan webhook sengaja memakai bentuk yang sama persis (TD §5: satu bentuk lead, bukan
      bentuk kedua). Tapi Aturan #33 berbunyi "ISO 8601 UTC `Z`", jadi salah satu dari keduanya keliru.
      Memperbaikinya mengubah response API yang sudah dipakai dashboard **dan** mobile, jadi bukan
      pekerjaan Phase 7. Perlu diputuskan: revisi Aturan #33, atau jadwalkan perubahan API lintas klien.

## Kontrak kabel untuk #102

Nilai literal yang #101 tetapkan dan #102 harus cocokkan **persis**. Semuanya gagal diam-diam:

- [ ] `webhook.SignatureHeader` = `X-Jualin-Signature`, isi `t=<unix>,v1=<hex>` — worker harus memakai
      `webhook.Sign`, **bukan** menyusun ulang header-nya sendiri. Menyusun ulang dan salah urutan
      (`"<body>.<ts>"`) menghasilkan signature yang valid secara bentuk dan tidak pernah cocok di sisi
      penerima.
- [ ] Body yang ditandatangani harus **byte yang benar-benar dikirim**. Mem-parse `payload` lalu
      me-marshal ulang sebelum menandatangani akan mengubah urutan kunci dan membuat setiap signature
      gagal diverifikasi, tanpa error di sisi kita.
- [ ] Worker harus **mendekripsi** `secret_ciphertext` lewat `crypter` yang sama sebelum menandatangani.
- [ ] `delivery_id` disuntikkan saat kirim (lihat bagian di atas).

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
