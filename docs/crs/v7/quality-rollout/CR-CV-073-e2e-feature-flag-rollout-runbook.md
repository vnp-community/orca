# CR-CV-073 — E2E, cờ `code_intel_enabled` theo tenant, rollout, quay lui và tài liệu vận hành

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-073 |
| **Tên** | Bộ e2e (service, stack dev, Playwright web và Electron), cờ `code_intel_enabled` theo tenant, rollout theo giai đoạn, kế hoạch quay lui, tài liệu vận hành (cài GitNexus/CodeGraph trên dev server, token, runbook sự cố) |
| **Loại** | Chất lượng / Vận hành |
| **Priority** | 🔴 P0 (gate trước khi bật cho người dùng) |
| **Effort** | Medium (6 đến 8 ngày, rải theo các CR khác; phần cờ làm sớm) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-010, 011, 012, 013 (service, cờ, quyền); CR-CV-040 (kênh); CR-CV-050, 051 (frontend); CR-CV-070, 071, 072 (gate); phần cờ chỉ cần CR-CV-010 và 013 |
| **Mở khoá** | GA của tính năng "Xem code" |
| **Tác động** | `backend-go/services/code-intel-service` (`e2e/`, bảng và RPC cờ, migration hai dialect, interceptor), `backend-go/services/api-gateway` (kênh `codeIntel.settings.*`, CR-CV-040), `backend-go/ci/` (script mới), `.github/workflows/`, `deploy/dev/docker-compose.yml`, `deploy/dev/scripts/migrate.sh`, `backend-go/deploy/postgres-init-databases.sh`, `backend-go/Makefile`, `backend-go/go.work`, `frontend/src/renderer/src/test-support/` (fake backend), `tests/e2e/code-intel-web/` (mới), `tests/playwright.web.config.ts`, `tests/code-intel/` (mới), `docs/guides/code-intel/` (mới) |

---

## 1. Bối cảnh và vấn đề

1. README v7 O8 chốt: toàn bộ tính năng sau cờ `code_intel_enabled`, **theo tenant**, mặc định tắt. Chưa nói cờ lưu ở đâu, ai đọc, ai thi hành, tắt giữa chừng thì sao. README mục 3.5 (bảng), 3.6 (RPC) và 3.7 (kênh) chưa có gì cho cờ.
2. **Cơ chế cờ thật trong repo (đã đọc):**
   - Cờ tiến trình của gateway: `MCP_ENABLED` (mặc định `false`) trong `api-gateway/internal/config/config_mcp.go` và `deploy/dev/docker-compose.yml:576` (`MCP_ENABLED: ${MCP_ENABLED:-false}`); tắt thì `/mcp` không mount, `mcp-service` không dial, kênh `mcp.*` trả `MCP_DISABLED`.
   - Cờ theo tenant: bảng `mcp.tenant_settings` của `mcp-service` (`domain/tenant_settings.go`: `TenantSettings{TenantID, Enabled, ...}`, `DefaultTenantSettings(tenantID, enabled, ...)` tạo dòng lười), biến `MCP_TENANT_DEFAULT_ENABLED` quyết định `enabled` của dòng **mới** (không đổi dòng cũ, `mcp-service/README.md:35,73`), RPC `UpdateTenantSettings` (`proto/orca/mcp/v1/mcp.proto:67`), và frontend đọc qua kênh `mcp.server.info` (xuất hiện trong `test-support/mcp-integration/mcp-pane-states.integration.test.tsx`; chưa đọc component dùng nó), ẩn mục MCP khi tắt (test `tests/e2e/mcp-web/rollout.web.e2e.ts`).
   - Không có hệ thống feature-flag chung trong repo: grep `feature_flag|FeatureFlag` trong `backend-go` chỉ trúng một test publisher. Vì vậy cờ của code-intel theo mẫu của `mcp-service` và của v6 CR-REQ-025 (bảng `tenant_settings` ở service chủ).
3. E2E hiện có để tái dùng (đã đọc): `tests/e2e/mcp-web/*.web.e2e.ts` chạy trên Chromium với **WebSocket giả** (`support/mock-orca-ws.ts` trả lời dialect session-client `{id, authToken, method, params}` → `{id, ok, result|error, _meta}` từ `frontend/src/renderer/src/test-support/mcp-fake-backend.ts`); biến `MCP_E2E_BASE_URL` chạy thêm các ca `@dev-stack` trên stack thật; cấu hình `tests/playwright.web.config.ts` (`testDir: './e2e/mcp-web'`, `testMatch: '**/*.web.e2e.ts'`, `webServer` là Vite cổng 5174, script `test:e2e:mcp-web`). Electron có `tests/playwright.config.ts` và specs `tests/e2e/*.spec.ts` với quy tắc ở `tests/e2e/AGENTS.md` (build `--mode e2e`, **khẳng định trên DOM, không trên store**, ưu tiên unit-test slice khi logic thuần). Python: `tests/mcp/` và `tests/backend/` chạy trên stack dev (`mcp_check_framework.py`, `check_framework.py`).
4. Hạ tầng cài đặt trên dev server (đã đọc): agent chạy bằng systemd `deploy/agent/orca-agent.service` với **`PATH=/home/ubuntu/.local/bin:/home/ubuntu/bin:/usr/local/bin:/usr/bin:/bin:/snap/bin`**; agent tìm công cụ trong `PATH` của nó (`agent-tool-registry.ts`, `discoverTools`). Trên máy khảo sát: `gitnexus` cài bằng npm toàn cục (`/usr/lib/node_modules/gitnexus`, lệnh `/usr/bin/gitnexus`), `codegraph` ở `~/.codegraph/versions/v1.4.1/bin/codegraph` với liên kết `~/.local/bin/codegraph`. AGENTS.md của repo có dòng "npm 11 crash → `npm i -g gitnexus`; #1939". Kích thước chỉ mục cho Orca: `.gitnexus/` 2,7 GB, `.codegraph/` 1,3 GB (đo bằng `du`, 2026-10-05); `.gitnexus/` nằm trong `.git/info/exclude`.
5. **`gitnexus analyze` mặc định ghi vào cây làm việc** (đã kiểm chứng: `gitnexus analyze --help` và `git status` hiện `M AGENTS.md`, `M CLAUDE.md` sau khi chạy `analyze` trên chính repo này): chèn mục vào `AGENTS.md`/`CLAUDE.md`, cài `.claude/skills/gitnexus/`, ghi `~/.gitnexus/registry.json`. Cờ `--index-only` tắt việc chèn tệp. Đây là rủi ro vận hành lớn nhất của `codeintel.reindex` (CR-CV-004, 072): reindex không có `--index-only` làm bẩn diff người dùng đang review. Runbook và e2e ở đây phải khẳng định điều ngược lại.
6. Đăng ký service mới dễ sót: `deploy/dev/scripts/migrate.sh` có danh sách `SERVICES` cố định (`auth tenant ... mcp issuestatussync`), `backend-go/deploy/postgres-init-databases.sh` có `DATABASES`, `backend-go/Makefile` có `SERVICES`, `backend-go/go.work` liệt kê module. Thiếu `codeintel` ở một trong số đó thì DB không được migrate hoặc service không build trên môi trường nào đó (cùng bài học v6 CR-REQ-025 mục 1.3).

