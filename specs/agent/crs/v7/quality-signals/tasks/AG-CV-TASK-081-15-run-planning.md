# AG-CV-TASK-081-15: Lập kế hoạch run: mở suite, thay mẫu, chọn module Go, chiến lược scope

**From Solution:** [AG-CV-SOL-081-quality-profile-catalog-and-preflight](../solutions/AG-CV-SOL-081-quality-profile-catalog-and-preflight.md) mục 5.4
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-run-planning.ts` (mới), `.test.ts` (mới)
**Depends on:** AG-CV-TASK-081-10, 081-12, 081-14, AG-CV-TASK-081-01
**Status:** [x] DONE

## Context

CR-081 2.2, 2.5; hợp đồng §5.2. Hàm thuần theo đầu vào (đọc `go.work` qua tiêm).

## Việc cần làm

1. `planRun(input): { steps: PlannedStep[]; scopeWidened: boolean; scopeFiles }`: id suite/profile → danh sách profile; profile không có ở catalog → ném `PROFILE_UNKNOWN` (`available[]`).
2. Thay mẫu: `{bin:x}` → đường dẫn đã phân giải (task 16, tiêm); `{files}`/`{files|d}`; `{tmp:n}` = `<runDir>/n`; `{base}`; `{gitCommonDir}` = `path.resolve(root, git rev-parse --git-common-dir)` đã `realpath`.
3. `scopeStrategy`: `append-files` (≤ 300; hơn → bước chạy đủ + `scopeWidened`; không có tệp khớp `fileGlobs` → bước `skipped`, `skipReason:"no_files"`); `full-run-filter`; `go-modules` (đọc khối `use (...)` và `use ./x` của `backend-go/go.work`, mỗi đường dẫn `realpath` nằm trong `backend-go`; module có tệp đổi theo tiền tố dài nhất; `common/**`/`proto/**` đổi → mọi module + `scopeWidened`); `none`.
4. `scopeArgv` theo `scope`.
5. Mỗi `PlannedStep` mang `env` từ `buildQualityChildEnv` (task 01), `timeoutMs`, `maxOutputBytes`, `heavy`, `exit`, `parserKey`.
6. Từ chối `cwd` có realpath thoát worktree; từ chối tệp chèn không hợp lệ.

## Kiểm thử

Bảng ca: `fast`/`standard` mở đúng; `append-files` 4 và 301 tệp; module Go từ `go.work` giả (21 mục); đổi `backend-go/common/x.go` → mọi module; tên tệp `-x`, symlink thoát; `cwd` symlink; không có `os.exec` nào được gọi (hàm thuần). Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-run-planning.test.ts`.

## Tiêu chí hoàn thành

- [x] Test quét mọi `PlannedStep.argv` của catalog: không token cấm.
- [x] `quality.run` tên lạ ném trước khi tạo bất kỳ `PlannedStep`.

## Rủi ro

Giá trị `{base}` nằm trong `--against` của buf (chuỗi ghép): chưa chắc buf chấp nhận `origin/main` làm `branch=`; task 11 xác nhận.
