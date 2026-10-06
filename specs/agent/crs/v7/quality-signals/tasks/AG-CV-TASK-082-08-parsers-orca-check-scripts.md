# AG-CV-TASK-082-08: Parser đầu ra 3 script `check-*` (max-lines, styled-scrollbars, reliability-gates)

**From Solution:** [AG-CV-SOL-082-quality-parsers-and-fingerprint](../solutions/AG-CV-SOL-082-quality-parsers-and-fingerprint.md) mục 5.4
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-parser-orca-check.ts` (mới), `.test.ts` (mới)
**Depends on:** AG-CV-TASK-082-02, 082-04
**Status:** [ ] TODO

## Context

Đã đọc mã: `::error::New max-lines bypass not allowed: <entry>` và `::error::Stale max-lines baseline entry (prune it): <entry>`; `check-styled-scrollbars`: 4 dòng tiêu đề rồi `<path>:<line>:<col> <text>`; `check-reliability-gates`: `Reliability gate manifest check failed with N issue(s):` rồi `- <gateId>: <msg>`. Script `--init`/`--prune` GHI baseline: argv catalog không bao giờ có chúng.

## Việc cần làm

1. `orca-check@max-lines`: `<entry>` bắt đầu `inline ` → `file=<đường dẫn>`; mục `mobile-config` → `file:"mobile/.oxlintrc.json"`; `kind=new-bypass` (error) / `stale-baseline` (warning); `ruleId=orca-check/max-lines-ratchet/<kind>`; `anchorOverride=<entry>`.
2. `orca-check@styled-scrollbars`: bỏ 4 dòng tiêu đề; file nối `desktop/`; `ruleId=orca-check/styled-scrollbars/unstyled`.
3. `orca-check@reliability-gates`: `file:"desktop/config/reliability-gates.jsonc"`, `anchorOverride=<gateId>`, `ruleId=orca-check/reliability-gates/invalid-manifest`; thành công (`... check passed for N gate(s).`) → 0 phát hiện, `passed`.
4. Phát hiện ở mức repo khi không phân tích được dòng nhưng thoát ≠ 0: `failure: format_drift`.

## Kiểm thử

Chuỗi đầu ra mẫu dựng theo mã script (đã đọc) + fixture thật từ task 01 khi có: new-bypass, stale, mobile entry, scrollbar nhiều dòng, gate id, thành công, rác. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-parser-orca-check.test.ts`.

## Tiêu chí hoàn thành

- [ ] Không bao giờ tạo phát hiện từ dòng khung; thoát 1 mà 0 phát hiện → drift.

## Rủi ro

Hai bản `max-lines-baseline.txt` (gốc và `desktop/`): profile chạy ở `.` (README v7 điểm 20).
