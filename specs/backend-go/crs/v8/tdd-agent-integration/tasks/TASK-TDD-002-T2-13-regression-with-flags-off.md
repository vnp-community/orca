# TASK-TDD-002-T2-13: Hồi quy: cờ tắt thì mọi thứ như cũ

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 1 (P1), 6
**Checklist:** [T2.13](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P0 · **Area:** toàn repo
**Files:** test hiện có của `request-service`, `task-service`, `api-gateway`, `infra-fleet-service`, `agent` (vitest `src/relay`), `frontend` (vitest hook plan), bộ e2e `tests/stages/e2e/orca-flows`
**Depends on:** T2.1 đến T2.12 (chạy lại sau mỗi cụm thay đổi)
**Status:** `[ ]` TODO

---

## Context

Mọi thay đổi ở Giai đoạn 2 phải theo cờ (engine `openspec`, Solution bằng agent, `knowledgeRoot`), mặc định tắt. Bộ e2e thật đã có (Claude thật trên repo `deepseek-harness`, tốn lượt AI và khoảng nửa giờ cho bộ đầy đủ) là phép thử hồi quy tốt nhất cho luồng Request.

## Việc cần làm

1. Chạy `go test ./...` ở `backend-go/services/{request-service,task-service,api-gateway,infra-fleet-service}` và `backend-go/cmd/orca-cli`; ghi kết quả.
2. Chạy vitest của agent (`agent/`, thư mục `src/relay`) và của hook plan ở `frontend/` (`usePlanTree`, `request-plan-generation`).
3. Redeploy stack local (`bash deploy/dev/scripts/redeploy-local.sh <dịch vụ đã đổi>`, `BUILD_FRONTEND=1` nếu đổi frontend) và chạy smoke `PLT`.
4. **Xin xác nhận chi phí** rồi chạy e2e thật với cờ tắt: `ORCA_E2E_GUARD_IGNORE='specs/:docs/:frontend/:backend-go/:tests/:deploy/' python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real deepseek-review` và `--real deepseek-execute`; chỉ chạy `--real deepseek` đầy đủ trước khi hợp nhất.
5. Ghi số liệu (xanh/đỏ, thời gian) và so với kết quả đã ghi trong TASK-FE-E2E-028.

## Tiêu chí hoàn thành

- [ ] Tất cả test và e2e đã chạy xanh như trước khi sửa.
- [ ] Không có Request, worktree hay dữ liệu người dùng mồ côi sau e2e (`report_orphan_requests.py`, `report_orphan_worktrees.py`).

## Kiểm thử

- Chính các bộ nêu trên.

## Rủi ro và ghi chú

- Bộ e2e dùng repo của người dùng với các biện pháp an toàn đã có (chặn push, dấu vân tay); không chạy khi chưa được phép.
