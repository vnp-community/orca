# TASK-REQ-039-03: Worktree đề xuất OpenSpec dùng `base_ref` và bộ kiểm ref qua git-gateway

**From Solution:** [BE-REQ-SOL-039](../solutions/BE-REQ-SOL-039-request-worktree-base-branch.md)
**Priority:** P1
**Service:** `request-service`
**File:** `internal/usecase/{ports_workspace,base_ref_checker}.go` (sửa), `internal/adapter/grpcclient/{proposal_workspace,git_base_ref_checker}.go` (sửa/mới), `cmd/server/wire_plan.go`/`wire_execution.go` (sửa nơi dựng `ProposalWorkspace`), nơi gọi `Ensure` (`engine_openspec.go`, `openspec_plan_builder.go`)
**Depends on:** TASK-REQ-039-02
**Status:** [x] DONE (2026-10-19)

---

## Context

- `ProposalWorkspace.Ensure` luôn dùng `BaseRefDefault`; `EnsureWorkspaceInput` không có nhánh.
- git-gateway đã có `PrefetchCreateBase` và lỗi `WORKTREE_BASE_REF_NOT_FOUND` kèm `available`.
- Khoá idempotency `request-proposal:<id>` và nhánh `ProposalBranch(số)` phải giữ nguyên.

## Việc cần làm

1. `EnsureWorkspaceInput.BaseRef`; nơi gọi `Ensure` truyền `req.BaseRef`.
2. `Ensure`: `base := in.BaseRef`; rỗng thì `BaseRefDefault`; dùng cho `create` lần đầu, lần tạo lại khi worktree đã bị xoá (khoá mới) và `existingFor`.
3. `GitBaseRefChecker` (mới) cài `BaseRefChecker` bằng `PrefetchCreateBase`: timeout 10 giây; `NotFound` khi mã lỗi `WORKTREE_BASE_REF_NOT_FOUND` (lấy `available` làm gợi ý, tối đa 5); `Unreachable` cho `Unavailable`, `DeadlineExceeded`, dev server offline.
4. Khi tạo worktree thất bại vì ref biến mất: trả lỗi miền `base_ref_not_found` để `request.activity`/Execution hiển thị nguyên nhân (không phải lỗi chung).
5. Worktree đề xuất đã tồn tại với gốc khác `base_ref` hiện tại: giữ nguyên (đã khoá); log mức info.

## Kiểm thử

- `TestEnsure_UsesRequestBaseRef`, `_EmptyUsesDefault`, `_RecreateAfterRemovalKeepsBaseRef`, `_BaseRefGone_TypedError`.
- `TestGitBaseRefChecker_Found/NotFound/Unreachable` với client gRPC giả có độ trễ và lỗi.
- Git thật: repo tạm có `main`, `release/x`; `Ensure` với `release/x` tạo worktree có `git rev-parse HEAD` bằng SHA `release/x`; không chỉ định thì bằng SHA `main`.

## Tiêu chí hoàn thành

- [x] Worktree đề xuất rẽ từ `base_ref` khi có, từ nhánh mặc định khi rỗng.
- [x] Idempotency và tên nhánh không đổi.
- [x] Lỗi ref biến mất có mã riêng, hiển thị được.

## Rủi ro và lưu ý

- `PrefetchCreateBase` gọi mạng; trên SSH có thể chậm: đo và ghi vào PR.
- Không đổi hành vi dọn worktree đề xuất (`wire_proposal_janitor.go`).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.

---

## Ghi chú triển khai (2026-10-19)

### Tệp đã đổi (đường dẫn dưới `backend-go/`)

