## Why

Some deployments should expose and operate on only one Slurm partition that represents the cloud-switchable node pool. The current dashboard and inventory API always discover all partitions, which means operators can see and trigger switch actions outside the intended cloud scope unless they rely on convention alone.

This needs to stay optional so existing installations keep the current behavior when no extra configuration is provided. Because the dashboard is a static UI served by nginx, the scoped partition also needs to be exposed through the existing safe runtime config mechanism rather than a new browser-only API.

## What Changes

- Add an optional `SLURM_CLOUD_PARTITION` environment variable to deployment and daemon configuration.
- Keep the current inventory and switch behavior unchanged when `SLURM_CLOUD_PARTITION` is unset.
- When `SLURM_CLOUD_PARTITION` is set, constrain the dashboard inventory scope to that partition only and exclude nodes that are not members of the configured partition.
- When `SLURM_CLOUD_PARTITION` is set, reject switch actions that target nodes or partition selections outside the configured cloud partition so the restriction is enforced server-side, not only in the UI.
- Extend the nginx-generated dashboard runtime config with a safe `slurmCloudPartition` value so the browser can hide non-cloud partition choices and remove the misleading "All partitions" view when the deployment is explicitly scoped.
- Update operator documentation and generated API descriptions to explain the optional scoped-partition behavior.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `daemon-deployment`: add the optional `SLURM_CLOUD_PARTITION` environment contract and require that the nginx runtime config whitelist can expose it safely to the dashboard.
- `rest-api`: change dashboard inventory and switch request behavior so a configured cloud partition limits visible partition data and allowed switch targets.
- `node-switch-dashboard`: change the dashboard partition navigation and action controls to honor the runtime-configured cloud partition scope.
- `swagger-generation`: update generated API descriptions for the scoped inventory and switch behavior.

## Impact

- Affected code: `internal/config`, `cmd/main.go`, switch/inventory services and handlers, nginx runtime config generation, dashboard UI logic, deployment manifests, and related tests.
- Public behavior: `GET /v1/dashboard/inventory` and `POST /v1/switches` gain optional partition-scope enforcement when `SLURM_CLOUD_PARTITION` is configured.
- Updated docs: dashboard/operator docs, `.env` examples, and generated Swagger output.
