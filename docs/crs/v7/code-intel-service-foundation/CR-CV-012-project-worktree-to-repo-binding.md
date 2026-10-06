# CR-CV-012 — Ánh xạ project/worktree → dev server → repo và tổng hợp trạng thái index

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-012 |
| **Tên** | Phân giải `(project, worktree)` thành `(dev server, đường dẫn tuyệt đối, repo GitNexus/CodeGraph)`; RPC `BindRepo`, `ListRepoBindings`, `GetIndexStatus`; tổng hợp trạng thái từ `codeintel.status` |
| **Loại** | Feature (nền định tuyến) |
| **Priority** | 🔴 P0 (E16 của nghiên cứu; mọi view cần) |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-010, 011; dùng `codeintel.status` của CR-CV-001, 002 (có thể làm trước với agent giả) |
| **Mở khoá** | CR-CV-013 (hạn mức theo dev server), 021 (collector), 022, 024, 030, 036, 040 |
| **Tác động** | `backend-go/services/code-intel-service/{internal/usecase, internal/adapter/grpcclient, internal/adapter/grpc, internal/adapter/eventbus}`, `backend-go/proto/orca/codeintel/v1/codeintel_binding.proto` (mới). Không sửa `project-service`, `infra-fleet-service` |

---

## 1. Bối cảnh và vấn đề

Quy tắc đã chốt (O4, [README v7](../README.md) mục 2): không nhận tên repo hay đường dẫn tự do từ client; khớp **đường dẫn đã đăng ký**. Khi đọc code thật để thực hiện quy tắc đó, phát hiện:

### 1.1 Project ↔ dev server ↔ worktree đang lưu như thế nào

| Điều | Nơi lưu | Bằng chứng |
|------|---------|------------|
| Dev server của một repo nằm ở **`Repo.dev_server_id`** (Phase 10), rỗng nghĩa là repo cục bộ. `Project.dev_server_id` còn đó nhưng `RebindDevServer` chỉ còn "trong thời gian ngưng dùng"; thay bằng `RebindRepoDevServer` | `project-service` | `backend-go/proto/orca/project/v1/project.proto` dòng 20 đến 27, 332 đến 346; `git-gateway-service/internal/adapter/grpcclient/resolver.go` dòng 170 đến 231 (lấy `DevServerID` từ `GetRepo`, không từ project) |
| Đường dẫn checkout chính là **`Repo.url`**: với repo gắn dev server, `url` chính là đường dẫn tuyệt đối trên dev server | `project-service` | `usecase/create_project.go` dòng 136 (`domain.NewRepo(..., in.RepoPath, ...)`); chú thích `GetRepo` ở `usecase/get_repo.go`; `resolver.go` dòng 215 đến 220 |
| Đường dẫn worktree là **`Worktree.path`**, kèm `repo_id`, `project_id`, `branch`, `status`, `base_ref` | `project-service` (`project.worktrees`) | `project.proto` dòng 523 đến 558 |
| Worktree do Orca tạo có UUID; worktree "external" (`git worktree add` bên ngoài) **không có dòng** nào và được gateway tổng hợp id `<repoId>::<path>`; `IsMainWorktree` là phần tử đầu của kết quả quét đĩa | `api-gateway` | `wscompat/channels_worktree.go` dòng 792 đến 827 |
| Ngoài ra id thư mục làm việc `...::workspace:<uuid>` | frontend | `frontend/src/shared/worktree-id.ts` |
| Dev server: `infra-fleet-service` có `ListDevServers` (toàn tenant, không lọc theo người dùng), **không có `GetDevServer`**; `DevServer` mang `mode`, `approval_status` (`pending_approval`/`approved`/`rejected`), `kind` (`AGENT_KIND_DEV_SERVER`/`AGENT_KIND_MOBILE_EMULATOR`), `health_status`, `platform` | `infra-fleet-service` | `infrafleet.proto` dòng 48, 477 đến 515; chú thích "no GetDevServer" ở `project-service/internal/adapter/grpcclient/infra_fleet_dev_server_lister.go` |
| `RelayByDevServer` kiểm **tenant** (`DevServerRepository.Get(ctx, tenantID, id)`) và **kết nối sống** (`INFRA_DEV_SERVER_NOT_CONNECTED`, `FailedPrecondition`), **không** kiểm người dùng, nhóm truy cập, `approval_status` hay `mode` | `infra-fleet-service` | `usecase/relay_by_dev_server.go` dòng 38 đến 66 |
| Hầu như không có `infra.connections` cho worktree; `git-gateway-service` đi thẳng `RelayByDevServer` | `git-gateway-service` | chú thích `ResolveConnection` ở `resolver.go` dòng 74 đến 100 |

### 1.2 Cạm bẫy quyền khi tra cứu

