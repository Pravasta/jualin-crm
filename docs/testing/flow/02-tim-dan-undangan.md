# 2 — Tim & Undangan

Prasyarat: [`01-registrasi-dan-autentikasi.md`](./01-registrasi-dan-autentikasi.md) selesai — login
sebagai `owner@test.local` (password yang sudah diganti di langkah 1.6).

Menguji: undang anggota, terima undangan (dua cabang — user baru vs. user yang sudah punya akun),
ganti role, nonaktifkan anggota (tiga cabang), notifikasi.

## 2.1 Undang anggota baru (Employee)

1. Buka menu **Tim** (`/team`).
2. Klik **+ Undang anggota**.
3. Isi **Email**: `employee1@test.local`, **Role**: `Employee`. Kirim.

**Hasil yang diharapkan:** muncul di daftar **Undangan tertunda**. Perhatikan pilihan role di
dropdown — harus ada **Admin, Manager, Employee** (bukan Owner — Owner tidak bisa diundang langsung,
hanya dipromosikan belakangan).

4. Ambil tautan undangan dari log (pola sama seperti
   [`01-registrasi-dan-autentikasi.md`](./01-registrasi-dan-autentikasi.md) §1.2):

```bash
docker compose logs api | grep -o 'http://localhost:3000/invitations/accept[^\\"]*' | tail -1
```

## 2.2 Terima undangan — cabang "user baru"

`employee1@test.local` **belum** pernah punya akun di sistem ini sama sekali (email berbeda dari
Owner), jadi ini menguji cabang "buat akun baru".

