# AG-CV-TASK-006-05: Cây `desktop/src/relay/`: sao lõi, vá `runToolCommand`, vá `dispatcher.ts`, nối `relay.ts`

**From Solution:** [AG-CV-SOL-006-relay-ssh-part-b-handlers](../solutions/AG-CV-SOL-006-relay-ssh-part-b-handlers.md) mục 2.3
**Priority:** P2
**Area:** `desktop/` (ngoài `agent/`, cần chủ sở hữu desktop duyệt)
**File:** `desktop/src/relay/{codeintel-*,gitnexus-*,codegraph-*,codeintel-symbol-ref*,codeintel-relay-handlers}.ts` (mới, bản sao), `desktop/src/relay/agent-tool-registry.ts` (sửa như AG-CV-TASK-001-04), `desktop/src/relay/dispatcher.ts` (sửa ≤ 5 dòng), `desktop/src/relay/relay.ts` (sửa), `desktop/src/relay/codeintel-relay-handlers.test.ts` (mới)
**Depends on:** [004](./AG-CV-TASK-006-04-wire-into-agent-relay-main.md); điều kiện: O-5
**Status:** [ ] TODO

## Context
`relay.js` Part B thật build từ `desktop/src/relay/relay.ts` (`build-relay.mjs:19`). `desktop/relay.ts` không có `registerAuthStatusHandlers`; có `loadAgentConfig()` không tham số (`desktop/src/relay/agent-config.ts:73`), `relayLogLine` (`relay.ts:65`). `desktop/src/relay/agent-tool-registry.ts` giống bản `agent/` (đã `diff` 400 dòng đầu). `shared/git-capability-cache.ts`, `git-worktree-command-capabilities.ts` đã có ở `desktop/src/shared/`.

## Việc cần làm
1. Impact `handleRequest`, `runToolCommand` (bản `desktop/`); báo blast radius.
2. Sao các file lõi (cùng PR với `agent/`); kiểm mọi import `../shared/...` có ở `desktop/src/shared/`; thiếu thì sao tối thiểu hoặc báo.
3. Áp bản vá `runToolCommand` (cùng nội dung task 001-04); bản vá `dispatcher.ts` (cùng task 006-03).
4. `relay.ts`: dựng `loadAgentConfig()` + logger `relayLogLine` (khuôn `agent/relay.ts:468-492`), gọi `registerCodeIntelHandlers` ngay trước `dispatcher.onRequest('orca.cli'…)` (`:494`; điểm cắm đề xuất, kiểm lại cấu trúc `main()`).
5. Không sửa các khác biệt có sẵn giữa hai cây.

## Kiểm thử
`desktop`: test bản sao (khuôn task 02); build relay: `node desktop/config/scripts/build-relay.mjs` (kiểm bundle sinh ra; chưa kiểm cấu hình pnpm ở `desktop/`).
Lệnh: `cd /opt/repos/orca/desktop && pnpm exec vitest run src/relay/codeintel-relay-handlers.test.ts` (chưa kiểm cấu hình vitest `desktop/`).

## Tiêu chí hoàn thành
- [ ] `desktop` build; cùng đầu vào cho cùng đầu ra như `agent/`; `error.data` đến được.

## Rủi ro
- Hai cây lệch; Node 18 ở relay từ xa (API dùng trong `codeintel-*` chưa kiểm).
