# BE-CV-TASK-032-02: Mô hình miền `contract` và `codeintel_contract.proto` (chỉ message)

**From Solution:** BE-CV-SOL-032-proto-and-wscompat-contract-catalog
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_contract.proto` (mới), `backend-go/services/code-intel-service/internal/domain/contract/catalog.go` (mới), `.../catalog_test.go`
**Depends on:** BE-CV-TASK-032-01; `codeintel_common.proto` (BE-CV-SOL-020)
**Status:** [ ] TODO

---

## Context

PQ-07/PQ-29: message nội bộ, không RPC; số field do chủ sở hữu CR-032 gán (CR không ghi số).

## Việc cần làm

1. Proto: `ServiceContract`, `RpcContract`, `RpcEdge`, `WsChannel`, `WsTarget`, `ContractWarning`, `ContractCatalog`, `MessageShape`/`FieldShape`; theo thứ tự khai báo trong CR 2.2 gán số 1..n; **không** khai báo `service`/`rpc`.
2. Miền Go tương ứng (CR 2.2) + hàm `Merge(edges)` gộp theo `(callerSymbol, serviceName, rpc)`, giữ `confidence` lớn nhất, nối `evidence`.
3. Hằng mã cảnh báo: `RPC_UNIMPLEMENTED`, `CHANNEL_DUPLICATE`, `CHANNEL_DYNAMIC`, `CLIENT_UNRESOLVED`, `PROTO_PARSE_ERROR`.
4. `buf generate`.

## Kiểm thử

`cd backend-go/proto && buf lint && buf breaking --against '.git#branch=main,subdir=backend-go/proto'`; `go test ./services/code-intel-service/internal/domain/contract/...` (chưa chạy): `Merge` giữ confidence cao nhất, evidence không trùng.

## Tiêu chí hoàn thành

- [ ] `buf` xanh; không RPC trong file.
- [ ] `Merge` có test.

## Rủi ro và lưu ý

- Tên message có tiền tố rõ để không trùng package (`Contract*` đã nằm ngoài danh sách PQ-29; kiểm trùng bằng test tên message).
