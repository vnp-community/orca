# BE-CV-TASK-040-17: Kênh `codeIntel.dismissFinding`, `codeIntel.reviewState.get`, `codeIntel.reviewState.save`

**From Solution:** BE-CV-SOL-040-codeintel-write-and-stream-channels
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_state_review.go` (mới), `channels_codeintel_state_review_test.go` (mới)
**Depends on:** TASK-040-07; stub `DismissFinding` (BE-CV-SOL-037-structure-findings-and-dismissals), `GetReviewState`/`SaveReviewState` (BE-CV-SOL-011-repositories-and-maintenance + CR-052/060)
**Status:** [ ] TODO

---

## Context

UI-API 3.1, 4.6; PQ-05 (dismiss: `findingKey ≤ 128`, `action` dismiss|restore, `disposition` ignored|resolved, `reason`/`note` ≤ 500; `Dismiss` không miễn cổng với `error`), PQ-22 (`review_states`, `version:0` khi chưa có, `save` với `expectedVersion:0` = tạo, `turnMarkers` ≤ 5 chỉ dòng mức worktree), PQ-14 (`reviewState.save` ≤ 256 KiB; `readingProgress ≤ 64 KiB`, `notes ≤ 256 KiB/500 mục`). Xung đột => `CODEINTEL_VERSION_CONFLICT | {"currentVersion"}`. Cần `SetReadLimit` (TASK-040-08) để khung 256 KiB tới được.

## Việc cần làm

1. `dismissArgs{sel, FindingKey, Action, Disposition, Reason, Note}`: kiểm như SOL 2.1; `Core.DismissFinding`; trả `{findingKey, dismissed, disposition?}`.
2. `reviewStateGetArgs{sel, BaseCommit, HeadCommit}` (rỗng hợp lệ; `checkGitRefLike` khi khác rỗng); `Core.GetReviewState`; kết quả `ReviewState` phẳng (không envelope); `version:0` giữ nguyên từ service.
3. `reviewStateSaveArgs{sel, BaseCommit *string, HeadCommit *string, ReadingProgress json.RawMessage, Notes json.RawMessage, TurnMarkers json.RawMessage, Status string, ExpectedVersion *int64}`: `BaseCommit/HeadCommit/ReadingProgress/ExpectedVersion` bắt buộc **có mặt**; `ExpectedVersion >= 0`; `len(ReadingProgress) ≤ 64 KiB`, `len(Notes) ≤ 256 KiB`; `TurnMarkers` là mảng ≤ 5 phần tử; `Status` rỗng|open|reviewed; tổng đã chặn bởi `MaxArgsBytes = 256 KiB`.
4. Chuyển `ReadingProgress/Notes/TurnMarkers` sang proto theo kiểu chốt bởi CR-011/052/060: nếu chuỗi JSON => truyền nguyên (đã kiểm hợp lệ bằng `json.Valid`); nếu message => `protojson.Unmarshal` (không `DiscardUnknown`); lỗi => `INVALID_PARAMS` nêu tên trường.
5. Không bao giờ log hay đưa nội dung `notes` vào lỗi (đây là văn bản người dùng).

## Kiểm thử

- Validate: `expectedVersion` -1/thiếu; `baseCommit` thiếu khoá (khác rỗng); `readingProgress` 64 KiB+1; `notes` 256 KiB+1; `turnMarkers` 6; JSON hỏng; `status` `done`; `findingKey` 129; `reason` 501; `action` `delete`.
- Fake: `Aborted` => `VERSION_CONFLICT` giữ `currentVersion`; `reviewState.get` trả `version:0`; deadline 8 s; metadata tenant.
- Khung 256 KiB đi qua `Registry.Dispatch` (kiểm `MaxArgsBytes`), 256 KiB+1 bị từ chối `INVALID_PARAMS` (`reason:"too_large"`).
- Lệnh: `go test ./internal/adapter/wscompat/ -run 'CodeIntelState(Review|Dismiss)'`.

## Tiêu chí hoàn thành

- [ ] Ba kênh đúng shape; xung đột phiên bản đến client nguyên vẹn.
- [ ] Không trường cấm; cỡ chặn trước giải mã.
- [ ] Nội dung ghi chú không xuất hiện ở lỗi/log.

## Rủi ro và lưu ý

- Khung > 320 KiB làm rớt kết nối thay vì lỗi (xem TASK-040-08).
- Quyền `review_write` thi hành ở service; gateway không.
- `send` sẽ nuốt lỗi `save`: FE phải dùng `invoke`.
