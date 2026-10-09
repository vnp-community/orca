# Đánh giá ảnh hưởng của các bug phát hiện qua e2e nghiệm thu

**Nguồn:** [`tests/ORCA-E2E-ACCEPTANCE-REPORT.md`](../../../../tests/ORCA-E2E-ACCEPTANCE-REPORT.md) (lượt chạy 2026-10-09, `main` tại `98e451354` cộng thay đổi chưa commit) và bộ test [`tests/stages/e2e/orca-flows`](../../../../tests/stages/e2e/orca-flows/README.md).
**Phạm vi tài liệu:** đánh giá ảnh hưởng, gom nguyên nhân gốc. Giải pháp ở [`solutions/`](./solutions/README.md), checklist ở [`tasks/`](./tasks/README.md). Phần giao diện ở [`specs/frontend/bugs/e2e-acceptance-v1`](../../../frontend/bugs/e2e-acceptance-v1/IMPACT-ASSESSMENT-FRONTEND.md).

> **Trung thực về bằng chứng.** Mức ảnh hưởng dưới đây suy ra từ mã, log và kết quả test đã chạy trên một stack local một người dùng (admin), một tenant, Postgres. Chưa có số đo lưu lượng thật. Chỗ nào là suy luận được ghi "suy luận".

## 1. Thang đánh giá

| Mức | Nghĩa |
|---|---|
| **Critical** | Chặn hoàn toàn một luồng chính cho mọi người dùng, không có đường vòng bằng công cụ công khai |
| **High** | Chặn một luồng quan trọng trong điều kiện phổ biến, hoặc gây gián đoạn vận hành khó chẩn đoán |
| **Medium** | Sai dữ liệu hoặc hành vi trong tình huống có điều kiện; luồng chính vẫn chạy |
| **Low** | Sai mã lỗi, trải nghiệm, hoặc chỉ ảnh hưởng chẩn đoán |

Mỗi bug được chấm theo: phạm vi bị chặn, ai bị ảnh hưởng, tính toàn vẹn dữ liệu, bảo mật, khả năng phát hiện (lỗi im lặng nguy hiểm hơn lỗi ồn ào), đường vòng, và trạng thái hiện tại của bản sửa.

## 2. Danh mục bug

| ID | Bug | Mức | Trạng thái mã hiện tại |
|---|---|---|---|
| BUG-E2E-001 | Gateway không có kênh `clarification.*` | **Critical** | Đã sửa trong working tree (chưa commit, có unit test) |
| BUG-E2E-002 | `request-service` và gateway thiếu token nội bộ chung; compose thiếu mount OPA | **Critical** | Đã sửa trong compose và `.env` local (chưa commit); chưa có kiểm tra tự động khi triển khai |
| BUG-E2E-003 | `project-service` gọi `infra-fleet` không gắn tenant | High | Sửa cục bộ 2 adapter (chưa commit, chưa có unit test) |
| BUG-E2E-004 | Agent thiếu RPC `fs.checkPath` | High | Đã thêm (chưa commit, chưa có test) |
| BUG-E2E-005 | `build-local.sh` làm mất mount của container đang chạy | High | Đã sửa bằng `rsync` (chưa commit) |
| BUG-E2E-006 | Gateway không truyền `CreatorId` khi tạo task | Medium | Đã sửa ở gateway (chưa commit, chưa có test); **dữ liệu cũ chưa được vá** |
| BUG-E2E-007 | Xoá task không tồn tại trả `Internal` | Low | Đã sửa (có unit test, chưa commit) |
| BUG-E2E-008 | Bước workflow nền không mang danh tính người chạy | High | Sửa cục bộ (chưa commit, chưa có test); **chạy lại sau khi khởi động lại vẫn mất danh tính** |
| BUG-E2E-009 | Tạo Request trong project không tồn tại thành công | Medium | **Còn mở** (thử sửa bằng `GetProject` không dùng được, đã hoàn tác) |
| BUG-E2E-010 | Không có đường cấp khoá AI cho `ai.complete` | **Critical** cho mọi bước AI | **Còn mở**; hướng xử lý đã chốt: dùng phiên đăng nhập CLI của dev server (SOL-E2E-005) |
| OBS-A | `project.get` với id lạ trả `Internal`, không phải `NotFound` | Low | Còn mở (cùng gốc với 009) |
| OBS-B | Câu hỏi `kind: text` nhưng chỉ nhận chuỗi số (`time_box_hours`) | Low | Còn mở |
| OBS-C | Giới hạn ghi 5/giây, burst 10, "chưa đo" | Medium (suy luận) | Còn mở |
| OBS-D | 49 stage e2e của Go tự skip; tài liệu `guides/request` lạc hậu | Low | Còn mở |

