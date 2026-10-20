# FE-REQ-TASK-043-05: Công tắc chế độ thực thi theo dự án trong Project Settings

**From Solution:** [FE-REQ-SOL-043](../solutions/FE-REQ-SOL-043-approve-plan-and-run-all-steps-ui.md)
**Priority:** P2
**Area:** frontend / project
**File:** `frontend/src/renderer/src/hooks/useProjectEngineSettings.ts` (mới), `frontend/src/renderer/src/components/project/ProjectRequestExecutionSection.tsx` (mới), `frontend/src/renderer/src/components/project/ProjectSettings.tsx` (sửa một dòng nhúng), `frontend/src/shared/request-run-all.ts` (parse cài đặt engine) và test
**Depends on:** FE-REQ-TASK-043-01
**Status:** [x] DONE (2026-10-20)

---

## Context

- Hôm nay chế độ chỉ đặt được bằng kênh WS `request.engineSettingsSet` (admin) hoặc biến môi trường; `RELEASE-2026-10-DEPLOY.md` mục 4.3 và dòng 207 nói rõ chưa có màn hình. `frontend/src` không gọi hai kênh này.
- Kênh `request.engineSettings` trả `{projectId, engine, openspecMinVersion, solutionAgent, executionMode, version, preflight?}`. `request.engineSettingsSet` nhận `{projectId, engine, openspecMinVersion, solutionAgent, executionMode, expectedVersion}`, **chỉ admin** (`channels_request_engine.go:83-87`); `engine` rỗng thành `native`, `solutionAgent` là bool trần, chỉ `executionMode` rỗng là giữ nguyên (`manage_project_engine_settings.go:93-118`). Vì vậy phải đọc rồi ghi lại cả bộ.
- Xung đột phiên bản trả `REQUEST_ENGINE_SETTINGS_VERSION_CONFLICT` (`domain/engine_errors.go:19`).
- Mẫu mục có lưu ở tab General: `ProjectJiraMappingSection.tsx` (`translate`, `toast`, `describeError`). Vai trò admin: `useAppStore.getState().currentUser?.role === 'admin'`. `ProjectSettings.tsx` còn nhiều chuỗi cứng và một `TODO` có sẵn: không sửa phần cũ.
- Mặc định triển khai (`REQUEST_DEFAULT_EXECUTION_MODE`) áp cho dự án chưa có dòng cài đặt (`version == 0`); lưu sẽ ghi cả engine hiện hành vào dòng đó (R5 của CR).

## Việc cần làm

1. `request-run-all.ts`: `parseEngineSettings(raw)` → `{projectId, engine, openspecMinVersion, solutionAgent, executionMode: 'step_by_step' | 'continuous', version} | null` (thiếu/sai kiểu → `null`; không ném lỗi).
2. `useProjectEngineSettings(projectId)`: trạng thái `loading | ready | forbidden | unsupported | error`; `reload()`; `saveExecutionMode(mode)` gọi `ENGINE_SETTINGS_SET` với toàn bộ giá trị vừa đọc và `expectedVersion = version`; xung đột → `reload()` rồi báo "Someone else changed these settings. The latest values were loaded."; kênh không tồn tại (gateway cũ) → `unsupported`; `FORBIDDEN` → `forbidden`.
3. `ProjectRequestExecutionSection`: hai lựa chọn "Ask before each step" / "Run all steps after one approval" (radio hoặc `Select` của `components/ui`), mô tả "Applies to requests that start running from now on. Requests already running keep their mode.", ghi chú khi chọn chạy hết "Orca will start each step with the access of the person who approves the plan."; admin sửa được, người khác chỉ đọc kèm "Only an administrator can change this."; khi `version === 0` thêm dòng nhỏ "Saving also stores this project's current engine settings."; thành công → `toast.success`; có trạng thái tải, lỗi (nút Retry), `unsupported` (không hiển thị mục).
4. `ProjectSettings.tsx`: thêm đúng một dòng nhúng `<ProjectRequestExecutionSection projectId={projectId} />` sau `ProjectJiraMappingSection` trong tab General.
5. Chuỗi dùng `translate('auto.components.project.ProjectRequestExecutionSection.<key>', ...)`; i18n 5 locale ở task 06.

## Kiểm thử

- Parser: thiếu/sai kiểu.
- Hook: đọc; lưu gửi đủ bộ (`engine`, `openspecMinVersion`, `solutionAgent` nguyên văn) và `expectedVersion`; xung đột tải lại; `forbidden`; `unsupported`.
- Component: admin đổi và lưu; không admin chỉ đọc; tải/lỗi/Retry; `version === 0` có dòng ghi chú; không hiển thị khi `unsupported`.
- `ProjectSettings.test` hiện có xanh.

