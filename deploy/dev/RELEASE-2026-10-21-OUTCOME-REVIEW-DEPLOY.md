# Triển khai bản 2026-10-21: Outcome Review đầy đủ, Plan tự sinh, chọn nhánh gốc, duyệt bước và chạy tất cả

**Cập nhật:** 2026-10-21 · Nhánh `release/outcome-review-2026-10-21` tại commit `426893d40` (cùng commit với `feat/e2e-acceptance-fixes`) · `package.json` ghi `1.4.138-rc.6` (chưa đổi số phiên bản).

Tài liệu này là **bổ sung** cho [`README.md`](README.md) (luồng deploy chung, token nội bộ, Docker daemon) và cho
[`RELEASE-2026-10-DEPLOY.md`](RELEASE-2026-10-DEPLOY.md) (bản 2026-10-17). Nó nói về những gì **thay đổi từ commit `cdf8fc008`** (mốc của tài liệu kia) đến commit trên:
cần deploy gì, cấu hình nào phải bật để dùng đủ chức năng, cách kiểm tra, cách quay lại, và **giới hạn đã biết** (mục 9). Mọi lệnh chạy từ thư mục gốc repo.

## 0. Danh sách kiểm "đủ chức năng" (đọc mục này trước)

Một người triển khai chỉ cần đi hết danh sách này; các mục sau giải thích từng dòng.

| # | Việc | Ở đâu | Kiểm |
|---|---|---|---|
| 1 | Deploy các thành phần ở mục 2 và áp **13 migration** (mục 3) | server | `run_acceptance.py platform` xanh |
| 2 | Request flow bật: `REQUEST_FLOW_ENABLED=true` và công tắc theo tenant | `.env`, kênh tenant | tab Requests có trong sidebar |
| 3 | Token nội bộ đã sinh (`ensure-request-env.sh`) | `.env` | `orca-go-request` healthy |
| 4 | Engine OpenSpec và Solution do agent viết: `REQUEST_DEFAULT_ENGINE=openspec`, `REQUEST_DEFAULT_SOLUTION_AGENT=true` (mặc định của compose) | `.env` | `orca engine show --project <id> --check` |
| 5 | **Bật Outcome Review**: `REQUEST_DEFAULT_OUTCOME_REVIEW=manual` (mặc định `off`) hoặc `orca engine set --outcome-review manual` theo project | `.env` hoặc CLI | tab **Outcome review** hiện trong Request |
| 6 | Vẽ hình: `REQUEST_REVIEW_DRAW_ENABLED=true`, `REQUEST_REVIEW_DRAW_ENGINE=auto` (mặc định) | `.env` | Architecture/Flow có hình |
| 7 | Dev server: agent mới, `tdd-agent` trong `PATH`, `openspec`, Python 3, Claude Code đã đăng nhập | dev server | `--check` không báo `blocked` |
| 8 | Dev server: 4 skill tri thức **và** skill `diagram-design` (mục 4.4) | dev server | `--check` không báo `SKILL_MISSING` |
| 9 | Mỗi project: `.tdd-agent/config.toml` và đồ thị tri thức đã `init` | repo của project | review thiết kế ra `kg.snapshot` khác rỗng |
| 10 | `AGENT_WORK_DIR` chứa repo **và** các worktree nằm cạnh nó | dev server | lượt Plan/Task ghi được file |
| 11 | CLI `orca` build lại từ mã mới (bản cũ **không có** `--outcome-review`) | máy quản trị | `orca engine set --help` có `--outcome-review` |

## 1. Bản này có gì

