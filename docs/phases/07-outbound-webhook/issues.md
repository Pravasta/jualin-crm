# Phase 7 — Outbound Webhook · Issues

> Indeks. **Tanpa kolom status** — status hidup di [GitHub Issues](https://github.com/Pravasta/jualin-crm/milestone/10) (ADR-008).
> Apa & kenapa di [`prd.md`](./prd.md) · bagaimana di [`td.md`](./td.md).

---

## Daftar

| # | Judul | Cakupan | TD |
|---|---|---|---|
| [100](https://github.com/Pravasta/jualin-crm/issues/100) | Migration `0008`, `safedial`, domain `webhook`, CRUD endpoint | Dua tabel, daftar tolak IP, lima berkas ADR-011, 5 endpoint kelola | §1, §2, §3, §6, §8 |
| [101](https://github.com/Pravasta/jualin-crm/issues/101) | Signature HMAC + enqueue: pemicu dari `lead` | `signature.go`, `Enqueue`, bridge `WebhookEnqueuer` | §2, §5 |
| [102](https://github.com/Pravasta/jualin-crm/issues/102) | Worker: klaim, kirim, retry, reaper, retensi | Infrastruktur async pertama di produk | §3.2–3.3, §4, §9, §10 |
| [103](https://github.com/Pravasta/jualin-crm/issues/103) | Dashboard: `/connect/webhook` | Endpoint, riwayat pengiriman, kirim ulang | §6, §7 |
| [104](https://github.com/Pravasta/jualin-crm/issues/104) | Dokumentasi verifikasi + penutup Phase 7 | `/connect/webhook/docs`, dokumentasi arsitektur, 14 AC | §2, §12, §15 |

---

## Urutan

```
#100 ──► #101 ──► #102 ──► #103 ──► #104
```

Berurutan penuh — tidak ada yang bisa paralel, berbeda dari Phase 6 (#85 ‖ #86).

- **#101 butuh #100** — `Enqueue` menulis ke tabel yang belum ada sebelum `0008`.
- **#102 butuh #101** — worker tidak punya apa pun untuk diklaim sampai ada yang menulis baris `pending`.
- **#103 butuh #102** — riwayat pengiriman kosong dan tombol kirim ulang tidak ada artinya sampai
  pengiriman sungguhan pernah terjadi.
- **#104 butuh semuanya** — contoh verifikasi signature hanya bisa jujur setelah payload sungguhan
  pernah terkirim dan diverifikasi.

**#100 dan #101 sengaja dipisah** meski keduanya murni Go dan tidak besar. Alasannya sama seperti
`apikey` #46/#47 dan `form` #85/#87: satu issue membangun **penyimpanan kredensial**, issue berikutnya
membangun **yang memakainya**. Menggabungkannya menghasilkan PR yang mencampur dua kelas review —
"apakah skemanya benar" dan "apakah pemicunya di tempat yang tepat".

---

## Batas per issue

| Issue | Setelah selesai, yang **belum** ada |
|---|---|
| #100 | Belum ada yang menulis baris pengiriman, belum ada yang mengirimnya. `ClaimDue`/`Reap`/`Purge` ada tapi **nol pemanggil** — preseden `apikey.FindByKeyID` (#46), `form.FindByPublicKey` (#85) |
| #101 | Baris `pending` menumpuk dan **tidak pernah terkirim**. Itu keadaan yang diinginkan, bukan bug |
| #102 | Pengiriman jalan penuh, tapi **hanya bisa dikelola lewat `curl`** |
| #103 | Owner bisa mengelola semuanya, tapi **penerima belum punya dokumentasi** cara memverifikasi signature |
| #104 | Phase 7 tutup. Inbound webhook (7.5) dan penegakan paket (8) belum disentuh |

Yang di luar batas ini ada di [`prd.md`](./prd.md) bagian *Di luar cakupan*, dan bersifat mengikat.

---

## Kenapa lima issue, dan kenapa worker punya issue sendiri

**#102 sengaja berdiri sendiri.** Ia infrastruktur async pertama di seluruh produk — antrian tahan
crash, klaim aman lintas instance, retry berjeda, pemulihan baris menggantung, retensi. Setiap satu
dari lima hal itu punya cara gagal yang berbeda, dan tiga di antaranya **hanya muncul di bawah
konkurensi atau setelah crash** — dua keadaan yang tidak pernah terlihat di test berurutan.

Menempelkannya ke issue lain berarti bagian paling sulit di-review dari phase ini masuk sebagai
tambahan pada PR yang sudah membahas hal lain. Preseden yang sama: halaman embed Phase 6 dipisah jadi
#88 justru karena ia kelas kemampuan baru (HTML, CSP, escaping), bukan karena ia besar.

**`safedial` masuk #100, bukan issue sendiri.** URL tidak boleh **tersimpan** tanpa divalidasi, jadi
validatornya harus lahir bersama tempat pertama yang membutuhkannya. Memisahkannya berarti #100
menyimpan URL yang belum tervalidasi — walau sementara, itu keadaan yang tidak boleh pernah ada di
`main`.

---

## Setelah kelimanya selesai

Phase 7 tutup bila **14 acceptance criteria** di [`prd.md`](./prd.md) terpenuhi — terutama #1 (request
sungguhan sampai ke penerima nyata), #4 (SSRF ditolak dua kali), dan #12 (dua instance tidak mengirim
ganda). Lalu:

0. **Cek `docs/issues/*` milik Phase 7** — checklist deviasi/keputusan dari #100–#104 yang perlu
   ditinjau ulang sebelum phase benar-benar ditutup. Tiap poin terbuka **diputuskan atau dinyatakan
   ulang dengan pemicu eksplisit**, bukan dilewati.

   Yang sudah ada:
   - [`101-webhook-signature-enqueue.md`](../../issues/101-webhook-signature-enqueue.md) — penyimpangan
     Aturan #20 (`whsec_` dienkripsi, bukan di-hash) yang **belum tercermin di `freeze.md`**; baris
     keempat `authentication.md` yang masih kurang; `delivery_id` yang harus disuntikkan #102;
     rotasi kunci enkripsi; dan timestamp `+07:00` vs. Aturan #33 yang lebih luas dari Phase 7.
     Berisi juga **kontrak kabel untuk #102** — empat nilai yang kalau tidak dicocokkan persis akan
     gagal diam-diam, bukan melempar error. **Keempatnya sudah dipenuhi dan dibuktikan di #102.**
   - [`102-webhook-worker.md`](../../issues/102-webhook-worker.md) — **TD §4.1 masih menggambarkan
     `SKIP LOCKED` seolah ia yang menjamin exactly-once**, padahal terbukti bukan (predikat status yang
     menjamin; `SKIP LOCKED` menjamin liveness) — perlu diperbaiki supaya orang berikutnya tidak
     menghapus predikat status. Plus: ambang reaper yang sengaja tidak configurable, biaya
     `DisableKeepAlives` yang belum diukur, dan retensi malas yang kini ada di dua tempat.
   > Langkah ini ada di sini sejak phase dibuka — bukan retroaktif. Phase 5 dan Phase 6 melewatkannya
   > karena tidak pernah memuatnya, dan enam berkas menumpuk tanpa dibaca (#98). Preseden yang bekerja:
   > [`04-public-api/issues.md`](../04-public-api/issues.md) langkah 0.
1. `api.md` (bab *Webhook Keluar* + dua error code), `authentication.md`, `authorization.md`,
   `multi-tenancy.md` diperbarui (TD §15)
2. `docs/testing/flow/` bertambah berkas: mendaftarkan endpoint, menerima pengiriman sungguhan,
   memverifikasi signature dari sisi penerima
3. `docs/STATUS.md` — Phase 7 ✅
4. Buka **Phase 7.5 — Inbound Webhook** atau **Phase 8 — Subscription**, sesuai keputusan pemilik produk

> **Tidak ada pihak ketiga di phase ini.** Berbeda dari Phase 5 (Firebase) dan Phase 6 (Turnstile),
> tidak ada akun yang perlu diurus lebih dulu — verifikasi manual cukup dengan server penerima lokal
> (`WEBHOOK_ALLOW_PRIVATE_TARGETS=true`) atau layanan penampung publik gratis. **Tidak ada issue yang
> terblokir menunggu apa pun.**

> **Staging masih belum ada.** Phase 7 tidak mengubah itu — dan di phase ini absennya lebih terasa:
> webhook keluar adalah fitur pertama yang perilakunya bergantung pada **di mana ia berjalan**
> (resolusi DNS, IP keluar, jaringan yang bisa dijangkau). Yang terbukti benar di laptop tidak otomatis
> benar di produksi.
