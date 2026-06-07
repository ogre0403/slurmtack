## Context

`internal/config/config.go` currently reads one `PLACEHOLDER_SIF_PATH` value and `internal/slurm/restclient.go` injects it directly into `singularity run <path>`. That assumes every `slurm_to_openstack` execution, regardless of workload user, can read the same absolute SIF path.

That model breaks in clusters where placeholder images are stored under each Slurm user's home directory. The system already resolves an execution-scoped workload identity and persists it for asynchronous placeholder submission, but it does not persist any execution-scoped SIF selection and the REST API has no way to override the image filename per request.

This change is cross-cutting because the selected SIF filename must survive request validation, execution persistence, async orchestrator processing, restart recovery, and operator documentation. It also needs to avoid turning a filename override into an arbitrary path override.

## Goals / Non-Goals

**Goals:**
- Replace the fixed placeholder SIF file path model with a home-based directory configuration plus an effective SIF filename for each execution.
- Allow `slurm_to_openstack` callers to override the placeholder SIF filename per request while keeping an env-provided default.
- Persist the effective filename needed by async placeholder submission so accepted executions do not change behavior when env defaults later change.
- Keep the resolved SIF path under the effective workload user's home directory and reject filename/path inputs that can escape that boundary.

**Non-Goals:**
- Supporting request-time override of the placeholder SIF directory path.
- Discovering user home directories from NSS, LDAP, or Slurm metadata.
- Verifying at request time that the selected SIF file already exists on every GPU node.
- Generalizing home resolution beyond the existing `/home/<user>` convention used elsewhere in the daemon.

## Decisions

### 1. Keep `PLACEHOLDER_SIF_PATH`, but change its meaning to a home-relative directory

**Choice:** The daemon will keep the env name `PLACEHOLDER_SIF_PATH`, but it will no longer represent a full file path. Instead it will represent a directory path relative to the effective workload user's home, for example `slurmtack/build/output`. A new env var `PLACEHOLDER_SIF_FILE` will hold the default SIF filename.

The resolved absolute directory becomes:

`/home/<effective-workload-user>/<PLACEHOLDER_SIF_PATH>`

The resolved SIF file becomes:

`/home/<effective-workload-user>/<PLACEHOLDER_SIF_PATH>/<effective-file>`

`PLACEHOLDER_SIF_PATH` must be a normalized home-relative directory segment. It must not be empty, absolute, or contain `..`.

**Rationale:** This keeps the operator-facing model close to the user's request ("PATH" plus "FILE") while enforcing that the SIF lives under each user's home directory. Reusing the existing env name minimizes deployment churn compared with introducing an entirely new path variable name.

**Alternatives considered:**
- Keep a full absolute path and only add a filename override: rejected because the permission problem remains if the absolute path points into one user's private directory.
- Introduce `${HOME}`-style template expansion: rejected because a plain home-relative directory is simpler to validate and avoids adding a mini template language.

### 2. Resolve the effective SIF filename during request intake and persist it on the execution

**Choice:** `POST /v1/switches` for `slurm_to_openstack` will accept an optional `placeholder_sif_file` field. The service resolves one effective filename during request handling:

1. Use `placeholder_sif_file` when the request provides it.
2. Otherwise use `PLACEHOLDER_SIF_FILE`.
3. Reject the request before creating an execution if neither source produces a valid filename.

The resolved filename is persisted on the execution record as a basename, not as a full path. Validation will require a non-empty simple filename that contains no path separators or traversal segments.

**Rationale:** Placeholder submission is asynchronous. If the daemon recomputed the filename later from env defaults, accepted executions could silently change behavior after a config update or restart. Persisting the effective filename freezes request semantics while still keeping the directory policy deployment-controlled.

**Alternatives considered:**
- Persist only the request override and recompute the default later: rejected because executions created without an override would become sensitive to later env changes.
- Persist the full resolved absolute path: rejected because it would freeze deployment-level path mistakes and make it harder for operators to correct the shared directory policy for queued executions.

### 3. Resolve the final absolute SIF path at placeholder-submit time from current config and persisted execution data

**Choice:** The Slurm client will build the runtime SIF path from:

- the execution's effective workload user,
- the current daemon `PLACEHOLDER_SIF_PATH` directory config,
- the execution's persisted effective filename.

The client will normalize and join those components before embedding the path into the placeholder script. Script generation will treat the resolved path as data, not raw shell text, so filenames cannot inject additional shell syntax.

**Rationale:** The filename is execution-specific and must be durable. The directory path is deployment policy and may need to be corrected globally without editing queued executions. Resolving the directory at submit time preserves that operator control.

**Alternatives considered:**
- Resolve and persist the full absolute path at request time: rejected because a later fix to `PLACEHOLDER_SIF_PATH` would not help already-queued executions.

### 4. Add migration fallback for pre-change executions with no stored filename

**Choice:** Execution persistence gains a new `placeholder_sif_file` column with an empty-string default for migrated rows. When the orchestrator or Slurm client handles an execution whose stored filename is empty, it will fall back to the current `PLACEHOLDER_SIF_FILE` so pre-change queued executions remain runnable after rollout.

New executions created by the updated API path will always store a non-empty effective filename.

**Rationale:** This avoids breaking in-flight executions during deployment while still moving the steady-state model to a persisted effective filename.

**Alternatives considered:**
- Require every existing row to be backfilled during migration: rejected because the original execution data does not include enough information to distinguish "request intentionally omitted filename" from "execution predates the field", and blocking rollout on manual DB repair is unnecessary.

### 5. Keep execution detail read APIs unchanged for now

**Choice:** The change will not add the effective placeholder SIF filename to `GET /v1/switches/:id` in this proposal.

**Rationale:** The user request only needs request-time control and async correctness. Exposing the filename is optional operator drilldown, not required for the behavior change itself. Keeping it internal narrows API churn for this change.

**Alternatives considered:**
- Expose `placeholder_sif_file` in execution detail APIs: rejected for now to keep the contract focused on the request and submission behavior. It can be added later if operators need that visibility.

## Risks / Trade-offs

- [Breaking config semantics for `PLACEHOLDER_SIF_PATH`] → Mitigation: update `docker/.env`, README examples, and deployment docs with explicit before/after conversion guidance from full file path to directory plus filename.
- [Filename override could become path traversal or shell injection] → Mitigation: validate `placeholder_sif_file` and `PLACEHOLDER_SIF_FILE` as simple basenames and quote the resolved runtime path in the generated script.
- [Clusters may not use `/home/<user>` as the workload home root] → Mitigation: document the assumption as part of this change and keep broader home-resolution configurability out of scope.
- [Migrated executions may not have a stored filename] → Mitigation: allow a temporary fallback to the current env default when the persisted filename is empty.

## Migration Plan

1. Convert deployment config from one full file path to split values, for example:
   - old: `PLACEHOLDER_SIF_PATH=/home/cloud-user/slurmtack/build/output/placeholder-agent.sif`
   - new: `PLACEHOLDER_SIF_PATH=slurmtack/build/output`
   - new: `PLACEHOLDER_SIF_FILE=placeholder-agent.sif`
2. Deploy the schema migration that adds the persisted execution filename column.
3. Roll out the API/service/orchestrator/Slurm client changes together with updated operator docs.
4. Verify one `slurm_to_openstack` request without `placeholder_sif_file` and one request with an explicit override filename.
5. Roll back only after restoring the old full-path config model and ensuring no queued executions depend on the new request field semantics.

## Open Questions

None. The design intentionally fixes home resolution to `/home/<user>` and keeps filename visibility out of read APIs so this change stays bounded.
