# TASK-TDD-002-T3-02: Lưu và hiển thị đề xuất ghi ngược trong kết quả task

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.5
**Checklist:** [T3.2](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P1 · **Area:** task-service + request-service + orca-cli
**Files:** kết quả thực thi task (T2.11), `orca-cli` (`flow_render.go`, lệnh xem task/Request), gateway view
**Depends on:** T2.11
**Status:** `[ ]` TODO

---

## Context

Skill `tdd-knowledge-writeback` dặn agent liệt kê vùng bị ảnh hưởng, sửa tài liệu cho sự kiện từ tài liệu và gọi `changeset_propose` cho sự kiện chỉ có trong KG; kết quả cuối ghi `writeback{documents[], proposal_id, proposal_ok, touched}`. Orca không áp gì: chỉ lưu và cho người xem để quyết định.

## Việc cần làm

1. Đảm bảo `writeback` từ khối kết quả (T2.11) được lưu cùng bản ghi thực thi của task (JSON, giới hạn kích thước).
2. Gateway/CLI: hiển thị theo task và gộp theo Request: danh sách tài liệu đổi, `proposal_id` và `ok`, số id bị chạm, cảnh báo nếu `proposal_ok=false`.
3. Gate (Phase hoặc hoàn tất) nêu số đề xuất đang chờ áp.
4. Không thêm bất kỳ đường nào cho Orca tự gọi `kg apply`.

## Tiêu chí hoàn thành

- [ ] Sau một task có ghi ngược, `orca` CLI cho thấy đề xuất và tài liệu đã đổi.
- [ ] Request không có ghi ngược hiển thị rõ "không có".

## Kiểm thử

- Test lưu và render; test không có đường áp tự động (kiểm bằng tìm kiếm mã và test hợp đồng).
