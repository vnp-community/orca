# TASK-TDD-002-T2-06: Dựng Plan từ `tasks.md` và lưu gói OpenSpec

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.4 (bước 6)
**Checklist:** [T2.6](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P0 · **Area:** request-service (Go + migration)
**Files:** `internal/domain/{openspec_change,plan_proposal}.go`, `internal/usecase/{run_plan_generation,commit_plan}.go`, mới `internal/usecase/openspec_plan_builder.go`, `internal/adapter/{postgres,mysql}/openspec_change_repository.go`, migration mới `0094_openspec_change_bundle` (hai dialect), `proto/orca/request/v1`
**Depends on:** T1.8, T2.4, T2.5
**Status:** `[ ]` TODO

---

## Context

`PlanProposal` từ parser chỉ mang tiêu đề, mô tả, `ac`, nhãn, phụ thuộc. Orca còn phải điền `PromptTemplate` cho từng task (để agent task biết cần làm gì), lưu gói để người duyệt đọc được sau khi worktree bị gỡ, và ghi lại nguồn gốc (snapshot KG, id đã đọc, hash skill) do agent báo trong result block.
Bảng `request.openspec_changes` (migration 0007) hiện có: `id, tenant_id, request_id, change_id, branch, status, tasks_sync_state/digest/error/next_retry_at, commit_sha, created_at, updated_at, version`; `domain.OpenSpecChange` chỉ có `ID, RequestID, Status, SyncState, Commit`.

## Việc cần làm

1. Migration `0094`: thêm vào `openspec_changes` các cột `tasks_md_digest`, `kg_snapshot`, `kg_read_ids` (JSON), `kg_gaps` (JSON), `kg_conflicts` (JSON), `skill_hash`, `bundle_json` (các tệp ≤ 256 KiB mỗi tệp, tổng giới hạn), `workspace_id`; hai dialect, có `down`.
2. Mở rộng `domain.OpenSpecChange` và `OpenSpecChangeRepository` (`Upsert`, `GetByRequest`); cập nhật cả Postgres và MySQL.
3. `openspec_plan_builder.go`: `BuildPromptTemplate(changeID, specPath, task)` sinh khối chuẩn cho task: id yêu cầu phục vụ, đích mã, lệnh `Verify`, `Done when`, quy ước (không sửa `openspec/`, `docs/tdd/`, `.tdd-agent/` trừ khi task nói; trả result block theo nonce), đường dẫn spec (theo quyết định T1.8); điền `TaskProposal.PromptTemplate`, `Satisfies` từ `ac`.
4. Sau khi qua kiểm T2.5: `ProposalWorkspace.Commit` (đường dẫn dưới `openspec/changes/<id>/`) lên nhánh proposal; đọc lại các tệp để lưu `bundle_json`; `OpenSpecChange.MarkReady()`; ghi `commit_sha`, `branch`, digest của vùng `tasks.md` (`PlanRegionDigest`).
5. Đọc result block của agent: `knowledge{snapshot, read[], gaps[], conflicts[], available}`, `change`, `validation`; lưu vào cột tương ứng; thiếu khối ⇒ cảnh báo, lưu rỗng.
6. `finish` vẫn lưu `EncodePlanProposal(p)` vào `run.RawOutput`; `CommitPlan` không đổi, nhận `PlanProposal` đã điền.
7. **Toàn vẹn duyệt:** digest của chủ thể gate Plan phải gồm digest của gói (`tasks_md_digest` + băm `bundle_json`), để người duyệt đồng ý đúng gói đã đọc; kiểm chỗ tính `subject_digest` của approval kiểu plan và bổ sung.

## Tiêu chí hoàn thành

- [ ] Plan dựng từ `tasks.md` mẫu đi qua `commit_plan` và cho cây Phase/task đúng thứ tự và phụ thuộc.
- [ ] `openspec_changes` lưu đủ trường ở cả hai dialect; sau khi gỡ worktree vẫn đọc được gói từ `bundle_json`.
- [ ] Đổi nội dung gói sau khi gate mở làm digest thay đổi và gate cũ không duyệt được.

## Kiểm thử

- Test đơn vị `BuildPromptTemplate` (golden), builder với `tasks.md` mẫu và lỗi định dạng.
- Test hợp đồng hai dialect cho repository mới.
- Test digest gate.

## Rủi ro và ghi chú

- `bundle_json` có trần kích thước; vượt trần ⇒ lưu danh sách tệp và băm, đọc nội dung từ nhánh.
- Chạy `gitnexus_impact` cho `OpenSpecChange` và `CommitPlan` trước khi sửa.
