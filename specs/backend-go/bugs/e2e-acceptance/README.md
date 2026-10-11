# Handoff + chỉ mục đợt triển khai: nghiệm thu e2e các luồng ORCA (Request flow CR v6)

Dành cho agent hoặc người nhận việc tiếp. Đọc hết file này trước khi sửa mã. Ngày chốt: 2026-10-09 (đã cập nhật sau đợt hoàn thiện cuối ngày). Nhánh: `feat/e2e-acceptance-fixes`.

## 1. Bối cảnh một đoạn

Mục tiêu của công việc: làm cho luồng Request (CR v6), workflow, task và dev-server/agent chạy ổn định trên stack Docker local, với máy Mac của người dùng làm dev server và **Claude Code đã đăng nhập** làm nguồn AI (không dùng khoá API). Quy trình đã áp dụng: viết bộ e2e nghiệm thu, chạy, thu lỗi, viết đánh giá + giải pháp + checklist, rồi sửa. Đã đi hai vòng (v1, v2).

**Kết quả cuối:** xem mục 5 (e2e thật qua UI 6/6 xanh với Claude Code thật; backend `--ai-stub` 114 PASS, 0 FAIL; Claude thật 103 PASS, 0 FAIL).

## 2. Bản đồ tài liệu (đọc theo thứ tự)

| Tài liệu | Dùng để |
|---|---|
| [tests/ORCA-E2E-ACCEPTANCE-REPORT.md](../../../../tests/ORCA-E2E-ACCEPTANCE-REPORT.md) | báo cáo gốc + mục 9 (kết quả sau sửa) |
| [v1/](./v1/) | đánh giá ảnh hưởng, giải pháp theo nhóm nguyên nhân gốc, **checklist 35 task** (`tasks/README.md`, mỗi file task có mục "Kết quả thực thi") |
| [v2/](./v2/) | lỗi tìm thêm khi chạy lại, giải pháp giai đoạn Plan, checklist v2 |
| [specs/frontend/bugs/e2e-acceptance-v1/tasks/](../../../frontend/bugs/e2e-acceptance-v1/tasks/README.md) | 8 task frontend, **chưa bắt đầu** |
| [CONTRACT-request-ui-api.md](../../crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md) | hợp đồng kênh WS (đã thêm mục 2.6 `clarification.*`, `valueType`, `retryAfterMs`, mã lỗi) |
| [tests/stages/e2e/orca-flows/README.md](../../../../tests/stages/e2e/orca-flows/README.md) | cách chạy và thêm test e2e |

Ký hiệu checklist: `[x]` xong, `[~]` một phần, `[ ]` chưa làm, `[-]` không áp dụng.

## 3. Dựng môi trường để tiếp tục (đã kiểm trên macOS arm64 + Colima)

```bash
export DOCKER_HOST=unix:///Users/<you>/.colima/default/docker.sock   # Colima; Docker Desktop thì bỏ
export TESTCONTAINERS_RYUK_DISABLED=true                              # chỉ cho test Go dùng testcontainers
bash deploy/dev/scripts/ensure-request-env.sh                         # sinh token nội bộ còn thiếu vào deploy/dev/.env (không in giá trị)
bash deploy/dev/scripts/redeploy-local.sh request-service api-gateway # build, migrate, up -d, restart nếu có container bị tạo lại, smoke PLT
node agent/build.mjs && bash deploy/dev/scripts/start-local-agent.sh  # dev server = máy này, KHÔNG có khoá API trong env của agent
python3 tests/stages/e2e/orca-flows/run_acceptance.py                 # toàn bộ; thêm tên suite để chạy một phần
```

