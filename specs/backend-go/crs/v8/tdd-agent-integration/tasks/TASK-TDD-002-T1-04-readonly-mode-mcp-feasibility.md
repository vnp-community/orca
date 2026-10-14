# TASK-TDD-002-T1-04: Chế độ chỉ đọc có gọi được tool MCP không

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.3, 6 (H1)
**Checklist:** [T1.4](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P1 · **Area:** spike (không sửa Orca)
**Files:** `agent/src/relay/agent-readonly-tool-policy.ts` (đọc), `evidence/T1.4-readonly-mcp.md` (mới)
**Depends on:** T1.3
**Status:** `[ ]` TODO

---

## Context

Chế độ chỉ đọc của `agent.execPrompt` hiện chạy `claude --print … --permission-mode plan --tools Read,Glob,Grep` (`buildReadonlyArgs`). `--tools` giới hạn **công cụ dựng sẵn**; chưa rõ tool MCP có bị chặn bởi `--permission-mode plan` hay cần `--allowedTools mcp__tdd-agent__*`. Kết quả quyết định T2.9: dùng chế độ chỉ đọc cho Solution hay phải dùng chế độ ghi hẹp.

## Việc cần làm

1. Chạy `claude --print` với đúng bộ cờ readonly (`--permission-mode plan --tools Read,Glob,Grep`) cộng `--mcp-config /tmp/tdd-mcp.json`, hỏi agent gọi `mcp__tdd-agent__kg_search`; ghi tool có chạy hay bị từ chối và thông báo lỗi.
2. Thử thêm `--allowedTools "mcp__tdd-agent__kg_search" …` (danh sách đầy đủ ở Phụ lục A) với và không có `--permission-mode plan`.
3. Thử `--strict-mcp-config` nếu CLI có (kiểm bằng `claude --help`) để đảm bảo chỉ MCP đã chỉ định được nạp.
4. Xác nhận agent không ghi được file trong cấu hình chọn (thử yêu cầu tạo file): phải thất bại hoặc bị từ chối.
5. Chọn cấu hình tối thiểu chạy được; ghi chính xác bộ cờ, phiên bản `claude` (2.1.266 lúc đo) và cách phát hiện cờ (`detectClaudeFlags`).
6. Nếu không có cấu hình nào vừa chỉ đọc vừa gọi được MCP: ghi kết luận để T2.9 dùng chế độ ghi hẹp trong worktree riêng.

## Tiêu chí hoàn thành

- [ ] Có bộ cờ cụ thể (hoặc kết luận phủ định) kèm log chứng minh.
- [ ] Xác nhận ghi file bị chặn trong cấu hình chọn.

## Kiểm thử

- Kiểm thủ công với log; ghi lệnh để chạy lại.

## Rủi ro và ghi chú

- Hành vi của `claude` thay đổi theo phiên bản: T2.1 phải phát hiện khả năng bằng probe cờ (`detectClaudeFlags`), không giả định.
