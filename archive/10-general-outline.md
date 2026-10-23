<!-- Doc 10 — Kontrak General, MVP & Roadmap (§72A–105)
     Source of truth untuk kontrak dan acceptance MVP general. -->

# 72A. General Contracts & MVP

Dokumen ini menetapkan kontrak aktif untuk workspace multi-agent general. Format implementasi bebas selama memenuhi invariants dan acceptance di bawah. Kontrak historis yang tidak termasuk scope aktif disimpan terpisah di arsip.

## 72A.1 Aturan Dasar (Invariants)

1. Agent tidak memanggil agent lain secara langsung. Semua komunikasi dimediasi Control Plane.
2. Agent hanya meminta delegasi; Orchestrator memvalidasi dan membuat task.
3. Budget child tidak boleh melampaui sisa budget parent.
4. Model call wajib melalui reserve, policy check, dan usage settlement.
5. Payload event inline dibatasi; konten besar disimpan sebagai artifact dan direferensikan.
6. Event append-only dan memiliki urutan per run.
7. Idempotency key mencegah request yang sama dieksekusi berulang.
8. Event memiliki correlation dan causation ID, kecuali event root.
9. Type dan payload divalidasi terhadap kontrak sebelum diterima.
10. Tool call terstruktur divalidasi; output invalid mengikuti retry budget.
11. Status terminal task bersifat final. Pekerjaan ulang membuat task baru.
12. Batas delegasi, depth, fan-out, concurrency, dan iterasi ditegakkan backend.
13. Setiap run berakhir dengan satu event terminal yang memiliki reason.
14. Agent tidak dapat mengubah budget atau policy-nya sendiri.
15. Event dipersist sebelum disalurkan ke subscriber.
16. Tool non-idempotent dengan outcome tidak diketahui tidak boleh diulang sebelum rekonsiliasi atau keputusan human.
17. Approval mengikat snapshot immutable atas aksi, input, versi sumber, dan policy yang disetujui.
18. Digest approval hanya mengelompokkan tampilan; setiap item divalidasi dan diberi grant sendiri.
19. Sumber yang berubah membatalkan approval terkait.
20. Validator bersifat domain-specific dan opsional; timeout dan retry mengikuti capability tool.
21. Penanganan kegagalan jaringan deterministik dan tidak menghabiskan budget model.
22. Memory retrieval memeriksa scope, sumber, versi, freshness, dan status supersede.
23. Session policy menentukan context sticky atau per-step serta kapan sesi di-flush.

## 72A.2 Identifier & Addressing

Workspace, run, task, event, message, conversation, artifact, tool call, dan approval memiliki ID unik yang stabil. ID agent adalah slug konfigurasi workspace. Address actor menggunakan bentuk `agent:<id>`, `human:<id>`, atau `system:<component>`; semua actor divalidasi terhadap workspace/run yang aktif.

## 72A.3 Event Envelope

Setiap event membawa protocol/schema version, ID, type, timestamp, workspace/run/correlation ID, sender, recipient, payload, dan principal terautentikasi. Event terkait task membawa `task_id`; event turunan membawa `causation_id`. Payload memakai object JSON dengan schema sesuai event type.

Contoh konseptual:

```json
{
  "protocol_version": "societas/1",
  "schema_version": "1",
  "id": "evt_example",
  "type": "task.created",
  "workspace_id": "ws_demo",
  "run_id": "RUN-001",
  "task_id": "TASK-001",
  "from": "system:orchestrator",
  "to": "agent:market_researcher",
  "payload": {"goal": "Research target market"}
}
```

Implementasi wajib melengkapi timestamp, correlation, principal, dan field envelope lain sesuai transport/auth contract.

## 72A.4 Common Definitions

Timestamp disimpan dalam format UTC yang tervalidasi. Checksum memakai `sha256:` lowercase atas byte konten yang ditentukan. Reference artifact selalu mengikat ID, versi, dan checksum; jangan mengganti versi yang diminta dengan versi terbaru secara implisit.

## 72A.5 Core Objects