## 2. Giải pháp đề xuất

### 2.1 Cờ `code_intel_enabled`

| Hạng mục | Quyết định |
|---|---|
| Hai tầng | Tổng: biến `CODE_INTEL_ENABLED` của `code-intel-service` (mặc định `false`). Theo tenant: bảng `codeintel.tenant_settings(tenant_id, code_intel_enabled, updated_by, updated_at)` (**mới**, migration cho Postgres và MySQL dùng `common/dbcapability`; Postgres có RLS thật, xem CR-CV-072 2.7) |
| Hiệu lực | `effective = CODE_INTEL_ENABLED && tenant_settings.code_intel_enabled`; không có dòng nghĩa là `false`; lỗi đọc nghĩa là `false` (fail closed) |
| Ai thi hành | Chỉ `code-intel-service`, ở một interceptor gRPC (`internal/adapter/grpc/feature_gate.go`, mới). Gateway **không** có bản sao cờ (CR-CV-040 G5), nên không lệch giữa WS, MCP và push |
| RPC | `GetSettings`, `SetSettings` trên `CodeIntelService` (**mới**; README 3.6 chưa có, ghi vào "Điều chỉnh hợp đồng"). `SetSettings` chỉ cho `role=admin` (`Identity.Role`, gateway chuyển qua metadata) và ghi audit `codeintel.settings.set` |
| Kênh | `codeIntel.settings.get` (mọi thành viên tenant, luôn trả được) và `codeIntel.settings.set` (admin), đã có trong danh sách chốt của CR-CV-040 |
| Ai đọc | Frontend gọi `codeIntel.settings.get` lúc khởi động (CR-CV-050) để hiện hoặc ẩn lối vào Review (dashboard, Source Control, Cmd+K, right sidebar; CR-CV-061); **không** mở stream `codeIntel.subscribe` khi tắt (khẳng định bằng e2e, như `backend.streamCount()` ở test MCP) |
| Lỗi | `CODEINTEL_DISABLED` (`FailedPrecondition`) cho mọi RPC ngoài `GetSettings` |
| Mặc định tenant mới | `false` ở mọi giai đoạn tới GA; khác `MCP_TENANT_DEFAULT_ENABLED=true` của `mcp-service`. Đổi mặc định là quyết định sản phẩm riêng (câu hỏi mở 3) |
| Bật cho tenant | admin tenant dùng `codeIntel.settings.set`; vận hành đặt `CODE_INTEL_ENABLED=true` cho `code-intel-service` trong `deploy/dev/docker-compose.yml` (`${CODE_INTEL_ENABLED:-false}`, theo mẫu `MCP_ENABLED`) |
| Bộ đệm | Interceptor không đệm quá 5 giây (đề xuất) để tắt cờ có hiệu lực gần tức thì; ghi số giây vào runbook |
| Agent | Phương thức `codeintel.*` luôn có trên agent (khi đã triển khai) nhưng **không ai gọi** khi cờ tắt, nên không phát sinh tải. Tuỳ chọn tắt cứng tại máy: biến `ORCA_CODEINTEL_DISABLED=1` trong `.env` của agent (CR-CV-001 quyết định có làm không) |

Hành vi khi cờ tắt giữa chừng:

| Nhóm | Khi tắt |
|---|---|
| Đọc view, `RequestReindex`, ghi review/C4/finding/binding | `CODEINTEL_DISABLED` |
| `GetSettings` | cho phép |
| Job reindex đang chạy | chạy tới hết (không thể huỷ an toàn một `analyze` giữa chừng); không nhận job mới |
| Nội bộ (consumer `codeintel.indexChanged`, hủy cache, outbox, dọn `expires_at`) | vẫn chạy, để khi bật lại không phục vụ cache cũ và outbox không tắc |
| Dữ liệu `review_states`, `c4_overrides`, `finding_dismissals`, `repo_bindings` | giữ nguyên, không xoá; cờ không ẩn hay xoá dữ liệu |

### 2.2 Bốn tầng kiểm thử

