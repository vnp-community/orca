# CR-CV-013 — Phân quyền, audit, hạn mức đồng thời và che dữ liệu nhạy cảm

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-013 |
| **Tên** | Kiểm quyền thành viên project/tenant (OPA), cờ tính năng theo tenant, bảo vệ RPC chỉ-gateway, audit, hạn mức đồng thời và tốc độ theo tenant/dev server/người dùng, che bí mật |
| **Loại** | Feature (bảo mật và vận hành) |
| **Priority** | 🔴 P0 (chặn lộ mã nguồn chéo tenant/project và chặn làm quá tải dev server) |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-010, 011, 012 |
| **Mở khoá** | CR-CV-021 (collector gọi `AgentCallGate`), 022, 036, 040, 072, 073 |
| **Tác động** | `backend-go/services/code-intel-service/{internal/domain, internal/usecase, internal/adapter/{grpc,grpcclient,callgate,policyengine}}` (mới), `backend-go/policy/orca-authz/code_intel.rego` và `code_intel_test.rego` (mới), `backend-go/ci/check-opa-bundle-in-images.sh` (đã thêm ở CR-CV-010). Không sửa `common/*`, `auth-service`, `project-service` |

---

## 1. Bối cảnh và vấn đề

README v7 mục 6 đặt "mọi use case gọi `tenant.RequireTenantID`" và nghiên cứu [03 §5](../../../research/view-code/03-command-and-data-flow.md) viết "quyền đọc code-intel tương đương quyền đọc worktree (tenant + project membership); `reindex` cần quyền ghi". Khi đọc cách các service hiện có thực hiện:

### 1.1 Mô hình quyền hiện có

| Nơi | Cách làm | Bằng chứng |
|-----|----------|------------|
| `api-gateway` | Xác thực người dùng rồi chuyển danh tính qua metadata: `x-orca-tenant-id`, `x-orca-user-id`, `x-orca-role`, `x-orca-client-ip` | `api-gateway/internal/adapter/grpc/dial.go` dòng 49 đến 57 (`AttachIdentity`) |
| Service nội bộ | `grpcmw.TenantExtractionInterceptor` đọc bốn khoá đó vào ctx; use case gọi `tenant.RequireTenantID`/`tenant.UserID`/`tenant.Role`. Thiếu `Role` phải coi là không phải admin (fail closed) | `common/grpcmw/grpcmw.go` dòng 58 đến 80; `common/tenant/tenant.go` |
| `project-service` | `requireProjectAccess`: tra membership (`owner`/`member`) rồi hỏi OPA `data.orca.authz.project.allow` với `{caller_project_role, caller_global_role, action}`; `admin` toàn cục luôn qua; mọi lỗi tra cứu là từ chối; ghi audit cả allow lẫn deny. `GetProject` còn lọc theo tenant | `project-service/internal/usecase/authorization.go` dòng 103 đến 144; `policy/orca-authz/project.rego` |
| `task-service` | `ResolvePermission`: BFS quyền + OPA `task_grant.rego` + `auditclient`; "không có quyền" và "không tồn tại" cho cùng lỗi `TASK_NO_GRANT` để không lộ sự tồn tại | `task-service/internal/usecase/resolve_permission.go` |
| `mcp-service` | Cờ theo tenant (`tenant_settings.enabled`, mặc định từ env), kill switch, hạn mức cửa sổ trượt trong DB, `internalcaller.Guard` cho RPC chỉ-gateway, `mcp.rego` từ chối mặc định khi rủi ro không rõ | `mcp-service/internal/domain/tenant_settings.go`; `internal/adapter/postgres/tool_call_repository.go` dòng 127 đến 155; `policy/orca-authz/mcp.rego` dòng 8 đến 14, 179 |
| Kênh `files.*`, `git.*` ở gateway | **Không có kiểm thành viên project**: grep `authoriz\|member\|permission\|RequireProject` trong `wscompat/channels_files.go` và `channels_git.go` ra rỗng, và `git-gateway-service/internal` không có kiểm membership/OPA | grep chỉ-đọc ngày 2026-10-05 |

Hệ quả: đọc mã trên dev server hiện chỉ giới hạn theo tenant. Code-intel sẽ là đường đọc đầu tiên **kiểm membership theo project**, nên không thể "sao" cách kiểm của `files.*`; phải gọi `project-service` (như CR-CV-012). Chưa kiểm chứng gateway hay `git-gateway-service` có kiểm ở tầng khác; nếu có, đó là bổ sung, không thay thế.

### 1.2 Ranh giới tin cậy yếu

