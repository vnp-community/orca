# Triển khai bản 2026-10: thực thi liên tục, màn hình Request mới, sửa lỗi QA 2026-10-16

**Cập nhật:** 2026-10-17 · Nhánh `feat/e2e-acceptance-fixes` · Đối chiếu với mã tại các commit
`09702be57` (CR-REQ-038 backend), `c05d46425` (CR-REQ-037 frontend), `cdf8fc008` (sửa lỗi QA 2026-10-16).

Tài liệu này là **bổ sung** cho [`README.md`](README.md) (luồng deploy chung, token nội bộ, Docker daemon). Nó chỉ nói về điều thay đổi trong bản này:
cần deploy service nào, biến cấu hình mới, cách bật từng tính năng, cách kiểm tra và cách quay lại. Bàn giao cho agent khác nằm ở
[`docs/handoff/2026-10-17-request-flow-and-qa-fixes.md`](../../docs/handoff/2026-10-17-request-flow-and-qa-fixes.md).

## 1. Bản này có gì

| Nhóm | Thay đổi | Ai thấy |
|---|---|---|
| Thực thi liên tục (CR-REQ-038) | Dự án chạy `execution_mode=continuous`: duyệt Plan **một lần** thì mọi Phase tự chạy. Mặc định vẫn `step_by_step` | Người duyệt Plan, admin dự án |
| Nhật ký hoạt động và tóm tắt thực thi (CR-REQ-038) | Kênh `request.activity`, `request.executionSummary`; trường `engine`, `executionMode` trên Request; `openSpecProfile` trên `request.flow` | Frontend, MCP, agent |
| Màn hình chi tiết Request mới (CR-REQ-037) | Dòng thời gian theo luồng của từng loại, thanh "việc tiếp theo", tab Execution, tab History thành Activity feed, Plan một thẻ duyệt khi chạy liên tục | Mọi người dùng |
| Ngôn ngữ đầu ra AI (CR-REQ-046) | Biến `REQUEST_DEFAULT_OUTPUT_LANGUAGE` (`vi` mặc định, `en`): phân tích và Plan mới ra tiếng Việt; giá trị sai làm `request-service` không khởi động (mục 4.1) | Mọi người dùng, ops |
| QA-01 Terminal | Terminal mở từ worktree mở đúng thư mục worktree, không còn `$HOME`; thư mục không dùng được thì báo `cwdFellBack` | Người dùng web |
| QA-02 Engine | `REQUEST_DEFAULT_ENGINE=openspec` chỉ chuyển `change_request` và `refactor`; `bug`, `task`... vẫn native | Người dùng, QA |
| QA-04 Jira | Jira Server/Data Center nhận email + PAT (Bearer, API v2); Cloud giữ Basic + API v3 | Người dùng tab Tasks |
| QA-05 Log agent | Dòng chẩn đoán `[claude-code:<mã>]` không còn hiện như lỗi | Người xem log agent |
| QA-06 Nhãn | Duyệt `task_list` giữ nhãn "Approve task list" | Người duyệt |
| Chưa làm | QA-03 nguồn task GitHub (hoãn theo yêu cầu): `github.listWorkItems` với `repo:"id:<repoId>"` vẫn lỗi | Xem mục 8 |

## 2. Cần deploy gì

| Thành phần | Có đổi? | Lý do | Cách |
|---|---|---|---|
| `request-service` | **có** | thực thi liên tục, activity, summary, mặc định engine, **migration 0098** | `redeploy-local.sh request-service` (áp migration trước khi restart) |
| `api-gateway` | **có** | kênh `request.activity`, `request.executionSummary`, `terminal.create` nhận `cwd` và trả `cwdFellBack`, MCP tools, OPA `agent_rpcs` | `redeploy-local.sh api-gateway` |
| `infra-fleet-service` | **có** | proto `infrafleet.proto` thêm `cwd_fell_back = 8`; truyền `cwd` xuống agent | `redeploy-local.sh infra-fleet-service` |
| `issue-tracking-service` | **có** | dò kiểu xác thực Jira, API v2 cho Server/DC | `redeploy-local.sh issue-tracking-service` |
| `frontend` | **có** | màn hình Request mới, 192 khoá i18n, terminal gửi `cwd`, nhãn, log agent | `BUILD_FRONTEND=1 redeploy-local.sh frontend api-gateway` |
| Dev Server Agent (`agent/out/agent.js`) | **có** | `pty.create` báo `cwdFellBack` | `node agent/build.mjs`, rồi restart agent **và** pty-daemon (mục 5) |
| `task-service`, `project-service`, các service còn lại | không | | |

