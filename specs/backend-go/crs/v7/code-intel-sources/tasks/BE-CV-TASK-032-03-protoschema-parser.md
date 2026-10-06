# BE-CV-TASK-032-03: Bộ phân tích `.proto` tự viết (service, rpc, stream, message, enum, oneof)

**From Solution:** BE-CV-SOL-032-proto-and-wscompat-contract-catalog
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/protoschema/tokenizer.go`, `parser.go`, `message_shape.go` (mới) và `_test.go`
**Depends on:** BE-CV-TASK-032-01, BE-CV-TASK-032-02
**Status:** [ ] TODO

---

## Context

SOL-032 mục 2.C.1. Phương án A (không dependency; O-7).

## Việc cần làm

1. Tokenizer: comment (giữ comment dẫn đầu RPC cắt 300 ký tự làm `Doc`), chuỗi, số, ký hiệu.
2. Parser đệ quy xuống: `syntax`, `package`, `import`, `option` (bỏ qua), `service`/`rpc` (`stream` hai phía), `message` lồng, `enum`, `oneof`, `repeated`/`optional`; cấu trúc lạ → `PROTO_PARSE_ERROR` + bỏ qua tới `}` cân.
3. `message_shape.go`: `MessageShape` (tên, số, kiểu, nhãn) chỉ khi `includeShapes`.
4. Đầu ra: `[]ServiceContract` + `go_package` để ánh xạ alias import ở task 04.
5. Không đệ quy vô hạn; giới hạn độ sâu lồng 16 và kích thước file 1 MiB.

## Kiểm thử

`go test ./services/code-intel-service/internal/adapter/protoschema/...` (chưa chạy): bảng ca cú pháp; chạy trên toàn bộ 18 proto thật khẳng định số `rpc` = số `grep` độc lập (548) và cờ stream; fuzz ngắn.

## Tiêu chí hoàn thành

- [ ] 18 file, 548 RPC, cờ stream khớp.
- [ ] Cú pháp lạ không panic.

## Rủi ro và lưu ý

- Không kiểm tra ngữ nghĩa (import/kiểu); kiểm chéo descriptor ở task 07.
