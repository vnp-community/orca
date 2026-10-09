# Handoff + chỉ mục đợt triển khai: nghiệm thu e2e các luồng ORCA (Request flow CR v6)

Dành cho agent hoặc người nhận việc tiếp. Đọc hết file này trước khi sửa mã. Ngày chốt: 2026-10-09. Nhánh: `feat/e2e-acceptance-fixes` (tách từ `main`, một commit).

## 1. Bối cảnh một đoạn

Mục tiêu của công việc: làm cho luồng Request (CR v6), workflow, task và dev-server/agent chạy ổn định trên stack Docker local, với máy Mac của người dùng làm dev server và **Claude Code đã đăng nhập** làm nguồn AI (không dùng khoá API). Quy trình đã áp dụng: viết bộ e2e nghiệm thu, chạy, thu lỗi, viết đánh giá + giải pháp + checklist, rồi sửa. Đã đi hai vòng (v1, v2).

**Kết quả cuối:** `python3 tests/stages/e2e/orca-flows/run_acceptance.py` cho **103 PASS, 0 FAIL, 4 BLOCKED, 1 SKIP** (lúc đầu 84 PASS, 3 FAIL). BLOCKED/SKIP không phải lỗi sản phẩm: MCP tắt (`MCP-01/02`), đường "không có AI" không dựng được khi dev server có Claude (`AIS-01/03`), `TSK-08` tuỳ chọn.

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

1. **Frontend (8 task, 0 làm)**: `specs/frontend/bugs/e2e-acceptance-v1/tasks/`. Backend đã sẵn sàng (xem ghi chú đầu `README.md` ở đó). Bắt đầu từ FE-E2E-001 (test hợp đồng đọc `channel_inventory.json`).
2. **CI** (029, 035, 026): chưa có workflow nào cho `orca-flows`, `check-agent-methods-called.sh`, `check-grpc-dials.sh` (đã có target Make `check-agent-methods-called`, `check-grpc-dials`, chưa gọi trong `.github/workflows`). TASK-035 cần stub `claude` xác định để e2e chạy được khi không có tài khoản Claude; sau đó mở khoá `AIS-01/03` và chạy `--strict`.
3. **Hotfix `single_task` (TASK-V2-011)**: hotfix chưa có Plan (task sửa phải được tạo trong `analyzing`, trong luồng ghi kết quả chẩn đoán, cổng `pre_deploy`). Xem `SOL-V2-001` mục 3.
4. **Stage e2e Go còn skip (TASK-E2E-028)**: giờ Plan đã có nên viết được các stage Solution/Plan/CommitPlan/ReportTaskOutcome trong `backend-go/services/request-service/e2e`.
5. **Phần một phần**: 016 (metric `orca_request_orphan_total` + runbook), 017 (rà mọi repo `Delete/Update/Get` tìm lỗi "not found" không dùng sentinel), 020 (healthcheck compose, kiểm OPA bundle/DB trong `/readyz`, rà guard rỗng ở service khác), 022 và 030 (cập nhật `deploy/dev/README.md`, `docs/guides/request/*`, ghi `DOCKER_HOST` Colima, bảng trạng thái giai đoạn).
6. **Thực thi sau khi duyệt Plan chưa được kiểm trên stack thật**: các test `PLN-*` cố ý *từ chối* Approval để không chạy agent AI trên máy dev. Nên có một lượt kiểm tay có chủ đích (`ORCA_E2E_EXECUTE_AGENT=1` cho `TSK-08`) và kiểm `StartPhase` với Plan có Phase.
7. **Dọn dữ liệu local**: ~27 Request mở trỏ project đã xoá (từ các lượt thử cũ). Người dùng chưa quyết định; chạy `python3 tests/stages/e2e/orca-flows/report_orphan_requests.py` để xem, `--cancel` chỉ khi được phép.

## 6. Chưa được kiểm chứng (nói thẳng)

- `gitnexus impact` / `detect_changes`: **chưa chạy** (không có công cụ trong phiên). Quy tắc của repo bắt buộc trước khi merge; agent nhận việc có công cụ nên chạy `detect_changes({scope:"compare", base_ref:"main"})` và ghi blast radius vào PR. Điểm chạm lớn: `common/grpcmw` (mọi service), `registerRequestChannels`, `AnalysisRunner`, `ProjectService` proto.
- `buf lint` / `buf breaking`: chưa chạy (máy không có `buf`). Proto chỉ cộng thêm (RPC `ResolveProjectAccess`, trường `ClarificationQuestion.value_type`); mã sinh bằng `protoc` + `protoc-gen-go v1.36.11` / `protoc-gen-go-grpc 1.6.1` nên header khác vài file cũ (v1.36.12). Nên sinh lại bằng `buf generate` khi có.
- Test tích hợp `project-service` Postgres (`TestRepository_MalformedIDIsNotFound`) biên dịch được nhưng **chưa chạy** (thiếu CLI `migrate`); đã kiểm gián tiếp bằng e2e `NFC-03/08`. Test tích hợp request-service (Postgres + MySQL) đã chạy.
- Migration MySQL `workflow 0013`, `infra-fleet 0040` chỉ kiểm cú pháp/ngữ nghĩa nhẹ; `0093` (request) đã chạy up/down trên MySQL 8 thật và qua hợp đồng cả hai dialect.
- Lỗi có sẵn, không do thay đổi này: `common/tracing` (`TestTraceEventSpanProcessor_OnEnd_ErrorStatusPublishesFailLevel`), hai test socket unix của infra-fleet (đường dẫn quá dài trên macOS), nhiều lỗi `tsc` trong `agent/` (kiểm bằng `grep` đã chắc không thuộc file mới).

### Commit đã bỏ qua hook pre-commit (có sự đồng ý của người dùng)

Commit dùng `--no-verify` vì hook `lint-staged` (oxlint + oxfmt) đỏ do lỗi **có sẵn ở HEAD** trong hai file agent: `agent/src/relay/ai-complete-handler.ts` và `agent-capability-report.ts` (vượt `max-lines` 300, biến thừa `now`/`memInfo`, kiểu `Array<T>`/`ReadonlyArray<T>`, `import()` type), cộng `oxfmt` bị SIGKILL trên máy này. Lỗi do mã mới đã được sửa (oxlint sạch trên các file mới). Việc nên làm: tách hai file trên dưới 300 dòng (theo `AGENTS.md`, không tắt `max-lines`, không thêm vào baseline) rồi chạy lại hook.

## 7. Quy ước dự án cần giữ (AGENTS.md)

Comment ngắn giải thích "vì sao"; không đặt tên file chung chung (`helpers`, `utils`); không tắt `max-lines`; mọi thay đổi phải nghĩ tới SSH và đa nền tảng (macOS/Linux/Windows); proto chỉ cộng thêm; kiến trúc hexagonal (domain/usecase/adapter, cổng định nghĩa ở phía dùng). Tài liệu viết tiếng Việt.

## 8. Những gì cố ý KHÔNG nằm trong commit này

Thuộc một luồng việc khác (workflow-engine v7), để nguyên ở working tree của người chủ: `docs/crs/v7/workflow-engine/`, `specs/backend-go/crs/v7/workflow-engine/`, `tests/backend/`, `tests/client/`, `tests/e2e/`, `tests/playwright.workflow-web.config.ts`, thay đổi `package.json` (script `test:e2e:workflow-web`) và `tests/shared/orca_route_catalog.py`. Cũng không commit `agent/tsconfig.tsbuildinfo` (sản phẩm build).