| Tầng | Chạy ở đâu | Phụ thuộc | Chặn PR |
|---|---|---|---|
| T1 e2e trong tiến trình | `backend-go/services/code-intel-service/e2e/` (mới), build tag `e2e`, testcontainers | agent **giả** phát lại fixture của CR-CV-070 (kết quả `codeintel.*` đã chuẩn hoá), `infra-fleet` giả trong bộ nhớ, outbox thật, DB thật | **Có**, ma trận `postgres`, `mysql` |
| T2 web trên Chromium | `tests/e2e/code-intel-web/*.web.e2e.ts` (mới), WS giả từ `code-intel-fake-backend.ts` (mới), cờ `@dev-stack` cho stack thật | Vite (như MCP) | **Có** khi PR đụng `frontend/src/renderer/src/**` liên quan (đường dẫn lọc) |
| T3 stack dev thật | `tests/code-intel/` (Python, theo `tests/mcp/mcp_check_framework.py`), chạy qua `backend-go/ci/code-intel-e2e/run-code-intel-e2e.sh` (mới) | `deploy/dev/docker-compose.yml` dựng service thật; một agent thật gắn dev server; GitNexus và CodeGraph thật; repo mẫu của CR-CV-070 | Không; hằng đêm và `workflow_dispatch` (`.github/workflows/code-intel-e2e.yml`, mới) |
| T4 repo cỡ Orca | cùng T3 trên một commit Orca cố định, chỉ mục dựng sẵn trên runner chuyên dụng (cùng với benchmark CR-CV-071) | Không, báo cáo |
| Electron smoke | `tests/e2e/code-intel-electron.spec.ts` (mới, tối thiểu) | build `--mode e2e` | Không chặn ở v7; chạy theo `test:e2e` hiện có |

Lý do tách: stack đầy đủ và công cụ ngoài không ổn định, không đưa vào cổng chặn (cùng nguyên tắc v5 CR-MCP-015 và v6 CR-REQ-025). Chưa kiểm chứng stack `deploy/dev` chạy được trong runner CI (nhiều service, Vault, NATS); T3 không chặn vì lý do đó.

### 2.3 Bộ kịch bản e2e

Mỗi kịch bản kiểm: kết quả/lỗi đúng, sự kiện, audit, **không rò** (secret, tenant), và với T3: cây làm việc của worktree **không đổi** (`git status --porcelain` trước và sau, chốt chặn cho rủi ro mục 1.5).

| ID | Tầng | Kịch bản |
|---|---|---|
| E01 | T1, T3 | `GetIndexStatus`: công cụ có sẵn, `indexedAt`, `stale` đúng khi HEAD vượt quá `lastCommit` |
| E02 | T1, T3 | `GetChangeOverlay` cho worktree có thay đổi: symbol, luồng bị ảnh hưởng, thứ tự đọc; `headCommit` đúng |
| E03 | T1, T3 | `GetImpact` và `GetSymbol`; symbol trùng tên (hai tệp) → `CODEINTEL_AMBIGUOUS_SYMBOL` kèm `candidates`, chọn một rồi gọi lại |
| E04 | T1, T3 | `GetErd` từ migration của repo mẫu (Postgres và MySQL) |
| E05 | T1, T3 | `RequestReindex`: job → `reindexProgress` → `codeIntel.changed`; lần hai trong lúc chạy → `CODEINTEL_REINDEX_IN_PROGRESS`; **`git status --porcelain` không đổi** và không có `AGENTS.md`/`CLAUDE.md`/`.claude/skills` bị sửa (kiểm `--index-only`) |
| E06 | T1 | cache: lần hai trúng cache (`cache=hit`), `codeIntel.changed` huỷ cache, lần ba gọi lại agent; singleflight gộp 10 yêu cầu đồng thời thành một lần gọi agent |
| E07 | T1, T3 | công cụ thiếu (không có `gitnexus` trong `PATH`) → `CODEINTEL_TOOL_UNAVAILABLE`; chưa có index → `CODEINTEL_INDEX_MISSING` kèm gợi ý reindex |
| E08 | T1 | `workspaceRoot` không đăng ký → `CODEINTEL_REPO_NOT_REGISTERED`; worktree lồng không dùng index repo cha |
| E09 | T1 | timeout: agent giả chậm → `CODEINTEL_TIMEOUT`; việc nền vẫn xong, lần gọi lại trúng cache |
| E10 | T1 | kết quả bị cắt: `truncated:true` và `totalCount` đi hết chuỗi tới UI |
| E11 | T1 | quyền (ma trận CR-CV-072 2.8): chỉ-đọc không reindex được; người tenant khác nhận `NotFound` |
| E12 | T1 | cờ (bảng 2.1): tắt → `CODEINTEL_DISABLED`; bật tắt bật giữa job reindex; tenant A bật không ảnh hưởng tenant B; `GetSettings` luôn trả |
| E13 | T1 | `review_states`: lưu với `expectedVersion`, xung đột → `CODEINTEL_VERSION_CONFLICT`; ghi chú còn nguyên sau tắt cờ rồi bật lại |
| E14 | T1, T3 | dev server mất kết nối giữa chừng (agent giả ngắt) → lỗi rõ, UI không treo (hạn xếp hàng ~20 s của backend, README v7 mục 7); `stale` giữ dữ liệu cũ từ cache nếu có |
| E15 | T3 | CodeGraph thật: `status`, `query`, `callers` cho repo mẫu; hợp nhất `SymbolRef.key` giữa hai nguồn (CR-CV-020) |
| E16 | T3 | phiên bản công cụ lạ (giả lập `--version` khác) → `compatibility=untested` hoặc `incompatible` theo CR-CV-070 2.6 |
| E17 | T1, T3 | MCP (chỉ khi CR-CV-041): `codeIntel_status` → `changeOverlay` → `symbol`; kết quả bọc `untrusted-content`; không có tool `reindex` |
| E18 | T1 | chu kỳ đầy đủ qua WS thật: `codeIntel.subscribe` nhận `changed` và `reindexProgress`; stream đứt → `resync:true` |

Ma trận tính năng (`TestEveryRPCHasScenario`): đọc danh sách RPC từ proto (README 3.6) và khẳng định mỗi RPC xuất hiện trong ít nhất một kịch bản T1; thêm RPC mà quên kịch bản thì đỏ (cùng tinh thần v6 CR-REQ-025 D5).

