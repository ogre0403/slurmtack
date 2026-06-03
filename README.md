# slurmtack

slurmtack 是一個用來協調 GPU 節點在 Slurm 與 OpenStack 間切換的 daemon。依照目前實作，主程式會啟動:

- HTTP API
- SQLite state store
- MQ-driven orchestrator worker
- RabbitMQ publisher / consumer（`openstack_to_slurm` admission 需要）

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

若你要啟用 reboot 後的 SSH reachability，請另外取消註解 `SSH_USER` / `SSH_PORT` / `SSH_OPTIONS`，並設定 `SSH_PRIVATE_KEY_PATH` 指向 daemon 所在主機上可讀的私鑰檔案。只要任一 SSH runner 變數被設定，`SSH_PRIVATE_KEY_PATH` 就會變成必填。

啟動後，服務會監聽 `http://127.0.0.1:8080`。

### 方式二: Docker Compose

Compose 使用 `docker/.env`，建議直接從 compose 專用範本開始。

```bash
cp docker/.env.example docker/.env
# 視環境調整 docker/.env
make up
```

`docker/.env.example` 已經預先把 `DB_PATH` 設成 `/data/slurmtack.db`，而且 `PLACEHOLDER_SIF_PATH` 的建議值也對應 `docker-compose` 內建的 `/data/placeholder-agent.sif` 掛載點。

如果 daemon 跑在容器內且要啟用 SSH runner，`SSH_PRIVATE_KEY_PATH` 必須填容器內路徑，並且那個路徑要對應到已掛載進容器的可讀私鑰檔。

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
		"requested_by": "local-dev",
		"node_name": "gpu-01"
	}'
```

預期會拿到 `202 Accepted`，並回傳一個 `execution_id`:

```json
{
	"execution_id": "<execution-id>",
	"status_url": "/v1/switches/<execution-id>"
}
```

對 `openstack_to_slurm` 而言，request body 必須帶 `node_name`。這一筆 execution 會先以 `awaiting_target_node` 建立，並先記錄 API 傳入的 `node_name`。這代表 request 已被接受，也已經進入 MQ admission path，但 daemon 還不會 acquire lease、做 precheck，或開始任何 host-level mutation；它會先等自己 publish 的節點綁定事件被 consumer 處理完。

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

### 4. `openstack_to_slurm` 的 admission 流程

建立 request 後，daemon 會自動往 `gpu-switch.events` 發送兩筆 admission 事件。第一筆是 routing key `execution.requested`:

```json
{
	"execution_id": "<execution-id>",
	"direction": "openstack_to_slurm"
}
```

第二筆是 routing key `execution.node_selected`，內容直接使用同一個 API request 內的 `node_name`:

```json
{
	"execution_id": "<execution-id>",
	"node_name": "gpu-01"
}
```

`awaiting_target_node` 的意思是: execution 已經被持久化，也已經進入 MQ admission path，但還沒完成 node-selection event 的 consume 與 admission。只要還停在這個 state，daemon 就不會 acquire lease，也不會做 OpenStack/SSH/Slurm 動作。

這個流程不需要人工再對 RabbitMQ 發送任何訊息。`execution.node_selected` 被 consumer 收到後，daemon 會:

- 把 `node_name` 綁到 execution
- 將 state 從 `awaiting_target_node` 推進到 `node_identified`
- 以 MQ-driven worker 繼續既有的 lease / precheck / quiesce / reboot / attach / verify 流程

## 可選環境變數

本地啟動建議使用 [/.env.example](/workspaces/slurmtack/.env.example)，docker-compose 則建議使用 [/docker/.env.example](/workspaces/slurmtack/docker/.env.example)。如果只想知道有哪些整合入口，目前程式會讀取這些設定:

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
- `SSH_USER`
- `SSH_PORT`
- `SSH_OPTIONS`
- `SSH_PRIVATE_KEY_PATH` — 只要設定任一 SSH runner 參數，這個值就必填，而且必須是 daemon 目前執行環境中可讀的私鑰路徑
- `SSH_POLL_INTERVAL`
- `SSH_POLL_TIMEOUT`
- `PLACEHOLDER_SIF_PATH`

若 `SSH_PRIVATE_KEY_PATH` 缺少、路徑不存在，或 daemon 無法讀取該檔案，程式會在 startup 階段直接退出，並回報明確的 `SSH_PRIVATE_KEY_PATH` 驗證錯誤，而不是等到 workflow 跑到 reboot/ssh poll 時才失敗。

## RabbitMQ + OpenStack-to-Slurm 啟動摘要

`openstack_to_slurm` 現在走 MQ-driven admission，操作上請直接記住這幾點:

- API request body 必須帶 `direction=openstack_to_slurm`、`requested_by` 與 `node_name`
- API 持久化 execution 後，會自動發 `execution.requested`
- API 持久化 execution 後，也會自動發對應的 `execution.node_selected`
- execution 會先停在 `awaiting_target_node`
- `awaiting_target_node` = request 已接受，但 daemon 還在等自己 publish 的 node-selection event 被 consume；此時不會碰 lease、不會碰 host
- 不需要人工再推任何 RabbitMQ 訊息
- `execution.node_selected` 被 consume 後，daemon 會綁定節點、轉到 `node_identified`，然後繼續下游 workflow

如果你在操作環境裡看見 execution 長時間停在 `awaiting_target_node`，第一個要檢查的是:

- daemon 是否有 `AMQP_URL`
- `gpu-switch.events` / `execution.node_selected` 是否真的送到 broker 並被 consumer 收到
- daemon log 裡是否出現 `request.node_selected_publish_failed`、`mq.malformed_message` 或 `mq.advance_failed`

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

如果你是本地直接 `go run ./cmd`，建議從 [/.env.example](/workspaces/slurmtack/.env.example) 複製；如果你是用 docker-compose，建議從 [/docker/.env.example](/workspaces/slurmtack/docker/.env.example) 複製。

```bash
cp docker/.env.example docker/.env
# 視你的環境修改 docker/.env

