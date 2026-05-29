## 1. Request Contract and Submission Flow

- [x] 1.1 Update `openstack_to_slurm` validation so `POST /v1/switches` requires `node_name` and no longer treats it as an error.
- [x] 1.2 Extend the switch service publisher abstraction to publish `execution.node_selected` with the persisted `execution_id` and requested `node_name`.
- [x] 1.3 Keep execution persistence before publish and add logging/tests for the `openstack_to_slurm` publish-failure path.

## 2. MQ Publisher and Wiring

- [x] 2.1 Add `PublishNodeSelected` to the MQ publisher implementation using the existing exchange, routing key, and event schema.
- [x] 2.2 Update daemon startup wiring and any test doubles so the service can publish `execution.requested` for `slurm_to_openstack` and `execution.node_selected` for `openstack_to_slurm`.

## 3. Verification

- [x] 3.1 Update API and service tests to cover accepted `openstack_to_slurm` requests with `node_name` and rejected requests without it.
- [x] 3.2 Add or update MQ/integration coverage proving an API-created `openstack_to_slurm` execution can be admitted through the published `execution.node_selected` event without a manual RMQ client step.
- [x] 3.3 Run the relevant OpenSpec validation and targeted Go tests for API, service, MQ, and integration slices.