- Không có xác thực service-to-service bằng mTLS trong code: `common/internalcaller` ghi rõ "There is no mesh peer-identity interceptor in this codebase yet… shared-secret metadata check as defense in depth" (`internalcaller.go` dòng 1 đến 5, "defense in depth"). Danh tính trong metadata là tin cậy theo mạng. Một tiến trình bất kỳ chạm được cổng 9090 có thể giả `x-orca-tenant-id`/`x-orca-user-id`. Với dữ liệu là mã nguồn, cần `internalcaller.Guard` trên **mọi** RPC (mẫu `mcp-service`: `MCP_INTERNAL_CALLER_TOKEN`, gateway gửi cùng token bằng `ClientInterceptor`, `api-gateway/cmd/server/mcp_governance_wiring.go` dòng 24 đến 33).
- `Guard` với token rỗng từ chối mọi lời gọi (fail closed, `internalcaller.go` dòng 22 đến 24).

### 1.3 Giới hạn của `common/auditclient`

`auditclient.Append(ctx, tenantID, actorID, action, target, outcome, ip)` (`common/auditclient/client.go`):

1. Đồng bộ nhưng nuốt lỗi RPC (`_, _ = c.auth.AppendAuditEntry(...)`): mất bản ghi mà không ai biết; không có thử lại, không có đo.
2. Chỉ truyền 6 trường. Proto `AppendAuditEntryRequest` có thêm `actor_type`, `target_type`, `target_id`, `metadata_json` (`auth.proto` dòng 371 đến 383) nhưng `Append` không điền; `auth-service` coi `actor_type` rỗng là `user`.
3. `outcome` phải đúng `allowed` hoặc `denied`; giá trị khác bị `domain.NewAuditEntry` từ chối với `ErrInvalidOutcome` (`auth-service/internal/domain/audit.go` dòng 129 đến 135) và lỗi đó bị `Append` nuốt: **bản ghi mất lặng lẽ**. (Rỗng thì mặc định `allowed`.)
4. Không có hạn chót riêng: dựa vào ctx của lời gọi; `auth-service` chậm làm chậm request gốc (chú thích "non-blocking" của repo nghĩa "không chặn quyết định khi lỗi", không phải bất đồng bộ).
5. `mcp-service` không dùng `Append`: ghi sự kiện vào outbox `orca.mcp.audit.appended` và `auth-service` nhận bằng consumer riêng (`auth-service/internal/adapter/natsconsumer/mcp_audit_ingest.go`; payload `audit_id`, `actor_id`, `action`, `actor_type`, `target_type`, `target_id`, `outcome`, `metadata`, xem `mcp-service/internal/domain/audit_event.go`). Cách này bền hơn nhưng cần consumer riêng cho mỗi subject ở `auth-service`.
6. Giới hạn độ dài `target` ở `audit_log` chưa kiểm chứng.

### 1.4 Hạn mức

- Không có thư viện giới hạn tốc độ dùng chung; `golang.org/x/time v0.15.0` chỉ là phụ thuộc gián tiếp (`// indirect`) ở nhiều `go.mod` (task, notification, ...) và không có mã nguồn nào gọi `rate.NewLimiter` (grep `rate.NewLimiter` ra rỗng).
- `apperrors` không có kind `ResourceExhausted`; `ToGRPCStatus` ánh xạ kind lạ thành `codes.Unknown` hoặc `Internal` (`apperrors.go` dòng 109 đến 130). `mcp-service` tự trả `status.Error(codes.ResourceExhausted, ...)` (`internal/adapter/grpc/governance_server.go` dòng 299).
- Mã nguồn không có kill switch chung cho tính năng mới; mẫu cờ theo tenant là bảng cài đặt riêng của service (`mcp.tenant_settings`, và `request.tenant_settings` ở CR-REQ-025).
- Chi phí mỗi lần gọi CLI ~1,8 giây và DB CodeGraph ~1,2 GB (README v7 mục 1), `gitnexus analyze` Orca mất nhiều phút (nghiên cứu 02 §4): một người dùng có thể làm quá tải dev server dùng chung cho cả agent lập trình.

## 2. Giải pháp đề xuất

### 2.1 Chuỗi kiểm tra cho mọi RPC

| # | Bước | Nơi | Lỗi |
|---|------|-----|-----|
| 0 | Token nội bộ (`internalcaller.Guard` và `StreamGuard`) | interceptor | `INTERNAL_CALLER_REQUIRED` (`PermissionDenied`, mã sẵn có) |
| 1 | Có tenant và user trong ctx (`tenant.RequireTenantID`, `tenant.UserID`) | `grpcmw` + interceptor stream (CR-CV-010 2.4) | `CODEINTEL_NO_TENANT` (`Unauthenticated`) |
| 2 | Giới hạn thô theo `(tenant, user)` chống bão request (mục 2.6, L1) | bộ nhớ, rẻ | `CODEINTEL_RATE_LIMITED` |
| 3 | Cờ `code_intel_enabled` của tenant (mục 2.3) | `tenant_settings` + cache | `CODEINTEL_FEATURE_DISABLED` |
| 4 | Quyền theo project và hành động (mục 2.2) | `project-service` + OPA | `CODEINTEL_NOT_AUTHORIZED` |
| 5 | Phân giải đích (CR-CV-012) | | mã của CR-CV-012 |
| 6 | Cổng gọi agent: đồng thời, tốc độ theo dev server/tenant (mục 2.6, L2) | collector (CR-CV-021) | `CODEINTEL_CONCURRENCY_LIMIT`, `CODEINTEL_RATE_LIMITED` |
| 7 | Thực thi; **kiểm quyền luôn đứng trước tra cache** (CR-CV-022) | | |
| 8 | Che dữ liệu nhạy cảm trên đầu ra (mục 2.5) | | |
| 9 | Audit (mục 2.4) | | |

