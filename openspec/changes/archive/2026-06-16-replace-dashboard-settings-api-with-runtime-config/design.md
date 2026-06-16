## Context

The dashboard is served as static assets from the nginx container, while the daemon loads `SLURM_SIF_PATH` from environment variables and currently re-exposes it through `GET /v1/dashboard/settings`. That route is authenticated, documented in Swagger, and covered by API and UI tests even though it exists only to let the browser assemble a read-only SIF-location hint.

At the same time, the browser cannot read the server-side `.env` directly. If we want to remove the API, the replacement must be a deployment-time or startup-time injection mechanism rather than a direct filesystem read from the UI.

## Goals / Non-Goals

**Goals:**

- Let the dashboard read `SLURM_SIF_PATH` without calling a dedicated backend API.
- Keep the exposed surface limited to explicitly whitelisted, non-secret settings.
- Remove the now-unnecessary API route, DTOs, Swagger annotations, docs, and tests.
- Keep the existing SIF hint behavior and guidance text intact from the operator's perspective.

**Non-Goals:**

- Expose arbitrary `.env` variables to the browser.
- Change daemon-side validation or runtime use of `SLURM_SIF_PATH`.
- Introduce a new authenticated configuration API as a replacement.

## Decisions

### Decision: Use an nginx-generated runtime JavaScript config file

The nginx container will generate a small `dashboard-config.js` file at startup that assigns `window.SLURMTACK_CONFIG`. `index.html` will load this file before `dashboard.js`, allowing the dashboard to synchronously read configuration during initialization.

This matches the actual deployment boundary: the browser reads a public asset served by nginx, not the daemon's `.env`, and the config is available without an extra network round-trip through the API.

**Alternatives considered**

- Keep `GET /v1/dashboard/settings`: rejected because it preserves a special-purpose API contract solely for one safe UI hint.
- Have the browser fetch `.env` or mount it directly: rejected because browsers cannot safely read server-side env files, and exposing raw env files would be unsafe and overly broad.

### Decision: Whitelist only safe dashboard settings

The generated runtime config will include only `slurmSifPath` and `slurmSifPathConfigured`. The generation step must build the file from explicit assignments, not by serializing the whole environment.

This keeps the exposure boundary narrow and makes future additions opt-in.

**Alternatives considered**

- Dump selected env vars generically by prefix: rejected because it encourages accidental exposure growth.
- Embed config inline in `index.html`: rejected because a separate generated file is easier to no-cache and easier to evolve without rewriting the static HTML template.

### Decision: Generate the file in a writable runtime location and serve it explicitly

The current dashboard assets are mounted read-only into `/usr/share/nginx/html`, so the generated config file cannot be written back into that tree. The nginx container should generate `dashboard-config.js` in a writable path and expose it through nginx configuration.

This avoids mutating the read-only dashboard asset mount and keeps the static source tree unchanged at runtime.

**Alternatives considered**

- Make the dashboard asset mount writable and generate into the source tree: rejected because it weakens deployment discipline and couples runtime output to repo-mounted assets.

### Decision: Remove the dashboard settings API completely

Once the dashboard reads runtime config, the implementation should delete the settings handler, route, DTO, Swagger annotation coverage, and related tests instead of keeping both mechanisms in parallel.

This prevents drift between two sources of truth and makes the public contract explicit.

**Alternatives considered**

- Keep both API and runtime config temporarily: rejected for this change because there is no compatibility consumer other than the bundled dashboard.

## Risks / Trade-offs

- [Risk] `SLURM_SIF_PATH` becomes readable by anyone who can load the dashboard assets. → Mitigation: treat it as accepted non-secret operator guidance and expose only the validated home-relative path segment.
- [Risk] nginx and daemon could see different values if their environments drift. → Mitigation: wire both services from the same `.env` source in Compose and document that both containers must be restarted together after config changes.
- [Risk] Browsers may cache stale runtime config. → Mitigation: serve `dashboard-config.js` with `Cache-Control: no-store`.
- [Risk] Removing the route breaks tests/docs that still reference it. → Mitigation: update API tests, dashboard UI string tests, Swagger artifacts, and dashboard documentation in the same change.

## Migration Plan

1. Add nginx runtime config generation and switch the dashboard to consume it.
2. Remove `/v1/dashboard/settings` from server registration, DTOs, tests, and Swagger annotations.
3. Regenerate/update documentation and Swagger artifacts to reflect the reduced API surface.
4. Deploy by restarting both nginx and daemon containers from the same `.env`; rollback by restoring the API route and dashboard fetch logic if needed.

## Open Questions

None.