Quy tắc sau mọi lần redeploy một service bị **tạo lại** (không chỉ restart): restart cả nhóm theo thứ tự
`infra-fleet → git-gateway → task → request → api-gateway`. Các service giữ địa chỉ container cũ; triệu chứng là `REQUEST_NOT_IMPLEMENTED` hoặc
`REQUEST_ENGINE_NO_CONNECTION`. `redeploy-local.sh` đã tự làm khi thấy "Recreate". Sau redeploy api-gateway có thể crash-loop (exit 137) khoảng một phút;
nếu kéo dài: `docker compose up -d --force-recreate api-gateway` (**không** làm vậy với `vault`, `postgres`, `nats`).

### Trình tự đề xuất (stack local)

```bash
cd /path/to/orca
bash deploy/dev/scripts/redeploy-local.sh infra-fleet-service issue-tracking-service request-service
BUILD_FRONTEND=1 bash deploy/dev/scripts/redeploy-local.sh frontend api-gateway
node agent/build.mjs && bash deploy/dev/scripts/start-local-agent.sh
```

Script cuối của `redeploy-local.sh` chạy suite `platform` làm smoke test. Nếu `PLT-08/09/10` đỏ với `orca-go-request` đang "Restarting", request-service chưa
sẵn sàng: xem `docker logs orca-go-request` (thường thiếu token nội bộ, xem `README.md` mục "Token nội bộ").

### Máy chủ từ xa

`bash deploy/dev/scripts/sync-to-server.sh <version>` build và đồng bộ cả 17 binary, áp migration và `up -d`. Agent trên dev server build bằng `node agent/build.mjs`, rồi
`scp agent/out/agent.js` hoặc dùng `deploy/agent/scripts/deploy-agent.sh`, theo [`deploy/agent/README.md`](../agent/README.md). Dev server dùng chung giữa nhiều người: báo trước vì restart pty-daemon làm mất mọi terminal đang mở.

## 3. Migration

| Số | Nội dung | Dialect |
|---|---|---|
| `0098_execution_mode` | `project_engine_settings.execution_mode`, `requests.execution_mode` (ghim một lần khi bắt đầu thực thi), `phase_starts.start_trigger` (`manual` hoặc `auto`) | Postgres và MySQL |

- Cột mới có giá trị mặc định `step_by_step`, nên dự án hiện có không đổi hành vi.
- `redeploy-local.sh` áp migration tự động (`SKIP_MIGRATE=1` để bỏ). Tay: `bash deploy/dev/scripts/migrate.sh request-service`.
- Cả hai dialect có file `.down.sql` (xem mục 7 trước khi dùng).
- Các migration của service khác không đổi trong bản này.

## 4. Cấu hình

### 4.1 Biến môi trường của `request-service` (`deploy/dev/.env`)

| Biến | Giá trị | Mặc định (compose) | Ý nghĩa |
|---|---|---|---|
| `REQUEST_DEFAULT_ENGINE` | `native` hoặc `openspec` | `openspec` | Engine của dự án **chưa lưu cài đặt**. `openspec` chỉ chuyển loại dùng profile `full` (`change_request`, `refactor`) |
| `REQUEST_DEFAULT_SOLUTION_AGENT` | `true` hoặc `false` | `true` | Solution do agent đọc knowledge graph viết |
| `REQUEST_DEFAULT_EXECUTION_MODE` | `step_by_step` hoặc `continuous` | `step_by_step` | Chế độ thực thi của dự án chưa lưu cài đặt |
| `REQUEST_DEFAULT_OUTPUT_LANGUAGE` | `vi` hoặc `en` | `vi` | Ngôn ngữ AI viết phân tích và Plan cho **cả triển khai** (CR-REQ-046). Giá trị khác làm `request-service` không khởi động (xem dưới) |
| `REQUEST_FLOW_ENABLED`, `REQUEST_APPROVAL_ENABLED` | `true` hoặc `false` | `false` | Hai cờ cũ, vẫn bắt buộc (xem `README.md`) |

