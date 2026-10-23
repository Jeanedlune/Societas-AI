CREATE TABLE event_outbox (
    event_id TEXT PRIMARY KEY NOT NULL REFERENCES events(event_id),
    workspace_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    sequence INTEGER NOT NULL CHECK (sequence > 0),
    delivery_state TEXT NOT NULL DEFAULT 'pending'
        CHECK (delivery_state IN ('pending', 'leased', 'delivered', 'dead')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at TEXT NOT NULL,
    lease_owner TEXT,
    lease_expires_at TEXT,
    delivered_at TEXT,
    last_error_code TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (workspace_id, run_id, sequence),
    CHECK ((delivery_state = 'leased' AND lease_owner IS NOT NULL AND lease_expires_at IS NOT NULL)
        OR (delivery_state != 'leased' AND lease_owner IS NULL AND lease_expires_at IS NULL))
);

CREATE INDEX event_outbox_ready_idx
    ON event_outbox(delivery_state, available_at, workspace_id, run_id, sequence);
CREATE INDEX event_outbox_stream_idx
    ON event_outbox(workspace_id, run_id, sequence, delivery_state);

INSERT INTO event_outbox (
    event_id, workspace_id, run_id, sequence, delivery_state, attempts,
    available_at, created_at, updated_at
)
SELECT event_id, workspace_id, run_id, sequence, 'pending', 0,
       created_at, created_at, created_at
FROM events;
