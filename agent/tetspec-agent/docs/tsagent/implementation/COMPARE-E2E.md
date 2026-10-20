# So sánh đầu-cuối v2 với v1 trên workflow MiniPay CR-001 (T9.1)

Chạy 2026-10-12. Đầu vào là đầu ra Claude **thật** của lần chạy workflow trước (2026-10-09, chế độ `claude`): patch TDD đã duyệt, `solution.md`, `supplement.md` và `review.json` của phiên testspec-agent. Chỉ bước `testspec` và `gate_testspec` chạy lại với tsagent v2 (`TSAGENT_LLM=claude-cli`), trên một bản sao của minipay-demo:

1. `run_generate`: lát cắt KG → `tdd-kg/v1` → `start_run` → đọc ba tài liệu, coverage, validation, traceability, `list_gaps`.
2. `run_generate(supplement=…)`: gửi thêm nguồn bổ sung, Draft ghi lại tại chỗ.
3. `approve_and_baseline`: `approve_nodes` + `baseline_spec` qua MCP (mô phỏng người duyệt ở `gate_testspec`).

Bỏ qua: chạy lại các phiên Claude phía trước (impact, kg_update, solution) vì đầu ra của chúng được dùng lại nguyên văn; tình huống từ chối ở `gate_testspec` (resume phiên testspec-agent) đã có test ở chế độ mock (`tmp/minipay-demo/tests/test_testspec.py`).

Đầu ra: [output-docs/CR-001-v2/](output-docs/CR-001-v2/README.md) là **lần chạy lại 2026-10-13** (mục cuối); v1: [output-docs/CR-001-llm/](output-docs/CR-001-llm/README.md). Đầu ra của lần chạy 2026-10-12 đã thay bằng lần chạy lại.

## Chỉ số

| Chỉ số | v1 (`claude-cli`, 2026-10-09) | v2 (`claude-cli`, 2026-10-12) |
| --- | --- | --- |
| Test Requirement | 25 | 64 (L1 3, L2 56, L3 5) |
| Test Design | 40 | 94 |
| Coverage item | 183 | 258 |
| Test Case | 183 | 258 (L1 4, L2 243, L3 11) |
| TR có tiêu chí đánh số theo điều kiện | 0 | 64/64 |
| TR chép tên yêu cầu | phần lớn | 0 |
| TR có `scopeNote` khi chủ thể có nhiều TR | 0 | 64/64 |
| Assertion có oracle | không có khái niệm | 259/259 |
| Test NFR vô nghĩa (101 %, "ngưỡng + 1" cho chỉ tiêu 0) | có | 0 (TSO-016) |
| Nút "chờ xác nhận" | 123 (lan qua Integration dùng chung) | 87 điều kiện bị 58 TestGap chặn, mỗi gap có câu hỏi và owner |
| Điều kiện LLM đề xuất thêm (cần duyệt) | 0 | 21 obligation `llmProposal` |
| Lỗi chặn khi sinh | 0 | 0 (2 luật chặn baseline: TSO-007, TSO-020, được giải ở cổng duyệt) |
| Trạng thái cuối | Draft (chưa qua cổng) | Baselined v1, 3.395 nút |
| Lượt gọi LLM của tsagent | khoảng 44 | lượt 1: 110 (A 32, B 35, C 43); lượt bổ sung: 34 mới + 73 cache |
| Thời gian | — | lượt 1: 52 phút (tuần tự); lượt bổ sung: 5 phút 20 giây (`TSAGENT_LLM_PARALLEL=4`, cache) |

## Khoảng trống của v1 (BASELINE-V1)

| # | Khoảng trống | v2 |
| --- | --- | --- |
| G1 | Trùng đồng thời | 18 TC `concurrency` |
| G2 | Chết giữa chừng | 7 TC (FaultModel `afterCommit`, khởi động lại) |
| G3 | Ngân sách xấu nhất | 1 TC `timing` trường hợp xấu nhất |
| G4 | Throttle | 2 TC trên MECH-SchedulerThrottle |
| G5 | Mất 99 %, 100 lần cùng lúc | 5 TC, measurement giữ nguyên văn |
| G6 | Test NFR vô nghĩa | 0 |
| G7 | Không có L3 | 11 TC L3 (MOD-Notify, INT-02, INT-03 là ngữ cảnh của lát cắt) |
| G8 | Một hành trình L1 | 4 TC L1 (luồng chính, luồng lỗi, ngân sách) |
| G9 | Hồi quy | có cơ chế (`tso_regression`, kiểm ở TST-12); chưa áp dụng vì đây là phiên bản 1 |
| G10 | BR-10 (UTC+7) | chưa: BR-10 chỉ có trong SRS, không thuộc đầu vào |
| G11 | TR chép tên, tiêu chí dồn | 64/64 TR có tiêu chí đánh số, 0 TR chép tên |
| G12 | TR chồng lấn | 64/64 có `scopeNote` |
| G13 | `pendingConfirmation` lan | 0 gap từ giả định lan qua nút dùng chung |
| G14 | Phân quyền từng endpoint | 21 TC phủ 6/6 endpoint |

## Chất lượng (mẫu)

