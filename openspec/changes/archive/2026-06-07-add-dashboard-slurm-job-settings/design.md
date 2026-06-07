## Context

The dashboard is a static HTML/JavaScript page served from `docker/nginx/html/` and currently keeps only the API bearer token in browser `localStorage`. Recent archived changes added request-scoped `slurm_account`, `slurm_user`, `slurm_user_token`, and `placeholder_sif_file` support to `POST /v1/switches`, plus placeholder submission behavior that depends on those fields. The missing piece is a dashboard-side way to collect those values for `slurm_to_openstack` without making the operator craft raw API requests, while also preventing a UI-triggered switch when that Slurm submission profile is incomplete.

The requested UX is page-global rather than node-specific: the values apply when the operator launches a partition-scoped `slurm_to_openstack` action, and the username should not be a manual input because it can be derived from the Slurm JWT payload.

## Goals / Non-Goals

**Goals:**

- Add a top-right dashboard settings entry point for placeholder-job submission settings.
- Persist operator-entered Slurm job settings locally in the browser across page reloads.
- Derive one effective Slurm workload username from the JWT payload and show it as read-only feedback.
- Merge the stored settings into `slurm_to_openstack` request payloads without changing `openstack_to_slurm` behavior.
- Fail early in the dashboard when the Slurm job settings are incomplete or when a stored Slurm token cannot be decoded into a usable workload username.

**Non-Goals:**

- Adding new backend endpoints, DTO fields, or server-side persistence for dashboard settings.
- Verifying JWT signatures in the browser or making the browser the source of truth for token validity.
- Changing execution detail APIs, inventory APIs, or the placeholder-job backend lifecycle introduced by the June 7 changes.

## Decisions

### Store Slurm job settings as a dashboard-local profile in `localStorage`

**Choice:** The dashboard will store one JSON object in browser `localStorage` for `slurm_user_token`, `slurm_account`, and `placeholder_sif_file`. The header-level settings UI will load that profile on startup, let the operator update or clear it, and recompute the derived username whenever the token changes.

**Rationale:** The dashboard is already a static client that stores the API bearer token locally, and this change is explicitly operator-specific rather than cluster-global. Keeping the profile in the browser avoids backend secret persistence and fits the existing deployment model.

**Alternatives considered:**

- Store Slurm job settings server-side: rejected because it would require a new secret-bearing API and persistence model for what is currently a pure static dashboard.
- Use `sessionStorage`: rejected because the operator would need to re-enter the Slurm settings after each reload or browser restart.

### Derive `slurm_user` client-side from a deterministic JWT claim precedence

**Choice:** The dashboard will decode the JWT payload with base64url parsing only and derive the effective workload username from the first non-empty supported claim in this order: `sun`, `username`, `preferred_username`, then `sub`. The derived username will be displayed as read-only output in the settings UI and will not be stored separately from the token.

**Rationale:** The user asked to avoid a manual username field, and the browser already has the token needed to derive that value. Keeping the derived username ephemeral prevents stale drift between a stored token and a separately stored username.

**Alternatives considered:**

- Require the operator to type `slurm_user`: rejected because it duplicates token-contained identity and increases mismatch risk.
- Support only one claim name: rejected because token payload shapes vary across environments, while the UI still needs deterministic behavior.

### Treat the Slurm job settings profile as required for dashboard-triggered `slurm_to_openstack`

**Choice:** When the operator launches `slurm_to_openstack`, the dashboard will require one complete, valid Slurm job settings profile before it sends `POST /v1/switches`. A complete profile consists of:

- a non-empty `slurm_user_token` that decodes to a supported workload username;
- a non-empty `slurm_account`;
- a non-empty `placeholder_sif_file`.

When the profile is complete, the dashboard will send:

- `slurm_account`;
- `placeholder_sif_file`;
- `slurm_user_token` plus the derived `slurm_user`.

If the profile is incomplete or invalid, the dashboard will not send the switch request and will instead surface an operator-visible validation message. `openstack_to_slurm`, inventory reads, execution history, and cancellation remain unchanged.

**Rationale:** The user clarified that this Slurm information is mandatory for running the placeholder-backed `slurm_to_openstack` flow from the UI. Making the dashboard enforce completeness prevents avoidable failed submissions and keeps the operator-facing workflow explicit.

**Alternatives considered:**

- Allow missing settings and rely on daemon defaults: rejected because it conflicts with the required operator workflow for dashboard-triggered `slurm_to_openstack`.
- Reuse the API bearer token as the Slurm workload token: rejected because the dashboard already treats API auth and Slurm workload auth as separate concerns.

### Put the settings entry point in the header beside the existing status controls

**Choice:** The dashboard header will gain a top-right settings button or similar control beside the health badge. Opening it reveals a compact settings surface for the three editable values plus the derived username and clear/save actions.

**Rationale:** The requested location is top-right, and these settings are page-wide execution defaults rather than node-card data. Keeping them in the header makes the scope clear and avoids cluttering the partition action bar.

**Alternatives considered:**

- Add the inputs directly into the partition action bar: rejected because they would crowd the primary action area and repeat across partition changes.
- Put settings in the right-side execution panel: rejected because that panel is for history drilldown, not request composition.

## Risks / Trade-offs

- [Slurm workload token is stored in browser `localStorage`] → Mitigation: keep the token confined to the operator's browser, never echo it into execution detail views or logs, and provide a clear action in the settings UI.
- [Different JWT payload shapes may not expose a supported username claim] → Mitigation: use a documented claim precedence and block `slurm_to_openstack` submission with a clear dashboard error when no username can be derived.
- [Client-side decode does not verify token authenticity] → Mitigation: use decode only to populate the paired request fields and operator feedback; the backend and Slurm API remain responsible for accepting or rejecting the token at execution time.
- [The dashboard becomes stricter than the raw API, which still supports daemon defaults] → Mitigation: document that this requirement applies to dashboard-triggered `slurm_to_openstack` actions, not to non-UI API clients.
- [Operators may confuse the Slurm job settings with the API bearer token] → Mitigation: label the feature specifically as Slurm job or placeholder submission settings and keep it visually separate from API auth prompting.

## Migration Plan

1. Update the dashboard HTML and JavaScript to add the header settings control, local settings state, JWT decoding, required-settings validation, and `slurm_to_openstack` payload enrichment.
2. Extend dashboard-facing tests to assert the new header region, settings-related JavaScript behavior, action gating, and payload composition.
3. Update dashboard documentation to explain which settings are stored locally, how the username is derived, and that the settings are required for dashboard-triggered `slurm_to_openstack`.

Rollback is low-risk because the change is frontend-only. Reverting the static assets removes the browser-side settings flow while leaving the existing backend request fields and defaults intact.

## Open Questions

- None.