git-gateway-service:
- `internal/usecase/prefetch_create_base.go` (viết lại): ref rỗng hoặc bắt đầu bằng `-` là `InvalidArgument`; ref không có là `NotFound` `WORKTREE_BASE_REF_NOT_FOUND` kèm `available: ...` (nhánh chứa tên được hỏi xếp trước, tối đa 20); `Unavailable` (`WORKTREE_PREFETCH_UNAVAILABLE`), `DeadlineExceeded` (`WORKTREE_PREFETCH_TIMEOUT`) cho mạng/host; lỗi dev server offline giữ mã `GITGATEWAY_DEV_SERVER_UNREACHABLE`; còn lại `Internal`. Phân loại "thiếu" trước "không với tới" vì git in "Could not read from remote repository" sau cả hai.
- `internal/domain/git_ref_errors.go` (mới): nhận dạng văn bản lỗi git.
- `internal/adapter/localgit/executor.go`, `internal/adapter/grpcclient/relay_executor.go` `FetchAndResolveRef`: fetch hỏng mà ref phân giải được trong clone (nhánh chưa push, mất mạng) thì trả commit cục bộ, đúng cái `git worktree add` sẽ phân giải. Relay không còn gọi `git.fetchRef` (agent không có phương thức này, nằm trong `ci/agent-methods-called-known-gaps.txt`; đã xoá dòng đó, `ci/check-agent-methods-called.sh` xanh): dùng `git.exec fetch` + `rev-parse FETCH_HEAD^{commit}`.

request-service:
- `internal/usecase/proposal_workspace_reservation.go` (mới): `ReservingProposalWorkspace`.
- `internal/usecase/{ports_workspace,base_ref_checker,engine_openspec,tasks_md_sync,run_proposal_generation}.go`; `internal/domain/{base_ref_errors,openspec_change,solution_errors}.go` (lý do `execution`, `StoredBaseRefLock`, `IsBaseRefNotFound`, `OpenSpecChange.Reopen`, mã run `REQUEST_BASE_REF_NOT_FOUND`).
- `internal/adapter/grpcclient/{proposal_workspace,proposal_workspace_create,base_ref_gateway_errors,project_default_repo,git_base_ref_checker}.go`; xoá `base_ref_checker_unavailable{,_test}.go`.
- `internal/adapter/{postgres,mysql}/worktree_presence.go` (dùng `domain.StoredBaseRefLock`, bỏ hàm `lockReason` trùng lặp).
- `cmd/server/{main,wire_plan,wire_solution,wire_base_ref_test}.go`.
- Hợp đồng: `contracttest/proposal_reservation_contract.go` (mới, chạy trong `RunBaseRefLockContract`), `base_ref_lock_contract.go` (ca `executing`).

### Quyết định cho bốn vấn đề của brief

