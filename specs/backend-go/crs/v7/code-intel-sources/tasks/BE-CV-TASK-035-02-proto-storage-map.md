# BE-CV-TASK-035-02: Proto `codeintel_storage.proto` và dòng RPC `GetStorageMap`

**From Solution:** BE-CV-SOL-035-storage-map
**Priority:** P2
**Service:** `code-intel-service` · `proto`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_storage.proto` (mới); `backend-go/proto/orca/codeintel/v1/codeintel.proto` (sửa: thêm một dòng `rpc`); `backend-go/proto/orca/codeintel/v1/codeintel_common.proto` (sửa nhỏ, chỉ khi `ViewKind` chưa có `VIEW_KIND_STORAGE`; thuộc `BE-CV-SOL-020`, cần chủ solution duyệt)
**Depends on:** BE-CV-SOL-010 (có `codeintel.proto`), BE-CV-SOL-020 (có `codeintel_common.proto`: `SourceRef`, `ServiceRef`, `WorktreeSelector`, `ResultMeta`)
**Status:** [x] DONE

---

## Context

Hợp đồng §2.1 dòng 13 giao `codeintel_storage.proto` cho CR-035; PQ-07 cấm tên khác; PQ-29 cố định `SourceRef`/`ServiceRef`. CR không ghi số field nên chủ sở hữu CR gán theo thứ tự khai báo (hợp đồng §2, quy tắc số field). Chạy `ls backend-go/proto/orca/codeintel/v1` trước khi viết để chắc tên tệp chưa bị PR khác chiếm.

## Việc cần làm

1. Tạo `codeintel_storage.proto`, `package orca.codeintel.v1`, `import "orca/codeintel/v1/codeintel_common.proto";`, đúng khối message ở solution 035 mục 2.B: `StorageMap`, `Store`, `Binding`, `Topic`, `GetStorageMapRequest{selector=1, env_filter=2, include_legacy=3}`, `GetStorageMapResponse{map=1, meta=2}`. Số field như solution (đã gán, không đổi về sau).
2. Thêm field (mới, additive) `Binding.database = 9` và `StorageMap.warnings = 7`; ghi trong comment proto rằng hai field này chờ chủ hợp đồng thêm vào §4.4 (solution 035 mục 4, L3, L7).
3. `change` ở `Store` (12) và `Binding` (8): comment "dành sẵn, v7 không điền".
4. Thêm `rpc GetStorageMap(GetStorageMapRequest) returns (GetStorageMapResponse);` vào `service CodeIntelService` trong `codeintel.proto`. **Không** khai báo RPC nào khác.
5. Nếu `ViewKind` chưa có giá trị cho `storage`, thêm `VIEW_KIND_STORAGE` (số kế tiếp, theo `BE-CV-SOL-020`); `_UNSPECIFIED = 0` đã có.
6. Chạy sinh stub theo quy trình của `BE-CV-SOL-010` (`buf generate`), commit stub nếu repo commit stub Go ở `backend-go/proto/gen/go/orca/codeintel/v1`.
7. Đối chiếu bảng hợp đồng §3.1: dòng `GetStorageMap` (`env_filter, include_legacy`) khớp; nếu khác phải sửa hợp đồng trước (PR riêng).

## Kiểm thử

- `cd backend-go/proto && buf lint` và `buf breaking --against '.git#branch=main,subdir=backend-go/proto'` (gọi `buf` trực tiếp, không `make proto-lint` vì có `|| true`).
- Unit round-trip: `proto.Marshal`/`Unmarshal` một `StorageMap` có `warnings`, `Binding.database`, Topic có `ServiceRef` ở `internal/adapter/grpc/storage_map_proto_test.go` (mới).

## Tiêu chí hoàn thành

- [x] `buf lint`, `buf breaking` xanh.
- [x] Không còn RPC nào khai báo mà thiếu message.
- [x] Số field khớp solution; tên file khớp PQ-07.
- [x] Hai field đề xuất (7, 9) có ghi chú chờ hợp đồng.

## Rủi ro và lưu ý

- `ServiceRef` trùng tên với kiểu ở `infrafleet` nếu import nhầm; dùng alias import Go `codeintelv1` (hợp đồng §2.2).
- Nếu chủ hợp đồng từ chối `Binding.database`, lens Lưu trữ mất liên kết service↔DB; phương án dự phòng: nhồi vào `via` (xấu), nên chờ quyết định.