| Nhóm | Thay đổi | Ai thấy |
|---|---|---|
| **Outcome Review** (CR-OR-001..030) | Review cấp cao của một Request: Architecture, Flow, Behavior, Data, Contracts, Requirements, Config; hai pha (`design` trước code, `delivered` sau code); quyết định từng mục, gửi lại thành Request con, duyệt; hình do skill `diagram-design` hoặc engine dựng sẵn của Orca vẽ | Người duyệt, admin |
| Vẽ hình bằng skill bên thứ ba | Skill `diagram-design` (MIT, `cathrynlavery/diagram-design`) được vendor vào `skills/diagram-design/` và là **mặc định** (`REQUEST_REVIEW_DRAW_ENGINE=auto`); không có skill thì Orca tự vẽ | Admin, người duyệt |
| Chặn duyệt review hỏng | Review mà lượt agent mới nhất lỗi **không Approve được** (`REQUEST_REVIEW_CONTEXT_FAILED`); log lý do từ chối kết quả agent | Người duyệt, vận hành |
| Chọn nhánh gốc (CR-REQ-039) | Worktree proposal và task cắt từ **nhánh người dùng chọn**; `repo.listBranches` liệt kê nhánh cục bộ và chỉ-có-trên-remote | Người tạo Request |
| Phiên bản phân tích (CR-REQ-041) | Phân tích có phiên bản; gửi lại bằng danh sách yêu cầu; xem bản cũ ở chế độ chỉ đọc | Người duyệt Analysis |
| Plan tự sinh, tài liệu và phiên bản Plan | Sau khi duyệt Analysis, Plan OpenSpec tự sinh; có phiên bản và yêu cầu sửa; **"Approve and run all steps"** | Người duyệt Plan |
| Duyệt bước (workflow) | `workflow-service` có bảng duyệt bước (migration `0014_step_approvals`) | Người dùng workflow |
| Prompt catalog và relay dùng chung (CR-REQ-042) | `backend-go/prompts/*` và `common/agentrelay`; `agent.execPrompt` nhận prompt qua stdin | Mọi lượt AI |
| Sửa lỗi | Hotfix chặn sửa Plan sai phiên bản; subject outbox cho grant task; consumer Plan tự sinh bỏ qua sự kiện phát lại; terminal preview luôn UTF-8; `BaseRefDefault` xử lý tên có `/` và repo không có remote | Mọi người |

## 2. Cần deploy gì

| Thành phần | Có đổi? | Lý do | Cách |
|---|---|---|---|
| `request-service` | **có** | Outcome Review, Plan tự sinh, base ref, phiên bản, **migration 0099–0109** | `redeploy-local.sh request-service` (áp migration trước khi restart) |
| `api-gateway` | **có** | kênh `request.review.*`, `repo.listBranches`, MCP tool `request_review_*`, `request.event` (`review.changed|progress`), OPA `agent_rpcs` | `redeploy-local.sh api-gateway` |
| `task-service` | **có** | nhánh gốc đến worktree task, subject outbox, **migration 0022** | `redeploy-local.sh task-service` |
| `git-gateway-service` | **có** | `repo.listBranches`, base ref, `git.baseRefDefault` | `redeploy-local.sh git-gateway-service` |
| `workflow-service` | **có** | duyệt bước, **migration 0014**; bản trước của nhánh này **không biên dịch**, đã sửa ở `4aaab4218` | `redeploy-local.sh workflow-service` |
| `infra-fleet-service`, `orchestration-service` | **có** | prompt catalog, relay dùng chung; `infra-fleet`: preview terminal UTF-8 | `redeploy-local.sh infra-fleet-service orchestration-service` |
| `frontend` | **có** | tab Outcome review, chọn nhánh gốc, phiên bản Analysis/Plan, "Approve and run all steps", khoá i18n mới (5 ngôn ngữ) | `BUILD_FRONTEND=1 redeploy-local.sh frontend api-gateway` |
| Dev Server Agent (`agent/`) | **có** | `agent.execPrompt` qua stdin, khả năng `readonly`/`knowledgeHub`, `BaseRefDefault` | `node agent/build.mjs`, rồi restart agent **và** pty-daemon (mục 6) |
| CLI `orca` | **có** | `engine set --outcome-review`, `--plan-auto-generate`, `engine show --check` nêu lỗi review | `cd backend-go/cmd/orca-cli && go build -o orca .` rồi đặt vào `PATH` |
| Gói chính sách OPA | **có** (`request.rego`) | RPC review, quyền agent | `build-local.sh` tự `rsync` `backend-go/policy/orca-authz` vào `deploy/dev/policy/orca-authz` |

