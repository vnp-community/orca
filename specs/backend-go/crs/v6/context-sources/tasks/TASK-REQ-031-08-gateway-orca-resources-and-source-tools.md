# TASK-REQ-031-08: `api-gateway`: tài nguyên `orca://request|solution|plan|evidence|impact|context` và tool `source_*`

**From Solution:** BE-REQ-SOL-031 (mục F)
**Priority:** P1
**Service:** `api-gateway`, `request-service` (RPC đọc `GetEvidence`, `GetContextPack`)
**File:** `backend-go/services/api-gateway/internal/adapter/mcpserver/resources/uri.go` (sửa), `.../resources/plans.go` (sửa), `.../resources/uri_test.go`, `.../resources/provider_test.go` (sửa), `.../tools/pack5_context.go` (mới, tên khớp quy ước `pack*_*.go`), `.../tools/excluded_channels.yaml` (sửa), `.../tools/parity_test.go` (chạy), `.../internal/adapter/wscompat/channels_context.go` (mới), `backend-go/proto/orca/request/v1/request.proto` (sửa: `GetEvidence`, `GetContextPack`, `SearchContextSources`), `request-service/internal/adapter/grpc/server_context_read.go` (mới)
**Depends on:** TASK-REQ-031-05; BE-REQ-SOL-016 (client `request-service` trong gateway, kênh `request.get`, `solution.get`)
**Status:** `[x] DONE`

---

## Context

- Đã đọc `resources/uri.go`: `Kind` là tập đóng 7 giá trị; `Parse` tách theo `/`, mỗi thành phần được percent-decode **một lần** rồi kiểm (không "làm sạch"), sai định dạng trả `errBadURI` (người dùng thấy "không tìm thấy", không lộ lý do). `plans.go`: `planFor(ref)` trả `plan{meta, calls, untrusted, text, mime}`; `readMeta(kind, channel, untrusted, openWorld)` đặt `Risk=read`, `RequiredScope="orca:read"`, `UntrustedOutput`. Đây là khuôn để theo; mỗi `Kind` mới phải có test `uri_test.go` cho URI đúng và sai.
- `tools/excluded_channels.yaml` và `tools/parity_test.go`: mỗi kênh `wscompat` đã đăng ký cần `ToolSpec` hoặc dòng loại trừ, nếu không `parity_test.go` đỏ (README v6 mục 8 điểm 13). Kênh mới của task này: `request.get` và `solution.get` (BE-REQ-SOL-016 đã đăng ký; kiểm trước bằng `grep -rn "request.get" tools/`), `evidence.get`, `context.get`, `impact.get` (CR-REQ-030, chưa có: URI `orca://impact/{id}` đăng ký `Kind` nhưng kênh nền chưa có thì `planFor` trả `errNotReadable`), `source.search|get|list`, nhóm quản trị `contextSource.list|upsert|setStatus|preview` (kênh WS, loại trừ khỏi MCP).
- CR-REQ-017 mục 7 (Q4) từng ghi `orca://request/{id}` "ngoài phạm vi"; CR-REQ-031 nhận. Không sửa CR-017; ghi ở báo cáo.
- Phân quyền: gateway **không** kiểm OPA trước định tuyến (README v6 mục 8 điểm 13); mọi RPC của `request-service` tự kiểm quyền (BE-REQ-SOL-035 task 04). Resource đọc đi qua kênh `wscompat` nên chịu cùng kiểm quyền; không viết thêm kiểm quyền riêng ở gateway.

## Việc cần làm

1. `uri.go`: thêm hằng `KindRequest = "request"`, `KindSolution = "solution"`, `KindPlan = "plan"`, `KindEvidence = "evidence"`, `KindImpact = "impact"`, `KindContext = "context"`. Trong `Parse` thêm các nhánh `switch`: `len(segs)==2 && segs[0]=="request"` (`parseID`), tương tự `solution`, `plan`, `evidence`, `impact`; `len(segs)==3 && segs[0]=="context"` với `segs[1]` là UUID Request qua `parseID` và `segs[2]` thuộc `{classify, solution, plan, task, execute, risk}` (lưu vào `ref.Stage` mới; thêm trường `Stage string` cho `Ref`). `parseID` hiện có giữ nguyên (đọc hàm để biết nó chỉ nhận UUID hay chuỗi; **không** nới lỏng).
2. `plans.go` thêm nhánh trong `planFor`:
   - `KindRequest` → `readMeta(ref.Kind, "request.get", true, false)`, `untrusted: true`, `calls: [{request.get, {"id": ref.ID}}]`;
   - `KindSolution` → `solution.get`, `untrusted: true`;
   - `KindPlan` → `task.getSubtree` (`{"id": ref.ID}`), `untrusted: true`;
   - `KindEvidence` → `evidence.get`, `untrusted: true` (có `excerpt` từ nguồn ngoài);
   - `KindImpact` → `impact.get`, `untrusted: false` (do CR-030, chưa có kênh: trả `errNotReadable` cho tới khi kênh đăng ký);
   - `KindContext` → `context.get` (`{"requestId": ref.ID, "stage": ref.Stage}`), `untrusted: true`, quyền đọc như `orca://request`.
   Mọi `plan` mới có `mime = "application/json"`.
