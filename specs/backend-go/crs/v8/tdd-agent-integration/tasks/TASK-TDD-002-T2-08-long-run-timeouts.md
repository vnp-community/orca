# TASK-TDD-002-T2-08: Thời gian chạy dài: giới hạn riêng cho engine `openspec`, lease, trần của agent, timeout của UI và CLI

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.4 (bước 8), 6 (H10)
**Checklist:** [T2.8](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P1 · **Area:** request-service, agent, frontend, orca-cli
**Files:** `request-service/internal/usecase/analysis_settings.go`, `agent/src/relay/agent-exec-prompt-timeouts.ts`, `frontend/src/renderer/src/hooks/request-plan-generation.ts` (`PLAN_RUN_TIMEOUT_MS`), `backend-go/cmd/orca-cli/internal/command/plan_cmd.go` (`--timeout` mặc định 4 phút), test đi kèm
**Depends on:** T2.4
**Status:** `[ ]` TODO

---

## Context

Dữ kiện đã kiểm: `AnalysisRunner.execute` đã gia hạn lease mỗi `Heartbeat` (30 s) với `LeaseTTL` 90 s, nên run dài không bị thu hồi chỉ vì lâu (mối lo ban đầu của H10 chủ yếu đã được xử lý). Còn lại: `AgentTimeoutMS` mặc định 600 000 (10 phút); agent giới hạn `timeoutMs` trong khoảng `MIN_TIMEOUT_MS` đến `MAX_TIMEOUT_MS` = 15 phút (mặc định 5 phút); UI chờ Plan tối đa 240 s (`PLAN_RUN_TIMEOUT_MS`); CLI `plan generate` mặc định 4 phút. Run engine `openspec` có thể mất vài phút đến hơn 10 phút (lần `tdd-agent change` thật mất 3 đến 5 phút mỗi vòng).

## Việc cần làm

1. `AnalysisSettings`: thêm `SpecTimeoutMS` (mặc định 900 000, bằng trần 15 phút của agent), cấu hình được; dùng trong `openspecEngine`; hai lần gọi (lần thử lại) mỗi lần có ngân sách riêng và tổng giới hạn do cấu hình.
2. Xác nhận khoảng chờ của relay (`infra-fleet` `Relay`/`RelayByDevServer`) và các deadline gRPC giữa gateway, request-service, infra-fleet không ngắn hơn `SpecTimeoutMS + slack` (hiện `slackOverTimeout` = 30 s); sửa nếu có trần thấp hơn.
3. Nếu cần vượt 15 phút: nâng `MAX_TIMEOUT_MS` có chủ ý (kèm test) hoặc chia run; ghi quyết định.
4. Phản hồi đề xuất Plan thêm `engine`; UI (`request-plan-generation.ts`) chọn `PLAN_RUN_TIMEOUT_MS` theo engine (ví dụ 18 phút cho `openspec`, giữ 240 s cho `native`) và hiện thông báo "đang viết gói OpenSpec, có thể mất vài phút".
5. CLI `plan generate`: mặc định `--timeout` theo engine (đọc từ phản hồi `propose`), giữ cờ để ghi đè.
6. Run bị bỏ sót (instance chết) vẫn được sweeper `RecoveryInterval` fail đúng.

## Tiêu chí hoàn thành

- [ ] Run giả chạy 5 phút (hai chu kỳ lease) hoàn tất mà không bị thu hồi.
- [ ] UI và CLI không báo hết thời gian trước khi run `openspec` hoàn tất trong ngân sách.
- [ ] Engine `native` giữ nguyên các giá trị cũ.

## Kiểm thử

- Test đơn vị chọn timeout theo engine; test runner với worker giả chạy lâu (đồng hồ giả); vitest cho hook UI.

## Rủi ro và ghi chú

- Giới hạn 15 phút của agent là trần cứng hiện tại cho một lượt `execPrompt`.
