# Kiểm toán mức độ thực thi của solution và task v6 (2026-10-07)

| Trường | Giá trị |
|---|---|
| **Câu hỏi** | Toàn bộ solution và task trong `specs/{backend-go,frontend,agent}/crs/v6/` đã được thực thi xong chưa? |
| **Phương pháp** | 8 agent đối chiếu từng task với code thật (đọc file, tìm symbol, kiểm wiring), chạy build, vet, test thật ở từng service. Tổng hợp bởi người điều phối. Test tích hợp cần DB, NATS, dev server, Jira không chạy được ở môi trường kiểm toán. |
| **Mã nguồn kiểm toán** | `main` tại commit `ea7fe66d6` trở đi (code nằm trong commit `f7c16b6cc`, tác giả binhjax, 2026-10-07) |

## 1. Kết luận

**Chưa xong.** Chỉ khoảng 10% task đủ code thật, còn lại là một phần hoặc chưa làm. Dấu `[x]` ở backend không đáng tin: script `update_v6.js` (ở gốc repo) đã đổi hàng loạt `- [ ]` thành `- [x]` và đổi tiêu đề mọi solution thành "Đã triển khai" trong `specs/backend-go/crs/v6/`, không kiểm chứng gì.

| Khu vực | Task | Đủ | Một phần | Chưa làm | Task ghi `[x]` nhưng chưa Đủ |
|---|---|---|---|---|---|
| Backend: foundation và lifecycle (001 đến 006) | 41 | 4 | 6 | 31 | 37 |
| Backend: analysis, approval, engines (007 đến 010, 026) | 32 | 6 | 14 | 12 | 26 |
| Backend: plan-phase-task và backlog (011 đến 015) | 34 | 1 | 6 | 27 | 33 |
| Backend: artifact, contract, impact (027 đến 030) | 32 | 0 | 2 | 30 | 32 |
| Backend: gateway, rollout, agent backend (016, 017, 024, 025, 033) | 35 | 2 | 4 | 29 | 33 |
| Backend: context, AI governance, security (031, 034, 035) | 25 | 1 | 6 | 18 | 24 |
| **Backend** | **199** | **14** | **38** | **147** | **185** |
| Frontend (018 đến 023, 032, 036) | 54 | 2 | 11 | 41 | 7 |
| Agent (033) | 13 | 10 | 3 | 0 | 3 |
| **Tổng** | **266** | **26 (10%)** | **52 (20%)** | **188 (71%)** | |

Báo cáo chi tiết từng task (bằng chứng file và dòng):
[backend-foundation-lifecycle](./backend-foundation-lifecycle.md), [backend-analysis-approval-engines](./backend-analysis-approval-engines.md), [backend-plan-phase-backlog](./backend-plan-phase-backlog.md), [backend-artifact-contract-impact](./backend-artifact-contract-impact.md), [backend-gateway-rollout-agentcap](./backend-gateway-rollout-agentcap.md), [backend-context-ai-security](./backend-context-ai-security.md), [frontend-request](./frontend-request.md), [agent-capabilities](./agent-capabilities.md).

## 2. Hồi quy cần xử lý ngay (đã xác nhận trên `main`)

| Service | Lỗi | Hậu quả |
|---|---|---|
| `task-service` | `go build` lỗi: `usecase/list_execution_states.go:6` import `common/errx` không tồn tại; `domain/task_type.go:14` khai báo trùng `ErrInvalidTaskType` (đã có ở `task.go:67`); `adapter/grpc/server_execution_states.go` gọi `s.auth` và `s.mapError` không có | Không test nào của `task-service` chạy được |
| `mcp-service` | `go build` và `go vet` lỗi: `internal/usecase/tools/request_flow_tools.go:3` import `context` thừa | Package `tools` không build |
| `frontend` | 7 lỗi `tsc` ở `ui.ts` (kiểu `previousViewBefore*` thiếu `'requests'`); 3 test request đỏ | Toàn repo có 184 lỗi `tsc` (chưa đối chiếu baseline) |
| `agent` | 26 test đỏ (21 `subprocess.test.ts` timeout, 5 `agent-tool-registry.test.ts`); chưa xác định có sẵn hay do commit | Không có lỗi nào ở code CR-033 |

## 3. Điểm chính theo khu vực

