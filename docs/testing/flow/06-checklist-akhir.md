# 6 — Checklist akhir

Rekap satu halaman. Centang setelah **benar-benar** mengklik lewat browser/`curl` — bukan diasumsikan
dari membaca kode. Kalau ada yang gagal, jangan dicentang; catat sebagai issue baru (lihat
[`README.md`](./README.md) bagian *Kalau sesuatu gagal*).

## Kalimat inti MVP — dari nol sampai selesai

- [ ] Owner mendaftar
- [ ] Verifikasi email (email sungguhan terkirim ke Mailpit)
- [ ] Login
- [ ] Undang employee
- [ ] Employee menerima undangan & login
- [ ] Owner membuat API key
- [ ] "Website" (curl) mengirim lead lewat API key
- [ ] Owner meng-assign lead ke seseorang
- [ ] Anggota yang ditugaskan menerima notifikasi
- [ ] Follow-up dari HP — login mobile, telepon/WhatsApp, ubah status, catatan (lihat `07`)
- [ ] Update lead (status, catatan)
- [ ] Konversi lead menang ke Customer

## 0 — Menjalankan aplikasi

- [ ] `docker compose` (postgres + mailpit + api) menyala tanpa error
- [ ] Migration berhasil sampai versi terbaru
- [ ] `GET /health` dan `/health/ready` keduanya `200`
- [ ] Dashboard (`npm run dev`) menyala, `http://localhost:3000` bisa dibuka
- [ ] Belum login → otomatis diarahkan ke `/login`
- [ ] `http://localhost:8025` (Mailpit) bisa dibuka

## 1 — Registrasi & Autentikasi

- [ ] Registrasi organization baru berhasil, **tidak** langsung login
- [ ] Email verifikasi muncul di Mailpit, tautannya berhasil memverifikasi
- [ ] Membuka tautan verifikasi **kedua kalinya** tidak error/500
- [ ] Login berhasil setelah verifikasi
- [ ] Password salah → pesan generik, bukan detail teknis
- [ ] Login berulang gagal → mulai diblokir (backoff)
- [ ] Logout mengembalikan ke `/login`, sesi benar-benar hilang
- [ ] Lupa password: pesan sama persis untuk email terdaftar maupun tidak
- [ ] Reset password via tautan di Mailpit berhasil
- [ ] Password lama tidak lagi berlaku setelah reset

## 2 — Tim & Undangan

- [ ] Undang anggota — dropdown role: Admin, Manager, Employee (bukan Owner)
- [ ] Terima undangan, cabang **user baru** (isi nama+password)
- [ ] Terima undangan, cabang **user sudah ada** (satu tombol konfirmasi)
- [ ] Employee di `/team`: daftar anggota terlihat (read-only), tapi tanpa tombol undang/ubah/nonaktifkan
- [ ] Employee di `/connect/api`: pesan "tidak tersedia untuk role Anda", **nol** panggilan API
- [ ] Admin **tidak bisa** ubah role Owner (403)
- [ ] Owner **bisa** promosikan orang lain jadi co-Owner
- [ ] Nonaktifkan anggota **tanpa** lead terbuka → langsung, tanpa dialog
- [ ] Nonaktifkan anggota **dengan** lead terbuka → `409` → dialog unassign/reassign
- [ ] Notifikasi penugasan muncul di lonceng, bisa ditandai terbaca

## 3 — Lead & Pipeline

- [ ] Buat lead manual berhasil, status awal **Baru**, sumber **Manual**
- [ ] Pencarian & filter status bekerja, filter bertahan setelah refresh (di URL)
- [ ] Edit field lead tersimpan
- [ ] Transisi status jalur utama: Baru → Dihubungi → Memenuhi Syarat → Penawaran → Menang
- [ ] Lompat status lebih dari satu langkah **ditolak**
- [ ] Kalah **wajib** memilih alasan
- [ ] "Buka kembali ke Baru" dari status Kalah berhasil
- [ ] Edit dari tab basi (versi lama) → konflik `409`, **tidak** menimpa diam-diam
- [ ] Penugasan lead tersimpan, entri activity muncul
- [ ] Task: buat, tandai selesai, **tidak bisa** dibuka kembali lewat checkbox
- [ ] Activity timeline berisi kalimat Indonesia yang masuk akal, bukan enum mentah

## 4 — Customer

- [ ] Konversi hanya muncul/berhasil untuk lead status **Menang**
- [ ] Tidak bisa konversi dua kali
- [ ] Data customer hasil konversi cocok dengan lead asalnya
- [ ] Tautan "Berasal dari lead" mengarah balik dengan benar
- [ ] Edit nama customer **tidak** mengubah nama lead asalnya
- [ ] Manager bisa baca customer, **tidak bisa** tulis (403)