Bẫy đã gặp, đừng đạp lại:
- **Không tạo lại Vault/Postgres/NATS.** Vault dev lưu trong bộ nhớ; `redeploy-local.sh` từ chối khởi động lại các service này.
- **Tạo lại container làm các service giữ IP cũ** ("unknown service ...", `REQUEST_NOT_IMPLEMENTED`). Script đã tự restart mọi service Go khi thấy "Recreate". Nếu tự chạy `docker compose up`, hãy restart tay.
- **Kiến trúc**: máy arm64 phải build `GOARCH=arm64` (script tự nhận). macOS không có `timeout`, `sha256sum`; zsh không tách từ biến `$var`.
- **Giới hạn ghi của request-service**: 5 ghi/giây, burst 10 (đổi bằng `REQUEST_RATE_<READ|WRITE|AI|WEBHOOK>_PER_SECOND/_BURST`). Client e2e tự thử lại theo `retryAfterMs`.
- **Agent mất kết nối khi infra-fleet khởi động lại**: từ giờ agent tự nối lại sau ~6 giây (đóng bằng mã 1001). Agent build cũ thì thoát hẳn; chạy lại `start-local-agent.sh`.
- Không dùng `claude --bare` (bỏ qua OAuth/keychain). CLI chưa đăng nhập trả exit 1 + JSON `is_error:true`, `result:"Not logged in · Please run /login"`.
- Mật khẩu admin và token đọc từ `deploy/dev/.env` (gitignored). **Không bao giờ in ra log hay commit.**

## 4. Đã làm được gì (tóm tắt theo hướng kỹ thuật)

- **Hợp đồng tầng gateway↔service↔agent↔frontend**: kênh `clarification.*`; test parity RPC↔kênh (`channels_request_parity_test.go` + bảng `request_rpc_channel_map_test.go`); tệp vàng danh mục kênh (`wscompat/testdata/channel_inventory.json`, `UPDATE_GOLDEN=1`); script kiểm method backend gọi agent (`backend-go/ci/check-agent-methods-called.sh`, nợ đã biết trong `agent-methods-called-known-gaps.txt`, chỉ được giảm); `fs.checkPath` ở agent.
- **Danh tính xuyên service**: `common/grpcmw.NewClient` chuyển tenant/user/actor; mọi service dùng nó (`backend-go/ci/check-grpc-dials.sh` chặn dial thủ công; api-gateway cố ý loại trừ vì tự gắn danh tính); `tenant.Detach`; workflow lưu `started_by` (migration `0013`) và khôi phục khi recover; task suy người tạo từ danh tính.
- **Truy cập project**: RPC nội bộ `ProjectService.ResolveProjectAccess` (guard `internalcaller`, token `PROJECT_INTERNAL_CALLER_TOKEN`); request-service từ chối project không tồn tại (`REQUEST_PROJECT_NOT_FOUND`); id project sai định dạng là NotFound.
- **AI qua phiên Claude đã đăng nhập** (`agent/src/relay/ai-complete-cli-session.ts`): `claude -p --output-format json --tools ""`, prompt qua stdin, cwd tạm rỗng, env whitelist, hàng đợi 2+8, kill cả nhóm tiến trình khi quá giờ. Thứ tự nguồn: env key → `resolvedApiKey` → phiên CLI. Agent công bố `aiSources`; request-service kiểm sẵn sàng AI bằng khả năng thật và trả mã `REQUEST_AI_*` có cấu trúc. Công tắc `ORCA_AI_CLI_SESSION=off`.
- **Giai đoạn Plan** (CR-REQ-012, phần request-service): `GeneratePlan` (chạy nền trên `AnalysisRun` loại `plan`, migration `0093`), `GetPlanProposal`, `CommitPlan` (ghi qua task-service `CreatePlanTree`, ghi `plan_task_id`, mở Approval `plan`/`task_list`/`pre_deploy` theo `StartGate`, chạy lại an toàn, làm lại Plan có `supersedes_plan_id`), `PlanArtifacts` (digest cây con). Điểm vào đọc code: `usecase/generate_plan.go`, `run_plan_generation.go`, `commit_plan.go`, `cmd/server/wire_plan.go`.
- **Vận hành**: giới hạn tốc độ cấu hình được + metric `orca_request_rate_limited_total{class}`; `/readyz` kiểm token nội bộ; `value_type` của câu hỏi Clarification; sức khoẻ dev server làm mới ngay khi agent attach; chủ đề outbox `dev_server.health_degraded` đưa vào stream NATS (migration infra-fleet `0040`); `redeploy-local.sh` tự chạy migration.
- **Bộ e2e** `tests/stages/e2e/orca-flows` (Python thuần, không cần pip): suite `platform devserver workflow request matrix exits ai plan task notfound`; `report_orphan_requests.py` (chỉ đọc, `--cancel` mới huỷ).

