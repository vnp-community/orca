# TASK-TDD-002-T4-07: Ghi số liệu pilot và đối chứng

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 5 (tiêu chí 7)
**Checklist:** [T4.7](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P1 · **Area:** báo cáo
**Files:** `evidence/T4.7-pilot-report.md` (mới)
**Depends on:** T4.4, T4.6, T2.14
**Status:** `[ ]` TODO

---

## Context

Trả lời câu hỏi "hướng này có đáng không": số liệu thật thay cho phán đoán.

## Việc cần làm

1. Tổng hợp theo giai đoạn: thời gian, số lượt gọi mô hình và tool KG, chi phí, số lần phải can thiệp ngoài gate, số task phải sửa tay hoặc chạy lại.
2. Đối chứng: Solution có KG so với `ai.complete` (T1.2); Plan từ OpenSpec so với Plan `ai.complete` cũ cho cùng Request (nếu chạy được trên project thử): độ cụ thể, số task chạy được ngay, truy vết `Serves`.
3. Chất lượng gói: tỷ lệ task đủ bốn dòng, số lần thử lại, lý do thất bại.
4. Ghi những điều skill/prompt đã đổi và điều còn yếu.

## Tiêu chí hoàn thành

- [ ] Có báo cáo một trang với bảng đối chiếu từng tiêu chí SOL-002 mục 5 (đạt/không đạt, số liệu).

## Kiểm thử

- Rà bởi người dùng.