## 5 — API Publik

- [ ] Buat API key: secret lengkap tampil **satu kali**, hilang setelah dialog ditutup
- [ ] Contoh curl dari dialog reveal **benar-benar berhasil** (201) pada percobaan pertama
- [ ] Lead dari API muncul di dashboard dengan sumber **API** + nama kunci
- [ ] `Idempotency-Key` sama, dikirim dua kali → `id` sama, bukan duplikat
- [ ] Field di luar `leads:write` (mis. `assigned_to_membership_id`) → `403 insufficient_scope`
- [ ] Endpoint selain `POST /v1/leads` dengan API key → `401` (bukan `403`)
- [ ] Body tanpa `name` → `400 validation_failed`
- [ ] Header `X-RateLimit-*` ada dan `Remaining` berkurang tiap request
- [ ] Halaman dokumentasi integrasi menampilkan `key_prefix` kunci yang dipilih + placeholder
- [ ] Cabut kunci → daftar tetap menampilkan barisnya (bukan hilang)
- [ ] Request setelah cabut → `401 invalid_api_key` seketika

## 7 — Mobile Android

- [ ] Login user session (bukan API key), biometric saat buka kembali, tolak masuk bila biometric gagal
- [ ] Daftar lead tetap terbaca dalam mode pesawat
- [ ] Telepon/WhatsApp masing-masing mencatat activity — **dikonfirmasi juga dari dashboard**
- [ ] Activity hanya tercatat setelah aplikasi eksternal benar-benar terbuka (batalkan sekali → tidak ada entri palsu)
- [ ] Ubah status bentrok → dialog konflik, tidak pernah menimpa diam-diam
- [ ] Tugas Saya: tandai selesai, tidak bisa dibuka kembali
- [ ] Push notification muncul dalam hitungan detik saat lead di-assign
- [ ] Push ditangani benar di ketiga keadaan aplikasi (foreground/background/mati) + kasus ditekan saat belum login
- [ ] Nonaktifkan employee dari dashboard → aplikasi kehilangan akses pada panggilan API berikutnya
- [ ] Logout menghapus device token dari backend (push berikutnya tidak sampai)
- [ ] Uninstall aplikasi → `device_tokens` dibersihkan pada percobaan kirim berikutnya, bukan menumpuk

## 8 — Formulir Embed (Phase 6)

- [ ] Menu **Connect** → kartu Formulir aktif; kartu Webhook "Belum tersedia" (bukan "terkunci paket")
- [ ] Buat formulir dari dashboard — enam field terisi label Indonesia default
- [ ] Ubah label field & aktifkan/nonaktifkan; "wajib" ikut mengaktifkan field, label kosong menolak simpan
- [ ] Allowlist: `localhost` tanpa skema ditolak; `http://localhost:9099` diterima
- [ ] **Salin snippet, tempel ke `index.html` kosong, sajikan dari `http://localhost:9099` → formulir muncul dengan label yang diubah**
- [ ] Field nonaktif tidak dirender; tinggi iframe menyesuaikan isi (embed.js)
- [ ] **Isi formulir dari browser (tunggu >2 detik) → lead masuk, Sumber: Formulir, tanpa pembuat/penerima**
- [ ] Angka Submission di daftar formulir bertambah
- [ ] Halaman yang sama dari port lain (di luar allowlist) → iframe ditolak / submit `403`
- [ ] Varian tanpa script tetap bekerja (tinggi tetap); varian JSX memakai `style={{ border: 0 }}`
- [ ] Employee di `/connect/form` (dan `/connect/form/{id}` langsung): pesan "tidak tersedia untuk role Anda", **nol** panggilan `/v1/forms`
- [ ] Nonaktifkan formulir → hilang dari daftar, iframe jadi `404`, lead yang sudah masuk tetap ada

## Kalau semua tercentang

Alur inti MVP (Phase 0–5) terbukti benar-benar jalan sebagai **satu rangkaian utuh**, bukan cuma
lolos test per-fitur satu-satu — untuk pertama kalinya termasuk "follow-up dari HP" sungguhan, bukan
gantinya lewat dashboard. Simpan hasil pengecekan ini (tanggal + siapa yang menjalankan) di
`docs/STATUS.md` atau catatan sesi Anda sendiri — dokumen ini sendiri **tidak** menyimpan status,
sesuai konvensi repo (ADR-008).
