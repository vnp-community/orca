# BE-CV-TASK-012-02: Proto `codeintel_binding.proto` (`RepoBinding`, `BindRepo`, `ListRepoBindings`) và đăng ký RPC

**From Solution:** BE-CV-SOL-012-target-resolution-and-bindings
**Priority:** P0
**Service:** `proto`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_binding.proto` (mới), `backend-go/proto/orca/codeintel/v1/codeintel.proto` (thêm `rpc`), stub sinh vào `backend-go/proto/gen/go/orca/codeintel/v1/`
**Depends on:** BE-CV-TASK-010-04; cổng G0 (`codeintel_common.proto` của SOL-020 có `WorktreeSelector`, `ToolIndexStatus`, `IndexScope`)
**Status:** [x] DONE

---

## Context

Hợp đồng §2.1 #4 và §2.3 (`IndexStatus` nguyên văn), PQ-04, PQ-08 (`IndexScope` có `UNSPECIFIED = 0`). Quy tắc: số field đã ghi ở hợp đồng là chuẩn, còn lại do chủ sở hữu CR gán theo thứ tự khai báo và không đổi. Field 5 của `IndexStatus` (`index_basis`) **chưa khai báo** (Q1 SOL-012): không `reserved`.

## Việc cần làm

1. Nếu SOL-020 chưa merge: dừng, hoặc tạo `WorktreeSelector`/`IndexScope`/`ToolIndexStatus` **theo đúng hợp đồng** trong PR chung với SOL-020 (không tự khai báo bản khác).
2. Tạo `codeintel_binding.proto` với `RepoBinding`, `ActiveReindexJob{id, stage, optional int32 percent}`, `IndexStatus` (các field 1–4, 6, 7, 8 theo §2.3; bỏ 5), `BindRepoRequest/Response`, `ListRepoBindingsRequest/Response`, `GetIndexStatusRequest{selector=1, refresh=2}` + `GetIndexStatusResponse{status=1}`.
3. Thêm vào `service CodeIntelService`: `BindRepo`, `ListRepoBindings`, `GetIndexStatus`.
4. Import theo quy tắc §2.1: `binding` ← `common`; không import `codeintel.proto`.
5. Test phản chiếu (Go, dùng `protoreflect`): mọi message request không có trường tên chứa `path|root|repo_name|dev_server|workspace`; `ListRepoBindingsRequest` không có `selector`.
6. Sinh stub (`make proto-gen`), commit.

## Kiểm thử

- `cd backend-go/proto && buf lint --path orca/codeintel`; `buf breaking --path orca/codeintel --against '../.git#branch=origin/main,subdir=backend-go/proto'`; `go build ./...`; test phản chiếu.

## Tiêu chí hoàn thành

- [x] `buf lint`/`breaking` xanh; stub commit; test phản chiếu xanh.
- [x] `IndexStatus` khớp §2.3 (trừ field 5 hoãn).

## Rủi ro và lưu ý

- Thêm `index_basis = 5` sau này phải import `codeintel_index_basis.proto`: quy tắc import §2.1 cho phép.
