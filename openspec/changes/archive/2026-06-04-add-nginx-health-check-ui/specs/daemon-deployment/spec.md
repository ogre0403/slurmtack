## MODIFIED Requirements

### Requirement: Docker Compose stack

The docker-compose.yaml SHALL define at minimum three services: `nginx`, `slurmtack`, and `rabbitmq`. The services MUST communicate over a user-defined bridge network instead of `network_mode: host`. `nginx` MUST publish the stack's browser-facing HTTP port to the host and serve as the only public entrypoint for the validation page and proxied health API. `slurmtack` MUST NOT publish any host port and MUST be reachable only from the compose network, where nginx proxies `GET /api/health` to the daemon's internal `GET /health` endpoint. `rabbitmq` MUST remain available on host ports 5672 and 15672 while also participating in the compose network for service-to-service traffic.

#### Scenario: Stack starts with nginx as public entrypoint

- **WHEN** `docker-compose up` is run on the staging host
- **THEN** `nginx`, `slurmtack`, and `rabbitmq` all start successfully
- **AND** the validation page is reachable through nginx on the published HTTP port
- **AND** `GET /api/health` succeeds through nginx without requiring direct host access to slurmtack

#### Scenario: Slurmtack is not directly exposed to the host

- **WHEN** the stack is running
- **THEN** the compose definition exposes no host port mapping for the `slurmtack` service
- **AND** browser or host access must go through nginx to reach the daemon HTTP surface

#### Scenario: RabbitMQ remains externally reachable

- **WHEN** the stack is running
- **THEN** RabbitMQ management UI is accessible on port 15672 and AMQP is accessible on port 5672