1. **Logout** dari akun Owner dulu (Aturan #24 — dua sesi tidak bercampur; lebih aman diuji terpisah).
2. Buka tautan undangan dari 2.1 di browser (bisa browser yang sama setelah logout, atau jendela
   private/incognito supaya lebih jelas terisolasi).

**Hasil yang diharapkan:** form meminta **nama** + **password** (bukan cuma tombol "Gabung" — email
ini belum terdaftar).

3. Isi nama `Sari Employee`, password apa saja yang valid, submit.

**Hasil yang diharapkan:** diarahkan ke `/login`. Login dengan `employee1@test.local` + password yang
baru saja dibuat.

**Hasil yang diharapkan:** berhasil masuk ke Beranda — **tanpa** perlu verifikasi email terpisah
(menerima undangan sekaligus memverifikasi, sesuai TD).

4. Menu **Tim** tetap muncul di navigasi untuk Employee (nav tidak difilter per-role) — buka `/team`.

**Hasil yang diharapkan:** daftar anggota **terlihat** (read-only — siapa saja perlu tahu siapa
rekan setimnya), tapi tombol **+ Undang anggota**, ganti role, dan **Nonaktifkan** **tidak ada**.
Bagian **Undangan tertunda** juga tidak muncul sama sekali untuk role ini (bukan cuma disembunyikan —
`listInvitations` tidak pernah dipanggil untuk role tanpa hak itu).

5. Buka `/settings/api-keys` sebagai Employee.

**Hasil yang diharapkan:** **tidak** ada daftar kunci sama sekali — hanya pesan *"Manajemen API key
tidak tersedia untuk role Anda."* Buka tab Network di devtools browser: **tidak ada** panggilan ke
`/v1/api-keys` sama sekali (gerbangnya ada di atas fetch, bukan cuma menyembunyikan hasilnya).

## 2.3 Undang orang yang sudah punya akun — uji cabang "user sudah ada" + login multi-organization

Cabang ini muncul saat email yang diundang **sudah** terdaftar (di organization mana pun — ADR-007:
satu user bisa jadi anggota lebih dari satu organization). Cara paling realistis mengujinya:
daftarkan dulu akun mandiri di organization **lain**, baru undang emailnya ke `Toko Testing`.

1. **Logout.** Daftar organization **baru** (ulangi [`01-registrasi-dan-autentikasi.md`](./01-registrasi-dan-autentikasi.md)
   §1.1–§1.3): **Nama organization** `Toko Lain`, **Nama lengkap** `Rina Sudah Terdaftar`, **Email**
   `manager1@test.local`, password bebas. Verifikasi emailnya lewat tautan di log seperti biasa.
2. **Logout** dari `manager1@test.local`, login kembali sebagai `owner@test.local`.
3. Di `/team` milik `Toko Testing`, klik **+ Undang anggota** dengan **Email**: `manager1@test.local`,
   **Role**: `Manager`.
4. Cari tautan undangannya di log (§2.1 langkah 4), lalu **logout** dan login sebagai
   `manager1@test.local` (pakai password dari langkah 1).

**Hasil yang diharapkan saat login:** karena `manager1@test.local` sekarang anggota **dua**
organization (`Toko Lain` sebagai Owner, `Toko Testing` — belum, undangan belum diterima), pada
titik ini login masih hanya mengarah ke `Toko Lain` (satu-satunya organization yang statusnya aktif).

5. Buka tautan undangan `Toko Testing` dari langkah 4 sambil sesi `manager1@test.local` ini aktif.

**Hasil yang diharapkan:** form **tidak** meminta nama/password lagi — cukup satu tombol konfirmasi
("Gabung"), karena akun sudah ada.

6. **Logout**, login lagi sebagai `manager1@test.local`.

**Hasil yang diharapkan:** sekarang muncul layar **"Pilih organization"** — karena user ini kini
anggota **dua** organization sekaligus (ADR-007). Pilih `Toko Testing` — harus masuk sebagai Manager
di organization itu, terpisah total datanya dari `Toko Lain`.

7. Coba buka tautan undangan yang sama sekali lagi (sudah diterima) **tanpa login** (logout dulu):
   harus diarahkan untuk login dulu, bukan layar error.

## 2.4 Ganti role

1. Login kembali sebagai `owner@test.local`.
2. Di `/team`, ubah role `manager1@test.local` dari Manager ke **Admin**.

**Hasil yang diharapkan:** berhasil, badge role di baris itu berubah jadi Admin.

3. **Login sebagai Admin baru** (`manager1@test.local`) di jendela terpisah, coba ubah role Owner
   (`owner@test.local`) jadi apa pun.

**Hasil yang diharapkan:** **ditolak** (403) — Admin tidak boleh mengubah role Owner.

4. Balik ke sesi Owner, coba promosikan Admin (`manager1@test.local`) jadi **Owner** (co-owner).

**Hasil yang diharapkan:** **berhasil** — Owner boleh menaikkan orang lain jadi Owner juga (co-owner
sah, lihat `architecture/authorization.md`). Setelah ini, turunkan lagi role-nya ke Admin supaya
langkah berikutnya (nonaktifkan) tidak terganggu — Owner terakhir di organization tidak boleh
dinonaktifkan.

## 2.5 Nonaktifkan anggota — cabang tanpa lead terbuka

Anggota **tanpa** lead terbuka (`employee1@test.local`, belum pernah ditugaskan apa pun sampai titik
ini):

1. Di `/team`, klik **Nonaktifkan** pada baris `employee1@test.local`.

**Hasil yang diharapkan:** **langsung** nonaktif — **tanpa** dialog muncul (tidak ada lead terbuka
untuk dipindahkan).

> Dua cabang lain (member **punya** lead terbuka → ditolak `409` → dialog **lepas penugasan**
> atau **pindahkan ke anggota lain**) butuh lead yang sudah ditugaskan lebih dulu — diuji nanti di
> [`03-lead-dan-pipeline.md`](./03-lead-dan-pipeline.md) §3.7, bukan di sini. `manager1@test.local`
> sengaja dibiarkan aktif untuk itu — jangan dinonaktifkan sekarang.

## 2.6 Notifikasi

Selama §3 (lead) nanti menugaskan lead ke seseorang, **cek ikon lonceng notifikasi** (selalu terlihat
di header, di halaman mana pun) untuk anggota yang menerima penugasan itu:

**Hasil yang diharapkan:** setelah login sebagai anggota yang ditugaskan, lonceng menunjukkan
notifikasi baru berisi kalimat lengkap Bahasa Indonesia (bukan kode/ID mentah) tentang lead yang
ditugaskan. Klik notifikasi menandainya terbaca; badge count berkurang.

---

Selesai di sini: `employee1@test.local` (Employee, **nonaktif**) dan `manager1@test.local`
(Admin, **aktif** — sengaja dibiarkan begitu untuk §3.7 di berkas berikutnya). Lanjut ke
[`03-lead-dan-pipeline.md`](./03-lead-dan-pipeline.md).