### 2.4 Playwright (T2) và Electron

**Hạ tầng (mới):**

| Việc | Chi tiết |
|---|---|
| Fake backend | `frontend/src/renderer/src/test-support/code-intel-fake-backend.ts` (mẫu `mcp-fake-backend.ts`): trả lời `codeIntel.*` theo dialect session-client; có thao tác kịch bản `setSettings({enabled})`, `setIndex({stale, indexedAt})`, `failNext(code)`, `pushChanged()`, `pushProgress({percent})`, `setOverlay(...)`, `setErd(...)`, `streamCount()`. Dữ liệu dựng từ **tệp vàng C2** của CR-CV-070 (`backend-go/services/code-intel-service/testdata/agent-results/`), không bịa dữ liệu thứ hai |
| Hỗ trợ | `tests/e2e/code-intel-web/support/mock-code-intel-ws.ts` tái dùng `mockOrcaApp` và `bootApp` của `mcp-web/support` (di chuyển sang thư mục dùng chung đặt tên theo nội dung nếu cần; không tạo `helpers`/`utils`) |
| Cấu hình | `tests/playwright.web.config.ts` hiện có `testDir: './e2e/mcp-web'`. Đề xuất chuyển sang `projects: [{name:'mcp-web', testDir:'./e2e/mcp-web'}, {name:'code-intel-web', testDir:'./e2e/code-intel-web'}]` (Playwright cho `testDir` theo project), giữ `testMatch: '**/*.web.e2e.ts'` để không lẫn với project Electron `**/*.spec.ts`; thêm script `test:e2e:code-intel-web` vào `package.json`; biến `CODE_INTEL_E2E_BASE_URL` cho ca `@dev-stack` (hoặc tái dùng `MCP_E2E_BASE_URL`: quyết định ở câu hỏi mở 4) |

**Specs (khẳng định trên DOM, theo `tests/e2e/AGENTS.md`):**

| Spec | Nội dung |
|---|---|
| `flag.web.e2e.ts` | cờ tắt: không có lối vào Review ở Source Control/Cmd+K/right sidebar, không lỗi trang liên quan, `backend.streamCount()==0`; cờ bật: có; admin bật được từ cài đặt, người thường không thấy nút; tắt trực tiếp (push) làm lối vào biến mất không cần tải lại (mẫu "kill switch banner appears live" ở `rollout.web.e2e.ts`) |
| `review-summary.web.e2e.ts` | mở tab `review` từ Source Control: thanh tóm tắt (số symbol, luồng, bảng chạm), chip index hiện `indexedAt` và `stale`; thứ tự đọc và tiến độ; điều hướng bàn phím; chuỗi hiển thị qua `translate()` (không lộ khoá i18n thô) |
| `states.web.e2e.ts` | `CODEINTEL_TOOL_UNAVAILABLE` (thông điệp + hướng dẫn), `CODEINTEL_INDEX_MISSING` (nút reindex), `CODEINTEL_TIMEOUT` (thử lại), `CODEINTEL_AMBIGUOUS_SYMBOL` (hộp chọn `candidates`), `truncated` (huy hiệu "đã cắt"), lỗi quyền (thông điệp trung tính), mất kết nối (không treo) |
| `reindex.web.e2e.ts` | bấm reindex → thanh tiến trình theo `reindexProgress`; lần hai bị từ chối `REINDEX_IN_PROGRESS`; xong thì chip `stale` biến mất nhờ push `changed` |
| `lenses.web.e2e.ts` | lens Ảnh hưởng và ERD (MVP) hiện dữ liệu từ tệp vàng; nhãn chứa HTML/`<script>`/chỉ thị Mermaid hiển thị như văn bản (liên hệ CR-CV-072 2.9) |
| `dev-stack.web.e2e.ts` (`@dev-stack`) | khi có `*_E2E_BASE_URL`: đăng nhập, bật cờ, mở Review cho worktree của repo mẫu, thấy dữ liệu thật |
| `code-intel-electron.spec.ts` (Electron) | `window.api.codeIntel` tồn tại ở preload; cờ tắt ẩn lối vào; khẳng định trên DOM, build `--mode e2e` |

Hai render target (README v7 mục 6): web (Chromium, `web-preload-api`) bắt buộc chặn; Electron (`window.api` preload) chỉ smoke ở v7.

### 2.5 Kiểm tra đăng ký đầy đủ

`backend-go/ci/check-code-intel-service-wiring.sh` (mới, mẫu `check-opa-bundle-in-images.sh`, và `check-request-service-wiring.sh` đề xuất ở v6 CR-REQ-025) thất bại nếu thiếu code-intel ở: `backend-go/go.work`, `SERVICES` trong `backend-go/Makefile`, `DATABASES` trong `backend-go/deploy/postgres-init-databases.sh` (`codeintel`), danh sách `SERVICES` của `deploy/dev/scripts/migrate.sh`, service `code-intel-service` và `migrate-code-intel` trong `deploy/dev/docker-compose.yml`, biến `CODE_INTEL_SERVICE_ADDR` cho `api-gateway`, workflow `backend-go-code-intel-service.yml` (mẫu `backend-go-task-service.yml`, ma trận `dialect`). Chạy trong workflow của service. Kiểm chứng: cố ý xoá `codeintel` khỏi `migrate.sh` thì script đỏ.

### 2.6 Rollout theo giai đoạn

Khớp với các đợt ở README v7 mục 5. Số ngày và ngưỡng là đề xuất ban đầu, chưa có số đo thực.

