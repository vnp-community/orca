# BE-CV-TASK-085-02: Proto `codeintel_quality_gate.proto` (service `QualityGateService`, message cổng/profile/waiver/trend)

**From Solution:** BE-CV-SOL-085-quality-gate-evaluator-and-profiles
**Priority:** P0
**Service:** `proto`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_quality_gate.proto` (mới); stub `proto/gen/go/orca/codeintel/v1`
**Depends on:** BE-CV-SOL-010 (cổng G0: `codeintel_common.proto` có `WorktreeSelector`), BE-CV-SOL-082 (`codeintel_quality.proto`)
**Status:** [x] DONE

## Context
Hợp đồng §2.1 dòng 19 và §3.2 là danh sách chuẩn. File này chứa `service QualityGateService` **đầy đủ** (mọi RPC của §3.2) nhưng **chỉ** khai báo RPC mà message đã có: các solution 085-waivers, 089, 090, 092, 093 mỗi bên thêm dòng `rpc` của mình khi message của họ merge (quy tắc: RPC chưa có message thì không khai báo). Import: `codeintel_common`, `codeintel_quality`, và file message của từng CR.

## Việc cần làm
1. Message: `QualityGate`, `QualityGateReason` (có `category`, `tool`, `code`, `params`, `run_id`, `waived_count`; PQ-34), `QualityProfile`, `QualityProfileDefinition` (+ `when_changed_paths` repeated string, đề xuất L9), `QualityWaiver`, `QualityTrendPoint` (PQ-33, `metrics` dùng `optional`/wrapper để khoá vắng ≠ 0).
2. Request/response: `GetQualityGate`, `GetQualityProfile`, `SaveQualityProfile` (ở task này); `WaiveFinding`, `GetQualityTrend` (message ở đây, handler ở 085-08/12). `selector` là field 1 mọi request; enum có `_UNSPECIFIED=0`; chữ thường ở dạng chuỗi theo PQ-32.
3. Số field: gán theo thứ tự khai báo của CR, ghi comment "không đổi".
4. `buf lint` (STANDARD), `buf breaking` (FILE) gọi **trực tiếp**; sinh stub `buf generate`.

## Kiểm thử
- `buf lint`, `buf breaking --against '.git#branch=main,subdir=backend-go/proto'`; test Go nhỏ: marshal/unmarshal `QualityGate` fixture; `metrics` thiếu khoá ⇒ không có presence.

## Tiêu chí hoàn thành
- [x] lint+breaking xanh; [x] không message trùng tên ở package; [x] RPC chỉ khai báo khi có message.

## Rủi ro
- Không dùng `make proto-lint` (có `|| true`). Hai file proto cùng sửa service dễ xung đột merge: thống nhất thứ tự dòng `rpc` theo §3.2.
