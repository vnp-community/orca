# AG-CV-TASK-083-02: Parse profile Go, ánh xạ import path → đường dẫn repo, loại trừ

**From Solution:** [AG-CV-SOL-083-coverage-collection](../solutions/AG-CV-SOL-083-coverage-collection.md) mục 5.1
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-coverage-go-profile.ts` (mới), `.test.ts` (mới)
**Depends on:** AG-CV-TASK-083-01
**Status:** [x] DONE

## Context

CR-083 2.3. Đọc theo luồng (`readline`); hợp nhất khối trùng theo `(file,start,end)`.

## Việc cần làm

1. `parseGoCoverProfile(stream, ctx: { modules: { dir: string; modulePath: string }[]; exclude: ExcludeRule[] }) → { blocks: CoverBlock[]; unmappedBlocks: number; mode: "set"|"count"|"atomic" }` với dòng `<importPath>/<file>.go:<sl>.<sc>,<el>.<ec> <numStmt> <count>`.
2. `readGoWorkModules(root)`: parse khối `use (...)` và `use ./x` của `backend-go/go.work`; mỗi module: `module <path>` từ `go.mod` (đọc tệp); đường dẫn nằm trong `backend-go` (realpath).
3. Ánh xạ: cắt tiền tố module path, ghép thư mục module → tương đối gốc repo; không khớp → `unmappedBlocks`.
4. `COVERAGE_EXCLUDE_DEFAULTS`: `*.pb.go`, `*_grpc.pb.go`, đoạn `/gen/`, `*_test.go`, `testutil`, `usecasetest/` (đối chiếu CR-036, câu hỏi mở 3).
5. `parseGoCoverFunc(text) → {file,line,name,pct}[]` (nếu task 01 xác nhận định dạng).
6. Tổng hợp theo tệp `{stmts, coveredStmts, pct}` (pct là phân số 0..1) và theo module.

## Kiểm thử

Fixture task 01 + ca tổng hợp: khối lồng, nhiều dòng, trùng giữa gói, `mode: count`, tệp ngoài module, `-func` có hàm trùng tên, CRLF. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-coverage-go-profile.test.ts`.

## Tiêu chí hoàn thành

- [x] Không đường dẫn tuyệt đối trong kết quả; tệp loại trừ không vào mẫu số.

## Rủi ro

Profile hàng chục MiB: kiểm bộ nhớ phẳng.