Với ba biến `REQUEST_DEFAULT_ENGINE`, `REQUEST_DEFAULT_SOLUTION_AGENT`, `REQUEST_DEFAULT_EXECUTION_MODE`, giá trị sai không làm service chết: nó giữ mặc định an toàn và ghi cảnh báo lúc khởi động (`... is not native or openspec; native applies`).
**Ngoại lệ có chủ ý là `REQUEST_DEFAULT_OUTPUT_LANGUAGE`**: giá trị khác `vi` và `en` làm `request-service` thoát mã 1 trước khi mở kho dữ liệu, với thông báo
`REQUEST_DEFAULT_OUTPUT_LANGUAGE "fr" is not supported; use one of: vi, en` (xem bằng `docker compose logs request-service`), vì một lỗi gõ im lặng rơi về ngôn ngữ khác sẽ đổi ngôn ngữ của cả triển khai.
Cài đặt đã lưu cho một dự án **luôn thắng** biến môi trường (riêng ngôn ngữ đầu ra không có cài đặt theo dự án: biến này áp cho mọi dự án).

**Tác động của `REQUEST_DEFAULT_OUTPUT_LANGUAGE` (đổi mặc định của mọi triển khai sang tiếng Việt):**

- Sau lần deploy, phân tích (Solution, chẩn đoán, Findings, Answer) và Plan **mới** của mọi tenant ra tiếng Việt; tài liệu đã tạo giữ nguyên.
- Muốn tiếng Anh như trước: đặt `REQUEST_DEFAULT_OUTPUT_LANGUAGE=en`. Với `en`, chỉ thị tiếng Anh vẫn được gửi, nên AI viết tiếng Anh ngay cả khi Request bằng tiếng Việt (khác hành vi cũ là để AI tự theo ngôn ngữ của Request).
- Giao diện chưa có locale `vi`: phân tích tiếng Việt hiện dưới giao diện tiếng Anh sẽ trộn hai ngôn ngữ.
- Kỹ năng `tdd-openspec-author` có mục `Language` mới chỉ có hiệu lực sau khi hợp nhất vào nhánh mặc định của `vnp-community/orca` và chạy `npx skills update tdd-openspec-author --global` trên từng dev server; prompt vẫn mang chỉ thị ngôn ngữ trước đó.
- Quay lại: đặt `en` rồi `redeploy-local.sh request-service`.

Mặc định của deploy này (`docker-compose.yml` và `.env.example`) là **`openspec` và `true`**; chế độ thực thi vẫn `step_by_step`. Đây là mặc định của compose: binary
`request-service` chạy ngoài compose mà không có biến vẫn rơi về `native` và `false` (`EngineDefaultsFromEnv`). Muốn quay về native: đặt hai biến thành `native` và `false` trong `.env`
rồi `redeploy-local.sh request-service`. Hệ quả: dự án mới có repo thiếu `.tdd-agent/config.toml` sẽ lỗi `KNOWLEDGE_CONFIG_MISSING` cho `change_request`/`refactor`
và cho Solution do agent viết, cho đến khi dự án đó được đặt `native` (4.3).

### 4.2 Ý nghĩa của engine mặc định sau QA-02

Mỗi loại Request có một profile OpenSpec:

| Profile | Loại | Với `REQUEST_DEFAULT_ENGINE=openspec` | Dự án **tự chọn** `openspec` |
|---|---|---|---|
| `full` | `change_request`, `refactor` | chạy OpenSpec | chạy OpenSpec |
| `light` | `bug`, `security`, `performance` | **native** | chạy OpenSpec |
| `none` | mọi loại còn lại | native | native |

