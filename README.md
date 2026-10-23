# Societas — General Multi-Agent Workspace

Societas adalah workspace multi-agent general untuk workflow seperti pembuatan konten, riset pasar, dan analisis penjualan. Sepuluh dokumen spesifikasi aktif berada di `docs/`.
Implementation backlog: [tasks/README.md](tasks/README.md) dengan task ber-dependency dan acceptance criteria.
Nomor section dipertahankan untuk menjaga referensi silang (`#48`, `5A.8`, `72A.5`, `I3`, dst);
urutan onboarding mengikuti peta dokumen di bawah.

Nomor yang loncat (34–35, 73–78, 84, dan lainnya) sengaja dipertahankan dari penomoran spesifikasi awal, bukan tanda ada file hilang.

**Urutan baca linear (onboarding):** Doc 01 §1–5 → Doc 03 §6 (agent lifecycle), §7 (task), §8 (DAG) → Doc 02 §5A (control plane) → kembali ke Doc 03 §7.1/§9 (intake/delegation) → Doc 04–10. Detail intake §7.1 dapat dilewati pada pembacaan pertama; nomor section dan file tetap stabil.

## Aturan Revisi

- Revisi dilakukan di file topik terkait, bukan di dokumen gabungan.
- Nomor section **jangan diubah** — referensi silang antar-file bergantung padanya.
- Jika menambah section baru, gunakan nomor desimal di sebelah section induk
  (mis. `5A.28.x`, dengan `x` nomor subseksi baru) agar urutan tetap terbaca tanpa menggeser nomor yang sudah ada.
- **Section 72A (Doc 10) adalah Source of Truth** untuk schema, event, dan invariants —
  jika ada konflik dengan doc lain, 72A yang berlaku.

## Peta Dokumen

| File | Section | Topik |
|------|---------|-------|
| [01-vision-and-core.md](docs/01-vision-and-core.md) | 1–5 | Visi, masalah, konsep inti (workspace, agent, role) |
| [03-tasks-and-lifecycle.md](docs/03-tasks-and-lifecycle.md) | 6–9 | Agent lifecycle, task system, intake & routing pipeline (§7.1), task graph, delegation |
| [02-ai-control-plane.md](docs/02-ai-control-plane.md) | 5A.1–5A.28 | Control Plane: orchestrator, budget, context, router (Jev), policy, intake pipeline (§7.1 dirujuk dari sini) |
| [04-communication-and-events.md](docs/04-communication-and-events.md) | 10–16 | Communication model, event model/bus, message categories, streaming, agent conversation |
| [05-artifacts-memory-tools.md](docs/05-artifacts-memory-tools.md) | 17–21 | Artifact system, artifact flow, memory system, tools + MCP client |
| [06-permissions-approvals-roles.md](docs/06-permissions-approvals-roles.md) | 22–32 | Permission, approval & risk tier, autonomy, role customization, model provider, cost tracking |
| [07-dashboard-config-storage.md](docs/07-dashboard-config-storage.md) | 33–37 | Dashboard, workspace.yaml, dual-storage SQLite + Vector DB |
| [08-runtime-architecture.md](docs/08-runtime-architecture.md) | 38–56 | Runtime, event bus, retry, parallelism, conflict resolution, voice/multimodal, backpressure, retention |
| [09-security-sandbox-observability.md](docs/09-security-sandbox-observability.md) | 57–72 | Security model, secrets, tool sandbox, MCP search, observability, replay, context budget, protocol versioning |
| [10-contracts-mvp-roadmap.md](docs/10-contracts-mvp-roadmap.md) | 72A–105 | **Source of Truth**: invariants, schemas, approval, event registry, error catalog, MVP, roadmap, priorities |

## Fokus dan Arsip

Sepuluh dokumen aktif berfokus pada workflow general. Materi historis berada terpisah di `archive/` dan bukan requirement MVP.

## Titik Panas Revisi Terakhir (quick lookup)

