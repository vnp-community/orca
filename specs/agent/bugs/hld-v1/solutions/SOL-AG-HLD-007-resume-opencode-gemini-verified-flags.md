# SOL-AG-HLD-007 — Implement Resume Thật Cho OpenCode/Gemini Bằng Flag Đã Được Verify Trong Repo

**Fixes:** [BUG-AG-HLD-007](../BUG-AG-HLD-007-resume-not-supported-opencode-gemini.md)
**TDD Ref:** [TDD-AG-12 §4 — Supported AI Agent CLIs / `AGENT_BINARY_SPECS.buildArgs`](../../../tdd/v5/12-agent-spawner.md)
**File:** `agent/src/relay/agent-spawner.ts`
**Effort:** 2-3 giờ (implement + tests + đồng bộ tài liệu BL-AG-03)
**Status:** 🔴 TODO

---

## Phân Tích

Code thật `agent/src/relay/agent-spawner.ts`, `AGENT_SPECS` (dòng 119-142):

```ts
// index 2: gemini
{ binary: 'gemini',   buildArgs: () => ['--stream'], apiKeyEnvVar: 'GEMINI_API_KEY' },
// index 3: opencode — no API key needed (uses its own auth)
{ binary: 'opencode', buildArgs: () => [],            apiKeyEnvVar: null },
```

Cả hai `buildArgs` đều cố định, bỏ qua `req.resumeId` — khớp đúng như bug report mô tả.

### Đề bài yêu cầu: không đoán bừa cú pháp CLI

Bug report đề xuất 2 hướng: (a) implement nếu biết chắc cú pháp, (b) nếu không chắc, downgrade tài liệu + disable nút Resume. **Đã tìm được bằng chứng chắc chắn trong chính codebase này** (không phải đoán) — nên chọn hướng (a).

`agent/src/shared/agent-session-resume.ts` có hàm `getAgentResumeArgv()` (dòng 195-220) — dùng thật cho luồng "resume sleeping agent" của TUI/native-chat (một call site khác, không phải `agent.spawn`, nhưng cùng gọi đúng binary CLI thật):

```ts
export function getAgentResumeArgv(
  agent: ResumableTuiAgent,
  providerSession: AgentProviderSessionMetadata
): string[] | null {
  const id = providerSession.id
  switch (agent) {
    case 'claude':
      return providerSession.key === 'session_id' ? ['claude', '--resume', id] : null
    case 'codex':
      return providerSession.key === 'session_id' ? ['codex', 'resume', id] : null
    case 'gemini':
      return providerSession.key === 'session_id' ? ['gemini', '--resume', id] : null   // ← verified
    ...
    case 'opencode':
      return providerSession.key === 'session_id' ? ['opencode', '--session', id] : null // ← verified
    ...
  }
}
```

Hai điểm quan trọng rút ra từ đây:

1. **Gemini hỗ trợ resume đầy đủ** qua flag `--resume <id>` — **giống hệt cú pháp của Claude**. Bảng BL-AG-03 hiện ghi Gemini là "⚠️ Partial" — theo bằng chứng này, sau khi implement, Gemini nên được nâng lên "✅ Full" (cần QA xác nhận trước khi đổi trạng thái tài liệu, xem mục Verification).
2. **OpenCode resume KHÔNG dùng cú pháp `resume <id>`** như BL-AG-03 hiện mô tả (`docs/logic/agent-orchestration/BL-AG-03-resume-session.md:81-89` ghi "✅ Full — `resume <id>`"). Cú pháp đã verify và đang chạy thật trong repo là **`--session <id>`**. Tài liệu BL-AG-03 bị sai cú pháp, không phải sai "có/không hỗ trợ" — cần sửa luôn dòng ví dụ lệnh trong tài liệu đó (xem "Files Liên Quan").

Vì `getAgentResumeArgv()` phục vụ một call site khác (resume phiên đã "ngủ" qua terminal, không phải RPC `agent.spawn` headless trên Dev Server), **đây vẫn không phải bằng chứng 100% chắc chắn cho path `agent.spawn`** — nhưng nó là bằng chứng mạnh nhất hiện có trong repo (flag thật, đã chạy production cho đúng 2 binary này), tốt hơn nhiều so với đoán. Do đó Verification bên dưới yêu cầu smoke test thật với binary `gemini`/`opencode` trước khi đổi Status sang DONE.

**Blast radius:** `AGENT_SPECS` risk LOW theo GitNexus (không có caller tĩnh khác ngoài `resolveAgentSpec`/`buildAgentArgs` trong cùng file). Thay đổi chỉ mở rộng 2 nhánh `buildArgs`, giữ nguyên args khi không có `resumeId` — không phá vỡ luồng spawn mới (non-resume) hiện tại.

---

## Thay Đổi Cần Thực Hiện

### File: `agent/src/relay/agent-spawner.ts`

Sửa 2 entry trong `AGENT_SPECS` (dòng 136-139):

