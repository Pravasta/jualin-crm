# ADR-001 — Go Monolith

> **Status:** ✅ Accepted — 17 Agustus 2026

## Konteks

Jualin CRM punya beberapa domain yang secara konseptual terpisah: identity, lead, integration, notification, subscription. Godaan memisahkannya menjadi service sejak awal selalu ada, terutama karena diagram arsitekturnya terlihat rapi.

Konteks nyata: tim sangat kecil, produk belum punya pengguna, dan strategi harga menuntut biaya infrastruktur rendah.

## Keputusan

**Satu binary, satu database, satu deployment unit.**

```
Next.js (dashboard) ─┐
Flutter (mobile)    ─┼─► Go monolith ─► PostgreSQL
Sistem eksternal    ─┘
```

Modul dipisahkan sebagai **paket** di dalam `internal/`, bukan sebagai service.

Tidak ada Redis, message broker, atau search engine. PostgreSQL menangani job queue (bila nanti dibutuhkan), rate limit counter, dan pencarian.

## Alasan

| | |
|---|---|
| **Biaya ops** | Setiap komponen infrastruktur adalah beban permanen — monitoring, backup, upgrade, debugging lintas proses. Untuk tim kecil, microservices menambah 5× ops cost untuk 0 business value. |
| **Ekonomi** | Strategi harga terjangkau menuntut biaya per tenant rendah dan dapat diprediksi |
| **Transaksi** | Registrasi atomik (user + organization + membership + subscription) trivial dalam satu database, rumit lintas service |
| **Kecepatan** | Refactor lintas modul masih murah selama batasnya berupa paket |

## Konsekuensi

**Positif:** deployment sederhana · transaksi lintas domain mudah · biaya infrastruktur minimal · refactor murah

**Negatif:** seluruh aplikasi di-deploy bersama · satu bug bisa menjatuhkan semuanya · scaling hanya vertikal sampai ada pemisahan

**Mitigasi:** batas antar modul dijaga di level paket sejak awal, sehingga pemisahan nanti menjadi refactor mekanis, bukan penulisan ulang.

## Kapan dievaluasi ulang

**Bukan** karena traffic naik, jumlah baris kode bertambah, atau "sudah waktunya". Hanya bila ada **bottleneck terukur** yang tidak bisa diselesaikan dalam monolith:

- Satu domain punya profil resource yang benar-benar berbeda (mis. pemrosesan berat yang mengganggu latensi request)
- Tim sudah cukup besar sehingga koordinasi deployment menjadi hambatan nyata
- Ada kebutuhan compliance yang menuntut isolasi proses

Ketiganya harus dibuktikan dengan data, bukan diantisipasi.