## Tiêu chí hoàn thành

- [x] Admin đổi chế độ dự án từ giao diện mà không làm đổi engine, `openspecMinVersion`, `solutionAgent`. (`useProjectEngineSettings.test.ts` "saves the new mode together with everything it read" và `ProjectRequestExecutionSection.test.tsx` "lets an administrator switch and save, keeping the other settings as read" với engine `openspec`, `1.2.3`, `solutionAgent: true`; chưa chạy trên stack).
- [x] Người không phải admin thấy giá trị và lý do không sửa được. ("shows anyone else the value read-only with the reason").
- [x] Request đã ghim không bị đổi (mô tả đúng trong giao diện). (dòng mô tả "Applies to requests that start running from now on. Requests already running keep their mode."; việc không đổi Request đã ghim là hành vi backend, chưa kiểm trên stack).

## Rủi ro và lưu ý

- Chạy `gitnexus_impact` cho `ProjectSettings`.
- Chưa rõ vai trò admin ở client có luôn khớp `callerIsAdmin` của backend (admin toàn hệ thống); lỗi `FORBIDDEN` từ backend vẫn được hiển thị đúng.
- Câu hỏi mở 5 của CR: chủ dự án không phải admin chưa đổi được.

## Ghi chú triển khai (2026-10-20)

**File** (`frontend/src/`): mới `renderer/src/hooks/useProjectEngineSettings.ts` (+ test, 11), `renderer/src/components/project/ProjectRequestExecutionSection.tsx` (+ `__tests__/ProjectRequestExecutionSection.test.tsx`, 9); `shared/request-run-all.ts` thêm `parseEngineSettings`; `ProjectSettings.tsx` thêm đúng một import và một dòng nhúng sau `ProjectJiraMappingSection`.
- **Ngoài danh sách**: `components/project/__tests__/ProjectSettings.test.tsx` — thêm mock cho mục mới (như các mục khác) và một test "embeds the request execution section in the General tab, after the Jira mapping". File test này **đã đỏ sẵn trên HEAD** (8/8 ca báo `useConfirmationDialog must be used inside ConfirmationDialogProvider` vì `ProjectSettings` gọi hook xoá dự án mà test không bọc provider); tôi thêm mock `../../confirmation-dialog` nên nay 9/9 xanh.

**Thiết kế**
- Hook: trạng thái `loading | ready | forbidden | unsupported | error` (kênh không có hoặc `feature_off` → `unsupported`, mục không hiển thị); `saveExecutionMode` gửi cả bộ vừa đọc + `expectedVersion`; `REQUEST_ENGINE_SETTINGS_VERSION_CONFLICT` → đọc lại, trả `conflict`; token đọc chống kết quả muộn của dự án cũ; khoá lưu bằng `useRef`.
- Mục dùng `SettingsSegmentedControl` có sẵn (`role=radiogroup`/`radio`) vì `components/ui` không có radio; bản nháp chỉ là state `draft`, giá trị hiển thị còn lại là giá trị backend trả. Admin theo `currentUser.role === 'admin'`; `FORBIDDEN` từ backend biến mục thành chỉ đọc cho dự án đó. Dòng `version === 0` chỉ hiện với người sửa được.
- `parseEngineSettings`: `engine` bắt buộc là chuỗi (ghi lại nguyên văn, kể cả rỗng), `openspecMinVersion`/`solutionAgent`/`version` thiếu thì lấy mặc định proto3, `executionMode` rỗng hoặc thiếu = `step_by_step`, giá trị lạ → `null`. Đối chiếu với `EngineSettingsView` của gateway (`channels_request_engine.go`): các khoá `projectId, engine, openspecMinVersion, solutionAgent, executionMode, version`.

**Blast radius**: `ProjectSettings` chỉ được dựng bởi `ProjectSwitcher` (trang Workspace). Không đổi mục nào khác trong tab General; `react-doctor` có 1 cảnh báo có sẵn ở `ProjectSettings.tsx:95` (không phải của thay đổi này).

**Chưa kiểm**: giao diện thật trong hộp thoại (độ rộng của control, cuộn); chưa chạy trên stack. Admin ở client khớp backend (`callerIsAdmin`) chưa kiểm.

**Kiểm chứng**: `npx vitest run --config config/vitest.config.ts src/renderer/src/hooks/useProjectEngineSettings.test.ts src/renderer/src/components/project`: 12 file, 91 test xanh.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
