# TASK-TDD-002-T2-14: Quan sát: ghi số lần gọi tool, snapshot KG, hash skill, thời gian, chi phí

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 5 (tiêu chí 7)
**Checklist:** [T2.14](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P2 · **Area:** agent + request-service + orca-cli
**Files:** `agent/src/relay/agent-exec-prompt-output-capture.ts` (stream-json), `agent-print-mode-exec.ts` (kết quả), `request-service` (cột/metric), `orca-cli`
**Depends on:** T2.1, T2.6, T2.11
**Status:** `[ ]` TODO

---

## Context

Agent đã chạy `claude` với `--output-format stream-json --verbose` khi hỗ trợ, nên thấy được từng lời gọi tool và (ở thông điệp kết thúc) thống kê sử dụng. Để chứng minh "agent gọi tool KG thật" (tiêu chí nghiệm thu) và đo chi phí, cần đưa các số đó vào kết quả và lưu.

## Việc cần làm

1. Agent: thêm vào kết quả `execPrompt` trường `toolCalls` (đếm theo tên tool, chỉ tên, không nội dung) và `usage` (token, chi phí nếu có trong thông điệp kết thúc của stream-json); giữ trần kích thước; báo bằng feature nếu cần.
2. Go: parse `toolCalls`/`usage`; lưu vào bản ghi run/thực thi cùng `kg_snapshot`, `skill_hash`, thời gian bắt đầu/kết thúc.
3. Metric (Prometheus của request-service): `request_engine_runs_total{engine,kind,result}`, `request_engine_preflight_failures_total{code}`, `request_knowledge_tool_calls_total{tool}` (mới), `request_spec_check_violations_total{code}` (mới).
4. CLI: `orca request show` và `orca plan proposal` hiển thị thời gian, số lần gọi tool, chi phí (nếu có).
5. Không ghi nội dung tham số hay kết quả tool vào log/metric (có thể chứa văn bản KG).

## Tiêu chí hoàn thành

- [ ] Một run thật cho thấy số lần gọi `mcp__tdd-agent__*` và chi phí ở CLI.
- [ ] Metric xuất hiện và có nhãn đúng.

## Kiểm thử

- Test parser `toolCalls`/`usage` với stream-json mẫu; test metric.

## Rủi ro và ghi chú

- Chi phí chỉ có khi `claude` báo; nếu không thì ghi "không có", không ước đoán.
