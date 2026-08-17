# ADR-007 — Kardinalitas User → Organization

> **Status:** ✅ **Accepted** — 17 Agustus 2026
> **Diputuskan:** A-soft (bagian 8)
> **Konteks:** Clarification: User, Organization & Multi-Tenancy (dokumen pemilik produk, 17 Agustus 2026)
> **Terpengaruh:** [`architecture/freeze.md`](../architecture/freeze.md) bagian 7, 8.3 (`memberships`)
>
> **Prinsip yang dikunci oleh ADR ini:**
>
> > *"1 user = 1 organization" adalah **pengalaman produk normal**, bukan invariant sistem.*
> >
> > Karena itu ia **tidak** berarti "1 user hanya boleh punya 1 membership" sebagai aturan database.
>
> Kesederhanaan ditegakkan di lapisan UI. Schema tetap terbuka karena menutupnya tidak
> menambah kesederhanaan apapun, tetapi menutup jalur keluar secara permanen.
>
> Menjadi anggota organization kedua hanya terjadi lewat **undangan** — tidak pernah atas
> inisiatif pengguna sendiri.

---

# 1. Ringkasan Rekomendasi

**Rekomendasi: Option A sebagai model produk, tanpa constraint kardinalitas di database.**

Disebut **A-soft** di seluruh dokumen ini.

| Lapisan | Keputusan |
|---|---|
| **Model produk** | **Normal product experience: 1 user = 1 organization.** Tidak ada organization switcher. Registrasi selalu membuat organization baru. Tidak ada tombol "Buat organisasi lain". Organization kedua hanya bisa diperoleh lewat undangan. |
| **Schema** | `memberships (user_id, organization_id)` **tanpa** `UNIQUE(user_id)` |
| **Login** | Bila user punya tepat 1 membership → langsung masuk. Bila > 1 → satu layar pemilihan organization. |
| **Undangan** | Email yang sudah punya akun **boleh** diundang ke organization lain |

**Inti argumennya:**

> Seluruh manfaat yang Anda inginkan dari Option A — UX sederhana, login sederhana, billing sederhana — diperoleh dengan **tidak membangun UI multi-organization**, bukan dengan memasang constraint di database.
>
> Constraint tersebut tidak menambah satupun manfaat itu, tetapi menciptakan satu-satunya biaya yang benar-benar tidak bisa dibatalkan: **fragmentasi akun**.

---

# 2. Reframe: Ini Bukan Keputusan Schema

Dokumen klarifikasi membingkai ini sebagai pilihan antara dua bentuk data. Setelah ditelusuri, bentuk datanya **identik** di kedua opsi — `users`, `memberships`, `organizations` — persis seperti yang Anda catat sendiri di §7 dan §17.

Yang benar-benar berbeda hanya **tiga hal**, dan ketiganya berada di lapisan produk, bukan lapisan data:

| # | Perbedaan | Lapisan | Biaya membalikkannya |
|---|---|---|---|
| 1 | Ada tombol "Buat organisasi baru" di dashboard? | UI | ~1 jam |
| 2 | Ada organization switcher dalam sesi berjalan? | UI + auth | ~1–2 hari |
| 3 | Boleh mengundang email yang sudah punya akun? | Business rule | ~2 jam |

Ketiganya reversibel dengan biaya rendah. **Tidak satupun memerlukan migration.**

Karena itu, pertanyaan sesungguhnya bukan *"Option A atau B?"* melainkan:

> **Apakah ada alasan menambahkan `UNIQUE(user_id)` pada `memberships`?**

Dan jawabannya, setelah semua dimensi ditelusuri di bagian 5: **tidak ada.**

## 2.1 Tiga varian, bukan dua

Membedakan A-strict dari A-soft adalah inti keputusan ini.

| Varian | Constraint DB | Keunikan email | Login | Cara mendapat organization kedua |
|---|---|---|---|---|
| **A-strict** | `UNIQUE(user_id)` | global | selalu langsung | Harus daftar akun baru dengan **email berbeda** |
| **A-strict-2** | `UNIQUE(user_id)` | per organization | **butuh identifier org saat login** | Akun baru, email boleh sama |
| **A-soft** ⭐ | tidak ada | global | langsung, kecuali > 1 membership | Cukup diundang |
| **B penuh** | tidak ada | global | pemilihan + switcher dalam sesi | Buat sendiri atau diundang |

