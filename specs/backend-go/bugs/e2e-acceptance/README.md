# Handoff + chỉ mục đợt triển khai: nghiệm thu e2e các luồng ORCA (Request flow CR v6)

Dành cho agent hoặc người nhận việc tiếp. Đọc hết file này trước khi sửa mã. Ngày chốt: 2026-10-09 (đã cập nhật sau đợt hoàn thiện cuối ngày). Nhánh: `feat/e2e-acceptance-fixes`.

## 1. Bối cảnh một đoạn

Mục tiêu của công việc: làm cho luồng Request (CR v6), workflow, task và dev-server/agent chạy ổn định trên stack Docker local, với máy Mac của người dùng làm dev server và **Claude Code đã đăng nhập** làm nguồn AI (không dùng khoá API). Quy trình đã áp dụng: viết bộ e2e nghiệm thu, chạy, thu lỗi, viết đánh giá + giải pháp + checklist, rồi sửa. Đã đi hai vòng (v1, v2).

**Kết quả cuối:** Claude thật: **103 PASS, 0 FAIL, 4 BLOCKED, 1 SKIP**; `run_acceptance.py --strict --ai-stub --allow-blocked MCP-01,MCP-02`: **110 PASS, 0 FAIL, 2 BLOCKED (MCP-01/02, vì `MCP_ENABLED` tắt), 1 SKIP (TSK-08 tuỳ chọn)**. Lúc đầu là 84 PASS, 3 FAIL. Với Claude thật `AIS-01/03` BLOCKED vì không dựng được đường "không có AI"; chế độ stub phủ chúng.

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

## 5. Việc còn lại, theo thứ tự nên làm

Đợt hoàn thiện cuối ngày đã xong: 016, 017, 020, 026, 028, 030, 035, V2-011, và các lỗi V2-018..021 (xem [v2/tasks/README.md](./v2/tasks/README.md)). Còn lại:

1. **Frontend + UI live: xong** (2026-10-10, `specs/frontend/bugs/e2e-acceptance-v1/tasks/README.md`): FE-E2E-001..009, spec Playwright live `run_ui_flow.py` (3 test qua UI thật: change_request, ask-back, question) xanh với AI stub và Claude thật. Còn `[~]`: FE-007 (chưa thử với task cũ sau backfill), kiểm trên Electron, và vitest/lint toàn frontend đỏ do lỗi có sẵn trên HEAD.
1b. **`ApprovalGates.CheckPlan` chưa được nối**: hàm chặn duyệt Plan khi còn giả định `needs_confirmation` chưa xác nhận không được handler duyệt Plan nào gọi (chỉ handler Solution nối `ApprovalGates`), nên `REQUEST_PLAN_UNCONFIRMED_ASSUMPTIONS` không bao giờ phát ra; frontend đã sẵn sàng hiển thị. Cần quyết định nghiệp vụ theo CR-REQ-028 mục `plan_assumption` (và đường "Hỏi lại" cho giả định Plan: `clarification.request source=plan_assumption`, `sourceRef`=id Plan, `questionKey=assumption:<id>`).
1c. **Sự kiện thời gian thực**: gateway không đẩy `solution.superseded` và `clarification.answered` qua `request.subscribe`, UI phải thăm dò (đã có, giới hạn). Nên đẩy sự kiện để giảm độ trễ và tải.
1d. **`consumeUntilDone` NAK không trễ** (`common/eventbus`): handler lỗi vĩnh viễn vẫn hot-loop; nên `NakWithDelay` có backoff. Phân tích chỉ đọc mới kiểm với stub, chưa với Claude thật (`ORCA_E2E_READONLY_ANALYSIS=1`); hotfix/security/performance chưa có e2e riêng.
2. **CI chạy thật (TASK-029, một phần)**: `.github/workflows/orca-flows-e2e.yml` và `backend-go-request-contracts.yml` đã viết, `actionlint` sạch, nhưng **chưa chạy trên runner GitHub**. Cần `workflow_dispatch` thử, rồi 5 đêm xanh liên tiếp trước khi thành cổng chặn. Chưa qua shellcheck.
3. **README triển khai từ máy sạch (TASK-022)**: `deploy/dev/README.md` đã viết lại và từng lệnh đã kiểm không phá stack, nhưng chưa ai dựng stack theo README từ máy sạch.
4. **Thực thi sau khi duyệt Plan với agent AI thật**: các test `PLN-*` cố ý *từ chối* Approval để không chạy agent AI trên máy dev. Cần một lượt kiểm tay có chủ đích (`ORCA_E2E_EXECUTE_AGENT=1` cho `TSK-08`) và `StartPhase` với Plan có Phase; hotfix `single_task` mới kiểm bằng stub AI.
5. **Điểm có chủ đích còn mở của TASK-017**: scm-integration trả 404 của API provider thành `Internal` (cần sentinel "upstream not found" xuyên 5 adapter); git-gateway gộp lỗi hạ tầng project-service thành `WORKTREE_REPO_NOT_FOUND` vì frontend dùng mã đó làm đường lùi mềm — chốt với frontend trước khi đổi.
6. **Script T2 hỏng sẵn**: `backend-go/ci/request-e2e/run-request-e2e.sh` gọi `tests/request/check_request_flow_types.py` đã bị xoá ở commit `8beb6703d`. `orca-flows` thay phần việc này; nên bỏ script và `request-e2e.yml` hoặc viết lại.
7. **Bật MCP**: `deploy/dev/.env` đang để trống `AUTH_MCP_PRINCIPAL_CALLER_TOKEN`, `MCP_INTERNAL_CALLER_TOKEN`, `OAUTH_INTERNAL_CALLER_TOKEN`; sau TASK-020 `auth-service`/`mcp-service` fail-closed khi token rỗng. Khi bật MCP (và để mở khoá `MCP-01/02`) phải chạy `deploy/dev/scripts/ensure-mcp-env.sh`. `code-intel-service` không lắp guard dù gateway gửi token (đã ghi, chưa sửa).
8. **Hotfix đã tạo trước bản sửa V2-011** đang kẹt ở `awaiting_plan_approval` không tự phục hồi: huỷ hoặc trả về backlog rồi mở lại.
9. **Metric `orca_request_orphan_total`** chưa thấy trên stack thật (chỉ test đơn vị); lô 100 dòng toàn lỗi vĩnh viễn của outbox thông báo chưa có metric đếm dòng bị giữ (chỉ log ERROR).

## 6. Chưa được kiểm chứng (nói thẳng)

- `gitnexus impact` / `detect_changes`: **chưa chạy** (không có công cụ trong phiên). Quy tắc của repo bắt buộc trước khi merge; agent có công cụ nên chạy `detect_changes({scope:"compare", base_ref:"main"})` và ghi blast radius vào PR. Điểm chạm lớn: `common/grpcmw`, `registerRequestChannels`, `AnalysisRunner`, proto `ProjectService`/`AuthService`, `ProposeRequestClassification`, `StartExecution`/`completePhases`, guard `internalcaller` của auth/mcp. Blast radius đã rà tay bằng grep, ghi trong từng file task.
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
