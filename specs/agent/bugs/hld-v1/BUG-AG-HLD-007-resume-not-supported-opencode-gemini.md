# BUG-AG-HLD-007 — Resume session không hoạt động cho OpenCode và Gemini dù được quảng cáo hỗ trợ

**Mức độ:** 🟡 Medium
**Status:** 🔴 Open
**Module:** `agent/src/relay/agent-spawner.ts` (`AGENT_SPECS`)
**Phát hiện:** 2026-08-08 (audit `agent/` code vs thiết kế — mảng "RPC Dispatch & Agent Lifecycle")

---

## Mô tả

Bảng "Session Resume Support by Agent" (`docs/logic/agent-orchestration/BL-AG-03-resume-session.md:81-89`) ghi:

| Agent | Design |
|---|---|
| Claude | ✅ Full — `--resume <id>` |
| Codex | ✅ Full — session file |
| OpenCode | ✅ Full — `resume <id>` |
| Gemini | ⚠️ Partial |

Đối chiếu `AGENT_SPECS` (`agent/src/relay/agent-spawner.ts:119-142`):

- Claude: `req?.resumeId ? ['--resume', req.resumeId] : [...]` — ✅ khớp.
- Codex: `['--session-file', \`~/.codex/${req.resumeId}.json\`]` — ✅ khớp.
- **OpenCode**: `buildArgs: () => []` — luôn trả mảng rỗng, **bỏ qua `resumeId` hoàn toàn** (dòng 139).
- **Gemini**: `buildArgs: () => ['--stream']` — cố định, **không dùng `req.resumeId`** (dòng 137).

## Hậu quả

- Khi user chọn "Resume session" cho một agent chạy trên OpenCode hoặc Gemini, agent sẽ **âm thầm khởi động phiên mới** thay vì tiếp tục phiên cũ — mất toàn bộ context hội thoại trước đó mà UI không có cảnh báo gì (vì RPC vẫn trả thành công, chỉ là hành vi bên trong sai).
- Đây là sai lệch hành vi thật, gây trải nghiệm người dùng tệ (tưởng đang resume nhưng thực chất bắt đầu lại).

## Bằng chứng

```
docs/logic/agent-orchestration/BL-AG-03-resume-session.md:81-89 → bảng resume support
agent/src/relay/agent-spawner.ts:119-142 → AGENT_SPECS
agent/src/relay/agent-spawner.ts:139 → OpenCode buildArgs luôn trả []
agent/src/relay/agent-spawner.ts:137 → Gemini buildArgs cố định ['--stream'], bỏ qua resumeId
```

## Đề xuất fix

- Nếu OpenCode/Gemini CLI có hỗ trợ resume qua flag tương ứng (cần xác nhận cú pháp thật của từng CLI), implement `buildArgs(req)` đọc `req.resumeId` giống Claude/Codex.
- Nếu CLI thật sự không hỗ trợ resume, cập nhật `BL-AG-03` xuống "❌ Không hỗ trợ" và disable nút "Resume" ở UI cho 2 agent này để tránh gây hiểu nhầm.

## Tham khảo

- Audit: `audit/agent/rpc-dispatch-lifecycle-vs-design-review.md` §2.7