set -a
. ./docker/.env
set +a
```

如果你不是用 docker-compose，而是直接在主機上跑 daemon，則把上面的路徑改成 repo 根目錄的 `.env.example` / `.env` 即可。

然後啟動 RabbitMQ 與 daemon:

```bash
make up
```

當 `AMQP_URL` 有設定時，daemon 啟動時會自動宣告 MQ topology:

- exchange: `gpu-switch.events`
- queue: `gpu-switch.requested`
- queue: `gpu-switch.node-selected`
- queue: `gpu-switch.allocation`
- queue: `gpu-switch.drained`

對目前的 admission path 來說，四種 routing key 的角色分別是:

- `execution.requested`: API 建立 execution 後發出，讓 daemon admission 新工作
- `execution.node_selected`: API 在建立 `openstack_to_slurm` execution 後自動發出，讓 daemon 把 request 綁到具體節點
- `execution.allocation`: placeholder agent 回報 `slurm_to_openstack` 分配結果
- `execution.drained`: placeholder agent 回報 Slurm source node 已 drain 完成

### 3. 建立 `slurm_to_openstack` execution

這種方向在 request 建立時通常還不知道實際會切哪一台節點，所以可以用 `slurm_constraint` 讓 Slurm 選一台符合條件的 GPU node；如果叢集有多個 partition，也可以額外帶 `slurm_partition`，把 placeholder job 限制到指定 partition。

**`node_name` 不是 `slurm_to_openstack` 的有效 request 欄位。** 若 request body 中包含 `node_name`，API 會回傳 `HTTP 400`。`slurm_to_openstack` execution 的節點身分由 placeholder agent 的 `execution.allocation` 事件決定，而非由呼叫端指定（詳見步驟 4）。

```bash
curl -X POST http://127.0.0.1:8080/v1/switches \
	-H 'Authorization: Bearer dev-token' \
	-H 'Content-Type: application/json' \
	-d '{
		"direction": "slurm_to_openstack",
		"requested_by": "staging-operator",
		"slurm_constraint": "gpu-a100",
		"slurm_partition": "gpu-maint"
	}'
```

`slurm_partition` 是可選欄位；若省略，daemon 會維持目前行為，讓 Slurm 使用預設 partition 選擇。

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
	--partition=<slurm_partition> \
	--constraint=<slurm_constraint> \
	--export=EXECUTION_ID=<execution_id>,AMQP_URL=<amqp_url>,SLURM_API_URL=<slurm_api_url>,SLURM_JWT_TOKEN=<slurm_jwt_token> \
	--wrap="singularity run /shared/images/placeholder-agent.sif"
```

如果 request 沒有提供 `slurm_partition`，上面的 `--partition=<slurm_partition>` 這行就不會出現在實際送出的 job script 裡。

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

MQ consumer 收到後，會把 execution 從 `awaiting_source_allocation` 推進到 `node_identified`，並在此時將 `node_name` 綁定到 execution 記錄。這是 `slurm_to_openstack` execution 的 `node_name` 第一次變為 authoritative；在收到 `execution.allocation` 之前，execution 的 `node_name` 欄位會保持空白。

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

### 10. 手動 smoke test: 直接用 Slurm 送 placeholder job，並檢查 MQ 是否收到事件

如果你要測的只是「placeholder agent 能不能從 Slurm job 內成功 publish 到 RabbitMQ」，最簡單的方式是不要先走 API / execution 狀態機，而是直接手動 `sbatch`。這個 smoke test 只驗證 `placeholder-agent -> MQ` 這一段，所以 `EXECUTION_ID` 可以用任意測試字串。

前提:

- RabbitMQ 已啟動
- daemon 至少啟動過一次，讓 `gpu-switch.events` exchange 與既有 queue topology 已宣告完成
- `build/output/placeholder-agent.sif` 已建好，並放在 Slurm compute node 可讀的位置
- 送 job 的 shell 已有可用的 `SLURM_JWT_TOKEN`