- Dual-Storage: Doc 05 (§19.5), Doc 07 (§37)
- 2x Rebuttals & escalation_lead: Doc 02 (5A.2), Doc 06 (§30.1), Doc 07 (§36), Doc 08 (§48)
- Deterministic Guard & OS Isolation: Doc 06 (§22), Doc 09 (§59)
- Risk Tier & Runtime Escalation: Doc 06 (§23.1), Doc 10 (72A.5), Doc 02 (5A.8)
- Human Attention Budget / Digest Approval: Doc 06 (§23.2), Doc 07 (§33.7, §36–37), Doc 08 (§40.1), Doc 10 (I18, 72A.7–72A.10)
- Dynamic Context Assembly: Doc 02 (5A.4), Doc 07 (§36)
- Task Pause/Resume & Step Function: Doc 06 (§23), Doc 07 (§33.3), Doc 08 (§40), Doc 10 (72A.5–72A.7)
- MCP Client & Search: Doc 05 (§21.1), Doc 09 (§62–63)
- Domain Validation: Doc 02 (5A.22), Doc 07 (§36 optional validators), Doc 10 (I20)
- Contract-First (codegen, contract.lookup): Doc 05 (§17.1), Doc 10 (72A.5 `contract_hash`)
- OpenRouter Usage Accounting: Doc 06 (§32.1), Doc 02 (5A.10, 5A.26)
- Dynamic Enum Injection & Blind Delegation: Doc 02 (5A.8), Doc 06 (§30.1), Doc 10 (72A.8, 72A.10)
- Provider Rate Limiter (RPM/TPM token bucket): Doc 02 (5A.8)
- Network Failure Handling (Deterministic Go Runtime): Doc 02 (5A.17)
- Memory Curation / anti Zombie Memory: Doc 05 (§19.6, §20), Doc 07 (§37.3), [Doc 02 (5A.25 — Semantic Retrieval)](docs/02-ai-control-plane.md#5a25-semantic-retrieval)
- Session Policy: [I23 — sumber utama aturan mode/lifecycle/flush](docs/10-contracts-mvp-roadmap.md#72a1-aturan-dasar-invariants); Doc 02 (5A.4), Doc 07 (§36), dan Doc 08 (§40.1) hanya merujuk aturan ini.
- Review repair: tool outcome/retry (I16, §21, §44, 72A.8–10), pause recovery (72A.6), contract invalidation (§17.1, 72A.6/8), aggregate payload boundary (I5, 72A.12), semantic path guard (§22.2, §59)
- Approval snapshot SHA-256/JCS: Doc 08 (§40.2), Doc 10 (§72A.8); invalidation lewat `approval.invalidated`
- Source freshness / anti-stale memory: Doc 05 (§19.6), Doc 07 (§37.2–37.3)

## Audit Kontrak

Script audit asli yang dirujuk laporan review tidak disertakan. `audit-spec.py` adalah **pengganti transparan**: ia memeriksa link dokumen aktif dan membaca schema/fixture historis dari `archive/10-contracts-legacy.md`. Konten arsip dipertahankan untuk pemeriksaan kompatibilitas, bukan sebagai requirement MVP aktif.

```bash
python3 -m venv .venv-audit
.venv-audit/bin/pip install -r requirements-audit.txt
.venv-audit/bin/python audit-spec.py --output validation-results.json
.venv-audit/bin/python test-audit-spec.py
# Opsional: periksa semua nomor heading lama tetap ada dan berurutan
.venv-audit/bin/python audit-spec.py --baseline-ref <commit-sebelum-revisi>
# Opsional: checks untuk kode audit
.venv-audit/bin/pip install ruff==0.11.13 mypy==1.15.0 \
  types-PyYAML==6.0.12.20250915 types-jsonschema==4.25.1.20251009
.venv-audit/bin/ruff check audit-spec.py test-audit-spec.py
.venv-audit/bin/mypy audit-spec.py test-audit-spec.py
```

Exit `0` = pemeriksaan statis lulus; exit `1` = kegagalan parse/schema/registry/contoh/fixture/probe/penomoran/referensi dokumentasi. Audit memeriksa sintaks JSON/YAML, Draft 2020-12, ID unik, `$ref`, registry, schema yatim, contoh kanonik, tabel transisi, aggregate payload, serta fixture approval historis. Audit juga memeriksa fence tertutup, keberadaan nomor 5A/72A di prose, target file/anchor tautan Markdown aktif, kecocokan nomor label dengan section target, daftar wire event bertanda `<!-- event-types -->`, dan kelengkapan field contoh `<!-- example:workspace.yaml -->`.

Gunakan tautan dengan label nomor dan judul untuk referensi silang (mis. `[5A.25 — Semantic Retrieval](docs/02-ai-control-plane.md#5a25-semantic-retrieval)`): nomor yang masih ada tetapi menunjuk topik yang salah dapat terdeteksi lewat ketidakcocokan label/anchor. Audit **tidak menyimpulkan maksud semantik referensi nomor tanpa tautan**, dan daftar event tanpa marker tidak diperiksa terhadap registry. YAML diperiksa sintaks dan kelengkapan contoh, bukan validasi schema konfigurasi runtime.

Probe boundary adalah validator referensi spesifikasi, **bukan implementasi runtime**. Audit lulus tidak membuktikan otorisasi/approval consumption, crash recovery, rekonsiliasi provider, live resource versions, sandbox, atau memory GC — belum ada runtime di repo.
