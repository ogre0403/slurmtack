## ADDED Requirements

### Requirement: Repo-owned fake passthrough bundle matches the existing passthrough interface

The system SHALL provide a repo-owned fake passthrough script bundle for non-GPU environments. The bundle MUST expose the same entrypoint filenames and single-argument CLI contract as the existing passthrough bundle: `reconfigure.sh <enable|disable>` and `verify.sh <enable|disable>`. Any helper files sourced by those entrypoints MUST live in the same directory so the orchestrator can stage and execute the bundle without special-case handling.

#### Scenario: Fake bundle can be selected through the existing script-dir configuration
- **WHEN** an operator points `GPU_PASSTHROUGH_SCRIPT_DIR` at the fake passthrough directory
- **THEN** the selected directory contains the same `reconfigure.sh` and `verify.sh` entrypoints expected by the existing orchestration flow
- **AND** the bundle can be staged and invoked without renaming files or adding a new orchestration-specific interface

### Requirement: Fake passthrough reconfiguration succeeds deterministically on hosts without GPUs

The system SHALL provide a standalone executable fake passthrough reconfiguration script that accepts exactly one action argument, `enable` or `disable`, and completes successfully as a deterministic no-op workflow for either action. The script MUST make it clear from its output that it is a fake/test-only passthrough operation, and it MUST exit non-zero for unsupported actions. The script MUST NOT fail only because the host lacks NVIDIA GPUs, VFIO configuration, or passthrough kernel state.

#### Scenario: Fake enable reconfigure succeeds on a GPU-less host
- **WHEN** an operator runs the fake reconfiguration script with `enable` on a host without NVIDIA GPUs
- **THEN** the script exits successfully
- **AND** the script reports that fake passthrough enable was simulated without requiring real GPU host changes

#### Scenario: Fake disable reconfigure succeeds on a GPU-less host
- **WHEN** an operator runs the fake reconfiguration script with `disable` on a host without NVIDIA GPUs
- **THEN** the script exits successfully
- **AND** the script reports that fake passthrough disable was simulated without requiring real GPU host changes

#### Scenario: Fake reconfigure rejects unsupported actions
- **WHEN** an operator runs the fake reconfiguration script with an action other than `enable` or `disable`
- **THEN** the script exits with a non-zero status
- **AND** the script reports that the action is invalid

### Requirement: Fake passthrough verification succeeds deterministically on hosts without GPUs

The system SHALL provide a standalone executable fake passthrough verification script that accepts exactly one action argument, `enable` or `disable`, and completes successfully as a test-only verification path for either action. The script MUST make it clear from its output that the verification is fake and MUST exit non-zero for unsupported actions. The script MUST NOT require real GPU hardware, VFIO bindings, or passthrough kernel state to report success.

#### Scenario: Fake enable verification succeeds after a test reboot flow
- **WHEN** an operator runs the fake verification script with `enable` in a non-GPU test environment
- **THEN** the script exits successfully
- **AND** the script reports that fake passthrough enabled-state verification was simulated

#### Scenario: Fake disable verification succeeds after a test reboot flow
- **WHEN** an operator runs the fake verification script with `disable` in a non-GPU test environment
- **THEN** the script exits successfully
- **AND** the script reports that fake passthrough disabled-state verification was simulated
