## Context

The current switch API accepts a shared request shape for both directions, including an optional `node_name`. For `slurm_to_openstack`, that field is not authoritative because node selection happens later through the placeholder allocation event. Even so, the service currently persists any request-time `node_name`, and API tests rely on that behavior when reading execution status and lists.

This creates a contract mismatch: the documented workflow says `slurm_to_openstack` remains unbound until allocation, but the request layer still lets callers inject a node name up front. That is especially confusing in operator workflows and debugging because execution records can appear to identify a node before the placeholder job has actually claimed one.

## Goals / Non-Goals

**Goals:**

- Make the `slurm_to_openstack` API contract explicit: request-time `node_name` is invalid.
- Ensure newly created `slurm_to_openstack` executions remain unbound until the allocation event binds the node.
- Align API tests and operator-facing documentation with the actual workflow contract.

**Non-Goals:**

- Change the `openstack_to_slurm` contract, which still requires `node_name`.
- Change AMQP schemas for `execution.requested`, `execution.node_selected`, `execution.allocation`, or `execution.drained`.
- Redesign how placeholder allocation binds the execution to a node.

## Decisions

### 1. Reject `node_name` for `slurm_to_openstack` requests

The API/service layer will treat `node_name` as an invalid field when `direction=slurm_to_openstack` and return a client-visible validation error instead of persisting the value.

This is preferred over silently dropping the field because silent acceptance still suggests that the caller can meaningfully choose a node in this workflow. A validation error makes the directional contract obvious and prevents stale automation from continuing to send misleading data.

Alternative considered:

- Ignore `node_name` for `slurm_to_openstack` while accepting the request. Rejected because it preserves an ambiguous contract and makes client mistakes harder to notice.

### 2. Keep node binding exclusively on the allocation path

`slurm_to_openstack` executions will continue to start with an empty `node_name` and only gain a bound node when the placeholder agent publishes `execution.allocation`.

This preserves the existing MQ-driven allocation design and avoids unnecessary changes to consumer or placeholder-agent behavior.

Alternative considered:

- Add a separate request-time hint field for future node targeting. Rejected because the current issue is about removing a misleading contract, not introducing another pre-allocation selector.

### 3. Update tests and docs to match the directional contract

Tests that currently create `slurm_to_openstack` executions with request-time `node_name` will be rewritten to use valid inputs and to assert rejection when `node_name` is supplied. Operator docs and examples will describe `node_name` as an `openstack_to_slurm` input or an allocation-time output, but not as a valid `slurm_to_openstack` request field.

This reduces the chance that internal examples reintroduce the old assumption after the API validation changes land.

## Risks / Trade-offs

- [Backward incompatibility for existing clients that still send `node_name`] → Return a clear 400 validation error and update docs/examples in the same change.
- [Tests or tooling may rely on pre-bound `slurm_to_openstack` executions for convenience] → Replace those setups with valid request fixtures or direct store setup where a bound node is intentionally needed.
- [Documentation drift may remain outside OpenSpec] → Update the primary operator guide (`README.md`) and any nearby workflow docs touched by this contract.

## Migration Plan

1. Update API/service validation to reject `node_name` for `slurm_to_openstack`.
2. Adjust request/status/list tests that currently depend on request-time node persistence for this direction.
3. Update user-facing examples and field descriptions to remove `node_name` from `slurm_to_openstack` submission payloads.
4. Deploy with release notes noting that `slurm_to_openstack` submissions carrying `node_name` now fail validation.

Rollback is straightforward: restore the previous validation behavior if compatibility problems outweigh the contract cleanup.

## Open Questions

- None.
