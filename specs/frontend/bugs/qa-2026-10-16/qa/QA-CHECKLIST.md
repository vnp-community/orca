# Checklist QA thủ công và tự động: 5 lỗi 2026-10-16

Dùng sau mỗi bản sửa. `[ ]` chưa kiểm, `[x]` đạt, `[!]` lỗi (ghi chú dưới mục). Tự động hoá nằm ở cột "Lệnh". Không bao giờ dán token vào ghi chú.

## 0. Chuẩn bị

- [ ] Stack local chạy (`deploy/dev`), `python3 tests/stages/e2e/orca-flows/run_acceptance.py platform devserver` xanh (11 + 7 test).
- [ ] Đăng nhập web `http://localhost:28080` bằng tài khoản quản trị.
- [ ] Máy dev server là máy này, `dev-local-mac` ở trạng thái Connected (Settings → Dev servers).
- [ ] Xoá Request thử và dự án thử sau khi chạy (bộ test tự dọn; kiểm bằng `report_orphan_requests.py`).

## 1. Terminal mở đúng thư mục (BUG-QA-01)

| # | Bước | Mong đợi | Đạt |
|---|---|---|---|
| 1.1 | Bấm worktree primary của một dự án (thanh bên), chờ terminal | Dấu nhắc hiện tên thư mục repo (không phải `~`); `pwd` = đường dẫn repo | [ ] |
| 1.2 | Bấm một worktree phụ (nhánh khác) | `pwd` = thư mục worktree đó | [ ] |
| 1.3 | Mở terminal thứ hai trong cùng worktree (`+`) | Cũng ở đúng worktree | [ ] |
| 1.4 | Tải lại trang, mở lại terminal của worktree | Vẫn đúng thư mục (không về `~`) | [ ] |
| 1.5 | Xoá thư mục worktree trên đĩa rồi mở terminal | Có thông báo "directory not found … opened your home directory"; không im lặng | [ ] |
| 1.6 | Terminal chạy agent (Launch agent) trong worktree | Agent khởi động với thư mục worktree | [ ] |
| 1.7 | Dev server qua SSH (nếu có) | Đường dẫn của máy xa, không phải của máy trình duyệt | [ ] |
| Lệnh | `run_acceptance.py devserver qa` → QA-TERM-01..03; `ORCA_E2E_LIVE=1 npx playwright test -c tests/configs/playwright.web.config.ts --project=request-live qa-regressions -g QA-UI-TERM-01` | xanh | [ ] |

## 2. Luồng bug và task với engine native (BUG-QA-02)

| # | Bước | Mong đợi | Đạt |
|---|---|---|---|
| 2.1 | Dự án không có cài đặt engine (dùng mặc định triển khai `openspec`): tạo Request "… crash on save", xác nhận loại Bug | Thang tiến độ: Classification → Analysis → Plan → Execution; **không có Handover**; Request `engine = native` | [ ] |
| 2.2 | Tab Analysis: chẩn đoán, duyệt | Sang Planning | [ ] |
| 2.3 | Tab Plan (cỡ M): tạo Plan | Thẻ "Approve plan"; không có Approval Phase | [ ] |
| 2.4 | Tạo Request "add a subtract function", xác nhận loại Task | Không có tab Analysis; thang: Classification → Plan → Execution; engine `native` | [ ] |
| 2.5 | Task: tab Plan | Thẻ "Approve task list" (BUG-QA-06: hiện "Approve plan" cho tới khi sửa) | [ ] |
| 2.6 | Duyệt và chạy (cần `ORCA_E2E_EXECUTE_AGENT=1`) | Execution theo dõi tới Completed; không có Handover/Verify | [ ] |
| 2.7 | Đặt dự án `openspec` tường minh (`request.engineSettingsSet`), tạo bug | Bug mới → `openspec` (có Handover); Task vẫn `native` | [ ] |
| Lệnh | `run_acceptance.py devserver qa` → QA-ENG-01..03; `run_ui_flow.py --ai-stub -- qa-bug-task-native-flow`; `run_ui_flow.py --real native-execute` (tốn AI) | xanh | [ ] |

## 3. Nguồn task GitHub (BUG-QA-03)

