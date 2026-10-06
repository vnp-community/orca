# BE-CV-TASK-085-06: Use case `EvaluateQualityGate` và handler `GetQualityGate`

**From Solution:** BE-CV-SOL-085-quality-gate-evaluator-and-profiles
**Priority:** P0
**Service:** `code-intel-service`
**File:** `internal/usecase/evaluate_quality_gate.go`, `internal/adapter/grpc/quality_gate_server.go` (mới)
**Depends on:** BE-CV-TASK-085-04, 085-05; BE-CV-SOL-036 (overlay, qua port), 037, 082
**Status:** [ ] TODO

## Context
Chỉ đọc DB; không gọi agent, không qua `AgentCallGate`. `QualityGateServer` nhúng `UnimplementedQualityGateServiceServer`; solution khác thêm phương thức vào cùng struct. Trả `comparison: []` cho tới khi BE-086 lắp port (L8).

## Việc cần làm
1. Ghép `GateInput` từ port: `RunReader`, `FindingReader`, `ChangedFileSource`, `StructureFindingSource`, `CoverageReader`, `IndexFreshnessReader`, `WaiverReader`, `DismissalReader`; đồng hồ DB một lần.
2. `GetQualityGate`: `base_ref?` (mặc định merge-base), `profile_name?` (mặc định `default`), `include_waived_detail`; trả `gate`, `waivers≤50`, `evaluated_at`, `profile_definition_digest`, `comparison`.
3. `record`/`turn_key` nhận nhưng chưa ghi (xử lý ở 085-10).
4. Fixture JSON `QualityGate` khớp ui-api §4.7 (có/không trường tuỳ chọn).

## Kiểm thử
- Use case với port giả: thiếu overlay, thiếu coverage, run running, run sai HEAD; handler: tham số sai ⇒ `CODEINTEL_INVALID_PARAMS`; golden JSON.

## Tiêu chí hoàn thành
- [ ] không gọi agent (kiểm bằng fake đếm); [ ] fixture khớp hợp đồng; [ ] tenant khác không dùng được `selector`.

## Rủi ro
- Hình dạng thật `ChangeOverlay`/`Finding` chưa chốt; port cô lập thay đổi.