Bước 3 đứng trước 4 để tenant chưa bật không gây tải lên `project-service`. Người dùng không phải thành viên và project không tồn tại hoặc thuộc tenant khác đều nhận `CODEINTEL_NOT_AUTHORIZED` (không lộ sự tồn tại, như `TASK_NO_GRANT`).

### 2.2 Quyền theo project: OPA `code_intel.rego` (mới)

Vai trò lấy từ `project-service`, không lưu cục bộ:

1. `GetProject(project_id)` với danh tính của người gọi (CR-CV-012 2.3): thành công nghĩa là project thuộc tenant của ctx **và** người gọi là thành viên hoặc admin toàn cục (`requireProjectAccess any_member`, đã lọc tenant: `repo.Get(ctx, tenantID, id)`). Đây là cổng tenant+thành viên. Không tin `ListMembers`/`GetRepo`/`GetWorktree` làm cổng tenant (mục 1 của CR-CV-012).
2. `ListMembers(project_id)` tìm `role` (`owner`/`member`) của người gọi; admin toàn cục không có dòng thì `role=""`.
3. Hỏi OPA `data.orca.authz.codeintel.allow` với `{caller_project_role, caller_global_role, action}` (`caller_global_role` từ `tenant.Role`; rỗng nghĩa là không phải admin).
4. Cache quyết định `(tenant, user, project, action)` tối đa `CODEINTEL_AUTHZ_CACHE_TTL` = 10 giây (gỡ người khỏi project mất hiệu lực trong ≤ 10 giây; chưa có sự kiện thành viên để huỷ sớm).

Bảng hành động (`project.rego` chỉ có `owner`/`member`; `viewer` là việc sau theo chú thích của file đó):

| `action` | RPC | `owner` | `member` | admin toàn cục |
|----------|-----|:-------:|:--------:|:--------------:|
| `read` | `BindRepo`, `ListRepoBindings`, `GetIndexStatus`, `GetStructure`, `GetArchitecture`, `ListDataFlows`, `GetDataFlow`, `GetErd`, `GetStorageMap`, `GetSubgraph`, `GetImpact`, `GetRouteMap`, `GetChangeOverlay`, `GetReadingOrder`, `ListFindings`, `GetContractDiff`, `GetReviewState`, `GetC4Overrides`, `GetReindexJob`, `StreamCodeIntelEvents` | ✓ | ✓ | ✓ |
| `read_source` | `GetSymbol` (trả mã nguồn) | ✓ | ✓ | ✓ |
| `review_write` | `SaveReviewState`, `DismissFinding` (và khôi phục) | ✓ | ✓ | ✓ |
| `reindex` | `RequestReindex` | ✓ | ✓ | ✓ |
| `c4_write` | `SaveC4Overrides` | ✓ | ✗ | ✓ |

`read_source` tách riêng dù hiện cùng quyền với `read`, để siết sau (ví dụ chỉ vai trò repo `developer` trở lên) mà không đổi RPC. `reindex` cho `member` vì người review thường là thành viên; hạn mức (2.6) chặn lạm dụng. `c4_write` chỉ `owner` vì ghi đè C4 là tri thức chung của repo (CR-CV-011: khoá theo `repo_id`). Vai trò chức năng trên repo (`repo_members`: `developer`/`lead`/`admin`, `repo.rego`) **không** dùng ở MVP vì đọc worktree hiện không đòi hỏi nó (`ListRepos` chỉ cần `any_member`); xem Q1.

File `code_intel.rego` theo khuôn `project.rego` (`default allow := false`; `allow if input.caller_global_role == "admin"`; bảng `action_roles`). Nạp bằng `policy.NewEvaluator(cfg.OPABundlePath)` và `Warm(ctx, "data.orca.authz.codeintel.allow")` lúc khởi động: lỗi nạp thì **không khởi động** (như `task-service/cmd/server/main.go` dòng 293 đến 298 và `mcp-service`). Test Rego `code_intel_test.rego` chạy trong `make opa-test` (`opa test policy/orca-authz/`). Dockerfile và compose đã mount bó Rego (CR-CV-010 2.8).

Mọi lỗi tra cứu (project-service lỗi, OPA lỗi) là **từ chối** (`CODEINTEL_NOT_AUTHORIZED` cho mất quyền; lỗi hạ tầng trả `Internal` với mã `CODEINTEL_AUTHZ_UNAVAILABLE`), không bao giờ cho qua.

### 2.3 Cờ `code_intel_enabled` (O8)

