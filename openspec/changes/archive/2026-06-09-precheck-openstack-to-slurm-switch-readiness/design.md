## Context

目前 `openstack_to_slurm` 的 live orchestrator 路徑在 `locked` 階段執行的 precheck，只確認 OpenStack compute service 可讀；真正會檢查 resident instances 與 active migrations 的是 `source_quiescing` 階段的 `verify_source_quiesce`。結果是 execution 已經進到 quiesce 之後，才發現其實 source node 還不能切換，operator 在 execution detail 中只看到流程停在較後面的 step。

另一個缺口是 step timeline 的資料模型。目前 durable step 只有 `error_class`、`exit_code` 與 evidence path，沒有保證存在可直接顯示在 UI 的拒絕原因文字。即使 precheck 被改成早期拒絕，dashboard 仍然無法在 execution detail steps 裡直接呈現為什麼被拒絕。

這個 change 會同時調整 workflow gating 與 step observability，讓 `openstack_to_slurm` 在 precheck 就能明確回答「能不能切換」以及「為什麼不能」。

## Goals / Non-Goals

**Goals:**

- 在 `openstack_to_slurm` 的 precheck 階段就檢查 source readiness，至少涵蓋 resident instances、active migrations、以及切換前需要的 compute-service 條件。
- 當 precheck 判定不能切換時，以 `precheck_blocked` 結束 execution，而不是先進入 `source_quiescing` 再卡在 `verify_source_quiesce`。
- 將拒絕原因以 operator-friendly 的 summary 持久化到 step timeline，並由 step API 與 dashboard execution detail 顯示。
- 保留現有 state machine 與 failure classification 的主結構，盡量把改動集中在 precheck、step persistence、API DTO 與 dashboard rendering。

**Non-Goals:**

- 重新設計整個 `openstack_to_slurm` state machine 或移除 `verify_source_quiesce` 這個 step。
- 引入新的 execution detail endpoint 或即時推播機制。
- 把完整 OpenStack 回應 payload 原封不動地存進 step timeline API；這個 change 聚焦在可讀的拒絕摘要，而不是完整證據瀏覽。

## Decisions

### 1. Make precheck the primary source-readiness gate for `openstack_to_slurm`

`openstack_to_slurm` precheck 將改為執行完整的 source-readiness probe，而不是只檢查 compute service 是否可讀。這個 probe 會蒐集至少三種 blocker 訊號：

- host 上是否仍有 resident instances；
- host 上是否仍有 active migrations；
- compute service 是否仍處於不允許切換的狀態。

只要任一 blocker 存在，precheck 就直接失敗，execution 以 `precheck_blocked` 結束，並且不進入 `precheck_passed` 或 `source_quiescing`。

Alternatives considered:

- 繼續只在 `verify_source_quiesce` 檢查 VM/migration。Rejected because 這正是目前過晚暴露 blocker 的問題來源。
- 把拒絕前移到 API request admission。Rejected because 這些訊號是 node-bound 的 live control-plane 檢查，仍然屬於 lease 後的 precheck 階段，而不是單純 request-shape validation。

### 2. Keep `verify_source_quiesce` as a post-quiesce backstop, not the first blocker detector

`verify_source_quiesce` 仍然保留，因為 quiesce 與 detach 之間仍需要一個 post-action confirmation point；但它不應再是一般已知 blocker 第一次被看到的地方。設計上會把「已知不可切換」的條件前移到 precheck，而 `verify_source_quiesce` 只負責確認 quiesce 後的 source 狀態確實達標，並處理 precheck 與 quiesce 之間可能發生的 control-plane drift。

這讓 workflow 保持防禦性，但 operator 在常見阻擋情境下會更早收到拒絕。

Alternatives considered:

- 完全移除 `verify_source_quiesce`。Rejected because 這會失去 quiesce 後的 safety confirmation，對現有 state machine 風險太高。
- 讓 precheck 與 verify 都各自組裝不同 blocker 規則。Rejected because 容易造成兩處判斷漂移。

### 3. Produce a deterministic step-level `error_summary` for blocked prechecks

step persistence 會新增一個短文字欄位，例如 `error_summary`，用來放 operator-visible 的拒絕原因摘要。對於 `precheck_blocked`，daemon 應該從 probe 結果組裝穩定、可讀、可比較的 summary，例如：

- `resident instances: 2`
- `active migrations: 1`
- `compute service still enabled`

若同時存在多個 blocker，summary 需要合併成單一穩定字串，而不是只回傳第一個錯誤。內部仍可保留原始錯誤與 log/evidence 作為較細節的診斷來源，但 API/UI 至少要有這段 summary 可直接顯示。

Alternatives considered:

- 只依賴 `error_class`。Rejected because `precheck_blocked` 無法說明到底是 VM、migration，還是 compute-service 狀態阻擋。
- 只把原因寫到 stdout/stderr log path。Rejected because operator 還要跳出 execution detail 才能看見，沒有滿足這次需求。

### 4. Reuse one readiness-evaluation path for both workflow behavior and observability

precheck 的 blocker 判斷與 summary 組裝應該共用同一份 readiness evaluation 邏輯，而不是讓 orchestrator、OpenStack adapter、API 各自拼接字串。實作上可以抽出一個 probe result 結構，包含：

- observed counts / booleans；
- normalized blocker list；
- operator-visible summary string。

orchestrator 用它來決定是否 transition 到 `precheck_passed`，step helper 用它來填 `error_summary`，必要時 `verify_source_quiesce` 也能重用相同結果格式作為 backstop diagnostics。

Alternatives considered:

- 直接回傳一般 `error` 字串，由呼叫端各自解析。Rejected because 容易讓 workflow decision、step persistence、UI 顯示對同一失敗出現不一致字串。

## Risks / Trade-offs

- [Precheck 與 verify 之間仍可能發生 control-plane drift] → 保留 `verify_source_quiesce` 作為 backstop，避免只靠單次 precheck 判定。
- [新增 step 欄位需要 schema/API/UI 一起調整] → 使用向後相容的可選欄位 `error_summary`，舊資料可保持空值。
- [多個 blocker 的 summary 若格式不穩定，測試與 UI 會脆弱] → 定義固定排序與固定片語，避免依 map iteration 或原始 error 字串順序輸出。
- [把更多拒絕情境前移到 precheck 可能改變 operator 習慣] → 保持 failure class 不變為 `precheck_blocked`，只調整被拒絕的時點與可見原因。

## Migration Plan

1. 在 step store schema 加入可選的 step-level `error_summary` 欄位，並更新 store read/write 路徑。
2. 將 `openstack_to_slurm` precheck 改為呼叫完整 readiness probe，讓 resident instances / active migrations / compute-service blockers 在 precheck 直接失敗。
3. 更新 `GET /v1/switches/:id/steps` DTO 與 Swagger，回傳新的 step rejection summary 欄位。
4. 更新 dashboard execution detail step rendering，在 failed precheck step 顯示 `error_summary`。
5. 以一般 code rollout 發佈。舊 execution 的 steps 沒有 `error_summary` 也仍可正常讀取；rollback 只需回退程式碼，資料庫新增欄位可保留。

## Open Questions

- None. 欄位命名可在實作時最終落在 `error_summary` 或同等語意名稱，但行為上需要的是可持久化、可回傳、可顯示的 step-level 拒絕摘要。
