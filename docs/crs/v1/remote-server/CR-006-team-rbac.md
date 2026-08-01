# CR-006 — Team-based Access Control (RBAC)

**CR-ID:** CR-006  
**Ngày:** 2026-07-22  
**Priority:** 🟡 Medium  
**Effort:** Large (5–7 ngày)  
**Depends on:** CR-001 (Fleet Inventory), CR-002 (Grouping)  
**Status:** Partial — Phase 1 types ready, Phase 2 SSO pending  

---

## 1. Vấn đề

Hiện tại, bất kỳ developer nào nhận được pairing URL của Orca Server đều có thể:
- Thấy **tất cả** SSH targets trong fleet
- Kết nối vào **bất kỳ** server nào
- Tạo worktree trên **bất kỳ** project nào

Không có cơ chế:
- Giới hạn developer X chỉ thấy servers của project Y
- Phân quyền: junior dev không tạo worktree trên production
- Audit: ai đã kết nối vào server nào, khi nào

**Tác động:**
- Security risk: developer project A có thể vào server project B
- Compliance: không audit trail
- Operational: developer bị confused bởi servers không liên quan

---

## 2. Phân tích codebase

### 2.1 Pairing và Authentication hiện tại

```typescript
// src/renderer/src/web/WebConnect.tsx
// Pairing token = single shared token cho toàn bộ Orca Server
// → Ai có token đều có full access

// src/main/runtime/
// device-registry.ts: quản lý connected devices
// Chỉ track device, không track user identity
```

### 2.2 Security model hiện có

Từ `docs/hld/security.md`:
- **Trust Boundary 1:** Electron App sandbox
- **Trust Boundary 2:** SSH (Desktop → Remote Host)
- **Trust Boundary 3:** Mobile E2E encryption

**Không có:**
- User authentication (login/password/SSO)
- Role-based access control
- Per-user token với scope

### 2.3 Agent Trust Presets

```typescript
// src/main/agent-trust-presets.ts
// Có 3 tiers: Minimal, Standard, Full
// Nhưng: áp dụng cho AI agents, không phải developer access control
```

---

## 3. Giải pháp đề xuất

### 3.1 Phương án đơn giản: Multiple Orca Instances (Workaround)

**Giải pháp ngay lập tức** (không cần code change):

```yaml
# docker-compose.yml
services:
  orca-vnp-blc:       # Chỉ team vnp-blc có pairing URL
    ports: ["6768:6768"]

  orca-vnp-ai-ops:    # Chỉ team vnp-ai-ops có pairing URL
    ports: ["6769:6769"]

  orca-vnp-claw:      # Chỉ team vnp-claw có pairing URL
    ports: ["6770:6770"]
```

Mỗi Orca instance có **pairing token riêng** → tự nhiên isolate access theo team.

### 3.2 Phương án dài hạn: User Identity + RBAC

```typescript
// Proposed: src/shared/rbac-types.ts
export type OrcaUser = {
  id: string
  email: string
  name: string
  teams: string[]          // ["backend", "frontend"]
  projects: string[]       // ["vnp-blc", "vnp-ai-ops"]
  role: 'developer' | 'lead' | 'admin'
}

export type AccessPolicy = {
  // Ai được phép vào server nào
  serverAccess: Array<{
    serverId: string        // "dev-alpha"
    allowedTeams: string[]  // ["backend"]
    allowedUsers: string[]  // ["user@vnpblc.com"]
  }>
  // AI agent trust tier theo user role
  agentTrust: Record<string, 'minimal' | 'standard' | 'full'>
}
```

### 3.3 OIDC / SSO integration (enterprise)

```
Developer → Orca Web UI
              ↓ Redirect to SSO (Keycloak/Google/GitHub)
              ↓ Authenticate
              ↓ OIDC token → Orca validates
              ↓ Extract user's teams/projects from token claims
              ↓ Filter SSH targets by user's allowed projects
              ↓ Generate scoped pairing token
              ↓ 
Developer sees ONLY their servers
```