Dùng `tenant_settings` của CR-CV-011. Hiệu lực = dòng của tenant có `code_intel_enabled=true`; không có dòng thì tạo lười với giá trị `CODEINTEL_TENANT_DEFAULT_ENABLED` (mặc định `false`, mẫu `mcp-service`); lỗi đọc là `false`. Cache 30 giây. CR này **chỉ đọc** cờ; RPC bật/tắt, audit của việc đổi cờ, hiển thị cho frontend và kế hoạch rollout là CR-CV-073. Cờ tắt khi đang có job reindex: job tiếp tục (agent không bị dừng) nhưng mọi RPC mới trả `CODEINTEL_FEATURE_DISABLED`; ngoại lệ `GetReindexJob` để xem kết quả. `codeintel.indexChanged` đến khi cờ tắt vẫn được xử lý (huỷ cache).

### 2.4 Audit

Mọi bản ghi qua cổng `AuditRecorder.Record(ctx, AuditEntry{Action, Target, Outcome})` ở `internal/usecase/ports.go`; hiện thực `internal/adapter/grpcclient/audit_recorder.go` bọc `auditclient.Client.Append` (cách mà `project-service` và `task-service` dùng):

- Hàng đợi có giới hạn (256 mục, 2 worker, hạn chót 2 giây mỗi lời gọi) để RPC không chờ `auth-service`; đầy thì bỏ và tăng bộ đếm `codeintel_audit_dropped_total` (CR-CV-071); khi tắt máy xả tối đa 5 giây. Đây là thay đổi so với `Append` đồng bộ của repo, có chủ ý (mục 1.3 điểm 4).
- `Outcome` là hằng `AuditAllowed`/`AuditDenied`; hàm dựng từ chối giá trị khác ngay lúc biên dịch/kiểm thử (mục 1.3 điểm 3).
- `actor_id` = `tenant.UserID`, `ip` = `tenant.ClientIP`, `tenant` từ ctx. `target` ngắn, dạng `loại:id`, tối đa 200 ký tự (chưa kiểm chứng giới hạn cột).
- Chi tiết bổ sung (id binding, commit, view, số byte, thời gian) ghi vào log có cấu trúc `slog` với `audit=true` và `trace_id`; **không** vào audit_log cho tới khi có cách mang `metadata_json` (Q3).

| Hành động (`action`) | Khi | `outcome` | `target` |
|----------------------|-----|-----------|----------|
| `codeintel.access` | OPA từ chối (mọi `action`) | denied | `project:<id>` |
| `codeintel.bind` | `BindRepo` tạo binding mới | allowed | `repo_binding:<id>` |
| `codeintel.symbol.read` | `GetSymbol` trả nội dung (mã rời dev server) | allowed | `symbol:<binding>:<key cắt 120>` |
| `codeintel.reindex.request` | `RequestReindex` được nhận | allowed | `repo_binding:<id>` |
| `codeintel.reindex.limit` | `RequestReindex` bị hạn mức chặn | denied | `repo_binding:<id>` |
| `codeintel.review.save` | `SaveReviewState` thành công | allowed | `review:<binding>` |
| `codeintel.finding.dismiss`, `codeintel.finding.restore` | bỏ qua/khôi phục phát hiện | allowed | `finding:<repo>:<key cắt 120>` |
| `codeintel.c4.save` | `SaveC4Overrides` thành công | allowed | `c4:<repo>:<container>` |

Không audit từng lần đọc thông thường (khối lượng lớn; dùng số đo) và không audit từ chối do tốc độ ở RPC đọc. `GetSymbol` bị audit vì là đường duy nhất để mã nguồn rời dev server (nghiên cứu 06 §2 "rò rỉ mã nguồn → ghi audit"). Kết thúc job reindex đã có sự kiện outbox `orca.codeintel.reindex.finished` (CR-CV-010).

### 2.5 Che dữ liệu nhạy cảm

`internal/domain/secret_redactor.go` (mới): bộ che theo mẫu của `mcp-service/internal/domain/secret_redactor.go` (viết lại, không import chéo service), dấu `[REDACTED]`, trả `changed`. Mẫu: token GitHub (`gh[pousr]_...`, `github_pat_...`), khoá AWS (`AKIA`/`ASIA` + 16 ký tự), `Bearer <token>`, `sk-...`, `xox[abprs]-...`, JWT (`eyJ....`), khối `-----BEGIN ... PRIVATE KEY-----`, và cặp `password|passwd|secret|token|api_key|authorization` `[:=]` giá trị. Thiên về che thừa (dương tính giả chỉ hiện `[REDACTED]`).

