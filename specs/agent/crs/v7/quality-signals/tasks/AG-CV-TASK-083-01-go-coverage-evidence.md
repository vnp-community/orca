# AG-CV-TASK-083-01: Thu bằng chứng thật: định dạng `-coverprofile`, `go tool cover -func`, package không test

**From Solution:** [AG-CV-SOL-083-coverage-collection](../solutions/AG-CV-SOL-083-coverage-collection.md) mục 4,5.3
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/__fixtures__/coverage-go/` (mới), `agent/scripts/capture-go-coverage-fixtures.mjs` (mới), `agent/src/relay/quality-coverage-fixture-contract.test.ts` (mới)
**Depends on:** không
**Status:** [ ] TODO

## Context

Spike S1 của CR-083: chưa biết (a) dòng profile `mode: set`, (b) `go tool cover -func` in gì, (c) package không có test file có xuất khối `count=0` không, (d) module path vs thư mục. Chạy trên **module mẫu trong thư mục tạm**, không trên Orca.

## Việc cần làm

1. Module mẫu: 2 module trong `go.work` tạm (một có test, một không), package có hàm được test/không được test, tệp `*.pb.go` giả, tệp `gen/`.
2. Script chụp: `go test -covermode=set -coverprofile=<tmp>/<module>.out ./...` mỗi module (`GOTOOLCHAIN=local`), `go tool cover -func=<profile>`; ghi `MANIFEST.json` (`goVersion`, `argv`, băm); che đường dẫn.
3. Ghi vào PR: câu trả lời (a)-(d); thời gian và RAM trên module mẫu (số đo thật, không ngoại suy cho Orca).
4. Test hợp đồng: băm, ngân sách 20 KiB/tệp, không đường dẫn tuyệt đối.

## Kiểm thử

Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-coverage-fixture-contract.test.ts`. Chạy script thủ công một lần.

## Tiêu chí hoàn thành

- [ ] Fixture có `mode: set` thật và `-func` thật; PR trả lời (c).
- [ ] Nếu (c) = không có khối cho package không test, cập nhật solution 5.3 và task 04 trước khi viết.

## Rủi ro

Go vắng: `BLOCKED`. Đầu ra `-func` phụ thuộc phiên bản Go: ghi `goVersion`.
