## 1. Request Path

- [x] 1.1 Add `slurm_partition` to the API and service switch request structs for `slurm_to_openstack` requests
- [x] 1.2 Persist the requested partition on `domain.Execution` when a switch request is accepted
- [x] 1.3 Update API and service tests to cover requests with and without `slurm_partition`

## 2. Store Compatibility

- [x] 2.1 Add `requested_slurm_partition` to the execution schema and in-memory store round-trip behavior
- [x] 2.2 Update SQLite read and write queries to store and load `requested_slurm_partition`
- [x] 2.3 Add an idempotent SQLite startup compatibility step that adds `requested_slurm_partition` for existing databases
- [x] 2.4 Extend store tests to cover partition persistence and existing-schema compatibility

## 3. Placeholder Submission

- [x] 3.1 Thread the requested partition into every placeholder submission call site, including the orchestrator and allocation handler
- [x] 3.2 Update placeholder submission tests to verify partitioned and unpartitioned requests reach `slurm.PlaceholderJobRequest`

## 4. Verification

- [x] 4.1 Run focused tests for the touched API, store, orchestrator, and Slurm client packages
- [x] 4.2 Confirm the new change remains apply-ready in OpenSpec after the implementation checklist is complete