# TASK-REQ-031-03: Adapter nguồn nội bộ đọc repo qua Relay (quy ước, ADR, hợp đồng, schema, CI, OPA, git, CodeGraph)

**From Solution:** BE-REQ-SOL-031 (mục A cây `adapter/sources`, CR mục 2.3)
**Priority:** P1
**Service:** `request-service`
**File:** `backend-go/services/request-service/internal/adapter/sources/{repo_files.go,repo_conventions.go,repo_decisions.go,repo_specs.go,repo_contracts.go,repo_schema.go,repo_dependencies.go,ci_config.go,policy_opa.go,git_history.go,code_graph.go}` (mới) và `_test.go`; `.../internal/usecase/ports.go` (thêm `RepoReader`, `CodeGraphRunner`); `.../internal/adapter/grpcclient/relay_repo_reader.go` (mới)
**Depends on:** TASK-REQ-031-02
**Status:** `[x] DONE`

---

## Context

- Repo nằm trên dev server, AGENTS.md buộc hỗ trợ SSH và remote: mọi nguồn dựa vào repo đọc qua `InfraFleetService.Relay` hoặc `RelayByDevServer` (như `AICompleter` ở BE-REQ-SOL-007 task 04), **không** đọc filesystem của backend. Cổng chọn kết nối `AIConnectionResolver` (BE-REQ-SOL-005 mục C1: `connectionID = projectID` có thể trượt, rơi về `RelayByDevServer`) tái dùng; không viết lại logic.
- Tham số Relay đã đọc ở agent (đọc ngày làm solution, trong `agent/src/relay/`):
  - `fs.readFile {path}` (`fs-agent-extensions.ts:103`): nhận cả đường dẫn **tuyệt đối**, nếu tương đối thì nối `config.workDir`; **không có chống `..`** ở phía agent. Bảo vệ phải làm ở backend.
  - `fs.glob {pattern, cwd?, ignore?}` (`:219`): trả tối đa 200 kết quả, `ignore` mặc định `node_modules, .git, dist, out`.
  - `fs.grep {pattern, root?, maxResults?}` (`fs-agent-search-extensions.ts:26`): `maxResults` trần 200, dùng `rg` nếu có.
  - `git.history {worktreePath, limit?, baseRef?}` (`agent-git-handler-extended.ts:70`): cần `worktreePath` (đường dẫn worktree, không phải repo gốc).
  - `agent.exec`: tham số `{binary, args, ...}`; dùng cho `code_graph` (đọc `agent-rpc-dispatch-agent-exec.ts:72` trước khi dựng).
- Kết quả mọi lời gọi Relay là JSON-RPC `result`; lỗi `ServerError` kèm thông điệp, ánh xạ sang `missing{reason}` (không lỗi cho cả pack).
- Tệp CR kể tên có thật trong repo (đã `ls` khi viết CR): `AGENTS.md`, `CLAUDE.md`, `guides/STYLEGUIDE.md`, `.oxlintrc.json`, `config/max-lines-baseline.txt`, `docs/adrs/`, `docs/hld/`, `docs/crs/`, `specs/`, `backend-go/proto/orca/**`, `migrations/{postgres,mysql}`, `.github/workflows/backend-go-*.yml`, `backend-go/policy/orca-authz/*.rego`. Đường dẫn trong mã là **cấu hình mặc định** có thể ghi đè theo project (một số project khác repo này).

## Việc cần làm

1. Port ở `ports.go`:

```go
type RepoReader interface { // một kết nối tới dev server của project
    ReadFile(ctx context.Context, relPath string, maxBytes int) (RepoFile, error)       // fs.readFile
    Glob(ctx context.Context, pattern string, limit int) ([]string, error)               // fs.glob, cwd = gốc repo
    Grep(ctx context.Context, pattern, relRoot string, limit int) ([]GrepHit, error)      // fs.grep
    GitHistory(ctx context.Context, limit int, baseRef string) ([]GitCommit, error)       // git.history
    LastCommitTime(ctx context.Context, relPath string) (time.Time, error)                // git.exec log -1 --format=%cI -- <path>
}
type RepoReaderFactory interface { ForProject(ctx context.Context, projectID string) (RepoReader, error) } // ErrNotConnected nếu dev server rớt
type CodeGraphRunner interface { Explore(ctx context.Context, symbols []string) (CodeGraphResult, error); IndexAge(ctx context.Context) (time.Duration, error) }
```

