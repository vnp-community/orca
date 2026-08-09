# TASK-AG-HLD-012 — Implement Resume Thật Cho OpenCode/Gemini Bằng Flag Đã Verify

**Solution:** [SOL-AG-HLD-007](../solutions/SOL-AG-HLD-007-resume-opencode-gemini-verified-flags.md)  
**Bug:** [BUG-AG-HLD-007](../BUG-AG-HLD-007-resume-not-supported-opencode-gemini.md)  
**File:** `agent/src/relay/agent-spawner.ts`  
**Phụ thuộc:** —  
**Estimated:** 90 phút (bao gồm smoke test thật với binary CLI — xem Definition of Done)  
**Status:** 🟡 CODE DONE — 2026-08-09 (typecheck verified; **smoke test BẮT BUỘC với binary CLI thật CHƯA chạy được** — môi trường này không có opencode/gemini)

---

## Mục Tiêu

Implement resume thật cho OpenCode (`--session <id>`) và Gemini (`--resume <id>`) trong `AGENT_SPECS`, thay vì bỏ qua `req.resumeId` như hiện tại.

---

## Context

Đọc trước:
- `agent/src/relay/agent-spawner.ts` — `AGENT_SPECS` (dòng 119-142), đặc biệt 2 entry index 2 (gemini) và index 3 (opencode)
- `agent/src/shared/agent-session-resume.ts` — hàm `getAgentResumeArgv()` (dòng 195-219): nguồn bằng chứng flag `--resume`/`--session` đã verify, dùng thật cho luồng resume "sleeping agent" (call site khác, không phải `agent.spawn`, nhưng cùng binary CLI thật)

> [!WARNING]
> Đây KHÔNG phải bằng chứng 100% chắc chắn cho path `agent.spawn` headless-PTY trên Dev Server (khác context với sleeping-agent resume qua terminal). Đây là bằng chứng mạnh nhất hiện có trong repo, nhưng **bắt buộc phải smoke test bằng binary CLI thật trước khi merge** — xem "Definition of Done".

---

## Thay Đổi Cần Thực Hiện

**File:** `agent/src/relay/agent-spawner.ts`

Sửa 2 entry index 2 (gemini) và index 3 (opencode) trong `AGENT_SPECS`:

**TÌM:**
```ts
  // index 2: gemini
  { binary: 'gemini',   buildArgs: () => ['--stream'], apiKeyEnvVar: 'GEMINI_API_KEY' },
  // index 3: opencode — no API key needed (uses its own auth)
  { binary: 'opencode', buildArgs: () => [],            apiKeyEnvVar: null },
```

**THAY BẰNG:**
```ts
  // index 2: gemini — BUG-AG-HLD-007: resumeId giờ map sang `--resume <id>`,
  // cú pháp giống claude, verified qua getAgentResumeArgv() trong
  // ../shared/agent-session-resume.ts:206 (dùng thật cho sleeping-agent resume).
  {
    binary: 'gemini',
    buildArgs: (req) => req?.resumeId
      ? ['--resume', req.resumeId]
      : ['--stream'],
    apiKeyEnvVar: 'GEMINI_API_KEY',
  },
  // index 3: opencode — no API key needed (uses its own auth).
  // BUG-AG-HLD-007: resumeId map sang `--session <id>` — KHÔNG PHẢI `resume <id>`
  // như BL-AG-03 mô tả trước đây. Verified qua getAgentResumeArgv() trong
  // ../shared/agent-session-resume.ts:210.
  {
    binary: 'opencode',
    buildArgs: (req) => req?.resumeId
      ? ['--session', req.resumeId]
      : [],
    apiKeyEnvVar: null,
  },
```

> [!IMPORTANT]
> Không cần sửa `AgentBinarySpec.buildArgs` signature (đã nhận `req?: { resumeId?: string }`), không cần sửa `buildAgentArgs()` hay `handleAgentSpawn()` — `resumeId` đã được parse và forward sẵn (dòng 165-167, 282).

### File: `docs/logic/agent-orchestration/BL-AG-03-resume-session.md` (đồng bộ tài liệu — sau khi smoke test pass)

**TÌM** (dòng ~85-89):
```
| Agent | Resume Method | Support |
|-------|--------------|---------|
| Claude Code | `--resume <id>` | ✅ Full |
| Codex | session file | ✅ Full |
| OpenCode | `resume <id>` | ✅ Full |
| Gemini | Partial | ⚠️ Partial |
| Cursor | None | ❌ Không hỗ trợ |
```

