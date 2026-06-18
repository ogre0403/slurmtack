## 1. Failed Step Persistence

- [x] 1.1 Update the shared orchestrator failed-step helper so `attach_target` failures persist `error_summary` from the returned attach error before the execution is terminalized.
- [x] 1.2 Keep attach-step persistence aligned with execution terminalization so the failed `attach_target` step and execution detail expose the same readable failure summary.

## 2. Regression Coverage

- [x] 2.1 Add `slurm_to_openstack` attach-path coverage proving a failed `attach_target` step preserves `error_summary` when enabling the OpenStack compute service fails.
- [x] 2.2 Add `openstack_to_slurm` attach-path coverage proving a failed `attach_target` step preserves `error_summary` when Slurm attachment restore or readiness checks fail.
