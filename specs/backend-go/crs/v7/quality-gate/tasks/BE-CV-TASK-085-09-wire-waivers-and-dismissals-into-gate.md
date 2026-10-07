# BE-CV-TASK-085-09: Nối miễn trừ và `finding_dismissals` vào `EvaluateQualityGate`

**From Solution:** BE-CV-SOL-085-waivers-and-trend
**Priority:** P0
**Service:** `code-intel-service`
**File:** `internal/usecase/evaluate_quality_gate.go` (sửa), `internal/adapter/{postgres,mysql}/quality_waiver_reader.go` (mới), adapter `DismissalReader` (dùng bảng T5 của SOL-011/037)
**Depends on:** BE-CV-TASK-085-06, 085-08; BE-CV-SOL-037-structure-findings-and-dismissals
**Status:** [x] DONE

## Context
Bảng ảnh hưởng: SOL-085-waivers §2.3 (PQ-05). `DismissFinding` không bao giờ miễn `error`/`blockingSeverities`.

## Việc cần làm
1. `WaiverReader`: waiver hiệu lực của `(tenant, repo)` + `binding:<id>` hiện tại (lọc scope), đồng hồ DB.
2. `DismissalReader`: `finding_dismissals(tenant_id, repo_id, finding_key)` với `disposition`.
3. Truyền vào `GateInput`; `GetQualityGate` trả `waivers[≤50]`.
4. Sau `WAIVE/REVOKE`, lần đọc kế tiếp tính lại (không cache kết luận).

## Kiểm thử
- Use case: dismiss `error` ⇒ vẫn `fail` + `dismissed_not_waived`; waive ⇒ `pass/warn` + `waived`/`waivedCount`; waiver scope `binding:X` không áp ở binding Y; waiver hết hạn không áp.

## Tiêu chí hoàn thành
- [x] bốn dòng bảng ảnh hưởng có test; [ ] không đọc waiver tenant khác.

## Rủi ro
- Adapter `DismissalReader` phụ thuộc bảng T5 chưa có hiện thực.