Quy tắc sau mọi lần redeploy một service bị **tạo lại**: restart nhóm theo thứ tự `infra-fleet → git-gateway → task → request → api-gateway` (xem `RELEASE-2026-10-DEPLOY.md` mục 2). `redeploy-local.sh` tự làm khi thấy "Recreate". Triệu chứng nếu quên: `REQUEST_NOT_IMPLEMENTED` từ gateway hoặc `REQUEST_ENGINE_NO_CONNECTION` từ một lượt chạy. Sau redeploy `api-gateway` có thể crash-loop (exit 137) khoảng một phút; nếu kéo dài: `docker compose up -d --force-recreate api-gateway` (**không** làm vậy với `vault`, `postgres`, `nats`).

## 3. Migration

| Service | Số | Nội dung |
|---|---|---|
| request | `0099_request_base_ref` | nhánh gốc của Request (đóng băng khi có worktree) |
| request | `0100_analysis_runs_revision_index` | chỉ mục phiên bản phân tích |
| request | `0101_plan_auto_generation` | công việc sinh Plan tự động |
| request | `0102_openspec_change_redaction` | khử bí mật của change OpenSpec |
| request | `0103_openspec_change_revisions` | phiên bản của change OpenSpec |
| request | `0104_plan_auto_jobs_revision_origin` | nguồn của phiên bản trong công việc tự sinh |
| request | `0105_outcome_reviews` | bảng review và lượt chạy |
| request | `0106_project_outcome_review` | cài đặt review theo project |
| request | `0107_outcome_review_contexts` | kết quả context và thiết kế đã khoá |
| request | `0108_review_artifacts` | kho hình (blob theo digest) |
| request | `0109_review_decisions_profiles` | quyết định theo mục, hồ sơ vẽ |
| task | `0022_outbox_grant_subjects` | subject outbox cho grant task |
| workflow | `0014_step_approvals` | duyệt bước |

- Postgres và MySQL đều có `.up.sql` và `.down.sql`. Postgres bật RLS cho các bảng review (`app.tenant_id`).
- `redeploy-local.sh` áp migration của service được redeploy (`SKIP_MIGRATE=1` để bỏ). Tay: `bash deploy/dev/scripts/migrate.sh request` (cũng `task`, `workflow`).
- Cột/bảng mới đều tùy chọn hoặc có mặc định, nên dữ liệu hiện có không đổi hành vi.

## 4. Cấu hình

### 4.1 Nền tảng (nếu stack chưa bật request flow)

| Việc | Cách |
|---|---|
| Token nội bộ giữa các service | `bash deploy/dev/scripts/ensure-request-env.sh` (chỉ điền chỗ trống, không in giá trị). Thiếu token: `orca-go-request` unhealthy sau khoảng 50 giây |
| Cờ toàn cục | `REQUEST_FLOW_ENABLED=true` (mặc định `false`); `REQUEST_APPROVAL_ENABLED` giữ `false` trừ khi đã có handler duyệt |
| Công tắc theo tenant | `SetRequestFlowSettings` ([`docs/guides/request/admin-enable-request-flow.md`](../../docs/guides/request/admin-enable-request-flow.md)); cả hai cờ phải bật |

### 4.2 Biến môi trường của `request-service` (`deploy/dev/.env`)

Đọc **một lần lúc khởi động**: đổi xong chạy `redeploy-local.sh request-service`. Biến Outcome Review sai giá trị không làm service chết (giữ mặc định, ghi cảnh báo); riêng `REQUEST_DEFAULT_OUTPUT_LANGUAGE` sai thì service thoát ngay.

Giá trị khuyến nghị cho **đầy đủ chức năng**:

