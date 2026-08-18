# Phase 1 — Auth & Organization · PRD

> **Apa & kenapa.** Detail teknis di [`td.md`](./td.md).
> Sumber: [`architecture/freeze.md`](../../architecture/freeze.md) bagian 4 (Phase 1), 7 (keputusan B1–B5), 8.3 (migration `0002`) · [ADR-007](../../decisions/ADR-007-user-organization-cardinality.md)

---

## Tujuan

Menegakkan **fondasi tenancy yang benar sebelum ada satu pun data bisnis**.

Setiap tabel, endpoint, dan query di Phase 2 dan seterusnya akan bergantung pada bentuk `organization_id`, `TenantContext`, dan pola repository yang ditetapkan di sini. Kesalahan di lapisan ini tidak terlihat sebagai bug — ia terlihat sebagai data satu pelanggan yang muncul di layar pelanggan lain.

> Kegagalan di area ini bersifat fatal dan tidak bisa dipulihkan dengan permintaan maaf. Karena itu Phase 1 dikerjakan sebelum Lead, Customer, atau apapun yang terasa seperti "produk".

---

## Kebutuhan

| # | Sebagai… | Saya butuh… | Supaya… |
|---|---|---|---|
| 1 | Calon pelanggan | Mendaftar dengan nama organisasi, nama saya, email, dan password | Punya ruang kerja sendiri tanpa menunggu siapapun |
| 2 | Calon pelanggan | Memverifikasi email saya | Akun saya terbukti milik saya, dan email sistem sampai ke alamat yang benar |
| 3 | Owner | Login dan tetap login tanpa memasukkan password berulang kali | Bekerja tanpa gangguan |
| 4 | Owner | Mengundang anggota tim lewat email | Mereka bisa bekerja tanpa saya membuatkan akun manual |
| 5 | Orang yang diundang | Menerima undangan meski **sudah punya akun Jualin** | Tidak perlu membuat email kedua hanya untuk bergabung |
| 6 | Owner | Menonaktifkan anggota yang keluar | Akses mereka berhenti seketika, tapi riwayat pekerjaannya tetap utuh |
| 7 | Owner | Yakin data organisasi saya tidak terlihat organisasi lain | Saya bisa memercayakan data pelanggan saya ke sistem ini |
| 8 | Siapa pun | Mereset password yang terlupa | Tidak kehilangan akses permanen |

---

## Acceptance Criteria

Phase 1 selesai bila **semuanya** terpenuhi:

| # | Kriteria |
|---|---|
| 1 | Registrasi membuat `user` + `organization` + `membership(owner)` + `subscription(free)` dalam **satu transaksi** — gagal di salah satunya berarti tidak ada satupun yang tersimpan |
| 2 | Email verifikasi terkirim **setelah** transaksi commit, dan kegagalan pengiriman **tidak** membatalkan registrasi |
| 3 | Login ditolak sebelum email terverifikasi, dengan kode error yang bisa dibedakan client agar bisa menawarkan "kirim ulang" |
| 4 | Login berhasil menerbitkan access token (JWT, pendek) + refresh token opaque yang tersimpan di database dan bisa direvoke |
| 5 | Refresh token dirotasi setiap dipakai; memakai token lama yang sudah dirotasi **mencabut seluruh family** dan memaksa login ulang |
| 6 | Owner bisa mengundang email baru **maupun** email yang sudah punya akun — cabang kedua **wajib** melalui login, tidak pernah menyetel password |
| 7 | Menonaktifkan membership mencabut seluruh sesi aktifnya dalam transaksi yang sama |
| 8 | RBAC ditegakkan di **service layer** untuk keempat role, bukan hanya disembunyikan di UI |
| 9 | **Harness test isolasi tenant** berjalan di CI dan **terbukti bisa gagal** — menghapus `organization_id` dari satu query membuatnya merah |
| 10 | Test katalog memastikan setiap tabel tenant-scoped punya `organization_id` **dan** `UNIQUE (id, organization_id)` |
| 11 | Setiap endpoint yang mengirim email dibatasi per alamat email **dan** per IP |

