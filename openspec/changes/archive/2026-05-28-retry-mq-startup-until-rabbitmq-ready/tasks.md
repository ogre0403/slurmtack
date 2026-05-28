## 1. MQ startup supervision

- [x] 1.1 Refactor `internal/mq.Connection` so initial startup can reuse the existing exponential-backoff connection retry behavior instead of doing a one-shot dial in `cmd/main.go`
- [x] 1.2 Replace the one-shot MQ startup branch in `cmd/main.go` with a supervisor that retries connection, topology declaration, and consumer startup until success or shutdown
- [x] 1.3 Ensure partial MQ startup failures close any opened channel or connection before the next retry attempt

## 2. Lifecycle and observability

- [x] 2.1 Update MQ startup logging to distinguish `AMQP_URL` being unset from MQ being configured but temporarily unavailable
- [x] 2.2 Log successful MQ activation after a retry and stop the startup retry loop cleanly on context cancellation

## 3. Verification

- [x] 3.1 Add focused tests for the startup retry path in `internal/mq`, including eventual success after initial failures
- [x] 3.2 Add daemon-level coverage that a process started before RabbitMQ readiness eventually starts MQ consumption without a manual restart
- [x] 3.3 Add coverage that leaving `AMQP_URL` unset does not start the retry loop and still allows the daemon to run without MQ