| Biến | Khuyến nghị | Mặc định compose | Ý nghĩa |
|---|---|---|---|
| `REQUEST_FLOW_ENABLED` | `true` | `false` | bật request flow |
| `REQUEST_DEFAULT_ENGINE` | `openspec` | `openspec` | engine OpenSpec cho `change_request` và `refactor` |
| `REQUEST_DEFAULT_SOLUTION_AGENT` | `true` | `true` | Solution do agent đọc KG viết |
| `REQUEST_DEFAULT_EXECUTION_MODE` | `step_by_step` hoặc `continuous` | `step_by_step` | `continuous` = duyệt Plan một lần chạy mọi Phase |
| `REQUEST_DEFAULT_OUTPUT_LANGUAGE` | `vi` hoặc `en` | `vi` | ngôn ngữ AI viết phân tích, Plan **và review** |
| **`REQUEST_DEFAULT_OUTCOME_REVIEW`** | **`manual`** | `off` | cho project chưa lưu lựa chọn; `off` ẩn tab. Lựa chọn đã lưu của project luôn thắng |
| `REQUEST_REVIEW_KILL_SWITCH` | `false` | `false` | `true` ẩn tab và từ chối mọi RPC review (`REQUEST_REVIEW_DISABLED`); **không xoá dữ liệu** |
| `REQUEST_REVIEW_DRAW_ENABLED` | `true` | `true` | `false` = chỉ chữ và bảng, không bao giờ chạy lượt vẽ |
| `REQUEST_REVIEW_DRAW_ENGINE` | `auto` | `auto` | `auto`: skill `diagram-design` khi dev server có, không thì Orca tự vẽ; `agent`: chỉ skill (không có skill = không vẽ); `builtin`: chỉ Orca vẽ (miễn phí, tức thì, không cần dev server) |
| `REQUEST_REVIEW_DRAW_SKILL_SHA` | để trống | trống | 16 ký tự đầu sha256 `SKILL.md`. Trống = bản đã vendor (`1cad611bfe734dc9`, khớp `skills/diagram-design/SKILL.md`); `any` = chấp nhận bản đang cài |

Giới hạn tùy chọn (để trống = giữ giá trị dựng sẵn; ngoài khoảng thì giữ mặc định và ghi cảnh báo):

| Biến | Mặc định | Khoảng | Ý nghĩa |
|---|---|---|---|
| `REQUEST_REVIEW_CONTEXT_TTL` | 168h | 1h–2160h | thời gian dùng lại một kết quả context đã kiểm và thiết kế đã khoá |
| `REQUEST_REVIEW_CONTEXT_TIMEOUT` | 10m | 1m–15m | một lượt context (agent tự dừng ở 15 phút) |
| `REQUEST_REVIEW_CONTEXT_MAX_FILES` | 60 | 5–400 | số tệp agent được mở trong một lượt |
| `REQUEST_REVIEW_ARTIFACT_RETENTION` | 2160h | 24h–5 năm | giữ hình không còn hiệu lực |
| `REQUEST_REVIEW_ARTIFACT_QUOTA_MB` | 512 | 1–100000 | dung lượng hình của một tenant |
| `REQUEST_REVIEW_REGEN_PER_DAY` | 5 | 1–1000 | lần sinh lại mỗi Request mỗi ngày |
| `REQUEST_REVIEW_CONFIRM_COST` | không đặt | 0–10000 | USD ước tính trên đó sinh lại phải xác nhận; không đặt = không hỏi |

Biến **không nằm trong compose** nhưng có tác dụng (đọc trong binary, mặc định như ghi): `REQUEST_PLAN_AUTO_GENERATE` (`true`: Plan tự sinh sau khi duyệt Analysis), `REQUEST_CLARIFICATION_WORKERS_ENABLED` (`true`), `REQUEST_SKILLS_REPOSITORY_URL` (rỗng). Muốn tắt Plan tự sinh toàn bộ: thêm biến vào `environment:` của `request-service` trong compose rồi redeploy.

### 4.3 Cài đặt theo project (ghi đè mặc định, chỉ admin)

```bash
orca engine show --project <id> --check
orca engine set  --project <id> --engine openspec --solution-agent=true --outcome-review manual
orca engine set  --project <id> --plan-auto-generate=true              # Plan tự sinh theo project
orca engine set  --project <id> --min-version 1.2.0                    # bản openspec tối thiểu (tùy chọn)
```

