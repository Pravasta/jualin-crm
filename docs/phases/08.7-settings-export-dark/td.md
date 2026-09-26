# Phase 8.7 — Pengaturan Akun, Pengingat Tugas, Unduh Laporan, Mode Gelap · TD

> **Bagaimana.** Apa & kenapa di [`prd.md`](./prd.md). Delta saja. Aturan yang sudah ada di
> [`freeze.md`](../../architecture/freeze.md) dirujuk, tidak diulang.

---

## 1. Schema — `migrations/0011_settings_reminders.sql`

```sql
CREATE TABLE notification_preferences (
    id                uuid PRIMARY KEY,                 -- UUIDv7 dari aplikasi (Aturan #12)
    organization_id   uuid NOT NULL REFERENCES organizations (id),
    membership_id     uuid NOT NULL,
    push_enabled      boolean NOT NULL DEFAULT true,
    task_due_reminder boolean NOT NULL DEFAULT true,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_notification_preferences_id_org UNIQUE (id, organization_id),
    CONSTRAINT uq_notification_preferences_member UNIQUE (organization_id, membership_id),
    CONSTRAINT fk_notification_preferences_member
        FOREIGN KEY (membership_id, organization_id) REFERENCES memberships (id, organization_id)
);

ALTER TABLE tasks ADD COLUMN reminder_sent_at timestamptz;

ALTER TABLE notifications DROP CONSTRAINT ck_notifications_type,
  ADD CONSTRAINT ck_notifications_type CHECK (type IN
    ('lead_assigned','task_assigned','plan_quota_exceeded','task_due_soon'));
```

- **Baris preferensi tidak ada = default** (keduanya `true`). Baris dibuat saat pertama kali disimpan (upsert).
  Tidak ada backfill, dan membership baru tidak perlu baris.
- **Index klaim pengingat** mengikuti preseden antrian lintas-organization `webhook_deliveries` (Phase 7 TD §1.2): worker
  adalah infrastruktur, organization-nya adalah **keluaran** klaim, bukan masukan.
  `CREATE INDEX ix_tasks_reminder_due ON tasks (due_at) WHERE status = 'open' AND deleted_at IS NULL AND reminder_sent_at IS NULL AND due_at IS NOT NULL;`
  Pengecualian yang disengaja terhadap Aturan #16 (tanpa awalan `organization_id`), ditulis di komentar migration.
- `down`: drop index, kolom, tabel, dan kembalikan `CHECK` lama (aman hanya sebelum ada baris `task_due_soon`, sama
  seperti `0010`).

---

## 2. Endpoint

| Method & path | Principal | Isi | Catatan |
|---|---|---|---|
| `PATCH /v1/me/profile` | user, semua role | `{full_name}` | 1–120 karakter setelah trim. Mengembalikan bentuk `/v1/me`. `audit_log` |
| `PATCH /v1/organization` | user, **Owner/Admin** | `{name}` | Action baru `organization.update`. Manager/Employee → 403. `audit_log` |
| `POST /v1/auth/password/change` | user, semua role | `{current_password, new_password}` | Lihat §3 |
| `GET /v1/me/notification-preferences` | user, semua role | → `{push_enabled, task_due_reminder}` | Default bila belum ada baris |
| `PUT /v1/me/notification-preferences` | user, semua role | `{push_enabled, task_due_reminder}` | Upsert. Hanya milik membership sendiri |

