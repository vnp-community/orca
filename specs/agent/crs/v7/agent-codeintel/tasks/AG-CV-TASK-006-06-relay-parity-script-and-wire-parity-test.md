# AG-CV-TASK-006-06: Script parity hai cây và test tương đương Part A/Part B

**From Solution:** [AG-CV-SOL-006-relay-ssh-part-b-handlers](../solutions/AG-CV-SOL-006-relay-ssh-part-b-handlers.md) mục 2.3, 5
**Priority:** P2
**Area:** `desktop/config/scripts/` và `agent/`
**File:** `desktop/config/scripts/check-codeintel-relay-parity.mjs` + `.test.mjs` (mới), `agent/src/relay/codeintel-wire-parity.test.ts` (mới)
**Depends on:** [005](./AG-CV-TASK-006-05-desktop-relay-tree-port.md)
**Status:** [x] DONE

## Context
Chưa có script đồng bộ (audit); khuôn các `check-*.mjs` + `*.test.mjs` ở `desktop/config/scripts/`. Contract §8: `JSON.stringify(result)` Part A == Part B trừ `perf`, `startedAt`.

## Việc cần làm
1. Script so khớp nội dung `codeintel-*`, `gitnexus-*`, `codegraph-*` giữa `agent/src/relay` và `desktop/src/relay` (loại khác biệt import đã khai báo); thoát 1 khi lệch; in tệp lệch.
2. `codeintel-wire-parity.test.ts`: cùng tập ca (binary giả + fixture) qua `dispatchCodeIntelRpc` và `registerCodeIntelHandlers`; so JSON và mã lỗi.
3. Nối vào lint/CI: chờ chủ chốt (Q3); ghi vào PR.

## Kiểm thử
`node desktop/config/scripts/check-codeintel-relay-parity.mjs`; test của script; `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/codeintel-wire-parity.test.ts`

## Tiêu chí hoàn thành
- [x] Lệch cố ý một tệp làm script đỏ.

## Rủi ro
- Vị trí script chưa chốt (Q3).