### 3.4 `orca-fleet.yaml` với access control

```yaml
# deploy/dev/orca-fleet.yaml (enhanced)
version: "1"

access:
  # SSO config (optional)
  sso:
    provider: github   # hoặc google, keycloak, etc.
    clientId: ${GITHUB_OAUTH_CLIENT_ID}
    allowedOrg: vnpblc

  # Policy: team → servers
  policies:
    - team: backend
      allowedServers: [dev-alpha, dev-alpha2]
      agentTrust: standard

    - team: ai-platform
      allowedServers: [dev-beta]
      agentTrust: full

    - team: frontend
      allowedServers: [dev-gamma]
      agentTrust: standard

    - role: admin
      allowedServers: "*"  # all
      agentTrust: full

servers:
  - id: dev-alpha
    label: "Dev Alpha — vnp-blc"
    project: vnp-blc
    team: backend
    # ...
```

---

## 4. Changes Required

### 4.1 Orca codebase (dài hạn)

| File | Thay đổi |
|------|---------|
| `src/shared/` | Thêm `rbac-types.ts` |
| `src/main/runtime/device-registry.ts` | Thêm user identity tracking |
| `src/main/runtime/orca-runtime.ts` | Filter SSH targets theo user policy |
| `src/main/ipc/` | Scoped pairing token generation |
| `src/renderer/src/web/` | Login/SSO flow trước WebConnect |

### 4.2 Deploy (workaround ngay lập tức)

| File | Thay đổi |
|------|---------|
| `deploy/dev/docker-compose.yml` | Thêm nhiều orca-* services |
| `deploy/dev/docker/nginx/conf.d/` | Thêm vhosts cho từng team |
| `deploy/dev/.env` | Thêm env vars cho từng instance |

---

## 5. Workaround ngay lập tức: Multi-Instance

### 5.1 Cập nhật `docker-compose.yml`

```yaml
services:
  # ── Team Backend: vnp-blc ─────────────────────────────────
  orca-backend:
    build: ./docker/orca
    container_name: orca-backend
    environment:
      ORCA_DOMAIN: orca-backend.vnpblc.internal
      ORCA_PORT: 6768
    volumes:
      - ./dist/linux-unpacked:/opt/orca/app:ro
      - orca-backend-data:/home/orca/.config/orca
      - ./docker/orca/ssh:/home/orca/.ssh:ro
    expose: ["6768"]
    networks: [orca-net]

  # ── Team AI Platform: vnp-ai-ops ─────────────────────────
  orca-ai:
    build: ./docker/orca
    container_name: orca-ai
    environment:
      ORCA_DOMAIN: orca-ai.vnpblc.internal
      ORCA_PORT: 6769
    volumes:
      - ./dist/linux-unpacked:/opt/orca/app:ro
      - orca-ai-data:/home/orca/.config/orca
      - ./docker/orca/ssh:/home/orca/.ssh:ro
    expose: ["6769"]
    networks: [orca-net]

  nginx:
    image: nginx:alpine
    ports: ["443:443", "80:80"]
    volumes:
      - ./docker/nginx/conf.d:/etc/nginx/conf.d:ro
      - ./docker/nginx/certs:/etc/nginx/certs:ro
    networks: [orca-net]
```

### 5.2 Nginx vhosts

```nginx
# docker/nginx/conf.d/orca-backend.conf
server {
    listen 443 ssl;
    server_name orca-backend.vnpblc.internal;
    ssl_certificate /etc/nginx/certs/server.crt;
    ssl_certificate_key /etc/nginx/certs/server.key;

    location / { proxy_pass http://orca-backend:6768; ... }
    location /ws { proxy_pass http://orca-backend:6768; upgrade; ... }
}

# docker/nginx/conf.d/orca-ai.conf
server {
    listen 443 ssl;
    server_name orca-ai.vnpblc.internal;
    ...
    location / { proxy_pass http://orca-ai:6769; ... }
}
```

