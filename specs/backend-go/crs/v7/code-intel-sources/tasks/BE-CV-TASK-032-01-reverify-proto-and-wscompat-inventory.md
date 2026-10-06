# BE-CV-TASK-032-01: Re-verify kho proto, đăng ký server, kênh `wscompat` và chốt fixture

**From Solution:** BE-CV-SOL-032-proto-and-wscompat-contract-catalog
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/gocallgraph/testdata/INVENTORY.md` (mới), `.../protoschema/testdata/*.proto` (mới, bản sao rút gọn)
**Depends on:** BE-CV-TASK-030-05
**Status:** [ ] TODO

---

## Context

SOL-032 mục 1: số 464/461/455 kênh và 548 RPC chưa tách non-test. Task chốt số thật và mẫu cú pháp trước khi viết parser.

## Việc cần làm

1. Chạy chỉ-đọc: `ls backend-go/proto/orca/*/v1/*.proto | wc -l`; `grep -c '^\s*rpc ' ...`; `grep -n 'stream ' ...`; `grep -rn 'Register[A-Za-z]*Server(' backend-go/services/*/cmd/server/main.go`; đếm `r.Register*` ở `channels*.go` **không-test**; liệt kê hàm đăng ký dùng chung (`registerAccountsRelay`, `registerBrowserRelay`, ...).
2. Ghi vào `INVENTORY.md` kèm commit; liệt kê 7 RPC `Unimplemented` của `infra-fleet-service` (đối chiếu file `internal/adapter/grpc`).
3. Chép 4 proto nhỏ (`usage`, `annotation`, `notification`, một `mcp`) + một đoạn `infrafleet.proto` có `stream`, `oneof`, message lồng làm fixture parser.
4. Liệt kê cú pháp proto thật có thể làm vỡ parser tự viết (`map<`, `reserved`, `extend`, `option (` tuỳ biến): `grep`; kết quả kỳ vọng 0 theo CR.

## Kiểm thử

Không có test mã; review `INVENTORY.md`. `find` kiểm kích thước fixture < 300 KB.

## Tiêu chí hoàn thành

- [ ] Số liệu thật thay các con số CR trong tiêu chí.
- [ ] Fixture đủ `stream`, `oneof`, lồng, import nội bộ `orca/workflow/v1/workflow.proto`.

## Rủi ro và lưu ý

- Bản sao fixture phải cập nhật khi proto nguồn đổi; golden chính quét thư mục thật.
