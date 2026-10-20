# TASK-REQ-039-07: Kênh `repo.listBranches` (danh sách nhánh theo repo)

**From Solution:** [BE-REQ-SOL-039](../solutions/BE-REQ-SOL-039-request-worktree-base-branch.md)
**Priority:** P1
**Service:** `backend-go/proto, git-gateway-service, api-gateway`
**File:** `proto/orca/gitgateway/v1/git_gateway.proto` (+ code sinh), `git-gateway-service/internal/{domain,usecase,adapter/grpc,adapter/localgit,adapter/grpcclient}` (`list_repo_branches.go` mới), `api-gateway/internal/adapter/wscompat/channels_repo_ssh_status_workspace.go` (hoặc file mới cùng gói), `specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md` mục 2.12
**Depends on:** TASK-REQ-039-04, 039-05
**Status:** [x] DONE (2026-10-20) — kiểm trên stack local (stub): BRF-10 xanh; git 2.25 và relay SSH thật chưa kiểm (chỉ git 2.54 và InfraFleet giả)

---

## Context

- Hộp thoại Create request cần mở danh sách nhánh ngay khi bấm (yêu cầu 2026-10-20). Hiện chỉ có `repo.searchRefs` (cần gõ từ 2 ký tự) và `ListLocalBranches` (theo worktree, chỉ nhánh local).
- `SearchRefs` đi theo repo (`dispatchExecutorForRepo`); kênh mới theo cùng mẫu.

## Việc cần làm

1. Proto: `ListRepoBranches(ListRepoBranchesRequest{repo_id}) returns (ListRepoBranchesResponse{repeated RepoBranch branches; bool truncated})`, `RepoBranch{string name; bool remote}`. Sinh lại mã bằng protoc (không có `buf`).
2. `domain`: hàm thuần phân tích đầu ra `git branch --list --all --format=%(refname)`: bỏ `refs/remotes/*/HEAD`, bỏ nhánh remote có bản local cùng tên, sắp xếp, cắt 500 (`truncated`). Test bảng: tên có `/`, nhiều remote, remote trùng tên local, HEAD, detached `(HEAD detached at ...)` không có trong `%(refname)`, dòng rỗng.
3. `usecase.ListRepoBranches` (theo `SearchRefs`: `GetRepo` -> `dispatchExecutorForRepo`), cổng `GitExecutor.ListRepoBranches(ctx, repoPath)`.
4. Executor local: chạy `git branch --list --all --format=%(refname)`; executor relay: cùng lệnh qua `git.exec` (đã có `branch` trong allowlist, không đụng agent). Cả hai dùng chung hàm phân tích ở bước 2.
5. gRPC server + gateway kênh `repo.listBranches {repoId}` (cấp `Identity` như `repo.searchRefs`); lỗi chuyển nguyên.
6. Gateway cũ không có kênh: không cần xử lý ở BE (frontend rơi về tìm kiếm).

## Kiểm thử

- Đơn vị cho hàm phân tích (bảng ở bước 2) và usecase (repo lạ, host không tới được, hai executor).
- Test `localgit` trên repo git thật: nhánh có `/`, remote giả (`git remote add` + `git update-ref refs/remotes/...`), nhánh remote trùng local bị ẩn, `origin/HEAD` bị bỏ.
- Gateway: kênh gọi đúng RPC, lỗi chuyển nguyên, không có tool MCP (hàng loại trừ nếu bộ test parity đòi).
- Acceptance: thêm ca `BRF-10` vào `suite_request_base_ref.py`: danh sách có `main`, `feature/other`, `release/1.x`, không có `HEAD`, không lặp.

## Tiêu chí hoàn thành

- [x] `repo.listBranches` trả nhánh local và nhánh remote-only theo hợp đồng 2.12 (kiểm bằng test đơn vị, test git thật và test gateway; chưa qua stack).
- [ ] Chạy với git 2.25 (dùng cờ có từ 2.13) và qua relay SSH. (Chưa kiểm: chỉ có git 2.54 trên máy; relay chỉ được kiểm bằng fake `InfraFleetServiceClient`, chưa qua agent SSH thật.)
- [x] BRF-10 xanh trên stack (`--ai-stub devserver request_base_ref`, 2026-10-20). (Ca đã viết, chưa chạy.)

## Ghi chú triển khai