- `GetWorktree(id)` và `GetRepo(id)` của `project-service` **không lọc theo tenant ở tầng dữ liệu** (`WHERE id = $1`, `worktree_repository.go` dòng 115; `repo_repository.go` dòng 146); `GetWorktree` còn không gọi `RequireTenantID` (`usecase/get_worktree.go`), `GetRepo` chỉ kiểm có tenant trong ctx, không kiểm membership (chú thích `get_repo.go` dòng 19 đến 28 nói rõ: "tenant-scoped only"). Dùng chúng với id do client gửi là lỗ hổng đọc chéo tenant (IDOR).
- `GetProject`, `ListRepos`, `ListWorktrees`, `ListMembers` đều đi qua `requireProjectAccess` (membership + OPA `any_member`) và lọc theo `project_id`/tenant (`usecase/get_project.go`, `list_worktrees.go`, `list_repos.go`).
- `RebindRepoDevServer` không phát sự kiện nào (grep `Publish`/`event` trong `usecase/rebind_repo_dev_server.go` ra rỗng); `orca.project.devserver.changed` có payload kiểu thông báo `{user_ids,title,body,deep_link}` (`adapter/eventbus/publisher.go` dòng 49 đến 69), không mang id repo hay dev server mới. Chỉ có `orca.project.worktree.created/deleted` (`usecase/lifecycle_events.go`) với `worktree_id`, `project_id`. Vì vậy binding **không thể dựa vào sự kiện để biết đổi dev server**, phải kiểm lại khi dùng.

### 1.3 Điều agent làm, và những gì chưa có

- Agent không có "workspace root đăng ký": `RelayContext.registerRoot` là no-op, relay không còn danh sách cho phép đường dẫn (`agent/src/relay/context.ts` dòng 20 đến 33, `relay.ts` dòng 436 đến 455, chú thích trỏ tới `docs/relay-fs-allowlist-removal.md`). Câu "workspaceRoot đã nằm trong workspace root đăng ký" của README v7 mục 3.2 **chỉ đúng nếu backend bảo đảm**: danh sách đăng ký chính là dữ liệu của `project-service` (mục 2.2), và CR-CV-001 phải tự kiểm đường dẫn ở agent (realpath, nằm trong repo git).
- Registry GitNexus lưu **một đường dẫn cho mỗi tên** và chỉ có checkout chính: `gitnexus list` trên máy khảo sát liệt kê 12 repo, đường dẫn `/opt/repos/orca`, `/opt/repos/vnp-*`, không có worktree nào; `git worktree list` có `/opt/repos/orca/.claude/worktrees/dev-process-9beda3` và `/opt/repos/orca-deploy`; worktree đầu có `.codegraph/` nhưng **không có `.gitnexus/`**. Nghĩa là với worktree, "khớp đường dẫn đã đăng ký" mặc định sẽ **không khớp** và chỉ checkout chính có chỉ mục GitNexus (chạy `gitnexus list` và `ls`, chỉ-đọc).
- `LastHandshakeInfo` của Go **không giữ `tools[]`** (`devserveragent/session.go` dòng 47 đến 58: có `Platform`, `Arch`, `NodeVersion`, `AgentVersion`, `SessionID`, `Capabilities`; không có `Tools`), trái với README v7 mục 1 ("Go lưu qua `LastHandshakeInfo`"). Vì vậy việc dev server có `gitnexus`/`codegraph` hay không **phải lấy từ `codeintel.status`**, không từ handshake, cho tới khi CR-CV-023 thêm.
- Khi agent trả lỗi JSON-RPC, `RelayByDevServer` bọc thành `INFRA_AGENT_EXEC_FAILED` kiểu `Internal` và `apperrors.ToGRPCStatus` **không đưa nguyên nhân** vào status (`usecase/relay_by_dev_server.go` dòng 61, `common/apperrors/apperrors.go` dòng 97 đến 130). Mã `error.data.code` kiểu `CODEINTEL_REPO_NOT_REGISTERED` do agent đặt sẽ **mất** trước khi tới service này, cho tới khi CR-CV-023 ánh xạ. Vì thế CR này yêu cầu `codeintel.status` **không trả lỗi** cho các tình huống "chưa cài công cụ", "chưa index", "repo chưa đăng ký" mà trả kết quả thành công với trạng thái (mục 2.5).

## 2. Giải pháp đề xuất

### 2.1 Ba RPC và message (`codeintel_binding.proto`, mới)

Request **không có** trường đường dẫn, tên repo, dev server (kiểm bằng test phản chiếu proto, mục 5). Tenant, user, role đến từ metadata.

