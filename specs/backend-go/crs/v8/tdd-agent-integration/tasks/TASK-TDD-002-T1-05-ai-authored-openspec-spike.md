# TASK-TDD-002-T1-05: Spike: AI tự viết bộ OpenSpec cho BL-H01 bằng KG

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 3.4, 5 (tiêu chí 2), 6 (H2)
**Checklist:** [T1.5](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P0 · **Area:** spike (không sửa Orca)
**Files:** `skills/tdd-openspec-author/SKILL.md`, `evidence/T1.5-openspec-spike.md` (mới), script kiểm `evidence/check_bundle.py` hoặc test Go nhỏ gọi `domain.ParsePlanRegion`
**Depends on:** T1.3, T1.4 (cờ), T0.3
**Status:** `[ ]` TODO

---

## Context

Đây là phép thử quyết định của hướng mới. Pipeline `tdd-agent change` đã thất bại ở chỗ phân rã CR không ổn định và gói chỉ có một task chung chung (SOL-001 2.2b F1, F7). Ở đây AI viết gói theo skill `tdd-openspec-author` với KG và mã; câu hỏi: gói có đạt cấu trúc tối thiểu và có ổn định giữa các lần không. Parser của Orca (`domain.ParsePlanRegion`) từ chối dòng lạ trong vùng `orca:begin/end`.

## Việc cần làm

1. Chuẩn bị đầu vào như Orca sẽ đưa: `request=<ref>` (ví dụ `REQ-9001`), tiêu đề, body từ `docs/crs/v5/CR-BL-H01_ai-foundation.md`, **danh sách AC có id `AC-1..AC-7`** (ánh xạ từ `AC-AI-01..07`, ghi bảng ánh xạ), Solution đã chọn (lấy từ T1.2 hoặc viết tay phương án khuyến nghị), giới hạn số task (cỡ L: 6–30).
2. Chạy agent ghi (Phụ lục A, không `--permission-mode plan`, thêm `Bash(openspec:*)`, `Write`, `Edit`) trong bản sao thử, prompt dùng `tdd-openspec-author` + `tdd-knowledge`; ghi gói vào `openspec/changes/<id>/`.
3. Sau mỗi lần: `git status` chỉ đổi dưới `openspec/changes/<id>/`; `openspec validate <id> --strict --json`; chạy `domain.ParsePlanRegion(tasks.md, "REQ-9001")` (test Go nhỏ trong `request-service`, không commit) và in `Violation`; chạy kiểm Phụ lục B (≥ 6 task, đủ bốn dòng, `Code:` hợp lệ — tệp tồn tại hoặc thư mục cha tồn tại, mỗi AC có scenario, không còn kịch bản "trả 501").
4. Lặp ≥ 3 lần (xoá gói giữa các lần); ghi từng lần: số Phase/task, id được dẫn, `Violation`, vấn đề.
5. Đọc bằng mắt một gói: task có đủ cụ thể để một agent khác làm không (đích mã, kịch bản kiểm); ghi nhận xét và ví dụ tốt/xấu.
6. Chỉnh skill theo vấn đề lặp lại; chạy lại tới khi 2 lần liên tiếp đạt; ghi `evidence/T1.5-openspec-spike.md` (mẫu bảng ở Phụ lục C).

## Tiêu chí hoàn thành

- [ ] ≥ 3 lần chạy được ghi; ít nhất 2 lần liên tiếp: `openspec validate --strict` đạt, `ParsePlanRegion` không `Violation`, ≥ 6 task đủ bốn dòng, mọi `Code:` hợp lệ, mỗi AC một scenario, không còn kịch bản "trả 501".
- [ ] Mức giống nhau giữa các lần được ghi (số Phase/task dao động bao nhiêu, id được dẫn trùng bao nhiêu).
- [ ] Một gói mẫu tốt được lưu trong `evidence/` làm fixture cho T2.4/T2.5.

## Kiểm thử

- Script/test kiểm là phần của evidence và dùng lại trong T2.5.

## Rủi ro và ghi chú

- Nếu không đạt sau vài vòng chỉnh skill, đừng tăng độ phức tạp: ghi nguyên nhân (agent bỏ qua skill, KG thiếu, đích mã không xác định) và đưa vào cổng T1.9.
- Id AC thật do Orca đưa trong prompt; ở spike chỉ mô phỏng.