| Giai đoạn | Phạm vi | Điều kiện vào | Điều kiện chuyển tiếp |
|---|---|---|---|
| 0. Nội bộ | `CODE_INTEL_ENABLED=true` ở dev; một tenant thử; một dev server của đội; repo mẫu và Orca; đợt 1 và 2 (service chạy, kênh, `changeOverlay`) | T1 xanh hai dialect; script 2.5 xanh; CR-CV-070 tầng chặn xanh; CR-CV-072 tầng chặn xanh | 3 ngày không có cảnh báo `CodeIntelToolFormatDrift`, `CodeIntelReindexStuck`; `git status` của worktree không đổi sau reindex (E05) |
| 1. Dogfood (MVP review: đợt 3) | Tenant đội Orca, lens Ảnh hưởng, ERD, thứ tự đọc; repo Orca và vài repo `vnp-*` | T2 xanh; benchmark đêm trong ngân sách (CR-CV-071); runbook được diễn tập một lần | 2 tuần: `CodeIntelLatencyOverBudget` không bật quá 1 lần; không canary secret lộ; không sự cố rò tenant; tỉ lệ `result` lỗi (không tính `invalid_params`, `disabled`, `forbidden`) < 5 % |
| 2. Beta | Tenant opt-in (admin bật `settings.set`); thêm CodeGraph, C4, luồng, cấu trúc (đợt 4); repo vừa và nhỏ trước | CR-CV-072 live xanh 5 đêm liên tục; số liệu dogfood; dung lượng đĩa dev server đã tính (xem 2.8) | Không hồi quy ngân sách 2 tuần; phản hồi người dùng; chưa có `incompatible` công cụ kéo dài |
| 3. Mở rộng | Phát hiện, hợp đồng, phản hồi cho agent (đợt 5); `relay-ssh`, lưu trữ, MCP, mobile (đợt 6, từng mục có CR riêng và cổng riêng) | Mỗi mục có kiểm thử bảo mật bổ sung (CR-CV-072 cho MCP và StorageMap) | Mỗi mục: ít nhất 5 phiên review thật hoàn tất |
| 4. GA | Cờ theo tenant mở cho mọi admin; mặc định `false` giữ nguyên tới quyết định riêng | review bảo mật độc lập; ghi kết quả | |

"Bật theo lens" chưa có cơ chế: cờ là theo tenant, **không** theo lens (quyết định D7, giống v6 D7). Lens thuộc đợt sau chỉ xuất hiện khi bản frontend có lens đó; không phải cờ.

### 2.7 Kế hoạch quay lui

1. **Tắt cờ** của tenant (`codeIntel.settings.set enabled=false`) hoặc biến tổng `CODE_INTEL_ENABLED=false` rồi khởi động lại `code-intel-service`. Hiệu lực tức thì (≤ 5 giây) cho mọi RPC theo bảng 2.1; UI ẩn lối vào sau lần đọc `settings.get` kế tiếp hoặc khi tải lại.
2. **Cắt ở gateway** nếu service lỗi nặng: đặt `CODE_INTEL_SERVICE_ADDR=""` và khởi động lại `api-gateway`: mọi kênh trả `CODEINTEL_UNAVAILABLE` (CR-CV-040 2.1), không động tới DB.
3. **Agent:** không cần quay lui (không gọi thì không chạy); nếu cần tắt tại máy: `ORCA_CODEINTEL_DISABLED=1` rồi `sudo systemctl restart orca-agent` (nếu CR-CV-001 làm).
4. **MCP** (nếu CR-CV-041): tắt tool bằng chính sách tenant của `mcp-service` (CR-MCP-012) hoặc kill switch (CR-MCP-013); hoặc gỡ `pack1CodeIntel()` và trả dòng `codeIntel.*` về `excluded_channels.yaml`.
5. **Job reindex đang chạy:** chạy tới hết (không có RPC huỷ ở README 3.6). Nếu kẹt, runbook mục 2.9 nêu cách dừng tay và dọn dấu vết; **không** tự động.
6. **Dữ liệu:** không chạy `down` của migration khi bảng còn dòng; migration `down` phải từ chối nếu có dữ liệu. `graph_snapshots` hết hạn tự theo `expires_at`; dọn tay bằng câu lệnh trong runbook nếu cần giải phóng dung lượng.
7. **Chỉ mục trên dev server:** giữ nguyên (không `gitnexus clean`, không xoá `.gitnexus/` hay `.codegraph/`); chúng độc lập với Orca và còn dùng được bằng tay.
8. Diễn tập quay lui là điều kiện của giai đoạn 2; có test T1 cho thứ tự bật, tắt, bật (E12).

### 2.8 Tài liệu vận hành (`docs/guides/code-intel/`, mới, tiếng Việt)

| File | Nội dung |
|---|---|
| `README.md` | tổng quan, view nào có ở giai đoạn nào, ai làm gì |
| `install-gitnexus-codegraph-on-dev-server.md` | xem bảng dưới |
| `enable-code-intel-for-tenant.md` | cờ theo tenant, vai trò admin, rollout, quay lui (mục 2.1, 2.7) |
| `using-review-view.md` | mở Review, đọc chip `stale`, thứ tự đọc, reindex, các mã lỗi và ý nghĩa với người dùng |
| `runbook-code-intel.md` | bảng sự cố bên dưới |
| `code-intel-performance-and-metrics.md` | từ CR-CV-071 |
| `code-intel-threat-model.md` | từ CR-CV-072 2.1 |

Cập nhật: `backend-go/README.md` (hàng `code-intel-service`, RPC nào thật, nào chưa), `backend-go/services/api-gateway/README.md` (kênh mới), `deploy/agent/README.md` (mục công cụ code-intel), `docs/guides/mcp/` (nếu CR-CV-041), cột "Trạng thái" của mọi CR v7 khi triển khai.

**Cài GitNexus và CodeGraph trên dev server** (nội dung `install-gitnexus-codegraph-on-dev-server.md`, chỉ những gì đã kiểm chứng; phần chưa kiểm chứng ghi rõ):