```proto
message RepoBinding {
  string id = 1;
  string project_id = 2;
  string repo_id = 3;
  string worktree_id = 4;        // UUID project.worktrees.id, rỗng nếu không có
  string worktree_ref = 5;       // chuỗi người dùng gửi, đã xác thực (chỉ để hiển thị)
  string dev_server_id = 6;
  string workspace_root = 7;     // đường dẫn trên dev server
  string gitnexus_repo = 8;      // tên trong registry GitNexus; rỗng nếu chưa có
  string codegraph_path = 9;
  IndexScope index_scope = 10;   // EXACT | REPO_ROOT | UNRESOLVED
  int64 version = 11;
  google.protobuf.Timestamp created_at = 12;
  google.protobuf.Timestamp updated_at = 13;
}
message BindRepoRequest        { string project_id = 1; string worktree_ref = 2; }
message BindRepoResponse       { RepoBinding binding = 1; IndexStatus status = 2; }
message ListRepoBindingsRequest  { string project_id = 1; int32 limit = 2; }   // mặc định 100, tối đa 200
message ListRepoBindingsResponse { repeated RepoBinding bindings = 1; }
message GetIndexStatusRequest  { string project_id = 1; string worktree_ref = 2; bool refresh = 3; }
message GetIndexStatusResponse { IndexStatus status = 1; }
```

`IndexStatus` (mục 2.5) cũng khai báo ở file này vì CR-CV-012 là nơi tổng hợp; các CR khác (UI, CR-CV-050) chỉ đọc. Mọi CR gọi `codeintel.*` dùng chung hàm phân giải `ResolveTarget` (mục 2.3) thay vì tự đọc `project-service`.

| RPC | Hành vi | Quyền (CR-CV-013) |
|-----|---------|-------------------|
| `BindRepo` | Phân giải, upsert binding (idempotent theo `scope_key`), thử `codeintel.status`, trả binding + trạng thái. Không ghi gì lên dev server | `read` (chỉ ghi dữ liệu nội bộ của service) |
| `ListRepoBindings` | Liệt kê binding đã có của project cùng `last_status` đã lưu (không thăm dò dev server) | `read` |
| `GetIndexStatus` | Phân giải (lười tạo binding), trả trạng thái tổng hợp; `refresh=true` bỏ qua cache TTL | `read` |

### 2.2 Thuật toán phân giải `ResolveTarget(ctx, projectID, worktreeRef)`

Chạy **sau** kiểm quyền của CR-CV-013 (`GetProject` thành công nghĩa là tenant khớp và người gọi là thành viên). Kết quả `Target{ProjectID, RepoID, WorktreeID, WorkspaceRoot, DevServerID, Platform}`.

1. Chuẩn hoá `worktreeRef`: bỏ tiền tố `id:` (frontend, `channels_worktree.go` dòng 738 đến 743) và tiền tố `repo:` (kiểu của git-gateway). Phân loại:
   - **(a)** UUID hợp lệ: worktree do Orca ghi.
   - **(b)** `<repoId>::<path>` (không có hậu tố `::workspace:`): worktree external hoặc checkout chính.
   - **(c)** có hậu tố `::workspace:<uuid>` (thư mục làm việc, không phải repo git): **không hỗ trợ** ở MVP → `CODEINTEL_WORKTREE_REF_UNSUPPORTED` (`InvalidArgument`). Chưa kiểm chứng GitNexus có index thư mục không-git hay không.
   - **(d)** id repo trần (UUID có trong `ListRepos`): checkout chính của repo đó.
   - Dạng khác: `CODEINTEL_INVALID_ARGUMENT`.
2. `repos := ListRepos(project_id)`. Dùng `ListRepos`/`ListWorktrees` (bị khoá theo project và membership), **không** dùng `GetRepo`/`GetWorktree` theo id thô.
3. Theo loại:
   - **(a)** `ListWorktrees(project_id)`, tìm `id == worktreeRef`; không có → `CODEINTEL_WORKTREE_NOT_FOUND` (`NotFound`; đồng thời xoá binding `wt:<id>` nếu có). `path := wt.path`, `repo := repos[wt.repo_id]`.
   - **(b)** `repo := repos[repoId]` (không có → `CODEINTEL_WORKTREE_NOT_FOUND`). `path` do client gửi nên **không tin**: chấp nhận khi (i) bằng `repo.url` (sau chuẩn hoá), hoặc (ii) nằm trong `git-gateway-service.DetectWorktrees(repoId).on_disk_worktrees[].path` (sự thật trên đĩa, cùng nguồn mà `worktree.detectedList` dùng, `gitgateway.proto` dòng 139, 939 đến 953), hoặc (iii) trùng `Worktree.path` trong `ListWorktrees`. Ngoài ba trường hợp: `CODEINTEL_PATH_NOT_ALLOWED` (`PermissionDenied`). Kết quả (ii) cache 60 giây theo `(tenant, repoId)`.
   - **(d)** `path := repo.url`.