**THAY BẰNG:**
```
| Agent | Resume Method | Support |
|-------|--------------|---------|
| Claude Code | `--resume <id>` | ✅ Full |
| Codex | session file | ✅ Full |
| OpenCode | `--session <id>` | ✅ Full |
| Gemini | `--resume <id>` | ✅ Full |
| Cursor | None | ❌ Không hỗ trợ |
```

> [!IMPORTANT]
> Chỉ áp dụng thay đổi tài liệu này **sau khi** cả 2 smoke test (gemini + opencode) pass. Nếu 1 trong 2 fail, xem "Phương Án Dự Phòng" bên dưới — KHÔNG được nâng trạng thái "✅ Full" khi chưa QA xác nhận.

---

## Phương Án Dự Phòng (Nếu Smoke Test Thất Bại)

Áp dụng **CHỈ CHO binary bị fail**, khi `gemini --resume <id>` hoặc `opencode --session <id>` được xác nhận **không hoạt động đúng** khi spawn qua PTY headless trên Dev Server:

1. Revert `buildArgs` của binary đó về trạng thái cũ (bỏ qua `resumeId`, không đổi hành vi) — KHÔNG revert binary còn lại nếu nó pass smoke test.
2. Hạ dòng tương ứng trong `docs/logic/agent-orchestration/BL-AG-03-resume-session.md` xuống "❌ Không hỗ trợ", kèm ghi chú lý do (cú pháp verified nhưng không hoạt động trong context headless PTY).
3. Disable nút "Resume" ở UI cho model tương ứng (`frontend/src/renderer/src/components/workspace/AgentPanel.tsx` hoặc tương đương trong `desktop/`) để tránh gây hiểu nhầm cho user.

---

## Verify

```bash
# 1. Unit tests mới trong agent/src/relay/__tests__/agent-spawner.test.ts:
#    - resolveAgentSpec('gemini').buildArgs({resumeId: 'abc'}) === ['--resume', 'abc']
#    - resolveAgentSpec('gemini').buildArgs(undefined) === ['--stream']
#    - resolveAgentSpec('opencode').buildArgs({resumeId: 'abc'}) === ['--session', 'abc']
#    - resolveAgentSpec('opencode').buildArgs(undefined) === []
cd agent && npx vitest run src/relay/__tests__/agent-spawner.test.ts

# 2. Smoke test THẬT (bắt buộc trước khi merge/đổi Status → DONE):
#    a. Spawn agent.spawn model=gemini, lấy resumeId từ 1 session cũ (có sẵn
#       trong ~/.gemini hoặc tương đương) → xác nhận CLI thực sự nối tiếp
#       context cũ (không phải phiên mới).
#    b. Spawn agent.spawn model=opencode, tương tự với --session <id>.
#    Nếu 1 trong 2 fail → áp dụng "Phương Án Dự Phòng" cho binary đó.
```

---

## Definition of Done

- [ ] `AGENT_SPECS` entry `gemini` map `resumeId` → `['--resume', resumeId]`
- [ ] `AGENT_SPECS` entry `opencode` map `resumeId` → `['--session', resumeId]`
- [ ] Không resumeId → giữ nguyên args cũ (`['--stream']` cho gemini, `[]` cho opencode) — không regression cho spawn mới
- [ ] Unit tests mới pass (`cd agent && npx vitest run src/relay/__tests__/agent-spawner.test.ts`)
- [ ] **BẮT BUỘC**: smoke test với binary CLI thật (`gemini --resume <id>` và `opencode --session <id>`) đã chạy và xác nhận PASS trước khi merge — không dựa vào unit test vì hành vi phụ thuộc CLI thật
- [ ] Nếu smoke test fail cho 1 trong 2 binary: đã áp dụng "Phương Án Dự Phòng" (revert buildArgs của binary đó + hạ trạng thái tài liệu + disable nút Resume UI cho model đó) — chỉ merge phần binary pass
- [ ] `docs/logic/agent-orchestration/BL-AG-03-resume-session.md` bảng "Session Resume Support by Agent" đã cập nhật đúng cú pháp, CHỈ SAU KHI smoke test tương ứng pass
