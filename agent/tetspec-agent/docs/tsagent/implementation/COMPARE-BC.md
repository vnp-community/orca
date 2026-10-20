# So sánh tầng B, C v2 với mốc v1 (T6.5, TST-21)

Mốc v1: [BASELINE-V1.md](./BASELINE-V1.md); tầng A: [COMPARE-A.md](./COMPARE-A.md). Sinh bằng `scripts/compare_baseline.py --layers abc`; kiểm tự động ở `tests/system/test_compare_v1.py`.

Chế độ `none` (không LLM). Lời văn TR, bước TC do LLM viết ở chế độ `claude-cli` không đổi số liệu ở bảng này (luật quyết định số nút; LLM chỉ điền nội dung) nên chưa chạy thật toàn bộ để giữ chi phí; xem mục chạy thật ở Giai đoạn 9.

## Chế độ `none`

Gate trước duyệt (TSO-001…): gate GB: đạt; không có vi phạm.

| Chỉ số | v1 (`none`) | v2 |
| --- | --- | --- |
| Test Requirement | 25 (L1: 1, L2: 23, L3: 1) | 39 (L1: 3, L2: 31, L3: 5) |
| Test Design | 40 | 50 |
| Coverage item | 183 | 189 |
| Test Case | 183 (L1: 2, L2: 158, L3: 23) | 189 (L1: 4, L2: 175, L3: 10) |
| TR có tiêu chí đánh số theo điều kiện | 0 | 39/39 |
| TR chép tên yêu cầu | phần lớn | 0 |
| Assertion có oracle | không có khái niệm | 190/190 |
| Test NFR vô nghĩa | có (101 %, ngưỡng 0, 6 chu kỳ) | 0 |

| # | Test case v2 | Ghi chú |
| --- | --- | --- |
| G1 | `TC-L2-SCHEDULED-TRANSFER-AC-NFR-011-1-concurrency-c`, `TC-L2-SCHEDULED-TRANSFER-AC-NFR-011-2-concurrency-c`, `TC-L2-SCHEDULED-TRANSFER-EP-CancelSchedule-concurrency-concurrent-dup-c` … (6) |  |
| G2 | `TC-L2-SCHEDULED-TRANSFER-AC-NFR-011-2-concurrency-c`, `TC-L2-SCHEDULED-TRANSFER-AC-NFR-012-1-resilience-c`, `TC-L2-SCHEDULED-TRANSFER-NFR-012-resilience-target-c` … (4) |  |
| G3 | `TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-timing-worst-INT-01-c` |  |
| G4 | `TC-L3-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-performance-at-limit-c`, `TC-L3-SCHEDULED-TRANSFER-MECH-SchedulerThrottle-performance-over-limit-c` |  |
| G5 | `TC-L2-SCHEDULED-TRANSFER-NFR-010-timing-target-c`, `TC-L2-SCHEDULED-TRANSFER-NFR-013-performance-target-c` |  |
| G6 | — | 0 test NFR vô nghĩa (TSO-016) |
| G7 | `TC-L3-SCHEDULED-TRANSFER-MECH-CatchUp-resilience-holds-c`, `TC-L3-SCHEDULED-TRANSFER-MECH-RunClaim-concurrency-parallel-c`, `TC-L3-SCHEDULED-TRANSFER-MECH-RunClaim-resilience-crash-mid-c` … (10) | MOD-Notify, INT-02, INT-03 là ngữ cảnh của lát cắt CR-001; L3 sinh cho Mechanism và phụ thuộc trong phạm vi |
| G8 | `TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-errorHandling-fail-INT-01-c`, `TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-functional-main-c`, `TC-L1-SCHEDULED-TRANSFER-SCN-ScheduledRun-timing-budget-c` … (4) | L1 cho luồng chính, luồng lỗi và ngân sách của SCN-ScheduledRun |
| G9 | — | chọn hồi quy ở Giai đoạn 7 (`tso_regression`) |
| G10 | — | BR-10 chỉ có trong srs.md, không thuộc đầu vào; cần profile Extended |
| G11 | — | 39/39 TR có tiêu chí đánh số theo điều kiện; 0 TR chép tên yêu cầu |
| G12 | — | 39/39 TR có scopeNote khi chủ thể có nhiều TR |
| G13 | — | 0 TestGap từ giả định chưa xác nhận |
| G14 | `TC-L2-SCHEDULED-TRANSFER-RSP-Cancel-401-access-c`, `TC-L2-SCHEDULED-TRANSFER-RSP-Cancel-403-access-c`, `TC-L2-SCHEDULED-TRANSFER-RSP-Create-401-access-c` … (12) | phân quyền cho 6 endpoint |