4. `devServerID := repo.dev_server_id`. Rỗng → `CODEINTEL_NO_DEV_SERVER` (`FailedPrecondition`): repo cục bộ nằm trên máy người dùng, backend không với tới (D2). Không rơi về `Project.dev_server_id` (git-gateway cũng không đọc nó).
5. `ds := ListDevServers()` (không `kind` để một lần gọi; lọc ở bộ nhớ), cache 30 giây theo tenant. Kiểm lần lượt: có `ds.id` (không → `CODEINTEL_NO_DEV_SERVER`); `kind == AGENT_KIND_DEV_SERVER`; `approval_status == "approved"` (`CODEINTEL_DEV_SERVER_NOT_APPROVED`); `mode == CONNECTION_MODE_DIRECT_WEBSOCKET` (`CODEINTEL_DEV_SERVER_MODE_UNSUPPORTED`; D2, `relay-ssh` là CR-CV-006). Đây là phần **duy nhất** kiểm các điều kiện này vì `RelayByDevServer` không kiểm (mục 1.1).
6. Chuẩn hoá đường dẫn (mục 2.4). Sinh `scope_key` (CR-CV-011: `wt:<uuid>` cho (a), `path:<repo_id>:<sha256>` cho (b), (d)) và `path_hash`.
7. Upsert binding (`RepoBindingRepository.Upsert`). Nếu bản đã lưu khác `workspace_root` hoặc `dev_server_id` (worktree dời chỗ, `RebindRepoDevServer`): cập nhật, `index_scope=UNRESOLVED`, `gitnexus_repo=''`, `codegraph_path=''`, và `SnapshotRepository.DeleteByBinding`.

Chi phí: 3 đến 4 gọi gRPC nội bộ (`ListRepos`, `ListWorktrees`, `ListDevServers`, có thể `DetectWorktrees`); kết quả phân giải cache 30 giây theo `(tenant, project, worktreeRef)` **chỉ sau khi** kiểm quyền (quyền kiểm mỗi lần theo CR-CV-013, TTL ngắn riêng). Hết hạn hoặc `refresh=true` thì làm lại.

### 2.3 Dịch vụ phía ra (`internal/adapter/grpcclient`, mới)

| Client | RPC dùng | Ghi chú |
|--------|----------|---------|
| `project_client.go` | `ListRepos`, `ListWorktrees`, `GetProject`, `ListMembers` (CR-CV-013) | chuyển tiếp **tenant, user và role** từ ctx (khoá `grpcmw.MetadataTenantID/UserID/Role`, `common/grpcmw/grpcmw.go` dòng 24 đến 40); viết `identity_forwarding.go`. `withTenantMetadata` của `git-gateway-service`/`task-service` chỉ chuyển tenant, `withIdentityMetadata` thêm user nhưng **không có role**; thiếu user thì `requireProjectAccess` trả `PROJECT_NO_USER`, thiếu role thì quản trị toàn cục bị coi như không có quyền (`project-service/internal/usecase/authorization.go` dòng 73 đến 84) |
| `infra_fleet_client.go` | `ListDevServers`, `IsDevServerConnected`, `RelayByDevServer` | cùng cách chuyển danh tính; CR-CV-021 dùng lại connection |
| `git_gateway_client.go` | `DetectWorktrees` | chỉ cho worktree dạng (b); chỉ tenant (git-gateway kiểm tenant, không kiểm user) |

`grpc.NewClient` với `insecure` và `otelgrpc.NewClientHandler()` (mẫu `task-service/cmd/server/main.go` dòng 305); dial lười, lỗi kết nối không làm service chết.

### 2.4 Chuẩn hoá và so sánh đường dẫn (đa nền tảng, AGENTS.md)

`internal/domain/workspace_path.go` (mới), hàm `NormalizeWorkspacePath(raw, platform)`, `platform` lấy từ `DevServer.platform` (do handshake cung cấp, `infrafleet.proto` dòng 510):

