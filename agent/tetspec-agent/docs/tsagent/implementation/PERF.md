# Hiệu năng và vận hành tsagent v2 (T9.2, TST-16, TST-20)

Đo ngày 2026-10-12 bằng `scripts/perf.py --features 50` trên máy dev (một tiến trình, SQLite WAL, ổ cục bộ), LLM `none`.

Dữ liệu giả: 50 feature, mỗi feature 300 FR (mỗi FR 2 AC), 30 NFR, 20 module, 20 endpoint (3 field, 3 response mỗi endpoint). Mỗi feature sinh 10.944 nút KG test, 889 test case. DB cuối: **3.519.851 dòng, 426 MB**.

## Kết quả

| NFR | Chỉ tiêu | Đo được | Đạt |
| --- | --- | --- | --- |
| NFR-005 | Bước luật (ingest, seed, gom, kiểm, commit, render) cho feature 300 yêu cầu ≤ 60 s p95 | p50 5,0 s; p95 5,8 s; max 5,9 s | đạt |
| NFR-006 | Truy vấn đọc qua MCP ≤ 200 ms p95 (DB ≥ 1.000.000 dòng) | xem bảng dưới: truy vấn hẹp đạt; truy vấn nạp cả phiên bản lần đầu vượt | đạt một phần |
| NFR-009 | RTO khôi phục từ snapshot ≤ 5 phút | snapshot 4,8 s; khôi phục vào tệp trống + kiểm toàn vẹn 29,4 s (DB 426 MB) | đạt |
| — | DB và WAL sau 10 lần ghi lại Draft | DB 425,9 MB không tăng; WAL ổn định 22,3 MB (autocheckpoint mặc định đủ, không cần chỉnh) | — |

Thời gian MCP (gọi tool trong tiến trình, không tính HTTP; 20 lượt mỗi tool trên 20 feature khác nhau):

| Tool | Lần đầu p50 | Lần đầu p95 | Lặp lại p95 (DB không đổi) |
| --- | --- | --- | --- |
| `list_specs` | 1,3 ms | 1,8 ms | 1,9 ms |
| `sql_select` (view `v_test_case`, 50 dòng) | 2,9 ms | 3,0 ms | 3,0 ms |
| `list_gaps` | 3,5 ms | 3,7 ms | 3,7 ms |
| `get_testcase` | 324 ms | 537 ms | 6,2 ms |
| `get_coverage` (gồm chạy gate GS) | 1.223 ms | 1.924 ms | 4,5 ms |
| `get_spec` (render cả tài liệu) | 1.416 ms | 2.300 ms | 2,1 ms |

Cache đọc của gateway khóa theo dấu trạng thái tệp DB + WAL (đổi khi có bất kỳ giao dịch ghi nào, kể cả từ tiến trình khác), nên lượt đọc lặp lại trên cùng phiên bản dưới 10 ms. Lượt đầu của `get_testcase`, `get_coverage`, `get_spec` nạp cả phiên bản (khoảng 11.000 nút) nên vượt 200 ms.

## Giá trị thay cho "đề xuất" trong TDD v2

| NFR | TDD v2 (đề xuất) | Giá trị đề xuất mới theo số đo |
| --- | --- | --- |
| NFR-005 | 60 s p95 | giữ 60 s (đo 5,8 s, còn dư cho LLM tắt và máy chậm hơn) |
| NFR-006 | 200 ms p95 | 200 ms p95 cho truy vấn hẹp và lượt lặp lại; 2,5 s p95 cho lượt đầu của tool nạp cả phiên bản (cho tới khi xong F1) |
| NFR-009 | 5 phút | giữ 5 phút (đo 34 s cho DB 426 MB) |

## Việc theo dõi

| Mã | Việc | Liên quan |
| --- | --- | --- |
| F1 | `get_testcase`, `get_coverage` đọc theo truy vấn hẹp (chỉ TC và nút liên quan; đếm bằng SQL trên view) thay vì nạp cả phiên bản; `get_spec` dùng tệp đã render khi băm khớp `rendered_file` | NFR-006 lượt đầu |
| F2 | Đo lại qua HTTP với nhiều client đồng thời | NFR-006 |

## TC-66: ngữ cảnh Solution / Plan trên repo `notes` (2026-10-19)

Repo `notes` (service Go nhỏ, KG 11 phần tử, TDD `TDD-NOTES`). Ngữ cảnh: Solution của Request #2 (`GET /healthz`, phương án `opt-1`, 3 giả định, 3 câu hỏi) và OpenSpec change `req-2-them-api` (`tasks.md`, `proposal.md`, `design.md`, `specs/healthz/spec.md`). Chạy qua MCP stdio thật (`testspec_get` → `testspec_start` → `run_dispatch` → `run_get`).

| Cấu hình | TR | TC | TestGap | Ghi chú |
| --- | --- | --- | --- | --- |
| `llm = none`, không ngữ cảnh | 3 | 3 | 2 | `BRL-01-no-ac`, `BRL-02-no-ac` |
| `llm = none`, `solution` | 3 | 4 | 3 | thêm `FR-NOTE-03-ASM-REQ-2-A-2` (giả định 405 chưa xác nhận); A-1, A-3 là ràng buộc, không thành gap; có điều kiện từ tiêu chí AC-1 |
| `llm = none`, `plan` | 3 | 9 | 4 | 6 tiêu chí (2 task, 1 scenario, ...), 2 giả định từ mục Rủi ro / Đánh đổi của `design.md` thành TestGap |
| `claude-cli`, không ngữ cảnh (lần chạy trước) | 6 | 10 | 8 | 15 lượt gọi LLM; gap hỏi lại định dạng phản hồi, xác thực của `FR-NOTE-03` |
| `claude-cli`, `solution` | 7 | 16 | 9 | 94 s; gap định dạng/xác thực của `FR-NOTE-03` **biến mất**; test case dẫn nguồn `ADR-REQ-2-opt-1`, `CON-REQ-2-A-3`, `AC-REQ-2-AC-1` (body `{"status":"ok"}`, client ẩn danh, mutex khóa); gap mới là câu hỏi từ Solution (HEAD, trace `FR-NOTE-03` → `ENT-01` mâu thuẫn `BRL-02`) |

Chưa đo: `claude-cli` với `plan`; chi phí tiền (không đo). Số TestGap không giảm tổng thể vì ngữ cảnh cũng đưa thêm câu hỏi thật (A-2, HEAD); điều đo được là các gap đã trả lời biến mất và test case có nguồn từ Solution.

