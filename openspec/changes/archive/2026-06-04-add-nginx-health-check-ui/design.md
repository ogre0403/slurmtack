## Context

目前容器部署只有 `slurmtack` 與 `rabbitmq`，而且兩者都使用 `network_mode: host`，代表 slurmtack 的 HTTP API 直接對 host 開放。repo 已有不需認證的 `GET /health`，但沒有任何前端頁面或反向代理層，無法用瀏覽器驗證未來 Web 功能的基本連線，也不符合「由 nginx 對外、daemon 僅內部可達」的目標。

這次變更需要在不改動 slurmtack health handler 契約的前提下，新增一個最小可用的 Web 驗證入口，並把容器拓樸改成 nginx 對外、slurmtack 對內。

## Goals / Non-Goals

**Goals:**

- 提供一個由 nginx 容器直接提供的靜態 HTML 驗證頁面
- 讓頁面透過 nginx proxy 呼叫 slurmtack 的 health check，驗證 browser -> nginx -> slurmtack 的鏈路
- 讓 slurmtack 不再直接對 host 開 port，而只在 compose 內部網路上被 nginx 存取
- 保持現有 slurmtack `/health` API contract 不變，避免不必要的 daemon API 改動
- 讓這個佈署方式可作為後續更多 Web UI 與 API 代理的基礎

**Non-Goals:**

- 不在這次變更中實作完整前端框架或多頁面 UI
- 不改寫 slurmtack 現有 `/v1/*` API 行為或授權機制
- 不在這次變更中處理完整的 TLS、Ingress、外部 load balancer 或 production hardening
- 不新增超過 health check 以外的前端功能驗證流程

## Decisions

### Decision: Use an nginx container with mounted config and static assets

`docker-compose` 會新增 `nginx` 服務，直接使用官方 nginx image，並掛載 repo 內的 nginx 設定檔與靜態 HTML 目錄。這樣不需要再維護額外的 nginx build pipeline，也能把 HTML 與 proxy 規則都留在版本控制中。

Rationale:

- 官方 nginx image 已足夠支撐靜態檔案與反向代理需求
- 掛載設定檔與靜態檔比自建 image 更小、更直觀，適合目前的驗證用途
- 後續若要替換成更完整前端 build，也可以延續同樣的 nginx 入口

Alternatives considered:

- 為 nginx 額外建立 Dockerfile：目前沒有必要，會增加維護面
- 直接由 slurmtack 提供 HTML：會把 UI 靜態資產與 daemon API 綁在一起，不符合要求的 proxy 架構

### Decision: Replace host networking with a shared bridge network and publish only nginx

compose 會改成 user-defined bridge network。`nginx` 透過 host port 對外提供入口，`slurmtack` 不設定 `ports`，僅保留在內部網路以服務名 `slurmtack:8080` 被 proxy。`rabbitmq` 繼續作為 compose 內服務，是否保留既有 host port 由現有 compose 行為決定，但不再依賴 host network。

Rationale:

- 只要 slurmtack 不 publish port，就能滿足「不要讓 slurmtack 對外」
- bridge network 可讓 nginx 與 slurmtack 透過服務名互通，設定比 host network 更穩定
- 這種拓樸更接近未來實際 Web/UI 佈署方式

Alternatives considered:

- 保留 host network 並讓 nginx 代理 `127.0.0.1:8080`：daemon 仍直接暴露在 host，不符合要求
- 只把 nginx 放在 host network：服務發現與隔離更複雜，沒有明顯收益

### Decision: Expose a same-origin proxied path at `/api/health`

靜態頁面會向 nginx 的 `/api/health` 發出請求，由 nginx proxy 到 slurmtack 的 `GET /health`。頁面不直接知道 slurmtack 的容器名稱、port 或內部位址；成功時顯示 `ok` 類狀態，失敗時顯示不可用訊息。

Rationale:

- `/api/health` 明確區分 UI 靜態路徑與後端 API 路徑
- 同源請求不需要額外處理 CORS，最適合靜態 HTML 驗證
- 之後若要擴充其他 proxied API，可沿用 `/api/*` 命名方式

Alternatives considered:

- 讓頁面直接呼叫 `/health`：雖然可行，但靜態頁與 API 路徑不易區分，之後擴充較混亂
- 讓頁面直接呼叫 slurmtack 容器位址：違反由 nginx 代理的要求

### Decision: Keep the HTML intentionally minimal and self-contained

初版頁面只需要單一 static HTML，內含必要的 JavaScript 來在載入後呼叫 `/api/health`，並將結果渲染到頁面。若請求失敗，頁面需要顯示錯誤狀態與簡短訊息，讓操作者知道 proxy 或 backend 不可用。

Rationale:

- 這次的目的是驗證架構而不是建立完整前端工程
- 單檔 HTML 最容易快速驗證 nginx static hosting 與 proxy 行為
- 減少 tooling，讓後續是否導入真正前端框架保持開放

Alternatives considered:

- 直接導入 React/Vite 等前端工具鏈：對目前需求過重
- 只提供純靜態文案不打 API：無法驗證 proxy 架構

## Risks / Trade-offs

- [從 host network 改成 bridge network 會改變服務連線方式] → 在 compose 與 `.env`/文件中明確指定 service DNS 名稱與對外入口
- [nginx proxy path 規則若寫錯，容易造成 404 或 rewrite 問題] → 只先支援明確的 `/api/health` 對 `/health` 映射，避免過早泛化
- [未來前端擴充時，單頁 static HTML 可能需要重構] → 先固定 nginx 作為前端入口，保留之後替換 HTML 資產來源的空間
- [如果 rabbitmq 仍需對 host 提供管理介面，compose 網路調整可能帶來額外驗證成本] → 在任務中納入 compose 啟動與既有服務可達性檢查

## Migration Plan

1. 新增 nginx 設定檔與靜態 HTML 資產，定義 `/` 與 `/api/health` 路由。
2. 修改 `docker/docker-compose.yaml`，加入 nginx 服務、內部網路與對外 port 映射，移除 slurmtack 的直接對外暴露。
3. 視需要調整 docker 文件與環境變數範例，讓 slurmtack 與 rabbitmq 在 bridge network 下仍能互通。
4. 以 compose 啟動後驗證三件事：`/` 可載入、`/api/health` 經 proxy 成功、slurmtack 無法再直接由 host 連入。
5. 若需回滾，可移除 nginx 服務並恢復原本 compose 網路模式與 slurmtack 對外暴露。

## Open Questions

- None.
