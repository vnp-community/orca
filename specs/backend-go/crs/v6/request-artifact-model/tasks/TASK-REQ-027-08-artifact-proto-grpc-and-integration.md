# TASK-REQ-027-08: Proto `artifact.proto`, gRPC server, quyền đọc và kiểm thử tích hợp hai dialect

**From Solution:** BE-REQ-SOL-027
**Priority:** P0
**Service:** `request-service` · `proto`
**File:** `backend-go/proto/orca/request/v1/artifact.proto`, `backend-go/proto/orca/request/v1/request.proto` (sửa: thêm trường vào `Request`, `Solution`, `CreateRequestRequest`), `internal/adapter/grpc/artifact_server.go`, `internal/adapter/grpc/request_mapper.go` (sửa, của TASK-REQ-002-07), `internal/usecase/artifact_integration_test.go`, `services/request-service/README.md` (sửa) và test (mới trừ file sửa)
**Depends on:** TASK-REQ-027-04, 027-05, 027-06, 027-07, TASK-REQ-001-02 (proto khung), TASK-REQ-001-05 (gRPC server)
**Status:** [ ] TODO (một phần: bảy RPC thật và đã kiểm chứng 2026-10-08, buf sạch, kiểm mẫu nối CI; còn kịch bản (d) `CommitPlan` giả và (c) qua RPC)

---

## Context

CR-REQ-027 mục 2.10. README v6 mục 8 điểm 13: gateway không kiểm quyền OPA trước định tuyến nên mọi RPC của `request-service` **tự** kiểm quyền (tenant từ metadata, `tenant.UserID`, `tenant.Role` của `common/tenant`). Kênh WS và tool MCP gọi các RPC này do CR-REQ-016/017 đặt tên (xem `specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md`, đọc để khớp payload; không sửa file đó). `AppendRequestRevision` chỉ là use case nội bộ (CR-REQ-028 gọi), không RPC công khai; người dùng sửa nội dung qua `EditRequestContent` với `expected_revision`.

Thêm trường vào message có sẵn là cộng thêm (additive), `buf breaking` không báo; **không** đổi số trường đã có. `request.proto` hiện do TASK-REQ-001-02 và TASK-REQ-007-06, 012-04 mở rộng: đọc file lúc làm để chọn số trường kế tiếp, không dùng số trong tài liệu.

## Việc cần làm

1. `artifact.proto` (package `orca.request.v1`, `go_package` theo `buf.gen.yaml`): `rpc GetRequestCoverage(GetRequestCoverageRequest) returns (GetRequestCoverageResponse)` (rows `{ac_id, task_id, check_id, plan_task_id}` + `uncovered_ac_ids`), `rpc GetArtifactGraph(...)` (`repeated ArtifactEdge{rel, from_kind, from_id, to_kind, to_id}`, `bool partial`), `rpc ExportArtifactProjection(...)` (`filename`, `content`, `digest`), `rpc ResolveArtifactRef(...)` (`kind`, `artifact_id`, `request_id`), `rpc ListRequestRevisions(...)` (phân trang `page_token`, `RequestRevisionSummary{revision, cause, digest, actor_id, actor_kind, created_at}`; **không** trả `snapshot` ở danh sách), `rpc GetRequestRevision(...)` (có `snapshot_json`), `rpc EditRequestContent(...)` (`request_id`, `expected_revision`, `title`, `body`, `acceptance_criteria_json`, `type_fields_json`).
2. `request.proto` (sửa): `Request` thêm `acceptance_criteria_json`, `type_fields_json`, `content_revision`, `content_schema_version`;
   - `Solution` thêm `display_id`, `seq`, `provenance_json`, `input_request_revision`, `content_digest`
   - `CreateRequestRequest` thêm `acceptance_criteria_json`, `type_fields_json` tuỳ chọn.
   - Chạy `buf lint`, `buf generate`, `buf breaking --against '.git#branch=main'`.
3. `artifact_server.go`: mỗi handler: `tenant.RequireTenantID`;
   - quyền **đọc** theo cùng hàm `canReadRequest(ctx, request)` mà `GetRequest` của TASK-REQ-002-07 dùng (không tạo bản thứ hai)
   - `EditRequestContent` theo quyền ghi mức Request (tạm: `reporter_id` hoặc `role=admin`, ghi chú Q1 README mục 8)
   - lỗi qua `apperrors.ToGRPCStatus`
   - id lạ hoặc khác tenant đều `NOT_FOUND` (không lộ tồn tại).
