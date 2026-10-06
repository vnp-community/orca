# BE-CV-TASK-033-08: `MergeC4Overrides` (hàm thuần) và message `Get/SaveC4Overrides` trong proto

**From Solution:** BE-CV-SOL-033-c4-overrides-yaml
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/c4/merge_overrides.go` (mới), `backend-go/proto/orca/codeintel/v1/codeintel_c4.proto` (sửa: thêm message), `codeintel.proto` (thêm 2 rpc)
**Depends on:** BE-CV-TASK-033-07, BE-CV-TASK-033-02
**Status:** [ ] TODO

---

## Context

Solution mục 2.C; proto thêm sau task 02 để tránh va chạm.

## Việc cần làm

1. `MergeC4Overrides(view, doc) (view, []C4Warning)` theo quy tắc CR 2.5 (1–7): thứ tự components → hide → externals → relations; `merge` gộp (hợp symbolCount/quan hệ, bỏ cạnh nội bộ); `paths` tạo component và loại package; id lạ → `C4_UNKNOWN_ID`; cạnh không có đầu cuối → `C4_RELATION_SKIPPED`.
2. `origin` đúng; idempotent (áp hai lần cho cùng kết quả).
3. Proto: `GetC4OverridesRequest{selector=1, container=2}`, `GetC4OverridesResponse{document=1, version=2, updated_by=3, updated_at=4, seed_source=5}`, `SaveC4OverridesRequest{selector=1, container=2, document=3, expected_version=4}`, `SaveC4OverridesResponse{version=1, warnings=2}`; kiểu `version`: đề xuất `int64` (G1, xác nhận trước merge); 2 rpc vào `CodeIntelService`.

## Kiểm thử

`go test ./services/code-intel-service/internal/domain/c4/... -run Merge` (chưa chạy): bảng ca `merge`, `paths`, `hide`, id lạ, `relations.remove`, `merge` rồi `hide`; `buf lint && buf breaking`.

## Tiêu chí hoàn thành

- [ ] Mọi ca CR 2.5 có test; idempotent.
- [ ] `buf` xanh.

## Rủi ro và lưu ý

- Thứ tự áp dụng ảnh hưởng kết quả; ghi vào code (1 dòng) vì sao.