## 5. Kết quả cuối đợt và việc còn tồn đọng (2026-10-10, đã nghiệm thu bằng e2e thật qua UI)

**Đã đạt (chạy thật, Claude Code thật trên dev server local, không stub, từ giao diện web):**
- `run_ui_flow.py --real all`: 6/6 xanh (21.4 phút): thực thi `change_request` có Phase (5 task phụ thuộc), task có phụ thuộc, task 1 bước; phân tích chỉ đọc `question`, `bug`; theo dõi phiên agent (`monitor`). Cộng 3 test request-live (change_request, question; ask-back chỉ có bản stub).
- Backend `--strict --ai-stub`: 114 PASS, 0 FAIL, 2 BLOCKED (MCP tắt). Claude thật: 103 PASS, 0 FAIL, 4 BLOCKED, 1 SKIP.
- Trên web theo dõi được: panel Agent activity (dòng `▸ Read/Edit/Bash` thật), trạng thái dev server busy/idle + `macOS · arm64`, tab Result (Agent output), Open in Git (Source Control hiện file đổi), terminal worktree.
- Thiết kế an toàn e2e thật: repo tạm, hook chặn `PreToolUse`, dấu vân tay trước/sau; mọi lượt 0 vi phạm, 0 Request mồ côi.
- Lỗi sản phẩm tìm ra và đã sửa nhờ e2e thật: cổng Phase không mở (B1), task phụ thuộc kẹt (B2), danh tính thực thi (Q1), output agent không tới UI (G1), platform dev server (G2), `git.status` worktree rỗng + `CheckWorktreeDeleteSafety` luôn thấy sạch (G5c), một repo hỏng làm mất mọi worktree (G5), gateway panic, v.v. (xem v2/tasks/README.md và frontend tasks).

**Còn tồn đọng, theo ưu tiên:**
1. **Quyết định cần người dùng:**
   - **B4** — task thực thi luôn `--dangerously-skip-permissions` không sandbox (task-service gửi `trustPreset:"full"`). Cần chính sách theo project hoặc sandbox riêng.
   - **Hướng A (phiên PTY như TDD `specs/frontend/tdd/v5/15-task-graph-ui.md` / HLD C4.9) hay B (giữ `execPrompt`, hiển thị tương đương, cập nhật TDD)** — hiện đang chạy hướng B.
   - **G7** — hook an toàn của e2e từ chối scratchpad của phiên Claude (ngẫu nhiên); đề xuất cho phép hẹp + dọn 43 thư mục `/private/tmp/claude-501/*e2e-ui*` và 68 bản ghi `~/.claude/projects/*e2e-ui*`.
   - Xoá 4 project `probe-proj` cũ (repo đã mất) và 2 dev server `dev-local-mac-stub-in/out`.
   - Nối `ApprovalGates.CheckPlan` vào duyệt Plan (CR-REQ-028 `plan_assumption`).