```ts
const AGENT_SPECS: AgentBinarySpec[] = [
  // index 0: claude — (không đổi)
  { ... },
  // index 1: codex — (không đổi)
  { ... },
- // index 2: gemini
- { binary: 'gemini',   buildArgs: () => ['--stream'], apiKeyEnvVar: 'GEMINI_API_KEY' },
- // index 3: opencode — no API key needed (uses its own auth)
- { binary: 'opencode', buildArgs: () => [],            apiKeyEnvVar: null },
+ // index 2: gemini — BUG-AG-HLD-007: resumeId giờ map sang `--resume <id>`,
+ // cú pháp giống claude, verified qua getAgentResumeArgv() trong
+ // ../shared/agent-session-resume.ts:206 (dùng thật cho sleeping-agent resume).
+ {
+   binary: 'gemini',
+   buildArgs: (req) => req?.resumeId
+     ? ['--resume', req.resumeId]
+     : ['--stream'],
+   apiKeyEnvVar: 'GEMINI_API_KEY',
+ },
+ // index 3: opencode — no API key needed (uses its own auth).
+ // BUG-AG-HLD-007: resumeId map sang `--session <id>` — KHÔNG PHẢI `resume <id>`
+ // như BL-AG-03 mô tả trước đây. Verified qua getAgentResumeArgv() trong
+ // ../shared/agent-session-resume.ts:210.
+ {
+   binary: 'opencode',
+   buildArgs: (req) => req?.resumeId
+     ? ['--session', req.resumeId]
+     : [],
+   apiKeyEnvVar: null,
+ },
  // index 4: ollama — (không đổi)
  { ... },
]
```

Không cần sửa `AgentBinarySpec.buildArgs` signature (đã nhận `req?: { resumeId?: string }`), không cần sửa `buildAgentArgs()` hay `handleAgentSpawn()` — `resumeId` đã được parse và forward sẵn (dòng 165-167, 282).

### File: `docs/logic/agent-orchestration/BL-AG-03-resume-session.md` (đồng bộ tài liệu)

Sửa bảng dòng 81-89: cập nhật cú pháp OpenCode từ `resume <id>` → `--session <id>`; sau khi QA xác nhận Gemini resume hoạt động thật, đổi Gemini từ "⚠️ Partial" → "✅ Full — `--resume <id>`".

---

## Phương Án Dự Phòng (Nếu Smoke Test Thất Bại)

Nếu QA thủ công phát hiện `gemini --resume <id>` hoặc `opencode --session <id>` **không hoạt động đúng** khi spawn qua PTY headless trên Dev Server (khác context với sleeping-agent resume), áp dụng phương án an toàn hơn theo đề xuất (b) của bug report:

1. Revert `buildArgs` của binary đó về trạng thái cũ (bỏ qua `resumeId`, không đổi hành vi).
2. Hạ `docs/logic/agent-orchestration/BL-AG-03-resume-session.md` xuống "❌ Không hỗ trợ" cho binary đó, kèm ghi chú lý do (cú pháp verified nhưng không hoạt động trong context headless PTY).
3. Disable nút "Resume" ở UI cho model tương ứng (`frontend/src/renderer/src/components/workspace/AgentPanel.tsx` hoặc tương đương trong `desktop/`) để tránh gây hiểu nhầm cho user.

Không nên giữ tài liệu ở trạng thái "✅ Full" nếu chưa QA xác nhận — status DONE của solution này chỉ set sau khi cả 2 nhánh smoke test pass.

---

## Verification

```bash
# 1. Unit tests mới trong agent/src/relay/__tests__/agent-spawner.test.ts:
#    - resolveAgentSpec('gemini').buildArgs({resumeId: 'abc'}) === ['--resume', 'abc']
#    - resolveAgentSpec('gemini').buildArgs(undefined) === ['--stream']
#    - resolveAgentSpec('opencode').buildArgs({resumeId: 'abc'}) === ['--session', 'abc']
#    - resolveAgentSpec('opencode').buildArgs(undefined) === []
cd agent && npx vitest run src/relay/__tests__/agent-spawner.test.ts

# 2. Smoke test THẬT (bắt buộc trước khi đổi Status → DONE) — không thể verify
#    bằng unit test vì phụ thuộc hành vi binary CLI thật:
#    a. Spawn agent.spawn model=gemini, lấy resumeId từ 1 session cũ (có sẵn
#       trong ~/.gemini hoặc tương đương) → xác nhận CLI thực sự nối tiếp
#       context cũ (không phải phiên mới).
#    b. Tương tự cho model=opencode với --session <id>.
#    Nếu 1 trong 2 fail → áp dụng "Phương Án Dự Phòng" ở trên cho binary đó.
```

---

## Files Liên Quan

| File | Vai trò |
|------|---------|
| `agent/src/relay/agent-spawner.ts` | `AGENT_SPECS` — nơi sửa chính (2 entry: gemini, opencode) |
| `agent/src/shared/agent-session-resume.ts` | `getAgentResumeArgv()` — nguồn bằng chứng verified cho cú pháp `--resume`/`--session`, dùng cho 1 call site khác (sleeping-agent resume) |
| `docs/logic/agent-orchestration/BL-AG-03-resume-session.md` | Bảng "Session Resume Support by Agent" — cần sửa cú pháp OpenCode, cân nhắc nâng cấp Gemini sau QA |
| `desktop/src/relay/agent-spawner.ts` | Bản mirror cùng cấu trúc `AGENT_SPECS` trong package `desktop/` — cần áp cùng patch (ngoài scope, task riêng) |
| `agent/src/relay/__tests__/agent-spawner.test.ts` | Thêm test cases cho `buildArgs` của gemini/opencode |
| `frontend/src/renderer/src/components/workspace/AgentPanel.tsx` | Nơi hiển thị nút Resume ở UI — chỉ cần sửa nếu rơi vào "Phương Án Dự Phòng" |
