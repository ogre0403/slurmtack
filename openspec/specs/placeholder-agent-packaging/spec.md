## ADDED Requirements

### Requirement: Static binary build

The placeholder-agent MUST be compiled as a static Linux/amd64 binary with `CGO_ENABLED=0` so it runs in any Singularity container without shared library dependencies.

#### Scenario: Build produces static binary

- **WHEN** `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/placeholder-agent/` is run
- **THEN** output is a statically linked ELF binary with no dynamic library requirements

#### Scenario: Binary runs in minimal container

- **WHEN** the binary is placed in an alpine-based Singularity image
- **THEN** it executes without "not found" or shared library errors

### Requirement: Singularity definition file

A Singularity definition file (`build/placeholder-agent.def`) SHALL define the container image: alpine base, binary copied to `/usr/local/bin/`, and runscript that executes the agent.

#### Scenario: Build SIF image

- **WHEN** `singularity build placeholder-agent.sif build/placeholder-agent.def` is run with the binary present
- **THEN** a valid SIF image is produced

#### Scenario: Run SIF with environment

- **WHEN** `singularity run placeholder-agent.sif` is executed with required env vars set
- **THEN** the agent starts and behaves identically to running the bare binary

### Requirement: Shared filesystem deployment

The SIF image MUST be deployable to a shared filesystem path accessible by all GPU nodes. The daemon's job submission MUST reference this path via configurable `PLACEHOLDER_SIF_PATH`.

#### Scenario: Submit job with SIF path

- **WHEN** daemon submits a placeholder job
- **THEN** the sbatch command references `singularity run <PLACEHOLDER_SIF_PATH>`

#### Scenario: SIF not found on node

- **WHEN** the SIF path is invalid or not mounted on the allocated node
- **THEN** the Slurm job fails immediately and the daemon detects a non-zero exit code

### Requirement: Build script

A build script (`build/build-placeholder-agent.sh`) SHALL automate: compile the static binary, then build the SIF image. It MUST be runnable from the project root.

#### Scenario: Full build

- **WHEN** `./build/build-placeholder-agent.sh` is run on a machine with Go and Singularity installed
- **THEN** it produces `build/output/placeholder-agent.sif`

#### Scenario: Binary-only build

- **WHEN** Singularity is not available but Go is
- **THEN** the script produces the binary and prints a warning that SIF build was skipped
