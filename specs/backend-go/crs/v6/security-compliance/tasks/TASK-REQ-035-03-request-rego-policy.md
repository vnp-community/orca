# TASK-REQ-035-03: `request.rego` (ma trận hành động theo vai trò) và đưa vào ảnh container

**From Solution:** BE-REQ-SOL-035 (mục C)
**Priority:** P0
**Service:** `backend-go/policy/orca-authz`, `request-service`, `backend-go/ci`
**File:** `backend-go/policy/orca-authz/request.rego` (mới), `backend-go/policy/orca-authz/request_test.rego` (mới), `backend-go/services/request-service/internal/adapter/opaclient/request_policy.go` (mới), `.../internal/adapter/opaclient/request_policy_test.go` (mới), `backend-go/services/request-service/deploy/Dockerfile` (sửa/mới: `COPY policy`), `backend-go/ci/check-opa-bundle-in-images.sh` (sửa: thêm `request-service`)
**Depends on:** BE-REQ-SOL-001 (module, Dockerfile)
**Status:** `[x] DONE`

---

## Context

- `policy/orca-authz/project.rego` là khuôn: `package orca.authz.project`, `import rego.v1`, bảng `action_roles`, `default allow := false`, admin toàn cục luôn được. Test cạnh: `project_test.rego`. Gói khác: `task_grant.rego`, `repo.rego`, `admin.rego`.
- `services/project-service/internal/adapter/opaclient/client.go`: mẫu `Client{evaluator *policy.Evaluator}`, hằng `decisionQuery = "data.orca.authz.project.allow"`, gọi `evaluator.Decision(ctx, decisionQuery, map[string]any{...})`. `common/policy/evaluator.go`: `NewEvaluator(bundlePath)`, `Decision(ctx, query, input)`, `Warm`, `ValidateBundleAt`.
- Lý do dùng Rego cho ma trận (CR 2.3): dữ liệu nhỏ, đúng chỗ OPA đã dùng, "một nơi duyệt toàn bộ luật quyền" của `arch/07`; tập người duyệt Approval phụ thuộc dữ liệu nên giữ Go (CR-REQ-010).
- `ci/check-opa-bundle-in-images.sh` hiện chỉ có `svcs=(auth-service task-service annotation-service project-service mcp-service)` và build `docker build -f services/$svc/deploy/Dockerfile`; thêm `request-service` vào mảng.
- Ma trận nhóm → vai trò (từ solution mục B, C) phải **khớp từng chữ** với bảng RPC → nhóm của `rpc_catalog.go` (task 04). Agent chỉ được RPC trong `agent_rpcs`, dù người dùng phía sau là admin.

## Việc cần làm

1. Viết `request.rego` đúng như solution mục C: `group_roles`, `agent_rpcs`, `caller_roles` (tập suy ra từ `input.caller_project_role` khác rỗng và `input.is_reporter`), `default allow := false`, sáu luật `allow`. Comment đầu file nêu input `{action, rpc, caller_global_role, caller_project_role, is_reporter, actor_type}` và liên hệ `project.rego`.
2. Quyết định biên (ghi trong comment ngắn): nhóm `decide` và `authenticated` chỉ là **cửa trước** (mọi người không phải agent); tập người duyệt do Go (CR-010) kiểm tiếp; nhóm `admin` chỉ admin toàn cục (không nằm trong `group_roles`, nên chỉ luật admin áp dụng); nhóm `internal` không bao giờ qua Rego (do `Guard`).
3. `request_test.rego`: dùng `opa test`. Viết test dạng bảng bằng `with input as {...}`:
   - với từng nhóm trong `group_roles` × `{admin, owner, member, reporter, stranger}`: kết quả đúng ma trận;
   - `agent` × từng RPC trong `agent_rpcs` được khi người dùng phía sau đủ vai trò (`owner`), không được khi là `stranger`;
   - `agent` × `StartPhase`, `Approve`, `SetRequestFlowSettings`, `GeneratePlan`, `CancelRequest` luôn `false`, kể cả khi `caller_global_role = "admin"`;
   - `caller_global_role` rỗng với `caller_project_role` rỗng ⇒ `false`;
   - `action` không có trong bảng (ví dụ `"bogus"`) ⇒ `false`;
   - `decide`: user (kể cả không vai trò) `true`, agent `false`;
   - `authenticated`: user `true`, agent `false`.
