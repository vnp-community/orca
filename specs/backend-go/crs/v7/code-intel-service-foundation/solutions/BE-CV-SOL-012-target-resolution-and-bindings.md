# BE-CV-SOL-012-target-resolution-and-bindings: Phân giải `(project, worktree)` → dev server → đường dẫn, binding, `BindRepo`/`ListRepoBindings`

> **📋 Proposed.** Chưa chạy build/test nào. Phần thứ nhất của CR-CV-012; phần tổng hợp trạng thái index ở [`BE-CV-SOL-012-index-status-aggregation`](./BE-CV-SOL-012-index-status-aggregation.md).

**CR:** [CR-CV-012](../../../../../../docs/crs/v7/code-intel-service-foundation/CR-CV-012-project-worktree-to-repo-binding.md)
**Service:** `code-intel-service` (`internal/domain`, `internal/usecase`, `internal/adapter/{grpcclient,eventbus,grpc}`), `proto/orca/codeintel/v1/codeintel_binding.proto`
**TDD tham chiếu:** [`arch/02`](../../../../tdd/architecture/02-microservices-decomposition.md), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) (mục "AuthZ", "Multi-tenancy isolation"), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) (mục "gRPC conventions", "Event conventions", "Talking to the Dev Server Agent"), [`services/project-service`](../../../../tdd/services/project-service.md), [`services/infra-fleet-service`](../../../../tdd/services/infra-fleet-service.md)

---

## Hợp đồng áp dụng

| Mục | Áp dụng |
|---|---|
| PQ-04 | Request nhận `WorktreeSelector{project_id, worktree_ref}`; `worktree_ref` chuẩn hoá (bỏ `id:`/`repo:`; UUID, `<repoId>::<path>`, repo trần); dạng `::workspace:` → `CODEINTEL_WORKTREE_REF_UNSUPPORTED`; cache phân giải 30 s **sau** kiểm quyền |
| §2.1 #4, #2 | `codeintel_binding.proto` (RepoBinding, BindRepo*, ListRepoBindings*, GetIndexStatus*); `WorktreeSelector` thuộc `codeintel_common.proto` (CR-020) |
| §3.1 | `BindRepo` (`read`), `ListRepoBindings` (không `selector`: `project_id`, `limit ≤ 200`; không kênh), `GetIndexStatus` (SOL-012-index-status) |
| §3.3 | Client `ProjectService.{ListRepos,ListWorktrees,GetProject,ListMembers}`, `InfraFleetService.{ListDevServers,IsDevServerConnected}`, `GitGatewayService.DetectWorktrees`; **không** dùng `GetRepo`/`GetWorktree` theo id |
| §4.2 T2 | `repo_bindings` (`scope_key`, `path_hash`, `index_scope`) |
| §5 | Consumer bền `orca.project.worktree.deleted` (stream `PROJECT`, durable `code-intel-service-worktree-deleted`), dedup `processed_events` |
| PQ-03 | Mã: `CODEINTEL_WORKTREE_NOT_FOUND`, `_WORKTREE_REF_UNSUPPORTED`, `_PATH_NOT_ALLOWED`, `_NO_DEV_SERVER`, `_DEV_SERVER_NOT_APPROVED`, `_DEV_SERVER_MODE_UNSUPPORTED`, `CODEINTEL_DEV_SERVER_OFFLINE` = `Unavailable`, `CODEINTEL_INVALID_PARAMS`; quyền → `CODEINTEL_NOT_AUTHORIZED` |
| PQ-14 (6) | Client tới infra-fleet đặt `MaxCallRecvMsgSize(16 MiB)` (dial dùng chung với SOL-021) |
| H9 / AGENTS.md | SSH: chỉ dev server `direct-websocket` ở MVP (O-5); đường dẫn chuẩn hoá theo `platform` của dev server, không theo OS của service; O-14: chỉ POSIX ở MVP |

## Lệch giữa CR và hợp đồng