**Tidak ada `organization_id` atau `membership_id` di body mana pun** (Aturan #5): semuanya dari principal. Preferensi
selalu milik membership pemanggil, jadi tidak ada endpoint untuk mengubah preferensi orang lain.

---

## 3. Ganti password

1. Rate limit **per user** (kunci: user id) dengan limiter login yang sudah ada (progressive backoff, TD Phase 1 §11).
   Password lama salah menambah hitungan.
2. Verifikasi `current_password` dengan argon2id. Salah → `401 invalid_credentials` (kode yang sama dengan login, tanpa
   petunjuk tambahan).
3. `new_password` ≥ 12 karakter dan ≠ password lama → `400 validation_failed` per field.
4. Dalam satu transaksi: simpan hash baru, lalu **cabut semua refresh token milik user kecuali family sesi ini** (family
   dibaca dari cookie refresh request). Sesi lain di perangkat lain keluar pada refresh berikutnya. Access token yang
   masih hidup berakhir sendiri dalam ≤ 15 menit, dan itu keterbatasan yang diterima (tidak ada denylist access token).
5. `audit_log`: `password.changed`. **Tidak ada nilai password di log mana pun** (Aturan #26).
6. Tanpa email pemberitahuan (K3: tidak ada email baru di phase ini). Dicatat sebagai keputusan, bisa ditambah nanti.

---

## 4. Pengingat jatuh tempo

**Worker di dalam proses**, pola `webhook.Worker`: `time.Ticker` di goroutine yang dijalankan composition root, dan
berhenti bersama context server. Tanpa cron eksternal, tanpa broker (freeze). Konfigurasi: `TASK_REMINDER_INTERVAL`
(default 5 m) dan `TASK_REMINDER_BATCH` (default 100), divalidasi saat boot (ADR-010).

**Klaim (aman untuk banyak instance):**

```sql
UPDATE tasks SET reminder_sent_at = now()
WHERE id IN (
  SELECT t.id FROM tasks t
  JOIN leads l ON l.id = t.lead_id AND l.organization_id = t.organization_id AND l.deleted_at IS NULL
  WHERE t.status = 'open' AND t.deleted_at IS NULL AND t.reminder_sent_at IS NULL
    AND t.assigned_to_membership_id IS NOT NULL
    AND t.due_at > now() AND t.due_at <= now() + interval '24 hours'
  ORDER BY t.due_at
  LIMIT $1
  FOR UPDATE OF t SKIP LOCKED)
RETURNING id, organization_id, lead_id, assigned_to_membership_id, title, due_at;
```

Satu baris hanya bisa diklaim sekali (`reminder_sent_at IS NULL` + `SKIP LOCKED`). Dua instance tidak pernah
mengirim pengingat yang sama.

**Untuk setiap tugas yang diklaim:**
1. Baca preferensi penerima. Bila `task_due_reminder = false`, tidak ada yang dikirim; tugas tetap ditandai terkirim
   supaya tidak diperiksa ulang tiap tick.
2. Membership penerima harus masih aktif. Bila tidak, lewati.
3. Buat notifikasi in-app `task_due_soon` lewat `Notifier` yang sudah ada. Judul: *"Tugas jatuh tempo: {title}"*,
   `lead_id` dan `task_id` terisi supaya mengetuknya membuka lead.
4. **Setelah** transaksi (Aturan #32): push lewat `device.PushToMembership` bila `push_enabled`. Kegagalan push
   dicatat dan tidak diulang. In-app tetap ada.

**Jatuh tempo diubah:** `task.Usecase` yang mengubah `due_at` mengosongkan `reminder_sent_at`, supaya tugas yang
dijadwal ulang diingatkan lagi. Tugas yang dibuat dengan jatuh tempo < 24 jam diingatkan pada tick berikutnya.
**Tidak ada** pengingat untuk tugas yang sudah lewat jatuh tempo (PRD *Di luar cakupan*).

**Push yang sudah ada menghormati preferensi:** `device.PushToMembership` membaca `push_enabled` penerima sebelum
mengirim. Ini satu titik, jadi `lead_assigned`, `task_assigned`, dan `task_due_soon` berperilaku sama. Notifikasi
in-app tidak terpengaruh.

---

## 5. Dashboard

### 5.1 Pengaturan (`settings-screen.tsx`)

Empat bagian: **Profil** (nama bisa diubah; email dan role hanya dibaca) · **Organization** (nama, bisa diubah
Owner/Admin, dan Manager hanya membaca) · **Keamanan** (ganti password: lama, baru, ulangi baru) · **Notifikasi**
(dua saklar) · **Tampilan** (Terang/Gelap/Sistem, §5.3). Setiap form menyimpan sendiri. Kesalahan tampil per field.
Setelah simpan nama, sesi dimuat ulang (`useSessionRefresh`) supaya header ikut berubah.

### 5.2 Unduh Laporan (`reports-screen.tsx`)

- **CSV**: dibangun di browser dari data blok yang **sudah dimuat** (tanpa request baru) oleh fungsi murni
  `lib/report-csv.ts` (dengan test). Satu berkas, bagian per blok dipisah baris kosong, kepala berisi periode dan filter.
  UTF-8 **dengan BOM** supaya Excel membaca huruf Indonesia. Pemisah koma. Angka tanpa pemisah ribuan, dan persentase
  sebagai bilangan bulat di kolom "(%)" supaya tidak salah dibaca oleh locale desimal-koma. Nama berkas
  `laporan-jualin-{dari}-{sampai}.csv`. Tombol mati sampai semua blok selesai dimuat.
- **PDF**: `window.print()` dengan stylesheet `@media print` (sembunyikan sidebar, bilah bawah, filter, dan tombol;
  tampilkan kepala cetak berisi nama organization, periode, dan filter; blok tidak terpotong di tengah halaman;
  warna tetap). Pengguna memilih "Simpan sebagai PDF" di dialog cetak. **Tanpa library PDF.**

### 5.3 Mode gelap

- **Token `.dark` dirancang ulang** di `globals.css` (bawaan shadcn yang ada sekarang abu dingin dan primary putih,
  tidak pernah dirancang). Netral hangat hue 60, primary oranye yang lolos untuk teks putih **dan** sebagai teks di atas
  latar gelap, serta `destructive`. **Setiap pasangan dihitung** oleh `labels.test.ts` yang diperluas untuk blok
  `.dark`.
- **Warna status**: varian gelap per status di `STATUS_META` (teks/tepi di atas `--card` gelap ≥ 4.5:1), dipakai lewat
  CSS variable supaya `StatusBadge`, chip, dan grafik mengikuti tema tanpa JavaScript. Tint status (94% putih) diganti
  campuran ke `--card` di mode gelap.
- **Tanpa kedip**: skrip inline kecil di `<head>` (`layout.tsx`) membaca `localStorage['jualin-theme']` dan
  `prefers-color-scheme`, lalu memasang kelas `dark` **sebelum** render pertama. Pilihan per perangkat, tidak disimpan
  di server.
- Pengalih: di **Pengaturan → Tampilan**, plus tombol ikon di header untuk akses cepat.
- Sisa warna mentah di layar (bayangan FAB, `bg-white` di badge) diganti token.

---

## 6. Mobile

- Layar **Pengaturan notifikasi** (dua saklar) dari menu akun di kerangka aplikasi, memakai
  `GET/PUT /v1/me/notification-preferences`. Menyimpan butuh sinyal; saat offline saklar mati dengan keterangan
  (pola §12.6 brief 8.6).
- Notifikasi `task_due_soon` tampil di tab Notifikasi dengan label dan ikon sendiri. Mengetuknya membuka lead, dan push
  membuka lead dengan jalur deeplink yang sudah ada.
- Ganti password dan ubah nama **tidak** di mobile (Employee memakai "lupa password"). Dicatat.

---

## 7. Otorisasi

| Action | Owner | Admin | Manager | Employee |
|---|---|---|---|---|
| `organization.update` (baru) | ✅ | ✅ | — | — |
| Ubah profil sendiri, ganti password sendiri, preferensi sendiri | ✅ | ✅ | ✅ | ✅ |

Tiga yang terakhir tidak butuh Action: selalu terbatas pada user/membership pemanggil.

---

## 8. Rencana test

| Lapis | Isi |
|---|---|
| Repository | Upsert preferensi; **klaim pengingat**: jendela 24 jam, tidak mengklaim yang sudah diklaim, tugas selesai/terhapus/tanpa penanggung jawab/lead terhapus/sudah lewat, `due_at` diubah → dapat diklaim lagi |
| **Konkurensi** | Dua klaim **paralel** atas baris yang sama → total satu (Postgres asli, bukan berurutan. Pelajaran #19 dan #102) |
| **Isolasi tenant** | Preferensi dan profil tidak bisa menyentuh membership/organization lain; `PATCH /v1/organization` hanya organization sendiri |
| Usecase | Password lama salah → `invalid_credentials` + dihitung limiter; sesi lain dicabut, sesi ini tidak; preferensi `false` → tidak ada push/in-app pengingat; push `lead_assigned` menghormati `push_enabled` |
| Handler | Validasi per field, otorisasi per role (`organization.update`) |
| Dashboard | `report-csv.ts` (BOM, kutip, bagian, persen); kontras token `.dark`; skrip tema memilih `dark` dari `localStorage`/sistem |
| Mobile | Mapping tipe `task_due_soon`; saklar mati saat offline |
| Manual | `docs/testing/flow/11`: tugas jatuh tempo 1 jam lagi → pengingat di lonceng + HP; ganti password → perangkat lain keluar; unduh CSV → buka di Excel/Sheets; cetak PDF; mode gelap di semua layar |

---

## 9. Risiko

| Risiko | Penanganan |
|---|---|
| Pengingat ganda di banyak instance | Klaim `UPDATE … SKIP LOCKED … RETURNING`, dengan test konkurensi sungguhan |
| Push gagal setelah klaim | In-app sudah tercatat. Push tidak diulang, cukup dicatat di log. Jangan menahan klaim demi push (Aturan #32) |
| CSV salah dibaca Excel locale Indonesia | BOM + persen bulat + tanpa pemisah ribuan. Diuji manual di Excel **dan** Sheets |
| Mode gelap merusak kontras yang sudah dijaga | Test kontras diperluas ke `.dark`. Sapuan tata letak dijalankan di kedua tema |
| Kedip tema terang saat memuat | Skrip inline sebelum render, dengan test |