先準備測試用環境變數:

```bash
export EXECUTION_ID="manual-placeholder-$(date +%Y%m%d%H%M%S)"
export PLACEHOLDER_SIF_PATH=/shared/images/placeholder-agent.sif
export AMQP_URL=amqp://guest:guest@127.0.0.1:5672/
export SLURM_API_URL=http://slurm-head:6820
export SLURM_JWT_TOKEN=replace-me
export SLURM_API_USER=cloud-user
```

因為 daemon 自己也會 consume `gpu-switch.allocation` / `gpu-switch.drained`，要避免你在查 queue 時剛好被 daemon 吃掉，建議先額外建立兩個只給 smoke test 用的 probe queue:

```bash
curl -u guest:guest -H 'content-type: application/json' \
	-X PUT http://127.0.0.1:15672/api/queues/%2F/probe.placeholder.allocation \
	-d '{"durable":false,"auto_delete":true,"arguments":{}}'

curl -u guest:guest -H 'content-type: application/json' \
	-X PUT http://127.0.0.1:15672/api/queues/%2F/probe.placeholder.drained \
	-d '{"durable":false,"auto_delete":true,"arguments":{}}'

curl -u guest:guest -H 'content-type: application/json' \
	-X POST http://127.0.0.1:15672/api/bindings/%2F/e/gpu-switch.events/q/probe.placeholder.allocation \
	-d '{"routing_key":"execution.allocation","arguments":{}}'

curl -u guest:guest -H 'content-type: application/json' \
	-X POST http://127.0.0.1:15672/api/bindings/%2F/e/gpu-switch.events/q/probe.placeholder.drained \
	-d '{"routing_key":"execution.drained","arguments":{}}'
```

接著直接送出 placeholder job:

```bash
sbatch \
	--job-name="placeholder-smoke-${EXECUTION_ID}" \
	--nodes=1 \
	--ntasks=1 \
	--exclusive=user \
	--constraint=gpu-a100 \
	--export=ALL,EXECUTION_ID="${EXECUTION_ID}",AMQP_URL="${AMQP_URL}",SLURM_API_URL="${SLURM_API_URL}",SLURM_JWT_TOKEN="${SLURM_JWT_TOKEN}",SLURM_API_USER="${SLURM_API_USER}",POLL_INTERVAL=5s \
	--wrap="singularity run ${PLACEHOLDER_SIF_PATH}"
```

送出後先看 job 是否真的進到某台節點:

```bash
squeue -n "placeholder-smoke-${EXECUTION_ID}" -o "%.18i %.9T %.40N"
```

當 job 進入 `R` 後，placeholder agent 一啟動就會先發 `execution.allocation`。你可以用 RabbitMQ management API 把 probe queue 裡的訊息抓出來:

```bash
curl -u guest:guest -H 'content-type: application/json' \
	-X POST http://127.0.0.1:15672/api/queues/%2F/probe.placeholder.allocation/get \
	-d '{"count":10,"ackmode":"ack_requeue_false","encoding":"auto","truncate":50000}'
```

預期至少會看到一筆 JSON，body 內包含:

```json
{
	"execution_id": "<your EXECUTION_ID>",
	"job_id": "<slurm job id>",
	"node_name": "<allocated node>"
}
```

如果你還要繼續驗證 `execution.drained`，先從上一筆 allocation 訊息或 `squeue` 記下 `node_name`，然後手動把該節點 drain:

```bash
scontrol update NodeName=<allocated-node> State=DRAIN Reason=placeholder-smoke-test
```

placeholder agent 輪詢到 node 變成 `drained` / `down` 後，會再 publish 一筆 `execution.drained`。查法如下:

```bash
curl -u guest:guest -H 'content-type: application/json' \
	-X POST http://127.0.0.1:15672/api/queues/%2F/probe.placeholder.drained/get \
	-d '{"count":10,"ackmode":"ack_requeue_false","encoding":"auto","truncate":50000}'
```

預期 body 會長這樣:

```json
{
	"execution_id": "<your EXECUTION_ID>",
	"node_name": "<allocated node>"
}
```

測完建議做 cleanup:

```bash
scancel <slurm-job-id>
scontrol update NodeName=<allocated-node> State=RESUME
curl -u guest:guest -X DELETE http://127.0.0.1:15672/api/queues/%2F/probe.placeholder.allocation
curl -u guest:guest -X DELETE http://127.0.0.1:15672/api/queues/%2F/probe.placeholder.drained
```

如果 allocation queue 沒有看到訊息，優先檢查:

- `singularity run ${PLACEHOLDER_SIF_PATH}` 是否真的能在 compute node 啟動
- job environment 內是否真的帶到了 `AMQP_URL` / `SLURM_API_URL` / `SLURM_JWT_TOKEN`
- RabbitMQ `guest:guest` 是否允許從該節點連線
- `gpu-switch.events` exchange 是否已經被 daemon 宣告過


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
