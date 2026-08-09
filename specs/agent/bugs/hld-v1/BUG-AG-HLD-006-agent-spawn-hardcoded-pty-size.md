# BUG-AG-HLD-006 — `agent.spawn` không nhận `cols`/`rows` từ caller, luôn hardcode 220×50

**Mức độ:** 🟡 Medium
**Status:** 🔴 Open
**Module:** `agent/src/relay/agent-spawner.ts`
**Phát hiện:** 2026-08-08 (audit `agent/` code vs thiết kế — mảng "RPC Dispatch & Agent Lifecycle")

---

## Mô tả

Thiết kế BL-AG-01 (`docs/logic/agent-orchestration/BL-AG-01-khoi-dong-agent.md:131-138`) kỳ vọng `agent.spawn` nhận `cols`/`rows` từ caller để PTY khớp kích thước terminal thật của user.

Code thật: PTY tạo bởi `agent.spawn` luôn dùng kích thước hardcode:

```ts
// agent-spawner.ts:389
node-pty.spawn(spec.binary, args, { cwd, env, cols: 220, rows: 50 })
```

Params `agent.spawn` nhận thực tế (`agent-spawner.ts:261-296`) chỉ có `{ taskId, userId, modelId/model, accountId, cwd, resumeId, worktreePath, branchName }` — không có `cols`/`rows`.

## Hậu quả

- Output của AI agent CLI (đặc biệt các agent có UI tương tác dùng full-screen redraw, như Claude Code TUI) có thể bị wrap/render sai nếu terminal thật của user không phải 220×50 — đặc biệt rõ trên màn hình nhỏ hoặc panel chia đôi/ba trong UI Orca.
- Không có cách nào để client điều chỉnh kích thước PTY sau khi spawn (không thấy RPC `agent.resize` tương ứng trong dispatch).

## Bằng chứng

```
docs/logic/agent-orchestration/BL-AG-01-khoi-dong-agent.md:131-138 → kỳ vọng nhận cols/rows
agent/src/relay/agent-spawner.ts:261-296 → params agent.spawn thật, không có cols/rows
agent/src/relay/agent-spawner.ts:389 → node-pty.spawn(..., {cols: 220, rows: 50}) hardcode
```

## Đề xuất fix

Thêm `cols`/`rows` (optional, có default hợp lý) vào params `agent.spawn`, truyền xuống `node-pty.spawn()`. Cân nhắc thêm RPC `agent.resize` để đồng bộ khi user resize panel sau khi agent đã spawn.

## Tham khảo

- Audit: `audit/agent/rpc-dispatch-lifecycle-vs-design-review.md` §2.5