- Phải tuyệt đối: POSIX bắt đầu `/`; Windows `X:\` hoặc `X:/` hoặc UNC `\\host\share`. Không phải thì `CODEINTEL_INVALID_ARGUMENT`.
- Từ chối: ký tự NUL và điều khiển, đoạn `..`, độ dài > 4096.
- Gộp dấu phân cách lặp, bỏ dấu phân cách cuối (trừ gốc). Không dùng `path/filepath` của máy chạy `code-intel-service` (đường dẫn thuộc dev server, có thể khác hệ điều hành); viết bằng chuỗi theo `platform`.
- So sánh/băm: POSIX phân biệt hoa thường; Windows so sánh sau khi hạ chữ thường và đổi `/` thành `\`. Giá trị lưu `workspace_root` giữ nguyên chữ hoa/thường gốc. Chưa kiểm chứng quy tắc cho WSL (đường dẫn `/mnt/c/...` hay `\\wsl$\...`) và macOS (không phân biệt hoa thường theo mặc định); xem Q3.
- **Không** giải quyết symlink ở backend; `realpath` thuộc agent (CR-CV-001).

### 2.5 Trạng thái index tổng hợp

Backend gọi `codeintel.status` qua `RelayByDevServer{dev_server_id, method:"codeintel.status", params_json:{"workspaceRoot": <workspace_root>}}`, hạn chót `CODEINTEL_STATUS_TIMEOUT` (10 giây). Yêu cầu với agent (CR-CV-001, 002; ghi vào README v7 mục 3.2): `codeintel.status` **luôn thành công khi agent chạy** và `data` chứa một phần tử cho từng công cụ:

```jsonc
{ "tools": [ {
    "tool": "gitnexus|codegraph", "available": true, "version": "1.6.9",
    "registered": true, "repoName": "orca", "registeredPath": "/opt/repos/orca",
    "indexScope": "exact|repo_root|none",
    "state": "missing|building|ready|stale",
    "indexedCommit": "…", "headCommit": "…", "indexedAt": "…",
    "stats": {"files":0,"nodes":0,"edges":0,"communities":0,"processes":0},
    "pendingChanges": {"added":0,"modified":0,"removed":0}, "languages": [] } ] }
