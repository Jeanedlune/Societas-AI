# Baseline Implementasi MVP

Dokumen ini mencatat keputusan fondasi dari [Task-001](../tasks/Task-001.md). Keputusan ini membatasi pilihan implementasi untuk MVP; keputusan yang belum disepakati harus tetap berstatus **Decision required** sampai diputuskan oleh owner terkait.

## Keputusan

| Area | Keputusan | Status | Alasan | Owner |
|---|---|---|---|---|
| Bahasa dan runtime | Go untuk seluruh backend dan runtime. | Diputuskan | Doc 10 §86 menetapkan Go untuk runtime agent, Event Bus, API, dan concurrency; kontrak Section 72A tetap netral bahasa. | Architecture / Runtime |
| Framework UI | **Decision required.** Dashboard web adalah target produk, tetapi framework UI belum ditetapkan. | Terbuka | Doc 08 §38–39 memberi gambaran Web Dashboard dan struktur `web/`, tanpa memilih framework. Hindari mengunci dependency frontend sebelum keputusan ini dibuat. | Product / UI |
| Provider model awal | Gunakan mock provider untuk bootstrap dan workflow end-to-end lokal. Provider model nyata **Decision required**; Task-001 tidak memilih provider berbayar. | Diputuskan untuk dev; terbuka untuk provider nyata | Doc 10 §81 dan §82 menetapkan interface provider lokal; acceptance Task-058 secara eksplisit memakai provider/tool mock. Interface harus netral terhadap vendor. | AI Platform / Provider |
| Deployment lokal | Self-hosted, satu proses aplikasi lokal; backend Go menyediakan HTTP API dan berjalan di loopback secara default (`127.0.0.1`). Tidak membutuhkan layanan cloud Societas. | Diputuskan | Selaras dengan prinsip run locally di Doc 01 §3.1, konfigurasi server Doc 07 §36, serta strategi single-binary Go Doc 10 §86. | Runtime / Release |
| Storage operasional | SQLite sebagai source of truth operasional dan event store; metadata di SQLite, berkas artifact di filesystem workspace. | Diputuskan | Ditetapkan Doc 07 §37.1 dan Doc 08 §42.1; event harus commit sebelum delivery. | Storage |
| Domain contracts | Package Go `internal/contracts/v1` diturunkan dari Doc 10 §72A; schema JSON hasil ekstraksi dan model/event registry Go adalah generated output. Pin `go-jsonschema` v0.24.1 untuk model dan `github.com/santhosh-tekuri/jsonschema/v6` v6.0.3 untuk validasi Draft 2020-12. | Diputuskan | §72A tetap satu-satunya sumber schema dan event aktif. Validasi memakai embedded local schemas dan kode error stabil; detail input tidak dimasukkan ke error yang dikembalikan. | Architecture / Runtime |
| Cognitive / vector storage | **Decision required** untuk implementasi MVP. Doc 07 §37.2 menyebut ChromaDB atau `sqlite-vec` sebagai opsi, bukan keputusan. Jangan jadikan pilihan vector DB dependency bootstrap. | Terbuka | Workflow MVP dapat disiapkan dengan operational store dan mock; pemilihan harus menimbang deployment lokal dan kebutuhan retrieval yang sudah disepakati. | AI Platform / Storage |
| Mekanisme config | `workspace.yaml` untuk konfigurasi workspace; schema dan defaults ditetapkan Task-003. Secret hanya melalui environment/secret reference, tidak pernah literal di contoh config. | Diputuskan | Format dan contoh kanonik ada di Doc 07 §36; Task-003 menentukan schema, loader, dan validator. | Runtime / Security |

## Assumption register

### Kontrak domain Task-005