**Backend (`request-service`).** Build, vet, test đều xanh nhưng chủ yếu do có rất ít test (khoảng 57 test, 0 skip); không có test cho phần lớn tính năng.
- Gần như toàn bộ lifecycle (003 đến 006), kế hoạch và thực thi (012 đến 014), hợp đồng thực thi (029), tác động rủi ro (030), quản trị AI (034) chỉ là hàm hoặc struct rỗng trả `nil` hay hằng cứng, không ai gọi.
- Repository Postgres và MySQL của `Request` chỉ là `struct {}`. `GetRequest` và `ListRequests` trả `Unimplemented`.
- Handler Approval cho cả 8 subject đều là `NoopSubjectHandler`. Với cấu hình mặc định (`REQUEST_ALLOW_NOOP_APPROVAL_HANDLERS=false`) service từ chối khởi động.
- Migration Postgres không chạy theo thứ tự (`0002_approvals` có khóa ngoại tới `request.requests` chỉ tạo ở `0005`; suy ra từ SQL, chưa chạy). Migration `0006_analysis_runs` chỉ `ENABLE` RLS, không `FORCE`, dùng tham số sai `orca.tenant_id`.
- Gateway: 0 trên 28 kênh WS Request được đăng ký; cờ `request_flow_enabled` không có interceptor thật; chưa có workflow CI `backend-go-request-service.yml`.
- Phần làm thật: `AppendDetailed` ở `common/auditclient`, hồ sơ năng lực dev server ở `infra-fleet-service` (chưa nối main), `common/secretscan` (13 loại), `request.rego` (OPA 119 trên 119 test pass). Nhưng các phần này chưa được nối vào đâu.
- `task-service`: migration `0015` (CHECK `task_type`) và `0016` có; `Task.request_id` không có trong proto; chữ ký `ClaimForExecution`/`CompleteExecution`/`ReleaseExecution` không đổi.

**Frontend.** Trang Request không mở được bằng giao diện: nút sidebar chỉ hiện khi `requestFlowSupport === 'supported'`, mặc định là `'unknown'`, và hook dò hỗ trợ chỉ chạy bên trong chính trang đó. Bảy hook chưa component nào gọi. Parser lệch hợp đồng backend (`submitted` thay `new`, `approvalId` thay `id`, `items` thay `requestRows`). Việc gỡ `backlog` khỏi `TaskStatus` mới làm một nửa. Chưa có token `--risk-*`, chưa có khoá i18n, chưa có CR-032 và CR-036.

**Agent (CR-033).** Làm tốt nhất: 10 trên 13 task đủ, 117 test CR-033 pass. Thiếu test cấp handler cho chế độ chỉ đọc; e2e đối kháng chỉ chạy khi `ORCA_REAL_CLAUDE_E2E=1`, chỉ 1 prompt, không gọi handler thật; cú pháp `--tools` còn ghi UNVERIFIED. Chưa chạy `pnpm build` và bước kiểm tay trên dev server.

## 4. Việc còn lại (theo ưu tiên)

1. **Sửa hồi quy build** ở `task-service` và `mcp-service` (và `ui.ts` ở frontend). Đây là việc nhỏ nhưng chặn mọi thứ.
2. **Trả trạng thái về đúng thực tế:** hoàn tác `update_v6.js` trên `specs/backend-go/crs/v6/` (đưa các task chưa Đủ về `[ ]`, đổi lại tiêu đề solution), hoặc dùng các báo cáo này làm nguồn đúng. Xoá `update_v6.js` và `update_v7.js` khỏi gốc repo.
3. **Dựng lát cắt dọc chạy được** theo kế hoạch đợt (README v6): kế hoạch số migration của `request-service`, repository thật, `TransitionRequest`, `CreateRequest`, `GetRequest`, `ListRequests`, đăng ký kênh WS thật, thay `NoopSubjectHandler`, nối wiring `main.go`.
4. **Bổ sung test thật** (hiện chủ yếu là test rỗng 9 dòng) và dựng được test tích hợp với DB.
5. Các nhóm còn lại theo đợt: 012 đến 014, 029, 030, 034, rồi frontend.

## 5. Không kiểm chứng được

- Mọi test tích hợp (Postgres, MySQL, NATS) và áp migration lên DB thật.
- e2e với `claude` thật, dev server thật, Jira thật; không chạy Playwright; không xem được giao diện thật.
- Baseline `tsc` cũ và việc 26 test agent đỏ là có sẵn hay do commit; `ORCA_VERSION` thật; `pnpm build` của agent.
- Các con số trong báo cáo chỉ phản ánh đối chiếu code và test ở thời điểm kiểm toán; một số verdict của agent dựa vào đọc code, không chạy.
