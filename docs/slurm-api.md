# SLURM API Example

本文件示範 `infra_fusion` 目前已建立完成的 Slurm REST API (`slurmrestd`) 用法。

- Slurm 版本：`24.11.5`
- REST API：`http://<headnode>:6820/slurm/v0.0.40`
- 驗證方式：JWT（`X-SLURM-USER-NAME`、`X-SLURM-USER-TOKEN`）
- 說明：`v0.0.40` 的 `/job/submit` 與 `/node/{node_name}` 在 upstream 已標記為 deprecated，但在目前 `infra_fusion` 佈署仍可使用。

## Get Token

以下範例預設在 headnode 上執行；若從 bastion 呼叫，將 `HEADNODE_HOST` 改成 headnode 主機名稱或 IP。

```bash
HEADNODE_HOST="localhost"
API_BASE="http://${HEADNODE_HOST}:6820/slurm/v0.0.40"

# 一般提交 job 的使用者
API_USER="cloud-user"
JOB_TOKEN=$(sudo scontrol token username="$API_USER" lifespan=3600 | sed -n 's/^SLURM_JWT=//p')

# drain/resume node 需要 Slurm admin/operator 權限
# 預設可直接使用 root；若你有其他 Slurm 管理帳號，也可以改成該帳號。
ADMIN_USER="root"
ADMIN_TOKEN=$(sudo scontrol token username="$ADMIN_USER" lifespan=600 | sed -n 's/^SLURM_JWT=//p')

test -n "$JOB_TOKEN" && echo "JOB token ok"
test -n "$ADMIN_TOKEN" && echo "ADMIN token ok"
```

快速測試 API 是否可通：

```bash
curl -s "$API_BASE/ping" \
	-H "X-SLURM-USER-NAME: $API_USER" \
	-H "X-SLURM-USER-TOKEN: $JOB_TOKEN"
```

## Send Slurm Job

### Basic Example

`infra_fusion` 預設會建立 `testaccount`，並把 `cloud-user` 加進這個 account，所以以下 payload 直接使用 `account=testaccount`。

建立 submit payload：

```bash
cat > /tmp/slurm-job-submit.json <<'EOF'
{
	"script": "#!/bin/bash\nhostname\necho slurm-api-test-ok",
	"job": {
		"name": "api-demo-job",
		"partition": "all",
		"account": "testaccount",
		"current_working_directory": "/tmp",
		"environment": [
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
		],
		"tasks": 1
	}
}
EOF
```

送出 job：

```bash
export BASE_URL=http://192.168.95.58:6820/slurm/v0.0.40
export API_USER="cloud-user"
export JOB_TOKEN=$(sudo scontrol token username="$API_USER" lifespan=3600 | sed -n 's/^SLURM_JWT=//p')
export PARTITION=all
export PROJECT=testaccount

# Use this command

JOB_SUBMIT_RESULT=$(curl -s -X POST "$BASE_URL/job/submit" \
	-H "Content-Type: application/json" \
	-H "X-SLURM-USER-NAME: $API_USER" \
	-H "X-SLURM-USER-TOKEN: $JOB_TOKEN" \
	-d @/tmp/slurm-job-submit.json)

# or Use this one

curl -s -X POST "${BASE_URL}/job/submit" \
     -H "Content-Type: application/json" \
	 -H "X-SLURM-USER-NAME: $API_USER"   \
     -H "X-SLURM-USER-TOKEN: $JOB_TOKEN" \
     -d @- <<EOF
{
	"job": {
		"name": "api-demo-job",
		"partition": "${PARTITION}",
		"account": "${PROJECT}",
		"current_working_directory": "/home/${API_USER}",
		"standard_output": "/home/${API_USER}/slurm_api_%j.out",
		"environment": [
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
		],
		"tasks": 1
	},
 	"script": "#!/bin/bash\nsrun bash -c 'echo HOSTNAME: \$(hostname) , DATE : \$(date) , USER: ${API_USER} '"
}
EOF

echo "$JOB_SUBMIT_RESULT"

JOB_ID=$(printf '%s\n' "$JOB_SUBMIT_RESULT" | sed -n 's/.*"job_id"[[:space:]]*:[[:space:]]*\([0-9][0-9]*\).*/\1/p' | head -1)
echo "JOB_ID=$JOB_ID"
```

查詢 job 狀態：

```bash
curl -s "$API_BASE/jobs/state/?job_id=$JOB_ID" \
	-H "X-SLURM-USER-NAME: $API_USER" \
	-H "X-SLURM-USER-TOKEN: $JOB_TOKEN"
```

