# Societas MVP Backlog

Backlog ini memecah spesifikasi aktif di `docs/` menjadi 60 task implementasi. Jalankan menurut dependency, bukan nomor semata: task tanpa dependency dapat dikerjakan paralel. Setiap task harus ditutup dengan bukti acceptance criteria, tests yang relevan, dan pembaruan dokumentasi bila kontrak berubah.

## Fase

- **0. Fondasi produk:** Task-001–005
- **1. Kontrak, storage, dan event:** Task-006–013
- **2. Control plane dan runtime:** Task-014–024
- **3. Tools, permission, approval, dan memory:** Task-025–037
- **4. Workflow general:** Task-038–044
- **5. Dashboard dan observability:** Task-045–052
- **6. Validasi dan rilis:** Task-053–060

## Task

| ID | Task | Depends on |
|---|---|---|
| [Task-001](Task-001.md) | Kunci baseline implementasi MVP | - |
| [Task-002](Task-002.md) | Bootstrap aplikasi lokal | 001 |
| [Task-003](Task-003.md) | Definisikan workspace config | 001 |
| [Task-004](Task-004.md) | Definisikan agent registry | 003 |
| [Task-005](Task-005.md) | Tetapkan kontrak domain kanonik | 001 |
| [Task-006](Task-006.md) | Bangun schema validation | 005 |
| [Task-007](Task-007.md) | Siapkan SQLite migrations | 003, 005 |
| [Task-008](Task-008.md) | Implementasikan event store | 007 |
| [Task-009](Task-009.md) | Implementasikan transactional outbox | 008 |
| [Task-010](Task-010.md) | Implementasikan Event Bus | 006, 009 |
| [Task-011](Task-011.md) | Tambahkan idempotency dan replay | 008, 010 |
| [Task-012](Task-012.md) | Sediakan API workspace dan health | 002, 007 |
| [Task-013](Task-013.md) | Sediakan CLI init dan run | 002, 003, 012 |
| [Task-014](Task-014.md) | Implementasikan agent lifecycle | 004, 010 |
| [Task-015](Task-015.md) | Implementasikan task state machine | 005, 007, 010 |
| [Task-016](Task-016.md) | Implementasikan DAG scheduler | 015 |
| [Task-017](Task-017.md) | Implementasikan delegation dan routing | 004, 016 |
| [Task-018](Task-018.md) | Buat provider interface | 001, 003 |
| [Task-019](Task-019.md) | Implementasikan model router | 018, 004 |
| [Task-020](Task-020.md) | Terapkan budget defaults dan inheritance | 003, 005 |
| [Task-021](Task-021.md) | Implementasikan budget ledger | 007, 020 |
| [Task-022](Task-022.md) | Implementasikan context assembly | 019, 023, 035 |
| [Task-023](Task-023.md) | Implementasikan artifact service | 007, 015 |
| [Task-024](Task-024.md) | Implementasikan summary manager | 018, 022, 023 |
| [Task-025](Task-025.md) | Definisikan tool capabilities | 003, 005 |
| [Task-026](Task-026.md) | Implementasikan browser dan search | 025, 029 |
| [Task-027](Task-027.md) | Implementasikan spreadsheet import | 025, 029 |
| [Task-028](Task-028.md) | Implementasikan artifact filesystem tool | 023, 025, 029 |
| [Task-029](Task-029.md) | Implementasikan policy enforcement | 003, 005, 025 |
| [Task-030](Task-030.md) | Tangani retry dan outcome unknown | 011, 025, 029 |
| [Task-031](Task-031.md) | Implementasikan approval snapshot | 005, 023 |
| [Task-032](Task-032.md) | Implementasikan approval lifecycle | 007, 010, 029, 031 |
| [Task-033](Task-033.md) | Implementasikan digest approval | 032, 023 |
| [Task-034](Task-034.md) | Implementasikan pause dan recovery | 015, 007 |
| [Task-035](Task-035.md) | Implementasikan memory store | 007, 023 |
| [Task-036](Task-036.md) | Implementasikan memory retrieval/admission | 035, 031 |
| [Task-037](Task-037.md) | Implementasikan domain review evaluator | 023, 029 |
| [Task-038](Task-038.md) | Susun role presets general | 003, 004 |
| [Task-039](Task-039.md) | Implementasikan coordinator intake | 017, 019, 022 |
| [Task-040](Task-040.md) | Implementasikan market research workflow | 026, 035, 038, 039 |
| [Task-041](Task-041.md) | Implementasikan sales analysis workflow | 027, 038, 039 |
| [Task-042](Task-042.md) | Implementasikan content drafting workflow | 038, 039, 040, 041 |
| [Task-043](Task-043.md) | Implementasikan editor review workflow | 037, 042 |
| [Task-044](Task-044.md) | Lindungi aksi publikasi eksternal | 029, 032, 043 |
| [Task-045](Task-045.md) | Bangun organization dan agent view | 012, 014 |
| [Task-046](Task-046.md) | Bangun task board | 015, 016, 045 |
| [Task-047](Task-047.md) | Bangun live event stream | 010, 045 |
| [Task-048](Task-048.md) | Bangun artifact explorer | 023, 045 |
| [Task-049](Task-049.md) | Bangun approval center | 032, 033, 045 |
| [Task-050](Task-050.md) | Bangun usage dan budget dashboard | 019, 021, 045 |
| [Task-051](Task-051.md) | Bangun workspace settings | 003, 029, 045 |
| [Task-052](Task-052.md) | Tambahkan observability dan diagnostics | 008, 010, 012 |
| [Task-053](Task-053.md) | Tambahkan contract tests | 005, 006 |
| [Task-054](Task-054.md) | Tambahkan task dan delegation tests | 015, 016, 017 |
| [Task-055](Task-055.md) | Tambahkan event persistence tests | 008, 009, 011 |
| [Task-056](Task-056.md) | Tambahkan budget dan retry tests | 020, 021, 030 |
| [Task-057](Task-057.md) | Tambahkan approval security tests | 029, 031, 032, 033 |
| [Task-058](Task-058.md) | Jalankan end-to-end workflow MVP | 040, 041, 042, 043, 044, 047, 048, 049 |
| [Task-059](Task-059.md) | Jalankan reliability dan security gates | 026, 027, 028, 030, 034, 036, 057 |
| [Task-060](Task-060.md) | Siapkan release MVP lokal | 013, 050, 051, 052, 053, 054, 055, 056, 058, 059 |

## Aturan Eksekusi

- Gunakan [Baseline Implementasi MVP](../docs/00-mvp-implementation-baseline.md) sebagai rujukan Task-001. Task yang menyentuh keputusan di sana harus menautkannya dan tidak boleh mengunci keputusan berstatus **Decision required** tanpa mencatat opsi, dampak, serta owner.
- Jangan mulai task jika dependency belum selesai atau kontrak interface-nya belum tersedia.
- Bila task menemukan keputusan produk/teknis yang belum ditentukan, catat opsi, dampak, dan rekomendasi di task; jangan mengarang perilaku diam-diam.
- Task selesai hanya saat acceptance criteria lulus dan bukti (test, screenshot, trace, atau artifact) dilampirkan.
- Task 058 adalah demo MVP general: riset pasar dan analisis penjualan berjalan paralel, konten dibuat dan direview, publikasi menunggu approval.