Mã: `EffectiveEngine` và cờ `FromDeploymentDefault` ở `backend-go/services/request-service/internal/domain/engine_settings.go`. Nghiệm thu: `QA-ENG-01..03`.

Hai ràng buộc của OpenSpec và Solution do agent viết: repo cần `.tdd-agent/config.toml` (knowledge hub); thiếu thì Plan lỗi `KNOWLEDGE_CONFIG_MISSING`, Request đứng ở
`planning`. Xem [`docs/guides/request/knowledge-hub-setup.md`](../../docs/guides/request/knowledge-hub-setup.md). Một lượt OpenSpec thật tốn khoảng 7 đến 13 USD.

### 4.3 Cài đặt theo dự án (ghi đè mặc định)

Chỉ admin. Chế độ thực thi đổi được trên giao diện: **Project Settings, tab General, mục "Request execution"** (hai lựa chọn "Ask before each step" và "Run all steps after one approval");
người không phải admin chỉ thấy giá trị ở dạng chỉ đọc. Engine và `solutionAgent` vẫn đặt bằng kênh WebSocket `request.engineSettingsSet` (kênh này vẫn dùng được cho cả chế độ thực thi):

```json
{ "projectId": "<id>", "engine": "native", "openspecMinVersion": "", "solutionAgent": false,
  "executionMode": "continuous", "expectedVersion": <version vừa đọc từ request.engineSettingsGet> }
```

CLI `orca engine show|set --project <id>` chỉ đặt `--engine`, `--solution-agent`, `--min-version`; **chưa có** cờ cho `executionMode`. Để trống `executionMode` khi `Set`
nghĩa là giữ giá trị đã lưu.

### 4.4 Chế độ thực thi liên tục: hành vi cần biết trước khi bật

- Bật theo dự án (4.3) hoặc theo triển khai (`REQUEST_DEFAULT_EXECUTION_MODE=continuous`). Chế độ **ghim** vào Request khi nó vào `executing`; đổi cài đặt sau đó không ảnh hưởng Request đang chạy.
- Mỗi Phase kế tiếp được khởi động **với tư cách người duyệt Plan**. Nếu người đó không có quyền chạy thực thi, việc duyệt bị từ chối với
  `REQUEST_EXECUTION_ACTOR_NOT_ALLOWED`; nếu sau này họ mất quyền, Request quay về backlog (`blocked_dependency`).
- Dừng lại và chờ người khi: có review lệch (drift) đang chờ, Request về backlog, `awaiting_information`, hoặc một task hết lượt thử.
- Giao diện chỉ có **Stop run**, không có Pause.

### 4.5 Jira Server/Data Center (BUG-QA-04)

Không có biến cấu hình mới cho service. Người dùng tự nhập trong hộp thoại Connect Jira (**email có thể bỏ trống với Server/DC**, PAT dùng một mình). Cách service chọn:

1. Dò `GET /rest/api/2/serverInfo` bằng `Authorization: Bearer <PAT>`. Thành công và `deploymentType` là Server hoặc Data Center: dùng Bearer và API v2.
2. Nếu bị từ chối và có email: thử Basic `email:token` trên `/rest/api/3/myself` (Cloud), dùng API v3.
3. Còn lại: lỗi nói rõ lý do, không rơi về API v3 và không để lại kết nối nửa chừng.

Kết quả dò được nhớ theo (địa chỉ site, băm token), lỗi chỉ nhớ tối đa 30 giây.

Kiểm tra từ `.env` (chỉ để **test API nội bộ**; luồng giao diện vẫn nhập tay):

```bash
bash deploy/dev/scripts/check-jira-from-env.sh          # đọc JR_URL/JR_EMAIL/JR_PAT_TOKEN hoặc JIRA_SITE_URL/JIRA_EMAIL/JIRA_PAT_TOKEN
bash deploy/dev/scripts/check-jira-from-env.sh --keep   # giữ kết nối sau khi kiểm
```