TR `TR-SCHEDULED-TRANSFER-L2-NFR-011-reliability`:

> Kiểm ở mức L2 rằng một lần chạy lịch chuyển tiền đến hạn chỉ sinh tối đa 1 giao dịch: khi hai scheduler quét song song chỉ một bên chiếm được lần chạy, khi tiến trình khởi động lại sau lúc chiếm và xử lý lại thì không phát sinh giao dịch thứ hai, và khi lõi chuyển tiền được gọi lại với cùng Idempotency-Key thì không tạo giao dịch mới. Chỉ tiêu: 0 giao dịch trùng.
>
> Phạm vi: … Không phủ tính đúng của số tiền, lịch tính thời điểm đến hạn hay hiệu năng quét.

TC `TC-L3-SCHEDULED-TRANSFER-MECH-RunClaim-resilience-crash-mid-c`: given lần chạy đã chiếm (UPDATE đã commit) → given bật SIGKILL tại điểm `afterCommit` → when kill sau commit, khởi động lại, chờ vòng quét → then đếm giao dịch theo Idempotency-Key ≤ 1 → then lần chạy không bị bỏ sót → cleanup.

## Vấn đề phát hiện khi chạy thật (đã sửa)

| Vấn đề | Sửa |
| --- | --- |
| TSO-017 coi FR là điểm thực thi quyền khi LLM đề xuất điều kiện `access` cho FR, chặn gate | Ca "cho phép" tính từ điều kiện `functional` của chính chủ thể |
| `get_run_status` trả `error: None` khi gate chặn trước lúc bước mở giao dịch | Lấy lỗi từ `notes` |
| `list_gaps` vượt 30.000 ký tự (66 gap) | Phân trang `limit`, `offset`, `total` |
| Lượt bổ sung ghi lại Draft làm mất CR-001, chỉ còn CR ngầm định | Ghi lại Draft gộp CR cũ và mới |
| Gọi LLM tuần tự: 52 phút cho 110 lượt | `TSAGENT_LLM_PARALLEL` (gọi song song, áp tuần tự; kết quả giống hệt) |

## Kết luận

v2 không kém v1 ở chỉ số nào: nhiều TR và TC hơn có chủ đích (mỗi AC, Field, Response, Transition, Mechanism có điều kiện riêng), mọi TR có tiêu chí đánh số và phạm vi, mọi assertion có oracle, không còn test NFR vô nghĩa, phủ 12/14 khoảng trống của v1 (G9 có cơ chế, chờ phiên bản 2; G10 thiếu nguồn). Đổi lại: số nút cần duyệt lớn (3.234 nút chờ duyệt ở lượt cuối, chủ yếu tầng C do LLM viết) nên policy tự duyệt (`TSAGENT_AUTO_APPROVE`) và duyệt theo nhóm là cần thiết khi dùng thật.

## Chạy lại 2026-10-13 (repo tdd-agentic-ai, gọi LLM song song)

Cùng đầu vào (patch TDD đã duyệt, `solution.md` của lần chạy Claude 2026-10-09), tsagent chạy từ `src/tsagent` của repo tdd-agentic-ai: `TSAGENT_LLM=claude-cli`, **`TSAGENT_LLM_PARALLEL=4`**, DB và cache LLM mới (mọi lượt gọi Claude thật), chế độ `service` (không có lượt bổ sung `supplement.md`). Test specs cũ đã xóa trước khi chạy.

| Chỉ số | 2026-10-12 (tuần tự + lượt bổ sung) | 2026-10-13 (song song 4) |
| --- | --- | --- |
| Thời gian sinh | 52 phút (lượt đầu) + 5 phút 20 giây (bổ sung) | **13 phút 10 giây** (A 4 phút, B 2,5 phút, C 6 phút) |
| Lượt gọi LLM | 110 + 34 | 107 (A 30, B 36, C 41) |
| Test Requirement / Test Case | 64 / 258 | 68 / 257 (L1 4, L2 240, L3 13) |
| TR có tiêu chí đánh số, có `scopeNote`, không chép tên | 64/64 | 68/68 |
| Assertion có oracle | 259/259 | 258/258 |
| Test NFR vô nghĩa | 0 | 0 |
| Điều kiện LLM đề xuất thêm | 21 obligation | 23 obligation |
| TestGap mở / điều kiện bị chặn | 58 / 87 | 66 / 98 (chưa có lượt bổ sung nguồn) |
| Khoảng trống v1 có TC | G1 18, G2 7, G3 1, G4 2, G5 5, G7 11, G8 4, G14 21 | G1 19, G2 9, G3 1, G4 2, G5 5, G7 13, G8 4, G14 21 |
| Trạng thái cuối | Baselined v1 (`changedBy` bị mất CR-001, đã sửa D20) | Baselined v1, `changedBy = [CR-001]` |

Một nhóm ở tầng B giữ nội dung mẫu luật vì LLM trả lỗi sau lượt sửa; không ảnh hưởng gate. Số nút và điều kiện chênh nhẹ giữa hai lần là do nội dung LLM (đề xuất thêm, khoảng trống nêu ra) khác nhau giữa các lần gọi.