**A-strict-2 harus dicoret sekarang.** Bila email hanya unik per organization, sistem tidak bisa tahu akun mana yang dimaksud saat `budi@gmail.com` melakukan login atau reset password. Anda terpaksa meminta identifier organization di layar login (kode org atau subdomain). Itu **lebih rumit** daripada layar pemilihan yang ingin dihindari — dan menimpakan kerumitannya kepada 100% pengguna, bukan kepada sebagian kecil.

Sisa dokumen ini membandingkan **A-strict**, **A-soft**, dan **B penuh**.

---

# 3. Temuan yang Mengubah Analisis

## 3.1 ⚠️ Undangan employee membuat multi-membership tidak terhindarkan

Ini temuan terpenting, dan ia muncul dari fitur yang **sudah ada di MVP Anda** — bukan dari kebutuhan agency di masa depan.

Perhatikan skenario ini:

```
Budi mendaftar → jadi Owner "Toko ABC"           (budi@gmail.com)
Temannya di "Toko XYZ" mengundang budi@gmail.com sebagai Sales
```

Tidak ada satupun pihak di skenario ini yang meminta fitur multi-organization. Ini hanya seseorang yang diundang bekerja di tempat lain — perilaku paling normal dari sebuah undangan.

**Di bawah A-strict, sistem harus menolaknya.** Pesan errornya kira-kira: *"Email ini sudah terdaftar. Gunakan email lain."*

Empat konsekuensi:

| Konsekuensi | Penjelasan |
|---|---|
| **Terlihat seperti bug** | Bagi pengguna, ini undangan yang gagal tanpa alasan yang masuk akal |
| **Menghasilkan tiket support** | Pada produk berharga murah, biaya support adalah margin |
| **Memaksa email palsu** | Pengguna memakai `budi+xyz@gmail.com`, dan akun mereka terpecah selamanya |
| **Anda akan menemuinya pertama kali** | Saat menguji produk sendiri dengan email yang sama di dua organization |

Ini bukan kasus tepi teoretis. Di SMB Indonesia ia biasa terjadi:

- Sales freelance yang memegang dua brand
- Konsultan atau agency yang mengelola CRM beberapa klien
- Keluarga dengan dua unit usaha
- Admin yang membantu dua toko milik pemilik yang sama

## 3.2 ⚠️ Employee pindah perusahaan

Anda sendiri mengangkat kasus ini di §13. Perbandingannya tajam:

| | A-strict | A-soft |
|---|---|---|
| Org A menonaktifkan membership | ✅ | ✅ |
| Org B mengundang email yang sama | ❌ **Ditolak** — email sudah terpakai | ✅ Berhasil |
| Employee memakai login lama | ❌ Harus buat akun baru | ✅ Login yang sama |
| Riwayat di Org A tetap utuh | ✅ | ✅ |

Di bawah A-strict, satu-satunya jalan keluar adalah menghapus akun lama — yang akan merusak atribusi historis di Org A (siapa yang menangani lead ini tahun lalu?) — atau memaksa pengguna memakai email lain.

Perpindahan kerja bukan skenario langka. Ia normal, dan akan terjadi pada pelanggan Anda.

## 3.3 ⚠️ Koreksi: billing **identik** di kedua opsi

§4.2 menyebut Option A membuat billing lebih sederhana. Setelah ditelusuri, ini tidak akurat — dan penting diluruskan karena ia menjadi salah satu alasan utama Anda condong ke A.

Di **kedua** opsi, relasinya sama:

```
Organization ──1:1──> Subscription ──> Payment Service
```

Subscription selalu milik organization, tidak pernah milik user. Seorang user dengan dua membership tidak menciptakan konstruksi billing baru apapun: dua organization, dua subscription, masing-masing dibayar oleh organizationnya sendiri.

Kerumitan billing baru muncul bila Anda menginginkan **billing account di level user** yang menggabungkan beberapa organization ke dalam satu tagihan. Itu tidak diusulkan di manapun, dan bukan bagian dari Option B.

