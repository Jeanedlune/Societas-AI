CREATE TABLE events_v3 (
    event_id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    task_id TEXT,
    sequence INTEGER NOT NULL CHECK (sequence > 0),
    event_type TEXT NOT NULL,
    schema_version TEXT NOT NULL,
    correlation_id TEXT NOT NULL,
    causation_id TEXT,
    idempotency_key TEXT,
    payload_json TEXT NOT NULL CHECK (json_valid(payload_json)),
    envelope_json TEXT NOT NULL CHECK (json_valid(envelope_json)),
    created_at TEXT NOT NULL,
    UNIQUE (workspace_id, run_id, sequence),
    UNIQUE (workspace_id, run_id, idempotency_key),
    FOREIGN KEY (workspace_id, run_id) REFERENCES runs(workspace_id, run_id),
    FOREIGN KEY (workspace_id, run_id, task_id) REFERENCES tasks(workspace_id, run_id, task_id)
);

INSERT INTO events_v3(event_id, workspace_id, run_id, task_id, sequence, event_type, schema_version, correlation_id, causation_id, idempotency_key, payload_json, envelope_json, created_at)
SELECT event_id, workspace_id, run_id, task_id, sequence, event_type, schema_version, correlation_id, causation_id, idempotency_key, payload_json, envelope_json, created_at
FROM events;

DROP TABLE events;
ALTER TABLE events_v3 RENAME TO events;

CREATE INDEX idx_events_run_sequence ON events(workspace_id, run_id, sequence);
CREATE INDEX idx_events_type_created ON events(workspace_id, event_type, created_at);

CREATE TRIGGER events_immutable_update
BEFORE UPDATE ON events
BEGIN
    SELECT RAISE(ABORT, 'events are append-only');
END;

CREATE TRIGGER events_immutable_delete
BEFORE DELETE ON events
BEGIN
    SELECT RAISE(ABORT, 'events are append-only');
END;
