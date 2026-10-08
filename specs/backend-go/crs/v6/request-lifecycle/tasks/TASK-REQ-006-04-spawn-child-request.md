# TASK-REQ-006-04: Use case `SpawnChildRequest` và liên kết `request_links`

**From Solution:** BE-REQ-SOL-006
**Priority:** P1
**Service:** `request-service`
**File:** `internal/usecase/spawn_child_request.go`, `internal/usecase/spawn_child_request_test.go` (mới); `internal/usecase/create_request.go` (sửa: `Parent`); `internal/domain/request_child_rules.go`, `internal/domain/request_child_rules_test.go` (mới)
**Depends on:** TASK-REQ-006-02, TASK-REQ-004-04 (`CreateWithinTx`), TASK-REQ-002-04/05 (`RequestLinkRepository`)
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: `go test -tags integration -race ./internal/adapter/postgres ./internal/adapter/mysql -run RPC (13+ kịch bản gRPC, máy trạng thái thật); go test ./internal/adapter/grpc; buf lint/breaking --path orca/request/v1/request.proto`)

---

## Context

CR-REQ-006 mục 2.6. Quy tắc cha, con (bảng) nằm ở domain để test không cần DB. Lõi tạo Request là `CreateRequest.CreateWithinTx` (TASK-REQ-004-04, có `AllowTypeHint`). `RequestLinkRepository` có `Insert`, `ListChildren`, `ListParents`. Con vẫn đi đủ phân loại và xác nhận. Khoá idempotency của con có tiền tố cha (SOL-006 C5). `hotfix` kết thúc thì CR-REQ-014 gọi use case này; ở đây chỉ cung cấp lệnh.

## Việc cần làm

1. `domain/request_child_rules.go`: `ValidateChild(parent Request, reason LinkReason, typeHint RequestType) error`: bảng `spawned_by_spike` (cha `spike`, status {`awaiting_analysis_approval`,`completed`}, hint {`change_request`,`task`}), `spawned_by_question` (cha `question`, như trên), `followup_hotfix` (cha `hotfix`, status {`executing`,`completed`}, hint {`bug`,`task`}), `escalation` (mọi loại, mọi status trừ `cancelled`, hint tuỳ ý hoặc rỗng); sai tổ hợp `REQUEST_CHILD_NOT_ALLOWED`. Hằng `MaxChildrenPerParent=50`, `MaxAncestorDepth=5`.
2. `CreateRequestInput` thêm `Parent *ParentLink` (`ParentRequestID`, `LinkReason`) và `CreateWithinTx` đưa hai trường vào payload outbox `created` (`parent_request_id`, `link_reason`); không đổi hành vi khi `Parent == nil`.
3. `SpawnChildRequest.Execute(ctx, SpawnInput{ParentRequestID, LinkReason, Title, Body, TypeHint, ClientRequestID, ActorID, Provider}) (SpawnResult{Child Request; Created bool}, error)`:
   - `ClientRequestID` rỗng: `REQUEST_CLIENT_REQUEST_ID_REQUIRED`; `Provider` chỉ `manual` hoặc `mcp`.
   - `repo.Get(parent)` (`REQUEST_PARENT_NOT_FOUND`); `ValidateChild`.
   - đếm con: `RequestLinks.ListChildren(parent)` có ≥ 50: `REQUEST_CHILD_LIMIT`.
   - độ sâu: đi ngược bằng `ListParents` tối đa 5 bước; nếu cha đã ở cấp 5 (chuỗi tổ tiên đủ 4 cấp trên nó) thì `REQUEST_CHILD_DEPTH_EXCEEDED`.
   - `InTx`: `CreateWithinTx` (`ProjectID` của cha, `ClientRequestID = parentID + ":" + clientRequestID`, `Hints.TypeHint`, `AllowTypeHint=true`, `Parent`), nếu `Created=false` trả con cũ không chèn link; `RequestLinks.Insert(parent, child, reason, createdBy)`.
4. Gọi lặp cùng `ClientRequestID`: `Created=false`, một `request_links` duy nhất.
5. `escalation` từ cha `cancelled` bị chặn bởi `ValidateChild`.

## Kiểm thử

- `TestValidateChild_Matrix` (4 lý do nhân loại cha nhân status cha; sai tổ hợp lỗi): con từ `spike` với hint `bug` bị từ chối; từ `hotfix` với `bug` được; `escalation` từ `cancelled` bị từ chối.
- `TestSpawn_CreatesChildInClassifying_WithLinkAndPayload` (outbox `created` có `parent_request_id`, `link_reason`).
- `TestSpawn_IdempotentSameClientRequestID`, `_SameClientIDDifferentParents_TwoChildren`.
- `TestSpawn_ChildLimit50` (repo giả có 50 link), `_DepthExceeded` (chuỗi 6 cấp), `_DepthFiveOK`.
- `TestSpawn_MissingClientRequestID`, `_ParentNotFound`.
- `TestSpawn_RollbackLeavesNoLink` (ép lỗi sau `CreateWithinTx`: không còn link, không đốt số).
- Lệnh: `go test ./services/request-service/internal/domain/... ./services/request-service/internal/usecase/... -run "Child|Spawn"`.

## Tiêu chí hoàn thành

- [x] Quy tắc cha, con đúng bảng; con đi `classifying` như Request bình thường.
- [x] 12 lệnh đồng thời cùng khoá: một con, một link (kiểm tích hợp ở TASK-REQ-006-06).
- [x] Giới hạn 50 con và 5 cấp có test.
- [x] `CreateRequest` công khai vẫn bỏ qua `type_hint`.

## Rủi ro và lưu ý

- Giới hạn con mềm khi đồng thời (mỗi lệnh đếm trước giao dịch); chấp nhận vượt vài đơn vị.
- `escalation` chưa được README định nghĩa (Q1 của SOL-006); nếu bị bỏ, xoá hằng và nhánh.

## Ghi chú triển khai

- `SpawnChildRequest` nay tạo con qua `IntakeChildCreator` (`child_request_via_intake.go`) trên `CreateRequest.CreateWithinTx`: con vào `classifying`, `type_hint` nằm trong `source_hints` (cột), sự kiện `created` có `parent_request_id` và `link_reason`. `IdempotentChildCreator` không còn được lắp.
- Test: `SpawnChildAndLinks` (gRPC, thấy `type_hint`, replay trả cùng con, sai quy tắc cha bị từ chối), `SpawnChildConcurrent12SameKey` (12 lệnh đồng thời một con một link) hai dialect; `CreateRequest` công khai vẫn bỏ `type_hint` (`CreateRequest_IgnoresTypeHint`).
