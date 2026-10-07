# AG-CV-TASK-001-08: Phong bì kết quả, `headCommit`, `stale`, `perf`

**From Solution:** [AG-CV-SOL-001-codeintel-agent-foundation](../solutions/AG-CV-SOL-001-codeintel-agent-foundation.md) mục 2.2 (phong bì), 2.5
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-result-envelope.ts` (mới), `agent/src/relay/codeintel-head-commit.ts` (mới), và `codeintel-result-envelope.test.ts`, `codeintel-head-commit.test.ts` (mới)
**Depends on:** [06](./AG-CV-TASK-001-06-gitnexus-registry-and-repo-resolution.md) (`runGit`, binding)
**Status:** [x] DONE

## Context

Contract §2.2: kết quả luôn là **object JSON** (Go giải mã vào `map[string]any`, `client.go:444`); phong bì `{sources[], headCommit, stale, truncated, totalCount, warnings?, perf, data}`; `sources[].lineBase` luôn `1`; `commit` của CodeGraph luôn `null`; `perf` thay `toolTimingsMs` (PQ-19), `command` chỉ là hằng whitelist; giới hạn JSON 8 MiB (cắt dần mảng theo ưu tiên của method + `truncated:true`; vẫn vượt -> `OUTPUT_TOO_LARGE {bytes, limit}`). `stale` = OR các nguồn đã dùng: `indexedCommit !== headCommit` hoặc `rootMismatch/worktreeMismatch` hoặc `pendingChanges > 0`. `headCommit` = `git rev-parse HEAD`, cache 5 s, `null` nếu không đọc được (HEAD chưa sinh: `unborn`).

## Việc cần làm

1. `codeintel-head-commit.ts`: `getHeadCommit(workspaceRoot, deps)` bằng `runGit(['rev-parse','--verify','--quiet','HEAD'])`; cache 5 s theo `workspaceRoot`, `invalidateHeadCommit()`; lỗi/unborn -> `null`.
2. `codeintel-result-envelope.ts`: kiểu `CodeIntelSource = {tool, version, indexedAt, commit, lineBase: 1}`; `buildCodeIntelResult({sources, headCommit, stale, truncated, totalCount, warnings, perf, data})` trả object phong bì (bỏ `warnings` khi rỗng); `computeStale(sources, headCommit, flags)`; `PerfCollector` (`start()`, `recordCli({tool, command, ms, stdoutBytes})`, `build()` -> `{totalMs, queueWaitMs, cliCalls, cli[], parseMs, truncated}`); `fitResultToLimit(result, {prune: Array<{path, minKeep}>}, maxBytes)`: đo `Buffer.byteLength(JSON.stringify(...))`, cắt dần mảng theo thứ tự ưu tiên method cấp, đặt `truncated:true`, vẫn vượt -> `OUTPUT_TOO_LARGE`.
3. Quy ước: `perf` chỉ cho backend ghi metric/span, **không** vào cache snapshot (ghi chú ở tài liệu của kiểu); `totalCount` = tổng trước khi cắt hoặc `null`.
4. Không import transport; nhận `AbortSignal`/hạn qua tham số (không đọc `ws`).

## Kiểm thử

`codeintel-result-envelope.test.ts`: dựng phong bì đủ trường; `commit:null` cho CodeGraph; `lineBase` luôn 1; `computeStale` các tổ hợp (commit khác, mismatch, pending>0, tất cả sạch -> `false`); `fitResultToLimit` cắt mảng đến khi <= ngưỡng (ngưỡng nhỏ trong test), `truncated:true`; không cắt được -> `OUTPUT_TOO_LARGE`; `perf.cli[].command` chỉ nhận giá trị trong tập `cypher|context|impact|query|status|callers|callees|files|affected|detect-changes` (giá trị khác ném); `JSON.stringify(result)` là object (không mảng).
`codeintel-head-commit.test.ts`: repo tạm: sha 40 hex; repo chưa commit -> `null`; thư mục không phải repo -> `null`; cache 5 s (đồng hồ giả) và `invalidateHeadCommit`.

Lệnh: `pnpm exec vitest run src/relay/codeintel-result-envelope.test.ts src/relay/codeintel-head-commit.test.ts`

## Tiêu chí hoàn thành

- [x] Kết quả luôn là object; `perf` thay `toolTimingsMs`.
- [x] `stale` đúng công thức hợp đồng cho mọi tổ hợp.
- [x] Cắt 8 MiB không làm vỡ JSON (test dùng ngưỡng nhỏ).

## Rủi ro và lưu ý

- Ưu tiên cắt theo từng method do CR của method đó quyết (002, 003, 005); hàm chỉ nhận danh sách.
- `warnings` là chuỗi mã ổn định (không tiếng Việt): backend/FE dịch.
