# BE-CV-TASK-040-03: `decodeCodeIntelArgs`, `codeIntelParamError`, bộ kiểm chung (selector, ref, enum, thời gian)

**From Solution:** BE-CV-SOL-040-codeintel-channel-foundation
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_args.go` (mới), `channels_codeintel_args_selector.go` (mới), `channels_codeintel_args_test.go` (mới)
**Depends on:** TASK-040-02 (`pathsafety`)
**Status:** [x] DONE

---

## Context

- U1/U3 (UI-API §1): một object ở `args[0]`, `len(args) > 1` bị từ chối, giải mã chặt, khoá cấm bị từ chối nêu **tên trường**; `decodeArg` lỏng (`registry.go:267`) không dùng được.
- Session-client dialect: `params` vắng/`null` đã thành `{}` (`session_dialect.go:55`); client native có thể gửi `args: []` => coi như `{}`.
- Giới hạn: UI-API 2.4 (cỡ `args[0]`, `reason`/`note` ≤ 500, `findingKey` ≤ 128, `key` ≤ 1024, `pageToken` ≤ 512, `kinds` ≤ 32, `ifNoneMatch` ≤ 80, `projectId` ≤ 64, `worktreeId` ≤ 512, `depth` 1..3). Số, khoảng: **từ chối, không kẹp** (D3 của CR).
- Mã lỗi: `CODEINTEL_INVALID_PARAMS` (`data.field`, `reason?`), `CODEINTEL_PATH_NOT_ALLOWED` (UI-API 2.3).

## Việc cần làm

1. `codeIntelParamError{Code, Field, Reason, Limit}` có `Error()` dựng `CODEINTEL_INVALID_PARAMS: <mô tả ngắn> | {"field":"depth","reason":"out_of_range"}` (JSON hợp lệ, không giá trị tham số). Hàm tạo: `invalidParam(field, reason)`, `tooLarge(limit)` (`reason:"too_large"`, `limit`), `pathNotAllowed(field)` (`CODEINTEL_PATH_NOT_ALLOWED`, **không** nêu đường dẫn).
2. `decodeCodeIntelArgs[A codeIntelArgs](spec codeIntelChannelSpec, args []json.RawMessage) (A, error)`:
   a. `len(args) > 1` => `invalidParam("args","expected_single_params_object")`; `len(args)==0` hoặc `null` => `{}`;
   b. byte đầu (sau bỏ trắng) phải là `{`, ngược lại `invalidParam("args","not_an_object")`;
   c. `len(raw) > spec.MaxArgsBytes` => `tooLarge(spec.MaxArgsBytes)` **trước** khi giải mã;
   d. giải mã vào `map[string]json.RawMessage`; khoá thuộc `codeIntelForbiddenKeys` = `tenantId, userId, deviceId, role, devServerId, workspaceRoot, repo, args, command, cypher` => `invalidParam(<khoá>, "not_allowed")` (khớp không phân biệt hoa thường để chặn `TenantID`);
   e. `json.NewDecoder(...).DisallowUnknownFields()` vào `A`; lỗi => `invalidParam(<tên trường nếu rút được từ lỗi `unknown field`/`Field` của `*json.UnmarshalTypeError`>, "unknown_field"|"wrong_type")`; **không** chép giá trị vào thông điệp;
   f. dữ liệu thừa sau object (`dec.More()`/token kế tiếp) => `invalidParam("args","trailing_data")`;
   g. `in.validate()`.
3. `channels_codeintel_args_selector.go`: `type codeIntelSelector struct{ProjectID string \`json:"projectId"\`; WorktreeID string \`json:"worktreeId"\`}` (nhúng vào mọi kiểu args có "(sel)"), `validate()`: cả hai bắt buộc, `projectId` ≤ 64, `worktreeId` ≤ 512, không ký tự điều khiển; hàm `toProtoSelector(s) *codeintelv1.WorktreeSelector` (`project_id`, `worktree_ref`).
4. Bộ kiểm dùng chung (hàm, không phải "helpers"): `checkBoundedInt(field string, v *int, lo, hi int)` (con trỏ: vắng => bỏ qua; ngoài khoảng => từ chối), `checkEnum(field, v string, allowed ...string)` (rỗng cho phép nếu tuỳ chọn), `checkRelPath(field, v string)` (khác rỗng => `pathsafety.CleanWorktreePath`, lỗi => `pathNotAllowed`), `checkGitRefLike(field, v string)` (≤ 256, regex `^[\p{L}\p{N}_][\p{L}\p{N}._/@{}~^+-]*$`, không chứa `..`, không bắt đầu `-`; chống tiêm tuỳ chọn git; service kiểm lại), `checkRFC3339(field, v string)`, `checkStringMax(field, v string, n int)`, `checkPageToken(field, v string)` (≤ 512, gateway không diễn giải), `checkOpaqueID(field, v string)` (≤ 128, quyết định của solution cho jobId/runId/turnId/flowId nếu hợp đồng không nêu).
5. Hằng đặt tên ở catalog (TASK-040-07) hoặc đầu file này: `codeIntelMaxSelectors = 50`, `codeIntelMaxKinds = 32`.

## Kiểm thử

Bảng test (thêm vào `channels_codeintel_args_test.go`, kiểu args giả có `codeIntelSelector` + vài trường):
- `args` rỗng, `[{}, {}]`, `["x"]`, `[null]`, `[[1]]`; `{"tenantId":"t"}`, `{"TenantId":"t"}`, `{"workspaceRoot":"/x"}`, `{"cypher":"MATCH"}`; khoá lạ `{"foo":1}`; kiểu sai `{"depth":"2"}`; `{...}{...}`; cỡ vượt đúng 1 byte; `depth` 0, 4, -1 => từ chối; `{"path":"../x"}`, `{"path":"/etc"}`, `{"path":"a\\b"}`, `{"path":"%2e%2e/x"}`, `{"path":"．．/x"}` => `CODEINTEL_PATH_NOT_ALLOWED`; ref `-rf`, `a..b`, `a b` => `INVALID_PARAMS`, `origin/main`, `HEAD~3`, `feature/ünï` hợp lệ.
- Khẳng định thông điệp lỗi **không chứa** giá trị đã gửi (quét chuỗi `"/etc"`, `"MATCH"`).
- Phân tích hậu tố `data` là JSON hợp lệ bằng regex của UI-API 2.3.
- Dialect session-client: `params` vắng thành `{}` đi qua decode không lỗi với kiểu args chỉ có trường tuỳ chọn.
- Lệnh: `go test ./internal/adapter/wscompat/ -run 'CodeIntelArgs'`.

## Tiêu chí hoàn thành

- [x] Mọi khoá cấm của U3 bị từ chối kèm tên trường, không kèm giá trị.
- [x] Ngoài khoảng bị từ chối, không kẹp.
- [x] Đường dẫn độc hại => `CODEINTEL_PATH_NOT_ALLOWED`.
- [x] Không dùng `decodeArg`/`json.Unmarshal` lỏng trong gói này.

## Rủi ro và lưu ý

- Quét khoá cấm không phân biệt hoa thường có thể chặn trường hợp lệ đặt tên `Role` ở lớp con: kiểm chỉ **cấp một**.
- Giá trị chuỗi cho `kind`/`rule` của người dùng không được kiểm enum ở gateway (enum mở); service kiểm.
- `checkGitRefLike` là lớp đầu; ref hợp lệ với git nhưng chứa ký tự ngoài tập (ví dụ `:`) bị từ chối: chấp nhận vì UI chỉ gửi ref lấy từ danh sách nhánh; ghi nhận ở hợp đồng nếu FE cần mở rộng.