| # | Bước | Mong đợi | Đạt |
|---|---|---|---|
| 3.1 | Tasks → GitHub, dự án có remote `origin` GitHub | Không có dải "source hosts unavailable" vì lỗi nối dây | [ ] |
| 3.2 | Đưa trỏ chuột vào dải trạng thái | Tên host (`dev-local-mac`), không UUID; không lặp "disconnected" và "source tool unavailable" cho host đang Connected | [ ] |
| 3.3 | Chưa kết nối GitHub | Nút **Connect GitHub**; thông báo "GitHub is not connected" | [ ] |
| 3.4 | Kết nối GitHub (OAuth/PAT), làm mới | Danh sách issue/PR hiện; lọc "Assigned to me" chạy | [ ] |
| 3.5 | Máy không cài `gh` | Nguồn vẫn dùng được (không phụ thuộc `gh`) | [ ] |
| 3.6 | Dev server offline | Dải nói đúng "host offline" kèm hành động kết nối lại | [ ] |
| 3.7 | Repo không có remote GitHub (repo cục bộ) | Thông báo "no GitHub remote", không phải lỗi chung | [ ] |
| Lệnh | `run_acceptance.py devserver qa` → QA-GH-01..03; `qa-regressions -g QA-UI-GH-01` | xanh | [ ] |

## 4. Nguồn task Jira `https://jr.servicehub.vn` (BUG-QA-04)

| # | Bước | Mong đợi | Đạt |
|---|---|---|---|
| 4.1 | Điền `JIRA_SITE_URL`, `JIRA_EMAIL`, `JIRA_PAT_TOKEN` trong `deploy/dev/.env` (hoặc dùng `JR_*` ở `.env` gốc) | — | [ ] |
| 4.2 | Chạy `deploy/dev/scripts/connect-jira-from-env.sh` | In "connected as <tên hiển thị>"; **không** in token | [ ] |
| 4.3 | Tasks → Jira | Không còn "Connect your Jira site"; có danh sách issue | [ ] |
| 4.4 | Lọc "assigned to me", phân trang, mở một issue, xem bình luận | Chạy đúng (API v2) | [ ] |
| 4.5 | Hộp thoại kết nối thủ công với email + PAT | Thành công; email để trống cũng thành công | [ ] |
| 4.6 | Nhập PAT sai | Lỗi nói rõ site từ chối PAT; không "connected" nửa vời | [ ] |
| 4.7 | Kiểm log/phản hồi | Không có token ở bất kỳ đâu (log container, response, console) | [ ] |
| Lệnh | `run_acceptance.py devserver qa` → QA-JIRA-01..03 (máy chủ giả); `ORCA_E2E_JIRA_LIVE=1 run_acceptance.py devserver qa` → QA-JIRA-LIVE-01 (**người có `.env` chạy**); `ORCA_E2E_JIRA_LIVE=1 … qa-regressions -g QA-UI-JIRA-01` | xanh | [ ] |

## 5. Log chẩn đoán không hiện như lỗi (BUG-QA-05)

| # | Bước | Mong đợi | Đạt |
|---|---|---|---|
| 5.1 | Request → Execution → Details của task đã chạy xong (như #1252) | Cuối log **không có dòng đỏ** `[claude-code:unrecognized_model]` | [ ] |
| 5.2 | Mở "Diagnostics (n)" | Thấy nội dung gốc, tông trung tính, kèm gợi ý cập nhật CLI | [ ] |
| 5.3 | Task thất bại thật (agent lỗi) | Lỗi vẫn đỏ và nổi bật | [ ] |
| 5.4 | Nâng `claude` lên bản biết `claude-sonnet-5-5` | Không còn chẩn đoán | [ ] |
| Lệnh | `npx vitest run --config config/vitest.config.ts src/renderer/src/components/task/agent-activity` (bỏ `.fails` sau khi sửa); `run_ui_flow.py --real native-execute` (QA-UI-LOG-01) | xanh | [ ] |

## 6. Hồi quy chung

- [ ] `run_acceptance.py` toàn bộ: không thêm FAIL so với trước (suite `qa` là suite mới).
- [ ] Vitest vùng đã sửa (terminal-pane, task-source, agent-activity, jira) xanh; `tsc` không thêm lỗi mới.
- [ ] Đã rà bản dịch năm locale cho chuỗi mới.
