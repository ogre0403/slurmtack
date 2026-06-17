## 1. Slurm job-state lookup

- [x] 1.1 Extend the `slurm.Client` contract and `RestClient` implementation to query placeholder job state with workload identity and normalized terminal-state semantics.
- [x] 1.2 Add Slurm client tests covering non-terminal job states, terminal failure states, and Slurm API rejection for the job-state lookup path.

## 2. Allocation wait failure handling

- [x] 2.1 Update the orchestrator so `awaiting_source_allocation` remains actively monitored, including restart recovery for executions already waiting on a placeholder job.
- [x] 2.2 Fail `awaiting_source_allocation` executions as `failed_non_destructive` when the placeholder job reaches a terminal state before any allocation event, and persist a readable failure summary on both the wait step and execution.
- [x] 2.3 Add orchestrator regression tests for placeholder job terminal failure, placeholder completion without allocation, and late allocation race handling.

## 3. Dashboard refresh

- [x] 3.1 Refresh the selected execution detail and step timeline on the existing dashboard polling cadence so failed allocation waits are shown without manual reselection.
- [x] 3.2 Add dashboard UI tests covering a selected execution that transitions from allocation waiting to failed state during polling.
