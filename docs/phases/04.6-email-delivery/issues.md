# Phase 4.6 — Email Delivery · Issues

> Indeks pekerjaan. **Tanpa kolom status** — status hidup di GitHub ([ADR-008](../../decisions/ADR-008-delivery-workflow.md)).
>
> Status terkini: `gh issue list --milestone "Phase 4.6 — Email Delivery"`

**Milestone:** [Phase 4.6 — Email Delivery](https://github.com/Pravasta/jualin-crm/milestone/8)

---

## Daftar

| # | Judul | Aplikasi | Cakupan | TD |
|---|---|---|---|---|
| [63](https://github.com/Pravasta/jualin-crm/issues/63) | `SMTPMailer`: kirim email sungguhan, tutup `MAIL_FROM` yang tidak pernah dipakai | `crm_be` | `SMTPMailer` + `buildRFC5322`, config `SMTP_*` + validasi boot, composition root. Test unit (tanpa jaringan) + integrasi Mailpit lewat testcontainers | §2–§7, §9.1, §9.3 |
| [64](https://github.com/Pravasta/jualin-crm/issues/64) | Mailpit di dev environment + perbarui alur testing | `crm_be`, root, docs | Service `mailpit` di compose, `.env.example`, 5 berkas `docs/testing/flow/`, `authentication.md`. **Penutup phase** | §8, §9.2, §10 |

---

## Urutan

```
#63 SMTPMailer ──► #64 dev environment + docs (penutup)
```

| Dependensi | Sifat |
|---|---|
| #64 → #63 | **Keras.** Menyalakan Mailpit sebelum ada `SMTPMailer` yang bisa mengiriminya berarti menambah container yang tidak pernah menerima apa pun, dan memperbarui dokumentasi alur testing ke perilaku yang belum ada. |

**Tidak ada pekerjaan paralel.** Phase ini dua issue, berurutan.

---

## Batas per issue

| Issue | Berhenti di |
|---|---|
| #63 | SMTP **bisa** dipakai lewat env, dan terbukti mengirim ke Mailpit sungguhan di test. Tapi `make dev` **belum** menyalakan apa pun yang menerimanya, dan `docs/testing/flow/` masih menyuruh menggali log. |
| #64 | Phase 4.6 tutup. Pemilihan provider produksi, domain, dan SPF/DKIM/DMARC **tidak** disentuh. |

Yang di luar batas ini ada di [`prd.md`](./prd.md) bagian *Di luar cakupan*, dan bersifat mengikat.

---

## Kenapa hanya dua issue

Phase ini sengaja kecil. Bentuk yang dibutuhkan **sudah ada sejak Phase 1** — `mailer.Mailer` adalah
interface, dan komentarnya sendiri menyebut implementasi kedua akan datang. Yang dikerjakan di sini
hanya mengisinya, lalu membuat hasilnya terasa saat mengembangkan.

Godaan yang ditolak: menarik pemilihan provider produksi ke sini karena "sekalian sedang menyentuh
email". Justru sebaliknya — memilih SMTP sebagai transport adalah yang **membuat** keputusan provider
bisa ditunda tanpa biaya, karena Resend, Postmark, dan SES ketiganya berbicara SMTP. Menutupnya
sekarang berarti memilih di bawah tekanan tenggat demo, bukan berdasarkan kebutuhan yang terbukti.
