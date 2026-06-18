package store

import (
	"context"
	"database/sql"
	_ "embed"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/slurmtack/slurmtack/internal/domain"
)

//go:embed schema.sql
var schemaSQL string

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, err
	}
	if err := ensureExecutionColumns(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := ensureStepColumns(db); err != nil {
		db.Close()
		return nil, err
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) Ping() error {
	return s.db.Ping()
}

func (s *SQLiteStore) CreateExecution(_ context.Context, exec *domain.Execution) error {
	_, err := s.db.Exec(`INSERT INTO executions (
		id, node_name, direction, requested_by, requested_at,
		current_state, desired_owner, previous_owner, state_version,
		overall_status, lock_acquired_at, lock_released_at,
		final_error_code, final_error_summary, log_root,
		placeholder_job_id, requested_slurm_constraint, requested_slurm_partition,
		requested_slurm_account, slurm_workload_user, slurm_workload_token,
		placeholder_sif_file, allocation_event_at, cancellation_source_state
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		exec.ID, exec.NodeName, string(exec.Direction), exec.RequestedBy, exec.RequestedAt,
		string(exec.CurrentState), string(exec.DesiredOwner), string(exec.PreviousOwner), exec.StateVersion,
		string(exec.OverallStatus), nullTime(exec.LockAcquiredAt), nullTime(exec.LockReleasedAt),
		exec.FinalErrorCode, exec.FinalErrorSummary, exec.LogRoot,
		exec.PlaceholderJobID, exec.RequestedSlurmConstraint, exec.RequestedSlurmPartition,
		exec.RequestedSlurmAccount, exec.SlurmWorkloadUser, exec.SlurmWorkloadToken,
		exec.PlaceholderSIFFile, nullTime(exec.AllocationEventAt), string(exec.CancellationSourceState),
	)
	return err
}

func (s *SQLiteStore) GetExecution(_ context.Context, id string) (*domain.Execution, error) {
	row := s.db.QueryRow(`SELECT
		id, node_name, direction, requested_by, requested_at,
		current_state, desired_owner, previous_owner, state_version,
		overall_status, lock_acquired_at, lock_released_at,
		final_error_code, final_error_summary, log_root,
		placeholder_job_id, requested_slurm_constraint, requested_slurm_partition,
		requested_slurm_account, slurm_workload_user, slurm_workload_token,
		placeholder_sif_file, allocation_event_at, cancellation_source_state
	FROM executions WHERE id = ?`, id)
	return scanExecution(row)
}

func (s *SQLiteStore) ListExecutions(_ context.Context, filter ExecutionFilter) ([]*domain.Execution, error) {
	query := `SELECT
		id, node_name, direction, requested_by, requested_at,
		current_state, desired_owner, previous_owner, state_version,
		overall_status, lock_acquired_at, lock_released_at,
		final_error_code, final_error_summary, log_root,
		placeholder_job_id, requested_slurm_constraint, requested_slurm_partition,
		requested_slurm_account, slurm_workload_user, slurm_workload_token,
		placeholder_sif_file, allocation_event_at, cancellation_source_state
	FROM executions`

	var conditions []string
	var args []interface{}
	if filter.NodeName != "" {
		conditions = append(conditions, "node_name = ?")
		args = append(args, filter.NodeName)
	}
	if filter.Status != "" {
		conditions = append(conditions, "overall_status = ?")
		args = append(args, filter.Status)
	}
	if filter.Direction != "" {
		conditions = append(conditions, "direction = ?")
		args = append(args, filter.Direction)
	}
	if filter.RequestedFrom != nil {
		conditions = append(conditions, "requested_at >= ?")
		args = append(args, *filter.RequestedFrom)
	}
	if filter.RequestedTo != nil {
		conditions = append(conditions, "requested_at <= ?")
		args = append(args, *filter.RequestedTo)
	}
	if filter.Before != nil {
		conditions = append(conditions, "requested_at < ?")
		args = append(args, *filter.Before)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY requested_at DESC"
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*domain.Execution
	for rows.Next() {
		exec, err := scanExecutionRows(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, exec)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) ListActiveExecutions(_ context.Context) ([]*domain.Execution, error) {
	rows, err := s.db.Query(`SELECT
		id, node_name, direction, requested_by, requested_at,
		current_state, desired_owner, previous_owner, state_version,
		overall_status, lock_acquired_at, lock_released_at,
		final_error_code, final_error_summary, log_root,
		placeholder_job_id, requested_slurm_constraint, requested_slurm_partition,
		requested_slurm_account, slurm_workload_user, slurm_workload_token,
		placeholder_sif_file, allocation_event_at, cancellation_source_state
	FROM executions WHERE overall_status = ?`, string(domain.OverallStatusActive))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*domain.Execution
	for rows.Next() {
		exec, err := scanExecutionRows(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, exec)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) UpdateExecution(_ context.Context, exec *domain.Execution) error {
	res, err := s.db.Exec(`UPDATE executions SET
		node_name = ?, direction = ?, requested_by = ?, requested_at = ?,
		current_state = ?, desired_owner = ?, previous_owner = ?, state_version = ?,
		overall_status = ?, lock_acquired_at = ?, lock_released_at = ?,
		final_error_code = ?, final_error_summary = ?, log_root = ?,
		placeholder_job_id = ?, requested_slurm_constraint = ?, requested_slurm_partition = ?,
		requested_slurm_account = ?, slurm_workload_user = ?, slurm_workload_token = ?,
		placeholder_sif_file = ?, allocation_event_at = ?, cancellation_source_state = ?
	WHERE id = ?`,
		exec.NodeName, string(exec.Direction), exec.RequestedBy, exec.RequestedAt,
		string(exec.CurrentState), string(exec.DesiredOwner), string(exec.PreviousOwner), exec.StateVersion,
		string(exec.OverallStatus), nullTime(exec.LockAcquiredAt), nullTime(exec.LockReleasedAt),
		exec.FinalErrorCode, exec.FinalErrorSummary, exec.LogRoot,
		exec.PlaceholderJobID, exec.RequestedSlurmConstraint, exec.RequestedSlurmPartition,
		exec.RequestedSlurmAccount, exec.SlurmWorkloadUser, exec.SlurmWorkloadToken,
		exec.PlaceholderSIFFile, nullTime(exec.AllocationEventAt), string(exec.CancellationSourceState),
		exec.ID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) AdvanceState(_ context.Context, executionID string, expectedVersion int64, newState domain.SwitchState) error {
	res, err := s.db.Exec(`UPDATE executions
		SET current_state = ?, state_version = state_version + 1,
		    overall_status = CASE
		        WHEN ? IN ('completed') THEN 'succeeded'
		        WHEN ? IN ('failed_non_destructive','failed_needs_rollback','failed_manual_recovery','cancelled') THEN 'failed'
		        ELSE overall_status
		    END
		WHERE id = ? AND state_version = ?`,
		string(newState), string(newState), string(newState), executionID, expectedVersion,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		var exists bool
		s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM executions WHERE id = ?)", executionID).Scan(&exists)
		if !exists {
			return ErrNotFound
		}
		return ErrVersionConflict
	}
	return nil
}

func (s *SQLiteStore) AcquireLease(_ context.Context, lease *domain.NodeLease) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var existingExecID string
	var expiresAt time.Time
	err = tx.QueryRow("SELECT execution_id, expires_at FROM leases WHERE node_name = ?", lease.NodeName).Scan(&existingExecID, &expiresAt)
	if err == nil {
		if existingExecID != lease.ExecutionID && time.Now().Before(expiresAt) {
			return ErrLeaseHeld
		}
		_, err = tx.Exec(`UPDATE leases SET execution_id = ?, holder = ?, expires_at = ?, state_version = ? WHERE node_name = ?`,
			lease.ExecutionID, lease.Holder, lease.ExpiresAt, lease.StateVersion, lease.NodeName)
	} else {
		_, err = tx.Exec(`INSERT INTO leases (node_name, execution_id, holder, expires_at, state_version) VALUES (?, ?, ?, ?, ?)`,
			lease.NodeName, lease.ExecutionID, lease.Holder, lease.ExpiresAt, lease.StateVersion)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteStore) ReleaseLease(_ context.Context, nodeName string, executionID string) error {
	res, err := s.db.Exec("DELETE FROM leases WHERE node_name = ? AND execution_id = ?", nodeName, executionID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrLeaseNotHeld
	}
	return nil
}

func (s *SQLiteStore) GetLease(_ context.Context, nodeName string) (*domain.NodeLease, error) {
	var lease domain.NodeLease
	err := s.db.QueryRow("SELECT node_name, execution_id, holder, expires_at, state_version FROM leases WHERE node_name = ?", nodeName).
		Scan(&lease.NodeName, &lease.ExecutionID, &lease.Holder, &lease.ExpiresAt, &lease.StateVersion)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &lease, nil
}

func (s *SQLiteStore) CreateStep(_ context.Context, step *domain.StepRecord) error {
	_, err := s.db.Exec(`INSERT INTO steps (
		execution_id, step_name, sequence, host, started_at, ended_at,
		status, retry_count, exit_code, error_class, error_summary, command_id,
		stdout_path, stderr_path, snapshot_before_path, snapshot_after_path
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		step.ExecutionID, step.StepName, step.Sequence, step.Host, step.StartedAt,
		nullTime(step.EndedAt), string(step.Status), step.RetryCount, nullInt(step.ExitCode),
		string(step.ErrorClass), step.ErrorSummary, step.CommandID, step.StdoutPath, step.StderrPath,
		step.SnapshotBeforePath, step.SnapshotAfterPath,
	)
	return err
}

func (s *SQLiteStore) UpdateStep(_ context.Context, step *domain.StepRecord) error {
	res, err := s.db.Exec(`UPDATE steps SET
		host = ?, started_at = ?, ended_at = ?, status = ?, retry_count = ?,
		exit_code = ?, error_class = ?, error_summary = ?, command_id = ?, stdout_path = ?,
		stderr_path = ?, snapshot_before_path = ?, snapshot_after_path = ?
	WHERE execution_id = ? AND step_name = ? AND sequence = ?`,
		step.Host, step.StartedAt, nullTime(step.EndedAt), string(step.Status), step.RetryCount,
		nullInt(step.ExitCode), string(step.ErrorClass), step.ErrorSummary, step.CommandID, step.StdoutPath,
		step.StderrPath, step.SnapshotBeforePath, step.SnapshotAfterPath,
		step.ExecutionID, step.StepName, step.Sequence,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) ListSteps(_ context.Context, executionID string) ([]*domain.StepRecord, error) {
	rows, err := s.db.Query(`SELECT
		execution_id, step_name, sequence, host, started_at, ended_at,
		status, retry_count, exit_code, error_class, error_summary, command_id,
		stdout_path, stderr_path, snapshot_before_path, snapshot_after_path
	FROM steps WHERE execution_id = ? ORDER BY sequence`, executionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*domain.StepRecord
	for rows.Next() {
		var step domain.StepRecord
		var endedAt sql.NullTime
		var exitCode sql.NullInt64
		err := rows.Scan(
			&step.ExecutionID, &step.StepName, &step.Sequence, &step.Host, &step.StartedAt,
			&endedAt, &step.Status, &step.RetryCount, &exitCode, &step.ErrorClass,
			&step.ErrorSummary, &step.CommandID, &step.StdoutPath, &step.StderrPath,
			&step.SnapshotBeforePath, &step.SnapshotAfterPath,
		)
		if err != nil {
			return nil, err
		}
		if endedAt.Valid {
			step.EndedAt = &endedAt.Time
		}
		if exitCode.Valid {
			v := int(exitCode.Int64)
			step.ExitCode = &v
		}
		result = append(result, &step)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) RecordAdminTokenRenewal(_ context.Context, renewal *domain.AdminTokenRenewal) error {
	res, err := s.db.Exec(`INSERT INTO admin_token_renewals (
		issued_at, admin_user, login_node, trigger
	) VALUES (?, ?, ?, ?)`,
		renewal.IssuedAt, renewal.AdminUser, renewal.LoginNode, string(renewal.Trigger),
	)
	if err != nil {
		return err
	}
	if id, err := res.LastInsertId(); err == nil {
		renewal.ID = id
	}
	return nil
}

func (s *SQLiteStore) ListAdminTokenRenewals(_ context.Context) ([]*domain.AdminTokenRenewal, error) {
	rows, err := s.db.Query(`SELECT id, issued_at, admin_user, login_node, trigger
		FROM admin_token_renewals ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*domain.AdminTokenRenewal
	for rows.Next() {
		var r domain.AdminTokenRenewal
		var trigger string
		if err := rows.Scan(&r.ID, &r.IssuedAt, &r.AdminUser, &r.LoginNode, &trigger); err != nil {
			return nil, err
		}
		r.Trigger = domain.AdminTokenRenewalTrigger(trigger)
		result = append(result, &r)
	}
	return result, rows.Err()
}

func scanExecution(row *sql.Row) (*domain.Execution, error) {
	var exec domain.Execution
	var lockAcquired, lockReleased, allocEvent sql.NullTime
	err := row.Scan(
		&exec.ID, &exec.NodeName, &exec.Direction, &exec.RequestedBy, &exec.RequestedAt,
		&exec.CurrentState, &exec.DesiredOwner, &exec.PreviousOwner, &exec.StateVersion,
		&exec.OverallStatus, &lockAcquired, &lockReleased,
		&exec.FinalErrorCode, &exec.FinalErrorSummary, &exec.LogRoot,
		&exec.PlaceholderJobID, &exec.RequestedSlurmConstraint, &exec.RequestedSlurmPartition,
		&exec.RequestedSlurmAccount, &exec.SlurmWorkloadUser, &exec.SlurmWorkloadToken,
		&exec.PlaceholderSIFFile, &allocEvent, &exec.CancellationSourceState,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if lockAcquired.Valid {
		exec.LockAcquiredAt = &lockAcquired.Time
	}
	if lockReleased.Valid {
		exec.LockReleasedAt = &lockReleased.Time
	}
	if allocEvent.Valid {
		exec.AllocationEventAt = &allocEvent.Time
	}
	return &exec, nil
}

func scanExecutionRows(rows *sql.Rows) (*domain.Execution, error) {
	var exec domain.Execution
	var lockAcquired, lockReleased, allocEvent sql.NullTime
	err := rows.Scan(
		&exec.ID, &exec.NodeName, &exec.Direction, &exec.RequestedBy, &exec.RequestedAt,
		&exec.CurrentState, &exec.DesiredOwner, &exec.PreviousOwner, &exec.StateVersion,
		&exec.OverallStatus, &lockAcquired, &lockReleased,
		&exec.FinalErrorCode, &exec.FinalErrorSummary, &exec.LogRoot,
		&exec.PlaceholderJobID, &exec.RequestedSlurmConstraint, &exec.RequestedSlurmPartition,
		&exec.RequestedSlurmAccount, &exec.SlurmWorkloadUser, &exec.SlurmWorkloadToken,
		&exec.PlaceholderSIFFile, &allocEvent, &exec.CancellationSourceState,
	)
	if err != nil {
		return nil, err
	}
	if lockAcquired.Valid {
		exec.LockAcquiredAt = &lockAcquired.Time
	}
	if lockReleased.Valid {
		exec.LockReleasedAt = &lockReleased.Time
	}
	if allocEvent.Valid {
		exec.AllocationEventAt = &allocEvent.Time
	}
	return &exec, nil
}

func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

func ensureExecutionColumns(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(executions)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	existing := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		existing[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if !existing["requested_slurm_partition"] {
		if _, err := db.Exec(`ALTER TABLE executions ADD COLUMN requested_slurm_partition TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !existing["cancellation_source_state"] {
		if _, err := db.Exec(`ALTER TABLE executions ADD COLUMN cancellation_source_state TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !existing["requested_slurm_account"] {
		if _, err := db.Exec(`ALTER TABLE executions ADD COLUMN requested_slurm_account TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !existing["slurm_workload_user"] {
		if _, err := db.Exec(`ALTER TABLE executions ADD COLUMN slurm_workload_user TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !existing["slurm_workload_token"] {
		if _, err := db.Exec(`ALTER TABLE executions ADD COLUMN slurm_workload_token TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !existing["placeholder_sif_file"] {
		if _, err := db.Exec(`ALTER TABLE executions ADD COLUMN placeholder_sif_file TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	return nil
}

func nullInt(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

func ensureStepColumns(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(steps)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	existing := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		existing[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if !existing["error_summary"] {
		if _, err := db.Exec(`ALTER TABLE steps ADD COLUMN error_summary TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	return nil
}
