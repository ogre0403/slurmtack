# slurmtack

slurmtack 是一個用來協調 GPU 節點在 Slurm 與 OpenStack 間切換的 daemon。依照目前實作，主程式會啟動:

- HTTP API
- SQLite state store
- 背景 orchestrator
- 可選的 RabbitMQ consumer

## Quick Start

### 方式一: 本地啟動

需求:

- Go 1.22.4
- C toolchain 與 SQLite 編譯所需套件

建議直接從 repo 根目錄的環境變數範本開始:

```bash
cp .env.example .env
# 視環境調整 .env

set -a
. ./.env
set +a

go run ./cmd
```

啟動後，服務會監聽 `http://127.0.0.1:8080`。

### 方式二: Docker Compose

Compose 仍使用 `docker/.env`，最簡單的方式是從新的根目錄範本複製一份，再把 `DB_PATH` 改成容器掛載路徑。

```bash
cp .env.example docker/.env
# 將 docker/.env 內的 DB_PATH 改成 /data/slurmtack.db
make up
```

預設會:

- 啟動 `slurmtack`
- 啟動 `rabbitmq`
- 將 SQLite 資料檔掛載到 `/data/slurmtack.db`

停止服務:

```bash
make down
```

## 驗證服務

### 1. Health check

```bash
curl http://127.0.0.1:8080/health
```

預期回應:

```json
{"status":"ok"}
```

### 2. 建立一筆 switch request

目前 API 需要 Bearer token。

```bash
curl -X POST http://127.0.0.1:8080/v1/switches \
	-H 'Authorization: Bearer dev-token' \
	-H 'Content-Type: application/json' \
	-d '{
		"direction": "openstack_to_slurm",
		"node_name": "gpu-01",
		"requested_by": "local-dev"
	}'
```

預期會拿到 `202 Accepted`，並回傳一個 `execution_id`:

```json
{
	"execution_id": "<execution-id>",
	"status_url": "/v1/switches/<execution-id>"
}
```

### 3. 查詢 execution 狀態

```bash
curl http://127.0.0.1:8080/v1/switches/<execution-id> \
	-H 'Authorization: Bearer dev-token'
```

也可以列出所有 execution:

```bash
curl http://127.0.0.1:8080/v1/switches \
	-H 'Authorization: Bearer dev-token'
```

## 可選環境變數

本地或 staging 啟動時，優先使用 [/.env.example](/workspaces/slurmtack/.env.example)。如果只想知道有哪些整合入口，目前程式會讀取這些設定:

- `AMQP_URL`
- `SLURM_API_URL`
- `SLURM_JWT_TOKEN` — 工作負載 JWT（job submit / cancel / node read）
- `SLURM_API_USER` — 送出 job 的 Slurm 使用者（預設 `cloud-user`）
- `SLURM_ADMIN_USER` — drain/resume 操作使用的管理員帳號（預設同 `SLURM_API_USER`）
- `SLURM_ADMIN_JWT_TOKEN` — 管理員操作使用的 JWT（預設同 `SLURM_JWT_TOKEN`）
- `OS_AUTH_URL`
- `OS_PROJECT_NAME`
- `OS_USERNAME`
- `OS_PASSWORD`
- `SSH_POLL_INTERVAL`
- `SSH_POLL_TIMEOUT`
- `PLACEHOLDER_SIF_PATH`

## RabbitMQ + placeholder agent: Slurm-to-OpenStack 流程

這一段描述的是設計文件與現有元件所對應的完整執行路徑，也就是 `slurm_to_openstack` request 如何透過 RabbitMQ 與 placeholder agent 把 execution 從 `requested` 推進到 `source_detached`，再交給 orchestrator 繼續後續 attach 與 verify。

### 前置條件

- RabbitMQ 可從 daemon 與 GPU 節點連線
- `slurmrestd` 已啟用 JWT 驗證，且 daemon 與 placeholder agent 都能連線
- OpenStack API 可連線
- placeholder agent 的 SIF 已放在所有 GPU 節點可讀的 shared filesystem
- GPU 節點上的 `singularity run <PLACEHOLDER_SIF_PATH>` 可以啟動 agent

### 1. 建置並部署 placeholder agent

先在控制端建置 binary 與 SIF:

```bash
./build/build-placeholder-agent.sh
```

如果機器上有 `singularity`，腳本會產生:

- `build/output/placeholder-agent`
- `build/output/placeholder-agent.sif`

接著把 SIF 複製到所有 GPU 節點都看得到的 shared path，例如:

```bash
cp build/output/placeholder-agent.sif /shared/images/placeholder-agent.sif
```

### 2. 啟動 daemon，並打開 MQ / Slurm / OpenStack 整合設定