Task memiliki goal, owner, creator, status, priority, dependency, deadline, risk, budget, dan output references. Artifact memiliki ID, versi, type, owner, source references, checksum, timestamp, dan metadata. Budget mencatat limit, reservation, settlement, dan usage aktual.

### Usage

Usage mencatat input/output token, provider/model, durasi, tool calls, retry, serta cost jika tersedia.

## 72A.6 Task State Machine

Task menjalani status nonterminal seperti `pending`, `ready`, `running`, `blocked`, `awaiting_approval`, `paused`, dan `interrupted`, lalu berakhir pada `completed`, `failed`, atau `cancelled`. Transisi hanya dilakukan Orchestrator melalui event terdaftar. Task terminal tidak dibuka kembali.

## 72A.7 Event Registry

Registry aktif minimal mencakup event workspace/agent, task lifecycle dan delegation, message, model/budget, tool call, artifact, review, approval, memory, pause/resume, serta run lifecycle. Menambah event memerlukan type unik, payload schema, sender/recipient rules, dan behavior retry yang terdokumentasi.

## 72A.8 Approval & Tool Contracts

Approval request mengikat `action`, `workspace_id`, `run_id`, `task_id`, `policy_hash`, input references berversi, expiry, risk, dan delivery mode dalam immutable snapshot. `bound_hash` adalah SHA-256 atas canonical JSON snapshot. Grant/reject harus menggemakan approval ID dan hash yang sama. Aksi MVP mencakup tool yang memerlukan approval dan perubahan budget; publikasi eksternal tidak dijalankan sebelum grant valid.

Tool request mengikat tool name, argumen ternormalisasi, operation key bila didukung, dan versi resource yang relevan. Runtime mengulang validasi policy dan resource tepat sebelum dispatch.

## 72A.9 Error Catalog

Error memiliki code stabil, category, retryable flag, outcome (`not_started`, `completed`, atau `outcome_unknown` bila berlaku), serta detail ringkas yang aman untuk user. Retry dibatasi budget dan capability tool; error tidak boleh menyamarkan aksi eksternal yang hasilnya belum diketahui.

## 72A.10 End-to-End Workflow

1. Intake mengubah request menjadi brief berisi goal, sumber, output, batasan, dan risk hint.
2. Router memilih agent terdaftar dan model sesuai budget/policy.
3. Orchestrator membuat DAG task dan menjalankan dependency yang siap.
4. Agent mengirim pesan melalui Event Bus, memanggil tool berizin, dan menyimpan artifact berversi.
5. Reviewer memeriksa goal, sumber, metrik, asumsi, serta policy sesuai jenis output.
6. Approval Center meminta keputusan human untuk aksi eksternal yang dilindungi.
7. Sistem mengirim hasil, evidence, artifact, dan status task kepada user.

## 72A.11 General MVP Acceptance

MVP end-to-end menggunakan request strategi konten berdasarkan tren pasar dan performa penjualan:

- Market Researcher dan Sales Analyst menerima task terpisah yang dapat berjalan paralel.
- Riset mencatat sumber; analisis mencatat periode, metrik, dan keterbatasan data.
- Content Creator membuat draft dari artifact hasil riset dan analisis.
- Editor mengembalikan review yang tertaut pada versi draft.
- Event trace menunjukkan intake, delegation, task dependency, tool call, artifact, review, approval, dan hasil akhir.
- Publish tool tidak dipanggil sebelum approval; reject tidak menimbulkan side effect; grant sekali menghasilkan paling banyak satu eksekusi.
- Integration tests memakai provider dan tool mock, lalu memverifikasi event, status task, artifact/version/checksum, approval, serta jumlah side effect.
- Role, model, tools, permissions, budget, dan concurrency dapat dikonfigurasi.

## 72A.12 Versioning & Compatibility

Perubahan kontrak menaikkan schema version atau menambah field optional yang kompatibel. Field wajib baru atau perubahan semantik memerlukan versi baru. Reader menolak versi yang tidak didukung secara eksplisit.

## 72A.13 Contract Test Requirements

