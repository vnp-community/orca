# TASK-TDD-002-T3-05: Báo cáo nợ duyệt của KG

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.5, 6 (H4)
**Checklist:** [T3.5](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P2 · **Area:** tài liệu + script
**Files:** `evidence/` (script), tài liệu vận hành
**Depends on:** T1.6, T3.1
**Status:** `[ ]` TODO

---

## Context

Ghi ngược KG làm một số phần tử quay lại `needs_review`; người chủ KG cần số liệu để quyết định áp. Orca không đọc KG, nên số liệu do chính các lệnh của tdd-agent in ra.

## Việc cần làm

1. Nếu có T3.1: dùng `needs_review_before/after` do `kg apply` in; ghi vào bản ghi bàn giao thủ công hoặc qua CLI Orca khi người áp báo.
2. Script `evidence/review_debt.py`: chạy `tdd-agent -C <repo> status --json` và `review --list --json`, in số phần tử `needs_review` theo lớp; dùng trước/sau mỗi lần áp và mỗi lần `generate`.
3. Viết hướng dẫn: khi nào gom lô, khi nào hoãn áp, cách duyệt có chọn lọc theo lớp.

## Tiêu chí hoàn thành

- [ ] Có script chạy được và hướng dẫn; có số liệu "trước/sau" cho ít nhất một lần áp thật ở T4.
- [ ] Quyết định gom lô/ngưỡng được ghi.

## Kiểm thử

- Chạy script trên bản sao KG.
