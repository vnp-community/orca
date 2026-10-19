# TASK-TDD-002-T1-09: Cổng go/no-go sau Giai đoạn 1

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 5, 6
**Checklist:** [T1.9](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P0 · **Area:** quyết định
**Files:** `evidence/T1.9-gate.md` (mới)
**Depends on:** T1.1, T1.2, T1.3, T1.4, T1.5, T1.6, T1.7, T1.8
**Status:** `[ ]` TODO

---

## Context

Cổng quyết định có sửa Orca (Giai đoạn 2) hay không. Mục tiêu: không đầu tư 15 task code trên một giả định chưa kiểm chứng; bài học của spike pipeline là "qua luật không có nghĩa là dùng được".

## Việc cần làm

1. Tổng hợp số liệu từ các task T1.1 đến T1.8 vào một bảng một trang.
2. Chấm theo tiêu chí: agent dùng được MCP và skill (T1.3, T1.4); gói OpenSpec đạt cấu trúc và ổn định (T1.5); KG có ích: recall ≥ 80% (T1.1) và Solution tốt hơn hoặc không tệ hơn (T1.2); đã có quyết định ghi ngược (T1.7) và nơi lưu gói (T1.8).
3. Quyết định: GO (sang Giai đoạn 2), GO có điều kiện (nêu điều kiện), hoặc NO-GO kèm hướng thay (chỉnh skill/prompt, bật prose cho KG, chỉ làm phần đọc cho Solution, dừng).
4. Người dùng xác nhận; ghi vào `evidence/T1.9-gate.md` và cập nhật tiến độ ở checklist.

## Tiêu chí hoàn thành

- [ ] Có bảng số liệu và quyết định được người dùng xác nhận bằng văn bản.
- [ ] Nếu GO có điều kiện: mỗi điều kiện gắn với một task.

## Kiểm thử

- Không có test.

## Rủi ro và ghi chú

- Không bắt đầu T2.x trước khi có quyết định GO.
