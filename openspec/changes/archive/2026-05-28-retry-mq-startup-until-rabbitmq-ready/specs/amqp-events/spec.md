## ADDED Requirements

### Requirement: Startup connection recovery

When `AMQP_URL` is configured and RabbitMQ is not yet ready during daemon startup, the system SHALL keep retrying MQ activation with bounded exponential backoff until MQ becomes available or the daemon shuts down. MQ activation MUST include connection establishment, topology declaration, and consumer startup.

#### Scenario: RabbitMQ becomes ready after the daemon starts
- **WHEN** the daemon starts with `AMQP_URL` configured and the first MQ activation attempt fails because RabbitMQ is not yet ready
- **THEN** the daemon keeps retrying MQ activation, starts consuming from `gpu-switch.allocation` and `gpu-switch.drained` once RabbitMQ becomes available, and does not require a manual process restart

#### Scenario: Shutdown interrupts startup retry
- **WHEN** the daemon is retrying MQ activation during startup and receives shutdown
- **THEN** the daemon stops further retry attempts and exits cleanly without leaving a partial MQ startup loop running

#### Scenario: MQ remains optional when not configured
- **WHEN** the daemon starts without `AMQP_URL`
- **THEN** the daemon does not enter the MQ startup retry loop and continues running without MQ integration