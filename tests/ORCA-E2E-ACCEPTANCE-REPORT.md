# Báo cáo nghiệm thu e2e các luồng ORCA

**Ngày chạy:** 2026-10-09 · **Mã nguồn:** `main` tại `98e451354` cộng các thay đổi chưa commit liệt kê ở mục 7 · **Bộ test:** [`stages/e2e/orca-flows`](./stages/e2e/orca-flows/README.md)

## 1. Kết luận

| | Kết quả |
|---|---|
| Lượt chạy cuối | **85 pass, 1 fail, 5 blocked, 1 skip** (92 test, 33 giây) |
| Lỗi sản phẩm tìm được | 9: **8 đã sửa** (D1 đến D8, gồm cả lỗi cấu hình và build), **1 còn mở** (D9) |
| Bộ e2e T1 của `request-service` (Go, testcontainers) | từ **36 FAIL** (chạy tập con kịch bản, cờ và ma trận loại) xuống **0 FAIL** ở toàn gói (70 PASS, 49 stage tự skip, xem mục 6) |
| Có nghiệm thu được chưa? | **Chưa.** Còn 1 lỗi mở (`RQL-05`) và 5 test `BLOCKED` do môi trường chưa có API key AI và MCP đang tắt. Luồng Request chưa được kiểm tra qua các bước dùng AI (Solution, Plan, Phase, thực thi) |

Điều quan trọng nhất: **trước lượt này, một Request không thể đi qua bước xác nhận loại bằng API công khai.** Hệ thống tự mở một Clarification bắt buộc (tiêu chí chấp nhận), nhưng gateway không có kênh nào để trả lời. Đây là lỗi D1 ở mục 4, đã sửa. Bộ test hiện có `RQL-14` đến `RQL-17` và 11 test ma trận loại để giữ cho lỗi này không quay lại.

## 2. Phạm vi và thiết kế

Bộ test đi qua đúng đường UI dùng (đăng nhập cookie, rồi kênh `invoke` trên WebSocket `/ws`), không mock service nào, và chạy trên stack Docker local với **máy của người dùng làm dev server** (agent thật, repo git tạm).

| Suite | Số test | Luồng được kiểm tra |
|---|---|---|
| `platform` | 11 | UI, đăng nhập, WS ẩn danh bị chặn, container, gateway tới `request-service` và `workflow-service` |
| `devserver` | 7 | đăng ký, duyệt, kết nối dev server; tạo project gắn dev server (agent xác thực đường dẫn) |
| `workflow` | 13 | DAG: thứ tự wave, fan-out/fan-in, điều kiện, định nghĩa không hợp lệ, trạng thái cuối, bước `shell` trên dev server |
| `request` | 19 | cờ luồng, tạo Request và idempotency, phân loại, xác nhận loại, đồng thời, Clarification, giới hạn tốc độ |
| `matrix` | 13 | mỗi trong 11 loại Request: trạng thái đầu, đường đi, cổng, quy tắc Phase |
| `exits` | 11 | trả backlog, mở lại, huỷ, đổi loại, Request con, trạng thái cuối |
| `ai` | 6 | các bước dùng AI: thất bại an toàn khi không có AI, tạo được Solution/Plan khi có AI |
| `task` | 12 | CRUD task, cây con, thực thi agent (tuỳ chọn), MCP |

Quyết định thiết kế đáng chú ý:
- **Bốn trạng thái kết quả** (`PASS`, `FAIL`, `BLOCKED`, `SKIP`) để không lẫn lỗi sản phẩm với thiếu môi trường.
- **Oracle độc lập**: bảng 11 loại Request lấy từ README v6 mục 3.4 và CR-REQ-003, không sinh từ registry Go.
- **Đi qua cổng công khai**: nhờ vậy bắt được lỗi "RPC có nhưng gateway không mở".
- **Tự dọn dữ liệu** và **tự giãn nhịp** khi bị giới hạn tốc độ ghi.

