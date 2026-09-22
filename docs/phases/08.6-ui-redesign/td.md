# Phase 8.6 — Redesain UI · TD

> **Bagaimana.** Apa & kenapa di [`prd.md`](./prd.md).
> Ini **delta** untuk phase ini. Aturan yang sudah ada di [`freeze.md`](../../architecture/freeze.md)
> tidak diulang, hanya dirujuk.

---

## 1. Schema — **tanpa migration**

Kelima laporan baru dihitung dari kolom yang sudah ada:

| Blok laporan | Sumber |
|---|---|
| Tren lead masuk | `leads.created_at` + `organizations.timezone` |
| Sumber lead | `leads.source` (`manual`/`api`/`form`/`webhook`, `CHECK` di `0003`) + `customers.converted_from_lead_id` untuk conversion rate per sumber |
| Alasan kalah | `leads.lost_reason` (6 nilai, `CHECK` di `0003`) |
| Waktu respons | `leads.created_at` → `MIN(activities.created_at) WHERE type <> 'lead_created'` — **definisi yang sama persis** dengan `avg_response_seconds` yang sudah dipakai `GET /v1/metrics/employees` |
| Tugas per anggota | `tasks.due_at`, `tasks.completed_at`, `tasks.status`, `tasks.assigned_to_membership_id` |

