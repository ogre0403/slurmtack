## ADDED Requirements

### Requirement: Implement Store interface with SQLite

The SQLite store SHALL implement all methods defined by the `store.Store` interface: CreateExecution, GetExecution, ListExecutions, AdvanceState, UpdateExecution, AcquireLease, ReleaseLease, GetLease, CreateStep, UpdateStep, ListSteps.

#### Scenario: Create and retrieve execution

- **WHEN** CreateExecution is called with a valid Execution struct
- **THEN** GetExecution with the same ID returns the persisted execution with all fields intact

#### Scenario: List executions by node

- **WHEN** multiple executions exist for different nodes
- **THEN** ListExecutions with a node_name filter returns only executions for that node

### Requirement: Optimistic concurrency on state transitions

The SQLite store SHALL implement AdvanceState with optimistic concurrency control using state_version. The state_version MUST be incremented atomically on successful transition.

#### Scenario: Successful state advance

- **WHEN** AdvanceState is called with the correct expectedVersion
- **THEN** state is updated and state_version is incremented by 1

#### Scenario: Version conflict

- **WHEN** AdvanceState is called with a stale expectedVersion
- **THEN** store returns ErrVersionConflict and state is unchanged

### Requirement: Node lease exclusivity

The SQLite store SHALL enforce that only one execution can hold a lease for a given node at a time.

#### Scenario: Acquire lease on free node

- **WHEN** AcquireLease is called for a node with no active lease
- **THEN** lease is created and subsequent GetLease returns it

#### Scenario: Acquire lease on occupied node

- **WHEN** AcquireLease is called for a node with an existing active lease held by a different execution
- **THEN** store returns ErrLeaseHeld

#### Scenario: Release lease

- **WHEN** ReleaseLease is called with matching node_name and execution_id
- **THEN** lease is removed and subsequent AcquireLease for that node succeeds

### Requirement: Step record persistence

The SQLite store SHALL persist step records with all fields and support listing steps by execution_id in sequence order.

#### Scenario: Create and list steps

- **WHEN** multiple steps are created for an execution
- **THEN** ListSteps returns them ordered by sequence number

#### Scenario: Update step status

- **WHEN** UpdateStep is called with ended_at and status=failed
- **THEN** subsequent ListSteps reflects the updated fields

### Requirement: WAL mode and connection configuration

The SQLite store SHALL open the database in WAL journal mode with busy_timeout set to at least 5000ms to handle concurrent read access without immediate SQLITE_BUSY errors.

#### Scenario: Concurrent reads during write

- **WHEN** a write transaction is in progress
- **THEN** read queries succeed without blocking (WAL allows concurrent readers)

### Requirement: Schema initialization

The SQLite store SHALL create all required tables on first startup if they do not exist. Table creation MUST be idempotent (using CREATE TABLE IF NOT EXISTS or equivalent).

#### Scenario: First startup with empty database

- **WHEN** the daemon starts with a new empty database file
- **THEN** all tables are created and the store is fully operational

#### Scenario: Restart with existing database

- **WHEN** the daemon restarts with an existing database containing data
- **THEN** existing data is preserved and accessible