| # | CR-CV-012 nói | Hợp đồng | Xử lý |
|---|---|---|---|
| L1 | `BindRepoRequest{project_id, worktree_ref}` | PQ-04: `WorktreeSelector selector = 1` | Dùng `selector`; số field khác do chủ sở hữu CR gán (CR-012) |
| L2 | `CODEINTEL_INVALID_ARGUMENT` | PQ-03 (2) | `CODEINTEL_INVALID_PARAMS` |
| L3 | `CODEINTEL_DEV_SERVER_OFFLINE` = `FailedPrecondition` | PQ-03 (3): `Unavailable` | `KindUnavailable` (SOL-010 task 02) |
| L4 | `IndexStatus` có `repeated ToolIndexStatus`, `index_basis` | §2.3: có `index_basis = 5` (kiểu `IndexBasis` ở file của CR-080), `binding = 8` | Xem mục 7 (điểm hợp đồng mâu thuẫn): `index_basis` thêm sau |
| L5 | `IndexScope` enum `EXACT\|REPO_ROOT\|UNRESOLVED` | PQ-08 (5): `INDEX_SCOPE_UNSPECIFIED = 0`; trên dây `exact\|repo_root\|stale\|none` | Enum DB-vị-trí và enum dây khác nhau (SOL-012-index-status) |
| L6 | "chưa kiểm chứng agent phân biệt `exact`/`repo_root`" | Agent contract §4.1 quy định `classifyIndexBasis` | Backend chỉ ghi lại giá trị agent trả |
| L7 | `ListRepoBindings` không nêu quyền | §3.1: `read` | `read` |

## Phụ thuộc chéo khu vực

| Khu vực | Solution | Quan hệ |
|---|---|---|
| BE | `BE-CV-SOL-011-*` | Trước: `RepoBindingRepository`, `ProcessedEventRepository`, `SnapshotRepository.DeleteByBinding` |
| BE | `BE-CV-SOL-020-canonical-graph-model` | Trước (G0): `codeintel_common.proto` (`WorktreeSelector`, `ToolIndexStatus`, `IndexScope`) |
| BE | `BE-CV-SOL-013-*` | Sau: `ResolveTarget` chạy **sau** kiểm quyền SOL-013; SOL-013 dùng `GetProject`/`ListMembers` client của solution này |
| BE | `BE-CV-SOL-012-index-status-aggregation` | Sau |
| BE | `BE-CV-SOL-021/022/024/030/036/040` | Dùng `ResolveTarget` |
| BE | `BE-CV-SOL-023-infra-fleet-codeintel-transport` | Song song: ánh xạ lỗi agent; trước đó `INFRA_AGENT_EXEC_FAILED` làm mất mã |
| AG | `AG-CV-SOL-001-codeintel-agent-foundation` | `codeintel.status` luôn thành công (SOL-012-index-status) |
| FE | — | Frontend phải biết `projectId` (O-1) — việc của `FE-CV-SOL-050-*` |

---

## 1. Trạng thái hiện tại (Re-verify)

Đã đọc: `proto/orca/project/v1/project.proto` (`GetProject` dòng 14, `ListMembers` 17, `ListRepos` 38, `ListWorktrees` 79; `Repo.dev_server_id = 6` dòng 342; `Worktree{id,project_id,repo_id,path,…}` dòng 523; `ListReposRequest{project_id}`, `ListWorktreesRequest{project_id,status_in,older_than}` — **không phân trang**), `project-service/internal/usecase/{authorization.go,get_project.go,lifecycle_events.go}`, `proto/orca/infrafleet/v1/infrafleet.proto` (`ListDevServers` dòng 48, `RelayByDevServer` 102, `IsDevServerConnected` 133; `DevServer.approval_status = 6`; `ListDevServersRequest{kind}`), `infra-fleet-service/internal/usecase/relay_by_dev_server.go` (kiểm tenant + kết nối, **không** kiểm approval/mode/user; mọi lỗi `Exec` → `INFRA_AGENT_EXEC_FAILED`), `devserveragent/client.go` (`execTimeoutForMethod` chỉ `agent.execPrompt`), `proto/orca/gitgateway/v1/gitgateway.proto` (`DetectWorktrees{repo_id}` dòng 939, `on_disk_worktrees[].path`), `api-gateway/internal/adapter/wscompat/channels_worktree.go` (dòng 738–745 `stripWorktreeSelectorPrefix` chỉ bỏ `id:`; dòng 800–830 id tổng hợp `repoID::path`, `IsMainWorktree = i == 0`), `frontend/src/shared/worktree-id.ts` (`::workspace:`), `common/grpcmw/grpcmw.go` (4 khoá metadata), `common/eventbus/eventbus.go` (`Subscribe(ctx, streamName, consumerName, subject, fn)`).

### Correction relative to CR-CV-012