Script đăng nhập bằng tài khoản admin của stack, kết nối, liệt kê dự án và vài issue, rồi ngắt kết nối (trừ khi `--keep`). Token không bao giờ được in; thông báo lỗi bị che thành `***`.
Mã thoát: 0 ổn, 1 một bước lỗi, 2 thiếu cấu hình. Kết quả đã chạy với `https://jr.servicehub.vn`: 5 dự án, 5 issue giao cho người dùng của token.

### 4.6 Terminal (BUG-QA-01)

Không có biến cấu hình. Hợp đồng:

- `terminal.create` nhận `cwd` (đường dẫn tuyệt đối trên host của dev server). Không có `cwd` thì gateway lấy phần sau `::` của selector worktree.
- Kết quả có `cwdFellBack: true` khi thư mục yêu cầu không dùng được (rỗng, có byte NUL, không tồn tại, không phải thư mục) và agent mở `$HOME`. Giao diện in một dòng thông báo vào terminal.
- Proto: `infrafleet.proto` trường `cwd_fell_back = 8` (cộng thêm, tương thích ngược).
- Agent: `agent/src/relay/pty-agent-bridge.ts` (`handlePtyCreate`) chạy trong **pty-daemon**, tiến trình tách rời, sống qua việc restart agent.

### 4.7 Log agent (BUG-QA-05)

Dòng stderr dạng `[claude-code:<mã>] ...` hiện là dòng thông tin `ⓘ`. Chỉ hiện đỏ khi lượt chạy thất bại hoặc mã thuộc `auth_failed`, `authentication_failed`, `rate_limit`, `rate_limited`, `overloaded`.
Việc "unrecognized model" trong Claude Code CLI là cảnh báo của CLI, không làm hỏng lượt chạy.

## 5. Khởi động lại agent và pty-daemon

pty-daemon là tiến trình tách rời, sống qua việc restart agent. Vì vậy build agent mới **chưa** có tác dụng với `pty.create` cho đến khi daemon cũng được thay.
Với stack local, `start-local-agent.sh` chạy `pkill -f agent/out/agent.js`, mà dòng lệnh của cả agent lẫn daemon đều khớp; sau script có hai tiến trình mới:

```bash
node agent/build.mjs
bash deploy/dev/scripts/start-local-agent.sh
ps aux | grep "agent/out/agent.js" | grep -v grep      # kỳ vọng 2 tiến trình, pid mới
```

Mọi terminal đang mở bị mất khi daemon bị thay. Với dev server từ xa (systemd `orca-agent.service`): kiểm tra tay rằng daemon cũ đã dừng sau khi đổi `agent.js`
(chưa có bước tự động trong `deploy/agent/`).

## 6. Kiểm tra sau triển khai

Chạy lần lượt; mỗi lệnh từ thư mục gốc repo. **Không sửa gì trong repo khi chạy `run_ui_flow.py --real`**: nó coi mọi thay đổi trong checkout là rò rỉ.

| # | Lệnh | Kỳ vọng | Chứng minh |
|---|---|---|---|
| 1 | `python3 tests/stages/e2e/orca-flows/run_acceptance.py platform` | tất cả ok | stack sống, request-service nối được |
| 2 | `python3 tests/stages/e2e/orca-flows/run_acceptance.py devserver qa` | 17 pass, 2 fail (`QA-GH-01`, `QA-GH-03`), 1 skip (`QA-JIRA-LIVE-01`) | QA-01, 02, 04 |
| 3 | `ORCA_E2E_JIRA_LIVE=1 python3 tests/stages/e2e/orca-flows/run_acceptance.py qa` | `QA-JIRA-LIVE-01` ok (cần `JR_*` hoặc `JIRA_*` trong `.env`) | Jira site thật |
| 4 | `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- qa-bug-task-native-flow` | 3 pass | bug, task native, mặc định engine |
| 5 | `python3 tests/stages/e2e/orca-flows/run_ui_flow.py -- qa-regressions -g TERM-01` | 1 pass | terminal worktree. **Không** dùng `--ai-stub`: agent giả không spawn được PTY |
| 6 | `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real native-execute` | 2 pass (khoảng 6 phút, Claude Code thật, repo tạm) | thực thi bug, task native thật; log agent không hiện chẩn đoán như lỗi |
| 7 | `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real continuous` | pass | duyệt Plan một lần chạy hết Phase, tab Execution theo dõi được |