## 3. Đánh giá chi tiết

### BUG-E2E-001 — Gateway thiếu `clarification.list/get/answer/cancel` (Critical)

- **Cơ chế:** sau khi người dùng xác nhận loại, `request-service` tự mở một Clarification bắt buộc (tiêu chí chấp nhận, và các trường riêng theo loại như `time_box_hours`, `data_exposed`) rồi đưa Request sang `awaiting_information`. `RequestService` đã cài `ListClarifications/GetClarification/AnswerClarification/CancelClarification`, frontend (`useClarifications.ts`) đã gọi `clarification.*`, nhưng `api-gateway` không đăng ký kênh nào.
- **Phạm vi bị chặn:** **100% Request** đi qua xác nhận loại. Cả 11 loại, cả khi AI chạy tốt. Mọi giai đoạn sau (`analyzing`, `planning`, `executing`, `completed`) không thể đạt được qua API công khai. Đây là điểm nghẽn của toàn bộ tính năng Request v6.
- **Ai bị ảnh hưởng:** mọi người dùng, mọi tenant bật cờ luồng Request.
- **Dữ liệu:** Request tích luỹ ở `awaiting_information` đến hạn (`dueAt` +7 ngày, theo dữ liệu quan sát) rồi hết hạn; tính vào hạn mức `OpenRequestsPerTenant = 2000`, tức nếu không ai trả lời được thì tenant tự chạm trần.
- **Khả năng phát hiện:** **thấp, im lặng.** Frontend coi lỗi `unsupported` của `clarification.list` là "không có Clarification" (`useClarifications.ts` bỏ qua lỗi này). Người dùng thấy Request dừng ở trạng thái "chờ thông tin" mà không có câu hỏi nào để trả lời.
- **Đường vòng:** không có (admin cũng không). Chỉ gọi gRPC trực tiếp bên trong mạng nội bộ.
- **Vì sao lọt:** bộ test Go T1 và test frontend đều dùng mock; không test nào đi qua gateway thật. Không có kiểm tra "mọi RPC công khai đều có kênh".
- **Rủi ro còn lại sau bản sửa tạm:** chưa commit; chưa có kiểm tra tự động ngăn tái phát cho các RPC khác (xem mục 4, nhóm RC-A).

### BUG-E2E-002 — Thiếu token nội bộ và mount OPA cho `request-service` (Critical)

