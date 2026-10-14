# TASK-TDD-002-T4-06: Chạy BL-H02 sau khi BL-H01 hoàn tất

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 4.5
**Checklist:** [T4.6](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P1 · **Area:** orca + pilot
**Files:** CLI `orca`; `docs/crs/v5/CR-BL-H02_ai-case-generation.md`
**Depends on:** T4.4, T4.5
**Status:** `[ ]` TODO

---

## Context

BL-H02 phụ thuộc BL-H01 (`aiclient`). Chạy cùng quy trình sau khi mã BL-H01 có sẵn cho BL-H02 (cần hợp nhất nhánh task của BL-H01 vào nhánh nền mà task BL-H02 dùng, hoặc ghi rõ cách làm).

## Việc cần làm

1. Quyết định cách để mã BL-H01 có trong cơ sở của BL-H02 (hợp nhất cục bộ trên nhánh pilot, người dùng xác nhận) và ghi lại; KG không đổi ngoài việc người dùng chạy `generate`.
2. Lặp T4.4 với `CR-BL-H02_ai-case-generation.md`, cỡ M.
3. Lặp T4.5.

## Tiêu chí hoàn thành

- [ ] Như T4.4 và T4.5.

## Kiểm thử

- Như T4.4 và T4.5.

## Rủi ro và ghi chú

- Hợp nhất giữa các nhánh task là việc của người dùng; Orca không merge.
