# Operator Dashboard

The slurmtack dashboard is a browser-based interface served by the nginx container at `/`. It provides partition-scoped node inventory, ownership visualization, switch actions, and execution history drilldown without direct database or CLI access.

## Accessing the Dashboard

Open the stack's external HTTP entrypoint in a browser. On first load, the dashboard prompts for an API token (the same bearer token configured for the REST API). The token is stored in `localStorage` for subsequent visits.

## Layout

- **Header**: Shows backend health status (proxied via `/api/health`).
- **Partitions panel** (left): Lists discovered Slurm partitions. Click a partition to filter the node grid.
- **Nodes panel** (center): Displays one card per node with ownership badge, Slurm/OpenStack state summary, and available actions.
- **History panel** (right): Recent executions with filters by node, status, direction, and a requested-date range. The date range defaults to today back through the prior seven days. Click any execution to open the detail drawer.

## Node Ownership

Each node is classified as one of:

| Owner | Meaning |
|-------|---------|
| `slurm` | Node is active in Slurm (idle/alloc/mixed) and OpenStack compute service is disabled |
| `openstack` | OpenStack compute service is enabled and node is not active in Slurm |
| `switching` | An active execution is in progress for this node |
| `conflict` | Both Slurm and OpenStack report the node as active |
| `unknown` | Neither control plane reports the node as active |

## Slurm Job Settings

The dashboard requires a complete Slurm job settings profile before it allows `slurm_to_openstack` to be triggered. The settings entry point is the **Slurm Settings** button in the top-right header area.

### Required Fields

| Field | Description |
|-------|-------------|
| Slurm User Token | A JWT issued by the Slurm authentication system. Stored in browser `sessionStorage`. |
| Slurm Account | The Slurm account string (e.g. `proj-123`) used for placeholder job submission. |
| Placeholder SIF Filename | The container image filename (e.g. `placeholder-agent-debug.sif`) for the placeholder job. |

### Derived Workload User

The dashboard decodes the JWT payload (without signature verification) and extracts the effective workload username using the first non-empty claim in this order: `sun`, `username`, `preferred_username`, `sub`. The derived username is displayed as read-only feedback and is never stored separately — it is recomputed from the token each time the settings panel is opened or the page loads.

If the token cannot be decoded or no supported claim is present, the settings are treated as incomplete.

### Expected SIF Location Hint

The settings panel shows a read-only **Expected SIF Location** hint assembled from three inputs:

- The derived workload username (from the JWT)
- The `SLURM_SIF_PATH` value injected into the nginx container's runtime configuration at startup
- The placeholder SIF filename entered in the settings form

When all three are available, the dashboard displays the resolved path as:

```
/home/<derived-workload-user>/<SLURM_SIF_PATH>/<placeholder_sif_file>
```

For example, if the token resolves to `alice`, `SLURM_SIF_PATH=slurmtack/build/output`, and the filename is `placeholder-agent-debug.sif`, the hint reads:

```
/home/alice/slurmtack/build/output/placeholder-agent-debug.sif
```

This is the path where the SIF file **must exist** on the workload host before a `slurm_to_openstack` switch can succeed. The hint is guidance derived from deployment configuration — it does not verify that the file actually exists on disk.

When the path cannot be resolved, the hint shows explicit guidance:

- **No token or unresolvable user** — prompts the operator to enter a valid Slurm token so the workload user can be derived.
- **`SLURM_SIF_PATH` not configured** — indicates that the `SLURM_SIF_PATH` environment variable must be set in the deployment `.env` before the full path can be shown.
- **No SIF filename entered** — prompts the operator to fill in the placeholder SIF filename.

The hint recomputes live whenever the token or filename fields change. It reflects configuration injected at nginx container startup; both the nginx and daemon containers must be restarted together to pick up `.env` changes.

### Browser Storage

| Value | Storage |
|-------|---------|
| `slurm_user_token` | `sessionStorage` — cleared when the browser tab closes |
| `slurm_account` | `localStorage` — survives page reloads and restarts |
| `placeholder_sif_file` | `localStorage` — survives page reloads and restarts |

Use the **Clear** button in the settings panel to remove all stored values.

### Incomplete Settings Blocking

When the Slurm job settings are incomplete (missing token, undecodable username, missing account, or missing SIF filename), the dashboard blocks `slurm_to_openstack` submission with an operator-visible message. The **Slurm Settings** button appears highlighted when the profile is incomplete.

## Cloud Partition Scope (Optional)

When the deployment sets `SLURM_CLOUD_PARTITION` (e.g. `SLURM_CLOUD_PARTITION=gpu-cloud`), the system enters fixed-partition mode:

