# AG-CV-TASK-005-06: `crossCheck` bằng 3 dòng đầu của `gitnexus detect-changes`

**From Solution:** [AG-CV-SOL-005-detect-changes](../solutions/AG-CV-SOL-005-detect-changes.md) mục 2.2
**Priority:** P2
**Area:** `agent/`
**File:** `agent/src/relay/gitnexus-detect-changes-header.ts`, `gitnexus-detect-changes-header.test.ts` (mới)
**Depends on:** [AG-CV-TASK-001-05](./AG-CV-TASK-001-05-run-codeintel-tool-with-tempfile-and-output-classification.md)
**Status:** [ ] TODO

## Context
CLI chỉ in text, cắt 15 symbol/10 luồng; chỉ dùng cho `crossCheck`. Verb `detect-changes` đã ở whitelist. Phạm vi `-s compare` chưa kiểm chứng.

## Việc cần làm
1. Regex cố định `^Changes: (\d+) files?, (\d+) symbols?$`, `^Affected processes: (\d+)$`, `^Risk level: (\w+)$`; lệch -> `null` (không lỗi).
2. `riskHint {source:'gitnexus detect-changes', level, files, symbols, processes}`; so số với agent, lệch > 20% -> `crosscheck_mismatch`.

## Kiểm thử
`Changes: 446 files, 1508 symbols`, dòng lạ, rỗng, hoa/thường level.
Lệnh: `pnpm exec vitest run src/relay/gitnexus-detect-changes-header.test.ts`

## Tiêu chí hoàn thành
- [ ] Không parse danh sách symbol/luồng của CLI.

## Rủi ro
- Định dạng văn bản có thể đổi: chỉ làm `riskHint=null`.
