# TASK-REQ-035-09: Áp dụng `secretscan` (cổng vào, prompt, đầu ra), danh sách `env` cho agent, supply chain và đối chiếu `07-security-architecture`

**From Solution:** BE-REQ-SOL-035 (mục E, 2.9, 2.11 của CR)
**Priority:** P0 (cổng vào và prompt), P1 (CI, đối chiếu)
**Service:** `request-service`, `backend-go/ci`, `.github/workflows`
**File:** `backend-go/services/request-service/internal/usecase/{create_request.go (sửa), secret_ingress_guard.go (mới), prompt_redaction.go (mới)}`, `.../internal/domain/agent_env_allowlist.go` (mới), `.../internal/adapter/grpcclient/agent_prompt_relay.go` (sửa: dựng `env`), `.../internal/usecase/analysis_secret_redaction.go` (sửa: CR-008 chuyển sang `secretscan`), `.github/workflows/backend-go-request-service.yml` (sửa: `govulncheck`, Trivy), `backend-go/services/request-service/README.md` (sửa: bảng đối chiếu 2.11), và `_test.go` tương ứng
**Depends on:** TASK-REQ-035-01 (`secretscan`), 035-04 (interceptor), 035-06 (cột `contains_secret_suspected`, `redact_pii_in_prompts`), BE-REQ-SOL-004 (`CreateRequest`), 005, 007, 008, 012 (điểm dựng prompt và đầu ra), BE-REQ-SOL-034 task 04 (`AIGateway`)
**Status:** [ ] TODO (một phần: cổng vào, cờ, env, redactor mcp-service xong; còn nối prompt, CI, bảng 2.11)

---

## Context

- Ba điểm áp dụng (CR 2.6): (1) **cổng vào** (`CreateRequest`, webhook, MCP): `secretscan` độ tin cậy **cao** ⇒ thay `[REDACTED:<loại>]` ở `body` (và `title`) trước khi lưu, đặt `requests.contains_secret_suspected=true`; chọn che ngay vì Request đã dán bí mật là sự cố và giữ bản gốc chỉ nhân rủi ro. (2) **trước prompt**: mọi văn bản đi vào prompt (Request, `prior_artifacts`, kết quả `fs.*`, Context Pack) qua `Redact`; không cấu hình tắt. (3) **đầu ra** AI/agent trước khi lưu: CR-REQ-008 `analysis_secret_redaction.go` đổi sang dùng gói này.
- `agent-rpc-dispatch-ai.ts:141`: `ai.complete` nhận `resolvedApiKey`; `agent.execPrompt` nhận tham số `env` (xem `agent-print-mode-exec.ts`; đọc file để biết tên tham số thật). Tiêu chuẩn: `env` chỉ gồm `ORCA_REQUEST_ID`, `ORCA_PROJECT_ID` (CR-REQ-008 2.2); không token Orca, `credential_ref`, khoá nhà cung cấp; khoá nhà cung cấp nằm ở dev server.
- PII (email, số điện thoại): mặc định không che; tenant bật `redact_pii_in_prompts` thì che **trong prompt** (không trong bản lưu), bộ che PII của BE-REQ-SOL-031 task 02 (`PIIRedactor`).
- Log: không ghi `title`, `body`; `reporter_id` là UUID; lỗi trả client không chứa nội dung Request.
- `arch/07` đối chiếu (CR 2.11): AuthN browser/CLI, MCP; origin check `WS_ALLOWED_ORIGINS`; service-to-service; AuthZ; multi-tenancy 4 lớp; audit; secrets; input validation (`protovalidate`); supply chain (`govulncheck`, Trivy). Bảng phải được điền kết quả (đạt, chưa, ngoài phạm vi) trong PR đầu tiên của CR.

## Việc cần làm