| # | CR nói | Mã thật | Xử lý |
|---|---|---|---|
| C1 | Tiền tố `repo:` do git-gateway dùng | Chỉ **xác nhận** `id:` ở gateway; tiền tố `repo:` chưa tìm thấy trong file đã đọc | Vẫn chấp nhận và bỏ `repo:` (hợp đồng PQ-04); ghi "chưa kiểm chứng" |
| C2 | `GetProject` kiểm membership rồi lọc tenant | `GetProject` gọi `requireProjectAccess` **trước** `repo.Get(ctx, tenantID, id)`; admin toàn cục qua membership nhưng bị `repo.Get` lọc tenant | Đúng; dùng `GetProject` làm cổng |
| C3 | `ListWorktrees`/`ListRepos` bị khoá project+membership | `ListWorktrees` request chỉ `project_id` + bộ lọc; không có phân trang | Rủi ro danh sách lớn (mục 6); cache 30 s |
| C4 | `x-go-common-env` cần địa chỉ | Có sẵn (SOL-010 C3) | Không thêm |
| C5 | `withIdentityMetadata` của `git-gateway`/`task-service` thiếu role | Chưa đọc lại hai file đó ở lần soạn này | Tự viết `identity_forwarding.go` chuyển đủ bốn khoá `grpcmw`; ghi "chưa kiểm chứng" cho nhận định về hai service kia |
| C6 | `LastHandshakeInfo` không giữ `tools[]` | Chưa đọc lại `session.go`; hợp đồng PQ-18 xác nhận "hiện không có" | Dựa hợp đồng |

Chưa kiểm chứng: hành vi `project-service` cho dự án chia sẻ (`LinkSourceProject`); repo "hidden target"; `IsMainWorktree` luôn trùng `Repo.url`; `DetectWorktrees` có kiểm user hay chỉ tenant.

## 2. Giải pháp

### A. Proto `codeintel_binding.proto` (mới) và RPC

```proto
// số field do chủ sở hữu CR-012 gán; số đã ghi ở hợp đồng §2.3 (IndexStatus) là chuẩn
message RepoBinding { string id=1; string project_id=2; string repo_id=3; string worktree_id=4; string worktree_ref=5;
  string dev_server_id=6; string workspace_root=7; string gitnexus_repo=8; string codegraph_path=9;
  IndexScope index_scope=10; int64 version=11; google.protobuf.Timestamp created_at=12, updated_at=13; }
message BindRepoRequest { WorktreeSelector selector = 1; }
message BindRepoResponse { RepoBinding binding = 1; IndexStatus status = 2; }
message ListRepoBindingsRequest { string project_id = 1; int32 limit = 2; }   // mặc định 100, tối đa 200
message ListRepoBindingsResponse { repeated RepoBinding bindings = 1; }
```

Request **không** có trường đường dẫn, tên repo, dev server (test phản chiếu). `GetIndexStatus*` và `IndexStatus` ở solution kia nhưng cùng file proto (task 012-02 tạo file, 012-08 điền).

`workspace_root`, `dev_server_id` là trường **chỉ cho backend**: gateway không chuyển ra UI (CONTRACT-ui-api §2.2: không đường dẫn tuyệt đối ra UI); gateway view struct bỏ chúng (việc SOL-040).

### B. `ResolveTarget(ctx, sel) (Target, error)` (`internal/usecase/resolve_target.go`, mới)

Chạy **sau** kiểm quyền SOL-013 (`GetProject` đã thành công). Thuật toán nguyên văn CR mục 2.2 bước 1–7 với hiệu chỉnh:

