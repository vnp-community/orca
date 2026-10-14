# TASK-TDD-002-T1-06: Đo nợ duyệt của ghi ngược trên bản sao KG

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 2.2, 3.5, 6 (H4)
**Checklist:** [T1.6](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P1 · **Area:** spike (không sửa Orca)
**Files:** `evidence/T1.6-needs-review.md` (mới), script Python trong `evidence/`
**Depends on:** T0.3
**Status:** `[ ]` TODO

---

## Context

Đọc `transaction5.py`: `apply_changeset` đánh dấu `needs_review` cho phần tử bị chạm và mọi phần tử nối với chúng (duyệt theo thành phần liên thông). KG có hub (`BRL-64` nối 10 yêu cầu), nên một thay đổi nhỏ có thể kéo cả vùng lớn về chờ duyệt. Cần số liệu thật, vì nó quyết định ghi ngược phải gom lô hay chỉ áp khi người đồng ý.

## Việc cần làm

1. Tạo bản sao KG riêng (`sqlite3 .backup` baseline-1 vào thư mục thử; **không** chạm KG pilot).
2. Duyệt trước một số phần tử (`tdd-agent -C <bản sao> review --ids … --decision approved --actor claude --role SA …`) để có trạng thái `approved` đo được; ghi số phần tử `approved` ban đầu.
3. Qua MCP, `changeset_propose` ba ChangeSet cỡ 1, 5, 20 thao tác (`add` một `TestReceipt`/`ChangeSpecItem` kèm `link` tới `FR-SYS-07`; ghi `report`, đặc biệt `touched`).
4. Áp thử bằng script Python dùng API của tdd-agent (`o5.apply_changeset` trên `knowledge.materialize(head)` rồi `commit_graph` trên bản sao): đếm phần tử chuyển `needs_review` và so với tập `approved` ban đầu.
5. Thử cả ChangeSet chỉ `add` phần tử mới không có cạnh tới phần tử cũ (đối chứng).
6. Ghi bảng (mẫu ở Phụ lục C) và kết luận: mức nợ chấp nhận được hay cần gom/giới hạn.

## Tiêu chí hoàn thành

- [ ] Bảng số liệu cho ≥ 3 cỡ ChangeSet và ≥ 1 đối chứng.
- [ ] Kết luận có ngưỡng đề xuất (ví dụ "áp tối đa N op một lần" hoặc "chỉ áp sau khi người duyệt").

## Kiểm thử

- Script lặp được từ thư mục `evidence/`.

## Rủi ro và ghi chú

- Chỉ làm trên bản sao; xoá bản sao sau khi đo.
- Nếu mã thật của tdd-agent (`o5_path.load()`) khác `transaction5.py`, ghi nhận và dùng mã thật.