1. `secret_ingress_guard.go`: `type SecretIngressGuard struct{ clock Clock }` với `Apply(title, body string) (newTitle, newBody string, suspected bool, kinds []secretscan.Kind)`: `secretscan.RedactKinds(text, secretscan.High)` cho `title` và `body`; `suspected = len(kinds) > 0`. Không bao giờ log giá trị; chỉ trả `kinds`.
2. `create_request.go` (do SOL-004 tạo; sửa tối thiểu): gọi `Apply` **trước** chuẩn hoá khoá idempotency và trước khi ghi; `Request.ContainsSecretSuspected = suspected`; phản hồi `CreateRequestResponse` thêm `bool contains_secret_suspected` (proto additive) để UI báo người dùng (CR 2.6). Khoá idempotency tính trên nguồn (`source_ref`), không trên `body` đã che nên không đổi ngữ nghĩa giao lặp. Chạy `gitnexus_impact` trên `CreateRequest` use case trước khi sửa.
3. Webhook và MCP đều đi qua cùng use case `CreateRequest`, nên một điểm sửa; test riêng cho từng nguồn.
4. `prompt_redaction.go`: `type PromptRedactor struct{ pii *redaction.PIIRedactor; settings TenantSettingsReader }` với `Apply(ctx, text string) (string, error)`: luôn `secretscan.Redact`; khi `settings.RedactPII(ctx)` (đọc `tenant_settings.redact_pii_in_prompts`, lỗi đọc ⇒ **bật che**, fail closed) thì thêm `PIIRedactor`. Điểm gọi: `PromptRenderer`/`AIGateway` (BE-REQ-SOL-034 task 04) bọc mọi biến chuỗi trước `Render` (hook `PromptRedactor` trong `AICall`); các use case CR 005, 007, 008, 012 truyền văn bản thô, **không** tự che rải rác (một chỗ duy nhất).
5. `analysis_secret_redaction.go` (CR-008): thay bộ mẫu riêng bằng `secretscan.Redact`; giữ giao diện hàm cũ để ít chỗ gọi bị đổi.
6. `agent_env_allowlist.go`: `var AgentEnvAllowlist = []string{"ORCA_REQUEST_ID", "ORCA_PROJECT_ID"}` và `func BuildAgentEnv(requestID, projectID string) map[string]string` (chỉ hai khoá). `agent_prompt_relay.go` luôn dựng `env` bằng hàm này, không nhận `map` từ bên ngoài; không bao giờ truyền `resolvedApiKey`. Test bảo vệ: quét mọi lời gọi relay trong test và đòi `env` ⊆ allowlist.
7. Cổng ra ở đường lưu: `solutions.options`, `analysis_runs.raw_output`, `solutions.content_ref` đi qua `Redact` trước khi ghi (CR-007/008 đã nêu; task này chỉ bảo đảm có test).
8. CI (`backend-go-request-service.yml`): bước `go run golang.org/x/vuln/cmd/govulncheck@latest ./services/request-service/...` (ghim phiên bản thay vì `@latest` nếu repo đã có quy ước; kiểm workflow của service khác trước) và quét ảnh Trivy `aquasecurity/trivy-action` cho ảnh `request-service` (mức `CRITICAL,HIGH`, `exit-code: 1`, `ignore-unfixed: true`), theo `arch/07` mục Input validation & supply chain. Nếu repo chưa có Trivy ở workflow nào, ghi "chưa kiểm chứng" và đặt bước ở chế độ `continue-on-error: true` kèm nhãn việc cần chuyển sang chặn.
9. Ràng buộc `protovalidate` cho độ dài và enum trên các message mới của CR 031, 034, 035 (CR-REQ-001 cần nêu): thêm vào `.proto` với `buf.validate` nếu repo đã dùng (kiểm `buf.yaml` deps); chưa dùng thì kiểm thủ công ở handler (task 07) và ghi vào PR.
10. README của `request-service`: bảng đối chiếu `07-security-architecture.md` (10 hàng ở CR 2.11) với cột trạng thái điền thật sau khi làm; chỉ ghi "đạt" cho mục đã có test.

## Kiểm thử

- `secret_ingress_guard_test.go`: `body` chứa khoá riêng PEM ⇒ `[REDACTED:private_key]` và `suspected=true`; chứa `ghp_...` ⇒ che; chứa `Bearer abc12345` (mức `medium`) ⇒ **không che** ở cổng vào (nhưng prompt sẽ che); văn bản tiếng Việt có dấu không bị đổi; không có giá trị bí mật trong `kinds`.
- `create_request_test.go` (mở rộng): Request tạo từ `manual`, `webhook`, `mcp` đều qua guard; lưu `contains_secret_suspected`; log capture không chứa chuỗi bí mật mẫu; giao lặp trả cùng Request.
- `prompt_redaction_test.go`: prompt dựng ra không chứa chuỗi bí mật mẫu từ `Request.body`, `prior_artifacts` và kết quả `fs.*` giả; `redact_pii_in_prompts=true` che email, số điện thoại **trong prompt** nhưng bản lưu giữ nguyên; lỗi đọc cài đặt ⇒ che PII.
- `agent_env_allowlist_test.go`: `BuildAgentEnv` chỉ hai khoá; test relay: `env` luôn ⊆ allowlist, không có `resolvedApiKey`.
- `analysis_secret_redaction_test.go`: dùng cùng vector với `secretscan` (không còn hai bộ mẫu).
- CI: chạy workflow trên nhánh thử, ghi kết quả vào PR (chưa chạy ở thời điểm viết).
- Lệnh: `cd backend-go && go test ./services/request-service/internal/usecase/... ./services/request-service/internal/domain/... ./services/request-service/internal/adapter/grpcclient/... && go vet ./services/request-service/... && govulncheck ./services/request-service/...`.

