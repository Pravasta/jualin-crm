# Issue #102 — checklist penutupan phase

> Checklist ringkas, **bukan** catatan status. Status pekerjaan tetap hidup di GitHub Issues (ADR-008) —
> berkas ini mengumpulkan poin yang perlu **dicek ulang saat #104** menutup Phase 7. Detail lengkap ada
> di `docs/phases/07-outbound-webhook/notes.md` bagian `## #102`.
>
> **Ditinjau di #104 (3 September 2026).** Status tiap poin di bawah.

## Deviasi dari TD / ADR

- [x] **TD §13 dikoreksi di PR yang sama, bukan cuma dicatat.** Rencana aslinya membiarkan sisa batch
      `delivering` saat shutdown; implementasinya mengembalikan yang belum dicoba ke `pending`
      (`Release`), karena kalau tidak, setiap deploy menunda baris-baris itu 10 menit tanpa alasan.
      Ditulis sebagai §4.4 baru + baris §13 diperbarui. Tidak ada tindakan lanjut — dicatat supaya
      #104 tahu perubahan dokumen itu disengaja.

- [x] **D5 di `prd.md` diperjelas.** *"5 percobaan"* di sebelah daftar **lima** jeda tidak konsisten:
      lima percobaan hanya punya empat celah, sehingga jeda `6j` mustahil terpakai. Diputuskan pemilik
      produk (2 Sep 2026): **5 percobaan ulang setelah kiriman pertama = maksimal 6 panggilan HTTP**,
      semua jeda terpakai. Teks D5 diperbarui. Bukan perubahan keputusan, melainkan koreksi teks yang
      ambigu — dicatat karena mengubah jendela retry total dari 2j36m ke 8j36m.

## Keputusan yang perlu dicek ulang

- [x] **`SKIP LOCKED` ternyata bukan yang menjamin exactly-once — TD §4.1 diperbaiki di #104.**
      Pembagian yang benar: **`WHERE status = 'pending'` + row lock → exactly-once**; **`SKIP LOCKED` →
      liveness**. TD §4.1 kini memuat blok koreksi eksplisit dengan peringatan "jangan hapus predikat
      status dengan anggapan `SKIP LOCKED` sudah cukup", dan menunjuk ke `notes.md` `## #102`. Kedua
      properti diuji terpisah di `worker_concurrency_test.go`.

- [ ] **Ambang reaper 10 menit sengaja tidak bisa dikonfigurasi.** Margin keselamatan, bukan tuning
      knob — hanya benar selama jauh di atas `WEBHOOK_DELIVERY_TIMEOUT` (10s). Mengeksposnya mengundang
      penyetelan di bawah nilai itu → reaper satu instance merebut pengiriman instance lain → setiap
      pengiriman terkirim dua kali. **Pemicu peninjauan: bila pernah ada endpoint sah yang butuh lebih
      dari 10 menit untuk merespons** (belum pernah terjadi; `WEBHOOK_DELIVERY_TIMEOUT` default 10s
      menutupnya jauh sebelum itu). Tetap `const reapThreshold` di `worker.go`.

- [ ] **`DisableKeepAlives` membebani setiap pengiriman dengan satu handshake.** Wajib demi keamanan
      (daftar tolak SSRF hanya dievaluasi ulang tiap kirim bila tiap kirim membuka koneksi baru), belum
      pernah diukur di volume nyata. **Dimasukkan ke daftar bersama di `api.md` bagian *Angka batasnya
      belum pernah diukur*** (#104) — pemicunya sama: traffic produksi nyata. Jalan keluarnya bila jadi
      masalah **bukan** menyalakan kembali keep-alive, melainkan memindahkan cek deny-list ke tempat
      yang tetap dievaluasi per-request.

- [ ] **Retensi malas belum pernah diuji di volume produksi** — sama seperti `idempotency_key` (#47).
      Polanya kini ada di dua tempat; kalau bermasalah, bermasalah di keduanya. **Pemicu: traffic
      produksi nyata**, ditinjau **satu kali bersama** retensi `idempotency_key` (`docs/issues/047`),
      bukan peninjauan terpisah. Dicatat di `api.md` bagian *Webhook Keluar → Retensi* dan
      `td.md` §10.

## Kontrak kabel dari #101 — dipenuhi

Keempatnya dicek dan dibuktikan lewat penerima sungguhan yang memverifikasi signature dengan skema
terdokumentasi, bukan dengan kode kita:

- [x] `webhook.Sign` dipakai apa adanya; worker tidak menyusun ulang header
- [x] Yang ditandatangani adalah byte yang benar-benar dikirim (`delivery_id` disuntik lewat splice,
      bukan marshal ulang)
- [x] `secret_ciphertext` didekripsi lewat `crypter` yang sama
- [x] `delivery_id` disuntikkan saat kirim, sebagai kunci pertama

## Bug ditemukan & sudah diperbaiki (dicatat untuk arsip)

- [x] **Keep-alive melewati daftar tolak SSRF sepenuhnya.** `http.Transport` hanya memanggil
      `DialContext` saat butuh koneksi **baru**; pengiriman kedua ke endpoint yang sama memakai koneksi
      pool dan tidak pernah dicek lagi. Artinya jendela DNS rebinding justru terbuka untuk kasus paling
      umum — endpoint yang sering menerima event — padahal TD §3.2 mensyaratkan validasi tiap kirim.

      Ketahuan saat mereview rencana sendiri sebelum eksekusi, bukan lewat test. Diperbaiki dengan
      `DisableKeepAlives: true`, dan sekarang dijaga `TestHTTPClient_NeverReusesConnections`, yang
      menghitung koneksi diterima — satu-satunya cara mengamati ini dari luar, karena status code
      terlihat sama persis di kedua keadaan.

      **Kelas bugnya layak diingat:** menaruh pemeriksaan keamanan di sebuah hook tidak ada artinya
      sebelum memastikan hook itu benar-benar dipanggil setiap kali. Pertanyaan yang harus diajukan
      untuk setiap cek semacam ini: *"apa yang membuat ini berjalan pada request kedua?"*

- [x] **Hanya IP pertama yang di-dial.** Host dengan beberapa A record akan gagal total kalau record
      pertama mati, padahal dialer biasa mencoba semuanya — regresi keandalan yang muncul begitu
      resolusi diambil alih dari `net/http`. Diperbaiki dengan iterasi seluruh alamat tervalidasi.

- [x] **Kegagalan `Decrypt` tidak punya perlakuan.** Rotasi `WEBHOOK_SECRET_ENC_KEY` membuat **setiap**
      secret tak bisa didekripsi; tanpa penanganan khusus, jalur itu jatuh ke "transport error" dan
      seluruh antrian berputar retry 6 jam sekali selamanya menuju kegagalan yang sama. Sekarang
      `failed` permanen dengan alasan eksplisit.

- [x] **Urutan `onDrained` pada shutdown salah.** Sempat ditulis sebagai `defer`, yang membuatnya jalan
      setelah log `"shutdown complete"` dan **dilewati sama sekali** pada jalur `os.Exit(1)`. Diganti
      pemanggilan eksplisit setelah `srv.Shutdown` berhasil.

- [x] **Data race di test sendiri.** `srv.Config.ConnState` diset setelah `httptest.NewServer` mulai
      melayani. Ketahuan `go test -race`, diperbaiki dengan `NewUnstartedServer` + `Start()`. Dicatat
      karena polanya mudah terulang: setiap `srv.Config.*` harus diset sebelum server berjalan.