Index yang ada sudah menutupinya: `ix_leads_org_created`, `ix_leads_org_status`,
`ix_activities_org_lead`, `ix_tasks_org_assignee_due`. **Tidak ada index baru sampai ada bukti lambat**
(Aturan #27) — bila muncul, ia harus berawalan `organization_id` (Aturan #16).

> Kalau saat implementasi ternyata sebuah blok butuh kolom yang tidak ada: **berhenti dan laporkan**.
> Jangan menambah kolom diam-diam demi satu grafik.

---

## 2. Endpoint baru — lima, semuanya di `internal/metrics`

Paket `metrics` sudah menjadi rumah untuk agregat lintas domain dan sengaja **read-only** — tanpa
`Store`, tanpa `InTx` (Phase 3 TD §2). Kelima endpoint ini tinggal di sana; **tidak ada paket
`reports` baru** (Aturan #28: "Laporan" adalah nama layar, bukan domain).

Semua di bawah `GET /v1/metrics/*`, di belakang `authMW` yang sama, digerbangi
`authz.ActionMetricsRead` (Owner ✅ Admin ✅ Manager ✅ Employee ⛔) — tidak ada action baru di
`internal/shared/authz`.

| Endpoint | Menjawab |
|---|---|
| `GET /v1/metrics/trend` | Berapa lead masuk per hari/minggu |
| `GET /v1/metrics/sources` | Sebaran 4 sumber + conversion rate per sumber |
| `GET /v1/metrics/lost-reasons` | Sebaran 6 alasan kalah |
| `GET /v1/metrics/response-times` | Sebaran waktu sampai sentuhan pertama |
| `GET /v1/metrics/tasks` | Tugas selesai vs lewat jatuh tempo per anggota |

### 2.1 Filter bersama

`Filter` (`entity.go`) bertambah dua field, dan **keduanya berlaku untuk kelima endpoint baru serta
untuk `/summary` dan `/employees` yang sudah ada** — brief §10.2 meminta filter anggota dan sumber di
seluruh layar Laporan:

```go
type Filter struct {
	From   *time.Time
	To     *time.Time
	Assignee *uuid.UUID // leads.assigned_to_membership_id
	Source   *string    // leads.source
}
```

- Parsing mengikuti `parseFilter` yang ada: nilai tak terbaca **diabaikan diam-diam**, tidak ditolak.
  `source` di luar keempat nilai sah diperlakukan sebagai tidak diisi.
- `Assignee` **tidak pernah** memperluas akses: ia hanya mempersempit di dalam organization yang sama.
  Membership milik tenant lain menghasilkan daftar kosong, bukan 403/404 — ia hanya predikat.
- Rentang tetap membatasi **`leads.created_at`** (Phase 3 TD §2.1), termasuk untuk `/tasks`: satu
  definisi periode untuk seluruh layar. Lihat §2.6 untuk konsekuensinya.

### 2.2 `GET /v1/metrics/trend`

**`from` dan `to` wajib** — beda dari endpoint metrics yang ada, karena rentang tak berbatas berarti
`generate_series` tak berbatas. Tanpa keduanya, atau rentang > 366 hari, atau `to` < `from` →
`400 validation_failed` dengan `details` per field (katalog `api.md` yang ada, tanpa kode baru).

Lebar bucket **ditentukan server**, bukan client: rentang ≤ 45 hari → `day`, selebihnya → `week`
(Senin sebagai awal minggu, `date_trunc('week', …)`).

```json
{ "data": { "bucket": "day",
            "points": [ { "date": "2026-09-01", "count": 8 }, { "date": "2026-09-02", "count": 0 } ] },
  "meta": {} }
```

`date` adalah **tanggal kalender di `organizations.timezone`**, bukan timestamp — karena itu ia
`YYYY-MM-DD` dan bukan ISO 8601 `Z`. Aturan #33 mengatur timestamp; ini tanggal, dan menuliskannya
sebagai timestamp UTC justru akan menggeser hari bagi organization di WITA/WIT.

```sql
date_trunc('day', l.created_at AT TIME ZONE o.timezone)
```

**Bucket kosong tetap dikirim** (`count: 0`) lewat `generate_series` — garis tren yang melompati hari
tanpa lead adalah grafik yang berbohong.

### 2.3 `GET /v1/metrics/sources`

Keempat sumber **selalu** dikirim, termasuk yang nol.

```json
{ "data": [ { "source": "form", "count": 26, "converted_count": 5, "conversion_rate": 0.19 } ] }
```

`conversion_rate` **`null`** bila penyebutnya nol — penyebut memakai definisi yang sama dengan
`/summary`: total **dikurangi** `spam` dan `unqualified` (Phase 3 TD §2.2). Dua tempat menghitung
conversion rate dengan cara berbeda adalah bug yang menunggu dilaporkan pelanggan.

### 2.4 `GET /v1/metrics/lost-reasons`

Keenam alasan selalu dikirim, termasuk nol. Hanya lead ber-`status = 'lost'` yang dihitung;
`lost_reason` dijamin tidak NULL di sana oleh `ck_leads_lost_requires_reason`.

### 2.5 `GET /v1/metrics/response-times`

Empat bucket **tetap**, sesuai brief §10.3: `lt_1h`, `1h_4h`, `4h_24h`, `gt_24h`, ditambah
`never_touched` — lead yang belum pernah disentuh **bukan** "> 1 hari", dan menyembunyikannya membuat
layar ini memuji tim yang sebenarnya mendiamkan lead.

```json
{ "data": { "buckets": [ { "bucket": "lt_1h", "count": 18 } ], "median_seconds": 4920 } }
```

`median_seconds` `null` bila tidak ada lead yang pernah disentuh.

### 2.6 `GET /v1/metrics/tasks`

Per membership aktif (sama seperti `/employees`: **semua role**, bukan hanya Employee — penugasan tidak
dibatasi role):

```json
{ "data": [ { "membership_id": "…", "full_name": "…", "completed_count": 6, "overdue_count": 1 } ] }
```

- `completed_count`: `status = 'done'` **dan** `completed_at` di dalam rentang.
- `overdue_count`: `status = 'open'` **dan** `due_at < now()` — **snapshot saat ini, tidak mengikuti
  rentang**. "Berapa yang terlambat *pada 30 hari lalu*" tidak bisa dijawab; tabel `tasks` tidak
  menyimpan riwayat. Layar harus mengatakannya (§4.4), bukan membiarkan pengguna menyimpulkan sendiri.
- `deleted_at IS NULL` di kedua hitungan.

Ini satu-satunya endpoint yang rentangnya **tidak** menyentuh `leads.created_at`. Penyimpangan yang
disengaja dan wajib ditulis di `notes.md`.

### 2.7 Bentuk respons & error

Envelope `{data, meta}`, `snake_case`, error `{code, message}` (Aturan #33). Tidak ada kode error baru
selain pemakaian `validation_failed` di §2.2. Katalog di `architecture/api.md` ditambah lima baris
endpoint, bukan baris error.

---

## 3. Token desain dashboard

Sumber: `docs/design_handoff_jualin_crm/screens/Token Desain Jualin.dc.html`.

| Yang berubah di `crm_dashboard/src/app/globals.css` | Dari | Ke |
|---|---|---|
| `--background` | `oklch(1 0 0)` | `oklch(0.995 0.003 70)` — netral hangat |
| `--foreground` | `oklch(0.145 0 0)` | `oklch(0.22 0.012 60)` |
| `--muted-foreground` | — | `oklch(0.505 0.012 60)` |
| `--border` / `--input` | — | `oklch(0.90 0.008 60)` / `oklch(0.82 0.01 60)` |
| `--radius` | `0.625rem` | `0.5rem` |
| `--primary`, `--accent-strong`, `--destructive` | amber Jualin | **tidak berubah** |
| Font | Geist Sans + Geist Mono | **Plus Jakarta Sans** + **JetBrains Mono**, lewat `next/font/google` |

`--font-sans`/`--font-mono` di `@theme inline` sudah menunjuk variabel `next/font` (perbaikan #40);
penggantian keluarga font **tidak boleh** mengembalikan pola lama yang menunjuk dirinya sendiri.

**Kontras dihitung ulang, bukan disalin.** Preseden #40 (aksen gagal 4.45:1) dan #70 (tiga angka
desainer meleset): setiap pasangan diverifikasi dengan rumus luminansi relatif WCAG 2.1 dan tabelnya
ditulis di `notes.md`. Bila sebuah nilai gagal AA, **lightness-nya diturunkan** dan yang tertulis di
kode adalah nilai hasil koreksi.

### 3.1 Badge status — Opsi B di dashboard

Delapan status dibedakan **bentuk + tepi**, bukan warna saja (Aturan aksesibilitas brief §5.2):

| Bentuk | Status |
|---|---|
| Pil bertepi 1.5px | `new`, `contacted`, `qualified`, `proposal` — jalur utama |
| Kotak `radius 5px` bertepi 2px | `won`, `lost` — hasil akhir |
| Kotak bertepi **putus-putus** | `unqualified`, `spam` — dikecualikan dari conversion rate |

Satu komponen `components/status-badge.tsx` dipakai seluruh dashboard, termasuk sebagai warna seri di
grafik Laporan. `unqualified` dan `spam` wajib tetap terbedakan satu sama lain (brief §7.1).

---

## 4. Dashboard — struktur

Rute dan berkas yang ada **dipertahankan**; ini perombakan tampilan, bukan penataan ulang folder. Satu
rute baru: `src/app/(protected)/reports/`.

### 4.1 Kerangka responsif

`components/app-shell.tsx` menangani ketiga breakpoint (HP <768, Tablet 768–1023, Desktop ≥1024):

| Lebar | Navigasi |
|---|---|
| ≥1024 | Sidebar 232 px, label penuh, 9 item |
| 768–1023 | Sidebar 68 px, ikon saja, `title` sebagai label |
| <768 | Header atas + **bilah bawah tetap**: Beranda · Lead · Tugas · Laporan · **Lainnya** (sheet berisi Customer, Tim, Connect, Langganan, Pengaturan, identitas, Keluar) |

Breakpoint dideteksi dengan **CSS** (`@media`/varian Tailwind) sebagai sumber utama, bukan
`window.innerWidth` seperti prototipe — mengukur lebar di JavaScript membuat tata letak salah pada
render pertama dan mematahkan SSR. `useMediaQuery` hanya untuk yang tidak bisa dinyatakan di CSS
(mis. memilih Dialog vs Sheet).

Badge jumlah pada item **Lead** = lead tanpa pemilik aktif, sama seperti sekarang.

### 4.2 Tabel → kartu

Di bawah 768 px setiap tabel (lead, customer, tugas, anggota, API key, pengiriman webhook) menjadi
daftar kartu. **Tidak ada `overflow-x` pada halaman** di lebar mana pun; guliran horizontal hanya
diizinkan pada baris chip status dan stepper, yang memang dirancang begitu.

### 4.3 Dialog → sheet

Di HP, Dialog shadcn diganti Sheet dari bawah (atau layar penuh untuk form panjang). Isi, validasi per
field, dan konfirmasi lunak **tidak berubah** — hanya wadahnya.

### 4.4 Layar Laporan

`/reports`, satu kolom di HP, dua–tiga kolom di desktop. Filter global: periode, anggota, sumber (§2.1).
Delapan blok §10.3 brief. Aturan yang mengikat:

- Setiap grafik punya **padanan angka** yang terbaca (label nilai atau tabel ringkas).
- Warna seri = warna badge status yang sama (§3.1), dan **tidak pernah warna saja** sebagai pembeda.
- Blok "Distribusi status" **tidak** diberi label corong/funnel — datanya sebaran status **saat ini**.
- Conversion rate menuliskan pengecualian Spam + Tidak Memenuhi Syarat **di layar**.
- Blok tugas menuliskan bahwa "lewat jatuh tempo" adalah keadaan **sekarang**, bukan pada periode itu (§2.6).
- Keadaan: memuat (skeleton), belum ada data, data sedikit (angka tetap tampil + catatan "belum
  bermakna"), gagal memuat.
- Grafik dibangun dari `div`/SVG biasa. **Tidak ada dependensi grafik baru** (Aturan #27).

---

## 5. Mobile

`crm_employee/lib/shared/theme.dart` memuat token Phase 5 lengkap dengan rasio yang sudah dihitung
ulang saat #70. Phase ini menggantinya dengan **skala mobile** handoff (§04 Token Desain): hue sama
dengan dashboard, lightness turun ~0.09, tint 0.975, target **≥7:1** untuk teks status.

- Badge status **Opsi A** (ikon + tint) — satu widget bersama, dipakai di Lead Saya, Detail, dan
  Notifikasi.
- `primary` mobile `oklch(0.52 0.19 41)` (#bc2e00) — lebih gelap dari dashboard agar teks putih 5.98:1.
- Prosedur yang sama dengan #70 berlaku: **setiap angka dihitung ulang** dan tabelnya masuk `notes.md`.
- Lead Saya dan Detail Lead mengikuti tata letak handoff. Empat layar lain (Masuk, gerbang biometrik,
  Tugas Saya, Notifikasi) hanya mengganti token — **tata letaknya tidak diubah** (PRD K3).
- Pemilih status tetap dibangun dari transisi sah yang sudah ada, bukan dari daftar 8 status prototipe.
- Pita cache, pita pemotongan (#152), label jatuh tempo kalender, dan segmen Selesai (#148)
  dipertahankan apa adanya.

---

## 6. Otorisasi

Tidak ada perubahan. Laporan memakai `metrics.read` yang sudah ada — Owner/Admin/Manager ✅,
Employee ⛔ (`architecture/authorization.md` baris `metrics.read`). Employee tidak bisa masuk dashboard
sama sekali, jadi layar Laporan tidak butuh keadaan "tidak tersedia untuk role Anda".

Aksi yang tidak diizinkan untuk role **tidak ditampilkan**, bukan ditampilkan lalu ditolak — perilaku
yang sudah berjalan dan dipertahankan di setiap layar yang dirombak.

---

## 7. Rencana test

| Lapis | Isi |
|---|---|
| Repository (Go) | Lima query baru terhadap database sungguhan: bucket kosong ikut terkirim, alasan/sumber bernilai nol ikut terkirim, `never_touched` terpisah, batas bucket waktu respons (tepat 1 jam masuk `1h_4h`), pengelompokan tren memakai timezone organization (uji dengan `Asia/Jayapura`) |
| **Isolasi tenant (Aturan #7)** | Untuk **kelima** endpoint: data organization lain tidak pernah ikut terhitung; `assignee` milik tenant lain menghasilkan nol, bukan 403 |
| Usecase (Go) | Employee ditolak di kelima endpoint; Owner/Admin/Manager diizinkan |
| Handler (Go) | `trend` tanpa `from`/`to` → 400 `validation_failed`; rentang > 366 hari ditolak; `source` tak dikenal diabaikan; bentuk envelope |
| Dashboard (vitest) | Test yang sudah ada **tetap lolos tanpa diubah** kecuali kueri selektor berubah. Tambahan: komponen badge status, pemetaan bucket tren, keadaan "belum ada data" ≠ 0% |
| Mobile (flutter test) | Test yang sudah ada tetap lolos. Tambahan: pemilih status hanya memuat transisi sah |
| Manual, tertulis di `docs/testing/flow/` | 360 / 390 / 820 / 1440 px pada Masuk, Beranda, Daftar Lead, Detail Lead, Laporan: tidak ada guliran horizontal, target sentuh ≥44 px |

---

## 8. Risiko teknis

| Risiko | Penanganan |
|---|---|
| **Angka kontras desainer meleset** — sudah terjadi dua kali (#40, #70) | Hitung ulang seluruh pasangan sebelum menulis token. Tabel hasilnya masuk `notes.md` |
| **Perombakan visual diam-diam menghapus perilaku** (konflik 409, kredensial satu kali, dialog tiga cabang) | Test yang ada tidak boleh dilonggarkan. Bila sebuah test harus diubah, alasannya ditulis di PR |
| Query tren atas rentang panjang | `from`/`to` wajib, maksimum 366 hari, bucket minggu untuk rentang panjang |
| Delapan blok Laporan = delapan request saat halaman dibuka | Diterima untuk sekarang: tiap blok memuat sendiri dan gagal sendiri. Bila terbukti berat, penggabungan endpoint dicatat sebagai temuan, bukan dioptimalkan di depan (Aturan #27) |
| Ganti font mengubah lebar setiap baris tabel | Ganti font di issue **pertama**, sebelum layar mana pun disentuh, supaya layar berikutnya dibangun di atas metrik yang final |
| `window.innerWidth` gaya prototipe merembes ke produksi | CSS sebagai sumber breakpoint (§4.1). Ditinjau saat review tiap PR layar |
