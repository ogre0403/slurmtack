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

接著把 SIF 複製到每個 workload user 的 home 目錄下相同的相對路徑，例如 `/home/alice/slurmtack/build/output/placeholder-agent.sif`。設定 `SLURM_SIF_PATH` 為該 home-relative 目錄（如 `slurmtack/build/output`），`SLURM_SIF_FILE` 為預設檔名（如 `placeholder-agent.sif`）。每次 request 可透過 `placeholder_sif_file` 欄位覆蓋檔名。

> **Migration note:** `PLACEHOLDER_SIF_PATH` and `PLACEHOLDER_SIF_FILE` have been renamed to `SLURM_SIF_PATH` and `SLURM_SIF_FILE`. Rename these two variables in your deployment configuration before restarting the daemon.


### Available Environment Variables

目前程式會讀取這些設定:

1. Daemon 本身：
- `LISTEN_ADDR`
- `DB_PATH`: 是 daemon 目前行環境中的路徑，用來存放DB資料；若daemon 跑在容器內，對應到已掛載進容器的路徑。
- `JWT_SIGNING_KEY`（選填）: 用於簽發 Web Session JWT。若未設定，daemon 啟動時會自動產生隨機 key（重啟後現有 session 會失效）。

2. MQ相關：
- `AMQP_URL`

3. SLurm 相關：
- `SLURM_API_URL`
- `SLURM_JWT_TOKEN`: 工作負載 JWT（job submit / cancel / node read）。不再是啟動必要條件；若未設定，每個 `slurm_to_openstack` request 必須自帶 `slurm_user` + `slurm_user_token`。
- `SLURM_API_USER`: 送出 job 的 Slurm 使用者（預設 `cloud-user`）
- `SLURM_ADMIN_USER`: drain/resume 操作使用的管理員帳號（預設同 `SLURM_API_USER`）
- `SLURM_ADMIN_JWT_TOKEN`: 管理員操作使用的 JWT（預設同 `SLURM_JWT_TOKEN`）
- `SLURM_SIF_PATH`: Home-relative 目錄路徑（不能是絕對路徑或含 `..`）。Runtime 解析為 `/home/<workload-user>/<SLURM_SIF_PATH>/<effective-file>`。
- `SLURM_SIF_FILE`: 預設 SIF 檔名。若 request 未提供 `placeholder_sif_file` 則使用此值。

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

A machine-readable OpenAPI 2.0 spec is generated from code annotations. To regenerate after changing handlers:

```bash
make swagger
```

Output is committed to `docs/swagger/swagger.json` and `docs/swagger/swagger.yaml`.

### Create a switch from Slurm to OpenStack

這種方向在 request 建立時通常還不知道實際會切哪一台節點，**`node_name` 不是 `slurm_to_openstack` 的有效 request 欄位。** 若 request body 中包含 `node_name`，API 會回傳 `HTTP 400`。`slurm_to_openstack` execution 的節點身分由 placeholder agent 的 `execution.allocation` 事件決定，而非由呼叫端指定。

可選欄位：

| 欄位 | 說明 |
|------|------|
| `slurm_partition` | 指定 Slurm partition（省略則使用預設） |
| `slurm_constraint` | 指定 Slurm constraint |
| `slurm_account` | 指定 Slurm account，placeholder job 的 `job.account` 會使用此值 |
| `slurm_user` | Request-scoped 工作負載 Slurm 使用者（必須與 `slurm_user_token` 一起提供） |
| `slurm_user_token` | Request-scoped 工作負載 JWT（必須與 `slurm_user` 一起提供） |

**Workload Identity 解析規則：**
1. 若 request 同時提供 `slurm_user` 和 `slurm_user_token`，則使用 request 提供的身分。
2. 否則使用 daemon 設定的 `SLURM_API_USER` / `SLURM_JWT_TOKEN`。
3. 若兩者都沒有，request 會回傳 HTTP 400。

**Placeholder Job 行為：**
- `current_working_directory`、`standard_output`、`standard_error` 使用 `/home/<effective-user>/`。
- 若有指定 `slurm_account`，placeholder job 的 `job.account` 會設為該值。
- Placeholder script 會 export 使用的 `SLURM_API_USER` 和 `SLURM_JWT_TOKEN`。