> **Kesimpulan:** "Organization = Billing Account" yang Anda inginkan di §12 **sudah terpenuhi di kedua opsi**. Ini bukan pembeda.

## 3.4 ⚠️ Login: perbedaannya jauh lebih kecil dari yang terlihat

§4.3 menyebut Option A membuat login lebih sederhana. Benar, tapi ukurannya kecil:

```
POST /v1/auth/login { email, password, organization_id? }

  ├── 1 membership aktif   → terbitkan token untuk organization itu     (≥99% kasus)
  └── > 1 membership       → balas daftar organization, client memilih  (<1% kasus)
```

Cabang kedua adalah **satu percabangan di satu endpoint** dan satu layar sederhana. Itulah keseluruhan biaya "login lebih rumit".

Yang orang bayangkan saat mendengar "organization switcher" — active-organization state, invalidasi cache saat berpindah, konteks yang harus dijaga di setiap layar — **tidak ada di sini**. Semua itu muncul dari **berpindah di tengah sesi**, dan itulah yang kita tunda.

Di A-soft, organization terkunci di dalam token. Untuk berpindah, logout lalu login lagi. Tidak ada state yang perlu dikelola.

## 3.5 ✅ Yang Anda benar sepenuhnya

§9 Anda benar dan penting: **one-user-one-org tidak membuat Jualin berhenti menjadi multi-tenant.** Multi-tenancy ditentukan oleh banyaknya tenant yang berbagi platform dengan data terisolasi, bukan oleh banyaknya organization per akun. Tidak ada bagian dari analisis ini yang mempersoalkan itu.

Instingnya juga benar: kompleksitas yang belum memberi nilai harus ditolak. Perbedaan pendapat saya hanya pada **di lapisan mana** penolakan itu diterapkan — di UI, bukan di schema.

---

# 4. Biaya Sebenarnya dari A-strict: Fragmentasi Akun

Ini satu-satunya konsekuensi yang benar-benar tidak bisa dibatalkan.

## 4.1 Apa yang terjadi

Di bawah A-strict, satu orang dengan dua bisnis akan berakhir seperti ini:

```
budi@gmail.com        → Toko ABC
budi+xyz@gmail.com    → Toko XYZ     ← orang yang sama, sistem tidak tahu
```

## 4.2 Kenapa ini mahal untuk dibalikkan

Bila suatu saat Anda ingin pindah ke Option B, migrasinya bukan sekadar menghapus constraint. Anda harus **menggabungkan akun**:

| Masalah | Kenapa sulit |
|---|---|
| **Identifikasi** | Sistem tidak punya cara andal mengetahui bahwa dua akun adalah orang yang sama. Pengguna harus menyatakannya sendiri, dan Anda harus memverifikasinya. |
| **Kredensial** | Password mana yang bertahan? Bagaimana bila keduanya punya MFA? |
| **Atribusi** | Audit log, `created_by`, dan activity menunjuk `membership_id` dari akun lama. Menulis ulang riwayat merusak jejak audit; membiarkannya menghasilkan riwayat yang terpecah. |
| **Sesi & perangkat** | Refresh token, device token, preferensi notifikasi harus direkonsiliasi |
| **Tekanan waktu** | Pekerjaan ini selalu datang saat pelanggan sedang meminta, bukan saat Anda punya waktu luang |

Perkiraan realistis: **proyek berminggu-minggu dengan risiko integritas data**, dikerjakan di bawah tekanan.

## 4.3 Bandingkan dengan A-soft → B

| | Yang perlu dikerjakan |
|---|---|
| **A-strict → B** | Merge akun (di atas) + UI switcher |
| **A-soft → B** | **Tambahkan tombol "Buat organisasi baru" + switcher UI.** Nol migration data. |

Perbedaan ini — proyek migrasi data versus fitur UI — adalah keseluruhan taruhan dari keputusan ini.

---

# 5. Perbandingan Dimensi

Sesuai 20 dimensi yang Anda minta di §6.

