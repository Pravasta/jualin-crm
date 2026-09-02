-- +goose Up

-- Koreksi atas 0008. Kolom secret_hash menyimpan SHA-256 dari signing
-- secret, mengikuti pola api_keys (Aturan #20). Pola itu SALAH di sini,
-- dan 0008 sebenarnya sudah menyebut alasannya di komentarnya sendiri
-- sebelum tetap memakainya:
--
--   "kredensial keempat produk ini menerbitkan, dan yang PERTAMA dengan
--    arah kepercayaan terbalik: tiga sebelumnya (api_key, public_key,
--    device_token) untuk pihak lain membuktikan diri ke kita; signing
--    secret di sini untuk KITA membuktikan diri ke pihak lain."
--
-- Justru arah terbalik itu yang membuat hash tidak bisa dipakai. Tiga
-- kredensial sebelumnya diVERIFIKASI oleh kita: pemegang mengirimkan
-- rahasianya, kita meng-hash lalu membandingkan — hash cukup, dan hash
-- lebih aman karena kebocoran database tidak menghasilkan kredensial
-- yang bisa dipakai.
--
-- Signing secret dipakai untuk MENGHASILKAN bukti, bukan memeriksanya:
--
--     X-Jualin-Signature: t=<unix>,v1=HMAC-SHA256(secret, "<t>.<body>")
--
-- HMAC butuh secret ASLI sebagai kunci. SHA-256 searah, jadi tidak ada
-- jalur apa pun — bukan dari kolom ini, bukan dari secret_prefix (8 dari
-- 49 karakter), bukan dari pelanggan (createRequest tidak punya field
-- secret) — yang bisa mengembalikannya. Worker Phase 7 #102 tidak akan
-- pernah bisa menandatangani apa pun selama kolomnya berbentuk hash.
--
-- Penggantinya: ciphertext AES-256-GCM (internal/shared/crypter), kunci
-- dari WEBHOOK_SECRET_ENC_KEY. Ini penyimpangan TERTULIS dari Aturan #20,
-- bukan kelalaian — dicatat di td.md §2 dan docs/issues/. Enkripsi tetap
-- menjaga Aturan #21 (raw hanya tampil sekali) dan #26 (tidak pernah
-- di-log); yang dilepas hanya "kebocoran database tidak cukup untuk
-- memalsukan", dan itu memang tidak bisa dipertahankan untuk kredensial
-- yang harus kita pakai sendiri. Mitigasinya: kunci enkripsi hidup di
-- environment, bukan di database, jadi dump database saja tidak cukup.
--
-- Kolom lama DIHAPUS, bukan disimpan berdampingan: nilai di dalamnya
-- tidak bisa dikonversi (hash tidak bisa dibalik) dan tidak ada satu pun
-- yang membacanya. Endpoint yang terlanjur dibuat sebelum migration ini
-- harus dibuat ulang untuk mendapat secret yang bisa dipakai — di luar
-- lingkungan pengembangan, tabel ini masih kosong (0008 belum pernah
-- rilis ke produksi).
--
-- bytea, bukan text: keluaran crypter adalah nonce‖ciphertext‖tag biner.
-- Meng-encode-nya ke base64 hanya untuk muat di kolom text menambah 33%
-- ukuran dan satu lapis konversi tanpa memberi apa pun.

-- Baris yang sudah ada dihapus, bukan diberi ciphertext kosong sebagai
-- pengisi. Endpoint yang secret-nya tidak bisa dipulihkan bukan endpoint
-- yang setengah bekerja — ia tidak bisa mengirim satu pun pengiriman yang
-- terverifikasi, selamanya. Meninggalkannya dengan ciphertext kosong
-- hanya memindahkan kegagalan dari sini (terlihat, saat migrasi) ke
-- worker (senyap, saat produksi). Di luar pengembangan lokal tabel ini
-- masih kosong: 0008 belum pernah rilis.
--
-- Pengiriman dihapus lebih dulu — fk_webhook_deliveries_endpoint
-- menunjuk ke baris yang akan hilang.
DELETE FROM webhook_deliveries;
DELETE FROM webhook_endpoints;

ALTER TABLE webhook_endpoints DROP COLUMN secret_hash;

ALTER TABLE webhook_endpoints ADD COLUMN secret_ciphertext bytea NOT NULL;

-- +goose Down

-- Simetris dengan Up dan untuk alasan yang sama: kembali ke skema hash
-- membuat setiap secret terenkripsi yang sudah dibuat tidak bisa dipakai,
-- dan NOT NULL tanpa default tidak bisa dipasang di atas baris yang ada.
DELETE FROM webhook_deliveries;
DELETE FROM webhook_endpoints;

ALTER TABLE webhook_endpoints DROP COLUMN secret_ciphertext;

ALTER TABLE webhook_endpoints ADD COLUMN secret_hash text NOT NULL;