2. **Đã sửa (2026-10-11): web thêm project mất dev server** (TASK-FE-E2E-015..021, TASK-V2-035): chọn host khi thêm project, repo gắn host, git-gateway không fallback im lặng; repo cũ `dev_server_id` rỗng báo `GITGATEWAY_REPO_HOST_UNBOUND` cho tới khi dùng "Change dev server" hoặc `deploy/dev/scripts/rebind-unbound-repos.sh` (dry-run).
2a. **Đã sửa (2026-10-11): Add Project tạo project riêng; chặn tạo trùng repo; ô Answer** (TASK-FE-E2E-022..027, TASK-V2-036..040): mỗi Add Project tạo một project mới chứa đúng repo, `project.create` ghi project+owner+repo trong một giao dịch, trùng (cùng tenant, dev server, đường dẫn chuẩn hoá) bị chặn bằng `PROJECT_REPO_ALREADY_REGISTERED` (migration 0032, chỉ cho bản ghi mới), `AnswerView` đọc `answer_markdown`. E2E thật: `run_ui_flow.py --real addproject`.
2b. **Lỗi/gap đã biết chưa sửa:** (mới) nhập repo lồng nhau `project.scanNested` luôn lỗi vì agent thiếu handler `fs.scanNestedRepos`; kênh `orcaProfiles.findProjectProfiles` chưa có ở gateway; Radix thiếu Description ở hộp thoại Add Project; bảng markdown thiếu `remark-gfm`; `project.importNested` không thêm owner cho người nhập; G5b (`SCM_EMPTY_BRANCH`: frontend gửi `branch/base/repo`, gateway đọc `headBranch/baseBranch`); G5d (`git.upstreamStatus/diff/compare/history` trả snake_case, frontend cần camelCase) và G5e (`RelayExecutor.GetDiff` rỗng, `Commit` thiếu SHA) — xem diff của file đã đổi đang vỡ, được ghi `known-gap` trong spec; `--real monitor` còn một bước thông tin; không giữ lịch sử từng dòng hoạt động sau khi tải lại (cần lưu ở backend); `/readyz` cho auth/mcp/project; đẩy sự kiện `solution.superseded`/`clarification.answered`; `NakWithDelay` cho `consumeUntilDone`; hotfix/security/performance chưa có e2e riêng; điều kiện đua mất chunk đầu `· Session started` nếu infra-fleet chậm.
3. **Test flaky:** `RQL-13`/`RQL-19` của `run_acceptance.py --ai-stub` đỏ 1 trong 3 lượt đầy đủ ngay sau redeploy/khởi động lại agent (`REQUEST_VERSION_CONFLICT`, expected 4); 3/3 xanh khi chạy riêng suite `request` và 2/2 xanh ở các lượt đầy đủ sau đó. Chưa tách được nguyên nhân (nghi hai lượt phân loại chồng nhau: sự kiện + `request.classify` thủ công của test).
4. **Hạ tầng/CI:** workflow `orca-flows-e2e.yml`, `backend-go-request-contracts.yml` chưa chạy trên runner GitHub; spec thật (Claude thật) chưa có đường chạy CI (cần tài khoản Claude trên runner); README triển khai chưa dựng thử từ máy sạch; kiểm Electron chưa làm; lint/vitest toàn frontend vẫn đỏ do lỗi có sẵn trên HEAD; script T2 `backend-go/ci/request-e2e/run-request-e2e.sh` hỏng sẵn.
5. **Bảo mật/vận hành cần theo dõi:** grant `user` request-service tự cấp dưới danh nghĩa chủ cây (không tự thu hồi khi Request xong, audit ghi `granted_by` là chủ cây); task-service vẫn tin metadata `x-user-id`; token rỗng ở auth/mcp/project nay từ chối (cần `ensure-*-env.sh`).

## 6. Chưa được kiểm chứng (nói thẳng)

