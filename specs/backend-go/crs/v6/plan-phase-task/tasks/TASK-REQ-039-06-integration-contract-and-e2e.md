# TASK-REQ-039-06: Tích hợp, hợp đồng và e2e với repo fixture tài chính

**From Solution:** [BE-REQ-SOL-039](../solutions/BE-REQ-SOL-039-request-worktree-base-branch.md)
**Priority:** P1
**Service:** `tests/e2e, request-service`
**File:** `tests/stages/e2e/orca-flows/suite_request_base_ref.py` (mới, đăng ký trong `run_acceptance.py`), `tests/stages/e2e/fixtures/ledgerlite-finance/` (repo fixture, xem kế hoạch tổng mục 4), `backend-go/services/request-service/internal/adapter/contracttest/execution_flow_contract.go` (sửa)
**Depends on:** TASK-REQ-039-03, 039-04, 039-05
**Status:** [x] DONE (2026-10-20) — BRF-01..08 xanh trên stack local (stub); BRF-09 (OpenSpec thật, 7–13 USD) bỏ qua theo thiết kế, chỉ chạy khi được đồng ý

---

## Context

- Bộ acceptance chạy qua WebSocket gateway trên stack local; dev server là agent thật (`start-local-agent.sh`).
- Fixture `ledgerlite-finance` (đã dựng, chưa commit; `README.md` của nó là nguồn sự thật) là repo tài chính nhỏ có URD/SRS/SAD, `.tdd-agent/config.toml` và ba kịch bản trong `docs/e2e-scenarios.md`; e2e sao chép vào thư mục git tạm, chạy `tdd-agent generate` + `mcp enable --read-only`, rồi một commit. Tri thức đã sinh không được commit.
- Quy tắc: `run_ui_flow.py --real` không cho sửa checkout orca trong lúc chạy.

## Việc cần làm

1. Suite `suite_request_base_ref.py` (id `BRF-xx`): (a) tạo Request không `baseRef` → `baseRef==""`; (b) tạo với `release/x` → lưu đúng; (c) `INVALID`/`NOT_FOUND`; (d) `setBaseRef` khi chưa khoá thành công, phiên bản tăng; (e) sau khi tạo worktree đề xuất hoặc task đầu thì `LOCKED`; (f) **kiểm nhánh gốc thật**: dùng claude giả (`--ai-stub`) để chạy một Request `task` native trên fixture, đọc `git rev-parse` của worktree trong dev server và so với SHA của nhánh đã chọn; (g) checkout chính của fixture ở nhánh thứ ba khi không chỉ định: worktree rẽ từ nhánh mặc định.
2. Kịch bản OpenSpec (change_request) với `base_ref`: worktree đề xuất rẽ từ ref; chạy với claude giả nếu stub hỗ trợ OpenSpec, nếu không thì ghi rõ ca nào chỉ chạy được với Claude thật và để ở chế độ opt-in `ORCA_E2E_OPENSPEC=1` (tốn tiền, người dùng đồng ý trước).
3. Spec UI (`ui-live/request-base-branch.web.e2e.ts`) được làm ở FE-REQ-TASK-040-05; task này cung cấp fixture và helper tạo repo tạm (`fixture-repo-ledgerlite.ts`).
4. `execution_flow_contract.go`: kịch bản Request có `base_ref` đi hết một Plan với fake task service, kiểm `BaseRef` nhận được ở cả hai đường dispatch.
5. Đăng ký suite `request_base_ref` trong `run_acceptance.py`; thêm vào bộ chạy CI nếu không dùng AI thật.

## Kiểm thử

- `python3 tests/stages/e2e/orca-flows/run_acceptance.py devserver request_base_ref` xanh; `... qa` không hỏng.
- `go test ./services/request-service/internal/adapter/contracttest/... -run 'Execution' -v` xanh trên hai dialect.
- Chạy lại `qa-bug-task-native-flow` (stub) để chắc không hồi quy.

## Tiêu chí hoàn thành

- [x] Suite `BRF-xx` xanh trên stack local với repo fixture thật: 8 đạt, 1 bỏ qua (BRF-09, opt-in), 0 lỗi (`--ai-stub devserver request_base_ref`, 2026-10-20). Chạy thật phát hiện 2 lỗi sản phẩm đã sửa: relay `ListLocalBranches` dùng `for-each-ref` ngoài allowlist `git.exec` (BRF-04), và `BaseRefDefault` cắt nhánh có dấu `/` / rơi về nhánh đang checkout (BRF-07).
- [x] Ca nhánh gốc kiểm bằng `git rev-parse` thật, không mock (phần git là thật và có test âm; phía Orca chưa chạy trên stack).
- [x] Không hồi quy `request`, `matrix`, `ai`, `plan`, `analysis`, `task`, `aistub`: 102 đạt, 0 lỗi, 3 chặn (cần AI/MCP thật, có sẵn), 2 bỏ qua; UI `qa-bug-task-native-flow` 3/3; `qa-regressions` QA-UI-TERM-01 đạt (không stub), QA-UI-GH-01 đỏ vì nguồn GitHub bị hoãn theo yêu cầu.

