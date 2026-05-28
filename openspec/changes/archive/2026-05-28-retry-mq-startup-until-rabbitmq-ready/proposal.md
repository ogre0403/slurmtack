## Why

When `make up` starts the stack, `slurmtack` can begin before RabbitMQ is ready to accept AMQP connections. The daemon currently attempts MQ startup once, logs `mq.connect_failed` with `continuing_without_mq=true`, and never starts the consumer for that process. MQ-dependent executions then stall until an operator retries manually or restarts the container.

## What Changes

- Change MQ-enabled startup behavior so the daemon keeps recovering until RabbitMQ becomes available instead of permanently disabling MQ after the first failed connection attempt
- Ensure topology declaration and consumer startup happen automatically once the broker becomes reachable
- Make startup logging reflect that MQ is temporarily unavailable and being retried, rather than silently running forever without MQ consumption
- Add automated coverage for delayed RabbitMQ availability during daemon startup

## Capabilities

### New Capabilities

- (none)

### Modified Capabilities

- `amqp-events`: change daemon startup requirements so an instance configured with `AMQP_URL` eventually establishes MQ connectivity and begins consuming events after RabbitMQ becomes ready, without requiring a manual restart

## Impact

- Modified packages: `cmd/`, `internal/mq/`
- Likely test updates in `internal/mq/` and `cmd/`
- No REST API contract changes
- Improves local and staging behavior when RabbitMQ startup lags behind the daemon