- `--outcome-review`: `off`, `manual`, hoặc `default` (theo `REQUEST_DEFAULT_OUTCOME_REVIEW`). Không có chế độ tự chạy: mỗi review do người dùng bấm, vì tốn một lượt agent.
- Chế độ thực thi đổi trên giao diện: **Project Settings, tab General, mục "Request execution"**. Màn hình Project Settings **chưa có** công tắc Outcome Review; bật bằng CLI hoặc biến môi trường.
- `orca engine show --check` hỏi lại dev server và nêu **vì sao** review không chạy được (agent quá cũ, thiếu skill, không có KG). Lưu cài đặt không bị chặn bởi kết quả kiểm.
- Một Request chỉ review được từ `awaiting_plan_approval` trở đi; trước đó `REQUEST_REVIEW_NOT_READY`.

### 4.4 Dev server (máy chạy agent)

| Thành phần | Cài/kiểm | Dùng cho |
|---|---|---|
| Agent Orca mới | `node agent/build.mjs && bash deploy/dev/scripts/start-local-agent.sh` (mục 6) | báo `agent.execPrompt.readonly` và `agent.execPrompt.knowledgeHub`; nhận prompt qua stdin |
| Claude Code CLI đã đăng nhập | `claude --version`; mã lỗi `REQUEST_ENGINE_CLAUDE_NOT_AUTHENTICATED` nếu chưa | mọi lượt AI |
| `openspec` CLI | `openspec --version` | engine OpenSpec |
| `tdd-agent` trong `PATH` | `command -v tdd-agent` | đọc KG |
| Python 3 | `python3 --version` | agent chạy `self_check.py` của `diagram-design` |
| 4 skill tri thức | `npx skills add https://github.com/vnp-community/orca --skill tdd-knowledge tdd-openspec-author tdd-knowledge-writeback tdd-orca-task --global` | lượt context của review dùng riêng `tdd-knowledge` (chỉ công cụ đọc); `tdd-knowledge-writeback` **không** được review dùng |
| Skill `diagram-design` | `mkdir -p ~/.claude/skills && cp -R skills/diagram-design ~/.claude/skills/` | lượt vẽ khi engine là `auto`/`agent` |
| `AGENT_WORK_DIR` | phải chứa repo **và** các worktree nằm cạnh (`<repo>-task-<id>`, `<repo>-request-<n>-proposal`); thiếu thì ghi file lỗi "Path outside project root" | Plan và Task |

Skill `diagram-design` đã vendor kèm `LICENSE`, `NOTICE` (nêu rõ các chỗ đã sửa) và `MANIFEST.sha256`. Nguồn upstream: commit `f2f4d49ea8d632f17cf196ba26599277a93f767b`. Không cài skill thì `auto` tự vẽ bằng engine dựng sẵn: review vẫn có hình, kém tinh chỉnh hơn. Agent báo skill đã cài khi kết nối lại; hồ sơ khả năng của agent hạn một ngày, nên sau khi cài skill hãy `orca engine show --check` (bỏ qua cache).

### 4.5 Mỗi project cần gì để review thiết kế có nội dung

- Repo có **`.tdd-agent/config.toml`** và đồ thị tri thức đã dựng:
  ```bash
  cd <repo> && tdd-agent init --urd docs/URD.md --srs docs/SRS.md --sad docs/SAD.md
  ```
  Orca đọc KG của **repo chính** của project (không phải worktree task) và chỉ kiểm tệp có mặt; agent mới truy vấn KG.
- Thiếu KG: review vẫn có dữ kiện Orca đo và nói `kg_unavailable`; **pha `design` không chạy được** (cần KG).
- Muốn review phản ánh tài liệu thiết kế mới: chạy lại `tdd-agent init`, rồi **Regenerate** (xem giới hạn ở mục 9: regenerate hiện đọc lại cả KG).

## 5. Trình tự triển khai (stack local)