| Bước | Nội dung | Kiểm chứng |
|---|---|---|
| Điều kiện | Node (cho `gitnexus`), RAM trống ≥ 4 GiB (2 tiến trình `gitnexus` đồng thời ~1,7 GB, CR-CV-071), đĩa ≈ 4 GB cho một repo cỡ Orca (`.gitnexus` 2,7 GB + `.codegraph` 1,3 GB; số cho repo nhỏ hơn thấp hơn) | đã đo trên máy khảo sát |
| GitNexus | `npm i -g gitnexus` (AGENTS.md: "npm 11 crash → `npm i -g gitnexus`"); phiên bản hỗ trợ **1.6.9**; `gitnexus --version` | đã thấy bản cài toàn cục 1.6.9; lệnh cài là theo AGENTS.md, chưa chạy lại |
| CodeGraph | cài theo bộ cài riêng của công cụ vào `~/.codegraph/versions/<ver>/bin/codegraph` và liên kết vào `~/.local/bin/codegraph`; phiên bản hỗ trợ **1.4.1**; **không** chạy `codegraph install` (lệnh đó ghi cấu hình MCP của các agent khác) | cách cài chưa xác minh; chỉ thấy kết quả cài |
| `PATH` của agent | agent chạy bằng systemd với `PATH=/home/ubuntu/.local/bin:/home/ubuntu/bin:/usr/local/bin:/usr/bin:/bin:/snap/bin` (`deploy/agent/orca-agent.service`); công cụ cài ở nơi khác (ví dụ `nvm`) sẽ **không** được thấy → `CODEINTEL_TOOL_UNAVAILABLE`. Cách xử lý: đặt liên kết vào một thư mục trong `PATH` hoặc sửa `Environment=PATH=` rồi `daemon-reload` và `restart` | đã đọc unit file |
| Dựng chỉ mục | do người dùng bấm reindex trong Orca (O3). Thủ công: GitNexus **phải** `gitnexus analyze --index-only` (mục 1.5), CodeGraph `codegraph init`/`index` rồi `codegraph sync`. Chưa đo thời gian dựng chỉ mục Orca (nhiều phút theo nghiên cứu 06); `analyze` dùng `cores-1` luồng (tối đa 16) nên giới hạn bằng `--workers` trên máy chia sẻ | cờ đã đọc từ `--help`; thời gian chưa đo |
| Đăng ký repo | GitNexus ghi `~/.gitnexus/registry.json` (12 repo trên máy khảo sát); `-r` nhận tên hoặc đường dẫn tuyệt đối, khớp chính xác; hai repo trùng tên cần `--name` | đã kiểm chứng `-r <đường dẫn>` |
| Kiểm tra | `gitnexus list`; `codegraph status --json`; trong Orca: `codeIntel.status` phải ra `compatibility=verified` | |
| Token | **Hai loại khác nhau, đừng nhầm.** (a) Token **agent** (một lần, lấy bằng `POST /api/agent-token` hoặc `bash deploy/agent/scripts/connect-agent.sh`) để agent dial `wss://<orca>/agent`; hết hạn thì `systemd Restart=always` + `start.sh` lấy token mới. (b) Token **MCP PAT** (`tests/mcp/get_mcp_token.py`) chỉ cần cho tool `codeIntel_*` (CR-CV-041) và cho script e2e T3. Không có "token code-intel" riêng ở v7 | đã đọc `orca-agent.service`, `tests/mcp/README.md` |
| Cập nhật agent | `scp agent/out/agent.js ubuntu@<devserver>:~/orca-agent/agent.js` rồi `sudo systemctl restart orca-agent` (theo `deploy/agent/orca-agent.service`); sau khi nâng cấp, `codeintel.status` phải hiện phương thức mới trong `tools[]` của handshake | đã đọc |

### 2.9 Runbook sự cố (`runbook-code-intel.md`)

