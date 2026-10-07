# AG-REQ-TASK-033-11: `ai.complete` trả `usage`, nhận `maxTokens`, lỗi có `error.data`

**From Solution:** [AG-REQ-SOL-033-capability-report-handshake-and-ai-complete](../solutions/AG-REQ-SOL-033-capability-report-handshake-and-ai-complete.md) mục 2.5
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/ai-complete-handler.ts` (sửa), `agent/src/relay/agent-rpc-dispatch-ai.ts` (sửa), `agent/src/relay/__tests__/ai-complete-handler.test.ts` (sửa), `agent/src/relay/agent-rpc-dispatch-ai.test.ts` (mới)
**Depends on:** không
**Status:** [x] DONE

## Context

CR-033 mục 2.8 (do CR-REQ-034 cần): `handleAIComplete` thêm, tương thích ngược: (a) `usage: { inputTokens, outputTokens }` đọc từ phản hồi nhà cung cấp, cùng `provider`, `latencyMs`; (b) tham số `maxTokens` (mặc định 4096, trần 16384) vì `max_tokens: 4096` cố định có thể cắt cụt JSON Plan dài (CR-REQ-012); (c) lỗi có `error.data: { provider, httpStatus, retryable }` thay vì chỉ thông điệp chuỗi.

Đã đọc `ai-complete-handler.ts` (290 dòng): `AICompleteParams` có `prompt, format, taskId, model, accountId, resolvedApiKey`; kết quả `AICompleteResult = { content, model? }`; `callAnthropic` đặt `max_tokens: 4096`, `callOpenAI` đặt `max_tokens: 4096`, `callGoogle` không đặt gì; khi `!res.ok` ném `new Error(\`Anthropic API error ${res.status}: ${errBody}\`)`; `providerNameFromModel` đã có; Google đặt khoá trong query URL. `dispatchAiRpc` (`agent-rpc-dispatch-ai.ts`, case `ai.complete`) chỉ đọc sáu khoá, KHÔNG đọc `maxTokens`, và bọc mọi lỗi thành `makeError(rpc.id, AgentErrorCode.ServerError, 'ai.complete failed: ' + msg)`. Go (`devserveragent/jsonrpc.go:30`) đã có `JSONRPCError.Data json.RawMessage`, nên `error.data` tới được backend mà không sửa decoder. Người gọi hiện tại của `ai.complete` ở Go: `task-service` (`aidecompose_relay.go`) và `git-gateway-service` (`generate_commit_message.go`, `generate_pull_request_fields.go`).

Tên trường `usage` của ba nhà cung cấp chưa kiểm chứng bằng cuộc gọi thật (CR đánh dấu): Anthropic `usage.input_tokens`/`output_tokens`; OpenAI `usage.prompt_tokens`/`completion_tokens`; Google `usageMetadata.promptTokenCount`/`candidatesTokenCount`.

## Việc cần làm

1. `AICompleteParams` thêm `maxTokens?: number`. `AICompleteResult` thêm `provider: 'anthropic' | 'openai' | 'google' | 'unknown'`, `latencyMs: number`, `usage?: { inputTokens: number; outputTokens: number }`. Các hàm `callAnthropic`/`callOpenAI`/`callGoogle` trả `{ text: string; usage?: ... }` thay vì `string`; `dispatch` truyền `maxTokens`.
2. `maxTokens` hiệu lực: mặc định 4096 khi vắng; số nguyên dương; lớn hơn 16384 thì kẹp 16384. Không phải số nguyên dương: `dispatchAiRpc` trả `InvalidParams` với `data: { reason: 'INVALID_MAX_TOKENS' }` (không gọi nhà cung cấp). Anthropic và OpenAI: `max_tokens`. Google: chỉ khi người gọi gửi `maxTokens` thì thêm `generationConfig: { maxOutputTokens }` (giữ hành vi cũ khi vắng).
3. `usage`: chỉ gắn khi cả hai giá trị là số hữu hạn không âm; thiếu thì bỏ HẲN `usage` (số sai làm hỏng ngân sách của CR-REQ-034). `latencyMs = Date.now() - startedAt` đo quanh `dispatch`.
4. Lớp lỗi mới, xuất từ `ai-complete-handler.ts`:
   ```ts
   export type AICompleteErrorReason =
     'PROVIDER_HTTP_ERROR' | 'PROVIDER_TIMEOUT' | 'PROVIDER_NETWORK' | 'NO_API_KEY' | 'UNKNOWN_MODEL_PROVIDER'
   export class AICompleteProviderError extends Error {
     constructor(message: string, readonly info: {
       provider: string; httpStatus: number | null; retryable: boolean; reason: AICompleteErrorReason })
   }
   ```
   `message` GIỮ NGUYÊN định dạng cũ (`Anthropic API error 429: ...`, `ai.complete: No API key found ...`) để người gọi cũ đọc thông điệp vẫn đúng. `retryable`: `true` với HTTP 408, 409, 425, 429, 500 đến 599, và timeout/mạng; `false` với 400, 401, 403, 404, 422, `NO_API_KEY`, `UNKNOWN_MODEL_PROVIDER`.
5. Phân loại lỗi `fetch`: `AbortSignal.timeout` làm `fetch` ném `DOMException` tên `TimeoutError` thì `PROVIDER_TIMEOUT` (hạn 120 giây hiện có, `httpStatus: null`); lỗi mạng khác (`TypeError: fetch failed`) thì `PROVIDER_NETWORK`. KHÔNG sao chép `err.cause` hay URL vào `message`/`info` (Google có khoá trong URL).
6. `dispatchAiRpc`: bắt `AICompleteProviderError` và trả `makeError(rpc.id, AgentErrorCode.ServerError, \`ai.complete failed: ${msg}\`, { provider, httpStatus, retryable, reason })`. Lỗi khác (ví dụ lỗi `resolvedApiKey` thiếu từ `resolveApiKey`) giữ hành vi cũ (không có `data`) trừ khi chuyển thành `NO_API_KEY` ở bước 4. `prompt` rỗng và `InvalidParams` hiện có giữ nguyên.
7. Bảo đảm KHÔNG log hay đưa `apiKey` vào `data`; log hiện có (`log.error('ai.complete Anthropic ...: ${errBody}')`) giữ nguyên (không có khoá).
8. Đọc `maxTokens` trong `dispatchAiRpc`: `typeof p['maxTokens'] === 'number'` thì chuyển tiếp, kiểm hợp lệ ở đó; kiểu khác (chuỗi, `null`) thì `INVALID_MAX_TOKENS`.

## Kiểm thử

Thêm `describe('handleAIComplete usage, maxTokens and errors')` vào `__tests__/ai-complete-handler.test.ts` (mẫu: `vi.stubEnv`, `vi.stubGlobal('fetch', ...)`, `registerTraceSink` đã có trong file):
- `returns provider, latencyMs and usage for an Anthropic response` (fetch giả trả `{ content: [{type:'text', text:'x'}], usage: { input_tokens: 10, output_tokens: 5 } }`).
- `omits usage entirely when the provider response has none`; `... when usage fields are not numbers`.
- `reads OpenAI prompt_tokens/completion_tokens` và `Google usageMetadata`.
- `sends max_tokens 4096 by default and the requested value when maxTokens is given` (kiểm body của `fetch`); `clamps maxTokens above 16384`.
- `Google body has no generationConfig unless maxTokens is given`.
- `throws AICompleteProviderError with retryable=true for 429 and 503` và `retryable=false for 401 and 400`.
- `maps a TimeoutError from fetch to PROVIDER_TIMEOUT retryable` và `network failure to PROVIDER_NETWORK`.
- `error message keeps the legacy "Anthropic API error 429:" text`.
- `never contains the api key in message, info or logs` (`vi.stubEnv('ANTHROPIC_API_KEY', 'sk-test-secret')`, khẳng định JSON của lỗi không chứa nó; với Google dùng khoá giả và khẳng định lỗi mạng không chứa URL).
- Hồi quy: các test tracing hiện có xanh.

Tạo `agent-rpc-dispatch-ai.test.ts` (khuôn `MockWs`, `createWireState` của `agent-rpc-dispatch-misc.test.ts`; gọi `dispatchAiRpc` trực tiếp):
- `forwards maxTokens to the handler`.
- `returns InvalidParams INVALID_MAX_TOKENS for 0, -5, 1.5, "100", null`.
- `puts provider, httpStatus, retryable, reason in error.data for a provider error` và `keeps message prefixed with "ai.complete failed:"`.
- `omits error.data for an unclassified error` (hành vi cũ).

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run src/relay/__tests__/ai-complete-handler.test.ts src/relay/agent-rpc-dispatch-ai.test.ts`; rồi `pnpm test`. Không gọi nhà cung cấp thật trong test tự động.
Kiểm với nhà cung cấp thật (CHƯA CHẠY): một lần tay với khoá thử để xác nhận tên trường `usage` của Anthropic, OpenAI, Google; ghi kết quả vào ghi chú task.

## Tiêu chí hoàn thành

- [x] Gọi `ai.complete` như cũ (không `maxTokens`) cho kết quả vẫn có `content`, `model`, và thêm `provider`, `latencyMs` (người gọi cũ bỏ qua trường thừa).
- [x] `usage` đúng ở ba nhà cung cấp hoặc vắng hẳn.
- [x] `maxTokens` hợp lệ đi tới nhà cung cấp; không hợp lệ cho `INVALID_MAX_TOKENS`.
- [x] Lỗi nhà cung cấp có `error.data` với `provider`, `httpStatus`, `retryable`, `reason`; `message` giữ định dạng cũ.
- [x] Không có khoá API trong `message`, `data`, log.

## Rủi ro và lưu ý

- Backend cũ gọi agent mới: trường thừa vô hại; Go dùng `json.Unmarshal` thường (chưa grep toàn bộ nơi giải mã kết quả `ai.complete` ở `task-service` và `git-gateway-service`: kiểm không có `DisallowUnknownFields`).
- Backend mới gọi agent cũ: `maxTokens` bị bỏ qua (vẫn 4096), không có `usage`/`provider`/`error.data`. Backend chỉ tin các trường mới khi handshake `features` có `ai.complete.usage` (task 12); nếu không, giữ giả định 4096 và phát hiện cắt cụt bằng kiểm JSON.
- Tên trường `usage` chưa kiểm chứng; nếu sai, `usage` vắng (an toàn) nhưng CR-REQ-034 mất số liệu: bắt buộc kiểm thật trước khi bật ngân sách.
- Có thể cần `stopReason` (Anthropic `stop_reason: "max_tokens"`) để backend biết JSON bị cắt không cần đoán; ngoài CR, ghi ở câu hỏi mở 2 của solution.
- `AbortSignal.timeout` làm `TimeoutError` hay `AbortError` tuỳ phiên bản Node (target Node 22 theo `build.mjs`); test cả hai tên.
