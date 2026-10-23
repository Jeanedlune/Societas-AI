<!-- Doc 07 — Dashboard, Config, Storage (§33–37)
     Bagian dari societas-full-product-spec. Urutan dokumen diatur di README.md.
     Nomor section dipertahankan; referensi silang antar-doc memakai nomor section (#N, 5A.x, 72A.x). -->

# 33. Dashboard

Dashboard adalah interface utama untuk melihat organisasi.

## 33.1 Organization View

```text
SOCIETAS

Coordinator       ● Working
Content Creator   ● Drafting
Market Research   ● Searching
Sales Analyst     ● Analyzing
Editor            ○ Idle
```

---

## 33.2 Agent Detail

Menampilkan:

- role
- model
- status
- current task
- recent events
- memory
- tools
- permissions
- token usage
- artifacts

---

## 33.3 Task Board

View:

```text
Backlog
   |
   v
In Progress
   |
   v
Review
   |
   v
Done
```

Setiap kartu task aktif memiliki tombol kontrol UI:

```text
[ Pause ]   menjeda task (running -> pausing -> paused bila ada call in-flight)
[ Resume ]  melanjutkan task dengan context terakhir (paused -> running)
[ Cancel ]  membatalkan task permanen (-> cancelled)
```

Klik tombol mengirim **sinyal deterministik ke backend Go (0 token LLM)**. Kontrol yang sama tersedia via command tag di CLI/chat — backend mengintersepsi `#TASK-xxx pause` / `#TASK-xxx resume` secara deterministik tanpa memanggil LLM router.

Transisi yang diizinkan mengikuti state machine di 72A.6.

Pause memakai `task.paused` `phase: requested` (human → Orchestrator); sesudah call settle, Orchestrator menerbitkan `phase: completed`. Resume divalidasi backend lalu `task.resumed` diterbitkan Orchestrator. Deadline absolut pause dipersist di Task/SQLite dan tetap berlaku setelah restart; `running`/`pausing` pulih lewat `task.interrupted`. Cancel/TTL tidak berarti efek call pasti belum terjadi (72A.6/72A.12).

---

## 33.4 Agent Graph

Menampilkan hubungan:

```text
              Coordinator
          /       |       \
 Content Creator Research Sales Analyst
          \       |       /
                Editor
```

---

## 33.5 Live Event Stream

Contoh:

```text
12:01:03 CEO -> Research
Research agent communication patterns

12:01:05 Research started

12:01:08 Research -> CEO
Found 7 relevant implementations

12:01:09 Coordinator -> Sales Analyst
Compare this month's sales with the prior period

12:01:14 Sales Analyst started analysis
```

---

## 33.6 Artifact Explorer

User dapat melihat:

```text
Artifacts
├── content-draft.md
├── market-research.md
├── sales-analysis.csv
└── review.md
```

---

## 33.7 Approval Center

Approval Center memiliki dua jalur perhatian. Critical tampil seketika sebagai interupsi yang menghentikan aksi (`synchronous halt`); high-risk masuk antrean Digest Mode dan tidak memunculkan interupsi satu per satu.

Summary Manager mengumpulkan request high-risk yang eligible dan menampilkan kartu agregat per workspace/run saat interval terkonfigurasi atau akhir siklus. `max_items_per_card` memecah manifest besar menjadi beberapa kartu; overflow tidak di-approve otomatis. Setiap manifest `approval_batch/1` adalah artifact JCS immutable dan kartu menunjuk versi/hash yang dilihat; item baru tidak ikut klik lama. Baris memuat task, aksi/target/dampak, risk, expiry, base/kandidat, evidence, dan status valid/stale. **Inspect items** membuka item untuk approve/reject individual; **Approve All Validated** menjalankan validasi batch. Penjelasan LLM hanya teks bantu, bukan daftar otoritatif.

Klik mengirim `approval.batch_submitted` dengan manifest `batch_hash`/`batch_ref`. Backend memvalidasi actor, envelope, identity, JCS, schema, checksum, urutan serta uniqueness approval IDs. Manifest tak valid ditolak seluruhnya tanpa grant. Setelah manifest valid, tiap item menjalani revalidasi terpisah; item valid menghasilkan grant `scope: once` dengan ID/hash aslinya dan durable per-item processing receipt; item lain menampilkan `skipped` beserta alasan. User tetap dapat inspect, approve, atau reject satu item tanpa menyetujui manifest penuh. Hasil boleh parsial. Batch tidak menyetujui critical, tidak menggabungkan otorisasi, dan tidak melewati pemeriksaan final dispatch.

```text
Digest: 3 high-risk approvals pending

[Approve All Validated]
Publish content      Validated
Send campaign        Validated
Export sales report  Revalidation required
```

---

# 36. Workspace Configuration

Contoh `workspace.yaml` utuh. Nama model di bawah bersifat ilustratif dan harus diganti dengan ID model provider yang tersedia; API key memakai referensi environment, bukan nilai secret literal.

<!-- example:workspace.yaml -->
```yaml
version: "1"

workspace:
  name: "general-workspace"

server:
  host: "127.0.0.1"
  port: 7443

transport:
  event_bus: "go-channel"

providers:
  openrouter:
    api_key: "$OPENROUTER_API_KEY"
    analytics_management_key: "$OPENROUTER_MGMT_KEY"  # opsional, audit saja

models:
  router: jev-latest
  policy: jev-latest
  summarizer: cheap-model
  research: medium-model
  content_creator: strong-model
  market_researcher: medium-model
  sales_analyst: strong-model
  editor: medium-model
  ceo: strong-model

agents:
  ceo:
    role: "CEO"
    model: "strong-model"
    autonomy: 3
    permissions:
      filesystem: read

  content_creator:
    role: "Content Creator"
    model: "strong-model"
    autonomy: 2
    tools:
      - writer
      - filesystem

  market_researcher:
    role: "Market Researcher"
    model: "medium-model"
    autonomy: 2
    tools:
      - browser
      - search

  sales_analyst:
    role: "Sales Analyst"
    model: "strong-model"
    autonomy: 2
    tools:
      - spreadsheet
      - filesystem

  editor:
    role: "Editor"
    model: "medium-model"
    autonomy: 2
    permissions:
      filesystem: read

# Capability retry tepercaya (#21, I16)
tool_capabilities:
  filesystem.read:
    idempotent: true
    operation_key_enforced: false
  external.mutate:
    idempotent: false
    operation_key_enforced: false

# Pengambil keputusan akhir debat (#30.1, #48)
escalation_lead: "editor"     # atau "human" -> diserahkan ke dashboard

# Toolchain opsional untuk validasi artifact sesuai domain task
toolchain:
  type: "general"
  shared_cache_dir: "/tmp/societas-cache"
  pipeline:             # fail-fast: berhenti di step pertama yang gagal
    - name: "source_check"
      cmd: "validate-sources"
      parser: "json"
    - name: "artifact_check"
      cmd: "validate-artifact"
      parser: "json"

# Evaluasi risiko dinamis -> risk_tier / risk_tags task (#23.1, 72A.5)
risk:
  approval_delivery:
    critical: immediate
    high_mode: digest
    default: immediate
  digest_interval_seconds: 900
  notify_on_cycle_end: true
  max_items_per_card: 20
  path_patterns:
    critical:
      - "customer-data/**"
      - "finance/**"
      - "published/**"
      - "credentials/**"
    high:
      - "sales-data/**"
      - "campaigns/**"
      - "customer-reports/**"
  intent_keywords:
    critical:
      - "delete customer data"
      - "publish publicly"
      - "send campaign"
      - "transfer funds"
    high:
      - "export sales data"
      - "change pricing"
      - "publish scheduled content"

# Session Policy & Prompt Caching (I23)
session_policy:
  mode: sticky_until_done
  max_turns_before_flush: 15
  enable_prompt_caching: true
```

`providers` mengikuti isolasi key dan accounting di [§32.1](06-permissions-approvals-roles.md#321-provider-usage-accounting-openrouter). `models` memakai pemetaan peran di [5A.8 — Model Router](02-ai-control-plane.md#5a8-model-router). `tool_capabilities` adalah konfigurasi retry tepercaya di [§21](05-artifacts-memory-tools.md#21-tools), bukan klaim dari agen.

`toolchain` bersifat opsional dan menjalankan validator yang relevan dengan artifact/workflow yang dipilih; workspace tanpa validator lokal tetap dapat menggunakan tool jarak jauh sesuai permission. `escalation_lead` memilih agent yang terdaftar atau `human` (#30.1, #48).

`approval_delivery` mengatur jalur notifikasi, bukan permission, approval, atau expiry. Jika tidak dikonfigurasi, gunakan `immediate`; konfigurasi invalid ditolak, jangan memakai fallback yang melonggarkan policy. Saat `high_mode: digest`, `digest_interval_seconds` dan `max_items_per_card` wajib finite dan positif. Batas item memecah kartu, bukan auto-grant overflow. Akhir siklus berarti scheduler quiescent tanpa task runnable; task yang parked tidak membuat sistem menunggu hingga terminal. Persist waktu/membership notifikasi untuk restart. Critical selalu immediate dan tidak dapat diturunkan lewat setting; path/intent high tetap tunduk pada klasifikasi critical.

Aturan `session_policy` (mode, lifecycle, dan flush) hanya didefinisikan di [I23 — Session Policy](10-contracts-mvp-roadmap.md#72a1-aturan-dasar-invariants). Perakitan prompt dijelaskan di [5A.4 — Context Manager](02-ai-control-plane.md#5a4-context-manager), dan mekanisme parking worker di [§40.1](08-runtime-architecture.md#401-task-parking-step-function).

---

# 37. Storage

Penyimpanan dibagi menjadi dua pilar (Dual-Storage Architecture).

## 37.1 Operational Store — SQLite

Menyimpan data relasional dan operasional:

```text
agents
tasks (termasuk risk_tier, risk_tags)
events      (append-only event log)
messages
run history
approvals
approval batch manifests (immutable artifact refs)
approval batch item receipts (idempotency/recovery key per batch + approval ID)
versioned source references and checksums
review/approval evidence and artifact references
artifact metadata
memory (entry terstruktur)
usage / akumulasi budget
```

File artifacts tetap disimpan sebagai files. Database hanya menyimpan metadata.

## 37.2 Cognitive Store — Embedded Vector DB

Menyimpan representasi semantik yang sudah lolos admission guard #19.6, bukan data operasional:

```text
approved decision/spec dengan sumber dan versi terverifikasi
ringkasan dari sumber eligible yang sama
koordinat semantik memory eligible yang masih fresh
anti-pattern yang ditolak (cognitive_guardrail: true, tag: anti-pattern)
```

`artifact.created` hanya mencatat persistence, bukan trigger embedding. Source version, approval state bila diperlukan, serta freshness/supersede guard harus cocok sebelum cognitive admission. Implementasi: embedded vector DB seperti **ChromaDB** atau **`sqlite-vec`**. Embedding dihasilkan oleh model embedding lokal yang ringan — bukan model chat utama.

**Anti-pattern storage:** keputusan atau proposal yang ditolak tidak dibuang begitu saja. Ringkasan kondisi batasannya dapat disimpan sebagai memory dengan metadata `cognitive_guardrail: true` dan tag `anti-pattern`, setelah pemeriksaan sensitivitas dan policy.

## 37.3 Pemisahan Tanggung Jawab

- [5A.25 — Semantic Retrieval](02-ai-control-plane.md#5a25-semantic-retrieval) menanyakan Vector DB dan menerima ID referensi artifact/memory.
- SQLite tidak pernah dipakai untuk pencarian teks mentah atau `LIKE`.
- Vector DB tidak pernah dipakai untuk relasi data, status, atau ledger — itu tanggung jawab SQLite.
- ID hasil retrieval Vector DB **wajib di-validasi ulang ke SQLite** sebelum masuk context: yang `superseded` atau `stale` dibuang (#19.6 Memory Curation).
- Admission mengecek source version, approval bila diwajibkan, serta status freshness/supersede di Operational Store. Artifact belum admitted atau masih quarantine tidak boleh diretrieve meskipun vector row tertinggal.

---
