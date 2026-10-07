# AG-CV-TASK-082-06: Parser `go vet` (json/text) và `go test -json`

**From Solution:** [AG-CV-SOL-082-quality-parsers-and-fingerprint](../solutions/AG-CV-SOL-082-quality-parsers-and-fingerprint.md) mục 5.4
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-parser-go-vet.ts`, `quality-parser-go-test.ts` + test
**Depends on:** AG-CV-TASK-082-01, 082-02, 082-04
**Status:** [x] DONE

## Context

CR-082 2.7. Đọc theo luồng (đầu ra `go test -json` tới 64 MiB). `go vet -json` có thể thoát 0 kèm phát hiện: không áp drift guard.

## Việc cần làm

1. `govet@json`: chuỗi đối tượng JSON nối tiếp `{pkg:{analyzer:[{posn,message}]}}` lẫn dòng `# <pkg>` và lỗi biên dịch text; đọc cả stdout và stderr; `posn` tách regex từ cuối; lỗi biên dịch → `go-vet/build-failed` (typecheck); `govet@text` dự phòng (`go-vet/unknown`).
2. `gotest@json`: parse từng dòng `{Time,Action,Package,Test,Output,Elapsed,FailedBuild}`; gom `Output` theo `(Package,Test)`; `fail` có `Test` → `go-test/test-failed`; `panic:` → `go-test/panic`; timeout; build lỗi (`FailedBuild` hoặc đầu ra `file.go:l:c:` ở mức gói) → `go-test/build-failed` (typecheck); vị trí từ dòng `file_test.go:34:` đầu tiên; `testAnchor = <Package>/<Test>`.
3. Bộ nhớ: không giữ toàn bộ `Output` của test pass.

## Kiểm thử

Fixture task 01 + ca tổng hợp: 3 fail, 1 subtest, 1 panic, 1 build lỗi → 6 phát hiện; đầu ra 50 MiB tổng hợp (đo bộ nhớ, ghi vào PR); dòng không phải JSON xen kẽ; test song song xen kẽ. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-parser-go-vet.test.ts src/relay/quality-parser-go-test.test.ts`.

## Tiêu chí hoàn thành

- [x] 6 phát hiện đúng; bộ nhớ phẳng.

## Rủi ro

Độ chính xác vị trí khi `t.Parallel()`/`TestMain` in nhiều chưa đo.