4. `request_mapper.go` (sửa): ánh xạ các trường mới trong cả hai chiều;
   - `acceptance_criteria_json` và `type_fields_json` là chuỗi JSON chuẩn tắc, không dựng struct proto lồng (tránh đổi schema hai nơi).
5. `ExportArtifactProjection`: `format` rỗng hoặc `markdown`;
   - tệp trả về có `Content-Disposition`-tương-đương là `filename` dạng `<display_id>.md` (ví dụ `SOL-142.2.md`)
   - kích thước tối đa 1 MB.
6. `artifact_integration_test.go` (tag `integration`, Postgres và MySQL): kịch bản (a) `CreateRequest` có AC, kiểm `request_revisions` revision 1;
   - (b) `EditRequestContent` hai lần liên tiếp, kiểm revision 2 và 3, `ListRequestRevisions` đúng thứ tự, `GetRequestRevision(1)` vẫn có AC cũ sau khi AC bị `retired` ở revision 3
   - (c) `GenerateSolution` giả (fake AI) rồi `ResolveArtifactRef("SOL-<n>.1")`
   - (d) `CommitPlan` giả (fake task-service) rồi `GetRequestCoverage` và `GetArtifactGraph` (có `derived_from`, `implements`, và `contains` từ fake `GetSubtree`)
   - (e) đọc chéo tenant mọi RPC trả `NOT_FOUND`.
7. README của service: mục "Mô hình artifact" liệt kê RPC mới, quy tắc bất biến ở CR 2.9 (Request đổi bằng revision; Solution `approved` không sửa; spec Plan khoá khi duyệt), biến môi trường (nếu có) và phần **chưa kiểm chứng** (thư viện JSON Schema, tải thực tế).
8. Bổ sung `testdata` mẫu dùng chung hai service (copy có kiểm CI) cho `task.schema.json` và các mẫu spec;
   - thêm bước CI `diff -r request-service/testdata/artifacts/task task-service/testdata/artifacts` vào workflow của `request-service` (TASK-REQ-001-06) hoặc script `scripts/check-artifact-samples.sh`.

## Kiểm thử

- `TestArtifactServer_Read_RequiresTenant`
- `TestArtifactServer_CrossTenantNotFound` (từng RPC)
- `TestArtifactServer_ListRevisions_PaginatesAndOmitsSnapshot`
- `TestArtifactServer_GetRevision_ReturnsSnapshot`
- `TestArtifactServer_Export_Markdown_FilenameAndDigest`
- `TestArtifactServer_Export_UnknownFormat_InvalidArgument`
- `TestArtifactServer_EditContent_StaleRevision`
- `TestArtifactServer_EditContent_NotEditableAfterAnalysis`.
- `TestRequestMapper_NewFields_RoundTrip`.
- Hợp đồng: `buf lint`, `buf breaking`; kiểm số trường không bị dùng lại (so với nhánh chính).
- Integration (a) đến (e) ở bước 6, hai dialect.
- Kiểm tra mẫu dùng chung: script so khớp hai thư mục mẫu, fail khi lệch.
- Lệnh: `cd /opt/repos/orca/backend-go && (cd proto && buf lint && buf breaking --against '../.git#branch=main,subdir=backend-go/proto') ; go test ./services/request-service/internal/adapter/grpc/... -run 'Artifact|RequestMapper' && go test -tags=integration ./services/request-service/internal/usecase/... -run ArtifactIntegration`. (Cú pháp `buf breaking` theo `proto/buf.yaml` thật; kiểm lại lúc làm.)

## Tiêu chí hoàn thành

- [x] Bảy RPC hoạt động, mỗi RPC có test chéo tenant trả `NOT_FOUND`.
- [x] `ListRequestRevisions` không trả snapshot; `GetRequestRevision` trả đúng AC của revision cũ.
- [x] `buf lint` và `buf breaking` sạch; không dùng lại số trường. (2026-10-08: `buf lint --path` cho `artifact|clarification|decision|solution.proto` và `buf breaking --path orca/request --path orca/task --against ../../.git#branch=main,subdir=backend-go/proto` đều không báo lỗi; `.proto` không đổi)
- [ ] Năm kịch bản tích hợp xanh trên Postgres và MySQL. (a), (b), (e) qua RPC trên Postgres; (c), (d) chưa làm; MySQL ở mức contract
- [x] README ghi rõ quy tắc bất biến và phần chưa kiểm chứng.
- [x] Kiểm CI mẫu dùng chung giữa hai service có mặt. (`scripts/check-artifact-samples.sh` chạy xanh với `task-service/testdata/artifacts/task/canonical_cases.json` giống từng byte; đã thêm bước vào `.github/workflows/backend-go-request-service.yml`)