```bash
cd /path/to/orca
git checkout release/outcome-review-2026-10-21

# 1) token và cấu hình (một lần)
bash deploy/dev/scripts/ensure-request-env.sh
$EDITOR deploy/dev/.env        # theo mục 4.2: REQUEST_FLOW_ENABLED=true, REQUEST_DEFAULT_OUTCOME_REVIEW=manual

# 2) service (áp migration tự động; thứ tự để tránh địa chỉ container cũ)
bash deploy/dev/scripts/redeploy-local.sh infra-fleet-service orchestration-service git-gateway-service task-service workflow-service request-service
BUILD_FRONTEND=1 bash deploy/dev/scripts/redeploy-local.sh frontend api-gateway

# 3) agent và pty-daemon
node agent/build.mjs && bash deploy/dev/scripts/start-local-agent.sh

# 4) CLI
(cd backend-go/cmd/orca-cli && go build -o orca .)    # đặt `orca` vào PATH

# 5) skill diagram-design trên dev server, rồi bật cho project
mkdir -p ~/.claude/skills && cp -R skills/diagram-design ~/.claude/skills/
orca engine set --project <id> --engine openspec --solution-agent=true --outcome-review manual
orca engine show --project <id> --check
```

Cuối `redeploy-local.sh` chạy suite `platform` làm smoke test; nếu `PLT-08/09/10` đỏ với `orca-go-request` "Restarting" thì request-service chưa sẵn sàng (thường thiếu token, xem `docker logs orca-go-request`).

**Máy chủ từ xa:** `bash deploy/dev/scripts/sync-to-server.sh <version>` build và đồng bộ binary, áp migration và `up -d`. Agent: `node agent/build.mjs` rồi `deploy/agent/scripts/deploy-agent.sh` ([`deploy/agent/README.md`](../agent/README.md)). Dev server dùng chung: báo trước vì restart agent làm mất terminal đang mở. Migration từ xa: `bash deploy/dev/scripts/migrate.sh --remote request` (và `task`, `workflow`).

## 6. Khởi động lại agent và pty-daemon

pty-daemon là tiến trình tách rời, sống qua việc restart agent: build agent mới **chưa** có tác dụng với `pty.create` cho đến khi daemon cũng được thay. `start-local-agent.sh` làm cả hai (kỳ vọng 2 tiến trình, pid mới):

```bash
ps aux | grep "agent/out/agent.js" | grep -v grep
```

Mọi terminal đang mở bị mất. Với dev server từ xa (systemd `orca-agent.service`) kiểm tay rằng daemon cũ đã dừng.

## 7. Kiểm tra sau triển khai

Các lệnh 1, 3, 4 lấy từ bản trước và **chưa được chạy lại trên nhánh này**; lệnh 5 đã chạy thành công trên Request #2318 (khoảng 7 phút). Lệnh 6 đã chạy nhiều lần trong quá trình nghiệm thu; lần chạy đầy đủ gần nhất dừng ở bước review sau code vì lỗi test đã sửa (`a6c0ca631`), và còn gãy ở bước gửi lại phát hiện (giới hạn 7 ở mục 9).

| # | Lệnh | Kỳ vọng | Tốn tiền? |
|---|---|---|---|
| 1 | `python3 tests/stages/e2e/orca-flows/run_acceptance.py platform` | tất cả ok | không |
| 2 | `orca engine show --project <id> --check` | không có dòng `blocked`; không `SKILL_MISSING`/`AGENT_TOO_OLD` | không |
| 3 | `python3 tests/stages/e2e/orca-flows/run_acceptance.py --ai-stub devserver outcome_review` | ORV-01..09 pass | không |
| 4 | `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- qa-bug-task-native-flow` | pass | không |
| 5 | `ORCA_E2E_CICDKIT_REQUEST_ID=<id Request đã hoàn tất> python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real ci-cd-kit-review` | review sau code sinh, quyết định, duyệt | có: một lượt context (khoảng 1,1–1,5 USD) và một lượt vẽ (0,5–0,9 USD) |
| 6 | `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real ci-cd-kit` | luồng đầy đủ từ UI (Analysis, Plan, thực thi, review) | **có: khoảng 35 phút, hơn 8 USD** |

