## Context

The `openstack.Client` interface defines 5 methods (ListInstances, ListActiveMigrations, GetComputeService, DisableComputeService, EnableComputeService) that map to Nova compute API operations. The test OpenStack cluster uses Keystone v3 for authentication with username/password credentials.

Gophercloud is the standard Go SDK for OpenStack and provides typed access to Nova, Keystone, and other services. It handles token lifecycle (auto-renewal on expiry) internally.

## Goals / Non-Goals

**Goals:**

- Provide a production-ready Gophercloud client implementing `openstack.Client`
- Handle Keystone v3 authentication with automatic token renewal
- Map Gophercloud Nova responses to existing Go types (`Instance`, `ComputeServiceStatus`)
- Provide clear errors when OpenStack API calls fail
- Support integration testing against real OpenStack

**Non-Goals:**

- Application credential support (username/password is sufficient for staging)
- Caching of API responses (calls are infrequent, freshness matters)
- Retry logic (handled by orchestrator layer)
- Support for multiple OpenStack regions or clouds
- Ironic/bare-metal integration (only Nova compute API)

## Decisions

### SDK: Gophercloud v2

**Choice**: `github.com/gophercloud/gophercloud/v2`

**Alternatives considered**:
- Raw HTTP calls: Would need to reimplement auth token lifecycle, pagination, and response parsing
- OpenStack CLI wrapper: Fragile, requires CLI installed in container, output parsing

**Rationale**: Gophercloud is the de facto Go OpenStack SDK. It handles token renewal, pagination, and provides typed request/response structs. v2 is the actively maintained version.

### API Mapping

| Interface Method | Gophercloud Package | Nova API |
|---|---|---|
| ListInstances | `compute/v2/servers` | `GET /servers/detail?host={hostname}` |
| ListActiveMigrations | `compute/v2/extensions/migrations` | `GET /os-migrations?host={hostname}&status=running` |
| GetComputeService | `compute/v2/extensions/services` | `GET /os-services?host={hostname}&binary=nova-compute` |
| DisableComputeService | `compute/v2/extensions/services` | `PUT /os-services/{id}` with status=disabled |
| EnableComputeService | `compute/v2/extensions/services` | `PUT /os-services/{id}` with status=enabled |

### Authentication: Keystone v3 Password

**Choice**: `openstack.AuthenticatedClient` with `gophercloud.AuthOptions` (username, password, project-scoped)

The client is constructed once at daemon startup. Gophercloud handles token caching and re-authentication internally when tokens expire.

Environment variables follow the standard OpenStack convention:
- `OS_AUTH_URL` — Keystone endpoint (e.g., `http://controller:5000/v3`)
- `OS_USERNAME`
- `OS_PASSWORD`
- `OS_PROJECT_NAME`
- `OS_USER_DOMAIN_NAME` (default: "Default")
- `OS_PROJECT_DOMAIN_NAME` (default: "Default")

### Error Handling

Gophercloud errors include `gophercloud.ErrUnexpectedResponseCode` with the HTTP status and response body. The client SHALL:

1. Wrap Gophercloud errors with context (which operation, which host)
2. Detect 404 (not found) specifically for GetComputeService
3. Return all other errors as-is (wrapped) — the orchestrator decides retry policy

### Service ID Resolution

`DisableComputeService` and `EnableComputeService` need the service record ID, not just the hostname. The flow is:

1. Call `GET /os-services?host={hostname}&binary=nova-compute`
2. Extract the service ID from the result
3. Call `PUT /os-services/{id}` with the desired status

This two-step pattern is encapsulated within each method call.

## Risks / Trade-offs

- **Gophercloud version pinning** → OpenStack APIs are stable but Gophercloud v2 is still evolving. Mitigation: pin to a specific minor version in go.mod.
- **Token expiry during long gaps** → If the daemon sits idle for hours, the cached token may expire. Mitigation: Gophercloud auto-renews; no action needed unless Keystone itself is down.
- **ListActiveMigrations API availability** → The os-migrations extension may not be available on all OpenStack versions. Mitigation: test against target cluster; fall back to empty list if 404.
- **Integration tests mutate state** → Disable/enable compute service affects scheduling. Mitigation: use a dedicated test host not in active rotation.