| Dimensi | A-strict | **A-soft** ⭐ | B penuh |
|---|---|---|---|
| **Kesederhanaan produk** | Sama | **Sama** | Lebih rumit (switcher) |
| **UX** | Sama | **Sama** | Konsep tambahan bagi pengguna |
| **Registration flow** | Identik | **Identik** | Identik + "buat org lain" |
| **Login flow** | 1 jalur | **1 jalur + cabang <1%** | + switcher dalam sesi |
| **Multi-tenant architecture** | Identik | **Identik** | Identik |
| **Database design** | + `UNIQUE(user_id)` | **Tanpa constraint** | Tanpa constraint |
| **Authorization** | Identik — selalu dari tenant context | **Identik** | Identik |
| **Subscription / billing** | Identik (lihat 3.3) | **Identik** | Identik |
| **API Key** | Identik — milik organization | **Identik** | Identik |
| **Embedded Form** | Identik — milik organization | **Identik** | Identik |
| **Employee invitation** | ❌ **Rusak** untuk email yang sudah ada | **✅ Berfungsi** | ✅ Berfungsi |
| **Mobile authentication** | Identik | **Identik** | + pemilihan org di mobile |
| **Future scalability** | ❌ Terkunci | **✅ Terbuka** | ✅ Terbuka |
| **Customer experience** | ❌ Undangan gagal, akun terpecah | **✅ Mulus** | ✅ Mulus |
| **Implementation complexity** | Terendah | **+1 percabangan, +1 layar** | +switcher, +state aktif |
| **Migration complexity** | ❌ Merge akun bila berubah | **✅ Tidak ada** | Tidak ada |
| **Maintenance cost** | Sama | **Sama** | Sedikit lebih tinggi |
| **Business limitation** | ❌ Menolak konsultan/agency/multi-usaha | **✅ Tidak ada** | Tidak ada |
| **SaaS pricing model** | Identik — per organization | **Identik** | Identik |
| **Kemudahan MVP** | Terendah | **Praktis sama** | Lebih tinggi |

**Bacaan tabel ini:** dari 20 dimensi, A-strict dan A-soft **identik di 15 dimensi**. Pada 5 sisanya, A-strict kalah di empat (undangan, pengalaman pelanggan, migrasi, batasan bisnis) dan menang tipis di satu (implementasi, selisih satu percabangan dan satu layar).

---

# 6. Recommendation A — Bila Memilih 1 User = 1 Organization

Sesuai permintaan §19.

## Kapan model ini paling cocok

- Produk B2B di mana satu orang secara definisi hanya bekerja di satu tempat (mis. sistem HRIS internal)
- Produk dengan verifikasi identitas ketat, di mana satu identitas satu badan usaha
- Produk **tanpa** undangan lintas pihak

**Jualin CRM tidak memenuhi ciri ketiga** — dan itu yang menentukan.

## Risiko bisnis

| Risiko | Dampak |
|---|---|
| Menolak segmen konsultan/agency | Segmen ini justru bernilai tinggi: mereka membawa beberapa organization berbayar sekaligus |
| Undangan gagal untuk email yang sudah ada | Tiket support pada produk yang margin supportnya tipis |
| Employee pindah kerja terhambat | Friksi pada perilaku yang sepenuhnya normal |
| Akun terpecah | Merusak analitik pengguna dan mempersulit dukungan |

## Batasan

Satu orang tidak dapat, dengan satu login: memiliki dua bisnis · membantu bisnis kerabat · bekerja sebagai sales di dua perusahaan · mengelola CRM klien.

## Apakah mudah di-upgrade ke multi-org?

| Varian | Jawaban |
|---|---|
| **A-strict** | ❌ **Tidak.** Perlu merge akun (bagian 4). |
| **A-soft** | ✅ **Ya.** Hanya penambahan UI. |

## Apakah cocok untuk Jualin?

**Sebagai model produk: ya.** Registrasi = satu organization, tanpa switcher, tanpa konsep workspace. Ini sesuai positioning Anda dan saya dukung sepenuhnya.

**Sebagai constraint database: tidak.** Ia tidak menambah kesederhanaan apapun yang tidak sudah diberikan oleh model produk, dan ia merusak alur undangan yang sudah ada di MVP.

---

# 7. Recommendation B — Bila Memilih 1 User = N Organizations

## Kapan model ini paling cocok

