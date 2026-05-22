## 1. Dependencies

- [ ] 1.1 Add `github.com/gophercloud/gophercloud/v2` to go.mod
- [ ] 1.2 Add `github.com/gophercloud/gophercloud/v2/openstack` to go.mod

## 2. Client Implementation

- [ ] 2.1 Create `internal/openstack/gophercloud.go` with constructor `NewGophecloudClient(authOpts)` that authenticates with Keystone v3 and returns a `Client` interface implementation
- [ ] 2.2 Implement ListInstances: query `servers.List` with host filter, map to `[]Instance`
- [ ] 2.3 Implement ListActiveMigrations: query os-migrations with host+status filter, return migration IDs, gracefully handle 404
- [ ] 2.4 Implement GetComputeService: query os-services with host+binary filter, map to `ComputeServiceStatus`, error if not found
- [ ] 2.5 Implement DisableComputeService: resolve service ID via GetComputeService, then PUT disable with reason, handle idempotent case
- [ ] 2.6 Implement EnableComputeService: resolve service ID via GetComputeService, then PUT enable, handle idempotent case
- [ ] 2.7 Add error wrapping with operation name and hostname context on all methods

## 3. Unit Tests

- [ ] 3.1 Write unit tests using Gophercloud's `testhelper/client` fixtures to mock HTTP responses for each method
- [ ] 3.2 Test idempotent cases (disable already-disabled, enable already-enabled)
- [ ] 3.3 Test error wrapping includes operation and hostname context

## 4. Integration Tests

- [ ] 4.1 Create `internal/openstack/gophercloud_integration_test.go` gated with `//go:build integration`
- [ ] 4.2 Integration test: ListInstances on a known test host, verify response structure
- [ ] 4.3 Integration test: GetComputeService for test host, verify fields
- [ ] 4.4 Integration test: DisableComputeService then EnableComputeService on test host, verify state transitions
- [ ] 4.5 Integration test: ListActiveMigrations on test host (expect empty, confirm no error)