4. `request_policy.go`: `type RequestPolicy struct{ evaluator *policy.Evaluator }`, `const requestDecisionQuery = "data.orca.authz.request.allow"`, `func (p *RequestPolicy) Decision(ctx context.Context, in RequestPolicyInput) (bool, error)` với `RequestPolicyInput{Action, RPC, CallerGlobalRole, CallerProjectRole string; IsReporter bool; ActorType string}` chuyển thành map (khoá snake_case đúng như Rego). Lỗi đánh giá ⇒ trả `false, err` (fail closed); `Warm(ctx, requestDecisionQuery)` ở khởi động để lỗi bundle làm dịch vụ không lên.
5. Dockerfile: bảo đảm `COPY policy ./policy` (và bundle vào ảnh runtime ở đúng đường dẫn mà các service khác dùng: đọc `services/project-service/deploy/Dockerfile` để biết `OPA_BUNDLE_PATH` và đích sao chép; **không** đoán). Biến cấu hình `OPA_BUNDLE_PATH` thêm vào `config.go` của `request-service` (khuôn project-service).
6. `check-opa-bundle-in-images.sh`: thêm `request-service` vào `svcs`; cập nhật dòng `echo "All ${#svcs[@]} images..."` tự động (đã dùng độ dài mảng).
7. Thêm bước CI `opa test backend-go/policy/orca-authz -v` vào workflow `backend-go-request-service.yml` (nếu workflow policy chung đã chạy `opa test` cho cả thư mục thì không thêm; kiểm `.github/workflows/` trước, chưa kiểm chứng).

## Kiểm thử

- `opa test backend-go/policy/orca-authz/ -v` (cần `opa` CLI; CI hiện có thể đã cài: kiểm).
- `request_policy_test.go`: dùng `policy.NewEvaluator` trỏ thư mục `policy/orca-authz` thật (đường dẫn tương đối `../../../../../policy/orca-authz` từ package; kiểm đúng độ sâu khi viết): các trường hợp chính của bảng: `owner`+`triage` ⇒ true; `member`+`triage` ⇒ false; `agent`+`ClassifyRequest` với `owner` ⇒ true; `agent`+`Approve` ⇒ false; bundle không tồn tại ⇒ lỗi (và `Decision` trả false).
- `ci/check-opa-bundle-in-images.sh` chạy được với Docker (chạy tay; chưa chạy ở thời điểm viết).
- Lệnh: `cd backend-go && opa test policy/orca-authz -v && go test ./services/request-service/internal/adapter/opaclient/... && bash ci/check-opa-bundle-in-images.sh`.

## Tiêu chí hoàn thành

- [x] Ma trận Rego khớp bảng nhóm ở solution (có test đối chiếu tự động với `rpc_catalog.go` ở task 04).
- [x] `agent` luôn bị từ chối ở `decide`, `execute`, `admin`, `lifecycle`, `plan`, kể cả với admin.
- [x] Ảnh `request-service` chứa `policy/orca-authz/*.rego` (script CI xanh).
- [x] Lỗi đánh giá hoặc thiếu bundle làm dịch vụ không khởi động hoặc từ chối (fail closed).
- [x] `opa test` xanh.

## Ví dụ tham khảo

Bảng kết quả kỳ vọng (viết thành test; `Y` là được, `-` là từ chối):

| Nhóm | admin | owner | member | reporter | stranger | agent (user phía sau là owner) |
|---|---|---|---|---|---|---|
| `read` | Y | Y | Y | Y | - | Y (nếu RPC trong `agent_rpcs`) |
| `create` | Y | Y | Y | - | - | Y (`CreateRequest`, `SpawnChildRequest`) |
| `triage` | Y | Y | - | Y | - | chỉ `ClassifyRequest` |
| `analyze` | Y | Y | - | Y | - | chỉ `GenerateSolution` |
| `plan` | Y | Y | - | Y | - | - |
| `execute` | Y | Y | - | - | - | - |
| `lifecycle` | Y | Y | - | Y | - | - |

## Rủi ro và lưu ý

- Hai nguồn sự thật (Rego nhóm → vai trò và Catalog RPC → nhóm): test đối chiếu ở task 04 giữ chúng không lệch.
- Quyền "member không phân loại, `StartPhase` chỉ owner|admin" là đề xuất chưa chủ sản phẩm xác nhận (Q1); đổi chỉ cần sửa `group_roles` và test.
- `common/policy.Evaluator` kiểm bundle định kỳ (`SetCheckPeriod`); đổi bundle nóng cần xác nhận không làm đứt các Request đang chạy.
- Không có vai trò dự án `viewer`: người chỉ đọc phải là `member` (Q3).
