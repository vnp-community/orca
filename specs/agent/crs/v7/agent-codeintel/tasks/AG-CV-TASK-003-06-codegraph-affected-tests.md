# AG-CV-TASK-003-06: `codegraph affected` cho test bị ảnh hưởng

**From Solution:** [AG-CV-SOL-003-codegraph-extraction](../solutions/AG-CV-SOL-003-codegraph-extraction.md) mục 2.3
**Priority:** P1
**Area:** `agent/`
**File:** `agent/src/relay/codegraph-affected-tests.ts`, `codegraph-affected-tests.test.ts` (mới)
**Depends on:** [001](./AG-CV-TASK-003-01-codegraph-cli-output-parsing.md), [AG-CV-TASK-001-04](./AG-CV-TASK-001-04-run-tool-command-options-and-legacy-tool-guard.md) (`stdinText`)
**Status:** [ ] TODO

## Context
CR-003 2.6: `codegraph affected --stdin -p <root> -j -d 5` với tệp qua stdin; ≤ 500 tệp/lần, ≤ 200 test (`truncated`). Nếu `stdinText` chưa có: đối số, nhóm 50, mỗi giá trị không bắt đầu `-`.

## Việc cần làm
1. `getAffectedTests(binding, files[], ctx)` chia nhóm, hợp nhất, khử trùng, cắt 200.
2. Verb `affected` đã ở whitelist (task 001-03).

## Kiểm thử
Chia nhóm, giới hạn, stdin vs đối số, giá trị `-x` bị từ chối.
Lệnh: `pnpm exec vitest run src/relay/codegraph-affected-tests.test.ts`

## Tiêu chí hoàn thành
- [ ] `agent/src/relay/agent-tool-registry.ts` -> ≥ 1 test (fixture).

## Rủi ro
- Hỗ trợ `--stdin` của `affected` suy ra từ CR, chưa chạy.
