# BE-CV-TASK-031-07: Proto `codeintel_erd.proto` và RPC `GetErd`

**From Solution:** BE-CV-SOL-031-erd-model-and-access-scan
**Priority:** P0
**Service:** `proto` / `code-intel-service`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_erd.proto` (mới); `backend-go/proto/orca/codeintel/v1/codeintel.proto` (thêm một dòng `rpc GetErd`); stub sinh vào `backend-go/proto/gen/go/orca/codeintel/v1/`
**Depends on:** `codeintel_common.proto` (G0, BE-CV-SOL-020) và `codeintel.proto` có service rỗng-có-health (BE-CV-SOL-010)
**Status:** [x] DONE

---

## Context

SOL-031-erd mục 2.A. `buf.yaml`: lint `STANDARD`, breaking `FILE`. Hợp đồng PQ-07 chốt tên file; PQ-04 chốt `selector`; số field đã ghi trong CR là chuẩn.

## Việc cần làm

1. Re-verify nhanh: `ls backend-go/proto/orca/codeintel/v1/` — nếu `codeintel_common.proto` hoặc `codeintel.proto` chưa có, **dừng** (phụ thuộc G0). Đọc `codeintel_common.proto` để lấy đúng tên `WorktreeSelector`, `ResultMeta`, `SymbolRef`.
2. Tạo `codeintel_erd.proto` (package `orca.codeintel.v1`, `go_package` theo mẫu `github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1;codeintelv1`): `GetErdRequest` (selector=1 … if_none_match=8), `GetErdResponse` (model=1, meta=2, warnings=3, services=4), `ErdModel` (1–7 theo CR + `changes=8`), `ErdServiceInfo`, `ErdTable`(1–14), `ErdColumn`(1–9), `ErdIndex`(1–6), `ErdCheck`, `ErdPolicy`, `ErdRelation`(1–9), `ErdEndpoint`(1–3), `ErdExternalRef`, `ErdChange`, `ErdColumnShape`, `TableAccess`, `ParseWarning`.
3. Thêm `rpc GetErd(GetErdRequest) returns (GetErdResponse);` vào `service CodeIntelService` (`codeintel.proto`). Không khai báo RPC khác chưa có message (S8 của README feature).
4. Chạy `buf generate` (theo `buf.gen.yaml`) sinh stub; commit stub nếu repo commit stub (kiểm `git ls-files backend-go/proto/gen | head`).
5. Cập nhật bảng RPC ở `CONTRACT-codeintel-proto-and-data-map.md` §3.1 **chỉ nếu** thêm field/RPC ngoài bảng (PR hợp đồng riêng, §8.3-2) — task này không sửa file hợp đồng; nếu cần (G1: `services[]`), mở PR hợp đồng trước.

## Kiểm thử

- `cd backend-go/proto && buf lint` (chưa chạy).
- `cd backend-go/proto && buf breaking --against '.git#branch=main,subdir=backend-go/proto'` (CI gọi trực tiếp; `make proto-lint` có `|| true` nên không đủ).
- `go build ./proto/...` và test biên dịch: `GetErdRequest{}` có `GetSelector()`.
- Test phản chiếu: `protoreflect` liệt kê tên message trong package, khẳng định không trùng với file khác (PQ-29): `go test ./proto/... -run TestNoDuplicateMessageNames` (nếu test đó do SOL-020 thêm).

## Tiêu chí hoàn thành

- [x] `buf lint` và `buf breaking` xanh.
- [x] Mọi message tiền tố `Erd*` (trừ `TableAccess`, `ParseWarning` theo PQ-29).
- [x] RPC `GetErd` có trong service duy nhất `CodeIntelService`.
- [x] Không có `repo_binding_id` trong request.

## Rủi ro và lưu ý

- Số field `ErdChange`, `services=4`, `if_none_match=8` là đề xuất; nếu chủ sở hữu CR-031 chốt khác thì đổi **trước** merge (sau merge `buf breaking` khoá).
- `percent`/`ResultMeta` không liên quan; không đưa `dev_server_id` ra response (PQ-12: không ra UI).
