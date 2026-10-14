# TASK-TDD-002-T5-01: Test e2e tự động cho luồng mới trên repo tạm

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 5
**Checklist:** [T5.1](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P1 · **Area:** tests/stages/e2e
**Files:** `tests/stages/e2e/orca-flows/` (script CLI mới hoặc spec Playwright), `README.md` của bộ e2e
**Depends on:** T4.4
**Status:** `[ ]` TODO

---

## Context

Pilot chạy tay; cần phép thử lặp lại được, không đụng dữ liệu người dùng. Vì Orca đã có CLI, kịch bản CLI đơn giản hơn spec UI cho luồng này.

## Việc cần làm

1. Dựng repo tạm có tài liệu nhỏ (URD/SRS/SAD mẫu) và KG dựng bằng `tdd-agent generate --no-prose`.
2. Script `run_cli_flow.py --real knowledge`: tạo project tạm có repo tạm trên `dev-local-mac`, chặn push, chạy `orca` CLI từ `request create` đến `completed` với một Request nhỏ có kiểm chứng được, kiểm khẳng định tự động (có `design_references`, gói đạt kiểm cấu trúc, task có `Verification`, không đổi ngoài phạm vi), dọn.
3. Tái dùng `pilot_guard.py`, `report_orphan_*` và quy ước an toàn của bộ e2e hiện có.
4. Ghi vào `README.md` cách chạy, chi phí và điều kiện.

## Tiêu chí hoàn thành

- [ ] Chạy xanh trên stack local; dọn sạch; không lệ thuộc repo của người dùng.

## Kiểm thử

- Chính script (có cả ca đỏ cố ý: gói thiếu AC phải bị từ chối).

## Rủi ro và ghi chú

- Tốn lượt AI: chỉ chạy khi được phép.