| Chỗ áp dụng | Quy tắc |
|-------------|---------|
| `GetSymbol`: nội dung mã (`source`) và chữ ký | Luôn qua bộ che; đặt `redactionCount` |
| Chuỗi lỗi/`stderr` của agent trước khi log, lưu vào `reindex_jobs.message`, trả cho UI | Luôn qua bộ che, cắt 500 ký tự |
| Snapshot đồ thị (`graph_snapshots.payload`) | Chỉ trường văn bản tự do và nội dung mã đi qua bộ che; không duyệt toàn bộ payload 8 MiB (chi phí); tên symbol, đường dẫn không bị che. Chưa đo chi phí che |
| Đường dẫn bị chặn nội dung | File khớp danh sách chặn dưới đây: trả metadata (tên, loại, dòng) với `content=""`, `contentWithheld="sensitive_path"`, không phải lỗi |
| Kích thước | Từ chối nội dung > 200 KiB (`CODEINTEL_OUTPUT_TOO_LARGE`, mã của README) |
| Log | Không log payload, nội dung mã, ghi chú review; log `binding_id`, `view`, `bytes`, `duration`. `workspace_root` chỉ ở DEBUG |
| Ghi chú review (`review_states.notes`) | Do người dùng gõ, không che khi lưu (có thể chứa bí mật người dùng dán vào); không đưa vào log/audit |

Danh sách chặn nội dung (so khớp không phân biệt hoa thường, theo đường dẫn tương đối gốc repo): `**/.env`, `**/.env.*`, `**/*.pem`, `**/*.key`, `**/*.p12`, `**/*.pfx`, `**/*.kdbx`, `**/id_rsa*`, `**/id_ed25519*`, `**/*.tfstate`, `**/*.tfvars`, `**/credentials*`, `**/.npmrc`, `**/.netrc`, `**/secrets/**`, `**/.aws/**`, `**/.ssh/**`. Không có ngoại lệ cho thư mục test. Việc tôn trọng `.gitignore` là của agent (CR-CV-001); đây là lớp phòng thủ thứ hai ở backend. Cache là theo binding (không chia sẻ giữa project) và quyền luôn kiểm trước khi đọc cache.

### 2.6 Hạn mức và tốc độ

Hai tầng.

**L1: chống bão request** (bước 2): `rate.Limiter` theo `(tenant, user)`, `CODEINTEL_USER_RPS` = 20 yêu cầu/giây, burst 40, mọi RPC. Chống làm quá tải `project-service` (mỗi RPC chưa cache cần 2 đến 4 lời gọi nội bộ).

**L2: cổng gọi agent** `AgentCallGate` (`internal/usecase/agent_call_gate.go`, hiện thực `internal/adapter/callgate/token_bucket_gate.go`, mới). CR-CV-021 **bắt buộc** bọc mọi lời gọi `RelayByDevServer` của `codeintel.*` bằng `Acquire`/`release`; cache trúng không tốn hạn mức.

```go
type GateClass int // ClassLight, ClassHeavy, ClassSourceRead
type GateRequest struct{ TenantID, UserID, DevServerID string; Class GateClass }
type AgentCallGate interface {
    Acquire(ctx context.Context, r GateRequest) (release func(), err error)
}
```

| Lớp | Phương thức agent | Ví dụ |
|-----|-------------------|-------|
| `light` | `codeintel.status`, `codeintel.symbol`, `codeintel.routes` | |
| `heavy` | `codeintel.overview`, `processes`, `process`, `subgraph`, `impact`, `detectChanges` | mỗi lần ~1,8 giây CLI hoặc hơn |
| `source_read` | `fs.*`/`git.*` do CR-CV-030 (ERD, hợp đồng) | số lượng file lớn |

| Hạn mức | Mặc định | Biến |
|---------|----------|------|
| Đồng thời tối đa mỗi dev server (mọi lớp) | 4 | `CODEINTEL_MAX_INFLIGHT_PER_DEV_SERVER` |
| Đồng thời `heavy` mỗi dev server | 2 | `CODEINTEL_MAX_HEAVY_PER_DEV_SERVER` |
| Đồng thời mỗi tenant | 16 | `CODEINTEL_MAX_INFLIGHT_PER_TENANT` |
| `heavy` mỗi người dùng | 30/phút, burst 10 | `CODEINTEL_HEAVY_PER_MINUTE` |
| `light` mỗi người dùng | 120/phút, burst 30 | `CODEINTEL_LIGHT_PER_MINUTE` |
| Chờ khe trống tối đa | 2 giây, quá thì từ chối | `CODEINTEL_GATE_WAIT` |

Con số là mặc định khởi đầu, **chưa đo**; CR-CV-071 đặt ngân sách thật. Từ chối trả `CODEINTEL_CONCURRENCY_LIMIT` (hết khe) hoặc `CODEINTEL_RATE_LIMITED` (hết token), kèm `retry_after_seconds=N` trong thông điệp. `AgentCallGate` luôn nhả khe khi ctx hết hạn (kiểm thử rò rỉ).

**Làm mới index (`RequestReindex`)**, kiểm trong `ReindexAdmission.Admit` (`internal/usecase/reindex_admission.go`, mới) rồi `ReindexJobRepository.Create` (CR-CV-011):