查詢單一 job 詳細資訊：

```bash
curl -s "$API_BASE/job/$JOB_ID" \
	-H "X-SLURM-USER-NAME: $API_USER" \
	-H "X-SLURM-USER-TOKEN: $JOB_TOKEN"
```

### N4 Example

```shell
export BASE_URL=http://172.21.103.65:6820/slurm/v0.0.40
export API_USER=ogre0403
export JOB_TOKEN=$(scontrol token username="$API_USER" lifespan=3600 | sed -n 's/^SLURM_JWT=//p')
export PARTITION=slinky
export PROJECT=GOV113097

curl -s -X POST "${BASE_URL}/job/submit" \
     -H "Content-Type: application/json" \
     -H "X-SLURM-USER-TOKEN: $JOB_TOKEN" \
	 -H "X-SLURM-USER-NAME: $API_USER"   \
     -d @- <<EOF
{
  "job": {
    "name": "api_srun_migration",
    "partition": "${PARTITION}",
    "account": "${PROJECT}",
    "tasks_per_node": 1,
    "current_working_directory": "/home/${API_USER}",
    "standard_output": "/home/${API_USER}/slurm_api_%j.out",
    "tasks": 4,
    "environment": [
      "PATH=/usr/bin:/bin:/usr/local/bin:/usr/local/sbin:/usr/sbin",
      "LD_LIBRARY_PATH=/usr/lib"
    ]
  },
  "script": "#!/bin/bash\nsrun bash -c 'echo HOSTNAME: \$(hostname) , DATE : \$(date) , USER: ${API_USER} '"
}
EOF
```

## Drain and Resume Node

`drain` / `resume` 屬於管理操作，請使用具有 Slurm admin/operator 權限的 token。

先列出 node，確認要操作的名稱：

```bash
curl -s "$API_BASE/nodes/" \
	-H "X-SLURM-USER-NAME: $ADMIN_USER" \
	-H "X-SLURM-USER-TOKEN: $ADMIN_TOKEN"
```

### Drain Node

```bash
NODE_NAME="FUSION-03-worker-tf"
ADMIN_UID=$(id -u "$ADMIN_USER")

cat > /tmp/slurm-node-drain.json <<EOF
{
	"state": ["DRAIN"],
	"reason": "maintenance via slurmrestd"
}
EOF

curl -s -X POST "$API_BASE/node/$NODE_NAME" \
	-H "Content-Type: application/json" \
	-H "X-SLURM-USER-NAME: $ADMIN_USER" \
	-H "X-SLURM-USER-TOKEN: $ADMIN_TOKEN" \
	-d @/tmp/slurm-node-drain.json
```

確認 node 狀態：

```bash
curl -s "$API_BASE/node/$NODE_NAME" \
	-H "X-SLURM-USER-NAME: $ADMIN_USER" \
	-H "X-SLURM-USER-TOKEN: $ADMIN_TOKEN"
```

回傳中的 `state` 應包含 `DRAIN`。

### Resume Node

```bash
cat > /tmp/slurm-node-resume.json <<'EOF'
{
	"state": ["RESUME"]
}
EOF

curl -s -X POST "$API_BASE/node/$NODE_NAME" \
	-H "Content-Type: application/json" \
	-H "X-SLURM-USER-NAME: $ADMIN_USER" \
	-H "X-SLURM-USER-TOKEN: $ADMIN_TOKEN" \
	-d @/tmp/slurm-node-resume.json
```

再次確認 node 狀態：

```bash
curl -s "$API_BASE/node/$NODE_NAME" \
	-H "X-SLURM-USER-NAME: $ADMIN_USER" \
	-H "X-SLURM-USER-TOKEN: $ADMIN_TOKEN"
```

回傳中的 `state` 不應再包含 `DRAIN`；通常會回到 `IDLE` 或目前可服務的狀態。



## List Partition Nodes

```shell
export BASE_URL=http://192.168.95.58:6820/slurm/v0.0.40
export API_USER="cloud-user"
export JOB_TOKEN=$(sudo scontrol token username="$API_USER" lifespan=3600 | sed -n 's/^SLURM_JWT=//p')
export PARTITION=all

curl -s -X GET "$BASE_URL/nodes"    \
-H "X-SLURM-USER-NAME: $API_USER"   \
-H "X-SLURM-USER-TOKEN: $JOB_TOKEN" \
| jq ".nodes[] | select(.partitions[]? == \"$PARTITION\") | {name: .name, state: .state}"
```