Lệnh 2 vẫn có 2 fail GitHub cho đến khi QA-03 được làm (mục 8). Không có OpenSpec thật trong danh sách: lượt chạy trên `vnp-qa-platform` bị hoãn, tốn 7 đến 13 USD.

Kiểm tay nhanh trên giao diện (tài khoản admin):

1. Sidebar có dòng **Requests**. Mở một Request: dòng thời gian khớp loại (bug, task không có bước Handover; chỉ OpenSpec có Handover).
2. Mở terminal từ một worktree phụ trong sidebar, gõ `pwd`: ra thư mục worktree.
3. Tab Tasks, Jira: Connect Jira bằng email + PAT, thấy danh sách issue.
4. Ở Plan của một `change_request` (dự án `step_by_step`), bấm **"Approve and run all steps"** và xác nhận (hoặc đặt dự án `continuous` ở Project Settings, 4.3): tab Execution chạy hết và ghi "Runs all steps on its own", không phải duyệt từng Phase.

## 7. Quay lại

- **Ngôn ngữ đầu ra AI**: đặt `REQUEST_DEFAULT_OUTPUT_LANGUAGE=en` rồi `redeploy-local.sh request-service`; chỉ ảnh hưởng phân tích và Plan tạo sau đó.
- **Mặc định mới của bản này an toàn**: `step_by_step` và `native` khi không có biến. Để vô hiệu thực thi liên tục cho mọi dự án mới: đặt `REQUEST_DEFAULT_EXECUTION_MODE=step_by_step`; dự án đã lưu `continuous` cần đặt lại bằng `request.engineSettingsSet`.
- **Binary**: build lại bản cũ từ commit trước (trong một worktree riêng ở commit `46426bd95`) rồi `redeploy-local.sh`. Không cần xoá cột 0098; mã cũ bỏ qua chúng.
- **Migration 0098**: không cần chạy `.down` để quay lại binary cũ. Chỉ chạy `.down` (cả hai dialect đều có) khi chắc không còn Request `continuous` đang chạy, vì nó xoá cột ghim chế độ.
- **Frontend**: build lại từ commit cũ. Ngược lại, frontend mới chạy trên gateway cũ vẫn dùng được: khi không có `request.activity`, tab History hiện danh sách đổi loại.
- **Agent**: `agent.js` cũ vẫn chạy; terminal chỉ mất thông báo `cwdFellBack`.

## 8. Còn lại và giới hạn đã biết

- **QA-03 GitHub (chưa sửa)**: `github.listWorkItems` với `repo:"id:<repoId>"` trả `WORKTREE_REPO_NOT_FOUND` vì gateway chuyển nguyên tiền tố `id:`; không có thông tin đăng nhập thì lỗi bị gán `not_found` thay vì `auth_required`; dải "Some GitHub source hosts unavailable" hiện cả khi host đang nối.
  Phương án: [`specs/frontend/bugs/qa-2026-10-16/solutions/SOL-QA-03-github-task-source.md`](../../specs/frontend/bugs/qa-2026-10-16/solutions/SOL-QA-03-github-task-source.md). Test đỏ chờ sẵn: `QA-GH-01`, `QA-GH-03`, `QA-UI-GH-01`.
- Các bản dịch `es`, `ja`, `ko`, `zh` của 192 khoá mới chưa được người bản ngữ rà.
- Màn hình Project Settings chỉ có chế độ thực thi (4.3); engine và `solutionAgent` chưa có màn hình, cài bằng kênh WebSocket.
- `.env.example` chưa liệt kê khối `JIRA_*`/`JR_*` (mục 4.5).
- Một số lỗi có sẵn từ trước, không do bản này: test `infra-fleet` về đường dẫn socket SSH identity-agent quá dài, vài test frontend, và lỗi kiểu trong `agent/src/relay/pty-agent-bridge.test.ts` (`env` trên `{}`).
