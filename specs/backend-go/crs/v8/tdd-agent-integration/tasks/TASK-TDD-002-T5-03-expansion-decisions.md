# TASK-TDD-002-T5-03: Quyết định mở rộng

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.6 (Q2, Q3), 6
**Checklist:** [T5.3](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P2 · **Area:** quyết định
**Files:** `evidence/T5.3-decisions.md` (mới)
**Depends on:** T4.7, T5.2
**Status:** `[ ]` TODO

---

## Context

Sau pilot cần quyết định phạm vi tiếp theo dựa trên số liệu.

## Việc cần làm

1. Phản biện độc lập gói OpenSpec (một lượt agent thứ hai trước gate Plan): có đáng không (Q2).
2. Loại Request khác (`refactor`, `bug` với profile `light`, `security`, `performance`) (Q3).
3. Bật prose có kiểm soát để KG giàu hơn; ảnh hưởng tới xung đột `entity_match` (QG-04).
4. Tái xét pipeline `tdd-agent change` khi phân rã ổn định (T1.9a của CHECKLIST-001).
5. Ghi từng quyết định, người quyết, ngày.

## Tiêu chí hoàn thành

- [ ] Có ghi chú quyết định cho từng mục, mỗi mục "làm/không làm/hoãn" kèm lý do.

## Kiểm thử

- Không có test.