**認證流程：** 所有 protected `/v1/*` endpoint 需要 slurmtack-issued Web Session JWT。先透過 `POST /v1/auth/login` 用 Slurm token 換取 session token，再用該 token 呼叫 API。

```shell
# Step 1: 取得 session token（用 Slurm JWT 交換）
TOKEN=$(
curl -s -X POST http://localhost/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "slurm_user": "alice",
    "slurm_user_token": "'"$SLURM_JWT"'"
  }' | jq -r '.slurmtack_token'
)
```

基本範例（使用 daemon 預設身分）：

```shell
# Step 2: 用 session token 呼叫 API
EXEC_ID=$(
curl -X POST http://localhost/v1/switches \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "direction": "slurm_to_openstack",
    "requested_by": "opencode",
    "slurm_partition": "all"
  }' | jq -r '.execution_id'
)

curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost/v1/switches/$EXEC_ID" | jq .
```

使用 request-scoped credentials 和 account：

```shell
EXEC_ID=$(
curl -X POST http://localhost/v1/switches \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "direction": "slurm_to_openstack",
    "requested_by": "opencode",
    "slurm_partition": "gpu-maint",
    "slurm_account": "proj-123",
    "slurm_user": "alice",
    "slurm_user_token": "'"$SLURM_JWT"'"
  }' | jq -r '.execution_id'
)

curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost/v1/switches/$EXEC_ID" | jq .
```

GET response 會包含 `requested_slurm_account`（不會暴露 workload credentials）。


### Create a Switch from OpenStack to Slurm

對 `openstack_to_slurm` 而言，request body 必須帶 `node_name`。這一筆 execution 會先以 `awaiting_target_node` 建立，並先記錄 API 傳入的 `node_name`。

```shell
EXEC_ID=$(
  curl -s -X POST http://localhost/v1/switches \
    -H "Authorization: Bearer $TOKEN" \
    -H 'Content-Type: application/json' \
    -d '{
      "direction": "openstack_to_slurm",
      "requested_by": "manual-test",
      "node_name": "FUSION-03-worker-tf"
    }' | jq -r '.execution_id'
)

curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost/v1/switches/$EXEC_ID" | jq .
```



### Cancel Existing Switch

在 execution 進入非可逆的 host mutation 前，可以用 cancel endpoint 中止等待中的 execution。只有下列 wait state 可以取消:

- `awaiting_target_node`
- `awaiting_source_allocation`
- `source_quiescing`

其他 active state（如 `rebooting`、`verifying`）會回傳 `HTTP 409`，表示該狀態不在安全取消範圍內。成功取消後，execution 會進入 `cancelling`，orchestrator 執行對應的 cleanup 後再進入 `cancelled`


```shell
curl -X POST http://localhost/v1/switches/$EXEC_ID/cancel -H "Authorization: Bearer $TOKEN"
```

### List all switch

```shell
curl http://localhost/v1/switches -H "Authorization: Bearer $TOKEN"
```


### Health Check
```shell
curl http://localhost/api/health
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


## Troubleshooting

1. slurmtack 的 log 出現 `lease already held by another execution`

  ```json
  {
    "time":"2026-06-05T00:04:56.287044308Z",
    "level":"WARN",
    "msg":"execution.failed",
    "execution_id":"f6b4268f5af48fb35db8db73661736af",
    "failure_class":"precheck_blocked",
    "terminal_state":"failed_non_destructive",
    "error_code":"step_error",
    "error_summary":"lease already held by another execution"
  }
  ```
  
  這個錯誤是由於前一次的執行失敗，但並未正常釋放其持有的節點租約（lease），導致新的執行因租約被佔用而失敗。

  * 清除特定節點的租約：
  ```shell
  sqlite3 ./docker/slurmtack.db "DELETE FROM leases WHERE node_name = 'FUSION-03-worker-tf';"
  ```
  * 清除所有節點的租約：
  ```shell
  sqlite3 ./docker/slurmtack.db "DELETE FROM leases;"
  ```