## Requirements

### Requirement: List instances on a host

The client SHALL query Nova for all server instances on a given host via `GET /servers/detail?host={hostname}` and return them as a slice of `Instance` structs with ID, Name, and Status fields.

#### Scenario: Host with instances

- **WHEN** ListInstances is called with a hostname that has running VMs
- **THEN** client returns a slice containing each instance's ID, Name, and Status

#### Scenario: Host with no instances

- **WHEN** ListInstances is called with a hostname that has no VMs
- **THEN** client returns an empty slice and nil error

#### Scenario: API error

- **WHEN** Nova returns an error response
- **THEN** client returns an error wrapping the HTTP status and message

### Requirement: List active migrations on a host

The client SHALL query Nova for active migrations on a given host via the os-migrations API and return a slice of migration IDs.

#### Scenario: No active migrations

- **WHEN** ListActiveMigrations is called on a host with no in-progress migrations
- **THEN** client returns an empty slice and nil error

#### Scenario: Active migrations exist

- **WHEN** ListActiveMigrations is called on a host with running migrations
- **THEN** client returns a slice of migration ID strings

#### Scenario: Migrations API unavailable

- **WHEN** the os-migrations extension is not available (404)
- **THEN** client returns an empty slice and nil error (graceful degradation)

### Requirement: Get compute service status

The client SHALL query Nova for the compute service on a given host via `GET /os-services?host={hostname}&binary=nova-compute` and return a `ComputeServiceStatus` struct.

#### Scenario: Service exists

- **WHEN** GetComputeService is called with a hostname running nova-compute
- **THEN** client returns ComputeServiceStatus with Host, Status, State, and Enabled fields populated

#### Scenario: Service not found

- **WHEN** GetComputeService is called with a hostname that has no nova-compute service
- **THEN** client returns an error indicating the service was not found

### Requirement: Disable compute service

The client SHALL disable the nova-compute service on a given host by resolving the service ID and updating its status to disabled with the provided reason.

#### Scenario: Successful disable

- **WHEN** DisableComputeService is called with a valid hostname and reason
- **THEN** client resolves the service ID, sends PUT with status=disabled and reason, returns nil

#### Scenario: Already disabled

- **WHEN** DisableComputeService is called on an already-disabled service
- **THEN** client returns nil (idempotent)

#### Scenario: Service not found

- **WHEN** DisableComputeService is called with a hostname that has no nova-compute service
- **THEN** client returns an error indicating the service was not found

### Requirement: Enable compute service

The client SHALL enable the nova-compute service on a given host by resolving the service ID and updating its status to enabled.

#### Scenario: Successful enable

- **WHEN** EnableComputeService is called with a valid hostname
- **THEN** client resolves the service ID, sends PUT with status=enabled, returns nil

#### Scenario: Already enabled

- **WHEN** EnableComputeService is called on an already-enabled service
- **THEN** client returns nil (idempotent)

### Requirement: Keystone v3 authentication

The client SHALL authenticate with Keystone v3 using username, password, project name, and domain configuration. Token lifecycle (caching and renewal) MUST be handled automatically by Gophercloud.

#### Scenario: Successful authentication

- **WHEN** the client is constructed with valid credentials
- **THEN** subsequent API calls succeed with a valid token

#### Scenario: Invalid credentials

- **WHEN** the client is constructed with invalid username or password
- **THEN** the first API call returns an authentication error

#### Scenario: Token auto-renewal

- **WHEN** the cached token expires
- **THEN** Gophercloud transparently re-authenticates and the API call succeeds

### Requirement: Error context

All errors returned by the client SHALL include context about which operation failed and which hostname was targeted, wrapping the underlying Gophercloud/HTTP error.

#### Scenario: Wrapped error

- **WHEN** an API call fails
- **THEN** the error message includes the operation name (e.g., "disabling compute service") and the hostname