```

Trường theo `IndexStatus` ở nghiên cứu [05 §2.8](../../../research/view-code/05-graph-schemas.md). `indexScope` do agent quyết (đường dẫn trùng đúng registry: `exact`; worktree thuộc checkout chính đã đăng ký suy ra từ `git rev-parse --git-common-dir`: `repo_root`; còn lại: `none`). Chưa kiểm chứng agent làm được việc này; nếu không thì worktree không có chỉ mục riêng sẽ luôn `none` (mục 6). Backend chỉ **ghi lại** `indexScope`, `repoName`, `registeredPath` vào binding (`gitnexus_repo`, `codegraph_path`, `index_scope`), không bao giờ tự chọn tên repo (O4).

Gộp thành `IndexStatus.overall` theo thứ tự ưu tiên (đầu tiên khớp thì dừng):

| # | Điều kiện | `overall` | Ghi chú |
|---|-----------|-----------|---------|
| 1 | `codeintel_enabled` tắt | không tới đây (CR-CV-013 chặn trước) | |
| 2 | Dev server không kết nối (`INFRA_DEV_SERVER_NOT_CONNECTED`, hoặc `IsDevServerConnected=false`) | `OFFLINE` | trả `last_status` đã lưu kèm `last_status_at`; không phải lỗi của RPC này |
| 3 | Gọi `codeintel.status` lỗi/hết hạn (bao gồm `INFRA_AGENT_EXEC_FAILED`, `agent chưa có method` kiểu method not found) | `UNKNOWN` | kèm `error_code` thô; chưa phân loại được cho tới khi CR-CV-023 chuyển mã |
| 4 | Không công cụ nào `available` | `NOT_INSTALLED` | |
| 5 | Có `reindex_jobs` `queued`/`running` của binding | `BUILDING` | kèm `activeJob{id,stage,percent}` |
| 6 | Mọi công cụ `available` đều `state=missing` | `MISSING` | |
| 7 | Có công cụ `ready` và có công cụ `available` còn lại ở `missing`/`stale` | `DEGRADED` | liệt kê từng công cụ |
| 8 | Có công cụ `stale` (hoặc `indexedCommit != headCommit`, hoặc `indexScope=repo_root`) và không công cụ nào `ready` | `STALE` | `indexScope=repo_root` luôn đặt cờ `scopeMismatch=true` |
| 9 | còn lại | `READY` | |

`indexScope=repo_root` nghĩa chỉ mục thuộc checkout chính, **không phản ánh sửa đổi riêng của worktree**; UI phải hiển thị cảnh báo và các view dựa trên đồ thị (CR-CV-036) phải ghi chú độ chính xác.

Cache: `IndexStatus` giữ 15 giây trong bộ nhớ (`CODEINTEL_STATUS_TTL`), `singleflight` theo `binding.id` để nhiều người mở cùng lúc chỉ gọi dev server một lần (CR-CV-022 dùng cùng cơ chế cho snapshot). Mỗi lần thăm dò thành công: `SaveStatusCache(last_status, last_status_at)`. Ghi chú: `codeintel.indexChanged` (CR-CV-024) tra binding bằng `(tenant_id, dev_server_id, path_hash)` (mục 2.6) và gọi huỷ cache + `SaveStatusCache`.

### 2.6 Vòng đời binding và sự kiện

| Sự kiện | Xử lý |
|---------|-------|
| Lần đầu dùng (`BindRepo`, `GetIndexStatus` hoặc bất kỳ RPC `codeintel.*` nào qua `ResolveTarget`) | Tạo binding lười |
| Mỗi lần dùng | Phân giải lại; khác `workspace_root`/`dev_server_id` thì cập nhật và huỷ snapshot (mục 2.2 bước 7). Cách này thay cho sự kiện đổi dev server vốn không tồn tại |
| `orca.project.worktree.deleted` | Consumer bền (`Subscribe(ctx, "PROJECT", "code-intel-service-worktree-deleted", "orca.project.worktree.deleted", fn)`; stream `PROJECT` do `project-service` tạo, `cmd/server/main.go` dòng 177; bền để một lần trên toàn cụm, như `auth-service` làm với consumer audit, `natsconsumer/audit_ingest.go` dòng 24 đến 38) → dedup bằng `processed_events` `(tenant_id, event_id)` → `DeleteByWorktreeID(project_id, worktree_id)` |
| `orca.project.worktree.created` | Bỏ qua (binding tạo lười) |
| Binding không dùng 90 ngày | Công việc bảo trì xoá (`COALESCE(last_status_at, updated_at)` quá `CODEINTEL_BINDING_IDLE_RETENTION=2160h`); xử lý binding dạng (b), (d) và project bị xoá không có sự kiện |

Payload của `orca.project.worktree.deleted` theo `lifecycle_events.go` dòng 3 đến 17 (`worktree_id`, `project_id`, ...). Tên field khớp `worktreeLifecycleEventPayload`; consumer không phụ thuộc field `had_open_pr`.

### 2.7 Lỗi phía Go (bổ sung cho README v7 mục 3.3)

| Mã | Kind | Khi |
|----|------|-----|
| `CODEINTEL_WORKTREE_NOT_FOUND` | NotFound | worktree/repo không thuộc project |
| `CODEINTEL_WORKTREE_REF_UNSUPPORTED` | InvalidArgument | thư mục làm việc `::workspace:` |
| `CODEINTEL_PATH_NOT_ALLOWED` | PermissionDenied | đường dẫn của id tổng hợp không khớp mục 2.2 bước 3(b) (dùng lại mã của README) |
| `CODEINTEL_NO_DEV_SERVER` | FailedPrecondition | repo cục bộ, hoặc dev server không còn |
| `CODEINTEL_DEV_SERVER_NOT_APPROVED` | FailedPrecondition | `approval_status` ≠ `approved` |
| `CODEINTEL_DEV_SERVER_MODE_UNSUPPORTED` | FailedPrecondition | `mode` ≠ direct-websocket |
| `CODEINTEL_DEV_SERVER_OFFLINE` | FailedPrecondition | RPC khác `GetIndexStatus` khi agent không kết nối |

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|------------|-------|
| Phân giải bằng `ListRepos`/`ListWorktrees` theo project, không `GetRepo`/`GetWorktree` theo id | Hai RPC sau không lọc tenant; dùng chúng là mở đường IDOR (mục 1.2) |
| Dev server lấy từ `Repo.dev_server_id` | Phase 10; nơi `git-gateway-service` đọc |
| Binding dạng cache có kiểm lại mỗi lần dùng | Không có sự kiện đổi dev server; kiểm lại rẻ hơn đồng bộ sai |
| Đường dẫn id tổng hợp phải khớp ba nguồn tin cậy | Id do client sinh; không để nó chọn đường dẫn trên dev server |
| `codeintel.status` không lỗi khi thiếu công cụ/chỉ mục | Mã lỗi agent bị `RelayByDevServer` làm mất (mục 1.3) |
| `indexScope` do agent quyết, backend chỉ ghi | Registry GitNexus nằm trên dev server; backend không thấy (O4) |
| `OFFLINE` không phải lỗi của `GetIndexStatus` | UI cần hiện chip xám và dữ liệu lần cuối |
| Chỉ dev server `approved`, `direct-websocket`, `AGENT_KIND_DEV_SERVER` | `RelayByDevServer` không kiểm; D2 |

## 4. Tiêu chí chấp nhận

- [ ] `BindRepo`, `ListRepoBindings`, `GetIndexStatus` có message ở `codeintel_binding.proto`; `buf lint` xanh; không có trường đường dẫn, tên repo hay dev server trong request (test phản chiếu).
- [ ] Phân giải đúng cho bốn dạng id: UUID worktree, `<repoId>::<path>` (path = `Repo.url`; path có trong `DetectWorktrees`; path trùng `Worktree.path`), repo trần; dạng `::workspace:` trả `CODEINTEL_WORKTREE_REF_UNSUPPORTED`.
- [ ] Id tổng hợp với path ngoài ba nguồn (ví dụ `/etc`, `<repo>/../x`, `\\host\share` trên POSIX) trả `CODEINTEL_PATH_NOT_ALLOWED`, **không** có lần gọi nào tới `RelayByDevServer`.
- [ ] Project của tenant khác, worktree của project khác, repo của project khác: `CODEINTEL_NOT_AUTHORIZED` hoặc `CODEINTEL_WORKTREE_NOT_FOUND`, không lộ khác biệt tồn tại/không tồn tại giữa tenant.
- [ ] Repo cục bộ (`dev_server_id` rỗng), dev server `pending_approval`, `kind=MOBILE_EMULATOR`, mode ≠ direct-websocket: mỗi trường hợp trả đúng mã ở mục 2.7 và không gọi agent.
- [ ] `RebindRepoDevServer` (dev server đổi) rồi gọi lại: binding cập nhật `dev_server_id`, `index_scope=UNRESOLVED`, snapshot của binding bị xoá.
- [ ] `GetIndexStatus` trả đúng 9 trạng thái ở bảng 2.5 với agent giả (một trường hợp một test), thứ tự ưu tiên đúng khi nhiều điều kiện cùng đúng (ví dụ offline + job đang chạy → `OFFLINE`).
- [ ] Dev server offline: `GetIndexStatus` trả `OFFLINE` kèm `last_status` đã lưu, không lỗi; các RPC khác trả `CODEINTEL_DEV_SERVER_OFFLINE`.
- [ ] 20 lời gọi `GetIndexStatus` đồng thời cùng binding: đúng một lần `RelayByDevServer` (singleflight); `refresh=true` bỏ qua cache.
- [ ] Nhận `orca.project.worktree.deleted` hai lần cùng `event_id`: binding bị xoá một lần, lần hai không lỗi (dedup `processed_events`).
- [ ] Chuẩn hoá đường dẫn: bảng test POSIX (`/a//b/`, `/a/./b`, `/a/../b`), Windows (`C:\Repo\`, `c:/repo`, UNC), đường dẫn tương đối, NUL, dài > 4096.
- [ ] Mọi lời gọi tới `project-service`/`infra-fleet-service` mang `x-orca-tenant-id`, `x-orca-user-id`, `x-orca-role` (test với fake server kiểm metadata).
- [ ] Tên file không có `helpers`, `utils`, `common`, `misc`; không có `max-lines` disable mới.