## 3. Kết quả lượt chạy cuối

| Suite | PASS | FAIL | BLOCKED | SKIP |
|---|---|---|---|---|
| platform | 11 | 0 | 0 | 0 |
| devserver | 7 | 0 | 0 | 0 |
| workflow | 13 | 0 | 0 | 0 |
| request | 17 | **1** | 1 | 0 |
| matrix | 13 | 0 | 0 | 0 |
| exits | 11 | 0 | 0 | 0 |
| ai | 4 | 0 | 2 | 0 |
| task | 9 | 0 | 2 | 1 |
| **Tổng** | **85** | **1** | **5** | **1** |

### Test chưa pass

| ID | Trạng thái | Lý do | Cần gì để thành PASS |
|---|---|---|---|
| `RQL-05` | **FAIL** | Request được tạo trong project không tồn tại | sửa lỗi D9 (mục 5) |
| `RQL-09` | BLOCKED | bộ phân loại AI báo `classifier error` | `ANTHROPIC_API_KEY` trong môi trường của agent |
| `AIS-02` | BLOCKED | sinh Solution bằng AI | như trên |
| `AIS-04` | BLOCKED | sinh Plan bằng AI | như trên |
| `MCP-01`, `MCP-02` | BLOCKED | `MCP_ENABLED` không phải `true` trong `deploy/dev/.env` | bật MCP |
| `TSK-08` | SKIP | tuỳ chọn, tốn lượt dùng AI | `ORCA_E2E_EXECUTE_AGENT=1` |

Các test `RQL-09`, `AIS-02`, `AIS-04`, `MCP-01`, `MCP-02`, `TSK-08` **chưa từng được chạy thật**. Kỳ vọng của chúng viết theo tài liệu và hợp đồng, nên có thể cần chỉnh khi chạy lần đầu.

## 4. Lỗi sản phẩm đã sửa (chặn hoặc làm hỏng luồng)

| # | Lỗi | Hậu quả | Sửa ở đâu | Test giữ |
|---|---|---|---|---|
| D1 | `clarification.list/get/answer/cancel` **không có kênh nào ở gateway** dù frontend đã gọi và `request-service` đã cài RPC | Mọi Request dừng vĩnh viễn ở `awaiting_information`; không loại Request nào đi tiếp được | thêm `wscompat/channels_request_clarification.go` (+ view camelCase đúng định dạng parser frontend), đăng ký trong `channels_request.go`, loại khỏi MCP tại `excluded_channels.yaml` (Clarification là việc của người) | `RQL-14..17`, `MTX-01..13`; unit test `channels_request_clarification_test.go`, `TestRequestChannels_Inventory` |
| D2 | `request-service` và gateway **không có token nội bộ chung**; compose thiếu mount bundle OPA cho `request-service` | `REQUEST_FORBIDDEN` cho mọi lời gọi; `request-service` crash-loop | `deploy/dev/docker-compose.yml` (`GATEWAY_INTERNAL_TOKEN`, `REQUEST_INTERNAL_CALLER_TOKEN`, mount và `OPA_BUNDLE_PATH`); token sinh ngẫu nhiên trong `.env` | `PLT-10`, `PLT-09` |
| D3 | `project-service` gọi `infra-fleet` **không gắn tenant** (health check và relay) | `project.create` với dev server luôn lỗi `INFRA_NO_TENANT` | `dev_server_health_checker.go`, `dev_server_relay.go` | `DEV-04..06` |
| D4 | Agent **không có RPC `fs.checkPath`** mà `project-service` gọi để kiểm tra đường dẫn repo | không tạo được project trên dev server thật | `agent/src/relay/fs-agent-extensions.ts`, `agent-rpc-dispatch-fs.ts` | `DEV-04`, `DEV-05` |
| D5 | `build-local.sh` **xoá rồi tạo lại** `dist/` và `policy/` | container đang chạy mount thư mục cũ: UI trả 403, mọi lời gọi có OPA trả `REQUEST_FORBIDDEN` cho tới khi restart | đổi sang `rsync -a --delete` giữ nguyên thư mục | `PLT-01`, `PLT-10` (đã xác nhận: build xong không cần restart) |
| D6 | `task.create` ở gateway **không truyền `CreatorId`** | task-service không cấp grant `owner`; `task.getSubtree` trả rỗng cho cả người tạo | `wscompat/channels.go` | `TSK-07` |
| D7 | Xoá task không tồn tại trả **`Internal`** thay vì `NotFound` | client không phân biệt được lỗi người dùng với lỗi hệ thống | repo Postgres và MySQL trả `domain.ErrTaskNotFound`; `delete_task.go` ánh xạ `NotFound`; test usecase kiểm `KindNotFound` | `TSK-10` |
| D8 | Bước `shell` của workflow với `target: project:<id>` lỗi **`PROJECT_NO_USER`** | workflow không chạy được lệnh trên dev server qua project (cách F36 mô tả) | `execute.go` đưa `UserID` vào context nền; `serverresolver/resolver.go` chuyển sang `project-service` | `WFL-13` |

