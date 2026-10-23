<!-- Doc 05 — Artifact, Memory, Tools (§17–21)
     Bagian dari societas-full-product-spec. Urutan dokumen diatur di README.md.
     Nomor section dipertahankan; referensi silang antar-doc memakai nomor section (#N, 5A.x, 72A.x). -->

# 17. Artifact System

Output agent tidak boleh hanya dianggap sebagai text response.

Artifact adalah output yang dapat dipakai agent lain.

Contoh:

```text
research.md
architecture.md
benchmark.json
diagram.svg
test-report.md
content-draft.md
market-research.md
sales-analysis.csv
decision.md
review.md
```

## 17.1 Domain Context and Integrations

Task dapat mengikat versi sumber, brief, schema, atau dataset yang relevan agar review dan approval memeriksa input yang sama. Integrasi API/schema yang membutuhkan code generation merupakan profile opsional untuk workspace terkait; workflow engineering lama bukan requirement MVP.

Artifact memiliki:

- ID
- owner
- creator
- task
- type
- path
- checksum
- created_at
- updated_at
- metadata

---

# 18. Artifact Flow

```text
Specialist Agents
      |
      v
research.md / content-draft.md / sales-analysis.csv
      |
      v
Coordinator
      |
      v
review task
      |
      v
Editor / domain reviewer
      |
      v
review.md
```

---

# 19. Memory System

Memory dibagi menjadi beberapa level dan disimpan sesuai sifatnya pada dua pilar penyimpanan (Dual-Storage Architecture, lihat #37):

- Data terstruktur/operasional (task state, run history, approval, akumulasi budget) -> **Operational Store** (SQLite)
- Representasi semantik (embedding artifact, ringkasan riset, koordinat memori masa lalu) -> **Cognitive Store** (Embedded Vector DB)

## 19.1 Agent Memory

Informasi khusus agent.

Contoh:

```text
Content Creator preferences
Brand voice and approved claims
Market research findings
Past review feedback
```

## 19.2 Workspace Memory

Informasi seluruh organisasi.

Contoh:

```text
Business goals
Product direction
Company rules
Approved sources and definitions
```

## 19.3 Task Memory

Context yang hanya relevan terhadap satu task.

## 19.4 Run Memory

Context selama satu execution.

## 19.5 Memory Storage Mapping

Setiap level memory di atas di-map ke pilar penyimpanan:

- Agent Memory & Workspace Memory: entry terstruktur (sumber, timestamp, scope) di SQLite; hanya narasi yang lolos admission guard #19.6 boleh di-embed ke Vector DB.
- Task Memory: relasi task -> artifact/memory disimpan di SQLite; ringkasan tidak otomatis di-embed dan tetap melewati admission guard #19.6.
- Run Memory: append-only di event log SQLite; tidak di-embed (ephemeral).

Retrieval semantik selalu melewati Vector DB dan mengembalikan ID referensi ([5A.25 — Semantic Retrieval](02-ai-control-plane.md#5a25-semantic-retrieval)) — bukan pencarian teks mentah di SQLite.

## 19.6 Memory Curation (Anti Semantic Drift / Zombie Memory)

Vector DB bekerja berdasarkan kemiripan teks (cosine similarity), **bukan status kebenaran atau recency**. Dokumen lama dan baru dengan topik serupa bisa sama-sama ter-retrieve sehingga agen menerima fakta yang sudah bertentangan. Semakin lama workspace berjalan, makin banyak memory yang usang atau sudah diganti.

Lima aturan kurasi deterministik di Go + SQLite — tanpa algoritma AI tambahan:

1. **Filter pintu masuk — jangan embed semua hal.** Memory hanya boleh di-embed setelah sumber dan versi yang relevan jelas serta admission policy workspace terpenuhi. Chat/debat/transkrip, data sensitif, output tanpa sumber, draft yang belum disetujui, dan proposal yang ditolak tidak otomatis masuk ke Vector DB.
2. **Pola Supersede (tombstone di SQLite).** Tabel `artifacts` memiliki kolom `superseded_by` (ID artifact pengganti). Saat keputusan baru menggantikan yang lama, record lama ditandai — bukan dihapus. Setiap ID hasil retrieval Vector DB wajib di-filter ulang ke SQLite: `superseded_by != NULL -> skip`. Dokumen basi tidak pernah masuk context LLM lagi.
3. **Time-decay scoring.** Skor akhir retrieval = `vector_similarity x faktor_umur` (mis. dokumen minggu ini x1.0, 3 bulan x0.5), dihitung deterministik di Go — dokumen lama yang kebetulan mirip kalah dari dokumen baru yang relevan.
4. **Ikat memori ke sumber.** Setiap entry menyimpan metadata source ID, versi/checksum, scope, dan waktu. Background curation menandai memory stale saat sumber diganti, dihapus, atau melewati freshness policy.
5. **Admission guard.** Sebelum embedding, periksa approval dan versi sumber yang tepat, status/tombstone, sensitivitas, serta freshness di SQLite. Jika sumber berubah atau statusnya belum jelas, simpan artifact di Operational Store saja sampai direview ulang.

### Penyimpanan Batasan Negatif (Anti-Pattern)

Keputusan atau proposal yang ditolak **tidak dibuang**. Ringkasan kondisi batasannya dapat disimpan ke Vector DB (Cognitive Store) dengan:

- Metadata `cognitive_guardrail: true` — menandai entry sebagai batasan validasi, bukan contoh untuk ditiru
- Tag `anti-pattern` — membedakan dari best practice yang boleh diikuti
- Ringkasan fokus pada **alasan penolakan** dan **kondisi yang menyebabkan kegagalan**, bukan implementasi detail yang salah

Contoh anti-pattern yang disimpan:
- Klaim tanpa sumber yang dapat diverifikasi
- Materi yang tidak sesuai audiens atau brand policy
- Analisis yang mencampur periode atau definisi metrik berbeda

Anti-pattern ini tetap di-retrieval saat ada konteks serupa, tetapi **diisolasi secara struktural** di prompt LLM (lihat 5A.4) agar model memperlakukannya sebagai penalti/filter validasi akhir, bukan contoh kode yang harus ditiru.

Prinsip: Vector DB adalah **perpustakaan buku yang sudah lulus kurasi** — bukan tempat sampah. Edisi lama ditarik dari rak saat revisi terbit.

Semantic triage memakai sumber dan versi artifact yang tepat; dilarang memakai similarity sebagai pengganti bukti. Setelah replacement valid admitted, tombstone/supersede menarik versi lama lewat mekanisme existing. Provenance fingerprint/freshness lintas source dijelaskan di [DEC-005] W15 dan [DEC-006] W11/W12.

---

# 20. Memory Rules

Memory harus memiliki:

- source
- timestamp
- confidence
- scope
- owner

Jangan memasukkan semua chat ke memory secara otomatis — hanya artifact dengan sumber, versi, dan status admission yang tervalidasi yang boleh di-embed (#19.6).

Memory harus memiliki lifecycle.

Contoh:

```text
raw event
    |
    v
candidate memory
    |
    v
validated memory
    |
    v
persistent memory
    |
    v
superseded / stale  (tombstone di SQLite — tidak pernah kembali ke context, #19.6)
```

---

# 21. Tools

Agent dapat memiliki tools.

Contoh:

```text
filesystem
shell
browser
search
http
database
terminal
python
docker
kubernetes
```

Tool access harus configurable per agent.

Capability retry setiap tool disimpan dalam konfigurasi tepercaya Tool Runtime, bukan ditentukan oleh agen atau sekadar keberadaan field di input:

```yaml
tool_capabilities:
  filesystem.read:
    idempotent: true
    operation_key_enforced: false
  external.mutate:
    idempotent: false
    operation_key_enforced: false  # true hanya jika penyedia menjamin deduplikasi
```

Field yang tidak diisi berarti `false`. Untuk tool non-idempotent dengan `operation_key_enforced: true`, runtime membuat dan menyimpan `operation_key` sebelum dispatch pertama, mempertahankannya beserta argumen yang sama untuk seluruh retry, dan hanya retry selama jaminan deduplikasi penyedia masih berlaku. Tidak menambahkan field ini ke argumen MCP bila provider tidak mendukungnya. Timeout/network putus sesudah dispatch berarti `outcome_unknown`, bukan bukti tool belum berjalan. Tanpa jaminan idempotency/deduplikasi, rekonsiliasi atau keputusan human yang menyebut risiko duplikasi wajib mendahului percobaan lain (I16, 72A.8–10). Tool dengan hasil gagal logis memakai `tool.call_completed` `status: error`; tool pasti belum dispatch memakai `tool.call_failed` `outcome: not_started`.

## 21.1 Tool Interface — MCP Client

Antarmuka tool agen menggunakan arsitektur **MCP Client**: Tool Runtime bertindak sebagai client yang terhubung ke MCP server internal maupun eksternal (browser, search, spreadsheet, filesystem, dan lainnya). Tool baru ditambahkan dengan mendaftarkan server MCP — bukan mengubah kode agen. Permission tetap di-gate deterministik oleh Policy Engine (#22) sebelum call diteruskan ke MCP server.

---
