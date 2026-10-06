# AG-CV-TASK-073-01: Bộ đọc `RuntimeSwitches` (`ORCA_CODEINTEL_DISABLED`, `ORCA_CODEINTEL_REINDEX`, `ORCA_QUALITY_RUN`)

**From Solution:** [AG-CV-SOL-073-agent-kill-switch](../solutions/AG-CV-SOL-073-agent-kill-switch.md)
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel/runtime-switches.ts` + `.test.ts` (mới)
**Depends on:** không
**Status:** [ ] TODO

## Context

Agent-rpc §2.4 liệt kê ba biến; chỉ `ORCA_CODEINTEL_DISABLED` thuộc CR-073. Fail closed với giá trị lạ (solution D1). Giá trị `ORCA_CODEINTEL_REINDEX`/`ORCA_QUALITY_RUN`: chỉ `off` là tắt, giá trị lạ bị bỏ + cảnh báo (quy tắc "giá trị sai bị bỏ" §2.3).
Chưa chạy; mã dispatcher của AG-CV-SOL-001/081 chưa tồn tại: test import qua hằng đường dẫn ở đầu tệp.

## Việc cần làm

1. Cài `readRuntimeSwitches(env)` theo solution 2.2; kết quả `Object.freeze`.
2. `reindexDisabled`/`qualityDisabled` cũng true khi `codeintelDisabled`.
3. `warnings` chỉ chứa tên biến và mã (`invalid_value`), không chứa giá trị của biến khác.

## Kiểm thử

- `1/true/yes/on` (hoa thường, trim) → tắt
- vắng, rỗng, `0/false/no/off` → bật
- `maybe` → tắt + warning (fail closed)
- `ORCA_CODEINTEL_REINDEX=off` và giá trị lạ
- kết quả bị đóng băng; không đổi theo `process.env` sau khi đọc
- Test xanh.

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run <đường dẫn test>`. Chưa chạy.

## Tiêu chí hoàn thành

- [ ] Một bộ đọc duy nhất cho ba biến; 004/081 dùng nó.
- [ ] Không thêm phụ thuộc, không `max-lines` disable, tên tệp theo khái niệm.

## Rủi ro và lưu ý

- Fail closed có thể gây bất ngờ khi gõ nhầm: đã ghi cảnh báo + runbook.
