# BE-CV-TASK-085-11: Sự kiện `orca.codeintel.quality.gate_changed` qua outbox

**From Solution:** BE-CV-SOL-085-waivers-and-trend
**Priority:** P1
**Service:** `code-intel-service`
**File:** `internal/domain/quality_gate_changed.go`, mở rộng `record_quality_trend_point.go` (mới/sửa)
**Depends on:** BE-CV-TASK-085-10
**Status:** [x] DONE

## Context
Payload chuẩn hợp đồng §5 (snake_case, không `tenant_id`, không thông điệp lỗi/mã/đường dẫn). Đẩy lên UI là việc của BE-CV-SOL-024 + 040 (service tiêu thụ chính sự kiện này).

## Việc cần làm
1. Hàm thuần `ShouldEmit(prev *Point, cur Point)`: phát khi chưa có điểm hoặc `verdict` khác.
2. Ghi `outbox_events` **cùng transaction** với upsert điểm; `event_id` = UUID v5 của `(binding, head, profile_ref, verdict, sha256(sorted run_ids))`.
3. Payload: `{repo_binding_id, head_commit, base_commit, profile_ref, previous_verdict|null, verdict, run_ids[], turn_key?, evaluated_at}`.

## Kiểm thử
- Bảng `ShouldEmit`; rollback transaction ⇒ không event; cùng khoá hai lần ⇒ một event; kiểm payload không chứa khoá lạ (golden).

## Tiêu chí hoàn thành
- [x] chỉ phát khi `verdict` đổi; [ ] cùng transaction; [ ] payload khớp §5.

## Rủi ro
- Cửa sổ khử trùng JetStream hữu hạn; lớp chính là so điểm gần nhất ở DB.