- **Cơ chế:** `request-service` yêu cầu `GATEWAY_INTERNAL_TOKEN` và `SERVICE_INTERNAL_TOKEN`, rỗng thì **từ chối mọi RPC** (chỉ ghi log mức ERROR, vẫn khởi động). Compose không truyền `GATEWAY_INTERNAL_TOKEN`, gateway không nhận `REQUEST_INTERNAL_CALLER_TOKEN`. Block `request-service` cũng thiếu mount bundle OPA nên service **crash-loop** (`stat ../../policy/orca-authz`).
- **Phạm vi:** toàn bộ `request-service` không dùng được trên bất kỳ môi trường dựng từ compose hiện tại. Cả `issue-status-sync` (cần `SERVICE_INTERNAL_TOKEN`).
- **Khả năng phát hiện:** trung bình. Crash-loop thì ồn ào; trường hợp token rỗng thì lỗi trông như `REQUEST_FORBIDDEN` (cố ý không cho biết lý do), dễ nhầm với lỗi phân quyền.
- **Bảo mật:** hướng an toàn (fail-closed), không có lỗ hổng. Điểm cần chú ý là hành vi "rỗng thì tắt guard" ở các service khác dùng cùng cơ chế `internalcaller`: nếu một service coi token rỗng là "không kiểm tra" thì lỗi cấu hình sẽ mở cổng (suy luận; cần rà từng service, xem SOL-004).
- **Vì sao lọt:** compose không dùng cú pháp bắt buộc (`${VAR:?}`) cho các token này; không có script sinh token như `ensure-mcp-env.sh`.

### BUG-E2E-003 — `project-service` không gắn tenant khi gọi `infra-fleet` (High)

- **Cơ chế:** `IsReachable` và `DevServerRelay.CreateConnection/Relay` gọi `infra-fleet` mà không thêm metadata tenant. `infra-fleet` trả `INFRA_NO_TENANT`. Một số adapter cùng package dùng `withTenantMetadata`, hai adapter này bị sót.
- **Phạm vi:** `project.create` và `project.rebindDevServer` với dev server **luôn lỗi** (`PROJECT_DEV_SERVER_HEALTH_CHECK_FAILED` hoặc `…CONNECTION_FAILED`). Không có project gắn dev server thì không có luồng nào chạy agent: `task.execute`, bước `agent`/`shell` của workflow, các giai đoạn AI của Request đều cần dev server.
- **Khả năng phát hiện:** thấp. Thông báo lỗi ("failed to verify dev server is online, failing closed") che mất nguyên nhân gốc; log mức ERROR nằm ở service khác.
- **Quan hệ:** cùng nhóm nguyên nhân với BUG-006 và BUG-008 (RC-B).

### BUG-E2E-004 — Agent thiếu `fs.checkPath` (High)

- **Cơ chế:** `project-service` gọi relay `fs.checkPath` để xác thực đường dẫn repo; agent trả `Method not found`. Không có hợp đồng nào khai báo danh sách method agent phải hỗ trợ.
- **Phạm vi:** như BUG-003: không tạo được project trên dev server thật. Chỉ lộ ra khi có agent thật; test dùng agent giả của Go không bắt được.
- **Lưu ý:** kéo theo rủi ro triển khai lệch phiên bản: dev server chạy agent cũ (xem `AGENT_VERSION`, `MinAgentVersion`) sẽ lại lỗi sau khi backend nâng cấp.

### BUG-E2E-005 — `build-local.sh` xoá-tạo-lại thư mục được mount (High, vận hành)

- **Cơ chế:** `rm -rf dist && cp -r` và `rm -rf policy && cp -r` đổi inode thư mục. Container đang chạy vẫn mount thư mục đã xoá: nginx trả `403`, mọi service dùng OPA đọc bundle rỗng nên **từ chối mọi yêu cầu** (`REQUEST_FORBIDDEN`, `AUTH_NOT_ADMIN`…).
- **Phạm vi:** mỗi lần build lại trên máy dev/staging dùng cách triển khai này; ảnh hưởng 6 service dùng OPA và frontend.
- **Khả năng phát hiện:** **rất thấp.** Lỗi trông như phân quyền hỏng, không như lỗi triển khai; khỏi tự nhiên sau khi restart. Đã tốn công chẩn đoán hai lần trong một phiên.
- **Sau bản sửa:** thư mục giữ nguyên inode nên không còn lỗi; **mount theo file nhị phân** (`./bin/<svc>/orca`) vẫn cần restart container để nạp binary mới (hành vi bình thường của bind mount file, cần ghi rõ trong quy trình).

### BUG-E2E-006 — `task.create` không truyền `CreatorId` (Medium)

