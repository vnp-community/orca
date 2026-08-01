# BL-INT-01: CLI Auth Proxy (GitHub/GitLab qua SSH Relay)

**Domain:** Remote Source Control Integrations  
**Priority:** P1  
**Actor chính:** Carlos, Alex  
**Tham chiếu:** FR-18.1, F30

---

## Mô tả

Proxy các preflight check và auth flows cho GitHub CLI (`gh`) và GitLab CLI (`glab`) tới Dev Server qua SSH Relay. Giải quyết vấn đề CLI tools cần chạy trên Dev Server (không phải trên Orca Web Server).

## Problem

Trong web mode:
- `gh auth status` cần chạy trên **Dev Server** (nơi có `gh` được cài)
- Nhưng Orca Web Server chạy trên container/cloud riêng
- Giải pháp: proxy preflight.check request qua SSH relay tới Dev Server

## Preflight Proxy Flow

```
1. Renderer gửi: preflight.check { devServerId: "ds-abc" }
2. Backend tìm DevServer record by devServerId
3. Orca kết nối SSH → DevServer
4. Relay handler chạy trên DevServer:
   - GH_CONFIG_DIR=~/.config/gh/<userId>/
   - gh --version
   - gh auth status
5. Kết quả trả về Orca
6. Orca gọi mergePreflightStatuses(localStatus, remoteStatus)
7. Renderer hiển thị merged status
```

## Session Isolation (Config Dirs)

| Tool | Per-User Config Dir |
|------|-------------------|
| GitHub CLI | `GH_CONFIG_DIR=~/.config/gh/<userId>/` |
| GitLab CLI | `GLAB_CONFIG_DIR=~/.config/glab-cli/<userId>/` |

Inject qua env variables trong SSH exec. Mỗi user có config riêng, tránh credential cross-contamination.

## CLI Auth Login Flow (PTY)

```
1. User click "Login with GitHub" trên WebModeCliAuthSection
2. Frontend gọi: github.startAuthLogin({ devServerId })
3. Backend mở PTY trên Dev Server qua relay: "gh auth login"
4. PTY output stream tới frontend → hiển thị như terminal
5. User follow interactive prompts (browser redirect, paste code)
6. gh auth login hoàn thành → lưu credentials vào GH_CONFIG_DIR
7. Backend verify: gh auth status → emit success
```

## mergePreflightStatuses()

```typescript
// Kết hợp kết quả local check và remote (relay) check
// Ưu tiên: relay result > local result
function mergePreflightStatuses(
  localStatus: PreflightStatus,
  relayStatus: PreflightStatus
): PreflightStatus {
  // nếu relay có data → dùng relay
  // nếu relay error (SSH không kết nối được) → fallback local
  // nếu local không có → chỉ relay
}
```

## Source References

- `src/main/integrations/github-preflight.ts` — proxyPreflightCheck()
- `src/main/integrations/preflight-merge.ts` — mergePreflightStatuses()
- `src/renderer/src/components/WebModeCliAuthSection.tsx`
- `src/main/rpc/rpc-context.ts` — RpcMethodContext (devServerManager, userId injection)