1. **Phân biệt "thiếu" và "không với tới"**: xem trên. `GitBaseRefChecker`: `NotFound` (<=5 gợi ý, tên chứa ref xếp trước) chỉ khi gateway trả `NotFound` mang mã `WORKTREE_BASE_REF_NOT_FOUND`; mọi trường hợp khác không hỏi được (kể cả hết 10 giây, dev server offline, project không có repo) là `Unreachable`; caller huỷ ngữ cảnh thì trả lỗi ngữ cảnh. Client nil (không cấu hình git-gateway/project-service) thì `Check` trả `REQUEST_BASE_REF_CHECK_NOT_CONFIGURED` và `verifyBaseRef` trả lỗi đó thay vì lưu ref chưa kiểm. `main.go` nối `NewGitBaseRefChecker(solution.git, solution.proj)`, bỏ placeholder và dòng cảnh báo.
2. **Khe tranh chấp**: `ReservingProposalWorkspace.Ensure` trong MỘT transaction giữ `LockRequest`: đọc `BaseRef` mới từ DB (bỏ qua `in.BaseRef` của caller), ghi dấu `preparing` (`openspec_changes`) nếu chưa có (dùng lại nếu có; `abandoned` thì mở lại bằng `Reopen`; Request đã kết thúc thì không ghi gì), rồi tạo worktree ngoài khoá. Bọc ngoài nên mọi đường gọi `Ensure` (engine, `TasksMdSyncer`) được bảo vệ. `recordWorkspace` (run_proposal_generation) đọc lại hàng thay vì dùng ảnh chụp lúc bắt đầu run, vì ảnh chụp có thể là `abandoned` cũ và sẽ làm mất khoá. Kiểm: test đồng thời mức use case (300 vòng, `-race`), và trên DB thật cả hai dialect (40 vòng; Postgres 4 lần đổi trước, 36 lần dấu trước; MySQL 4 và 36). Kiểm tra đột biến: bỏ `Upsert` dấu thì cả test "dấu có trước khi hỏi git" và test đua đều đỏ, ở cả mức use case lẫn Postgres.
3. **Worktree task**: `domain.StoredBaseRefLock`: trạng thái cuối > worktree đề xuất > `executing` ("execution"). Cả hai dialect dùng chung hàm này. Request `executing` không cần hỏi task-service (`TaskAwareWorktreePresence` bỏ qua). Task 04 dựa vào đây.
4. **Ref biến mất**: lỗi miền `REQUEST_BASE_REF_NOT_FOUND` (kèm `ref`, `suggestions` <=5) tới run dưới mã `REQUEST_BASE_REF_NOT_FOUND` kèm câu "choose another base ref" và danh sách có sẵn. Quy tắc dọn dấu: chỉ khi git nói ref không tồn tại (bằng chứng chắc chắn chưa tạo gì) và dấu vẫn là `preparing` không có `WorkspaceID` thì chuyển `abandoned` (trong transaction có khoá, dùng ngữ cảnh không bị huỷ, hết 10 giây); khi đó người dùng đổi ref được, và lần chạy sau mở lại dấu. MỌI lỗi khác (gateway sập, lỗi chung) giữ dấu vì worktree có thể đã tồn tại. Chấp nhận: tiến trình chết giữa lúc ghi dấu và tạo worktree để lại dấu `preparing`, ref khoá đến lần chạy kế (lần chạy đó dùng lại dấu) hoặc đến khi huỷ Request (`ReleaseOnCancel` đã bỏ dấu `preparing`). Không thêm TTL để không đổi hành vi dọn dẹp.

### Phát hiện với git thật (quan trọng cho task 04)

Với git 2.54, `git worktree add -b <nhánh> <đường dẫn> release/y` khi `release/y` chỉ có dạng `origin/release/y` KHÔNG tạo `<nhánh>`: git tạo nhánh cục bộ `release/y` rồi checkout nó (worktree nằm trên nhánh của người dùng, commit sẽ vào đó). Vì vậy `proposal_workspace_create.go` không bao giờ gửi tên trần cho ref người dùng chọn: thử lần lượt `refs/heads/<ref>`, `refs/remotes/origin/<ref>`, `refs/tags/<ref>` (và `refs/remotes/<ref>` nếu gõ `origin/...`); ref đã bắt đầu bằng `refs/` dùng nguyên. Mỗi dạng đầy đủ hoặc phân giải hoặc lỗi sạch. Ref mặc định (rỗng) vẫn gửi trần như trước. `Workspace.BaseRef` và bản ghi project-service là dạng đầy đủ đã dùng. Task 04 (worktree của task) có cùng rủi ro và phải xử lý tương tự. Test git thật khẳng định worktree ở nhánh `request/42-proposal` và không có nhánh cục bộ `release/y`.

### Lệnh đã chạy và kết quả

- `cd backend-go/services/git-gateway-service && go build ./... && go vet ./... && go test ./... -race -count=1`: xanh (gồm `TestPrefetchCreateBase_*`, `TestFetchAndResolveRef_*` trên git thật có origin bare, nhánh chỉ cục bộ, không origin, remote không đọc được; `TestRelayExecutor_FetchAndResolveRef_*` với relay giả).
- `cd backend-go/services/request-service && go build ./... && go vet ./... && go test ./... -race -count=1`: xanh.
- Tích hợp hai dialect: `export DOCKER_HOST=unix:///Users/datbeohbbh/.colima/default/docker.sock TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock; go test -tags=integration ./internal/adapter/postgres/ ./internal/adapter/mysql/ -count=1 -p 1`: cả hai `ok` (47 s và 74 s), gồm `ProposalReservation` và ca `executing`.
- `bash backend-go/ci/check-agent-methods-called.sh`, `bash backend-go/ci/check-request-service-wiring.sh`: xanh. `gofmt -l`: rỗng.
- Tên test theo task: `TestEnsure_UsesRequestBaseRef`, `_EmptyUsesDefault`, `_RecreateAfterRemovalKeepsBaseRef`, `_BaseRefGone_TypedError` (grpcclient); `TestGitBaseRefChecker_Found/NotFound/Unreachable` (client gRPC giả có độ trễ và lỗi); git thật: `TestRealGit_*` (repo tạm main, release/x, release/y chỉ remote, checkout ở nhánh thứ ba: `release/x` cho HEAD bằng SHA release/x, rỗng cho HEAD bằng SHA main, không phải nhánh thứ ba).

