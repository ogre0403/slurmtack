## Context

The dashboard already derives one effective Slurm workload username from the operator-provided JWT and stores the placeholder SIF filename in browser state, but it has no access to the daemon's configured `SLURM_SIF_PATH`. The daemon and docs already define the runtime SIF location as `/home/<effective-workload-user>/<SLURM_SIF_PATH>/<effective-file>`, so the missing piece is a safe way for the browser to learn the home-relative path segment and render the same resolution logic before submission.

The existing dashboard inventory endpoint is focused on partition/node state. Reusing it for daemon configuration metadata would couple unrelated concerns and make the settings hint depend on inventory refresh timing. This change only needs non-secret path metadata and must not expose workload tokens, default filenames, or any server-side file existence probing.

## Goals / Non-Goals

**Goals:**
- Let the dashboard show the operator the resolved absolute SIF location using the derived workload username, configured `SLURM_SIF_PATH`, and typed filename.
- Expose only the non-secret configuration needed for that hint through an authenticated API contract.
- Keep the current `slurm_to_openstack` request payload and backend SIF resolution rules unchanged.
- Make incomplete-path states explicit so operators know whether the missing piece is the token-derived user, the filename, or daemon path configuration.

**Non-Goals:**
- Verifying that the resolved SIF file actually exists on disk or is readable from the workload host.
- Changing how the daemon validates `SLURM_SIF_PATH` or resolves the runtime path during job submission.
- Introducing a new persistence model for dashboard settings.

## Decisions

### 1. Add a dedicated authenticated dashboard-settings read endpoint

**Choice:** Add a protected dashboard metadata endpoint that returns the configured home-relative `SLURM_SIF_PATH` needed by the UI. The response exposes only safe configuration intended for operator guidance, not secrets or server-expanded absolute paths.

- **Rationale:** The browser cannot read `.env` directly, and the nginx-served static dashboard has no other reliable source for daemon runtime configuration. A dedicated endpoint keeps this concern separate from inventory polling and avoids overloading switch/detail responses with unrelated settings metadata.
- **Alternatives Considered:**
  - *Embed the path into `/v1/dashboard/inventory`*: Rejected because SIF-path guidance is not inventory data and would be refreshed only as a side effect of inventory polling.
  - *Render config into static HTML at build time*: Rejected because the dashboard is served statically and the value must reflect runtime daemon configuration, not image-build state.

### 2. Resolve the final operator hint in the browser from three inputs

**Choice:** Keep path assembly in dashboard JavaScript using the existing derived username logic plus the server-provided `SLURM_SIF_PATH` and the currently entered `placeholder_sif_file`. When all three inputs are present, the UI renders `/home/<derived-user>/<slurm_sif_path>/<placeholder_sif_file>` as read-only feedback.

- **Rationale:** The dashboard already owns JWT decoding and live form state, so it can recompute the hint immediately on token or filename edits without extra API calls. This keeps the feedback responsive and ensures the displayed path always matches the current unsaved form inputs.
- **Alternatives Considered:**
  - *Return a fully resolved absolute path from the server*: Rejected because the relevant username and filename may change in the browser before the operator saves or submits settings.
  - *Require the operator to infer the full path from separate fields*: Rejected because that keeps the current usability gap and does not actually reduce mistakes.

### 3. Treat the hint as deterministic guidance, not filesystem validation

**Choice:** The UI will present the resolved path as the expected SIF location and separately explain when the path cannot be computed. It will not attempt remote file checks, SSH probes, or backend `stat` calls.

- **Rationale:** The request asks for a prompt that tells users where the SIF should exist so they can verify placement. Real existence checks would require host-level access, introduce latency and failure modes unrelated to switch submission, and still would not guarantee the same filesystem context used at Slurm job runtime.
- **Alternatives Considered:**
  - *Probe the filesystem from the API server or over SSH*: Rejected because it introduces a new operational dependency and turns a UI hint into a host-access feature.
  - *Do nothing when the path is incomplete*: Rejected because operators would still have no idea which input or daemon config is missing.

## Risks / Trade-offs

- **[Risk] The dashboard exposes a daemon path convention to authenticated operators** → Mitigation: expose only the validated home-relative `SLURM_SIF_PATH` segment, not secrets or arbitrary filesystem reads.
- **[Risk] Operators may read the hint as proof that the file exists** → Mitigation: label the value as the expected SIF location and keep wording explicit that the path is where the file must be placed for the derived user.
- **[Risk] Runtime config changes will not appear in an already-open dashboard tab** → Mitigation: load the metadata during dashboard startup and document that a page reload refreshes the hint after daemon config changes.

## Migration Plan

1. Add the authenticated dashboard metadata endpoint that returns `SLURM_SIF_PATH` guidance.
2. Update the dashboard state model and settings panel to fetch that metadata and render the resolved path hint.
3. Extend UI/API tests and dashboard docs to cover the new hint behavior and missing-config messaging.
4. Deploy with no data migration; rollback is a standard code rollback because no persisted schema or stored payloads change.

## Open Questions

None.