Produk dengan pengguna lintas organisasi sebagai norma: Slack, Notion, Figma, GitHub, Linear. Semuanya mengasumsikan satu orang berpartisipasi di banyak ruang.

## Kompleksitas tambahan

| Komponen | Beban |
|---|---|
| Organization switcher | UI + endpoint + penerbitan ulang token |
| Active organization state | Harus konsisten di seluruh layar & cache |
| Pemilihan org di mobile | Layar tambahan, penyimpanan pilihan |
| Onboarding | Pengguna harus memahami konsep "workspace" |
| Notifikasi | Harus menyebutkan organization asal |
| Dukungan | "Kenapa data saya tidak muncul?" → ternyata salah organization |

## Manfaat bisnis

Segmen agency/konsultan menjadi mungkin. Tapi **manfaat ini sudah didapat di A-soft lewat undangan** — yang hilang hanyalah kemampuan membuat organization kedua sendiri.

## Apakah terlalu kompleks untuk MVP?

**Ya.** Switcher dan active-org state adalah kompleksitas nyata yang belum memberi nilai kepada pelanggan pertama Anda.

## Adakah alasan kuat memilihnya sejak awal?

**Tidak.** Tidak ada pelanggan yang memintanya, dan A-soft menjaga pintunya tetap terbuka tanpa biaya.

---

# 8. Rekomendasi Final

## Yang dikunci sekarang

| # | Keputusan | Alasan |
|---|---|---|
| 1 | **Model produk = Option A.** Registrasi membuat satu organization. Tanpa switcher. Tanpa "Buat organisasi lain". | Sesuai positioning; UX paling sederhana |
| 2 | **`memberships` tanpa `UNIQUE(user_id)`** | Constraint tidak menambah manfaat, tapi memutus jalur keluar |
| 3 | **Email unik global** pada `users` | Menjadikan A-strict-2 gugur; membuat login & reset password tidak ambigu |
| 4 | **Login menerima `organization_id` opsional**, dengan cabang > 1 membership | Satu percabangan, forward-compatible |
| 5 | **Undangan boleh menyasar email yang sudah punya akun** — user login dulu, lalu konfirmasi | Menutup jalur pengambilalihan akun sekaligus membuat alur bekerja |

## Yang ditunda

Tombol "Buat organisasi baru" · organization switcher dalam sesi · cross-organization reporting · agency/enterprise account · hierarki & parent organization · transfer & merge organization · dashboard multi-bisnis.

## Kenapa ini bukan over-engineering

Uji dengan Aturan #27 di freeze — *"abstraksi hanya setelah ada implementasi kedua yang nyata"*:

| Pertanyaan | Jawaban |
|---|---|
| Apakah ada abstraksi tambahan? | **Tidak.** `memberships` sudah ada di kedua opsi dan sudah Anda setujui di §7 & §17 |
| Apakah ada tabel tambahan? | **Tidak** |
| Apakah ada UI tambahan? | **Tidak** — switcher justru ditunda |
| Apakah ada kode tambahan? | **Satu percabangan** di endpoint login |

Rekomendasi ini **menghapus** satu baris constraint dari rencana. Ia lebih sedikit, bukan lebih banyak.

> Menambahkan `UNIQUE(user_id)` justru adalah bentuk premature design yang lain: mengunci kardinalitas untuk model bisnis yang belum diuji pasar, demi kesederhanaan yang sudah diperoleh dari model produknya sendiri.

## Bagaimana pengguna tetap merasakan "satu akun, satu bisnis"

| Skenario | Yang dialami pengguna |
|---|---|
| Daftar | Buat organization, langsung masuk |
| Login sehari-hari | Langsung ke dashboard, tanpa pertanyaan |
| Ingin membuat bisnis kedua | Tidak ada tombolnya — sesuai model produk Option A |
| Diundang ke organization lain | Berhasil. Saat login berikutnya, muncul pilihan satu kali. |

Dari sudut pandang 99% pengguna, **produknya adalah Option A.** Yang tersisa hanyalah pintu keluar yang tidak dipaku.

---

# 9. Migration Path A-soft → B

Bila suatu saat dibutuhkan:

