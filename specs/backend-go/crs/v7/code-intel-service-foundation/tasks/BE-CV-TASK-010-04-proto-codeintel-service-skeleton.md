# BE-CV-TASK-010-04: Proto `orca.codeintel.v1`: `codeintel.proto` chỉ chứa `service`, sinh stub

**From Solution:** BE-CV-SOL-010-scaffold-code-intel-service
**Priority:** P0
**Service:** `proto`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel.proto` (mới), `backend-go/proto/gen/go/orca/codeintel/v1/*.pb.go` (sinh, commit)
**Depends on:** BE-CV-TASK-010-01 (kết luận `buf lint`)
**Status:** [ ] TODO

---

## Context

Hợp đồng §2.1 #1: `codeintel.proto` do CR-010 tạo, chỉ chứa `service CodeIntelService`; mọi CR thêm dòng `rpc`; không file nào khác import `codeintel.proto`. §2: lint `STANDARD`, breaking `FILE` (`proto/buf.yaml`); quy tắc "RPC chưa có message thì không khai báo". Mẫu `go_package`: `proto/orca/notification/v1/notification.proto`.

## Việc cần làm

1. Tạo `codeintel.proto`: `syntax = "proto3"; package orca.codeintel.v1; option go_package = "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1;codeintelv1"; service CodeIntelService {}` kèm chú thích: danh sách RPC ở `CONTRACT-codeintel-proto-and-data-map.md` §3.1; không khai báo RPC khi chưa có message.
2. Nếu task 010-01 kết luận `buf lint` đỏ với service rỗng, áp phương án đã chọn (ghi trong PR) thay vì mặc định.
3. Sinh stub bằng `make proto-gen` (hoặc `cd backend-go/proto && buf generate`) vào `proto/gen/go/orca/codeintel/v1`; commit `.pb.go` và `_grpc.pb.go` như các package khác.
4. Không tạo file `codeintel_*.proto` nào khác (CR chủ sở hữu thêm sau; bảng §2.1).
5. Thêm test Go nhỏ trong module `proto` hoặc `code-intel-service` kiểm `CodeIntelService_ServiceDesc.ServiceName == "orca.codeintel.v1.CodeIntelService"` và `Methods` rỗng (test này đổi khi 011+ thêm RPC; viết sao cho đọc danh sách từ `ServiceDesc` để dùng lại ở SOL-013 guard test).

## Kiểm thử

- `cd backend-go/proto && buf lint --path orca/codeintel`
- `buf breaking --path orca/codeintel --against '../.git#branch=origin/main,subdir=backend-go/proto'` (bỏ qua nếu file chưa có trên `origin/main`).
- `go build ./...` trong module `proto`.

## Tiêu chí hoàn thành

- [ ] `buf lint` xanh; stub được commit; `go build` xanh.
- [ ] Không có RPC hay message thừa.

## Rủi ro và lưu ý

- Không dùng `make proto-lint` làm bằng chứng (kết thúc bằng `|| true`).
- Đổi tên package/`go_package` sau khi merge là breaking; chốt đúng ngay.
