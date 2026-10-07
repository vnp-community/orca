# AG-CV-TASK-084-01: Thu bằng chứng: định dạng đầu ra, mã thoát, `cwd` của các script `check-*`/`verify-*`

**From Solution:** [AG-CV-SOL-084-convention-rule-pack](../solutions/AG-CV-SOL-084-convention-rule-pack.md) mục 4,9
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/__fixtures__/quality-rules/` (mới), `agent/scripts/capture-rule-script-evidence.mjs` (mới), `agent/src/relay/quality-rule-fixture-contract.test.ts` (mới)
**Depends on:** AG-CV-TASK-082-01 (khung MANIFEST)
**Status:** [x] DONE

## Context

Spike S1 của CR-084: chỉ `check-max-lines-ratchet`, `check-styled-scrollbars`, `check-reliability-gates` được đọc mã; `verify-localization-catalog`, `audit-localization-coverage --check`, `check-feature-wall-assets` chưa biết đầu ra. Chạy **trên bản sao** của repo (thư mục tạm, `git worktree` hoặc `cp`), nhánh sạch; các script này chỉ đọc (xác nhận bằng đọc mã trước khi chạy; **không** chạy `--init`/`--prune`).

## Việc cần làm

1. Đọc mã từng script, liệt kê đối số và nơi ghi tệp; chỉ chạy script xác nhận chỉ đọc.
2. Script chụp: chạy từng script với `cwd` đúng (gốc hoặc `desktop`), ghi `exitCode`, thời gian, đầu ra (cắt 20 KiB, che đường dẫn) vào `__fixtures__/quality-rules/<script>/<commit-script-sha>/`.
3. Ghi vào PR: `cwd` cần thiết, phụ thuộc (module cần có), mã thoát khi lỗi, thời gian; hai bản `max-lines-baseline.txt` khác nhau thế nào và script dùng bản nào (`BASELINE_PATH` tương đối `cwd`).
4. Test hợp đồng: băm `MANIFEST.json`, không đường dẫn tuyệt đối.

## Kiểm thử

Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-rule-fixture-contract.test.ts`.

## Tiêu chí hoàn thành

- [x] Có fixture cho cả 6 script hoặc `BLOCKED` kèm lý do.
- [x] PR xác nhận không script nào ghi tệp ở chế độ mặc định.

## Rủi ro

Script cần TypeScript API ("TS 7 native CLI; AST consumers need legacy JS API" — chú thích trong script theo CR): có thể không chạy được độc lập; khi đó ghi `env_not_ready` làm hành vi mong đợi.