---

## Di luar cakupan

| Tidak dikerjakan | Ke |
|---|---|
| Lead, Customer, Activity, Task | Phase 2 |
| Dashboard UI apapun | Phase 3 |
| API Key & endpoint publik | Phase 4 |
| Mobile app | Phase 5 |
| Organization switcher, UI "buat organisasi kedua" | Tidak dijadwalkan — [ADR-007](../../decisions/ADR-007-user-organization-cardinality.md) |
| Penegakan limit / usage / plan | Phase 8 — `subscriptions` di sini hanya baris `free`, tanpa mesin apapun |
| Integrasi provider email sungguhan | Menunggu domain + provider (lihat `STATUS.md` bagian lead time) |
| Entity `Team` | Tidak dijadwalkan — Manager melihat seluruh organization |
| RBAC dinamis / custom role | Tidak dijadwalkan — role adalah enum |

### Dua hal yang sengaja tertahan

**Provider email sungguhan.** Phase 1 mengirim lewat `LogMailer` yang mencatat isi email ke log, bukan mengirimnya. Seluruh alur registrasi → verifikasi → login bisa diuji end-to-end tanpa domain dan tanpa akun provider. Adapter sungguhan adalah pertukaran satu implementasi interface, dikerjakan begitu domain + provider siap (item lead-time di `STATUS.md`).

**Aturan "reassign lead terbuka" saat menonaktifkan membership.** Freeze mensyaratkan operasi ini menolak berjalan bila masih ada lead terbuka. Di Phase 1 **belum ada tabel lead**, sehingga aturan itu belum bisa ditegakkan. Yang **wajib** ada sekarang adalah pencabutan sesi. Titik penegakan lead ditambahkan di Phase 2 — dicatat sebagai kewajiban eksplisit di `td.md`, bukan diserahkan pada ingatan.

---

## Dependensi

Phase 0 selesai — struktur project, config, logger, error envelope, `db.InTx`, tooling migration, dan harness `dbtest` semuanya sudah ada dan menjadi fondasi langsung phase ini.

**Tidak ada keputusan yang memblokir.** B1–B5 final di freeze bagian 7; kardinalitas User↔Organization final di ADR-007.

---

## Pembagian issue

Freeze memetakan Phase 1 sebagai **tiga** session. Setelah melihat isinya — 9 tabel dengan composite FK, ~12 endpoint, argon2id, rotasi token dengan deteksi reuse, RBAC, dan dua harness test — saya pecah menjadi **empat**, dengan urutan mengikat:

| Urut | Issue | Kenapa dipisah |
|---|---|---|
| 1 | Schema `0002` + tenant context + pola repository + test katalog | Menetapkan pola yang ditiru **seluruh** issue berikutnya. Sama seperti issue #1 di Phase 0: PR yang menetapkan konvensi layak direview tersendiri, sebelum ada yang menumpuk di atasnya. |
| 2 | Registrasi + verifikasi email | Alur tulis pertama yang menyentuh transaksi atomik dan efek samping di luar transaksi |
| 3 | Login + refresh rotation + logout + reset password | Siklus hidup sesi, satu kesatuan yang tidak masuk akal dipecah |
| 4 | RBAC + invitation + penonaktifan membership + **harness isolasi** | Otorisasi butuh endpoint dari #2 dan #3 untuk diuji |

Ini **penyimpangan dari peta session di freeze**, dicatat di sini agar terlihat. Bila Anda lebih memilih tiga PR, katakan saat review — issue tinggal digabung.

Rincian: [`issues.md`](./issues.md)

---

## Bukan tujuan phase ini

- **Bukan** membangun UI apapun — seluruh verifikasi lewat test dan `curl`
- **Bukan** mengoptimasi query — belum ada volume untuk diukur
- **Bukan** menyiapkan struktur untuk Lead yang "sudah pasti datang" di Phase 2 (Aturan #27, #28)
