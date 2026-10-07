# AG-CV-TASK-004-02: Lệnh reindex có kiểu và phân tích tiến độ

**From Solution:** [AG-CV-SOL-004-reindex-and-index-notifications](../solutions/AG-CV-SOL-004-reindex-and-index-notifications.md) mục 2.2, 2.5
**Priority:** P1
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-reindex-commands.ts`, `codeintel-reindex-progress.ts` và test cùng tên (mới)
**Depends on:** [AG-CV-TASK-001-03](./AG-CV-TASK-001-03-codeintel-command-whitelist-and-child-env.md)
**Status:** [x] DONE

## Context
`analyze` không `--index-only` làm bẩn `AGENTS.md`/`CLAUDE.md`, cài skill (CR-004 1.2). Định dạng tiến độ chưa kiểm chứng.

## Việc cần làm
1. `ReindexCommand = {tool:'gitnexus'|'codegraph', mode, repoRoot|projectPath}` và `buildReindexArgv(cmd)` theo bảng 2.2; env `GITNEXUS_WORKER_POOL_SIZE`.
2. `parseProgressLine(line)` -> `{message, percent|null}`: bỏ ANSI, ≤ 200 ký tự, `$HOME`->`~`, `percent` chỉ khi `/(\d{1,3})%/` trong 0..100; `createProgressLimiter()` ≤ 1/s, luôn phát khi đổi stage/state.

## Kiểm thử
Snapshot argv; không `analyze` nào thiếu `--index-only`; không `--embeddings/--skills/--name/--branch/--drop-embeddings`; quét nguồn: chuỗi `'analyze'` chỉ ở file này; dòng `42%`, `150%`, không khớp, ANSI, `$HOME`; limiter với đồng hồ giả.
Lệnh: `pnpm exec vitest run src/relay/codeintel-reindex-commands.test.ts src/relay/codeintel-reindex-progress.test.ts`

## Tiêu chí hoàn thành
- [x] `TestReindexArgvIsIndexOnly` xanh.

## Rủi ro
- Mẫu `%` là giả định; chốt ở task 09.