2. `relay_repo_reader.go`: bản cài trên `infrafleetv1.InfraFleetServiceClient`; `ForProject` dùng `AIConnectionResolver`; `ErrNotConnected` khi `Relay` trả lỗi kết nối (ánh xạ `missing{not_connected}`).
3. Chống đường dẫn (`repo_files.go`): `func SafeRelPath(p string) (string, error)`: từ chối đường dẫn tuyệt đối (bắt đầu `/`, `\`, hoặc có ổ đĩa `C:`), chứa phần tử `..`, ký tự NUL, độ dài > 512; chuẩn hoá bằng `path.Clean` (dạng `/` ở mọi nền tảng vì agent đọc theo repo POSIX hoặc Windows: dùng `path` chứ không `filepath` để không phụ thuộc OS của backend). Mọi `ReadFile`, `Glob` pattern, `Grep` root đều đi qua hàm này. Pattern glob có `..` bị từ chối.
4. Hàm dùng chung `readCapped(r RepoReader, rel string, max int) (SourceItem, error)`: gọi `ReadFile`, cắt theo `max`, tính `Digest` (sha256 nội dung đã cắt), `Size`, `Freshness` từ `LastCommitTime` (so với `TTLSeconds` của nguồn: `fresh` nếu tuổi ≤ TTL, `stale` nếu lớn hơn, `unknown` nếu không lấy được). `Ref` luôn là đường dẫn tương đối repo.
5. Mỗi adapter một file, cùng khuôn `Key() string`, `Search`, `Get`, `List`, `Subscribe` (trả `ErrSubscribeUnsupported`):
   - `repo_conventions.go`: đọc cố định `AGENTS.md`, `CLAUDE.md`, `guides/STYLEGUIDE.md`, `.oxlintrc.json`, `config/max-lines-baseline.txt`; thiếu tệp nào thì bỏ qua tệp đó (không lỗi); `Trust=high`. `Search` trả mọi tệp có mặt (quy ước luôn vào pack), `Query` chỉ để xếp hạng.
   - `repo_decisions.go`: `Glob("docs/adrs/**/*.md")` và `docs/hld/**/*.md` (giới hạn 200), rồi `Grep` theo từ khoá chuẩn hoá của `Query` (tối đa 5 từ dài nhất) để chọn 10 tệp; `Trust=high`, `Freshness` theo `LastCommitTime`.
   - `repo_specs.go`: như trên cho `docs/crs`, `specs`, `docs/logic`, `docs/features`; `Trust=medium` (đã gặp lệch với code).
   - `repo_contracts.go`: `Glob("backend-go/proto/orca/**/*.proto")`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_*.go`, `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/excluded_channels.yaml`; chỉ lấy tệp chứa từ khoá hoặc cùng tên service với `Request.project`; `Trust=high`.
   - `repo_schema.go`: `Glob("**/migrations/postgres/*.sql")` và `mysql`; trả danh sách (tên, không nội dung) cộng **số migration kế tiếp** mỗi service trong `Meta["next_migration"]`; tệp nội dung chỉ đọc khi trùng từ khoá.
   - `repo_dependencies.go`: `go.mod`, `package.json`, `pnpm-lock.yaml` (cắt 16 KB đầu, `truncated`).
   - `ci_config.go`: `.github/workflows/*.yml`, `Makefile`, `backend-go/ci/*`; `Meta["check_commands"]` rút lệnh `run:` (chỉ chuỗi, không thực thi).
   - `policy_opa.go`: `backend-go/policy/orca-authz/*.rego`.
   - `git_history.go`: `GitHistory(limit=30)`; mỗi commit một `SourceItem` (`Ref = sha`), nội dung là subject + file đổi; `Trust=high`; cần `worktreePath` lấy từ `ResolveConnection` (không có worktree thì `missing{not_connected}` lý do `no_worktree`).
   - `code_graph.go`: qua `CodeGraphRunner.Explore(symbols)` với `symbols` lấy từ từ khoá chữ hoa-thường kiểu định danh trong `Query` (regex `[A-Za-z_][A-Za-z0-9_]{3,}`, tối đa 5); `IndexAge` đưa vào `Freshness`: > 24 giờ thì `stale`; lỗi parse hoặc CLI thiếu thì `missing{reason: "invalid_item"}` kèm `Detail` ngắn.
6. Tất cả adapter đăng ký ở `sources/registry.go`: `func NewRegistry(f RepoReaderFactory, cg CodeGraphRunner, ...) usecase.SourceAdapterRegistry` ánh xạ `Key()` → adapter.
7. Không adapter nào ghi log nội dung tệp; chỉ `Ref`, `Size`, thời gian.

## Kiểm thử

- `repo_files_test.go`: `TestSafeRelPath` bảng: `AGENTS.md` ok; `../etc/passwd`, `/etc/passwd`, `C:\x`, `a/../../b`, `a\x00b`, chuỗi 600 ký tự đều lỗi; `docs//adrs/./x.md` chuẩn hoá `docs/adrs/x.md`.
- `repo_conventions_test.go`: `FakeRepoReader` thiếu `CLAUDE.md`: adapter vẫn trả bốn tệp còn lại; tệp > `MaxBytes` bị cắt, `Size` đúng.
- `repo_decisions_test.go`: Glob trả 300 tệp, adapter chỉ gọi `Grep` với tối đa 5 từ khoá; kết quả sắp xếp ổn định.
- `repo_schema_test.go`: danh sách `migrations/postgres/0001..0014` cho `Meta["next_migration"]="0015"`.
- `git_history_test.go`: không có worktree: `ErrNotConnected` có lý do `no_worktree`.
- `code_graph_test.go`: `FakeCodeGraphRunner` trả JSON hỏng: không panic, `missing`.
- `relay_repo_reader_test.go`: fake `InfraFleetServiceClient` kiểm đúng `method` (`fs.readFile`, `fs.glob`, `fs.grep`, `git.history`) và `params_json`.
- Lệnh: `cd backend-go && go test ./services/request-service/internal/adapter/sources/... ./services/request-service/internal/adapter/grpcclient/...`.

## Tiêu chí hoàn thành

- [x] Không có đường đọc repo nào ngoài `RepoReader` (kiểm: không import `os`/`io/fs` trong `adapter/sources`).
- [x] `SafeRelPath` được gọi trước mọi lời gọi Relay có đường dẫn (test dùng fake ghi lại mọi tham số).
- [x] Mọi `SourceItem` hợp lệ qua `Validate`; `Ref` là đường dẫn tương đối.
- [x] Dev server rớt: mọi adapter trả `ErrNotConnected`, Builder (task 05) sinh `missing`.
- [x] Mỗi nguồn nội bộ nào chưa kiểm chứng trên dev server thật được ghi rõ ở `README` của thư mục `sources/` (mục "đã chạy thật / chưa").

## Rủi ro và lưu ý

- Agent chấp nhận đường dẫn tuyệt đối trong `fs.readFile`; nếu backend quên `SafeRelPath` thì một Request độc có thể đọc `/etc/passwd` trên dev server. Đây là rủi ro an ninh chính của task; nên thêm test "Request chứa `../`" ở e2e (CR-REQ-025).
- `fs.grep` qua SSH trên repo lớn (hơn 20 000 tệp) chưa đo; giữ giới hạn 5 từ khoá, `maxResults=50`, timeout nguồn 20 giây.
- `git.history` cần `worktreePath`; Request chưa có worktree nên nguồn này thường rỗng ở bước `classify`.
- Định dạng CLI CodeGraph và GitNexus chưa kiểm chứng; có thể cần đi qua MCP của chúng, khi đó đổi bản cài `CodeGraphRunner`, không đổi adapter.
- Đường dẫn mặc định là của repo Orca; project khác phải ghi đè bằng `context_sources` (ngoài phạm vi, ghi vào `Câu hỏi mở` của solution nếu cần).