- `docs/10-contracts-mvp-roadmap.md` §72A adalah sumber kanonik. `internal/contracts/v1/schema/` dan `events_gen.go` diekstrak oleh `tools/contracts/extract_schemas.py`; model payload Go dibangkitkan oleh `tools/contracts/generate_types.py` dan `go generate ./internal/contracts/v1`.
- Versi generator dipatok ke `github.com/atombender/go-jsonschema@v0.24.1` dan dijalankan dengan `--only-models`. Schema kanonik tidak dimodifikasi untuk codegen; referensi eksternal hanya di-inline pada salinan sementara generator karena generator membutuhkan definisi properti referensi di file yang sama. Validasi dan penolakan payload invalid tetap tanggung jawab Task-006.
- `internal/contracts/v1/testdata/` menyediakan fixture payload konten, riset, sales, approval, dan payload invalid. Keduanya—schema hasil ekstraksi dan fixture—tidak boleh diedit langsung sebagai pengganti perubahan §72A.
- Task-006 memakai `jsonschema/v6` v6.0.3 untuk mendukung Draft 2020-12, format assertion, dan local schema references. Semua schema dipasang dari `go:embed`; pemrosesan event tidak mengambil schema dari jaringan. Error boundary hanya mengekspos kode tetap (`INVALID_JSON`, `INVALID_ENVELOPE`, `UNSUPPORTED_VERSION`, `UNKNOWN_EVENT_TYPE`, `INVALID_PAYLOAD`).

### Fakta dari spesifikasi aktif

- Produk adalah workspace multi-agent general yang dapat dijalankan sendiri secara lokal; tidak ada layanan cloud Societas yang wajib (Doc 01 §1–3).
- Workflow konten adalah use case end-to-end MVP: request strategi konten memicu riset pasar dan analisis penjualan paralel, penyusunan draft, review editor, lalu approval sebelum publikasi (Doc 10 §81; Task-058).
- Go dipakai untuk backend (Doc 10 §86); SQLite adalah operational store, dan file artifact disimpan terpisah (Doc 07 §37).
- Konfigurasi workspace menggunakan YAML dan secret direferensikan melalui environment, bukan ditulis sebagai nilai literal (Doc 07 §36).
- Tidak ada fitur workflow implementasi software yang menjadi dependency produk MVP. Task atau modul yang dibuat harus mendukung fondasi general atau workflow konten di atas.

### Asumsi sementara untuk implementasi

- Bootstrap dan demo lokal dapat memakai mock provider/tool sampai provider nyata diputuskan; mock tidak dimaknai sebagai provider produksi.
- Satu proses lokal dengan bind loopback cukup untuk MVP awal; kebutuhan multi-user/jaringan dan packaging platform spesifik belum disepakati.
- Framework UI dan vector store dapat ditunda selama modul awal belum membutuhkannya; jika dependency task membutuhkannya, owner harus menyelesaikan keputusan terkait lebih dulu.

### Pertanyaan terbuka

- Framework UI apa yang dipilih, dan apakah UI MVP dibangun bersamaan dengan backend?
- Provider model nyata pertama apa yang akan didukung? Keputusan harus mempertimbangkan adapter netral, model yang tersedia, kebijakan privasi, dan biaya; jangan menyematkan pilihan vendor berbayar dalam baseline ini.
- Apakah MVP memerlukan vector DB sejak alur awal, dan jika ya, apakah ChromaDB, `sqlite-vec`, atau opsi lain yang dipilih?
- Bagaimana bentuk distribusi lokal pertama (binary per OS atau cara lain), di luar kontrak satu proses lokal?

## Aturan handoff

1. Task implementasi yang menyentuh runtime, UI, provider, deployment, storage, atau config harus merujuk baseline ini dan tidak diam-diam mengubah keputusan.
2. Task yang bergantung pada keputusan berstatus terbuka harus mencatat opsi, dampak, dan owner keputusan; jangan membangun dependency produk di atas asumsi.
3. Alur penerimaan MVP tetap strategi konten end-to-end. Software-development workflow, coding agent, atau fitur sejenis bukan dependency produk MVP.
4. Perubahan keputusan diperbarui di dokumen ini dan ditautkan dari task terkait.
