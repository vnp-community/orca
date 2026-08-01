# Restructure v1 — Change Request Series

**Mục tiêu:** Tách biệt Backend và Frontend, đảm bảo hỗ trợ triển khai Web-only (không cần Electron) trong khi vẫn duy trì toàn bộ tính năng hiện có và giảm thiểu xung đột merge với upstream trong tương lai.

---

## ✅ Implementation Status

> **HOÀN THÀNH — 2026-07-23 | Tests: 34/34 (frontend) | 0 TS errors**

| CR | Tên | Status |
|----|-----|--------|
| CR-001 | Platform Interface & Adapter Layer | ✅ Implemented |
| CR-002 | Node.js Server Adapter | ✅ Implemented |
| CR-003 | IPC Abstraction / IRpcClient | ✅ Implemented |
| CR-004 | Web Entry & Bootstrap | ✅ Implemented |
| CR-005 | Build System (vite.web.config.ts) | ✅ Implemented |
| CR-006 | Docker Deployment | ✅ Implemented |
| CR-007 | Electron Mock Cleanup | ✅ Implemented |

---

## Bối cảnh

Orca hiện là một ứng dụng Electron với kiến trúc monolithic:
- `src/main/` — Electron Main Process (backend, IPC, PTY, SSH, Git, Runtime services...)  
- `src/renderer/` — React frontend, tương tác qua Electron IPC  
- `src/shared/` — Types, utilities dùng chung  
- `src/preload/` — Electron preload bridge  
- `src/server/` — Entry point Node.js hiện tại (hack đơn giản, chưa hoàn chỉnh)

Vấn đề: phần `src/server/` hiện tại chỉ là một wrapper mỏng, load toàn bộ `src/main/index.ts` thông qua mock Electron. Đây là giải pháp **tạm thời** và có các rủi ro:
- Khó bảo trì lâu dài
- Gây conflict nặng nếu merge code từ upstream
- Không có cơ chế rõ ràng để phân biệt code "Electron-only" vs "Node.js-compatible"
- Frontend (React) vẫn dùng `window.electron` APIs thay vì HTTP/WebSocket

---

## Chiến lược

Áp dụng **Adapter Pattern + Build Target Isolation**:
1. Giữ nguyên code Electron gốc — **không sửa** để tránh conflicts
2. Định nghĩa **Platform Interface** (abstraction layer) để cô lập Electron dependencies
3. Cung cấp hai implementations: `ElectronAdapter` (giữ nguyên) và `NodeAdapter` (mới)
4. Frontend dùng WebSocket RPC cho web mode thay vì IPC bridge
5. Build system phân tách rõ ràng thành 3 targets: `electron`, `node-server`, `web`

---

## Danh sách Change Requests

| CR | Tên | Mức độ ưu tiên | Phụ thuộc |
|----|-----|----------------|-----------|
| [CR-001](./CR-001-platform-interface.md) | Platform Interface & Adapter Layer | 🔴 Critical | — |
| [CR-002](./CR-002-node-adapter.md) | Node.js Adapter Implementation | 🔴 Critical | CR-001 |
| [CR-003](./CR-003-ipc-abstraction.md) | IPC → RPC Transport Abstraction | 🟠 High | CR-001 |
| [CR-004](./CR-004-web-entry.md) | Web Frontend — HTTP/WebSocket Mode | 🟠 High | CR-003 |
| [CR-005](./CR-005-build-system.md) | Build System Separation | 🟡 Medium | CR-002, CR-004 |
| [CR-006](./CR-006-docker-deployment.md) | Docker & Deployment Restructure | 🟡 Medium | CR-005 |
| [CR-007](./CR-007-electron-mock-cleanup.md) | Electron Mock Consolidation & Cleanup | 🟢 Low | CR-002 |

---

## Nguyên tắc thiết kế

1. **Zero upstream conflict**: Tất cả code Electron gốc trong `src/main/` **không được sửa đổi** (chỉ thêm file mới).
2. **Additive only**: Thêm các file và module mới, không xóa hay restructure file hiện có.
3. **Graceful degradation**: Electron mode chạy y chang như trước; Node mode dùng adapters.
4. **Type safety**: Interface phải có TypeScript declarations đầy đủ.
5. **Testability**: Mỗi adapter phải có unit tests riêng, không phụ thuộc vào Electron runtime.

---

## Timeline đề xuất

```
Phase 1 (CR-001, CR-002): ~1-2 tuần — Foundation
Phase 2 (CR-003, CR-004): ~2-3 tuần — Transport & Frontend
Phase 3 (CR-005, CR-006): ~1 tuần  — Build & Deploy
Phase 4 (CR-007):          ~1 tuần  — Cleanup
```
