## Why

The `openstack.Client` interface exists but has no real implementation — only fakes used in tests. The daemon needs to interact with a real OpenStack deployment to list instances on a host, check/manage compute service state, and verify migration status. Without this, the orchestration engine cannot perform OpenStack-side precheck, quiesce, detach, or attach operations during GPU node switches.

## What Changes

- Implement `openstack.Client` interface using Gophercloud, targeting Nova compute API
- Add Keystone v3 authentication (project-scoped token via username/password)
- Map all 5 interface methods to Nova API calls
- Add integration tests that run against a real OpenStack test cluster (gated by build tag)
- Handle OpenStack API errors and map to meaningful Go errors

## Capabilities

### New Capabilities

- `gophercloud-client`: Gophercloud-based implementation of the openstack.Client interface covering instance listing, migration checks, compute service enable/disable, and service state queries with Keystone v3 auth

### Modified Capabilities

(none — the `openstack.Client` interface contract remains unchanged)

## Impact

- **New files**: `internal/openstack/gophercloud.go`, `internal/openstack/gophercloud_test.go`
- **New dependencies**: `github.com/gophercloud/gophercloud/v2`, `github.com/gophercloud/gophercloud/v2/openstack`
- **External systems**: Requires access to OpenStack Keystone and Nova APIs for integration tests
- **Configuration**: Uses `OS_AUTH_URL`, `OS_PROJECT_NAME`, `OS_USERNAME`, `OS_PASSWORD`, `OS_USER_DOMAIN_NAME`, `OS_PROJECT_DOMAIN_NAME` env vars (standard OpenStack env)
