## 1. Nginx Validation UI

- [x] 1.1 Add an nginx config file and static asset directory for serving `/` and proxying `/api/health` to slurmtack's internal `/health` endpoint
- [x] 1.2 Create a single static HTML validation page that fetches `/api/health` on load and renders healthy/unhealthy status without referencing internal container addresses

## 2. Container Topology

- [x] 2.1 Update `docker/docker-compose.yaml` to add an `nginx` service, move the stack to a shared bridge network, and expose only nginx to the host while leaving slurmtack internal-only
- [x] 2.2 Preserve RabbitMQ host access on ports 5672 and 15672 and update any env/example configuration needed for service-to-service communication on the compose network

## 3. Documentation and Verification

- [x] 3.1 Update the deployment documentation to describe the nginx entrypoint, the static validation page, and the proxied `/api/health` check flow
- [x] 3.2 Run a compose-based smoke check that verifies `/` loads through nginx, `/api/health` returns the proxied health response, and slurmtack has no direct host port mapping