建議直接從 [/.env.example](/workspaces/slurmtack/.env.example) 複製，再按你的 staging 環境覆蓋值:

```bash
cp .env.example .env
# 視你的環境修改 .env

set -a
. ./.env
set +a
```

然後啟動 RabbitMQ 與 daemon:

```bash
make up
```

當 `AMQP_URL` 有設定時，daemon 啟動時會自動宣告 MQ topology:

- exchange: `gpu-switch.events`
- queue: `gpu-switch.allocation`
- queue: `gpu-switch.drained`

### 3. 建立 `slurm_to_openstack` execution

這種方向在 request 建立時通常還不知道實際會切哪一台節點，所以用 `slurm_constraint` 讓 Slurm 選一台符合條件的 GPU node。

```bash
curl -X POST http://127.0.0.1:8080/v1/switches \
	-H 'Authorization: Bearer dev-token' \
	-H 'Content-Type: application/json' \
	-d '{
		"direction": "slurm_to_openstack",
		"requested_by": "staging-operator",
		"slurm_constraint": "gpu-a100"
	}'
```

建立後的預期狀態序列如下:

1. API 建立 execution，初始 state 是 `requested`
2. orchestrator 呼叫 `SubmitPlaceholderJob`
3. execution 進入 `awaiting_source_allocation`

### 4. Slurm 提交 placeholder job，並在分配到節點後啟動 agent

依照目前 `SubmitPlaceholderJob` 的實作，daemon 會送出一個類似下面的 Slurm job:

```bash
sbatch \
	--job-name=gpu-switch-<execution_id> \
	--nodes=1 \
	--exclusive \
	--constraint=<slurm_constraint> \
	--export=EXECUTION_ID=<execution_id>,AMQP_URL=<amqp_url>,SLURM_API_URL=<slurm_api_url>,SLURM_JWT_TOKEN=<slurm_jwt_token> \
	--wrap="singularity run /shared/images/placeholder-agent.sif"
```

當 job 真正落到某台 GPU node 後，placeholder agent 會:

1. 讀取 `EXECUTION_ID`、`AMQP_URL`、`SLURM_API_URL`、`SLURM_JWT_TOKEN`
2. 取得本機 hostname，並去掉網域尾碼
3. 往 `gpu-switch.events` 發送 routing key `execution.allocation`

訊息內容如下:

```json
{
	"execution_id": "<execution-id>",
	"job_id": "<slurm-job-id>",
	"node_name": "<allocated-node>"
}
```

MQ consumer 收到後，會把 execution 從 `awaiting_source_allocation` 推進到 `node_identified`。

### 5. daemon 對已識別的節點執行 quiesce

當 execution 進入 `node_identified` 之後，orchestrator 會繼續執行:

1. acquire node lease
2. 進入 `locked`
3. 做 precheck，確認 OpenStack compute service 可查詢
4. 進入 `precheck_passed`
5. 呼叫 Slurm `DrainNode`
6. 進入 `source_quiescing`

### 6. placeholder agent 等待 drain 完成並回報 MQ

placeholder agent 會持續輪詢 `slurmrestd` 的 node state。當 state 變成 `drained`、`drained*`、`down` 或 `down*` 之後，會再發一筆 MQ 訊息:

routing key:

```text
execution.drained
```

body:

```json
{
	"execution_id": "<execution-id>",
	"node_name": "<allocated-node>"
}
```

MQ consumer 收到後，會把 execution 從 `source_quiescing` 推進到 `source_detached`。

### 7. `source_detached` 之後的後半段

從 `source_detached` 開始，後面的流程由 orchestrator 繼續驅動:

1. `host_reconfiguring`
2. `rebooting`
3. SSH reachability poll 成功後進入 `host_reachable`
4. 啟用 OpenStack compute service，進入 `target_attaching`
5. 進入 `verifying`
6. 成功時進入 `completed`

### 8. 觀察與除錯重點

你至少要同時看三個面向:

- daemon log: 看 execution 是否卡在 `awaiting_source_allocation` 或 `source_quiescing`
- RabbitMQ: 確認 `gpu-switch.allocation` / `gpu-switch.drained` 有訊息被消費
- Slurm job: 確認 placeholder job 是否成功啟動、是否拿到 node、是否非零退出

若 execution 卡住，常見對應如下:

- 卡在 `awaiting_source_allocation`: placeholder job 沒有拿到資源，或 agent 沒成功發 `execution.allocation`
- 卡在 `source_quiescing`: Slurm drain 沒完成，或 agent 沒成功發 `execution.drained`
- 進入 failed state: 代表 orchestrator 在 precheck、attach、reboot 或 verify 階段遇到外部依賴錯誤

### 9. 目前 repo 的接線狀態