## Tiêu chí hoàn thành

- [ ] `body` chứa khoá riêng PEM được lưu với `[REDACTED:private_key]` và `contains_secret_suspected=true`; prompt dựng ra không chứa chuỗi đó.
  - chưa: cờ và che ở cổng vào đã có test; phần "prompt dựng ra không chứa chuỗi đó" chưa nối được vì chưa có `PromptRenderer`/`AIGateway` (CR-034) trong cây này.
- [ ] `env` của `execPrompt` không chứa khoá nào ngoài danh sách cho phép (test trên bộ dựng tham số); không bao giờ gửi `resolvedApiKey`.
  - chưa: có `domain.BuildAgentEnv`/`EnvWithinAllowlist` (Go) và `agent/src/relay/agent-request-flow-env.ts` (TS) với test, nhưng chưa có call site relay `execPrompt` của request-service để áp.
- [x] Bộ mẫu che bí mật là **một** (`common/secretscan`); không còn bộ mẫu riêng ở `request-service`.
- [ ] CI có `govulncheck` và quét ảnh; bảng đối chiếu 2.11 được điền trong PR đầu tiên.
  - chưa: đã thêm bước `opa test` và `govulncheck` (advisory) vào workflow; quét ảnh Trivy và bảng 2.11 chưa làm, chưa chạy CI.
- [ ] Log, lỗi, sự kiện không chứa `title`/`body`.
  - chưa: chưa có test chụp log; code mới không log nội dung.

## Ví dụ tham khảo

Bảng đối chiếu `07-security-architecture.md` (điền trong PR đầu tiên; trạng thái ban đầu là dự kiến, chưa kiểm chứng):

| Mục | Trạng thái dự kiến | Ở đâu |
|---|---|---|
| AuthN browser/CLI | đạt (gateway, không đổi) | không đổi |
| AuthN MCP | đạt khi `actor_type=agent` có | 035-02 |
| Origin check | chưa (cần `WS_ALLOWED_ORIGINS` ở production) | CR-REQ-016 |
| Service-to-service (mTLS) | chưa; có `internalcaller` | 035-04 |
| AuthZ một gói OPA | đạt khi `request.rego` có | 035-03 |
| Multi-tenancy 4 lớp | đạt khi RLS và test quét có | 035-05 |
| Audit | đạt khi outbox có | 035-06 |
| Secrets | đạt (khoá ở dev server) | 035-09 |
| Input validation | một phần (protovalidate chưa kiểm chứng) | 035-07, 035-09 |
| Supply chain | đạt khi CI có | 035-09 |

## Rủi ro và lưu ý

- Che bí mật ngay cổng vào có thể làm mất thông tin hữu ích khi dương tính giả; người dùng thấy cờ và sửa lại (cần UI của CR-REQ-019 hiển thị `contains_secret_suspected`; ghi cho frontend).
- Regex không phủ hết; bí mật nằm trong mã nguồn đã đi qua `execPrompt` theo thiết kế (CR T2): không hứa loại bỏ hoàn toàn.
- Bước `Trivy` có thể đỏ do lỗ hổng của ảnh nền ngoài tầm sửa của dịch vụ: dùng `ignore-unfixed` và danh sách ngoại lệ có chủ.
- `protovalidate` chưa chắc có trong repo (chưa kiểm chứng `buf.yaml`): nếu không, kiểm thủ công.
- Đồng bộ CR 005, 007, 008, 012: họ đang sở hữu điểm dựng prompt; PR của họ phải gọi `PromptRedactor` qua `AIGateway` (xem BE-REQ-SOL-034).

## Ghi chú triển khai (2026-10-08)

## Tiến độ

Đã làm: `SecretIngressGuard` (nối vào `CreateRequest.CreateWithinTx`, nên manual/webhook/MCP/child đều qua; text quá cửa sổ quét bị từ chối `REQUEST_PAYLOAD_TOO_LARGE` vì `Truncated`), cờ `contains_secret_suspected` ghi cùng giao dịch, `PromptRedactor` (secret luôn che; PII theo cài đặt, lỗi đọc cài đặt thì che) có test nhưng chưa có điểm gọi, `domain.RedactSecrets` đã dùng `secretscan` (một bộ mẫu), mcp-service `SecretscanRedactor` (cộng thêm, mặc định cũ giữ nguyên), danh sách env agent.

Còn thiếu: nối `PromptRedactor` vào bộ dựng prompt khi CR-034 có; call site relay dùng `BuildAgentEnv`; Trivy; bảng đối chiếu 2.11; `protovalidate` (không sửa proto); đổi mcp-service mặc định sang `SecretscanRedactor` (chủ sở hữu mcp-service).
