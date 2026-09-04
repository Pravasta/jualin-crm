# Phase 8 — Subscription · Issues

> Indeks. **Tanpa kolom status** — status hidup di [GitHub Issues](https://github.com/Pravasta/jualin-crm/milestone/11) (ADR-008).
> Apa & kenapa di [`prd.md`](./prd.md) · bagaimana di [`td.md`](./td.md).

**Milestone:** [Phase 8 — Subscription](https://github.com/Pravasta/jualin-crm/milestone/11)

---

## Daftar

| # | Judul | Aplikasi | Cakupan | TD |
|---|---|---|---|---|
| [112](https://github.com/Pravasta/jualin-crm/issues/112) | Domain `subscription` penuh: peta kanal + `plan` di `GET /v1/me` | `crm_be` | `plan.go` (peta, jantung phase), `port.go`, `usecase.go`, `FindActiveByOrg`, `auth.PlanResolver` | §1, §2, §4, §6 |
| [113](https://github.com/Pravasta/jualin-crm/issues/113) | Gerbang paket di usecase: tiga kanal + `plan_upgrade_required` | `crm_be` | `PlanGate` per konsumen, bridge di composition root, urutan role→paket, error code baru | §3, §5, §11 |
| [114](https://github.com/Pravasta/jualin-crm/issues/114) | Dashboard: keadaan "terkunci oleh paket" di Connect | `crm_dashboard` | Tiga keadaan kartu, `plan` di sesi, `lib/plan.ts`, penanganan `403` balapan | §8, §4 |
| [115](https://github.com/Pravasta/jualin-crm/issues/115) | Dokumentasi + penutup Phase 8 | `crm_be` + docs | `api.md`, `authorization.md`, `testing/flow/`, review 10 AC. **Penutup phase** | §12, §15 |

---

## Urutan

```
#112 ──► #113 ──► #114 ──► #115
```

Berurutan penuh — tidak ada yang bisa paralel, sama seperti Phase 7 dan berbeda dari Phase 6 (#85 ‖ #86).

| Dependensi | Sifat |
|---|---|
| #113 → #112 | **Keras.** Tidak ada yang bisa digerbangi sebelum ada peta yang menjawab "boleh atau tidak". |
| #114 → #112 | **Keras.** Kartu terkunci merender `plan.channels` yang lahir di #112. |
| #114 → #113 | **Keras dalam praktik.** Kartu terkunci tanpa gerbang server hanyalah UI yang berbohong — persis yang ADR-012 §3 tolak. Menggabungkan keduanya sebagai "terlihat sekaligus ditegakkan" adalah satu-satunya urutan yang jujur. |
| #115 → semuanya | **Keras.** Prosedur verifikasi hanya bisa ditulis setelah gerbangnya benar-benar menolak sesuatu. |

**#112 dan #113 sengaja dipisah** meski keduanya murni Go dan tidak besar. Alasannya sama seperti
`apikey` #46/#47, `form` #85/#87, dan `webhook` #100/#101: satu issue membangun **kapabilitasnya**,
issue berikutnya membangun **yang memakainya**. Menggabungkannya menghasilkan PR yang mencampur dua
kelas review — *"apakah petanya benar"* dan *"apakah gerbangnya dipasang di tempat yang tepat"*.

---

## Batas per issue

| Issue | Setelah selesai, yang **belum** ada |
|---|---|
| #112 | `RequireChannel` ada dan diuji penuh, tapi **nol pemanggil** — tidak satu pun endpoint menolak apa pun. Preseden `apikey.FindByKeyID` (#46), `form.FindByPublicKey` (#85), `webhook.ClaimDue` (#100) |
| #113 | Gerbang menolak sungguhan lewat `curl`, tapi **tidak terlihat pengguna** — kartu Connect masih ketiganya aktif |
| #114 | Owner melihat dan merasakan batas paket, tapi **belum ada prosedur verifikasi tertulis** dan dokumen arsitektur belum menyebut gerbangnya |
| #115 | Phase 8 tutup. Harga, limit, `usage_counters`, dan payment service **belum disentuh** — dan memang tidak boleh |

Yang di luar batas ini ada di [`prd.md`](./prd.md) bagian *Di luar cakupan*, dan bersifat mengikat.

---

## Kenapa empat issue, dan kenapa tidak ada yang lebih besar

Phase ini **kecil secara sengaja**. Tidak ada migration (TD §1), tidak ada tabel baru, tidak ada env
var, tidak ada pihak ketiga, dan tidak ada satu pun angka. Yang dibangun hanyalah: satu peta, satu
pembaca, tiga titik pasang, dan satu keadaan UI.

Ukuran itu **hasil dari keputusan D1 dan D2**, bukan kebetulan. `freeze.md` 8.4 membayangkan Phase 8
sebagai `plans` + `subscriptions` + `usage_counters` + penegakan limit; phase ini menolak dua dari
empat karena angkanya belum ada. Kalau `usage_counters` ikut, phase ini akan punya migration, jalur
tulis di banyak usecase, dan dashboard pemakaian — untuk kuota yang belum satu pun ditetapkan.

**#113 yang paling perlu perhatian saat review**, dan bukan karena ia besar. Dua hal di dalamnya gagal
secara **senyap**:

1. **Urutan `authz` → `plan` terbalik** membocorkan keadaan paket organization kepada role yang tidak
   berhak mengelola kanalnya. Tidak ada error, tidak ada test yang gagal secara alami — hanya kode
   `403` yang salah isinya. Karena itu urutannya diuji dengan fake yang menolak **keduanya sekaligus**.
2. **Gerbang gagal terbuka.** `plan_code` adalah `text` tanpa `CHECK`, dan nilainya bisa datang dari
   luar CRM. Peta yang mengembalikan "boleh" untuk paket tak dikenal akan terlihat benar di setiap
   test yang memakai `free`.

---

## Setelah keempatnya selesai

Phase 8 tutup bila **10 acceptance criteria** di [`prd.md`](./prd.md) terpenuhi — terutama #3 (penolakan
dibuktikan lewat `curl`, bukan UI), #4 (gerbang terbukti bisa gagal), #8 (gagal tertutup), dan #9
(tidak ada satu pun angka). Lalu:

0. **Cek `docs/issues/*` milik Phase 8** — checklist deviasi/keputusan dari #112–#115 yang perlu
   ditinjau ulang sebelum phase benar-benar ditutup. Tiap poin terbuka **diputuskan atau dinyatakan
   ulang dengan pemicu eksplisit**, bukan dilewati.

   > Langkah ini ada di sini sejak phase dibuka — bukan retroaktif. Phase 5 dan Phase 6 melewatkannya
   > karena tidak pernah memuatnya, dan enam berkas menumpuk tanpa dibaca (#98). Preseden yang bekerja:
   > [`04-public-api/issues.md`](../04-public-api/issues.md) dan
   > [`07-outbound-webhook/issues.md`](../07-outbound-webhook/issues.md) langkah 0.

1. `api.md` (bab *Gerbang Paket* + satu error code) dan `authorization.md` (dua pertanyaan berbeda)
   diperbarui (TD §15)
2. `docs/testing/flow/` bertambah bagian: balik satu entri peta → `403` lewat `curl` + kartu terkunci
3. `docs/STATUS.md` — Phase 8 ✅, dan `Keputusan Belum Diambil` menyatakan pricing/limit/retensi
   **tetap terbuka**
4. Keputusan pemilik produk: **Phase 7.5 (Inbound Webhook)**, atau mengejar apa yang gate freeze
   sebenarnya minta — pengguna nyata, yang butuh deployment/staging

> **Tidak ada pihak ketiga di phase ini**, dan tidak ada angka yang harus diputuskan lebih dulu. Itu
> justru bentuk yang dipilih (`prd.md`, *Kenapa mekanismenya dibangun sebelum angkanya*). **Tidak ada
> issue yang terblokir menunggu apa pun.**

> **Apa yang phase ini tidak selesaikan, dan jangan berpura-pura sebaliknya.** Setelah #115 merge,
> produk punya gerbang paket yang bekerja dan **nol paket berbayar untuk digerbangi**. Kartu terkunci
> tidak akan pernah muncul di layar siapa pun sampai `planChannels` diisi paket kedua — dan itu
> menunggu data pengguna nyata (ADR-012 §4), bukan menunggu kode.

> **Staging masih belum ada.** Phase 7 tidak mengubah itu, dan Phase 8 juga tidak. Gerbang yang
> terbukti benar di laptop belum terbukti benar di tempat pelanggan memakainya.
