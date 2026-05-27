CREATE TABLE IF NOT EXISTS executions (
    id TEXT PRIMARY KEY,
    node_name TEXT NOT NULL DEFAULT '',
    direction TEXT NOT NULL,
    requested_by TEXT NOT NULL,
    requested_at DATETIME NOT NULL,
    current_state TEXT NOT NULL,
    desired_owner TEXT NOT NULL,
    previous_owner TEXT NOT NULL,
    state_version INTEGER NOT NULL DEFAULT 0,
    overall_status TEXT NOT NULL DEFAULT 'active',
    lock_acquired_at DATETIME,
    lock_released_at DATETIME,
    final_error_code TEXT NOT NULL DEFAULT '',
    final_error_summary TEXT NOT NULL DEFAULT '',
    log_root TEXT NOT NULL DEFAULT '',
    placeholder_job_id TEXT NOT NULL DEFAULT '',
    requested_slurm_constraint TEXT NOT NULL DEFAULT '',
    requested_slurm_partition TEXT NOT NULL DEFAULT '',
    allocation_event_at DATETIME
);

CREATE TABLE IF NOT EXISTS steps (
    execution_id TEXT NOT NULL,
    step_name TEXT NOT NULL,
    sequence INTEGER NOT NULL,
    host TEXT NOT NULL DEFAULT '',
    started_at DATETIME NOT NULL,
    ended_at DATETIME,
    status TEXT NOT NULL DEFAULT 'pending',
    retry_count INTEGER NOT NULL DEFAULT 0,
    exit_code INTEGER,
    error_class TEXT NOT NULL DEFAULT '',
    command_id TEXT NOT NULL DEFAULT '',
    stdout_path TEXT NOT NULL DEFAULT '',
    stderr_path TEXT NOT NULL DEFAULT '',
    snapshot_before_path TEXT NOT NULL DEFAULT '',
    snapshot_after_path TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (execution_id, step_name, sequence),
    FOREIGN KEY (execution_id) REFERENCES executions(id)
);

CREATE TABLE IF NOT EXISTS leases (
    node_name TEXT PRIMARY KEY,
    execution_id TEXT NOT NULL,
    holder TEXT NOT NULL DEFAULT '',
    expires_at DATETIME NOT NULL,
    state_version INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (execution_id) REFERENCES executions(id)
);