### Test git thật: giới hạn

Tiến trình git-gateway không được khởi động (executor của nó là gói `internal` của module khác). `realGitGateway` trong `proposal_workspace_real_git_test.go` cài lại các lời gọi adapter dùng (BaseRefDefault, CreateWorktree, GetStatus, PrefetchCreateBase) bằng đúng lệnh git và mã lỗi của executor/usecase git-gateway, nên phía request-service là thật (lệnh, ref, worktree trên đĩa, SHA); đường gRPC thật chưa chạy. Chưa chạy lên stack docker compose hay e2e.

### Blast radius (grep, không có GitNexus)

- `ProposalWorkspace.Ensure`/`EnsureWorkspaceInput`: 2 nơi gọi (`engine_openspec.go prepare`, `tasks_md_sync.go workspace`), cộng các fake test. Trường `BaseRef` thêm vào là tuỳ chọn.
- `solutionWiring.workspace` đổi sang kiểu giao diện; nơi dùng: janitor (`Remove`), `TasksMdSyncer`; `wireOpenSpecEngine` nay trả `(registry, error)` (1 nơi gọi).
- `recordWorkspace`: 1 nơi gọi. `verifyBaseRef`: `CreateRequest`, `SetRequestBaseRef`. `lockReason` của hai dialect bị xoá (chỉ dùng nội bộ).
- git-gateway `PrefetchCreateBase`: RPC `PrefetchCreateBase` -> kênh api-gateway `worktree.prefetchCreateBase` (frontend); mã lỗi đổi: thiếu ref `NotFound` (trước đây `Internal`), mạng `Unavailable`, ref rỗng `InvalidArgument`. `FetchAndResolveRef` còn được `ResolvePrBase`/`ResolveMrBase` dùng: nay cũng chấp nhận ref có trong clone cục bộ khi fetch hỏng.

### Lệch so với văn bản task và rủi ro

- Ngoài danh sách file được giao nhưng cần thiết: `domain/*` (hằng và hàm nhỏ), `usecase/run_proposal_generation.go`, `usecase/tasks_md_sync.go`, `backend-go/ci/agent-methods-called-known-gaps.txt`, `git-gateway localgit/relay executor`.
- `request.proto` còn ghi chú lý do khoá chỉ gồm "proposal, task, terminal": cần thêm "execution" (không sửa vì file của agent khác).
- Log mức info khi worktree có sẵn khác gốc chỉ có ở nhánh tìm theo bản ghi project-service (`existingFor`); nhánh idempotency-key trả bản ghi cũ mà không mang gốc nên không so sánh được.
- Chưa đo thời gian `PrefetchCreateBase` trên SSH (log `base ref checked` có `elapsed_ms` để đo khi chạy thật). Timeout 10 giây chưa kiểm với repo lớn.
- git đa ngôn ngữ (LANG khác tiếng Anh) làm lỗi git không nhận dạng được: rơi về `Internal`/`Unreachable`, an toàn (lưu ref chưa kiểm), không bao giờ coi là "thiếu".
- Ref chọn là nhánh cục bộ cũ hơn origin sẽ cắt từ bản cục bộ (không fetch khi tạo); checker thì fetch. Chưa xử lý.
- Dùng `refs/heads/...` làm gốc làm bản ghi `base_ref` của project-service ở dạng đầy đủ; chưa kiểm giao diện/merge-into-base có hiển thị tốt dạng đó không.