| Langkah | Sifat |
|---|---|
| 1. Tombol "Buat organisasi baru" | UI + endpoint |
| 2. `POST /v1/auth/switch-organization` — terbitkan token untuk membership lain | Backend, kecil |
| 3. Switcher di header dashboard | UI |
| 4. Pemilihan organization di mobile | UI |
| 5. Penyesuaian salinan teks & onboarding | Copy |

**Migration database: tidak ada. Data yang perlu di-backfill: tidak ada. Downtime: tidak ada.**

Perkiraan: beberapa hari, dikerjakan kapan saja tanpa tekanan.

---

# 10. Dampak terhadap Architecture Freeze

| Bagian freeze | Perubahan |
|---|---|
| **B1 — email unik global** | ✅ Tidak berubah. Justru diperkuat. |
| **B2 — login saat > 1 membership** | ✅ Tidak berubah. Rekomendasi yang sama, kini dengan alasan yang lebih kuat. |
| **B4 — undangan untuk email yang sudah ada** | ✅ Tidak berubah. Kini menjadi alur utama, bukan kasus tepi. |
| **8.3 `memberships`** | ⚠️ Perlu satu baris tambahan: **secara eksplisit menyatakan tidak ada `UNIQUE(user_id)`**, agar session mendatang tidak menambahkannya karena mengira itu kelalaian |
| **Decisions §3 (multi-org)** | ⚠️ Diperjelas: multi-org **didukung schema**, **tidak diekspos di UI** |

Selain itu, freeze tidak terpengaruh. Tidak ada tabel yang berubah bentuk.

---

# 11. Keputusan

**Disetujui 17 Agustus 2026.** Seluruh rekomendasi di bagian 8 diterima tanpa perubahan.

| # | Keputusan | Status |
|---|---|---|
| 1 | Model produk = Option A — 1 user → 1 organization | ✅ |
| 2 | Schema = User → N Membership | ✅ |
| 3 | **Tanpa** `UNIQUE(user_id)` pada `memberships` | ✅ |
| 4 | Email unik global pada `users` | ✅ |
| 5 | Existing user boleh diundang ke organization lain | ✅ |
| 6 | Login 1 membership → langsung masuk | ✅ |
| 7 | Login > 1 membership → layar pemilihan organization | ✅ |
| 8 | Tanpa organization switcher | ✅ |
| 9 | Tanpa UI create-organization | ✅ |
| 10 | Subscription tetap milik Organization | ✅ |
| 11 | Multi-tenant & tenant isolation tetap wajib | ✅ |

# 12. Konsekuensi

## Positif

- Alur undangan employee berfungsi untuk semua kasus, termasuk email yang sudah punya akun
- Employee pindah perusahaan tidak memerlukan akun baru; riwayat di organization lama tetap utuh
- Migrasi ke Option B (bila suatu saat dibutuhkan) tidak memerlukan migration data — cukup penambahan UI
- Tidak ada fragmentasi akun, sehingga analitik pengguna dan dukungan pelanggan tetap akurat

## Negatif

- Satu percabangan tambahan di endpoint login, dan satu layar pemilihan yang akan dilihat sebagian kecil pengguna
- Schema mengizinkan keadaan yang tidak diekspos produk — **harus dijaga dengan test**, bukan dengan constraint

## Konsekuensi yang harus ditegakkan

| # | Kewajiban | Ditegakkan di |
|---|---|---|
| 1 | Registrasi selalu membuat organization baru; email yang sudah ada ditolak dengan arahan untuk login | Service layer |
| 2 | Tidak ada endpoint atau UI untuk membuat organization kedua | Absennya endpoint |
| 3 | Undangan untuk user yang sudah ada **wajib** melalui login, tidak boleh menyetel password | Service layer + test keamanan |
| 4 | Token selalu terikat satu `organization_id` | Aturan #5 freeze |
| 5 | Test isolasi wajib mencakup kasus user dengan dua membership | Harness Phase 1 |

> Kewajiban #5 sengaja ditulis: karena schema mengizinkan keadaan yang tidak diekspos UI, satu-satunya penjaga adalah test. Tanpa kasus ini di harness, kebocoran lintas organization pada akun multi-membership tidak akan pernah tertangkap.
