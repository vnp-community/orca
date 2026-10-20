> DRAFT: see trace-manifest.json for blockers.

## Context

BRL-48 được thực thi bằng job hằng ngày và Alert `entitlement_expiring`.

SAD không thay đổi kiến trúc trong change này.

Hai dòng change-spec CS-001 và CS-002 cùng nhắm tới `internal/entitlement/reminder.go::Milestones`.

## Goals / Non-Goals

**Goals:**
- Job hằng ngày gửi đúng một nhắc nhở T-14 cho Entitlement thuê bao hết hạn sau 14 ngày.

**Non-Goals:**
- Không thay đổi kiến trúc (SAD architecture_change là false).

## Decisions

- Mốc T-14 được thêm vào `Milestones` tại `internal/entitlement/reminder.go`, nằm giữa T-30 và T-7.
- Mỗi mốc gửi đúng một lần, theo dõi bằng `reminded_milestones`.

## Risks / Trade-offs

- Pack không nêu rủi ro hay phương án thay thế nào cho change này.

## Open Questions

- Pack không cung cấp nội dung văn bản TDD, nên chưa có kế hoạch rollback hay migration; cần xác nhận.
- Chưa xác nhận hành vi gia hạn còn hơn 60 ngày (đưa về Active, xoá mốc đã nhắc) có cần kiểm thử riêng cho mốc T-14 hay không.
