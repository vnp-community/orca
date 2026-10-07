# BE-CV-TASK-085-04: `EvaluateGate` hàm thuần và chọn run phù hợp (bảng ca)

**From Solution:** BE-CV-SOL-085-quality-gate-evaluator-and-profiles
**Priority:** P0
**Service:** `code-intel-service`
**File:** `internal/domain/quality_gate_evaluator.go`, `quality_gate_run_selection.go` (mới)
**Depends on:** BE-CV-TASK-085-03
**Status:** [x] DONE

## Context
Thuật toán: CR-085 §2.3 + SOL-085-evaluator §2.3 (L4, L9, D3, D6). `fail > unknown > warn > pass` (F2). Không I/O, không import package `ai_review`, không import adapter.

## Việc cần làm
1. Kiểu `GateInput`, `RunView`, `FindingView`, `StructureFindingView` (`origin`), `CoverageView`, `WaiverView`, `DismissalView`, `IndexFreshness`; `EvaluateGate(def, ref, in, now) QualityGate`.
2. Chọn run: cùng binding, `profile`, `source=local`, `head_commit==HEAD`, trạng thái `succeeded|failed`; `queued|running`→`unknown(run_running)`; `cancelled`→`unknown`; `error_code≠''`→`unknown(env_not_ready|tool_failed)`; `work_tree_changed`→`unknown(tree_changed_during_run)`; scope phủ yêu cầu; run mới nhất.
3. Kết quả từng check theo bảng CR; check có `whenChangedPaths` mà `ChangedFiles==nil`→`unknown(no_overlay)`; coverage `estimated` không bao giờ `fail`; cấu trúc chỉ `origin=introduced` tính, `unknown`→`unknown`.
4. Waiver/dismissal: `waived`+`waivedCount`; `dismissed_not_waived` cho `error`/`blockingSeverities`.
5. `basedOn` đủ trường PQ-34; `profile="<name>@<scope>/v<ver>"`; `reasons ≤ 64` (gộp theo check, `params.merged`).

## Kiểm thử
- ≥ 40 ca bảng (mỗi dòng bảng CR × có/không waiver); ca phủ định: không ca nào ra `pass` khi check `required` thiếu dữ liệu; run sai HEAD không dùng; `fail`+`unknown` giữ cả hai lý do; test cấu trúc `go/parser` cấm import `ai_review`.

## Tiêu chí hoàn thành
- [x] bảng ca xanh; [ ] `index_commit≠HEAD` ⇒ `structure unknown` + `stale=true`; [ ] coverage tắt ⇒ không có lý do coverage.

## Rủi ro
- Phụ thuộc `error_code`/`dirty` của T8 (BE-082); thiếu thì mọi `failed` không phân biệt, coi `unknown`.
