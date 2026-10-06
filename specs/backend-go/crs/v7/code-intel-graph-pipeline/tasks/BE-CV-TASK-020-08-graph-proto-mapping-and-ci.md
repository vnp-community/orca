# BE-CV-TASK-020-08: Ánh xạ domain ↔ proto (`graph_proto_mapping.go`) và CI proto

**From Solution:** BE-CV-SOL-020-canonical-graph-model
**Priority:** P1
**Service:** `code-intel-service` · CI
**File:** `backend-go/services/code-intel-service/internal/adapter/grpc/graph_proto_mapping.go` (mới), `graph_proto_mapping_test.go` (mới); `.github/workflows/backend-go-code-intel-service.yml` (do BE-CV-SOL-010 tạo; sửa bước `buf`)
**Depends on:** TASK-020-03, TASK-020-07
**Status:** [ ] TODO

---

## Context

Ánh xạ nằm ở adapter (CR mục 2.5), không ở domain. CI phải gọi `buf lint` và `buf breaking` **trực tiếp** vì `make proto-lint` có `|| true` (hợp đồng §2; đã đọc Makefile).

## Việc cần làm

1. `ToProtoSymbolRef/FromProtoSymbolRef`, `ToProtoResultMeta`, `ToProtoToolIndexStatus`, và các hàm cho `ModuleGraph, SymbolGraph, FlowGraph, ImpactGraph, RouteMap, ArchitectureGraph, SymbolDetail`. `Risk` domain ↔ enum; `PendingChanges` nil ↔ vắng message.
2. `ResultMeta`: `view` từ `ViewKind`, `generated_at` kiểu `Timestamp`, `dev_server_id` có trong proto nhưng test khẳng định hàm `ToWireMeta` (nếu có ở gateway) không chuyển; ở task này chỉ ghi chú.
3. Test round-trip bằng reflection: điền giá trị khác mặc định cho **mọi** field (đệ quy `protoreflect`), qua `FromProto(ToProto(x))`, so sánh `proto.Equal`.
4. CI: bước `buf lint` và `buf breaking --against '.git#branch=main,subdir=backend-go/proto'` gọi trực tiếp trong `backend-go/proto`, không dùng `make proto-lint`; kích hoạt khi đổi `backend-go/proto/orca/codeintel/**`.

## Kiểm thử

- `cd backend-go/services/code-intel-service && go test ./internal/adapter/grpc/ -run GraphProtoMapping -race`.
- `cd backend-go/proto && buf lint && buf breaking --against '.git#branch=main,subdir=backend-go/proto'`.
- Thử cố ý đổi số một field trong nhánh thử để xác nhận `buf breaking` đỏ (chỉ xác nhận cục bộ, không commit).

## Tiêu chí hoàn thành

- [ ] Round-trip không mất field cho mọi kiểu đồ thị.
- [ ] Workflow gọi `buf` trực tiếp và đỏ khi phá hợp đồng.
- [ ] Không file tên `helpers/utils/common/misc`; không `max-lines` disable.

## Rủi ro và lưu ý

- Round-trip bằng reflection đệ quy có thể chậm với `map` lồng; giới hạn độ sâu 6.
- Nếu SOL-010 chưa tạo workflow, task này chỉ chuẩn bị đoạn YAML trong PR và ghi vào mô tả.