**Không sửa hay commit gì trong repo orca khi chạy `--real`** và **không mở phiên khác tạo branch/worktree trong repo orca**: guard coi mọi thay đổi trong checkout (kể cả của phiên khác) là vi phạm an toàn và dừng lần chạy. Không chạy hai lần `--real` chồng nhau.

Kiểm tay trên giao diện (admin):

1. Mở một Request có Plan: có tab **Outcome review** ngay sau Execution. Project để `off` thì không có tab.
2. Ở pha thiết kế, bấm tạo review: có Architecture/Flow ở chế độ **Design**, `kg.snapshot` khác rỗng.
3. Sau khi Request `completed`: tạo review sau code; chế độ **After code** có hình delta; mục cần quyết định có nút "Accept as exception"/"Send back".
4. Approve chỉ cho phép khi hết mục chờ quyết định **và** lượt agent mới nhất không lỗi.

## 8. Quay lại

| Muốn | Cách | Mất dữ liệu? |
|---|---|---|
| Ẩn Outcome Review ngay | `REQUEST_REVIEW_KILL_SWITCH=true` rồi `redeploy-local.sh request-service`: tab ẩn, mọi RPC review trả `REQUEST_REVIEW_DISABLED` | không; bỏ công tắc là review trở lại nguyên vẹn |
| Giữ review nhưng bỏ vẽ | `REQUEST_REVIEW_DRAW_ENABLED=false` | không |
| Không dùng agent để vẽ | `REQUEST_REVIEW_DRAW_ENGINE=builtin` (miễn phí) | không |
| Tắt cho một project | `orca engine set --project <id> --outcome-review off` | không |
| Tắt Plan tự sinh | `orca engine set --project <id> --plan-auto-generate=false`, hoặc `REQUEST_PLAN_AUTO_GENERATE=false` cho cả triển khai | không |
| Quay binary | build lại từ commit trước trong một worktree riêng rồi `redeploy-local.sh`; cột/bảng mới không cần xoá | không |
| Migration | `.down.sql` có cho cả 13 migration. Chỉ chạy khi chắc không còn dữ liệu review/Plan tự sinh/duyệt bước cần giữ; `0105`–`0109` xoá bảng review | **có, nếu chạy `.down`** |
| Frontend | build lại từ commit cũ; chưa kiểm chứng việc chạy frontend mới trên gateway cũ | không |

## 9. Giới hạn đã biết của bản này (đọc trước khi bật cho người dùng thật)

Các điểm dưới đây có thật trong bản này; mỗi điểm đã được ghi vào kế hoạch sửa ở [`docs/crs/v9/outcome-review-fidelity/`](../../docs/crs/v9/outcome-review-fidelity/README.md) (CR-OR-040..042, **chưa triển khai**).

