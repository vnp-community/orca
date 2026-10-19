# SOL-QA-02: e2e luồng `bug` và `task` với engine native

**Bug:** [BUG-QA-02](../BUG-QA-02-bug-task-native-flow-not-covered.md)

## Phần 1: quyết định engine cho loại `light`

Đề xuất A (xem bug). Thay đổi:
- `backend-go/services/request-service/internal/domain/engine_settings.go` `EffectiveEngine(pinned, settings, profile)`: khi `profile == light` và `settings == nil` (dự án chưa lưu cài đặt) thì `native`, **không** rơi xuống `EngineDefaultsFromEnv`. `NewDefaultedEngineSettings` (cmd/server/main.go) hiện trộn mặc định triển khai vào mọi dự án: tách cờ "đây là mặc định chứ không phải cài đặt tường minh" (`ProjectEngineSettings.Explicit`) để `light` chỉ nhận cài đặt tường minh.
- Cập nhật `request-flow-registry.fixture.json` (sinh từ `flow_fixture_test.go`) nếu đổi `OpenSpecProfile`; không đổi bảng này khi chỉ đổi cách chọn engine.
- Test miền: `EffectiveEngine` bảng 11 loại × (mặc định openspec, cài đặt tường minh openspec, cài đặt native) × (ghim, không ghim).

## Phần 2: bộ e2e QA

### Python (`suite_qa_regressions.py`, không tốn AI)
| ID | Kiểm |
|---|---|
| QA-ENG-01 | Dự án dùng mặc định triển khai: tạo Request, xác nhận `bug`; `request.get.engine == "native"` |
| QA-ENG-02 | Như trên với `task` |
| QA-ENG-03 | Dự án đặt tường minh `openspec`: `bug` → `openspec`, `task` → `native` (chặn lỗi theo hướng ngược lại) |

### Playwright (`qa-bug-task-native-flow.web.e2e.ts`, chạy `run_ui_flow.py --ai-stub` hoặc `--real`)
Dự án thử được đặt `engine=native, solutionAgent=false` (hàm `configureProject`) và khôi phục sau test.

**Ca BUG** (tiêu đề chứa "crash" để classifier giả ra `bug`; AI thật tự phân loại):
1. Tạo Request qua giao diện → phân loại → xác nhận loại `Bug`, cỡ `M`.
2. Trả lời Clarification chuẩn bị nếu có.
3. Tab Analysis: chẩn đoán (`diagnosis`) sinh ra; duyệt.
4. Thang tiến độ: chặng `Classification, Analysis, Plan, Execution`, **không có Handover** (native).
5. Tab Plan: tạo Plan, thẻ duyệt `Approve plan` (không phải `task_list`), không có Approval Phase ở cỡ M.
6. (Cờ thực thi) duyệt, tab Execution theo dõi tới `completed`, không có hàng Handover/Verify; `request.engine == native`.

**Ca TASK**:
1. Tạo Request "add a subtract function" → `Task`, cỡ `S`.
2. Không có tab Analysis (`analysisKind` rỗng); tab: `Overview, Plan, Execution, History, Related`.
3. Plan sinh ngay (không có Solution); thẻ duyệt là danh sách việc (`Approve task list`).
4. Thang tiến độ: `Classification, Plan, Execution`.
5. (Cờ thực thi) duyệt, chạy, `completed`; repo fixture có thay đổi.

Hai ca dùng chung `createClassifyAndConfirm`, `generatePlanThroughTheUi` của bộ live; thêm helper `expectStagesAndTabs(f, {stages, tabs, engine})` ở `live-qa-steps.ts`.

## Tiêu chí hoàn thành
- QA-ENG-01..03 xanh trên stack có `REQUEST_DEFAULT_ENGINE=openspec`.
- Ca BUG và TASK xanh với `--ai-stub` (đến Plan chờ duyệt) và với `--real` (đến `completed`).