## Rủi ro và lưu ý

- Chưa kiểm chứng: hiệu năng `GetArtifactGraph` với cây lớn, và `ExportArtifactProjection` cho Plan phụ thuộc `PlanReader` gọi hai RPC của `task-service` (`GetSubtree`, `GetTaskSpecs`).
- Quyền ghi mức Request chưa chốt (README v6 mục 8, cuối): `EditRequestContent` dùng quy tắc tạm; đổi khi CR-REQ-010 chốt.
- Trường JSON trong proto (`*_json`) tránh nhân đôi schema nhưng làm client phải tự parse; frontend (CR-REQ-018) dùng lại JSON Schema nếu xuất schema qua RPC (Q1 của SOL-027, chưa quyết).
- Thêm RPC vào `RequestService` làm `MustCoverAll`/bảng loại trừ `parity_test.go` của MCP đỏ nếu có kênh gateway mà thiếu `ToolSpec` (README mục 8 điểm 13); báo người giữ CR-REQ-016/017.

## Tiến độ (rf/art, 2026-10-08)

Đã làm và kiểm chứng (`go test ./internal/adapter/grpc`; `go test -tags integration ./cmd/server -run ArtifactAndClarificationFlow`):
- 7 RPC thật (`EditRequestContent`, `ListRequestRevisions`, `GetRequestRevision`, `GetRequestCoverage`, `GetArtifactGraph`, `ExportArtifactProjection`, `ResolveArtifactRef`) ở `adapter/grpc/server_artifact.go`, nối ở `cmd/server/wire_artifact.go` (một lời gọi `wireArtifact` trong `main.go`); test `TestArtifactServer_*` (cần tenant, chéo tenant `NOT_FOUND` cho từng RPC, phân trang không snapshot, snapshot, export tên tệp + digest, định dạng lạ `InvalidArgument`, sửa với revision cũ và sau `analyzing`) và `TestRequestMapper_NewFields_RoundTrip`. Chạy thật qua dịch vụ đầy đủ trên Postgres + NATS: tạo Request có AC, sửa hai lần, danh sách revision, đọc revision 1, resolve `REQ-n`, export, bảng phủ, đồ thị, đọc chéo tenant.
- Proto: **không sửa** `.proto` (đã đủ message, trường và RPC từ nhánh `rf/proto`). Trường mới của `Request` được ánh xạ ở `request_mapper.go` (JSON xuất ra dạng chuẩn tắc); `CreateRequest` nhận `acceptance_criteria_json`/`type_fields_json`.
- README của service có mục "Mô hình artifact".

Chưa làm: kịch bản tích hợp (c) `GenerateSolution` giả rồi `ResolveArtifactRef(SOL-n.1)` và (d) `CommitPlan` giả rồi bảng phủ + đồ thị với `contains` từ `GetSubtree` giả (cần 027-07; phần đơn vị nằm trong `MintSolutionID`, `GetArtifactGraph` và contract hai dialect nhưng chưa qua RPC); kịch bản (a), (b), (e) mới chạy qua RPC trên Postgres, MySQL chỉ ở mức contract repository (`TestMySQL_ArtifactContract`); `buf lint` báo lỗi có sẵn của repo (tên response `Empty`) và `buf breaking` chưa chạy được ở worktree (không có `.git` trong `backend-go`); kiểm CI mẫu dùng chung: có script `scripts/check-artifact-samples.sh` nhưng chưa nối vào workflow (bản của task-service nằm ở nhánh khác).

## Tiến độ sau hợp nhất (2026-10-08)

(a), (b), (e) chạy qua RPC thật trên cả Postgres và MySQL (`TestRun_ArtifactAndClarificationFlow_Postgres|MySQL`). (c) `GenerateSolution` rồi tra `SOL-n.1`, `SOL-n.1/opt-k`: kiểm ở mức contract hai dialect (`SolutionContract/SolutionArtifactsDecisionsAndGates` gọi `ArtifactIndexRepository.Resolve`); chưa qua RPC `ResolveArtifactRef` vì bộ khởi tạo của contract không dựng `ArtifactServer`. (d) `CommitPlan` giả: chưa làm (SOL-012 còn là stub).
