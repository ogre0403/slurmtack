## Context

目前 `GPU_PASSTHROUGH_SCRIPT_DIR` 會把 orchestrator 綁到 `scripts/gpu-passthrough` 這組真實 host scripts。這組腳本的失敗條件對 production GPU node 是正確的，例如 `enable` 沒找到 NVIDIA GPU 就必須 fail，`verify` 也必須真的檢查 VFIO 與 driver binding。

但在沒有 GPU 的測試環境中，這種「真實失敗」會讓我們無法區分兩件事：

- passthrough host state 不存在，所以真實腳本合理失敗；
- orchestrator 的 scp staging、SSH 執行順序、方向對應動作、reboot 後 verify 邊界其實是正確的。

這個 change 的目標是提供一個與真實 passthrough bundle 介面完全相容、但不依賴 GPU 硬體的 fake bundle，讓 end-to-end workflow 可以在 GPU-less 環境下被驗證。

## Goals / Non-Goals

**Goals:**

- 新增一個 repo-owned `scripts/fake-passthrough` bundle，檔名與 CLI surface 與 `scripts/gpu-passthrough` 一致。
- 讓 fake `reconfigure.sh` / `verify.sh` 在 `enable` 與 `disable` 下都能 deterministic 成功，不因為缺少 GPU、VFIO config 或 kernel args 而失敗。
- 讓 orchestrator 透過既有 `GPU_PASSTHROUGH_SCRIPT_DIR` 就能選用 fake bundle，不引入額外的 fake-mode orchestration 分支。
- 透過文件與測試明確區分 real bundle 與 fake bundle 的使用場景，避免把 test-only 行為誤認成 production contract。

**Non-Goals:**

- 放寬 `scripts/gpu-passthrough` 的真實驗證標準，讓它在沒有 GPU 的機器上也成功。
- 讓 orchestrator 自動偵測節點有沒有 GPU，並在 runtime 自動切換 real/fake bundle。
- 模擬完整的 VFIO、driver binding 或 reboot side effects；這個 change 只保證 orchestration flow 可以被驗證。

## Decisions

### 1. Add a separate fake bundle instead of weakening the real GPU bundle

真實 `gpu-passthrough` 腳本的責任是保護 production host state；如果把「沒有 GPU 也成功」塞回真實腳本，會直接稀釋原本的重要失敗訊號，讓真實錯誤變成假成功。較安全的做法是保留 real bundle 的硬性檢查，另外新增一個語意清楚的 fake bundle 專門給測試環境。

Alternatives considered:

- 直接修改 `scripts/gpu-passthrough`，在沒有 GPU 時回傳成功。Rejected because 這會讓 production 節點失去「GPU 不存在或偵測失敗時必須阻擋流程」的 safety guard。

### 2. Preserve the exact script interface so orchestration does not branch on bundle type

fake bundle 會維持與 real bundle 相同的檔名與 CLI 介面：`reconfigure.sh <enable|disable>`、`verify.sh <enable|disable>`，以及必要時同目錄下的 helper file。這讓 orchestrator 只需要依賴「compatible bundle」的概念，而不是在 code 裡額外判斷目前是 GPU mode 還是 fake mode。

Alternatives considered:

- 新增另一組 fake-only 檔名或新的 environment variable。Rejected because 這會把同一個 orchestration contract 分裂成兩套介面，增加 staging 與測試維護成本。

### 3. Make fake operations deterministic no-op success with explicit fake-mode output

fake scripts 的主要價值是驗證流程，不是模擬硬體。因此 `enable`/`disable` 的 `reconfigure` 與 `verify` 都應該走 deterministic 的 no-op success 路徑，並在 stdout/stderr 清楚標示「這是 fake/test-only passthrough」。這樣可以讓測試結果穩定、診斷時也不會誤以為真的做了 VFIO 變更。

Alternatives considered:

- 做更深入的 fake state 檔案模擬，讓 `enable` 與 `disable` 之間彼此相依。Rejected because 這會把 change 擴大成一個新的 host-state simulator，超出目前驗證 orchestration flow 的需求。

### 4. Keep selection in configuration, and document test-only usage explicitly

bundle 選擇仍然透過既有 `GPU_PASSTHROUGH_SCRIPT_DIR` 控制。default 與 production 文件仍然應該指向真實 `scripts/gpu-passthrough`；只有 non-GPU 測試或 CI 環境才改指向 fake bundle。這讓 rollout 與 rollback 都只是設定切換，不需要改 orchestrator state machine。

Alternatives considered:

- 讓 orchestrator 自動根據 node hardware 決定使用哪一組腳本。Rejected because 這會把測試策略與 production 行為混在一起，還需要新增硬體探測與決策邏輯，風險高於收益。

## Risks / Trade-offs

- [Fake bundle 被誤用到 production] → Mitigation: 目錄命名、README、script output、以及 env 文件都要明確標示 test-only，用語避免與 real bundle 混淆。
- [用 fake bundle 驗證通過，不代表 real GPU host state 一定正確] → Mitigation: 保留 real bundle 的既有驗證與 script tests，fake bundle 只作為 non-GPU flow validation 補充。
- [兩組 bundle 介面日後漂移] → Mitigation: 以相同檔名、相同行為入口、以及 focused tests 固定這份介面契約。

## Migration Plan

1. 新增 `scripts/fake-passthrough` 與對應的 script-level tests。
2. 更新 orchestrator focused tests，確認 `GPU_PASSTHROUGH_SCRIPT_DIR` 指向 fake bundle 時，reconfigure 與 verify 仍走既有 stage-and-run 流程。
3. 更新 README / env 文件，說明 real bundle 與 fake bundle 的選擇方式與風險。
4. 在 non-GPU 測試環境切換 `GPU_PASSTHROUGH_SCRIPT_DIR` 到 fake bundle；production 保持 real bundle 不變。

Rollback 只需要把測試環境的 script-dir 設定切回原本值，或回退新增的 fake bundle 與相關文件/測試變更。

## Open Questions

- None. 這個 change 不改變 orchestrator 的外部 API，也不需要新增 runtime negotiation；主要是補上一個明確、可配置、test-only 的 script bundle。
