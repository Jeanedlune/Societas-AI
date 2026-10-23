CREATE TABLE workspaces (
    workspace_id TEXT PRIMARY KEY,
    name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE agents (
    workspace_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    display_name TEXT NOT NULL,
    role TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'idle',
    config_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (workspace_id, agent_id),
    FOREIGN KEY (workspace_id) REFERENCES workspaces(workspace_id)
);

CREATE TABLE runs (
    workspace_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    goal TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'created',
    entry_agent_id TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (workspace_id, run_id),
    FOREIGN KEY (workspace_id) REFERENCES workspaces(workspace_id),
    FOREIGN KEY (workspace_id, entry_agent_id) REFERENCES agents(workspace_id, agent_id)
);

CREATE TABLE tasks (
    workspace_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    task_id TEXT NOT NULL,
    parent_task_id TEXT,
    title TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'ready', 'running', 'pausing', 'paused', 'interrupted', 'blocked', 'awaiting_approval', 'completed', 'failed', 'cancelled')),
    priority TEXT NOT NULL DEFAULT 'normal' CHECK (priority IN ('low', 'normal', 'high', 'critical')),
    risk_tier TEXT,
    owner_agent_id TEXT,
    blocked_reason TEXT,
    data_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (workspace_id, run_id, task_id),
    FOREIGN KEY (workspace_id, run_id) REFERENCES runs(workspace_id, run_id),
    FOREIGN KEY (workspace_id, run_id, parent_task_id) REFERENCES tasks(workspace_id, run_id, task_id),
    FOREIGN KEY (workspace_id, owner_agent_id) REFERENCES agents(workspace_id, agent_id)
);

CREATE TABLE events (
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
    UNIQUE (workspace_id, idempotency_key),
    FOREIGN KEY (workspace_id, run_id) REFERENCES runs(workspace_id, run_id),
    FOREIGN KEY (workspace_id, run_id, task_id) REFERENCES tasks(workspace_id, run_id, task_id)
);

CREATE TABLE messages (
    workspace_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    task_id TEXT,
    conversation_id TEXT NOT NULL,
    sender TEXT NOT NULL,
    recipient TEXT NOT NULL,
    body TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (workspace_id, message_id),
    FOREIGN KEY (workspace_id, run_id) REFERENCES runs(workspace_id, run_id),
    FOREIGN KEY (workspace_id, run_id, task_id) REFERENCES tasks(workspace_id, run_id, task_id)
);

CREATE TABLE artifacts (
    workspace_id TEXT NOT NULL,
    artifact_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    version INTEGER NOT NULL CHECK (version > 0),
    task_id TEXT NOT NULL,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    relative_path TEXT NOT NULL,
    checksum TEXT NOT NULL,
    size_bytes INTEGER NOT NULL CHECK (size_bytes >= 0),
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL,
    superseded_by TEXT,
    staleness TEXT NOT NULL DEFAULT 'fresh' CHECK (staleness IN ('fresh', 'stale')),
    metadata_json TEXT NOT NULL DEFAULT '{}',
    PRIMARY KEY (workspace_id, artifact_id, version),
    FOREIGN KEY (workspace_id, run_id, task_id) REFERENCES tasks(workspace_id, run_id, task_id)
);

CREATE TABLE approvals (
    workspace_id TEXT NOT NULL,
    approval_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    task_id TEXT,
    action TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'granted', 'rejected', 'invalidated', 'expired')),
    bound_hash TEXT NOT NULL,
    snapshot_ref_json TEXT NOT NULL CHECK (json_valid(snapshot_ref_json)),
    expires_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (workspace_id, approval_id),
    FOREIGN KEY (workspace_id, run_id) REFERENCES runs(workspace_id, run_id),
    FOREIGN KEY (workspace_id, run_id, task_id) REFERENCES tasks(workspace_id, run_id, task_id)
);

CREATE TABLE memories (
    workspace_id TEXT NOT NULL,
    memory_id TEXT NOT NULL,
    source_artifact_id TEXT,
    source_artifact_version INTEGER,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'stale', 'superseded', 'quarantined')),
    content_json TEXT NOT NULL CHECK (json_valid(content_json)),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (workspace_id, memory_id),
    FOREIGN KEY (workspace_id, source_artifact_id, source_artifact_version) REFERENCES artifacts(workspace_id, artifact_id, version)
);

CREATE TABLE usage_entries (
    workspace_id TEXT NOT NULL,
    usage_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    task_id TEXT,
    agent_id TEXT,
    source TEXT NOT NULL,
    input_tokens INTEGER NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
    output_tokens INTEGER NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
    cost_usd REAL NOT NULL DEFAULT 0 CHECK (cost_usd >= 0),
    created_at TEXT NOT NULL,
    PRIMARY KEY (workspace_id, usage_id),
    FOREIGN KEY (workspace_id, run_id) REFERENCES runs(workspace_id, run_id),
    FOREIGN KEY (workspace_id, run_id, task_id) REFERENCES tasks(workspace_id, run_id, task_id),
    FOREIGN KEY (workspace_id, agent_id) REFERENCES agents(workspace_id, agent_id)
);

CREATE INDEX idx_agents_workspace_status ON agents(workspace_id, status);
CREATE INDEX idx_runs_workspace_status ON runs(workspace_id, status, created_at);
CREATE INDEX idx_tasks_run_status_priority ON tasks(workspace_id, run_id, status, priority);
CREATE INDEX idx_tasks_owner_status ON tasks(workspace_id, owner_agent_id, status);
CREATE INDEX idx_events_run_sequence ON events(workspace_id, run_id, sequence);
CREATE INDEX idx_events_type_created ON events(workspace_id, event_type, created_at);
CREATE INDEX idx_messages_conversation ON messages(workspace_id, conversation_id, created_at);
CREATE INDEX idx_artifacts_task ON artifacts(workspace_id, task_id, created_at);
CREATE INDEX idx_approvals_status_expiry ON approvals(workspace_id, status, expires_at);
CREATE INDEX idx_memories_status ON memories(workspace_id, status, updated_at);
CREATE INDEX idx_usage_run_created ON usage_entries(workspace_id, run_id, created_at);
