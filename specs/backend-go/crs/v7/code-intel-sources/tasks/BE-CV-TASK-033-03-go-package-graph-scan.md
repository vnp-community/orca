# BE-CV-TASK-033-03: Quét package Go: doc, symbolCount, import nội bộ, tập phương thức cổng

**From Solution:** BE-CV-SOL-033-c4-component-view
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/gopackagegraph/package_scan.go`, `import_graph.go`, `port_implementations.go`, `package_doc.go` (mới) và `_test.go`
**Depends on:** BE-CV-TASK-033-01
**Status:** [x] DONE

---

## Context

Solution mục 2.C. Dùng `go/parser` thư viện chuẩn (`ImportsOnly` cho `uses`; parse đầy đủ chỉ file cần: có `_ usecase.`, interface ở `usecase`, `main.go`).

## Việc cần làm

1. `package_scan.go`: cho cây `internal/**`,`cmd/**` (đọc qua cổng SOL-030) → `PackageInfo{Path, Doc, SymbolCount, Imports}`; `symbolCount` đếm hàm/phương thức/kiểu không-test.
2. `package_doc.go`: câu đầu của `// Package …` (≤ 200 ký tự), cắt thông minh.
3. `import_graph.go`: tập import mỗi file gộp thành cạnh package→package nội bộ; bỏ `cmd/server`.
4. `port_implementations.go`: (a) dòng `var _ usecase.X = (*T)(nil)` hoặc `_ usecase.X = …` trong khối `var (`; (b) khớp **toàn bộ** tên phương thức interface `X` ở `usecase` với phương thức kiểu `T` trong adapter (conf 0,6); (c) tham số khởi tạo ở `main.go` chỉ làm bằng chứng phụ.
5. Chịu lỗi parse từng file (`GO_PARSE_ERROR`).

## Kiểm thử

`go test ./services/code-intel-service/internal/adapter/gopackagegraph/...` (chưa chạy); fixture `mini-service`; thiếu một phương thức → không khớp; interface nhúng interface khác (xử lý một cấp hoặc cảnh báo).

## Tiêu chí hoàn thành

- [x] Số `implements` bằng chứng mạnh = số dòng do `grep` độc lập.
- [x] Không panic với file hỏng.

## Rủi ro và lưu ý

- Interface nhúng nhiều cấp có thể thiếu phương thức: hạ conf, không bịa.
