# TASK-TDD-002-T2-15: Tài liệu và ghi chú thay đổi hợp đồng

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) toàn bộ
**Checklist:** [T2.15](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P2 · **Area:** tài liệu
**Files:** `backend-go/cmd/orca-cli/README.md`, `specs/backend-go/crs/v6/solution-engines/{solutions,tasks}/` (trạng thái từng TASK), `specs/backend-go/crs/v8/**`, tài liệu dev server (cài `tdd-agent`, `openspec`, skill)
**Depends on:** T2.1 đến T2.14
**Status:** `[ ]` TODO

---

## Context

Các thay đổi chạm nhiều hợp đồng: tham số `agent.execPrompt` (`knowledgeRoot`, `proposeChangesets`, kết quả `toolCalls`/`usage`), schema Solution (`design_references`), prompt Plan/Solution/task, cột DB mới, route gateway, lệnh CLI, trạng thái các TASK của CR-REQ-026 (từ "Proposed" thành đã làm một phần).

## Việc cần làm

1. Cập nhật README `orca-cli` (lệnh `plan bundle`, `engine`, `request cleanup`, các trường mới).
2. Cập nhật `BE-REQ-SOL-026` và `TASK-REQ-026-0x`: đánh dấu phần đã hiện thực, phần còn stub, và chỗ SOL-TDD-002 khác (Solution không dùng engine `openspec`, dùng agent chỉ đọc).
3. Viết hướng dẫn vận hành: cài `tdd-agent`/`openspec`/skill trên dev server, tạo KG (`tdd-agent -C <repo> generate`), bật cờ project, xử lý lỗi preflight.
4. Cập nhật SOL/CHECKLIST-TDD-002: tiến độ, quyết định T1.7/T1.8/T1.9.

## Tiêu chí hoàn thành

- [ ] Tài liệu khớp mã đã hợp nhất; mọi mã lỗi mới có mô tả và `fix_hint`.

## Kiểm thử

- Rà bằng việc làm theo hướng dẫn vận hành trên một máy dev server sạch (T4.3).