| Triệu chứng | Kiểm | Xử lý |
|---|---|---|
| UI báo "công cụ chưa có" (`CODEINTEL_TOOL_UNAVAILABLE`) | `tools[]` trong handshake của agent; `which gitnexus codegraph` **dưới `PATH` của systemd** (`sudo systemctl show orca-agent -p Environment`); `compatibility` trong `codeintel.status` | sửa `PATH`; cài đúng phiên bản; nếu `incompatible`: xem hàng "trôi định dạng" |
| `CODEINTEL_INDEX_MISSING` | `ls <repo>/.gitnexus/meta.json`, `codegraph status`; đĩa trống | bấm reindex; kiểm đĩa; xem log agent `journalctl -u orca-agent` |
| Luôn `stale` | `meta.json.lastCommit` so với `git rev-parse HEAD`; `gitnexus status` ghi "stale (re-run gitnexus analyze)" | reindex (tay hoặc UI); nhắc: Orca không tự reindex (O3) |
| Reindex kẹt (`CodeIntelReindexStuck`) | tiến trình `gitnexus analyze` còn không (`ps`); thấy tệp `.gitnexus/lbug.wal.missing-shadow.*` (đã gặp trên máy khảo sát) | chờ thêm; nếu đã quá ngưỡng, dừng tiến trình **bằng tay** và chạy lại reindex; chưa kiểm chứng cách phục hồi WAL (nghiên cứu 06 §2); không chạy hai `analyze` song song |
| Worktree đột nhiên "bẩn" sau reindex | `git status` có `AGENTS.md`/`CLAUDE.md`/`.claude/skills/gitnexus/` bị đổi | reindex đã chạy thiếu `--index-only`: báo lỗi CR-CV-004; hoàn tác thay đổi bằng git; không `git add` các tệp đó |
| `CODEINTEL_TIMEOUT` nhiều | `orca_codeintel_agent_queue_wait_seconds`, RSS dev server, số `gitnexus` đồng thời | giảm tải; kiểm dev server thiếu RAM (mỗi `gitnexus` 0,6 đến 0,85 GB); tăng RAM hoặc giảm hạn mức đồng thời |
| Trôi định dạng (`CodeIntelToolFormatDrift`) hoặc `incompatible` | `orca_codeintel_tool_info{version}`, `tool_compat`; phiên bản công cụ vừa tự nâng cấp? (`~/.codegraph/versions/`) | hạ về phiên bản đã hỗ trợ; hoặc chụp fixture mới và nâng `SUPPORTED_*` (CR-CV-070) |
| `CODEINTEL_REPO_NOT_REGISTERED` | `gitnexus list`; `~/.gitnexus/registry.json` có đường dẫn **chính xác** của worktree không | reindex worktree đó (đăng ký đúng đường dẫn); worktree lồng trong repo cha **không** dùng index của repo cha |
| `CODEINTEL_DISABLED` | `codeIntel.settings.get`; `CODE_INTEL_ENABLED` của service | bật cờ theo mục 2.1 |
| Push không tới UI | `orca_codeintel_events_total{result="dropped"}`, `orca_gateway_codeintel_streams_active`, NATS, `orca_codeintel_outbox_pending` | kiểm infra-fleet stream, outbox; UI vẫn có `codeIntel.reindexStatus` để hỏi tay |
| Dev server offline | trạng thái agent trong Orca; backend chỉ xếp hàng ~20 s (README v7 mục 7) | khởi động lại `orca-agent`; dữ liệu cache cũ vẫn hiển thị với `stale` |
| Nghi rò dữ liệu | audit đọc `GetSymbol`; log; canary | tắt cờ ngay (2.7), thu thập log, báo bảo mật |
| Khẩn cấp | | tắt cờ (mục 2.7) |

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Cờ thi hành ở `code-intel-service`, không ở gateway | Một điểm; WS, MCP, push đều theo |
| D2 | Tắt cờ: chặn đọc/ghi, cho `GetSettings`, giữ dữ liệu, cho job đang chạy hoàn tất | Không giam dữ liệu, không để lại `analyze` nửa chừng |
| D3 | Fail closed | Thiếu dòng hay lỗi đọc không được vô tình bật |
| D4 | T1 và T2 chặn PR; T3, T4 không | Công cụ ngoài và stack đầy đủ không ổn định |
| D5 | Kịch bản bao phủ mọi RPC (`TestEveryRPCHasScenario`) | Thêm RPC mà quên e2e thì CI đỏ |
| D6 | Rollback không dùng `down` migration | Tránh mất dữ liệu review |
| D7 | Cờ theo tenant, không theo lens | Đơn giản; lens theo bản frontend |
| D8 | Mặc định `false` kể cả tenant mới tới GA | An toàn; khác `MCP_TENANT_DEFAULT_ENABLED` |
| D9 | E2E T3 khẳng định cây làm việc không đổi sau reindex | `analyze` mặc định ghi vào `AGENTS.md`/`CLAUDE.md` (mục 1.5) |
| D10 | Dữ liệu fake backend lấy từ tệp vàng C2 | Một nguồn sự thật cho dữ liệu mẫu (CR-CV-070) |

## 4. Tiêu chí chấp nhận

- [ ] Bảng `codeintel.tenant_settings` và hai RPC cờ có trên Postgres và MySQL; không có dòng thì `GetSettings` trả `enabled=false`.
- [ ] Với `CODE_INTEL_ENABLED=false`, mọi RPC ngoài `GetSettings` trả `CODEINTEL_DISABLED`; biến bật nhưng tenant chưa bật cũng vậy; bật cả hai thì chạy.
- [ ] Tắt cờ giữa chừng: job reindex đang chạy hoàn tất, consumer sự kiện và outbox vẫn chạy, dữ liệu review còn nguyên sau khi bật lại.
- [ ] `SetSettings` từ chối người không phải admin và ghi audit.
- [ ] E01 đến E18 chạy xanh ở T1 trên Postgres và MySQL (những ca đánh dấu T1); `TestEveryRPCHasScenario` xanh; E05 khẳng định cây làm việc không đổi.
- [ ] Các spec Playwright ở 2.4 xanh ở T2; spec cờ khẳng định không mở stream khi tắt.
- [ ] `check-code-intel-service-wiring.sh` xanh và nằm trong CI; xoá `codeintel` khỏi `migrate.sh` thì script đỏ.
- [ ] Workflow `code-intel-e2e.yml` chạy được qua `workflow_dispatch` (chưa kiểm chứng stack dev chạy trong CI).
- [ ] `docs/guides/code-intel/` có đủ các tệp ở 2.8; hướng dẫn cài ghi rõ `--index-only` và `PATH` của systemd.
- [ ] Quay lui được diễn tập một lần ở dev (tắt cờ, cắt ở gateway, bật lại), kết quả ghi vào runbook.
- [ ] Không thêm `max-lines` disable (AGENTS.md); mọi chuỗi UI qua `translate()`, không hex cứng trong test component.

## 5. Kiểm thử

Bảng test là chính các mục 2.2 đến 2.4. Bổ sung:

| Test | Nội dung |
|---|---|
| `e2e/feature_flag_test.go` (mới) | E12, bảng 2.1, cô lập tenant |
| `internal/adapter/grpc/feature_gate_test.go` (mới) | interceptor theo từng RPC; fail closed khi lỗi đọc |
| `tenant_settings` repo test hai dialect | đọc, ghi, mặc định |
| `tests/code-intel/check_code_intel_flow.py` (mới) | T3: E01 đến E05, E07, E15 qua `/ws` |
| `tests/code-intel/check_code_intel_flag.py` (mới) | bật/tắt qua `codeIntel.settings.*` trên stack thật |
| `check-code-intel-service-wiring.sh` | script 2.5 |

Chưa chạy: toàn bộ danh sách trên là kế hoạch; mọi số ngày, ngưỡng là giả định.