Trước khi chạy bộ test, mình đã sửa D2 đến D5 và D1 trong lúc dựng demo, nên lượt chạy đầu tiên của bộ test đã thấy chúng được sửa. Bằng chứng "trước khi sửa" của D1 và D2 nằm ở phiên làm việc (Request #1 đến #3 kẹt ở `awaiting_information`, `REQUEST_FORBIDDEN` ở `request.flowStatus`), không phải ở một lượt chạy bộ test.

## 5. Lỗi sản phẩm còn mở

| # | Lỗi | Bằng chứng | Gợi ý sửa |
|---|---|---|---|
| **D9** | Tạo Request trong **project không tồn tại** vẫn thành công (`RQL-05`) | `request.create` với `projectId` hợp lệ về mặt UUID nhưng không có thật trả `created: true` | `CreateRequest` chỉ kiểm định dạng UUID; admin bỏ qua kiểm tra vai trò. Thêm cổng kiểm tra tồn tại. **Mình đã thử `GetProject`, không dùng được:** `project-service` trả `PermissionDenied` cho cả project không tồn tại lẫn project không có quyền (không phân biệt được), nên đã hoàn tác. Cần một RPC kiểm tra tồn tại ở `project-service` (đổi proto). Trong lúc chờ, chặn ở tầng gateway bằng `project.list` của chính người gọi |

Quan sát khác (chưa có test, không tính là lỗi đã xác nhận):
- `project.get` với id không tồn tại trả `Internal: PROJECT_MEMBERSHIP_LOOKUP_FAILED` thay vì `NotFound`. Cùng gốc với D9.
- Câu hỏi `type_fields.time_box_hours` của loại `spike` có `kind: text` nhưng chỉ nhận chuỗi chứa số (`"8"`). Câu trả lời dài bị từ chối với lỗi nói về kiểu `number`. Test `MTX-13` pass vì dùng `"8"`; UI không có gợi ý về định dạng.
- Giới hạn ghi của `request-service` là 5 lần/giây (burst 10), tài liệu ghi "chưa đo, đề xuất ban đầu". Một lượt chạy bộ test bị chặn 10 lần dù đã giãn nhịp. Nên đo bằng lưu lượng thật trước khi bật cho người dùng (xem CR-REQ-035).

## 6. Lỗi của bộ kiểm thử có sẵn và của chính bộ mới

**Bộ e2e T1 của `request-service`** (`backend-go/services/request-service/e2e`, 36 FAIL → 0 FAIL): các kịch bản chờ trạng thái `analyzing` ngay sau xác nhận loại, nhưng CR-REQ-028 chèn Clarification bắt buộc ở giữa. Đã sửa `kit.confirm` để tự trả lời Clarification, và đổi kỳ vọng metric chuyển trạng thái thành `awaiting_information → analyzing`. Kết quả: 70 PASS, 0 FAIL. **49 stage vẫn tự skip** với nhãn "implemented now, extend this stage" (Solution, Plan, `ReportTaskOutcome`, `CommitPlan`): RPC đã có nhưng stage chưa được viết (TASK-REQ-025-04). Đây là việc còn lại, không phải lỗi.

**Bộ mới**: lượt chạy đầu có 23 FAIL, trong đó phần lớn là lỗi của chính test (đã sửa): đường dẫn bundle tương đối, `cookiejar` của Python bỏ cookie `localhost`, cấu hình bước `shell` sai (`target` thay vì `connectionId`), tên tham số `rootId`, kỳ vọng sai về phát lại idempotent và về lịch sử đổi loại, quy tắc đổi loại (chỉ được mở rộng), sinh Solution là bất đồng bộ, và giới hạn tốc độ ghi làm hỏng hàng loạt test phía sau. Chỉ một số ít là lỗi sản phẩm thật: `RQL-05`, `TSK-07` (D6), `TSK-10` (D7), và D8, lộ ra sau khi sửa cấu hình của `WFL-12`.

## 7. Thay đổi mã trong lượt này (chưa commit)

| Khu vực | File |
|---|---|
| Gateway | `wscompat/channels_request_clarification.go` (mới), `channels_request_clarification_test.go` (mới), `channels_request.go`, `channels_request_test.go`, `channels.go`, `mcpserver/tools/excluded_channels.yaml` |
| project-service | `adapter/grpcclient/dev_server_health_checker.go`, `dev_server_relay.go` |
| task-service | `adapter/postgres/repository.go`, `adapter/mysql/repository.go`, `usecase/delete_task.go`, `delete_task_test.go`, `fakes_test.go` |
| workflow-service | `usecase/execute.go`, `adapter/serverresolver/resolver.go` |
| request-service | `e2e/kit_test.go`, `e2e/observability_test.go` |
| Agent | `src/relay/fs-agent-extensions.ts`, `agent-rpc-dispatch-fs.ts` |
| Hạ tầng | `deploy/dev/docker-compose.yml`, `deploy/dev/scripts/build-local.sh` |
| Bộ test | `tests/stages/e2e/orca-flows/` (mới), file báo cáo này |

Ngoài ra có các thay đổi chưa commit **không phải của lượt này** (đang có sẵn trong working tree): `task-service/cmd/server/main.go`, `deploy/dev/docker/git-gateway-runtime.Dockerfile`, `package.json`, `tests/shared/orca_route_catalog.py`, `deploy/dev/docker-compose.local.yml`. Conflict marker của lần `git stash pop` trước đã được giải quyết (còn `stash@{0}`).

`go test ./...` của 5 service chạm vào (api-gateway, task-service, workflow-service, project-service, request-service) đều pass. **Chưa chạy** `gitnexus impact` và `detect_changes` theo AGENTS.md, vì công cụ này không có trong phiên làm việc. Cần chạy trước khi commit.

## 8. Chưa được kiểm tra

- **Các bước dùng AI của Request**: phân loại có đề xuất, Solution, Plan, Phase, duyệt, thực thi (`StartPhase`, `ReportTaskOutcome`), đồng bộ Jira. Đây là phần lớn giá trị của luồng, nhưng cần API key AI trong môi trường của agent.
- **Giao diện (UI)**: các suite Playwright `request-web`, `code-intel-web` đã có sẵn trong `tests/stages/e2e/legacy`, mình chưa chạy. Bộ mới chỉ kiểm tra UI được phục vụ.
- **Phân quyền nhiều người dùng**: chỉ dùng tài khoản admin. Chưa kiểm tra người không phải admin, thành viên project, hoặc người duyệt khác người tạo.
- **Nguồn tiếp nhận ngoài thủ công**: Jira, GitHub, webhook, MCP (kịch bản E17, E18 của CR-REQ-025).
- **MySQL**: bộ mới chỉ chạy trên Postgres.
- **MCP**: đang tắt trong môi trường này.
- **Dev server từ xa**: dev server là chính máy chạy test; đường dẫn repo giả định cùng hệ tệp.

## 9. Kế hoạch sửa và điều kiện nghiệm thu

Đánh giá ảnh hưởng, solution theo nguyên nhân gốc và checklist task nằm ở:
- Backend: [`specs/backend-go/bugs/e2e-acceptance-v1`](../specs/backend-go/bugs/e2e-acceptance-v1/README.md)
- Frontend: [`specs/frontend/bugs/e2e-acceptance-v1`](../specs/frontend/bugs/e2e-acceptance-v1/README.md)

Điều kiện nghiệm thu đề xuất:

1. Cấp `ANTHROPIC_API_KEY` cho agent, bật `MCP_ENABLED=true`, rồi chạy `python3 run_acceptance.py --strict`. Điều kiện qua: **0 FAIL và 0 BLOCKED**.
2. Sửa D9 (`RQL-05`).
3. Bổ sung các stage đang skip của bộ T1 (Solution, Plan, Phase, `ReportTaskOutcome`) và chạy thêm một lượt `ORCA_E2E_EXECUTE_AGENT=1` để kiểm tra `task.execute` thật.
4. Chạy các suite Playwright `request-web` và `code-intel-web` trên cùng stack.
5. Bổ sung kịch bản phân quyền nhiều người dùng và nguồn Jira/GitHub/webhook.

## 10. Dữ liệu thử còn lại trong môi trường

- Template workflow có tên bắt đầu bằng `e2e-` (khoảng 47): không có API xoá template.
- Một số Request thử ở trạng thái chưa kết thúc, tạo từ các lượt thăm dò và lượt chạy đầu (khi dọn dẹp bị giới hạn tốc độ). Các lượt chạy mới đã huỷ hết Request mình tạo.
- Project **Demo (this Mac)** và Request #1 đến #3 của phần demo.
- Agent đang chạy nền trên máy này (`pkill -f agent/out/agent.js` để dừng); token hết hạn sau 1 giờ.

## 9. Cập nhật sau khi triển khai checklist (2026-10-09)

Chạy lại bộ `orca-flows` trên cùng stack sau khi sửa: **103 PASS, 0 FAIL, 4 BLOCKED, 1 SKIP** (460 giây, Claude thật qua phiên đăng nhập CLI của dev server, agent không có khoá API).

| Trước | Sau |
|---|---|
| `RQL-05` FAIL (Request tạo được trong project không tồn tại) | PASS: `ResolveProjectAccess` + `REQUEST_PROJECT_NOT_FOUND` |
| `AIS-02` FAIL | PASS: lỗi nằm ở test (không chờ run bất đồng bộ); Solution sinh bằng Claude thật |
| `AIS-04` FAIL (`REQUEST_NOT_IMPLEMENTED`) | PASS: giai đoạn Plan được cài đủ (SOL-V2-001) |
| `RQL-09`, `RQL-14..17` BLOCKED/FAIL | PASS |
| Không có suite | Suite mới `notfound` (NFC-01..08) và `plan` (PLN-01..08), đều PASS |

Còn BLOCKED (không phải lỗi sản phẩm): `MCP-01/02` (`MCP_ENABLED` tắt trong `deploy/dev/.env`), `AIS-01/03` (đường "không có AI" không dựng được khi dev server có phiên Claude). SKIP: `TSK-08`.

Lỗi mới tìm thấy khi chạy lại và đã sửa nằm ở [bugs/e2e-acceptance-v2](../specs/backend-go/bugs/e2e-acceptance-v2/README.md). Trạng thái từng task: [v1](../specs/backend-go/bugs/e2e-acceptance-v1/tasks/README.md), [v2](../specs/backend-go/bugs/e2e-acceptance-v2/tasks/README.md).