1. `ParseWorktreeRef(raw)` (domain): bỏ `id:`/`repo:`; phân loại `(a) UUID worktree`, `(b) <repoId>::<path>`, `(c) …::workspace:…` → `CODEINTEL_WORKTREE_REF_UNSUPPORTED`, `(d) repo trần`, khác → `CODEINTEL_INVALID_PARAMS`. Giới hạn `worktree_ref ≤ 512` (UI-API §2.1).
2. `ListRepos(project)`, `ListWorktrees(project)` (cache 30 s theo `(tenant, project)` sau kiểm quyền).
3. (a) tìm `wt.id`; không có → `CODEINTEL_WORKTREE_NOT_FOUND` + xoá binding `wt:<id>`; (b) path chấp nhận khi bằng `repo.url`, hoặc nằm trong `DetectWorktrees(repoId).on_disk_worktrees[].path` (cache 60 s `(tenant, repoId)`), hoặc trùng `Worktree.path`; ngoài ra `CODEINTEL_PATH_NOT_ALLOWED` **không gọi agent**; (d) `path = repo.url`.
4. `devServerID = repo.dev_server_id`; rỗng → `CODEINTEL_NO_DEV_SERVER` (không rơi về `Project.dev_server_id`).
5. `ListDevServers` (không `kind`, lọc bộ nhớ; cache 30 s theo tenant): tồn tại; `kind == AGENT_KIND_DEV_SERVER`; `approval_status == "approved"` (`CODEINTEL_DEV_SERVER_NOT_APPROVED`); `mode == CONNECTION_MODE_DIRECT_WEBSOCKET` (`CODEINTEL_DEV_SERVER_MODE_UNSUPPORTED`).
6. `NormalizeWorkspacePath(raw, platform)` (domain, theo `DevServer.platform`; mục C); sinh `scope_key` (`wt:<uuid>` cho (a); `path:<repo_id>:<sha256>` cho (b),(d)) và `path_hash`.
7. `Upsert`; nếu `workspace_root`/`dev_server_id` đổi: `index_scope='unresolved'`, `gitnexus_repo=''`, `codegraph_path=''`, `SnapshotRepository.DeleteByBinding`.

Cache kết quả phân giải 30 s theo `(tenant, user, project, worktreeRef)` — key có **user** để một người không kế thừa quyền của người khác; quyền kiểm mỗi lần (SOL-013). Lỗi "không tồn tại" và "tenant khác" cùng `CODEINTEL_NOT_AUTHORIZED`/`WORKTREE_NOT_FOUND` không lộ khác biệt.

### C. `NormalizeWorkspacePath(raw, platform)` (`internal/domain/workspace_path.go`, mới)

