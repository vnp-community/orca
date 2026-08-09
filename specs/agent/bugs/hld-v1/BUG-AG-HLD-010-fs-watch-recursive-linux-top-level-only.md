# BUG-AG-HLD-010 — `fs.watch` (nhánh WS-agent) chỉ watch top-level directory trên Linux, bỏ sót subfolder

**Mức độ:** 🟢 Low
**Status:** 🔴 Open
**Module:** `agent/src/relay/fs-agent-extensions.ts` (`handleFsWatch`)
**Phát hiện:** 2026-08-08 (audit `agent/` code vs thiết kế — mảng "AI Credential/Filesystem Watcher/Telemetry")

---

## Mô tả

Nhánh `fs.watch` dùng bởi binary `agent.js` (Dev Server Agent kết nối qua WebSocket — direct/relay mode) dùng `fs.watch` built-in của Node (`import { watch as fsWatchSync } from 'node:fs'`, `fs-agent-extensions.ts:7`) với `recursive: true`.

Theo tài liệu Node.js chính thức, option `recursive` của `fs.watch` **chỉ được hỗ trợ trên macOS và Windows** — trên Linux (và một số hệ thống khác), option này bị bỏ qua và chỉ watch đúng thư mục top-level được truyền vào. Comment trong code tự ghi nhận điều này (`fs-agent-extensions.ts:619-622`).

## Hậu quả

- Trên dev server chạy **Linux** (kịch bản phổ biến nhất cho remote dev server theo AGENTS.md — "SSH Use Case", "Cross-Platform Support"), watch một worktree root sẽ **bỏ sót toàn bộ thay đổi file trong subfolder** — chỉ nhận được sự kiện `fs.changed` cho file nằm trực tiếp ở top-level của path được watch.
- Điều này vi phạm yêu cầu cross-platform của AGENTS.md ("Orca targets macOS, Linux, and Windows... Keep all platform-dependent behavior behind runtime checks") — hành vi hiện tại âm thầm khác nhau giữa các platform mà không có cảnh báo hay fallback.
- Ảnh hưởng: các tính năng dựa vào file-change real-time (file explorer auto-refresh, live diff preview) sẽ có vẻ "đứng yên" khi user sửa file trong subfolder trên dev server Linux.

## Bằng chứng

```
agent/src/relay/fs-agent-extensions.ts:7    → import { watch as fsWatchSync } from 'node:fs'
agent/src/relay/fs-agent-extensions.ts:600-636 → handleFsWatch, recursive: true
agent/src/relay/fs-agent-extensions.ts:619-622 → comment tự ghi nhận Linux chỉ watch top-level
```

## Đề xuất fix

Trên Linux, dùng cùng cơ chế `@parcel/watcher` cluster đã có sẵn trong `agent/src/main/ipc/*` (dùng cho nhánh SSH-relay `relay.ts`, đã hỗ trợ recursive đúng trên mọi platform qua native binding) thay vì `fs.watch` built-in, để đồng nhất hành vi giữa 2 transport (direct/relay-websocket vs SSH-relay) và giữa các platform.

## Tham khảo

- Audit: `audit/agent/credential-fswatch-telemetry-vs-design-review.md` §2.2
- AGENTS.md §"Cross-Platform Support"
