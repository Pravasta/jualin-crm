# crm_employee

Aplikasi mobile untuk Employee. **Flutter**, dipin lewat FVM ke `3.44.0` (`.fvmrc`).

Cakupan: login + biometric · My Leads · My Tasks · lead detail · Call/WhatsApp dengan auto-Activity · ubah status · catatan · push notification · cache baca offline.

**Autentikasi memakai user session (JWT + refresh token opaque), bukan API key** — Aturan #24, lihat `docs/architecture/freeze.md` bagian 5.1.

**Android saja untuk sekarang** — iOS digenerate `flutter create` tapi tidak pernah dibangun atau diuji (keputusan M1, `docs/phases/05-employee-mobile/td.md` §1).

Perintah lewat `Makefile` di akar repo (`make mobile-get`, `make mobile-analyze`, `make mobile-test`, `make mobile-run`), atau `fvm flutter …` langsung di direktori ini.

Lihat `docs/architecture/freeze.md` bagian 4 dan `docs/phases/05-employee-mobile/` untuk detail.