## 5. Kiểm thử

- **Unit:** `NormalizeWorkspacePath` (bảng); phân loại `worktreeRef`; thuật toán `ResolveTarget` với fake `project`, `infra-fleet`, `git-gateway` (mỗi nhánh lỗi một test, kể cả `GetWorktree`/`GetRepo` không bao giờ được gọi); bảng ưu tiên `overall`; test phản chiếu proto "không có trường đường dẫn".
- **Integration (hai dialect):** `Upsert` giữ `id`; xoá theo worktree; truy vấn `(tenant_id, dev_server_id, path_hash)`.
- **Hợp đồng agent:** với agent giả trả `codeintel.status` theo mẫu mục 2.5 (fixture của CR-CV-070); kiểm chống lệch phiên bản công cụ (`version`) ở CR-CV-070.
- **Consumer:** NATS testcontainer (`common/testutil/nats.go`), phát `orca.project.worktree.deleted` hai lần.
- **Chưa chạy bất kỳ test nào ở thời điểm viết CR.**

## 6. Rủi ro và điểm chưa kiểm chứng

- Chưa kiểm chứng agent phân biệt được `exact` và `repo_root`; chưa kiểm chứng `gitnexus analyze` trong một worktree có tạo `.gitnexus/` riêng và đăng ký vào registry (registry khoá theo tên; hai thư mục cùng tên cơ sở có thể va chạm). Nếu worktree không có chỉ mục riêng, trải nghiệm review của phần lớn worktree sẽ là `STALE` + `scopeMismatch` — rủi ro sản phẩm lớn của series (mục 7 Q1).
- Chưa kiểm chứng `IsMainWorktree` (phần tử đầu của quét đĩa) luôn trùng `Repo.url`; nếu không trùng, nhánh (b)(i) không nhận ra checkout chính, nhưng nhánh (b)(ii) vẫn nhận.
- `ListDevServers` trả toàn tenant (có thể hàng trăm dev server); chấp nhận ở MVP với cache 30 giây, chưa đo. Nếu lớn, thêm `GetDevServer` ở `infra-fleet-service` (ngoài phạm vi series).
- Chưa kiểm chứng `project-service` có chế độ chia sẻ (`LinkSourceProject`, `GetSharedProjectData`) cho phép người xem project chứa thấy worktree của project nguồn; `ListWorktrees(container)` sẽ không trả chúng, nên review trên project chứa không hoạt động (ngoài phạm vi MVP).
- Chưa kiểm chứng repo "hidden target" (VM tạm, `hidden_target_id`; `project-service` chưa điền, chú thích `project.proto` dòng 421 đến 433) có cùng đường `RelayByDevServer`.
- Quyền thành viên được kiểm mỗi lần nhưng kết quả phân giải cache 30 giây: người bị gỡ khỏi project mất quyền ngay (CR-CV-013), nhưng worktree đổi chỗ có thể chậm tối đa 30 giây.
- `INFRA_DEV_SERVER_NOT_CONNECTED` trả ngay (không chờ kết nối lại, `relay_by_dev_server.go` dòng 55); README v7 mục 7 nói backend xếp hàng ~20 giây (`RECONNECT_WAIT_MS`); chưa kiểm chứng ở tầng `Exec`.
- Project có hàng trăm worktree làm `ListWorktrees` trả danh sách lớn ở mỗi lần phân giải; chưa đo.

