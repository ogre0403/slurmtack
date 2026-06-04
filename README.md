# slurmtack

## Quick Start

透過 docker-compose 啟動服務
```bash
cp docker/.env.example docker/.env
# 視環境調整 docker/.env
make up
```

### Build PlaceHolder Job SIF

```bash
cd build
sudo bash ./build-placeholder-agent.sh
```

接著把 SIF 複製到所有 GPU 節點都看得到的 shared path，且`PLACEHOLDER_SIF_PATH` 要設成這個shared path.


### Available Environment Variables

目前程式會讀取這些設定:

1. Daemon 本身：
- `API_TOKEN`
- `LISTEN_ADDR`
- `DB_PATH`: 是 daemon 目前行環境中的路徑，用來存放DB資料；若daemon 跑在容器內，對應到已掛載進容器的路徑。

2. MQ相關：
- `AMQP_URL`

3. SLurm 相關：
- `SLURM_API_URL`
- `SLURM_JWT_TOKEN`: 工作負載 JWT（job submit / cancel / node read）
- `SLURM_API_USER`: 送出 job 的 Slurm 使用者（預設 `cloud-user`）
- `SLURM_ADMIN_USER`: drain/resume 操作使用的管理員帳號（預設同 `SLURM_API_USER`）
- `SLURM_ADMIN_JWT_TOKEN`: 管理員操作使用的 JWT（預設同 `SLURM_JWT_TOKEN`）
- `PLACEHOLDER_SIF_PATH`: 是Slurm Cluster的shared Storage Path，Singularity會使用的SIF檔路徑。 

4. OpenStack 相關：
- `OS_AUTH_URL`
- `OS_PROJECT_NAME`
- `OS_USERNAME`
- `OS_PASSWORD`
- `OS_USER_DOMAIN_NAME`
- `OS_PROJECT_DOMAIN_NAME`


5. SSH 相關：
- `SSH_USER`
- `SSH_PORT`
- `SSH_OPTIONS`
- `SSH_PRIVATE_KEY_PATH`: 是 daemon 目前執行環境中可讀的私鑰路徑。如果 daemon 跑在容器內，對應到已掛載進容器的可讀私鑰檔。
- `SSH_POLL_INTERVAL`
- `SSH_POLL_TIMEOUT`

## API Reference

### Create a switch from Slurm to OpenStack

這種方向在 request 建立時通常還不知道實際會切哪一台節點，**`node_name` 不是 `slurm_to_openstack` 的有效 request 欄位。** 若 request body 中包含 `node_name`，API 會回傳 `HTTP 400`。`slurm_to_openstack` execution 的節點身分由 placeholder agent 的 `execution.allocation` 事件決定，而非由呼叫端指定。

要指定Partition的話，在BODY裡加上`"slurm_partition": "<PARTITION>"`，`slurm_partition` 是可選欄位；若省略，daemon 會維持目前行為，讓 Slurm 使用預設 partition 選擇。


```shell
EXEC_ID=$(
curl -X POST http://localhost:8080/v1/switches \
  -H "Authorization: Bearer changeme" \
  -H "Content-Type: application/json" \
  -d '{
    "direction": "slurm_to_openstack",
    "requested_by": "opencode",
    "slurm_partition": "all"
  }' | jq -r '.execution_id'
)

curl -s -H 'Authorization: Bearer changeme' \
  "http://127.0.0.1:8080/v1/switches/$EXEC_ID" | jq .
```


### Create a Switch from OpenStack to Slurm

對 `openstack_to_slurm` 而言，request body 必須帶 `node_name`。這一筆 execution 會先以 `awaiting_target_node` 建立，並先記錄 API 傳入的 `node_name`。

```shell
EXEC_ID=$(
  curl -s -X POST http://127.0.0.1:8080/v1/switches \
    -H 'Authorization: Bearer changeme' \
    -H 'Content-Type: application/json' \
    -d '{
      "direction": "openstack_to_slurm",
      "requested_by": "manual-test",
      "node_name": "FUSION-03-worker-tf"
    }' | jq -r '.execution_id'
)

curl -s -H 'Authorization: Bearer changeme' \
  "http://127.0.0.1:8080/v1/switches/$EXEC_ID" | jq .

```



### Cancel Existing Switch

在 execution 進入非可逆的 host mutation 前，可以用 cancel endpoint 中止等待中的 execution。只有下列 wait state 可以取消:

- `awaiting_target_node`
- `awaiting_source_allocation`
- `source_quiescing`

其他 active state（如 `rebooting`、`verifying`）會回傳 `HTTP 409`，表示該狀態不在安全取消範圍內。成功取消後，execution 會進入 `cancelling`，orchestrator 執行對應的 cleanup 後再進入 `cancelled`


```shell
curl -X POST http://localhost:8080/v1/switches/$EXEC_ID/cancel -H "Authorization: Bearer changeme"
```

### List all switch

```shell
curl http://localhost:8080/v1/switches -H "Authorization: Bearer changeme"
```


### Health Check
```shell
curl http://127.0.0.1:8080/health
```


## Slurm/OpenStack Commands Reference

```shell

sudo scontrol update NodeName=FUSION-03-worker-tf State=DRAIN reason=aa

sudo scontrol update NodeName=FUSION-03-worker-tf State=RESUME
```


```shell
openstack compute service set --enable FUSION-04-worker-tf nova-compute

openstack compute service set --disable FUSION-04-worker-tf nova-compute
```

jq 格式化log

```shell
docker logs docker-slurmtack-1 -f 2>&1 | jq .
```