這個流程所依賴的元件都已存在於 repo 中，而且 [cmd/main.go](/workspaces/slurmtack/cmd/main.go) 已經會依環境變數建立 `slurm.Client`、`openstack.Client` 與 `remote.Runner`。因此:

- API + SQLite + MQ consumer 可以直接跑
- RabbitMQ topology 可以直接建立
- placeholder agent 可以獨立建置與執行
- 只要提供正確的 Slurm、OpenStack、SSH 與 RabbitMQ 設定，orchestrator 就會直接使用這些 client 推進流程

如果你要先驗證狀態機與步驟順序，現有 repo 內最接近完整流程的參考是 [internal/engine/integration/switch_test.go](/workspaces/slurmtack/internal/engine/integration/switch_test.go)。

## 目前實作範圍

依照目前程式碼，README 這份文件分成兩層:

- quick start: 本地啟動與 API 驗證
- advanced flow: RabbitMQ + placeholder agent 的 Slurm-to-OpenStack 執行路徑

目前主程式已完成 Slurm、OpenStack、SSH runner 的基本接線。要真正跑完整切換，重點已經不是補 wiring，而是提供可用的外部端點、認證、shared SIF 路徑與 SSH 連線條件。

---

## Trace Logging（除錯指南）

Daemon 啟動時會把所有 log 以 JSON 格式寫到 stderr，預設 level 為 `Info`。每一個 log entry 都帶有結構化的 key/value，方便直接用 `jq` 篩選。

### 標準欄位

| 欄位 | 說明 |
|---|---|
| `execution_id` | 本次切換的唯一 ID |
| `direction` | `slurm_to_openstack` 或 `openstack_to_slurm` |
| `current_state` | 觸發這個 log 時的狀態機狀態 |
| `state_version` | 當時的 optimistic-lock 版本號 |
| `node_name` | GPU node hostname（若已知） |
| `action` | orchestrator 正在執行的 action 名稱 |
| `step_name` | engine step handler 的名稱 |

### 事件詞彙

| `msg` 值 | 觸發時機 |
|---|---|
| `request.accepted` | API 接到切換請求並建立 execution |
| `action.selected` | orchestrator tick 選定要執行的 action |
| `action.succeeded` | action 成功完成（如 acquire_lease） |
| `action.failed` | action 執行失敗，即將進入 failed state |
| `transition.requested` | runner 嘗試推進狀態機 |
| `transition.succeeded` | 狀態機推進成功 |
| `transition.failed` | 狀態機推進失敗（無效轉換或 store error） |
| `wait.entered` | 進入非同步等待（allocation / drained / ssh） |
| `wait.progress` | 等待中收到一個事件或輪詢一次 |
| `wait.satisfied` | 等待條件達成，可繼續推進 |
| `wait.timeout` | 等待逾時（目前只有 SSH reachability） |
| `step.started` | RunStep 開始執行某個 step handler |
| `step.succeeded` | step handler 成功回傳 |
| `step.failed` | step handler 回傳錯誤 |
| `execution.completed` | execution 成功走到 `completed` 狀態 |
| `execution.failed` | execution 進入任一 failed terminal 狀態 |

### 常用 jq 篩選範例

追蹤單一 execution 的完整生命週期：

```bash
journalctl -u slurmtack -o cat | jq -c 'select(.execution_id == "YOUR_EXEC_ID")'
```

只看 warning 以上（失敗或異常事件）：

```bash
journalctl -u slurmtack -o cat | jq -c 'select(.level == "WARN" or .level == "ERROR")'
```

查看所有 action 選擇紀錄（了解 orchestrator 決策路徑）：

```bash
journalctl -u slurmtack -o cat | jq -c 'select(.msg == "action.selected")'
```

查看某個 node 的所有等待進入與滿足事件：

```bash
journalctl -u slurmtack -o cat | \
  jq -c 'select(.node_name == "gpu-01") | select(.msg | startswith("wait."))'
```

追蹤 SSH reachability polling（Debug level，需將 daemon 的 slog level 調為 Debug）：

```bash
# 在 cmd/main.go 將 slog.LevelInfo 改為 slog.LevelDebug 後重啟
journalctl -u slurmtack -o cat | jq -c 'select(.msg == "wait.progress")'
```

### 排查 execution 卡住

1. 先確認 execution 的當前狀態：`GET /api/v1/switch/{id}`
2. 用 `jq` 過濾該 `execution_id` 的所有 log
3. 找最後一筆 `action.selected` 看 orchestrator 最後選了什麼
4. 若卡在 wait state，確認對應的 MQ 訊息（`wait.progress` → `wait.satisfied` 鏈）是否出現
5. 若出現 `action.failed` 或 `step.failed`，log 的 `error` 欄位會說明原因
