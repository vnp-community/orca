# AG-CV-TASK-037-03: `check --cycles --json`: biến thể whitelist, chạy qua tệp tạm, parser

**From Solution:** [AG-CV-SOL-037-structural-facts](../solutions/AG-CV-SOL-037-structural-facts.md) mục 5.1,6
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/codeintel-structural-cycles.ts` (mới), `.test.ts`; sửa `codeintel-command-whitelist.ts` (AG-CV-SOL-001) + `.test.ts`
**Depends on:** AG-CV-TASK-037-01, 037-02, AG-CV-SOL-001 (runner, whitelist)
**Status:** [ ] TODO

## Context

Hợp đồng §2.4/§9.1: `check` bị cấm trong đường đọc, chỉ mở `--cycles` cho `structuralFacts`; `-r <registryPath>` đứng cuối; stdout qua tệp tạm (pipe cắt cụt nhưng exit 0).

## Việc cần làm

1. Thêm biến thể `{ tool:"gitnexus", cmd:"check-cycles", repo: string }` vào `GitNexusCommand`; hàm dựng argv trả đúng `["check","--cycles","--json","-r", registryPath]`; từ chối mọi biến thể khác của `check` (`--branch`, `--fix`...).
2. `TestWhitelistIsClosed` cập nhật: tập lệnh đọc thêm đúng `check --cycles`; `TestRepoFlagIsLast`, `TestUserValuesNeverStartWithDash` vẫn xanh.
3. `runCycles(binding)`: chạy qua `runCodeIntelTool` (tệp tạm), parse JSON `{status, cycleCount, cycles:[{files:[…]}]}`; JSON cụt/thiếu khoá → `TOOL_FAILED reason="truncated_stdout"|"unknown_shape"`; `status` ∈ `cycles_found|clean` (khác → `unknown_shape`).
4. Xuất `data`: `{kind:"cycles", cycleCount, status, cycles}` (đường dẫn tương đối gốc repo, chuẩn hoá `/`; loại chứa `..`/tuyệt đối), sắp `cycles` theo `files.join("\\n")`; lọc `pathPrefixes` (vòng chỉ giữ nếu **mọi** tệp thuộc một tiền tố — hoặc tệp đầu; chọn: giữ nếu tệp đầu thuộc tiền tố; ghi rõ trong tài liệu hàm).

## Kiểm thử

Fixture task 01 (`cycles_found`, `clean`); stdout cụt; `check` thoát ≠ 0; argv test (`-r` cuối, không biến thể khác); phân trang. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/codeintel-structural-cycles.test.ts src/relay/codeintel-command-whitelist.test.ts`.

## Tiêu chí hoàn thành

- [ ] Chỉ `check --cycles --json -r` được spawn; `analyze|clean|check` còn lại bị cấm.
- [ ] Đọc luôn từ tệp tạm.

## Rủi ro

Giới hạn/thời gian `check` chưa biết; quy tắc lọc `pathPrefixes` cho vòng là lựa chọn của task (câu hỏi mở 3 của solution).
