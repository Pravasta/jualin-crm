# Phase 6 — Connect & Embedded Form · Issues

> Indeks pekerjaan. **Tanpa kolom status** — status hidup di GitHub ([ADR-008](../../decisions/ADR-008-delivery-workflow.md)).
>
> Status terkini: `gh issue list --milestone "Phase 6 — Connect & Embedded Form"`

**Milestone:** [Phase 6 — Connect & Embedded Form](https://github.com/Pravasta/jualin-crm/milestone/9)

---

## Daftar

| # | Judul | Aplikasi | Cakupan | TD |
|---|---|---|---|---|
| [85](https://github.com/Pravasta/jualin-crm/issues/85) | Migration `0007_forms`, domain `form`, CRUD kredensial | `crm_be` | `0007_forms` + `leads.source_form_id`, `internal/form`, format `pk_` (D3), 5 endpoint pengelolaan, audit log | §1, §2, §8, §11 |
| [86](https://github.com/Pravasta/jualin-crm/issues/86) | Permukaan Connect + pindahkan pengelolaan API key | `crm_dashboard` | `NAV_ITEMS`/`NAV_ICONS`/`nav.test.ts`, `/connect` + tiga kartu, `/connect/api(/docs)` pindahan, redirect rute lama | §10 |
| [87](https://github.com/Pravasta/jualin-crm/issues/87) | Endpoint submit publik + lima lapis anti-spam | `crm_be` | `POST /v1/forms/{public_key}/submit`, `PrincipalPublicForm` + `FormID`, cabang `authz` ketiga, Origin/honeypot/time-trap/rate limit/Turnstile. **Risiko keamanan tertinggi phase ini** | §3–§6, §9 |
| [88](https://github.com/Pravasta/jualin-crm/issues/88) | Halaman embed (iframe) + CSP per-form | `crm_be` | `GET /embed/{public_key}` + `GET /embed.js`, `html/template` + `embed.FS`, `frame-ancestors` per-form, auto-resize `postMessage` (D8). **Kelas kemampuan baru** | §7 |
| [89](https://github.com/Pravasta/jualin-crm/issues/89) | Manajemen form + snippet embed | `crm_dashboard` | Layar buat/ubah/nonaktifkan, toggle+label field, allowlist, snippet **dua varian** (D8). **Penutup phase** | §10, §12, §15 |

> **[#91](https://github.com/Pravasta/jualin-crm/issues/91)** menambahkan keputusan **D8** (tinggi
> iframe & script auto-resize) setelah phase dibuka — celah yang ditemukan saat membahas bentuk snippet,
> sebelum #88/#89 dikerjakan. Dokumen saja; cakupannya sudah terserap ke dua baris di atas.

---

## Urutan

```
#85 domain ──┬──► #87 submit publik ──┬──► #89 UI + penutup
             └──► #88 halaman embed ──┘
#86 Connect ───────────────────────────────► #89
```

| Dependensi | Sifat |
|---|---|
| #87 → #85 | **Keras.** Tidak ada yang bisa di-resolve sebelum kredensialnya ada. |
| #88 → #85 | **Keras.** Halaman perlu konfigurasi form untuk dirender. |
| #87 ↔ #88 | **Bukan prasyarat satu sama lain.** Boleh paralel. #88 menanam token yang #87 validasi, tapi keduanya bisa ditulis terpisah — kontraknya `formtoken` (TD §6), bukan urutan pengerjaan. |
| #86 → apa pun | **Tidak ada.** Boleh dikerjakan kapan saja, termasuk pertama. |
| #89 → #85, #86 | **Keras.** Butuh endpoint pengelolaan **dan** rumah `/connect`. |

**#86 boleh mendahului #85.** Sisanya berurutan sesuai graf.

---

## Batas per issue

| Issue | Berhenti di |
|---|---|
| #85 | Form bisa dibuat, dilihat, dan dinonaktifkan lewat `curl` **sebagai user**. Kredensialnya **belum bisa dipakai untuk apa pun** — tidak ada endpoint yang menerimanya. |
| #86 | `Connect` jadi menu, API key pindah, tautan lama tetap hidup. Kartu Formulir boleh mengarah ke rute yang **belum berisi**. |
| #87 | `curl` dari mesin luar membuat lead lewat `public_key`. **Belum ada halaman yang menyajikan formnya**, belum ada UI. |
| #88 | Halaman form tampil dan bisa di-iframe sesuai allowlist. Submit-nya ditangani #87. |
| #89 | Phase 6 tutup. Webhook (Phase 7) dan penegakan paket (Phase 8) **tidak** disentuh. |

Yang di luar batas ini ada di [`prd.md`](./prd.md) bagian *Di luar cakupan*, dan bersifat mengikat.

---

## Dua issue yang perlu perhatian ekstra saat review

**#87 — kredensialnya memang terbuka, jadi daftar kemampuannya yang harus tertutup.**

Ini bukan hiperbola: `public_key` tertanam di HTML setiap halaman yang memasang form. Siapa pun yang
membuka *view source* memilikinya. Yang membatasi kerusakannya **bukan** kerahasiaan kunci, melainkan
`publicFormAllows` yang berisi tepat satu baris.

Yang layak direview **bukan** jumlah test yang lolos, melainkan **bentuk** dua hal:

1. Apakah penolakan datang dari *"action ini tidak ada di peta"* (gagal aman — action baru di phase
   manapun otomatis tertutup) atau dari *"handler ini mengecek"* (gagal terbuka — handler yang lupa
   otomatis terbuka)?
2. Apakah test-nya **tabel atas seluruh `authz.Action`**, atau daftar action tulisan tangan? Yang kedua
   hijau hari ini dan bocor di phase berikutnya, saat seseorang menambah action dan tidak ada yang
   mengingatkannya.

Ditambah satu yang khas issue ini: **honeypot yang bisa dibedakan tidak ada gunanya.** Bila bot bisa
membedakan sukses palsu dari sukses sungguhan — lewat status code, bentuk body, atau **waktu
respons** — ia akan belajar, dan lapisannya hilang tanpa ada yang sadar.

**#88 — halaman HTML pertama di backend, dan satu-satunya jalur XSS di phase ini.**

Label field berasal dari input pelanggan dan dirender ke HTML. `html/template` melakukan escaping
kontekstual; `text/template` tidak. Keduanya punya API yang **hampir identik**, dan tertukarnya tidak
menghasilkan error apa pun — hanya kerentanan yang diam.

`frame-ancestors` juga harus per-form, bukan satu kebijakan global. Form dengan `allowed_origins`
kosong wajib **gagal tertutup** (`'none'`), bukan terbuka.

---

## Setelah kelimanya selesai

Phase 6 tutup bila **14 acceptance criteria** di [`prd.md`](./prd.md) terpenuhi — terutama #1
(snippet ditempel di halaman kosong → lead masuk) dan #2 (`public_key` tidak bisa membaca apa pun).
Lalu:

1. `api.md`, `authentication.md`, `authorization.md`, `multi-tenancy.md` diperbarui (TD §15)
2. `docs/testing/flow/` bertambah berkas: memasang form dan mengisinya dari browser
3. `docs/STATUS.md` — Phase 6 ✅, **kunci Turnstile** masuk *Punya Lead Time*
4. Buka **Phase 7 — Webhook**, kanal ketiga di `Connect` yang sudah berdiri

> **Sebelum #87 dikerjakan, urus akun Cloudflare Turnstile** (TD §14). Gratis dan hitungan menit, tapi
> ia satu-satunya pihak ketiga di phase ini. `CAPTCHA_PROVIDER=none` membuat #85, #86, #88, dan #89
> jalan penuh tanpanya — yang terhalang hanya verifikasi anti-spam sungguhan di #87.

> **Staging masih belum ada.** Phase 6 tidak mengubah itu. Tim QA tetap terhambat sampai deployment
> diurus terpisah — dan kriteria #1/#10 phase ini (menempel snippet ke halaman sungguhan) akan jauh
> lebih mudah diverifikasi setelah ada.
