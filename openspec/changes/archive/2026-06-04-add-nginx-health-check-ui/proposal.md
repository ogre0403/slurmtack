## Why

slurmtack 目前只有直接暴露的 API，沒有可供瀏覽器驗證的前端入口，也缺少未來 Web 功能可沿用的反向代理架構。需要先用最小範圍建立一個由 nginx 對外提供的靜態頁面，並把 slurmtack 留在容器內網，避免 daemon 直接對外暴露。

## What Changes

- 新增一個由 nginx 容器提供的靜態 HTML 驗證頁，作為後續 Web UI 的基礎入口
- 讓驗證頁先透過同源路徑呼叫 health check API，確認前端到後端的基本連線
- 在 nginx 設定中加入對 slurmtack 的反向代理，讓瀏覽器只打到 nginx，不直接連 daemon
- 調整容器拓樸與 compose 設定，讓 slurmtack 僅在內部網路提供服務，不再直接對 host 開 port
- 補上啟動與驗證文件，說明如何透過 nginx 驗證 `/api/health` 與靜態頁面

## Capabilities

### New Capabilities

- `health-check-ui`: 由 nginx 提供靜態 HTML，並透過反向代理路徑驗證 slurmtack health check API 的瀏覽器入口

### Modified Capabilities

- `daemon-deployment`: 將部署拓樸調整為 nginx 對外、slurmtack 僅內部可達，並以容器網路與 proxy 取代 daemon 直接暴露

## Impact

- **Deployment**: `docker/docker-compose.yaml` 需要新增 nginx 服務並改寫網路與 port 暴露方式
- **New assets**: 需要新增 nginx 設定檔與靜態 HTML 資源目錄
- **Daemon**: slurmtack `/health` API 行為不變，但 listen 與對外可達性會改由容器內網與 nginx 控制
- **Documentation**: README 或 docker 說明需補充新的驗證方式與對外入口