## 6. Rủi ro và điểm chưa kiểm chứng

- Chưa kiểm chứng stack `deploy/dev` chạy được trong runner CI, và agent thật + công cụ thật trong CI; T3 do đó không chặn.
- Chưa biết cách cài CodeGraph không tương tác (bộ cài, gói); phần cài trong runbook của nó là mô tả quan sát, không phải công thức.
- Thời gian dựng chỉ mục Orca (`gitnexus analyze`, `codegraph index`) chưa đo; ảnh hưởng ngân sách reindex và việc "chạy hết" khi tắt cờ.
- Chưa chạy `gitnexus analyze --index-only` để xác nhận hoàn toàn không đụng tệp nào trong cây làm việc (chỉ đọc `--help`: "skip all file injection (AGENTS.md, CLAUDE.md, skills)"); e2e E05 là kiểm chứng.
- `~/.gitnexus/registry.json` là trạng thái toàn cục theo `HOME`; hai người dùng khác nhau trên cùng dev server (hai `HOME`) có registry riêng; chưa kiểm chứng hành vi khi nhiều agent cùng `HOME`.
- Chưa rõ cơ chế tự nâng cấp CodeGraph (thư mục `~/.codegraph/versions/`); phiên bản trên dev server có thể trôi sau khi ta xác nhận phiên bản.
- Fake backend cho Playwright có thể lệch backend thật; `@dev-stack` và T3 là lưới bù, T2 chặn PR nhưng không chứng minh tích hợp.
- Dữ liệu e2e và dữ liệu mẫu có thể chứa mã nguồn của repo thật nếu dùng T4 (không có trong git).
- Hiệu lực tắt cờ ≤ 5 giây là giả định theo thiết kế bộ đệm; chưa có test thời gian thực.
- SSH và remote: e2e không giả định thực thi cục bộ; dev server qua SSH (`relay-ssh`, CR-CV-006) ngoài phạm vi; đường ghi chỉ qua RPC nên giữ hành vi SSH hiện có.

## 7. Câu hỏi mở

1. Có chấp nhận tầng T3, T4 không chặn PR và chạy hằng đêm trên runner chuyên dụng không (cần một máy lưu chỉ mục Orca)?
2. Cần RPC huỷ reindex không? README v7 3.6 không có; hiện chỉ có "chạy tới hết".
3. Mặc định cờ cho tenant mới ở GA: giữ `false` hay `true` như `MCP_TENANT_DEFAULT_ENABLED`? Cần quyết định sản phẩm.
4. Playwright: tái dùng biến `MCP_E2E_BASE_URL` hay thêm `CODE_INTEL_E2E_BASE_URL`; chuyển hỗ trợ dùng chung ra khỏi thư mục `mcp-web`?
5. Có đưa cờ vào `tenant-service` (cài đặt phân lớp) để frontend đọc chung với các cài đặt khác thay vì kênh riêng không? CR này chọn bảng ở `code-intel-service` để một chủ sở hữu.
6. README v7 cần thêm bảng `tenant_settings` (3.5), `GetSettings`/`SetSettings` (3.6), `codeIntel.settings.*` và `codeIntel.subscribe` (3.7).
7. `codeintel.reindex` phải truyền `--index-only` (CR-CV-004): cần cập nhật CR đó; có truyền thêm `--workers` để giới hạn CPU không?

## 8. Tham chiếu

- `backend-go/services/api-gateway/internal/config/config_mcp.go`, `backend-go/services/mcp-service/internal/domain/tenant_settings.go`, `backend-go/services/mcp-service/README.md` (dòng 35, 73, 117), `backend-go/proto/orca/mcp/v1/mcp.proto`
- `deploy/dev/docker-compose.yml` (dòng 65, 71, 576), `deploy/dev/scripts/migrate.sh`, `backend-go/deploy/postgres-init-databases.sh`, `backend-go/Makefile`, `backend-go/go.work`, `backend-go/ci/check-opa-bundle-in-images.sh`, `.github/workflows/backend-go-task-service.yml`, `.github/workflows/backend-go-issue-status-sync.yml`, `.github/workflows/e2e.yml`
- `tests/playwright.web.config.ts`, `tests/playwright.config.ts`, `tests/e2e/AGENTS.md`, `tests/e2e/mcp-web/rollout.web.e2e.ts`, `tests/e2e/mcp-web/support/mock-orca-ws.ts`, `frontend/src/renderer/src/test-support/mcp-fake-backend.ts`, `tests/mcp/README.md`, `tests/mcp/mcp_check_framework.py`, `tests/backend/check_framework.py`, `package.json` (script `test:e2e:mcp-web`)
- `deploy/agent/orca-agent.service`, `deploy/agent/agent-runtime.env.example`, `deploy/agent/README.md`, `agent/src/relay/agent-tool-registry.ts`
- `gitnexus analyze --help` (cờ `--index-only`, `--skip-agents-md`, `--workers`, `--name`), `~/.gitnexus/registry.json`, `.gitnexus/meta.json`, `AGENTS.md` (dòng "npm 11 crash")
- `docs/research/view-code/06-gaps-risks-roadmap.md` mục 2 và 3; `docs/crs/v7/README.md` O3, O8, mục 5 và 6; `docs/crs/v6/request-quality-rollout/CR-REQ-025-e2e-tests-feature-flag-rollout.md`; `docs/crs/v5/mcp-quality-rollout/CR-MCP-015-conformance-e2e-observability-rollout.md`
- `docs/crs/v7/quality-rollout/CR-CV-070-golden-fixtures-and-tool-contract-tests.md`, `CR-CV-071-performance-budgets-metrics-tracing.md`, `CR-CV-072-security-tests.md`; `docs/crs/v7/code-intel-gateway/CR-CV-040-api-gateway-codeintel-channels.md`