- `gitnexus impact` / `detect_changes`: **chưa chạy** (không có công cụ trong phiên). Quy tắc của repo bắt buộc trước khi merge; agent có công cụ nên chạy `detect_changes({scope:"compare", base_ref:"main"})` và ghi blast radius vào PR. Điểm chạm lớn: `common/grpcmw`, `registerRequestChannels`, `AnalysisRunner`, proto `ProjectService`/`AuthService`, `ProposeRequestClassification`, `StartExecution`/`completePhases`, guard `internalcaller` của auth/mcp. Blast radius đã rà tay bằng grep, ghi trong từng file task.
  - Đã thay bằng `codegraph` (gitnexus vẫn không dùng được): blast radius của cả nhánh so với `main`, test bị ảnh hưởng đã chạy và các điểm rủi ro nằm ở [`IMPACT-CODEGRAPH.md`](./IMPACT-CODEGRAPH.md).
- Đã kiểm lại ở đợt hoàn thiện: `buf breaking` xanh so với `main`; `buf lint` không tăng cảnh báo (307 có sẵn, giống `main`); `buf generate` cho mã khớp bản commit (chỉ khác header `protoc`). `buf` và CLI `migrate` cài vào thư mục scratchpad của phiên (`go install`), không nằm trong repo.
- Test tích hợp `project-service` Postgres chạy được: not-found PASS; **14 test worktree FAIL giống hệt trên `main`** (có sẵn).
- Lỗi có sẵn trên `main`, không do đợt này: `common/tracing` (`TestTraceEventSpanProcessor_OnEnd_ErrorStatusPublishesFailLevel`), test ssh agent/socket của `infra-fleet-service`, `git-gateway-service/localgit` (symlink `/private/var`), nhiều lỗi `tsc` trong `agent/`.
- Healthcheck compose: dương (`healthy` sau redeploy) đã thấy trên stack thật; âm (tắt token → unhealthy ~50 giây) chỉ có test đơn vị.
- Migration MySQL `workflow 0013`, `infra-fleet 0040` chỉ kiểm cú pháp/ngữ nghĩa nhẹ; `0093` (request) đã chạy up/down trên MySQL 8 thật.

### Commit đã bỏ qua hook pre-commit (có sự đồng ý của người dùng)

Commit `c57abd28c` dùng `--no-verify` vì hook `lint-staged` (oxlint + oxfmt) đỏ do lỗi **có sẵn ở HEAD** trong hai file agent: `agent/src/relay/ai-complete-handler.ts` và `agent-capability-report.ts` (vượt `max-lines` 300, biến thừa, kiểu `Array<T>`, `import()` type), cộng `oxfmt` bị SIGKILL trên máy này. Việc nên làm: tách hai file trên dưới 300 dòng (theo `AGENTS.md`, không tắt `max-lines`, không thêm vào baseline) rồi chạy lại hook.

## 7. Quy ước dự án cần giữ (AGENTS.md)

Comment ngắn giải thích "vì sao"; không đặt tên file chung chung (`helpers`, `utils`); không tắt `max-lines`; mọi thay đổi phải nghĩ tới SSH và đa nền tảng (macOS/Linux/Windows); proto chỉ cộng thêm; kiến trúc hexagonal (domain/usecase/adapter, cổng định nghĩa ở phía dùng). Tài liệu viết tiếng Việt.

## 8. Những gì cố ý KHÔNG nằm trong commit này

Thuộc một luồng việc khác (workflow-engine v7), để nguyên ở working tree của người chủ: `docs/crs/v7/workflow-engine/`, `specs/backend-go/crs/v7/workflow-engine/`, `tests/backend/`, `tests/client/`, `tests/e2e/`, `tests/playwright.workflow-web.config.ts`, thay đổi `package.json` (script `test:e2e:workflow-web`) và `tests/shared/orca_route_catalog.py`. Cũng không commit `agent/tsconfig.tsbuildinfo` (sản phẩm build).

Bẫy mới khi redeploy: cần `AUTH_DIRECTORY_CALLER_TOKEN` (cùng giá trị ở auth-service và request-service; `ensure-request-env.sh` sinh khi thiếu) và build/dựng lại `auth-service` trước `request-service` (`redeploy-local.sh auth-service request-service`).
