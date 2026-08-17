# Phase 0 — Foundation · PRD

> **Apa & kenapa.** Detail teknis di [`td.md`](./td.md).
> Sumber: [`architecture/freeze.md`](../../architecture/freeze.md) bagian 4 (Phase 0) & 13.2

---

## Tujuan

Menegakkan **setiap pola yang akan ditiru oleh 15+ session berikutnya**, sebelum ada satu pun business logic yang menutupinya.

Phase ini tidak menghasilkan nilai bagi pengguna akhir. Nilainya seluruhnya internal: setelah selesai, setiap session berikutnya punya jawaban yang sudah jadi untuk pertanyaan "bagaimana cara membaca config di sini", "bagaimana bentuk error di sini", "bagaimana menulis test yang menyentuh database di sini".

> Fondasi yang dibuat sambil lalu di tengah feature auth akan ditiru oleh semua session sesudahnya. Karena itu ia dikerjakan terpisah, dengan review tersendiri.

---

## Kebutuhan

Bukan kebutuhan pengguna akhir — kebutuhan **developer yang mengerjakan phase berikutnya**.

| # | Sebagai developer, saya butuh… | Supaya… |
|---|---|---|
| 1 | Menjalankan seluruh sistem dengan satu perintah | Tidak ada langkah setup manual yang tidak tercatat |
| 2 | Aplikasi gagal seketika saat config salah, dengan pesan yang jelas | Salah konfigurasi ketahuan saat startup, bukan saat request pertama di produksi |
| 3 | Setiap baris log bisa dilacak ke satu request | Debugging tidak menebak-nebak |
| 4 | Bentuk error yang sama di seluruh endpoint | Client tidak perlu menangani banyak bentuk |
| 5 | Menjalankan & membatalkan migration dengan aman | Perubahan schema bisa dicoba tanpa takut |
| 6 | Menulis test yang menyentuh PostgreSQL **asli** | Bug isolasi tenant nanti benar-benar tertangkap — mock tidak akan pernah menangkapnya |
| 7 | CI menolak kode yang tidak lolos lint/test/build | Kualitas tidak bergantung pada ingatan reviewer |

---

## Acceptance Criteria

Phase 0 dinyatakan selesai bila **ketujuhnya** terpenuhi:

| # | Kriteria |
|---|---|
| 1 | `docker compose up` → API merespons `GET /health` |
| 2 | Migration `up` lalu `down` berjalan bersih, tanpa objek tersisa |
| 3 | Config invalid → proses **gagal saat startup** dengan pesan yang menyebut variabel mana yang bermasalah |
| 4 | Log berbentuk JSON terstruktur dan memuat `request_id` di setiap baris yang berasal dari request |
| 5 | Error mengembalikan bentuk konsisten `{"error":{"code","message"}}` sesuai [`architecture/api.md`](../../architecture/api.md) |
| 6 | `go test ./...` berjalan terhadap PostgreSQL asli tanpa setup manual |
| 7 | CI hijau: lint + test + build |

---

## Di luar cakupan

Ditulis eksplisit karena tiga di antaranya akan terasa "sekalian saja".

| Tidak dikerjakan | Ke phase |
|---|---|
| Business logic apapun | 1+ |
| Endpoint domain | 1+ |
| **Tabel domain** (`organizations`, `users`, `memberships`) | 1 |
| Autentikasi, JWT, session | 1 |
| `internal/shared/tenant` — `TenantContext` | 1 |
| Harness test isolasi tenant | 1 |
| Rate limiting | 1 |
| Pengiriman email | 1 |
| Deployment ke server / CD | Setelah Phase 5 |

> **`TenantContext` sengaja ditunda.** Ia baru punya bentuk yang benar setelah ada principal untuk diisi. Membuatnya di Phase 0 berarti menebak, dan tebakan itu akan ditiru repository pertama.

> **Tabel domain sengaja ditunda.** Migration `0001` hanya berisi utilitas lintas tabel. Tabel yang dibuat di Phase 0 akan menjadi schema mati yang tidak dipakai kode manapun.

---

## Dependensi

Tidak ada. Ini phase pertama.

**Prasyarat yang sudah terpenuhi:** architecture freeze · bootstrap documentation · repository + label + milestone · `docs/workflow.md`

---

## Pembagian issue

Freeze memetakan Foundation sebagai **satu** session. Setelah ADR-008 menetapkan 1 issue = 1 session = 1 PR, pemetaan itu menghasilkan PR berisi ~30 berkas — terlalu besar untuk direview dengan teliti, padahal justru PR inilah yang menetapkan pola untuk semua session berikutnya.

**Dipecah menjadi tiga**, dengan urutan yang mengikat:

| Urut | Issue | Alasan urutan |
|---|---|---|
| 1 | Project skeleton + config + logging + error + **CI** | CI lebih dulu supaya PR #2 dan #3 sudah tergerbang sejak awal |
| 2 | Docker Compose + PostgreSQL + migration | Butuh kerangka dari #1 |
| 3 | Test harness terhadap PostgreSQL asli | Butuh database dari #2 |

Ini **penyimpangan dari peta session di freeze**, dicatat di sini agar terlihat, bukan disembunyikan. Bila Anda lebih memilih satu PR utuh, cukup katakan saat review — issue tinggal digabung.

Rincian: [`issues.md`](./issues.md)

---

## Bukan tujuan phase ini

- **Bukan** memilih hosting atau menyiapkan deployment
- **Bukan** mengoptimasi apapun — belum ada beban untuk diukur
- **Bukan** membangun abstraksi untuk kebutuhan Phase 1 yang belum berbentuk (Aturan #27)
