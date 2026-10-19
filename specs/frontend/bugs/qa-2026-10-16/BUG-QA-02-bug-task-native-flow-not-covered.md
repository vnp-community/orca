# BUG-QA-02: Luồng phân loại `bug` và `task` với engine native chưa có e2e, và `bug` đang chạy engine OpenSpec

**Mức độ:** P1 (khoảng trống kiểm thử + một lệch so với quyết định sản phẩm) · **Khu vực:** request-service (`EffectiveEngine`), bộ e2e · **Trạng thái:** đã sửa 2026-10-16 (mặc định của deployment chỉ chuyển change_request/refactor sang openspec); QA-ENG-01..03, UI bug/task native 3/3 và chạy thật native-execute 2/2 xanh

## Yêu cầu

Có e2e cho luồng phân loại ra `bug` và `task` với **engine native** (quyết định 2026-10-16: chỉ `change_request` và `refactor` chạy OpenSpec; mọi loại khác là native).

## Rà soát

- Bộ UI live hiện có (`request-live-flow`, `execute-real`) chỉ phủ `change_request` và `question`; `task` có ca thực thi thật nhưng không kiểm engine, không kiểm thang tiến độ, tab, thẻ duyệt theo luồng; `bug` chỉ có phân tích chỉ đọc (`readonly-real`), không có Plan → thực thi.
- Bộ Python `MTX` kiểm bảng luồng (`request.flow`) nhưng không kiểm `engine` của Request, không đi tới Plan.

## Lệch tìm thấy khi rà soát (cần quyết định)

Trên stack local, `deploy/dev/.env` đặt `REQUEST_DEFAULT_ENGINE=openspec`. Request #1240 (**bug**) trả `engine = "openspec"` (`request.list`), trong khi `task` (#1245) là `native`. Nguyên nhân: `domain.EffectiveEngine` chỉ trả `native` khi `OpenSpecProfile == none`; `bug`, `security`, `performance` có profile `light`, nên theo mặc định của triển khai họ chạy OpenSpec. Điều này trái quyết định "mọi loại khác là native" và làm tab Execution của `bug` hiện hàng Handover (thang tiến độ có chặng Handover) dù người dùng mong đợi native.

Hai cách xử lý (cần chủ sản phẩm chọn, đề xuất A):
- **A (đề xuất):** `REQUEST_DEFAULT_ENGINE` chỉ áp cho profile `full` (`change_request`, `refactor`). Loại `light` chỉ chạy OpenSpec khi **cài đặt dự án** nói rõ (`request.engineSettingsSet`).
- **B:** giữ nguyên, đổi tài liệu thành "`bug`, `security`, `performance` theo mặc định của triển khai", và test chạy với dự án đặt `native` tường minh.

## Test e2e phục vụ QA (xem solutions/SOL-QA-02)

- `suite_qa_regressions.py`: QA-ENG-01 (`bug` → native dưới mặc định openspec; **đỏ cho tới khi chọn A**), QA-ENG-02 (`task` → native).
- `ui-live/qa-bug-task-native-flow.web.e2e.ts`: đi từ tạo Request tới Plan chờ duyệt, kiểm chứng trên giao diện: loại, engine, thang tiến độ, tab, thẻ duyệt, không có Handover; bản có agent thật chạy thực thi (cờ `ORCA_E2E_EXECUTE_AGENT=1`).