## Rủi ro và lưu ý

- Ca OpenSpec thật tốn 7 đến 13 USD và cần `.tdd-agent/config.toml` trong fixture; không tự chạy khi chưa được đồng ý.
- Stack local mất Vault khi khởi động lại; chuẩn bị lại bằng `vault-init` trước khi chạy.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.


## Ghi chú triển khai (2026-10-19)

### Tệp

- Mới: `tests/stages/e2e/orca-flows/suite_request_base_ref.py` (BRF-01..09), `base_ref_stack.py` (project gắn repo fixture, tạo Request có `baseRef`, đi từ `planning` tới duyệt Plan, đọc trailer lỗi), `base_ref_worktrees.py` (đọc worktree và commit bắt đầu bằng git thật, dọn worktree), `test_base_ref_worktrees.py` (9 test đơn vị trên repo LedgerLite thật).
- Sửa: `run_acceptance.py` (import, dòng `request_base_ref` trong `SUITES`, thêm vào `AI_USING`), `README.md` (dòng trong "Danh mục test"), tệp task này.
- Không sửa `contracttest/**`: mục 4 của task đã được `execution_flow_base_ref_contract.go` (task 04) phủ đủ, xem dưới. Không đổi `ci/run-orca-flows-ci.sh` (ngoài danh sách tệp): chế độ `weekly` chạy mọi suite nên có BRF; `nightly` chưa có, thêm `request_base_ref` vào đó nếu muốn.
- Mục 3 (spec UI) và thư mục `tests/stages/e2e/fixtures/ledgerlite-finance/` không thuộc tác vụ này (helper `ledgerlite_fixture.py` đã có).

### Lệnh chạy suite (orchestrator)

`python3 tests/stages/e2e/orca-flows/run_acceptance.py --ai-stub devserver request_base_ref`

Suite tự dựng repo LedgerLite (`ledgerlite_fixture.create(release_branch=True, third_branch="feature/other")`: `main`, `release/1.x` có thêm một commit, checkout chính để ở `feature/other`), đăng ký thành project trên dev server, đặt engine `native` và `solutionAgent=false`, rồi dọn hết (huỷ Request, xoá project, `git worktree remove --force`, xoá repo). Giả định như `DEV-04`: dev server là máy chạy test.

### Mỗi ca chứng minh gì

| ID | Chứng minh |
|---|---|
| BRF-01 | Tạo không `baseRef`: `baseRef==""`, `baseRefLocked==false`, không có lý do khoá; `request.flowStatus.features` có `request.baseRef` |
| BRF-02 | Tạo với `release/1.x`: lưu đúng, `baseRefUnverified` không bật (dev server nối nên kiểm ref thật), `request.get` trả lại |
| BRF-03 | `-x` và `a..b` là `REQUEST_BASE_REF_INVALID` ở `request.create` và `request.setBaseRef`; lỗi không đổi version |
| BRF-04 | Ref lạ `release/9.x` là `REQUEST_BASE_REF_NOT_FOUND` ở cả hai kênh; trailer JSON có `suggestions` không rỗng và là nhánh có thật của repo; ref không đổi |
| BRF-05 | `setBaseRef` khi chưa khoá đổi ref và tăng version; `expectedVersion` cũ là `REQUEST_VERSION_CONFLICT` và không đổi gì; `""` đặt về mặc định |
| BRF-06 | BẰNG CHỨNG: Request `task` có `baseRef=release/1.x`, checkout chính ở `feature/other`; duyệt Plan; task-service tạo worktree `task/<id>`; `git rev-parse HEAD` của worktree bằng đỉnh `release/1.x` (hoặc con cháu của nó nếu agent thật đã commit) và không chứa đỉnh `feature/other`; checkout chính vẫn ở `feature/other` |
| BRF-07 | Như BRF-06 khi không đặt `baseRef`: HEAD của worktree bằng đỉnh `main` (nhánh mặc định), không chứa `release/1.x` hay `feature/other` |
| BRF-08 | Có worktree rồi thì `setBaseRef` là `REQUEST_BASE_REF_LOCKED` với lý do `task` hoặc `execution` (trailer), `request.get` báo `baseRefLocked=true` cùng lý do, ref không đổi |
| BRF-09 | Worktree đề xuất OpenSpec `request/<số>-proposal` rẽ từ `release/1.x` (HEAD là hoặc là con cháu của đỉnh nó, không chứa `feature/other`) và khoá ref với lý do `proposal` |

