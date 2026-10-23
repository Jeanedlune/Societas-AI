CREATE TABLE budget_ledger (
    ledger_id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    task_id TEXT,
    agent_id TEXT,
    scope TEXT NOT NULL CHECK (scope IN ('workspace', 'run', 'task', 'agent')),
    entry_type TEXT NOT NULL CHECK (entry_type IN ('grant', 'reserve', 'settle', 'refund', 'increase')),
    reservation_id TEXT,
    amount_tokens INTEGER NOT NULL,
    amount_cost_usd REAL NOT NULL,
    balance_tokens INTEGER NOT NULL,
    balance_cost_usd REAL NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (workspace_id, run_id) REFERENCES runs(workspace_id, run_id),
    FOREIGN KEY (workspace_id, run_id, task_id) REFERENCES tasks(workspace_id, run_id, task_id),
    FOREIGN KEY (workspace_id, agent_id) REFERENCES agents(workspace_id, agent_id)
);

CREATE TABLE budget_reservation_lease (
    reservation_id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    task_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    reserved_tokens INTEGER NOT NULL CHECK (reserved_tokens >= 0),
    reserved_cost_usd REAL NOT NULL CHECK (reserved_cost_usd >= 0),
    lease_status TEXT NOT NULL CHECK (lease_status IN ('active', 'settled', 'expired')),
    lease_ttl_seconds INTEGER NOT NULL DEFAULT 300 CHECK (lease_ttl_seconds > 0),
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (workspace_id, run_id, task_id) REFERENCES tasks(workspace_id, run_id, task_id),
    FOREIGN KEY (workspace_id, agent_id) REFERENCES agents(workspace_id, agent_id)
);

CREATE INDEX idx_budget_ledger_scope ON budget_ledger(scope, workspace_id, run_id, task_id);
CREATE INDEX idx_budget_ledger_reservation ON budget_ledger(reservation_id);
CREATE INDEX idx_lease_recovery ON budget_reservation_lease(lease_status, expires_at);