- **Cơ chế:** task-service chỉ cấp grant `owner` cho người tạo khi `CreatorID` khác rỗng. Gateway không truyền. Bảng `task.task_grants` có 0 dòng trong môi trường quan sát.
- **Phạm vi:** `task.getSubtree` (lọc theo từng node) trả rỗng cho cả chủ task; mọi kiểm tra quyền dựa trên grant có thể từ chối người tạo (suy luận: `task.get` không kiểm grant nên không lộ). Ảnh hưởng cây công việc, tiến độ cascade, và Plan/Phase của Request vì chúng là Task.
- **Dữ liệu:** **mọi task đã tạo qua gateway trước bản sửa đều mất quyền sở hữu** và sẽ vẫn rỗng trong `getSubtree` sau khi triển khai bản sửa. Cần backfill.
- **Bảo mật:** theo hướng "từ chối nhầm", không lộ dữ liệu.

### BUG-E2E-007 — Xoá task không tồn tại trả `Internal` (Low)

- **Cơ chế:** repo trả lỗi thường, usecase bọc thành `KindInternal`.
- **Ảnh hưởng:** client không phân biệt được "đã xoá rồi" (nên coi là thành công khi thử lại) với lỗi hệ thống; làm bẩn cảnh báo giám sát (lỗi 5xx giả); phá tính idempotent khi xoá lặp.

### BUG-E2E-008 — Workflow nền mất danh tính người chạy (High)

- **Cơ chế:** `Execute` chạy các wave ở goroutine với `context.Background()` chỉ mang tenant. Bước dùng đích `project:<id>` gọi `project-service.GetProject` (có kiểm thành viên) nên nhận `PROJECT_NO_USER`.
- **Phạm vi:** mọi bước `shell`/`agent`/`notification` nhắm project (cách mô tả trong F36). Đường `server:<id>` không bị. Workflow tự động hoá do `automation-service` kích hoạt có thể hưởng cùng lỗi (suy luận).
- **Phát hiện:** thấp: bước `failed` với thông điệp ở tầng sâu.
- **Rủi ro còn lại sau bản sửa tạm:** danh tính chỉ có khi chạy ngay sau `Execute`. `RecoverExecutions` (khôi phục sau khi service khởi động lại) dựng context không có người dùng nên **tái phát**. Và các chỗ khác dùng mẫu `context.Background()` + tenant cũng có nguy cơ tương tự.

### BUG-E2E-009 — Request vào project không tồn tại (Medium)

- **Cơ chế:** `CreateRequest` chỉ kiểm định dạng UUID. Admin bỏ qua kiểm tra vai trò. `ProjectRoleResolver` coi "project không tồn tại" như "không phải thành viên", và `GetProject` trả `PermissionDenied` cho cả hai trường hợp nên không dùng để kiểm tra tồn tại.
- **Phạm vi:** chỉ admin (hoặc luồng nội bộ/webhook/MCP nếu nguồn không bị kiểm). Request mồ côi không hiện trong danh sách theo project, nhưng **tính vào hạn mức tenant** (`OpenRequestsPerTenant`) và lọt vào số liệu/metric.
- **Bảo mật:** nguy cơ thấp cho người thường (họ bị chặn ở bước vai trò), nhưng nguồn `webhook`/`mcp` cần rà: nếu chúng cũng bỏ qua kiểm tra vai trò thì người ngoài tạo được Request ở project tuỳ ý (suy luận; cần xác minh trong SOL-003).

### BUG-E2E-010 — Không có đường cấp khoá AI cho `ai.complete` (Critical cho các bước AI)