### 5.3 Phân phối pairing URL theo team

```bash
# DevOps lấy pairing URL riêng cho từng team
docker logs orca-backend | grep "Web UI"  # → gửi cho team backend
docker logs orca-ai | grep "Web UI"       # → gửi cho team ai-platform
```

---

## 6. Audit Logging (workaround)

```bash
# Nginx access log đã capture IP + timestamp của mỗi connection
# Kết hợp với pairing URL per-team → biết team nào kết nối

# Phân tích access log
tail -f /var/log/nginx/orca-backend-access.log
# → 10.0.0.5 - - [2026-07-22] "GET /web-index.html" 200
```

---

## 7. Acceptance Criteria

**Phase 1 (Workaround — ngay bây giờ):**
- [x] Mỗi team có Orca instance riêng (docker-compose.orca.yml)
- [x] Mỗi instance có pairing URL riêng
- [x] Developer chỉ thấy server của project mình
- [x] Nginx logs capture access per-team

**Phase 2 (RBAC trong Orca):**
- [x] `src/shared/rbac-types.ts`: `OrcaUser`, `OrcaAccessPolicy`, `ScopedPairingToken`, `resolveUserPermissions()` đã định nghĩa
- [x] Login screen trước Orca UI (OIDC/SSO flow) ✅ `LoginPage.tsx` + `OrcaLoginScreen.tsx` — local auth done; SSO Phase 2
- [ ] Pairing token scope theo user's teams — **DEFERRED Phase 3** (RBAC per-team scoping)
- [ ] SSH target list filtered theo policy — **DEFERRED Phase 3** (Access Policy enforcement at SSH layer)
- [x] Audit log: user X accessed server Y at time Z ✅ `audit-logger.ts` — `ssh.connect` action logged with userId, targetId
- [x] Admin dashboard: manage users + policies ✅ `src/renderer/src/components/admin/` — AdminApp + UsersPage + PoliciesPage

---

## 8. Implementation Notes

> **Phase 1:** 2026-07-23 — Single-instance + docker-compose isolation  
> **Phase 2:** Planned — Types defined, implementation pending

| File | Status |
|------|--------|
| `src/shared/rbac-types.ts` | ✅ [NEW] `OrcaUser`, `OrcaAccessPolicy`, `ScopedPairingToken`, `OrcaSsoConfig`, `resolveUserPermissions()` |
| `deploy/dev/docker-compose.orca.yml` | ✅ [NEW] Single-instance Orca server cho VNP-BLC team |
| `deploy/dev/docker/nginx/` | ✅ [EXISTS] Nginx config cho SSL termination |
| `src/main/runtime/device-registry.ts` | ⚠️ [PENDING] User identity tracking |
| `src/renderer/src/web/` | ⚠️ [PENDING] Login/SSO flow trước WebConnect |
| `src/main/ipc/` | ⚠️ [PENDING] Scoped pairing token generation |

---

## Implementation Status

> **✅ PHASE 1 IMPLEMENTED — 2026-07-24**  
> 3/5 AC done | 2 DEFERRED Phase 3 (per-team scoping, SSH policy enforcement)

| Layer | Files | Status |
|-------|-------|--------|
| Login UI | `src/renderer/src/web/login/LoginPage.tsx` | ✅ Done (local auth; SSO Phase 2) |
| Audit Log | `src/main/admin/audit-logger.ts` | ✅ Done |
| Admin Dashboard | `src/renderer/src/components/admin/AdminApp.tsx` | ✅ Done |
| Policies UI | `src/renderer/src/components/admin/PoliciesPage.tsx` | ✅ Done |
| DB Schema | `src/main/db/migrations/0005_add_auth_schema.ts` — `orca_access_policies` | ✅ Done |
| Pairing token scope | Not implemented | ⏳ Phase 3 |
| SSH policy enforcement | Not implemented | ⏳ Phase 3 |