Tuyệt đối (POSIX `/`; Windows `X:\`/`X:/`/UNC); từ chối NUL/điều khiển, đoạn `..`, dài > 4096; gộp dấu phân cách lặp, bỏ dấu cuối (trừ gốc); **không** dùng `path/filepath` của máy chạy service; POSIX phân biệt hoa thường; Windows so sánh sau hạ chữ thường + `/`→`\` (giá trị lưu giữ nguyên); không giải quyết symlink (agent làm). MVP: chỉ POSIX hỗ trợ đầy đủ (O-14); `platform=windows` → vẫn chuẩn hoá nhưng agent trả `unsupported_platform` (không kiểm chứng).

### D. Client phía ra (`internal/adapter/grpcclient/`, mới)

`project_client.go`, `infra_fleet_client.go`, `git_gateway_client.go`, `identity_forwarding.go` (chuyển `x-orca-tenant-id`, `-user-id`, `-role`, `-client-ip`), `dial.go` (`grpc.NewClient` insecure + `otelgrpc.NewClientHandler()`, dial lười; infra-fleet thêm `grpc.MaxCallRecvMsgSize(16<<20)`; chuyển token nội bộ **không** cần — các service đích dùng danh tính metadata). Lỗi kết nối → `KindUnavailable` `CODEINTEL_UNAVAILABLE` nội bộ (client chưa sẵn sàng) không làm service chết.

### E. Vòng đời binding

| Sự kiện | Xử lý |
|---|---|
| Lần đầu dùng | Tạo lười (`Upsert`) |
| Mỗi lần dùng | Phân giải lại; đổi `workspace_root`/`dev_server_id` → cập nhật, xoá snapshot |
| `orca.project.worktree.deleted` | `eventbus.Consumer.Subscribe(ctx, "PROJECT", "code-intel-service-worktree-deleted", "orca.project.worktree.deleted", fn)` → `MarkProcessed(event_id)` (lần đầu) → `DeleteByWorktreeID(project_id, worktree_id)` với payload `{worktree_id, project_id,…}` (`lifecycle_events.go`); tenant lấy từ `Event.TenantID` gắn vào ctx |
| `…created` | Bỏ qua |
| Binding rảnh 90 ngày | Bảo trì (SOL-011) |

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | `ListRepos`/`ListWorktrees` thay `GetRepo`/`GetWorktree` | Hai RPC sau không lọc tenant (IDOR) |
| D2 | Đường dẫn của id tổng hợp phải khớp ba nguồn tin cậy | Id do client sinh |
| D3 | Dev server kiểm đủ 4 điều kiện ở service này | `RelayByDevServer` không kiểm approval/mode/user |
| D4 | Cache phân giải khoá theo user | Quyền theo người |
| D5 | Binding là cache có kiểm lại mỗi lần | Không có sự kiện đổi dev server |
| D6 | Consumer bền + dedup | Một lần trên cụm; at-least-once |

## 4. Tiêu chí chấp nhận

- [ ] Ba RPC/message có trong `codeintel_binding.proto`; `buf lint` xanh; không trường đường dẫn/tên repo/dev server trong request.
- [ ] Phân giải đúng 4 dạng id; `::workspace:` → `UNSUPPORTED`; path ngoài ba nguồn → `PATH_NOT_ALLOWED` và **không** gọi `RelayByDevServer` (fake đếm).
- [ ] Repo cục bộ / dev server `pending_approval` / `MOBILE_EMULATOR` / mode ≠ direct-websocket → đúng mã, không gọi agent.
- [ ] `RebindRepoDevServer` (giả lập đổi dev server) → binding cập nhật, `index_scope=unresolved`, snapshot bị xoá.
- [ ] `worktree.deleted` hai lần cùng `event_id` → xoá một lần, không lỗi.
- [ ] Chuẩn hoá đường dẫn qua bảng test (POSIX, Windows, UNC, tương đối, NUL, > 4096).
- [ ] Mọi lời gọi ra mang đủ 4 khoá metadata (fake server).
- [ ] Cô lập tenant: project/worktree/repo tenant khác → không rò sự tồn tại.
- [ ] Không file tên chung chung; không `max-lines` disable.

## 5. Kiểm thử

- **Unit:** `ParseWorktreeRef`, `NormalizeWorkspacePath`, `ResolveTarget` với fake project/infra-fleet/git-gateway (mỗi nhánh lỗi một test; `GetRepo`/`GetWorktree` không bao giờ được gọi — interface fake không có hai phương thức này), test phản chiếu proto.
- **Integration (hai dialect):** `Upsert` giữ `id`; `DeleteByWorktreeID`; truy vấn `(tenant, dev_server, path_hash)`.
- **Consumer:** NATS testcontainer, phát `worktree.deleted` hai lần.
- **Chưa chạy bất kỳ test nào.**

## 6. Rủi ro và điểm chưa kiểm chứng

- `ListWorktrees`/`ListRepos` không phân trang: project hàng trăm worktree làm phân giải nặng; cache 30 s.
- `ListDevServers` trả toàn tenant; chấp nhận với cache 30 s (chưa đo).
- Worktree liên kết thường không có chỉ mục riêng → `OVERLAY`/`STALE` (O-6).
- Người bị gỡ khỏi project mất quyền ngay (SOL-013) nhưng worktree dời chỗ chậm ≤ 30 s.
- `INFRA_DEV_SERVER_NOT_CONNECTED` trả ngay, không xếp hàng ~20 s như README v7; chưa kiểm chứng tầng `Exec`.
- Windows/WSL/macOS: quy tắc so sánh chưa chốt (O-14).

## 7. Câu hỏi mở và điểm hợp đồng cần chủ sở hữu quyết

- **Q1.** **Mâu thuẫn hợp đồng:** `IndexStatus.index_basis = 5` (§2.3) cần `IndexBasis` ở `codeintel_index_basis.proto` (chủ CR-080, đợt 7), nhưng `codeintel_binding.proto` (CR-012, đợt 2) import nó. Đề xuất đã áp: CR-012 **không khai báo** field 5 cho tới khi CR-080 tạo file (additive, số 5 dành sẵn, không `reserved`).
- **Q2.** `GetIndexStatus` ở §3.1 không liệt kê `selector` rõ trong bảng nhưng "selector là trường 1 của mọi request gắn worktree" (§3): áp dụng.
- **Q3.** Quy tắc so sánh đường dẫn WSL/macOS.
- **Q4.** Cho chỉ định tay worktree ngoài `project-service`? Mặc định: không (O4).

## 8. Tham chiếu

- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md` PQ-03, PQ-04, PQ-08, PQ-14; §2.1, §3, §4.2 T2, §5
- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-ui-api.md` §2.1–2.3
- `/opt/repos/orca/docs/crs/v7/code-intel-service-foundation/CR-CV-012-project-worktree-to-repo-binding.md`
- Các file mã đã liệt kê ở mục 1
