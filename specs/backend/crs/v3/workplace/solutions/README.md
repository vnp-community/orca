# Solutions Index — Orca Side: Workplace Integration

> **Repository:** `orca/` (TypeScript/Node.js)  
> **Đối ứng:** [`vnp-workspace/backend/specs/crs/v3/orca/solutions/`](../../../../../vnp-workspace/backend/specs/crs/v3/orca/solutions/README.md)  
> **Mục đích:** Các giải pháp yêu cầu thay đổi code phía **Orca codebase** (TypeScript) để tích hợp với vnp-workspace.

---

## SOL Index

| SOL | CR | Scope | Priority | Status | File |
|-----|----|-------|----------|--------|------|
| SOL-INT-006 | CR-ORCA-INT-006 | Orca Web Server (TypeScript) | P0 | 📐 PROPOSED | [SOL-INT-006-orca-callback-publisher.md](./SOL-INT-006-orca-callback-publisher.md) |

---

## Phân chia trách nhiệm

| Codebase | Location | Ngôn ngữ | Team |
|----------|----------|----------|------|
| **vnp-workspace** (Go) | `vnp-workspace/backend/specs/crs/v3/orca/solutions/` | Go | Backend team |
| **orca** (TypeScript) | `orca/specs/backend/crs/v3/workplace/solutions/` ← **(file này)** | TypeScript/Node.js | Orca team |

---

## SOL-INT-006 — OrcaCallbackPublisher

**Mục tiêu:** Thêm `OrcaCallbackPublisher` vào Orca Web Server — lắng nghe `task.statusChanged` EventBus và gọi webhook về vnp-workspace khi coding task hoàn thành/thất bại.

**Phụ thuộc:** vnp-workspace endpoint `POST /api/v1/orca-callbacks` (SOL-INT-003) đã deploy ✅

**Env vars cần thêm vào Orca:**

| Var | Mô tả | Default |
|-----|-------|---------|
| `WORKSPACE_CALLBACK_URL` | `https://api.vnp-workspace.internal/api/v1/orca-callbacks` | `""` |
| `WORKSPACE_CALLBACK_SECRET` | Shared secret — phải khớp `ORCA_CALLBACK_SECRET` bên vnp-workspace | `""` |

---

## Trạng thái

- **vnp-workspace side (Go):** ✅ IMPLEMENTED — 6/7 SOL deployed (SOL-INT-001, 002, 003, 004, 005, 007)
- **orca side (TypeScript):** 📐 PROPOSED — SOL-INT-006 chờ Orca team implement
