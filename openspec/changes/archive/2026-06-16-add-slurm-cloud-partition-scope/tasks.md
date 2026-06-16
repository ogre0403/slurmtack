## 1. Configuration And Runtime Config Wiring

- [x] 1.1 Add `SLURM_CLOUD_PARTITION` to daemon config loading and thread it through the server/service construction path.
- [x] 1.2 Wire `SLURM_CLOUD_PARTITION` into deployment assets such as Compose and `.env` examples, and extend nginx runtime config generation to publish `slurmCloudPartition` safely.
- [x] 1.3 Add or update config and runtime-config tests to cover both unset and configured cloud-partition modes.

## 2. Backend Scope Enforcement

- [x] 2.1 Update dashboard inventory handling so scoped mode returns only the configured partition and rejects conflicting `partition` query values.
- [x] 2.2 Extend switch admission so scoped `slurm_to_openstack` requests default to the configured partition, reject mismatched explicit partitions, and block `openstack_to_slurm` for nodes outside the configured partition.
- [x] 2.3 Update API handler, service, and integration-style tests to cover scoped inventory responses, scoped switch acceptance, and scoped switch rejection cases.

## 3. Dashboard Behavior And Documentation

- [x] 3.1 Update dashboard startup and partition rendering to consume `slurmCloudPartition`, remove the "Show all partitions" view in fixed-partition mode, and keep node actions limited to the scoped inventory.
- [x] 3.2 Update dashboard switch submission logic so fixed-partition mode always sends the configured `slurm_partition` for `slurm_to_openstack`.
- [x] 3.3 Refresh operator docs and Swagger annotations/artifacts to describe the optional `SLURM_CLOUD_PARTITION` behavior for inventory and switch operations.
