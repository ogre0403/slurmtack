## Why

目前 switch workflow 的 `reconfigure` 與 post-reboot `verify` 已經會直接執行 `scripts/gpu-passthrough`，但這組腳本在沒有 NVIDIA GPU 的節點上本來就會失敗。對沒有 GPU 的測試環境來說，這會把「硬體不存在」誤報成「流程失敗」，使我們無法驗證整體 scp/SSH/reboot 後 verify 的 orchestration 是否正確。

## What Changes

- 新增一組 repo-owned 的 `scripts/fake-passthrough` script bundle，提供與 `scripts/gpu-passthrough` 相同的檔名與 CLI 介面，讓無 GPU 環境也能跑完整 passthrough reconfigure/verify 流程。
- 定義 fake bundle 的行為契約：`reconfigure.sh` 與 `verify.sh` 都接受 `enable|disable`，在測試模式下執行 deterministic no-op 並回傳成功，不依賴實體 GPU 或 VFIO 狀態。
- 更新 orchestration requirement，明確允許 `GPU_PASSTHROUGH_SCRIPT_DIR` 指向任何相容的 passthrough script bundle，包括 fake bundle，並維持既有的 stage-then-run 流程不變。
- 更新 operator-facing 文件與測試，讓非 GPU 測試環境可以明確選用 fake bundle 來驗證整體 switch workflow。

## Capabilities

### New Capabilities
- `fake-passthrough-host-scripts`: 提供一組可在無 GPU 節點上執行的 fake passthrough scripts，保持與真實 GPU passthrough bundle 相同的介面與呼叫方式。

### Modified Capabilities
- `gpu-node-switch-orchestration`: host reconfigure 與 post-reboot verify 需支援使用相容的 fake passthrough bundle，以便在無 GPU 測試環境中驗證既有 staging 與 SSH 執行流程。

## Impact

- Affected code: `scripts/`, orchestrator passthrough script staging/execution tests, and operator-facing configuration/docs for `GPU_PASSTHROUGH_SCRIPT_DIR`.
- Operational impact: non-GPU test or CI environments can validate the full switch orchestration path without introducing false failures from the real GPU passthrough checks.
- Risk surface: operators could accidentally point production at the fake bundle, so the docs and script output need to make the test-only nature explicit.
