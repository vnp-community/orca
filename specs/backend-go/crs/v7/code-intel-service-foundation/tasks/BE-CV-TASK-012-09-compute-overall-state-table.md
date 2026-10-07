# BE-CV-TASK-012-09: `ComputeOverall`: 9 trạng thái, `OVERLAY`, `scope_mismatch`, ánh xạ `index_scope` DB

**From Solution:** BE-CV-SOL-012-index-status-aggregation
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/index_overall.go`, `index_overall_test.go` (mới)
**Depends on:** BE-CV-TASK-012-08
**Status:** [x] DONE

---

## Context

Bảng ưu tiên ở SOL-012-index-status mục 2.C (PQ-08: thêm `OVERLAY`; `scope_mismatch` cờ riêng). Hàm thuần, không I/O.

## Việc cần làm

1. `type Overall string` với 9 hằng HOA (PQ-32).
2. `ComputeOverall(in OverallInput) (Overall, scopeMismatch bool)`; `OverallInput{Offline bool; ProbeErr error; Tools []ToolView; ActiveJob *ActiveJob; WorktreeMismatch bool}`; thực hiện đúng thứ tự 2–10 của bảng; "công cụ khả dụng" = `available && supported`.
3. `DBIndexScope(tools []ToolView) IndexScopeDB` theo PQ-08 (4): `exact` nếu mọi công cụ khả dụng `exact`; `repo_root` nếu có `repo_root` hoặc `stale` gốc khác; `unresolved` nếu `none` hoặc không công cụ nào.
4. Đồng nhất tên enum dây (`exact|repo_root|stale|none`) và tên DB (`exact|repo_root|unresolved`) bằng hai kiểu khác nhau để không lẫn.

## Kiểm thử

- `go test ./services/code-intel-service/internal/domain/... -run Overall`
- Mỗi dòng bảng ≥ 1 test; test ưu tiên: offline + job → `OFFLINE`; probe lỗi + job → `UNKNOWN`; job + `missing` → `BUILDING`; `ready`+`missing` → `DEGRADED`; chỉ `repo_root/fresh_base` → `OVERLAY` và `scope_mismatch=true`; `stale` không `ready` → `STALE`; `ready` đầy đủ → `READY`; không công cụ → `NOT_INSTALLED`.
- Property: kết quả luôn thuộc 9 giá trị; không panic với đầu vào rỗng.

## Tiêu chí hoàn thành

- [x] Toàn bộ bảng có test; thứ tự ưu tiên khoá bằng test.

## Rủi ro và lưu ý

- Điều kiện `OVERLAY` khi một công cụ (Q2): ghi trong chú thích để đổi dễ.