- **Cơ chế:** `request-service` gửi `ai.complete` cho agent chỉ kèm `accountId` (nếu tìm được). Agent chỉ biết khoá từ biến môi trường của chính nó hoặc `resolvedApiKey` do server gửi. **Không có đoạn mã phía server nào điền `resolvedApiKey`** (tìm trong `infra-fleet-service` và `request-service` không thấy). Bảng `ai_provider.accounts` rỗng trong môi trường kiểm thử, nên `ResolveProvider` trả `no_scope_match`.
- **Phạm vi:** phân loại AI (`classifying`), sinh Solution, sinh Plan, chẩn đoán: **4 trong 7 giai đoạn của luồng Request** và `task.aiDecompose`. Khi thiếu khoá, hệ thống "thoái hoá an toàn" (Request vẫn tới `awaiting_type_confirmation`; run sinh Solution `failed` có mã lỗi) nên không mất dữ liệu, nhưng giá trị chính của tính năng không có.
- **Nhầm lẫn của người dùng:** người dùng đăng nhập Claude Code trên máy dev vẫn bị đòi khoá, vì `ai.complete` gọi thẳng HTTP API chứ không dùng CLI. Thông báo hiện tại ("classifier error") không chỉ ra cách khắc phục.
- **Bảo mật:** truyền khoá dạng plaintext qua RPC nội bộ là mẫu đã có (`agent.spawn`); phải bảo đảm không ghi log và chỉ qua kênh agent đã xác thực.

### Quan sát

- **OBS-A:** `project.get` id lạ trả `Internal: PROJECT_MEMBERSHIP_LOOKUP_FAILED`. Sai phân loại lỗi, làm khách hàng tưởng sự cố hạ tầng; cùng gốc BUG-009.
- **OBS-B:** câu hỏi `type_fields.time_box_hours` có `kind: text`, câu trả lời hợp lệ phải là chuỗi số `"8"`; câu dài bị từ chối với lỗi nhắc kiểu `number`. Người dùng không có gợi ý về định dạng. Ảnh hưởng trải nghiệm, nhất là với loại `spike`.
- **OBS-C:** giới hạn ghi 5/giây/burst 10 do tài liệu tự ghi là "chưa đo". Một lượt e2e bị chặn 10 lần dù đã giãn nhịp. Với thao tác hàng loạt (nhập nhiều issue Jira) có thể bị chặn (suy luận).
- **OBS-D:** 49 stage e2e Go còn skip, và `docs/guides/request/README.md` vẫn ghi các giai đoạn Solution/Plan/Phase là "chưa có". Dễ làm người vận hành tin sai về tình trạng thật.

## 4. Gom theo nguyên nhân gốc

Các bug không độc lập. Sửa từng cái riêng lẻ sẽ để lại cùng loại lỗi ở nơi khác.

| Gốc | Mô tả | Bug | Ảnh hưởng gộp |
|---|---|---|---|
| **RC-A** Hợp đồng giữa các tầng trôi lệch, không có kiểm tra tự động | RPC có ở service nhưng thiếu ở gateway; agent thiếu method mà backend gọi; frontend gọi kênh không tồn tại | 001, 004 (và OBS-D) | Luồng chính bị chặn mà test đơn vị vẫn xanh (mỗi tầng test với mock của tầng bên cạnh) |
| **RC-B** Danh tính/tenant không được truyền xuyên các bước (đồng bộ và bất đồng bộ) | metadata gRPC đi ra thiếu tenant/user; tác vụ nền dùng `context.Background()`; gateway không đưa danh tính vào miền | 003, 006, 008 | Lỗi `NO_TENANT`/`NO_USER`/quyền sai ở các bước sâu, thông báo lỗi che nguyên nhân; lặp lại ở mỗi adapter mới |
| **RC-C** Phân loại lỗi và kiểm tra tồn tại không nhất quán | "không có" và "không có quyền" bị gộp; repo trả lỗi thô bị bọc thành `Internal` | 007, 009, OBS-A | Dữ liệu mồ côi, mã lỗi sai, giám sát nhiễu |
| **RC-D** Triển khai/cấu hình mong manh, lỗi hiện ra như lỗi phân quyền | token nội bộ, mount OPA, mount bị vô hiệu sau build | 002, 005 | Ngừng hoạt động khó chẩn đoán sau mỗi lần dựng/triển khai |
| **RC-E** Đường cấp khoá AI chưa được xây hết | `accountId` có, `resolvedApiKey` không có nguồn | 010 | Mọi giai đoạn AI không chạy được ngoài môi trường tự đặt khoá cho agent |
| **RC-F** Bộ kiểm thử và tài liệu chậm hơn mã | e2e Go dựng trước CR-REQ-028; guides lạc hậu; không có e2e xuyên tầng trong CI | OBS-C, OBS-D, và là điều kiện để RC-A phát tác | Lỗi chỉ lộ khi chạy tay trên stack thật |

