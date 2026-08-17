Refs #

<!--
⚠️ Gunakan "Refs #N", JANGAN "Closes/Fixes/Resolves #N".

Kata kunci penutup membuat GitHub menutup issue otomatis saat merge — sebelum
branch lokal dibersihkan dan merge diverifikasi. Penutupan issue dilakukan
manual setelah merge. Lihat docs/workflow.md bagian 3.
-->

## Apa yang berubah

<!-- Ringkas. Reviewer harus paham tanpa membaca seluruh diff. -->

## Kenapa

<!-- Konteks singkat, atau tautan ke bagian TD. -->

## Penyimpangan dari TD

<!-- Kosongkan bila tidak ada. Bila ada: apa yang berbeda dan kenapa.
     Yang ditulis di sini juga harus masuk ke notes.md. -->

## Cara menguji

```bash
```

## Checklist

- [ ] Test lolos
- [ ] **Test isolasi tenant** ditambahkan/diperbarui bila menyentuh tenant boundary
- [ ] Tidak ada `organization_id` di DTO manapun
- [ ] Composite FK terpasang pada FK baru antar entity bisnis
- [ ] Tidak ada efek samping eksternal (email, HTTP) di dalam transaksi database
- [ ] Migration punya `down` yang bekerja
- [ ] Tidak ada rahasia ter-commit
- [ ] `notes.md` diperbarui
- [ ] `docs/STATUS.md` diperbarui bila perlu

## Catatan untuk reviewer

<!-- Bagian yang paling perlu diperhatikan, atau keputusan yang Anda ragu. -->