| Hạn mức | Mặc định | Cách thực thi |
|---------|----------|---------------|
| Một job đang chạy mỗi binding | 1 | `UNIQUE (active_key)` ở DB (toàn cụm, đúng cả khi nhiều bản sao) → `CODEINTEL_REINDEX_IN_PROGRESS` |
| Job đang chạy mỗi dev server | 1 (`CODEINTEL_REINDEX_PER_DEV_SERVER`) | đếm `reindex_jobs` `queued`/`running` nối với `repo_bindings.dev_server_id` (phương thức `CountActiveByDevServer`, thêm vào CR-CV-011 khi triển khai); đếm rồi tạo không nguyên tử nên chấp nhận vượt tạm thời 1 job ở cuộc đua; chưa kiểm chứng |
| Job đang chạy mỗi tenant | 2 (`CODEINTEL_REINDEX_PER_TENANT`) | như trên |
| Nghỉ sau job thành công của cùng binding | 5 phút (`CODEINTEL_REINDEX_COOLDOWN`) | so `finished_at` của job gần nhất |
| Mỗi người dùng | 6/giờ | `rate.Limiter` trong bộ nhớ |

Nghiên cứu 02 §4 và 06 §2 yêu cầu `codeintel.reindex` "nối tiếp, từ chối khi đang chạy"; agent cũng phải tự từ chối (CR-CV-004); lớp này là tuyến đầu ở backend. Job mồ côi (agent rớt) được dọn bởi bảo trì của CR-CV-011 để không giữ `active_key` mãi.

Tính chất: `rate.Limiter` và semaphore ở bộ nhớ nên với N bản sao hạn mức hiệu lực là N lần; hạn mức reindex ở DB là toàn cục. Compose dev chạy một bản (chưa kiểm chứng số bản sao production). Thêm phụ thuộc trực tiếp `golang.org/x/time` (cùng `v0.15.0` đã có ở các module khác) vào `go.mod` của service.

### 2.7 Ánh xạ lỗi sang gRPC

`apperrors` thiếu `ResourceExhausted`, và không sửa `common/apperrors` (blast radius). `internal/adapter/grpc/status_mapping.go` (mới): kiểu `domain.LimitError{Code, RetryAfter}`; hàm `toStatus(err)` thử `errors.As(err, *LimitError)` → `status.Error(codes.ResourceExhausted, code+": "+msg+" retry_after_seconds=N")`, còn lại `apperrors.ToGRPCStatus` (mẫu `mcp-service`).

| Mã | gRPC |
|----|------|
| `CODEINTEL_NOT_AUTHORIZED` | `PermissionDenied` |
| `CODEINTEL_FEATURE_DISABLED` | `FailedPrecondition` |
| `CODEINTEL_AUTHZ_UNAVAILABLE` | `Internal` |
| `CODEINTEL_RATE_LIMITED`, `CODEINTEL_CONCURRENCY_LIMIT`, `CODEINTEL_REINDEX_COOLDOWN` | `ResourceExhausted` |
| `INTERNAL_CALLER_REQUIRED` | `PermissionDenied` (sẵn có) |

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|------------|-------|
| Hỏi `project-service` mỗi lần (cache 10 giây), không sao bảng thành viên | Gỡ thành viên có hiệu lực nhanh; tránh hai nguồn sự thật |
| `GetProject` làm cổng tenant+thành viên | Các RPC tra theo id thô không lọc tenant (CR-CV-012 mục 1.2) |
| OPA `code_intel.rego` thay vì bảng Go | Đúng quy ước repo (policy nhúng, kiểm thử Rego); đổi chính sách không phải biên dịch lại logic Go |
| `internalcaller.Guard` mọi RPC, fail closed | Danh tính là metadata tin cậy theo mạng; dữ liệu là mã nguồn |
| Flag kiểm trước quyền | Không tạo tải lên `project-service` cho tenant chưa bật |
| Audit bất đồng bộ qua `auditclient.Append` | Dùng cơ chế chung của repo, tách độ trễ; chấp nhận mất bản ghi khi `auth-service` hỏng và đo được số rơi |
| Không audit đọc thường | Khối lượng; nhưng audit từ chối và đọc mã |
| Hạn mức ở bộ nhớ cho đọc, DB cho reindex | Đọc cần rẻ và có thể sai lệch nhỏ; reindex cần chính xác toàn cụm |
| Không sửa `common/apperrors`, `common/auditclient` | Blast radius lớn (hàng trăm điểm gọi `ToGRPCStatus`); thêm cách chuyển ở service |
| Che nội dung mã bằng regex bảo thủ và danh sách đường dẫn | Phòng thủ chiều sâu; agent là tuyến đầu |

## 4. Tiêu chí chấp nhận