## 5. Ảnh hưởng theo luồng nghiệp vụ

| Luồng | Trước các bản sửa | Sau các bản sửa tạm (chưa commit) | Rủi ro còn lại |
|---|---|---|---|
| Đăng nhập, UI, hạ tầng | chạy (sau mỗi lần build có thể 403) | chạy, không còn 403 sau build | cần restart khi đổi binary |
| Project gắn dev server | **không chạy** (003, 004) | chạy | agent cũ trên dev server khác; không có test cho hai sửa đổi |
| Workflow DAG, bước `condition` | chạy | chạy | — |
| Workflow bước `shell`/`agent` qua `project:` | **không chạy** (008) | chạy khi chạy lần đầu | mất danh tính khi khôi phục sau khởi động lại |
| Task CRUD | chạy; cây con rỗng; xoá lặp lỗi 5xx | chạy | task cũ vẫn rỗng cây con (cần backfill) |
| Request: tạo, phân loại (không AI), xác nhận loại | **chặn ở Clarification** (001), cần token (002) | chạy tới `planning`/`analyzing` | Request mồ côi (009) |
| Request: Solution, Plan, Phase, thực thi | **không đến được**, và cần AI (010) | đến được; **AI chưa chạy** | 010 mở; stage e2e chưa viết |
| Request: nguồn Jira/GitHub/webhook/MCP | chưa kiểm tra | chưa kiểm tra | không có bằng chứng |

## 6. Quyết định đã chốt (2026-10-09)

| # | Quyết định | Tác động tới kế hoạch |
|---|---|---|
| 1 | `ai.complete` dùng **phiên đăng nhập của dev server** (Claude Code CLI đã đăng nhập) thay cho khoá API | Nguồn mặc định mới ở agent (TASK-E2E-033, 034, 035); phân giải khoá phía server (TASK-E2E-023) hạ xuống tuỳ chọn P2. Hạn mức thuộc tài khoản đăng nhập trên dev server, dùng chung cho mọi người dùng ORCA trên máy đó; điều khoản dịch vụ cho máy dùng chung cần được chủ sản phẩm xác nhận (xem SOL-E2E-005 mục 5) |
| 2 | Thêm RPC nội bộ `ProjectService.ResolveProjectAccess` | Mở khoá TASK-E2E-013, 014, 015 (BUG-E2E-009, OBS-A) |

## 7. Hệ quả cho kế hoạch sửa

1. **Mọi bản sửa tạm đang ở working tree** và nhiều bản chưa có test; việc đầu tiên là biến chúng thành thay đổi có test, có commit, có kiểm tra tự động ngăn tái phát (RC-A, RC-D).
2. **Sửa theo nguyên nhân gốc**, không theo từng bug: một interceptor truyền danh tính cho mọi dial (RC-B), một RPC kiểm tra quyền truy cập project dùng chung cho tồn tại và vai trò (RC-C), một đường cấp khoá AI dùng chung cho mọi bước AI (RC-E).
3. **Có dữ liệu cần vá**, không chỉ mã: task thiếu grant `owner`; Request mồ côi; Clarification quá hạn.
4. **Điều kiện "thông luồng ổn định"**: bộ `orca-flows` chạy `--strict` xanh (0 FAIL, 0 BLOCKED) trên stack có AI giả, chạy tự động trong CI.