Unit tests memeriksa schema dan state transitions. Integration tests mencakup delegasi, retry, idempotency, penyimpanan artifact, approval, pause/resume, dan recovery. End-to-end MVP memakai mock provider/tool, data sintetis, serta assertions atas trace event dan side effect. Tes audit dokumen bukan pengganti tes runtime.

# 79. MVP

MVP harus membuktikan satu workflow general sebelum menambah banyak fitur.

```text
Human
  -> Coordinator
       -> Market Researcher ----+
       -> Sales Analyst --------+-> Content Creator -> Editor -> Human
```

Capabilities: konfigurasi agent, runtime, task/delegation, messaging, event stream, artifact, memory dasar, tool execution, human approval, dashboard dasar, Event Bus internal, orchestrator, budget/context manager, model router, cost tracking, dan policy engine.

# 80. MVP Demo

```text
societas init my-workspace
societas serve
societas start
```

User meminta strategi konten dari tren pasar dan data penjualan bulanan. Riset dan analisis berjalan paralel, Content Creator menyusun draft, Editor memeriksa sumber dan klaim, lalu Coordinator menyajikan hasil. Publikasi eksternal menunggu approval human.

Artifacts: `market-research.md`, `sales-analysis.csv`, `content-draft.md`, `review.md`.

# 81. MVP Acceptance Criteria

MVP dianggap selesai saat:

- minimal empat agent terkonfigurasi dapat berjalan dan statusnya terlihat
- task dapat dibuat, didelegasikan, dijalankan sesuai dependency, dan mencapai status terminal
- pesan dan event dapat di-stream serta ditelusuri per run
- browser/search dan filesystem artifact dapat dipakai dengan permission; spreadsheet/tool tambahan bersifat konfigurabel
- artifact dapat dibuat, diberi versi, dan dibaca agent lain
- approval dapat diberikan atau ditolak; tool eksternal terlindungi oleh approval
- budget, context, model selection, token/cost usage, iteration, fan-out, dan stop condition ditegakkan
- envelope, payload, registry, dan task transition tervalidasi
- integration test end-to-end pada §72A.11 lulus

# 82. Phase Roadmap

## v0.1 — Foundation

- Workspace/config, agent runtime, provider interface, task model, event model, SQLite, dan Web API.

## v0.2 — Multi-Agent

- Delegasi, messaging, task graph, artifact, dan memory dasar.

## v0.3 — Real-Time Layer

- Event Bus, streaming, subscriptions, presence, reconnect, heartbeat, dan backpressure.

## v0.4 — Tools and Approvals

- MCP client, permissions, risk policy, approvals, dan audit trail.

## v0.5 — Dashboard and General Workflow

- Dashboard task/artifact/approval dan demo konten-riset-penjualan end-to-end.

# 99. First Killer Demo

Coordinator membagi request strategi konten kepada Market Researcher dan Sales Analyst. Keduanya menghasilkan artifact bersumber, Content Creator menyusun draft, Editor memeriksa klaim, dan user menyetujui publikasi dari Approval Center.

# 101. Final Product Vision

Societas mengoordinasikan agent yang dapat dikonfigurasi untuk domain pengguna: role, state, task, tools, artifacts, review, dan approval. Sistem bekerja paralel, menjaga sumber dan izin tetap terlacak, serta menyerahkan keputusan penting kepada manusia.

# 104. Absolute MVP Scope

Satu flow yang wajib berfungsi:

```text
Human -> Coordinator
          -> Market Researcher --+
          -> Sales Analyst ------+-> Content Creator -> Editor
                                                    -> Approval -> Human
```

Flow mencakup task, delegation, agent messaging, tool call, artifact, event stream, human approval, dashboard dasar, budget/context control, dan validasi kontrak. Tidak ada ketergantungan pada source repository atau alat development.

# 105. Anti-Boncos Rules

1. Jangan broadcast full context ke semua agent.
2. Gunakan summary dan artifact reference.
3. Gunakan model murah untuk routing dan summarization.
4. Gunakan model kuat hanya ketika dibutuhkan.
5. Semua task memiliki token/cost budget.