BRF-06..08 chỉ chạy với `--ai-stub` hoặc `ORCA_E2E_EXECUTE_AGENT=1` (duyệt Plan khởi động agent); thiếu cả hai thì `SKIP` có lý do. Với `--ai-stub` agent là `claude` giả, từ chối công cụ ghi nên task hỏng ngay sau khi task-service đã tạo worktree: đủ cho các ca này, không tốn tiền. Không có AI để phân loại thì `BLOCKED`.

BRF-09 là opt-in: `ORCA_E2E_OPENSPEC=1`, không `--ai-stub` (claude giả không viết được OpenSpec), cần `tdd-agent` trên PATH (repo fixture có tri thức), Claude thật đăng nhập; tốn khoảng 7 đến 13 USD (Solution thật và lượt viết proposal). Mặc định là `SKIP` có thông báo, không giả lập. Ca dừng ngay khi worktree xuất hiện, rồi huỷ Request ở bước dọn.

### Kiểm chứng đã làm (không chạm stack)

- `python3 -m py_compile` cho 4 tệp Python mới/sửa; `python3 tests/stages/e2e/orca-flows/run_acceptance.py --list` hiện `request_base_ref`.
- `python3 -m unittest discover -s tests/stages/e2e/orca-flows -p 'test_base_ref_worktrees.py'`: 9 test xanh, trên repo LedgerLite thật và `git worktree` thật (worktree nằm ngoài repo như agent): đúng cho worktree rẽ từ `release/1.x` (không tính `main` là lỗi vì nó là tổ tiên) và từ `main`; ĐỎ khi worktree rẽ từ nhánh thứ ba hoặc từ nhánh sai; vẫn xanh khi agent commit thêm; dọn dẹp xoá cả worktree ngoài repo, chạy lại không lỗi. `test_ledgerlite_fixture.py` vẫn xanh (5 test).
- Chạy thử toàn bộ suite với một WebSocket giả (tệp trong scratchpad, không đưa vào repo) mô phỏng Orca nhưng tạo worktree bằng git thật: BRF-01..08 xanh; đột biến "worktree rẽ từ HEAD của checkout" làm BRF-06 và BRF-07 đỏ với thông điệp nêu rõ commit; đột biến "không bao giờ khoá" làm BRF-08 đỏ. Đây là kiểm logic của suite, KHÔNG phải kiểm sản phẩm.
- Hợp đồng (mục 4, task 04 đã làm): `ExecutionFlow_BaseRef_ReachesTaskServiceOnBothDispatchPaths` (5 ca: đường thường và đường hợp đồng, có ref, ref đặt sau khi tạo, không ref) đã được đăng ký trong `execution_flow_contract.go`. Chạy lại: `cd backend-go/services/request-service && DOCKER_HOST=unix:///Users/datbeohbbh/.colima/default/docker.sock TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock go test -tags=integration ./internal/adapter/postgres/ ./internal/adapter/mysql/ -run 'ExecutionFlow|BaseRef' -count=1 -p 1`: cả hai dialect xanh. Không có khoảng trống nên không thêm tệp hợp đồng.

### Vì sao `request_base_ref` đứng sau `aistub`

Duyệt Plan làm agent gọi `claude --print` kèm công cụ ghi với prompt thực thi; `fake_claude_cli/claude` từ chối và ghi lời gọi là `unknown`. `AST-03` (không prompt lạ) và `AST-02` đọc mọi lời gọi tại thời điểm chúng chạy, nên suite này chạy sau chúng. Pha 2 của `--ai-stub` (đăng xuất) không chạy suite này.

### Chưa kiểm chứng và rủi ro

- Chưa chạy trên stack: `BRF-06..08` cần task-service gửi worktree cho dev server là máy chạy test (thư mục `git worktree list` của repo fixture thấy được); nếu worktree của task nằm trong repo khác hoặc agent chạy trên máy khác thì các ca đỏ/hết hạn 150 giây.
- Giả định giao diện: `request.setBaseRef` trả `{request}`, trailer lỗi dạng ` {json}` cuối thông điệp, nhánh worktree task là `task/<id>` của các task `task.list {requestId}` (đọc từ mã task-service và gateway, chưa thấy thật). Lý do khoá `task` hoặc `execution` theo brief.
- Dọn: huỷ Request đang `executing` bị từ chối khi còn task `in_progress` (agent guard), nên bước dọn thử lại tới 90 giây; lỗi dọn chỉ in `[cleanup]`, không làm đỏ ca.
- `BRF-09` chưa chạy lần nào (cần Claude thật và tiền); logic dựng theo `PLN-08` và `engine_openspec.go` (`generatePlan` chạy proposal run, run tạo worktree `request/<số>-proposal`).
- Không có gì ghi nội dung Request hay bí mật vào log; suite chỉ in id, nhánh, SHA rút gọn.
