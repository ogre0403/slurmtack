## MODIFIED Requirements

### Requirement: Shared filesystem deployment

The SIF image MUST be deployable through a user-home directory pattern instead of one daemon-wide absolute file path. The daemon's job submission MUST resolve the runtime image reference as `singularity run /home/<workload-user>/<PLACEHOLDER_SIF_PATH>/<effective-file>`, where `PLACEHOLDER_SIF_PATH` is a configured home-relative directory, `PLACEHOLDER_SIF_FILE` is the default filename, and `placeholder_sif_file` may override the filename per `slurm_to_openstack` request. Operators MUST ensure the matching SIF file exists in the resolved directory for each workload user that may submit placeholder jobs.

#### Scenario: Submit job with default SIF filename

- **WHEN** daemon submits a placeholder job for workload user `alice`
- **AND** `PLACEHOLDER_SIF_PATH=slurmtack/build/output`
- **AND** `PLACEHOLDER_SIF_FILE=placeholder-agent.sif`
- **AND** the request body does not provide `placeholder_sif_file`
- **THEN** the sbatch command references `singularity run /home/alice/slurmtack/build/output/placeholder-agent.sif`

#### Scenario: Submit job with request-time SIF filename override

- **WHEN** daemon submits a placeholder job for workload user `alice`
- **AND** `PLACEHOLDER_SIF_PATH=slurmtack/build/output`
- **AND** the request body resolves `placeholder_sif_file=placeholder-agent-debug.sif`
- **THEN** the sbatch command references `singularity run /home/alice/slurmtack/build/output/placeholder-agent-debug.sif`

#### Scenario: SIF not found on node

- **WHEN** the resolved per-user SIF path is invalid or the file is not present on the allocated node
- **THEN** the Slurm job fails immediately and the daemon detects a non-zero exit code