## 7. Câu hỏi mở

- **Q1.** Worktree thường không có chỉ mục GitNexus riêng. Chấp nhận `repo_root` (cảnh báo độ chính xác), hay yêu cầu `codeintel.reindex` cho từng worktree (CR-CV-004: có đăng ký worktree vào registry không)? Mặc định: chấp nhận `repo_root` và cảnh báo.
- **Q2.** Thêm vào README v7: (a) mục 1 dòng "Handshake mang danh sách tool": Go không lưu `tools[]`; (b) mục 3.2: `codeintel.status` không lỗi khi thiếu chỉ mục/công cụ, trả `data.tools[]` như 2.5; (c) mục 3.3: các mã Go ở 2.7 và rằng mã lỗi agent mất ở `RelayByDevServer`; (d) mục 3.2: câu "workspace root đăng ký" chuyển nghĩa thành "đường dẫn có trong `project-service`". Cần cập nhật README.
- **Q3.** Quy tắc so sánh đường dẫn cho WSL và macOS.
- **Q4.** Có cần cho phép người dùng chỉ định tay worktree không thuộc dữ liệu `project-service` (thư mục tuỳ ý trong dev server)? Mặc định: không (O4).
- **Q5.** Thời gian lưu binding rảnh 90 ngày và TTL cache (15 giây trạng thái, 30 giây phân giải) là đề xuất, chưa có số đo.

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` mục 2 (O4), 3.2, 3.3, 3.5
- `/opt/repos/orca/docs/research/view-code/09-external-inputs-required.md` (E16), `06-gaps-risks-roadmap.md` mục 4, `05-graph-schemas.md` mục 2.8
- `/opt/repos/orca/backend-go/proto/orca/project/v1/project.proto` (dòng 20 đến 27, 173 đến 215, 332 đến 346, 523 đến 560, 614 đến 622, 880 đến 900)
- `/opt/repos/orca/backend-go/services/project-service/internal/usecase/{authorization.go,get_project.go,get_repo.go,get_worktree.go,list_worktrees.go,list_repos.go,create_project.go,lifecycle_events.go,rebind_repo_dev_server.go}`, `internal/adapter/postgres/{repo_repository.go,worktree_repository.go}`, `internal/adapter/eventbus/publisher.go`, `internal/adapter/grpcclient/infra_fleet_dev_server_lister.go`
- `/opt/repos/orca/backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (dòng 48, 102, 133, 363 đến 384, 477 đến 515, 596 đến 609, 866 đến 906)
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/usecase/{relay_by_dev_server.go,list_dev_servers.go,ports.go}`, `internal/adapter/devserveragent/{session.go,client.go}`, `internal/adapter/grpc/server.go` (dòng 660 đến 682)
- `/opt/repos/orca/backend-go/services/git-gateway-service/internal/adapter/grpcclient/{resolver.go,project_client.go,tenant_forwarding.go}`, `proto/orca/gitgateway/v1/gitgateway.proto` (`DetectWorktrees`)
- `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go`, `internal/adapter/grpc/dial.go` (`AttachIdentity`)
- `/opt/repos/orca/frontend/src/shared/worktree-id.ts`
- `/opt/repos/orca/agent/src/relay/context.ts`, `relay.ts` (`session.registerRoot` là no-op)
- `/opt/repos/orca/backend-go/common/{grpcmw/grpcmw.go,apperrors/apperrors.go,eventbus/eventbus.go}`, `backend-go/services/auth-service/internal/adapter/natsconsumer/audit_ingest.go` (consumer bền)
- Kết quả chỉ-đọc: `gitnexus list` (12 repo, không có worktree), `git worktree list`, `ls .claude/worktrees/*/.gitnexus` (không có), `.codegraph` (có)
- CR liên quan: [CR-CV-010](./CR-CV-010-scaffold-code-intel-service.md), [CR-CV-011](./CR-CV-011-code-intel-data-model-and-repositories.md), [CR-CV-013](./CR-CV-013-authorization-audit-and-quotas.md); agent: [`agent-codeintel`](../agent-codeintel/README.md); collector: [`code-intel-graph-pipeline`](../code-intel-graph-pipeline/README.md)
