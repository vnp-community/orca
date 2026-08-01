# F22 — Web Server Mode

| Trường | Giá trị |
|--------|---------|
| **ID** | F22 |
| **Tên** | Web Server Mode |
| **Ưu tiên** | P0 |
| **Trạng thái** | ✅ Phát hành |
| **CRs** | [restructure_v1](../crs/v1/restructure_v1/) |
| **TDD** | [TDD-11: Web Server Mode](../specs/backend/tdd/11-web-server-mode.md) |
| **Phiên bản** | v2.0+ |
| **ADR References** | ADR-001 |
| **HLD References** | C2, C3.6 |

---

## Mô tả

Orca có thể chạy như một **Node.js web server** (không cần Electron) — phục vụ giao diện web SPA qua HTTP và kết nối WebSocket. Đây là nền tảng cho Orca Cloud và deployment trên server.

---

## Vấn đề cần giải quyết

Trước đây Orca chỉ chạy được trong Electron (desktop). Để hỗ trợ:
- Truy cập Orca từ browser bất kỳ
- Deploy Orca trên cloud server (Docker, VPS)
- Nhiều developer chia sẻ 1 instance Orca trên remote server

→ Cần tách biệt platform runtime khỏi core business logic.

---

## Tính năng chi tiết

### Platform Abstraction Layer
- `IPlatformServices` interface thống nhất Electron và Node.js
- `NodeAdapter` — implement đầy đủ cho server mode
- `ElectronAdapter` — giữ nguyên desktop mode
- `getPlatform()` / `setPlatform()` singleton pattern

### HTTP + WebSocket Server
```
Port 6768 — WebSocket (OrcaRuntimeRpcServer + WebIpcBridge)
Port 6769 — HTTP (Express — SPA + health + auth + admin API)
```

### Web SPA Entry
- `bootstrapWebApp()` — mount React app sau khi WS connected
- `WebRoot` component — routing, pairing, auth gate
- `ConnectionStatusProvider` + `ConnectionStatusBanner` — realtime status
- `web-preload-api.ts` — `window.api` compat layer (same interface as Electron preload)

### IRpcClient Abstraction
- `IRpcClient` — shared interface
- `WebSocketRpcClient` — web mode implementation
- Electron mode: `ElectronRpcClient` (existing preload bridge)

### Build System
- `vite.web.config.ts` — builds `out/web/` bundle
- Multi-entry: `main` (web SPA) + `admin` (admin SPA)
- `base: './'` — hoạt động đúng dưới reverse proxy path prefix

---

## Luồng người dùng

```
1. Server admin deploy Orca Docker image
2. Browser navigate to https://orca.company.com
3. SPA load — ConnectionStatusProvider polling WS
4. Login page hiển thị (nếu ORCA_MULTI_USER=1)
5. Authenticated → WebRoot mount App.tsx
6. WS connected → full Orca UI hoạt động
```

---

## Tiêu chí chấp nhận

- [x] SPA serve đúng từ `out/web/web-index.html`
- [x] WebSocket `:6768` và HTTP `:6769` chạy trên 2 port tách biệt
- [x] `window.api` hoạt động giống Electron preload
- [x] `ConnectionStatusBanner` hiển thị khi mất WS
- [x] Docker build + healthcheck `/health/ready` hoạt động
- [x] Desktop Electron mode không bị ảnh hưởng

---

## Yêu cầu kỹ thuật

| Component | File |
|-----------|------|
| Platform context | `src/platform/context.ts` |
| Node adapter | `src/platform/adapters/node/` |
| Web bootstrap | `src/renderer/src/web/main-web-bootstrap.tsx` |
| Connection UI | `src/renderer/src/web/ConnectionStatusProvider.tsx` |
| RPC client | `src/platform/adapters/web/rpc-client.ts` |
| HTTP server | `src/server/http-server.ts` |
| Vite config | `vite.web.config.ts` |
| Docker | `deploy/prod/Dockerfile`, `docker-compose.yml` |

---

## Metrics

| KPI | Mục tiêu |
|-----|----------|
| Web SPA cold load | < 3s |
| WS reconnect time | < 2s |
| HTTP health response | < 50ms |