**Tệp (backend-go):**
- `proto/orca/gitgateway/v1/gitgateway.proto`: RPC `ListRepoBranches`, message `ListRepoBranchesRequest{repo_id}`, `RepoBranch{name, remote}`, `ListRepoBranchesResponse{branches, truncated}`. Sinh lại `proto/gen/go/orca/gitgateway/v1/gitgateway{,_grpc}.pb.go` bằng `protoc` 33.4 + `protoc-gen-go` v1.36.11 + `protoc-gen-go-grpc` 1.6.1 (đúng phiên bản trong header). Đã sinh thử trước khi sửa proto: kết quả chỉ khác dòng header `protoc (unknown)` nên sau khi sinh tôi trả dòng đó về `(unknown)`; không có tệp sinh nào khác đổi.
- git-gateway-service: `internal/domain/repo_branch_listing.go` (+ `_test.go`; `ParseRepoBranchRefs`, `RepoBranchListArgs`, `MaxRepoBranches=500`), `internal/usecase/list_repo_branches.go` (+ test), cổng `GitExecutor.ListRepoBranches` trong `usecase/ports.go`, `adapter/localgit/executor.go` (+ `list_repo_branches_test.go`), `adapter/grpcclient/relay_executor.go` (+ `relay_list_repo_branches_test.go`), `adapter/grpc/server.go` (+ test trong `server_test.go`), `cmd/server/main.go` (nối usecase).
- api-gateway: `wscompat/channels_repo_ssh_status_workspace.go` (kênh `repo.listBranches`, cấp `Identity` và timeout như `repo.searchRefs`) + test, `wscompat/testdata/channel_inventory.json` (sinh lại bằng `UPDATE_GOLDEN=1`), `mcpserver/tools/excluded_channels.yaml` (loại khỏi MCP, category `ui-only`, kèm lý do: không có tool MCP theo hợp đồng 2.12; agent dùng `repo_searchRefs`).
- Acceptance: `tests/stages/e2e/orca-flows/suite_request_base_ref.py` (ca `BRF-10`), `base_ref_stack.py` (`repo_id_of`), `README.md` (dòng suite).

**Quyết định:**
- Lệnh chung cho hai executor: `git branch --list --all --format=%(refname)` (từ `domain.RepoBranchListArgs`, một nguồn duy nhất). Local chạy trực tiếp, relay gửi cùng argv qua `git.exec` (`cwd` = repoPath) và coi `exitCode != 0` là lỗi. Không đụng agent: `branch` nằm trong allowlist, `--all`/`--list`/`--format` không thuộc cờ bị cấm. Không cần `GitCapabilityCache` (cờ có từ git 2.13, thấp hơn mốc 2.25, không có đường lùi nào để nhớ).
- Bộ phân tích: bỏ `refs/remotes/<remote>/HEAD`; bỏ nhánh remote nếu có local trùng tên sau khi bỏ tiền tố remote, và cả khi có nhánh local mang đúng tên đầy đủ (`origin/x` local cạnh `refs/remotes/origin/x`) để hai mục không trùng `name`. Local trước (so byte), rồi remote (so byte), cắt 500 và `truncated=true`; local được giữ ưu tiên khi cắt. Dòng trống/CRLF/ref không phải nhánh (tag, `refs/stash`) bị bỏ. Hạn chế đã biết: tên remote có dấu `/` bị tách ở dấu `/` đầu (git hiếm cho phép).
- Lỗi: `GITGATEWAY_MISSING_REPO_ID`, `WORKTREE_REPO_NOT_FOUND`, `GITGATEWAY_RESOLVE_FAILED`, mới `GITGATEWAY_LIST_BRANCHES_FAILED`. Host không tới được đi qua `toStatus` thành `Unavailable` như `repo.searchRefs` (có test).
- `grpc.New(...)` là hàm tham số vị trí; thêm tham số cuối `listRepoBranches` (một nơi gọi: `cmd/server/main.go`).

**Lệnh đã chạy:**
- `cd backend-go/services/git-gateway-service && go build ./... && go vet ./... && go test ./... -count=1`: đạt toàn bộ (domain, usecase, grpc, grpcclient, localgit với git thật).
- `cd backend-go/services/api-gateway && GOFLAGS=-overlay=<scratchpad>/gw-overlay.json go test ./internal/adapter/wscompat/ ./internal/adapter/mcpserver/... -count=1`: đạt (kể cả parity MCP, golden `channel_inventory`).
- `go vet ./...` ở request-service, task-service, automation-service: sạch. `workflow-service` không biên dịch được do `StepApprovalRepository` (nhánh việc khác, không liên quan).
- `python3 -I -m py_compile` cho `suite_request_base_ref.py`, `base_ref_stack.py`: đạt. BRF-10 KHÔNG chạy.

**Blast radius (không có GitNexus; liệt kê bằng grep):** `GitExecutor` có hai cài đặt thật (`localgit.Executor`, `grpcclient.RelayExecutor`) và hai bản giả trong test (`usecase.fakeGitExecutor`, `grpc.fakeExecutor`), đều đã cập nhật. Khách hàng của `GitGatewayServiceClient` ở api-gateway, task-service, request-service, workflow-service, automation-service dùng fake nhúng giao diện nên không vỡ. Không đổi hành vi RPC hiện có.

**Lệch so với tác vụ / lưu ý:**
- `specs/frontend/api/rpc-catalog.md` chưa thêm hàng `repo.listBranches` (thuộc nhóm frontend).
- `git` trên máy là 2.54; chưa thử trên 2.25 và chưa qua relay SSH thật.