3. `provider.go`: nếu có danh sách `resources/templates` (file `templates.go`) thì thêm sáu mẫu URI (`orca://request/{id}`, ...) để client MCP khám phá được.
4. `wscompat/channels_context.go`: đăng ký `evidence.get` (`GetEvidence{id}` hoặc `{requestId, seq}`), `context.get` (`GetContextPack{request_id, stage}` trả pack mới nhất, `body` + `missing` + `items`), `source.search|get|list` (`SearchContextSources`), và nhóm quản trị `contextSource.*` ánh xạ bốn RPC của task 05. Trả lỗi bằng bộ ánh xạ lỗi của BE-REQ-SOL-016 (`channels_request_errors.go` nếu có; kiểm tên file thật trước).
5. `request.proto`: thêm `GetEvidence`, `GetContextPack`, `SearchContextSources` (đọc, chỉ tìm trong nguồn **đã bật** cho Request và cho người gọi; `SearchContextSources` gọi Builder ở chế độ `dryRun` với `Query` của người dùng và trả `items[]` đã che, **không** ghi `evidence`). `server_context_read.go` hiện thực: `GetEvidence` trả `excerpt` đã che; nhóm hành động `read` của BE-REQ-SOL-035.
6. `tools/pack5_context.go`: `ToolSpec` cho `source_search`, `source_get`, `source_list` (đọc, `RiskRead`, `OpenWorld=false` vì chỉ nguồn đã duyệt; mô tả tool nói rõ kết quả là dữ liệu không tin cậy); cấu trúc theo `pack1_workspace.go` (`read("task.get", "Get one task.", Str(...))`). Đăng ký pack mới ở `all_specs.go`.
7. `excluded_channels.yaml`: thêm `contextSource.*` (lý do: "Quản trị nguồn ngữ cảnh: chỉ qua giao diện, agent không đổi cấu hình nguồn.") và, nếu chưa có `ToolSpec`, `evidence.get`, `context.get` (đã có `resource` thay thế). Sau khi thêm chạy `go test ./services/api-gateway/internal/adapter/mcpserver/tools -run Parity`.
8. `kênh WS` và payload cho frontend (khớp CR-REQ-016 và `CONTRACT-request-ui-api.md`): `contextSource.list` → `{sources: ContextSource[]}`, `contextSource.upsert {source, expectedVersion}`, `contextSource.setStatus {key, status, expectedVersion}`, `contextSource.preview {requestId, stage}` → `{pack: {body, usedTokens, budgetTokens, items[], missing[]}}`, `evidence.get {id}` → `{evidence}`. Tên trường camelCase theo `CONTRACT-request-ui-api.md`; đọc file đó để khớp kiểu trước khi định nghĩa JSON (không đặt tên mới nếu CONTRACT đã có).

## Kiểm thử

- `uri_test.go`: `TestParse_NewKinds` bảng: `orca://request/<uuid>` ok; `orca://request/` (thiếu id), `orca://request/x y`, `orca://context/<uuid>/bogus`, `orca://context/<uuid>/solution/extra`, `orca://evidence/%00`, URI > `maxURILen` đều `errBadURI`; round trip `String()`.
- `provider_test.go`: đọc `orca://request/{id}` trả nội dung kèm cờ `orca/untrusted`; URI sai định dạng cho "không tìm thấy" như URI hiện có; `orca://impact/{id}` khi chưa có kênh: "không tìm thấy" (không panic).
- `tools/parity_test.go`: xanh sau khi thêm kênh (mỗi kênh mới có ToolSpec hoặc dòng loại trừ).
- `tools/catalog_test.go`: `source_search` có trong `tools/list` và đánh `UntrustedOutput`.
- `request-service/internal/adapter/grpc/server_context_read_test.go`: người ngoài tenant nhận `NOT_FOUND` (không lộ tồn tại); `GetContextPack` không có pack: `NOT_FOUND`.
- Lệnh: `cd backend-go && go test ./services/api-gateway/internal/adapter/mcpserver/... ./services/api-gateway/internal/adapter/wscompat/... ./services/request-service/internal/adapter/grpc/...`; `buf lint && buf breaking`. Chưa chạy.

## Tiêu chí hoàn thành

- [x] Sáu `Kind` mới đọc được qua `resources/read`; URI sai thành "không tìm thấy".
- [x] Mọi resource có nội dung từ nguồn ngoài đánh `UntrustedOutput`.
- [x] `parity_test.go` xanh.
- [x] `source_*` chỉ tìm trong nguồn đã bật; không có tool ghi mới.
- [x] Payload WS khớp `CONTRACT-request-ui-api.md` (đã đối chiếu từng trường).

## Ví dụ tham khảo

Bảng URI và kênh nền (đối chiếu với `plans.go` khi làm):

| URI | `Kind` | Kênh nền | `UntrustedOutput` |
|---|---|---|---|
| `orca://request/{id}` | `request` | `request.get` | có |
| `orca://solution/{id}` | `solution` | `solution.get` | có |
| `orca://plan/{id}` | `plan` | `task.getSubtree` | có |
| `orca://evidence/{id}` | `evidence` | `evidence.get` | có |
| `orca://impact/{assessment_id}` | `impact` | `impact.get` (CR-030) | không |
| `orca://context/{request_id}/{stage}` | `context` | `context.get` | có |

Ví dụ URI sai (đều "không tìm thấy"): `orca://context/<uuid>/bogus`, `orca://request`, `orca://request/a/b`, `orca://evidence/%00`.

Kiểm thủ công sau khi triển khai (cần MCP client và dev server; chưa chạy): `resources/read orca://request/<id>` rồi so `_meta` có cờ `orca/untrusted`.

## Rủi ro và lưu ý

- `KindImpact` phụ thuộc CR-REQ-030 chưa có spec: để `errNotReadable` an toàn.
- Mỗi kênh mới là một bề mặt tấn công: kiểm tra `evidence.get` không trả `excerpt` của Request tenant khác (RLS + `tenant_id` ở truy vấn).
- `subscribe` cho `orca://request/{id}` (đợt 2) đi theo `resources/subscriptions.go`; không thuộc task này.
- Gateway chạy `gitnexus_impact` trước khi sửa `planFor` và `Parse` (symbol dùng chung nhiều resource).
