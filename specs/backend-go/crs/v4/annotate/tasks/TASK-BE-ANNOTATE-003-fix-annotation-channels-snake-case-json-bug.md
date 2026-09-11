# TASK-BE-ANNOTATE-003: Fix `annotation.create`/`list`/`update`/`markSent`/`sendToAgent`'s snake_case-JSON bug

**Solution:** none (not designed in advance — discovered live while implementing [SOL-FE-ANNOTATE-001](../../../../frontend/crs/v4/annotate/solutions/SOL-FE-ANNOTATE-001-persist-diff-comments-to-annotation-service.md)'s hydrate step) | **CR:** CR-ANNOTATE-001 (adjacent — not originally scoped)
**Depends on:** Không
**Status:** ✅ DONE (2026-09-11)

---

## Mục tiêu (ghi lại sau khi phát hiện, không phải kế hoạch trước)

Không có trong bất kỳ solution nào trước đó — `SOL-BE-ANNOTATE-001` khẳng
định "zero backend-go code change required" cho CR-ANNOTATE-001. Phát hiện
khi tra `annotationv1.Annotation`/`Anchor`'s generated Go struct trước khi
viết `annotationWireToDiffComment` (TASK-FE-ANNOTATE-002): các field JSON
tag là **snake_case** (`json:"file_path"`, `json:"created_at"`,
`json:"end_line"`...), không phải camelCase — trong khi
`registerAnnotationChannels`'s `annotation.create`/`list`/`update`/
`markSent` (và `channels_annotation_send.go`'s `SendReviewFeedbackToAgent`)
trả `resp.GetAnnotation()`/`resp` (proto thô) trực tiếp. Vì wscompat
envelope serialize `Result any` qua `encoding/json` thường, không phải
`protojson` (đã xác nhận qua nhiều comment cảnh báo có sẵn trong
`channels_infra_fleet.go`/`channels_workflow.go`/`channels_tenant_project.go`/
`channels_dev_server_access_control.go`/`channels_emulator_folderworkspace_host.go`
— cùng 1 lớp bug đã được tìm và sửa ở NHIỀU channel khác trước đây, nhưng bỏ
sót `annotation.*`), timestamp (`created_at`/`updated_at`/`sent_at`) sẽ
serialize thành `{"seconds":...,"nanos":...}` thay vì số Unix-ms, và mọi
field nhiều từ (`file_path`, `worktree_id`, `end_line`, `original_code`,
`sent_to_agent`) sẽ về dưới dạng snake_case mà không client TypeScript nào
đọc đúng (tất cả đọc camelCase).

**Mức độ nghiêm trọng**: đây là bug thật, ảnh hưởng **mọi caller hiện tại và
tương lai** của `annotation.create`/`list`/`update`/`markSent`, không chỉ
riêng hydrate — kể cả nếu `annotation-panel.tsx` (đã xoá, TASK-FE-ANNOTATE-003
[frontend]) còn tồn tại và có shape đúng, nó vẫn sẽ nhận response sai.

## Giải pháp

Theo đúng convention có sẵn (`channels_admin_users.go`'s `userView`/
`toUserView`): thêm `anchorView`/`annotationView` struct với JSON tag
camelCase đúng + `CreatedAtUnixMs`/`UpdatedAtUnixMs`/`SentAtUnixMs int64`
(qua `.AsTime().UnixMilli()`), và `toAnnotationView`/`toAnnotationViews`
converter. Áp dụng cho cả 5 chỗ trả annotation proto thô:
`annotation.create`, `annotation.list` (bọc thêm `nextPageToken`),
`annotation.update`, `annotation.markSent`, và
`channels_annotation_send.go`'s `SendReviewFeedbackToAgent`'s
`result["annotations"]`.

## Files đã sửa

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (MODIFY — thêm `anchorView`/`annotationView`/`toAnnotationView(s)`, áp dụng cho 4 handler)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_annotation_send.go` (MODIFY — 1 dòng, `toAnnotationViews` thay vì raw)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go` (MODIFY — `TestAnnotationMarkSentChannel_Wired` viết lại assertion theo shape mới)

## Verify thật đã chạy

`go build`/`go vet` sạch cho `api-gateway`; `go test ./services/api-gateway/...` —
toàn bộ package `ok` (bao gồm `wscompat` sau khi sửa lại
`TestAnnotationMarkSentChannel_Wired`, test này FAIL trước khi sửa — xác
nhận đúng đây là thay đổi behavior thật, không phải refactor vô hại). Toàn
bộ 17 service backend-go build sạch (`for d in services/*/; do go build
./...; done`).

## gitnexus

Không chạy `impact()` riêng cho `toAnnotationView` (symbol mới, chưa tồn
tại trước task này). Đã grep xác nhận không còn chỗ nào khác trong
`wscompat` trả raw `*annotationv1.Annotation`/`ListAnnotationsResponse`/
`MarkAnnotationsSentResponse` sau khi sửa (chỉ còn dùng nội bộ trong
`channels_annotation_send.go`'s logic tính toán, không trả ra ngoài).

## Liên quan

- [TASK-FE-ANNOTATE-002](../../../../frontend/crs/v4/annotate/tasks/TASK-FE-ANNOTATE-002-hydrate-and-backfill.md) (lý do phát hiện bug này)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_admin_users.go` (convention gốc đã tham chiếu)