- [ ] Mọi phương thức của `CodeIntelService` (liệt kê từ `CodeIntelService_ServiceDesc`, gồm stream) bị `internalcaller` bảo vệ; thêm RPC mới không có bảo vệ làm test đỏ. Token rỗng: mọi RPC bị từ chối.
- [ ] Người gọi không phải thành viên project, project của tenant khác, project không tồn tại: cùng `CODEINTEL_NOT_AUTHORIZED`, và **không** có lời gọi `RelayByDevServer` nào (fake đếm gọi).
- [ ] Bảng 2.2: với vai trò `owner`, `member`, admin toàn cục, không vai trò: mỗi `action` cho kết quả đúng (test Rego và test Go dùng bó Rego thật); `member` không `SaveC4Overrides`; admin toàn cục không thuộc tenant của project vẫn bị `GetProject` từ chối.
- [ ] `project-service` hoặc OPA lỗi: từ chối (không cho qua), mã `CODEINTEL_AUTHZ_UNAVAILABLE`; bó Rego thiếu: service không khởi động.
- [ ] Cờ tắt hoặc không có dòng: `CODEINTEL_FEATURE_DISABLED` trước mọi lời gọi tới `project-service`; lỗi đọc cờ cũng đóng.
- [ ] Gỡ thành viên rồi gọi lại sau TTL 10 giây: bị từ chối; trong TTL: đúng hành vi cache (test với đồng hồ giả).
- [ ] Audit: bảng 2.4 mỗi dòng một test; `Outcome` ngoài `allowed`/`denied` không thể dựng; hàng đợi đầy làm tăng bộ đếm bỏ rơi chứ không chặn RPC; `auth-service` giả chậm 5 giây không làm chậm RPC.
- [ ] `AgentCallGate`: 5 lời gọi `heavy` đồng thời tới cùng dev server với giới hạn 2 → 2 chạy, phần còn lại chờ ≤ 2 giây rồi `CODEINTEL_CONCURRENCY_LIMIT`; khe được nhả khi ctx huỷ; cache trúng không gọi `Acquire`.
- [ ] Reindex: lần thứ hai cùng binding → `CODEINTEL_REINDEX_IN_PROGRESS`; sau thành công, trong 5 phút → `CODEINTEL_REINDEX_COOLDOWN`; vượt 1 job/dev server hoặc 2/tenant → bị chặn; người dùng thứ 7 trong giờ → `CODEINTEL_RATE_LIMITED`.
- [ ] Lỗi hạn mức ra `codes.ResourceExhausted` kèm `retry_after_seconds`.
- [ ] Bộ che: bảng mẫu (token GitHub, AWS, Bearer, JWT, khối khoá riêng, `password=...`) bị thay `[REDACTED]`; chuỗi lỗi agent không lọt bí mật vào log và `reindex_jobs.message`.
- [ ] `GetSymbol` của file khớp danh sách chặn (`.env`, `*.pem`, `secrets/x.yaml`, `.AWS/credentials`): `content=""`, `contentWithheld="sensitive_path"`, không lỗi; nội dung > 200 KiB bị từ chối.
- [ ] Test cô lập tenant: người dùng tenant A với id project/worktree của tenant B không đọc được snapshot, review, ghi đè C4 nào (đường đọc cache, đọc DB và đọc trạng thái).
- [ ] `go vet`, `make lint`, `make opa-test` xanh; không `max-lines` disable mới.

## 5. Kiểm thử

