## 1. Enforce the request contract

- [x] 1.1 Update the API/service validation so `POST /v1/switches` rejects `node_name` when `direction=slurm_to_openstack`.
- [x] 1.2 Ensure accepted `slurm_to_openstack` executions are created without persisting a request-time `node_name`.

## 2. Update automated coverage

- [x] 2.1 Replace API/service tests that currently create `slurm_to_openstack` executions with request-time `node_name`, and add coverage for the new HTTP 400 rejection.
- [x] 2.2 Adjust status/list test fixtures so node-bound assertions use valid setup paths instead of relying on invalid `slurm_to_openstack` request payloads.

## 3. Align docs and examples

- [x] 3.1 Update `README.md` and nearby workflow docs to remove `node_name` from `slurm_to_openstack` request examples and field descriptions.
- [x] 3.2 Document that `node_name` becomes authoritative for `slurm_to_openstack` only after the placeholder allocation event binds the execution.