- **Inventory** (`GET /v1/dashboard/inventory`): Returns only the configured partition and its nodes. Requests with a conflicting `?partition=` filter receive HTTP 400.
- **Switch creation** (`POST /v1/switches`):
  - `slurm_to_openstack`: Uses the configured partition as the effective `slurm_partition`. Rejects an explicit `slurm_partition` that differs from the configured value.
  - `openstack_to_slurm`: Rejects nodes that are not members of the configured partition.
- **Dashboard UI**: The "Show all partitions" option is hidden. The partition list shows only the configured partition and switch actions always send `slurm_partition=<configured>`.

When `SLURM_CLOUD_PARTITION` is unset, all behavior remains unchanged (discovery-driven, all partitions visible).

Both the nginx and daemon containers must share the same `SLURM_CLOUD_PARTITION` value from the deployment `.env`. Restart both containers after changing this setting.

## Switch Actions

- **Switch to Slurm** (`openstack_to_slurm`): Available on node cards for nodes owned by OpenStack. Submits `POST /v1/switches` with `direction=openstack_to_slurm` and the node name.
- **Switch to OpenStack** (`slurm_to_openstack`): Rendered in a partition-scoped action bar above the node grid (not on individual node cards) because this workflow does not support request-time node targeting. Requires a complete Slurm job settings profile (see above). The request includes `slurm_account`, `placeholder_sif_file`, `slurm_user` (derived from token), and `slurm_user_token`. When the partition selection is `All`, the request omits `slurm_partition` so Slurm uses its default partition. When a specific partition is selected, the request includes `slurm_partition=<name>` to constrain placeholder job allocation. In fixed-partition mode, the configured cloud partition is always sent.
- **Cancel**: Available on node cards with an active execution. Submits `POST /v1/switches/:id/cancel`.

## Read Endpoints

### GET /v1/dashboard/inventory

Returns partition-scoped node inventory aggregated from Slurm, OpenStack, and execution state.

Query parameters:
- `partition` (optional): Limit response to a single Slurm partition.

### GET /v1/switches (expanded)

Lists executions with additional filters:
- `node`: Filter by node name
- `status`: Filter by overall status (`active`, `succeeded`, `failed`)
- `direction`: Filter by direction (`openstack_to_slurm`, `slurm_to_openstack`)
- `requested_from`: RFC3339 timestamp; returns only executions whose `requested_at` is at or after this instant
- `requested_to`: RFC3339 timestamp; returns only executions whose `requested_at` is at or before this instant
- `limit`: Maximum number of results
- `before`: RFC3339 timestamp for pagination (returns only executions before this time)

Results are ordered newest-first. A request where `requested_from` is after `requested_to` is rejected with HTTP 400. The `requested_from`/`requested_to` window is applied alongside the other filters and before pagination, so `before` cursors page through executions inside the selected range.

The dashboard's history panel exposes this range as **From** and **To** date inputs, defaulting to the operator's local calendar range from seven days before today through today. The selected dates are translated to the start (`00:00:00`) and end (`23:59:59`) of those local days before being sent as `requested_from` and `requested_to`. Changing either date resets the list to the first page.

### GET /v1/switches/:id (expanded)

Returns full execution metadata including `state_version`, `desired_owner`, `previous_owner`, lock timing, requested Slurm constraint/partition, placeholder job ID, allocation event timestamp, and cancellation source state.

### GET /v1/switches/:id/steps

Returns ordered step records for an execution, each including sequence, step name, host, timing, status, retry count, exit code, and error classification.

## Deployment

The dashboard is served as static HTML/JS from the nginx container's `/usr/share/nginx/html/` directory. All API calls use the existing `/v1/` proxy path already configured in nginx.

### Runtime Configuration

The nginx container generates a `dashboard-config.js` file at startup from whitelisted environment variables. This file is served at `/runtime/dashboard-config.js` with `Cache-Control: no-store` and loaded by the dashboard before `dashboard.js` executes.

The following environment variables are published through runtime configuration:

| Variable | Dashboard Field | Effect |
|----------|----------------|--------|
| `SLURM_SIF_PATH` | `slurmSifPath`, `slurmSifPathConfigured` | Enables the expected SIF location hint |
| `SLURM_CLOUD_PARTITION` | `slurmCloudPartition` | Activates fixed-partition mode in the dashboard |

These variables must be set in the deployment `.env` (the same source used by the daemon container). After changing `.env`, restart both the nginx and daemon containers for the new values to take effect.