| # | Giới hạn | Hệ quả | Giảm thiểu hiện tại |
|---|---|---|---|
| 1 | **Regenerate "đọc lại" cũng trích lại thiết kế** (`Force` bỏ qua thiết kế đã khoá) | Thiết kế có thể **co lại** (Request #2318: 17 container, 40 cạnh → 5 container, 3 cạnh với cùng snapshot) và nhãn "deviation" bị sai | Tránh Regenerate sau khi review đã tốt trừ khi KG/tài liệu đổi; kiểm số container ở hình Architecture trước khi tin nhãn |
| 2 | **Không có danh sách flow bị ảnh hưởng**: agent tự chọn flow nào được vẽ | Request chạm 2 flow có thể chỉ có 1 hình (#2318: Chat có, nhập Sheet không); không có chỉ báo "đã vẽ X/Y" | Đọc danh sách mục (contracts/behavior/data) để thấy phần không có hình |
| 3 | SAD của `vnp-qa-platform` chỉ có 3 sơ đồ tuần tự; flow dạng chữ do agent trích | Flow thiết kế phụ thuộc agent | — |
| 4 | **Kết quả agent mong manh**: một lỗi cục bộ làm hỏng cả kết quả, chỉ thử lại một lần | Một lượt bị từ chối tốn 1,5–2 USD (#2318: 3,51 USD cho hai lần) | Đã sửa prompt cho hai lỗi gặp; lý do từ chối nay được log (`agent result rejected`); sau sửa 2/2 lần chạy thật thành công (mẫu nhỏ) |
| 5 | Hình kiến trúc thiết kế cắt theo thứ tự ID, không theo Request | "+N more" che container liên quan | Dùng drilldown để xem danh sách container |
| 6 | Huy hiệu "not in design" đếm cạnh của hình delta, còn hàng đếm mục | Số trên tab có thể lệch số hàng | — |
| 7 | Sau khi Approve, **không gửi lại (Send back) được nữa**; Regenerate làm review về `ready` và mất lần duyệt | Phải gửi lại **trước** khi Approve | Spec e2e đầy đủ hiện Approve rồi mới gửi lại, nên **gãy ở bước cuối** (sửa thứ tự là việc còn lại) |
| 8 | Khung Flow ở chế độ After code có thể "agent không vẽ gì" dù có mục về flow | Thiếu hình, không thiếu mục | Thông báo trên tab đã nói rõ là agent không vẽ |
| 9 | Behavior thường không có hình (chỉ khi KG mô tả state machine) | Tab trống hình | — |
| 10 | Project Settings chưa có công tắc Outcome Review | Chỉ bật bằng CLI/biến môi trường | mục 4.3 |
| 11 | Bản dịch `es`, `ja`, `ko`, `zh` của chuỗi mới chưa được người bản ngữ rà | | |
| 12 | `pnpm lint` chạy `verify:localization-catalog`/`verify:localization-coverage` trỏ tới script **không tồn tại** trong `config/scripts` | Lint tổng báo lỗi từ trước, không do bản này | |
| 13 | Chưa chạy: Electron, trình đọc màn hình, GitNexus; một số test cũ lỗi từ trước (`RequestRepositoryContract`, `no-top-level-translate`, `localStorage`) | | |
| 14 | Worktree/branch cũ của lần e2e (`vnp-qa-platform-task-*`) chưa dọn | Tốn đĩa | dọn tay |

**Chi phí vận hành đã đo** (Request #2318, BL-E05, Claude thật): review thiết kế: context 1,39 USD + vẽ 0,77 USD; review sau code: context 1,09–1,54 USD + vẽ 0,52–0,92 USD; mỗi lần agent bị từ chối kết quả 1,5–2,0 USD. Đặt `REQUEST_REVIEW_CONFIRM_COST` (ví dụ `2`) để Regenerate hỏi trước khi chạy khi ước tính vượt ngưỡng; giữ `REQUEST_REVIEW_REGEN_PER_DAY` mặc định 5.

## 10. Tài liệu cũ lệch với bản này

Không sửa trong lần này; đọc theo bảng:

| Tài liệu | Dòng lệch | Đúng là |
|---|---|---|
| `RELEASE-2026-10-DEPLOY.md` §9 | "`frontend` chưa: màn hình Outcome review chưa làm" | đã có tab Outcome review |
| `RELEASE-2026-10-DEPLOY.md` §9, `docs/guides/request/outcome-review-setup.md` §1 và §5 | "skill `diagram-design` chưa được đưa vào `skills/`", "hằng ghim đang rỗng" | skill đã vendor, ghim mặc định `1cad611bfe734dc9` (`VendoredDiagramDesignSHA`); cài như mục 4.4 |
| `RELEASE-2026-10-DEPLOY.md` §3 | chỉ nêu migration 0098 | thêm 0099–0109 (request), 0022 (task), 0014 (workflow) |

## 11. Tham chiếu

- Hướng dẫn bật review: [`docs/guides/request/outcome-review-setup.md`](../../docs/guides/request/outcome-review-setup.md) (đọc cùng mục 10).
- Hướng dẫn tri thức: [`docs/guides/request/knowledge-hub-setup.md`](../../docs/guides/request/knowledge-hub-setup.md).
- CR: `docs/crs/v9/` (CR-OR-001..030) và `docs/crs/v9/outcome-review-fidelity/` (kế hoạch sửa các giới hạn mục 9).
- Skill vendor: `skills/diagram-design/NOTICE`, `skills/diagram-design/MANIFEST.sha256`.
