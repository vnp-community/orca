# BE-CV-TASK-037-08: Handler gRPC `ListFindings`/`DismissFinding`, đăng ký và bộ test đầu-cuối

**From Solution:** BE-CV-SOL-037-structure-findings-and-dismissals
**Priority:** P1
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/grpc/findings_handler.go`, `findings_handler_test.go`; `cmd/server/main.go` (sửa: đăng ký, nối bốn bộ phát hiện); `testdata/findings/golden/list_findings.json` (mới)
**Depends on:** BE-CV-TASK-037-06, BE-CV-TASK-037-07
**Status:** [x] DONE

---

## Context

Ánh xạ domain ⇄ proto, mã lỗi (PQ-02: `CODEINTEL_X: …` đầu message, hậu tố JSON ≤ 2 KiB), `ResultMeta` phẳng, kích thước ≤ 2 MiB.

## Việc cần làm

1. `findings_handler.go`: `ListFindings`, `DismissFinding`; `scope` UNSPECIFIED ⇒ `CHANGED`; `page_size` ngoài 1..200 ⇒ `CODEINTEL_INVALID_PARAMS` (không kẹp ngầm); `Finding.dismissed` điền khi có; chuỗi `title_key`/`params` giữ nguyên (backend không dựng câu chữ, H6); `proto.Size ≤ 2 MiB` (vượt ⇒ giảm trang, không `RESPONSE_TOO_LARGE` nếu còn giảm được).
2. `main.go`: khởi tạo bộ phát hiện (`layer`, `cycles`, `hotspot`, `deadexport`) và repository theo dialect (`switch caps.Dialect`); đăng ký handler.
3. Golden: `ListFindings` trên fixture TASK-037-01 ⇒ `list_findings.json`.
4. Đối chiếu bảng hợp đồng §3.1: dòng `ListFindings`, `DismissFinding` khớp (`review_write` cho dismiss).

## Kiểm thử

- `go test ./services/code-intel-service/internal/adapter/grpc/ -run Findings`: trường hợp lỗi (`DISABLED`, `NOT_AUTHORIZED`, `INVALID_PARAMS`, `TIMEOUT` có hậu tố), ánh xạ enum, golden, phiên thiết bị (`Identity.DeviceID != ""`) ⇒ `CODEINTEL_NOT_AUTHORIZED`.
- Đầu-cuối (fake agent + DB testcontainers hai dialect): dismiss ⇒ list ẩn ⇒ restore ⇒ list hiện; số `dismissed_count`.
- `buf lint/breaking` xanh.

## Tiêu chí hoàn thành

- [x] Toàn bộ §9 solution 037 đạt trên fixture.
- [x] Kênh `codeIntel.findings`/`codeIntel.dismissFinding` chưa đăng ký ở gateway (việc của `BE-CV-SOL-040-*`).

## Rủi ro và lưu ý

- Chưa có agent thật; chuyển sang tệp vàng G1/agent thật khi `AG-CV-SOL-037-structural-facts` xong và chạy lại golden.