- **Rego:** `policy/orca-authz/code_intel_test.rego` (bảng vai trò × `action`, `admin` toàn cục, vai trò rỗng).
- **Unit:** pipeline với `project`, OPA, cổng, audit giả (thứ tự bước và bỏ qua khi lỗi); cache TTL với đồng hồ giả; `AgentCallGate` (đồng thời, chờ, nhả khe, nhiều tenant không ảnh hưởng nhau); `ReindexAdmission`; ánh xạ `LimitError`; bộ che (bảng đầu vào/đầu ra, dương tính giả); danh sách chặn đường dẫn (kể cả `\` Windows, hoa thường).
- **Test an toàn thông tin:** quét AST/`ServiceDesc` để mọi RPC có guard; test "RPC không có tenant"; test chống IDOR (id tenant khác).
- **Integration:** `CountActiveByDevServer` ở hai dialect; `UNIQUE (active_key)` đã có ở CR-CV-011.
- **Hợp đồng gateway (CR-CV-040/072):** mã lỗi và `retry_after_seconds` tới UI.
- **Chưa chạy bất kỳ test nào ở thời điểm viết CR.**

## 6. Rủi ro và điểm chưa kiểm chứng

- Chưa kiểm chứng `ListMembers` ở project hàng trăm thành viên (gọi mỗi lần hết cache); có thể cần RPC "lấy vai trò của tôi" ở `project-service` (ngoài phạm vi series).
- `x-orca-role` chỉ đầy đủ ở đường cookie/session và JWT mới (`common/tenant/tenant.go` chú thích `Role`); lời gọi từ nguồn khác (MCP, CLI) có thể không mang role: quản trị toàn cục không qua được ở đó. Chấp nhận (fail closed).
- Danh tính vẫn là metadata tin cậy theo mạng; token nội bộ chỉ là lớp bảo vệ nông, không phải mTLS.
- Audit có thể mất bản ghi khi `auth-service` lỗi hoặc `outcome` sai (đã chặn bằng hằng); độ dài `target` chưa kiểm chứng; không có `metadata_json`.
- Hạn mức ở bộ nhớ nhân theo số bản sao; không có hạn mức trên chính dev server ngoài đồng thời (không đo tải CPU thật của dev server).
- Cơ chế đếm reindex theo dev server không nguyên tử; có thể vượt 1 job trong cuộc đua hiếm.
- Danh sách đường dẫn và regex che bảo thủ nhưng không đầy đủ (khoá bí mật dạng khác, bí mật nhúng trong mã): phòng thủ chiều sâu, không phải bảo đảm.
- Chưa kiểm chứng ảnh hưởng của `mcp.rego` nếu kênh `codeIntel.*` về sau được mở qua MCP (CR-CV-041): `mcp.rego` từ chối mặc định rủi ro không rõ (`decision := deny` khi `policy_undefined`), nên không mở ngầm.
- Quyền truy cập dev server theo nhóm (`ListDevServersForUser`, `GrantDevServerGroupAccess`) không được kiểm ở đây; `files.*`, `git.*` cũng không kiểm (xem Q2).

## 7. Câu hỏi mở

- **Q1.** Có cần siết `read_source`, `reindex` theo vai trò chức năng trên repo (`repo_members`: `developer`/`lead`/`admin`) không? Mặc định: không ở MVP.
- **Q2.** Người là thành viên project nhưng không có quyền truy cập dev server theo nhóm (CR-DS-006) có được xem code-intel không? Mặc định: có (ngang `files.*`/`git.*`).
- **Q3.** Audit bền và giàu chi tiết: thêm `auditclient.AppendDetailed` (cộng `actor_type`, `target_type`, `target_id`, `metadata_json`, trả lỗi) hoặc outbox `orca.codeintel.audit.appended` kèm consumer ở `auth-service` (theo `mcp-service`)? Cả hai chạm ngoài phạm vi series (`common`, `auth-service`). Mặc định: dùng `Append` ở MVP.
- **Q4.** `reindex` cho `member` hay chỉ `owner`? Mặc định: `member` kèm hạn mức.
- **Q5.** Số liệu hạn mức mặc định cần chốt từ đo đạc (CR-CV-071).
- **Q6.** Cập nhật README v7 mục 3.3: thêm mã Go `CODEINTEL_NOT_AUTHORIZED`, `CODEINTEL_FEATURE_DISABLED`, `CODEINTEL_RATE_LIMITED`, `CODEINTEL_CONCURRENCY_LIMIT`, `CODEINTEL_REINDEX_COOLDOWN`, `CODEINTEL_AUTHZ_UNAVAILABLE`, `CODEINTEL_NO_TENANT`; và mục 6 "Provider/quyền": làm rõ rằng đọc mã code-intel kiểm thành viên project, chặt hơn `files.*`/`git.*`.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` mục 2 (O2, O3, O8), 3.3, 3.8, 6
- `/opt/repos/orca/docs/research/view-code/03-command-and-data-flow.md` mục 5, `06-gaps-risks-roadmap.md` mục 2, `02-local-mcp-interaction.md` mục 4
- `/opt/repos/orca/backend-go/services/project-service/internal/usecase/{authorization.go,get_project.go,list_members.go,list_worktrees.go}`, `backend-go/policy/orca-authz/{project.rego,repo.rego,task_grant.rego,mcp.rego}`
- `/opt/repos/orca/backend-go/services/task-service/internal/usecase/resolve_permission.go`, `cmd/server/main.go` (dòng 293 đến 311, OPA và `auditclient`)
- `/opt/repos/orca/backend-go/services/mcp-service/internal/domain/{tenant_settings.go,audit_event.go,secret_redactor.go}`, `internal/adapter/postgres/tool_call_repository.go`, `internal/adapter/grpc/governance_server.go`, `README.md`
- `/opt/repos/orca/backend-go/common/{auditclient/client.go,internalcaller/internalcaller.go,policy/evaluator.go,grpcmw/grpcmw.go,tenant/tenant.go,apperrors/apperrors.go}`
- `/opt/repos/orca/backend-go/proto/orca/auth/v1/auth.proto` (dòng 58 đến 65, 366 đến 383), `services/auth-service/internal/{usecase/append_audit_entry.go,domain/audit.go,adapter/natsconsumer/mcp_audit_ingest.go}`
- `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/grpc/dial.go` (`AttachIdentity`), `cmd/server/mcp_governance_wiring.go`, `internal/adapter/wscompat/channels_files.go`, `channels_git.go`
- `/opt/repos/orca/docs/crs/v6/request-quality-rollout/CR-REQ-025-e2e-tests-feature-flag-rollout.md` mục 2.2 (mẫu cờ theo tenant)
- CR liên quan: [CR-CV-010](./CR-CV-010-scaffold-code-intel-service.md), [CR-CV-011](./CR-CV-011-code-intel-data-model-and-repositories.md), [CR-CV-012](./CR-CV-012-project-worktree-to-repo-binding.md)
