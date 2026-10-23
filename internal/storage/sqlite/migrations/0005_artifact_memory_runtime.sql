CREATE TRIGGER budget_ledger_immutable_update
BEFORE UPDATE ON budget_ledger
BEGIN
    SELECT RAISE(ABORT, 'budget ledger is append-only');
END;

CREATE TRIGGER budget_ledger_immutable_delete
BEFORE DELETE ON budget_ledger
BEGIN
    SELECT RAISE(ABORT, 'budget ledger is append-only');
END;

ALTER TABLE memories ADD COLUMN scope TEXT NOT NULL DEFAULT 'workspace'
    CHECK (scope IN ('workspace', 'run', 'task', 'agent'));
ALTER TABLE memories ADD COLUMN owner_id TEXT NOT NULL DEFAULT '';
ALTER TABLE memories ADD COLUMN source_checksum TEXT NOT NULL DEFAULT '';
ALTER TABLE memories ADD COLUMN confidence REAL NOT NULL DEFAULT 0
    CHECK (confidence >= 0 AND confidence <= 1);
ALTER TABLE memories ADD COLUMN freshness_until TEXT;
ALTER TABLE memories ADD COLUMN superseded_by TEXT;
ALTER TABLE memories ADD COLUMN sensitivity TEXT NOT NULL DEFAULT 'normal'
    CHECK (sensitivity IN ('normal', 'sensitive'));
ALTER TABLE memories ADD COLUMN admission_status TEXT NOT NULL DEFAULT 'candidate'
    CHECK (admission_status IN ('candidate', 'admitted', 'rejected'));
ALTER TABLE memories ADD COLUMN cognitive_guardrail INTEGER NOT NULL DEFAULT 0
    CHECK (cognitive_guardrail IN (0, 1));

CREATE INDEX idx_memories_scope ON memories(workspace_id, scope, owner_id, status);
CREATE INDEX idx_memories_source ON memories(workspace_id, source_artifact_id, source_artifact_version);

CREATE TABLE summaries (
    workspace_id TEXT NOT NULL,
    summary_id TEXT NOT NULL,
    artifact_id TEXT NOT NULL,
    artifact_version INTEGER NOT NULL,
    token_estimate INTEGER NOT NULL CHECK (token_estimate >= 0),
    generated_by TEXT NOT NULL,
    model TEXT NOT NULL,
    body_json TEXT NOT NULL CHECK (json_valid(body_json)),
    created_at TEXT NOT NULL,
    PRIMARY KEY (workspace_id, summary_id),
    FOREIGN KEY (workspace_id, artifact_id, artifact_version)
        REFERENCES artifacts(workspace_id, artifact_id, version)
);

CREATE INDEX idx_summaries_source ON summaries(workspace_id, artifact_id, artifact_version);
