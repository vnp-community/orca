# FE-REQ-SOL-040: Chọn nhánh gốc của worktree khi tạo và xem Request

> **📝 Chưa triển khai (0/5 task).** Soạn 2026-10-19 từ đọc mã.

**CR:** [CR-REQ-040](../../../../../../docs/crs/v6/request-frontend/CR-REQ-040-request-base-branch-picker.md)
**Area:** frontend / request
**Backend:** [BE-REQ-SOL-039](../../../../../backend-go/crs/v6/plan-phase-task/solutions/BE-REQ-SOL-039-request-worktree-base-branch.md) (trường `base_ref`, kênh `request.setBaseRef`, mã lỗi)
**Phụ thuộc solution:** [FE-REQ-SOL-037](./FE-REQ-SOL-037-request-detail-redesign-execution-tab-and-activity.md) (tab Execution), CR-REQ-018 đến 021 (trang Request)

---

## 1. Trạng thái hiện tại (re-verify)

Đã đọc ngày 2026-10-19 (đường dẫn dưới `frontend/src/renderer/src/`):

- `components/request/CreateRequestDialog.tsx`: `submit` gọi `callRequestRpc(REQUEST_RPC_METHODS.CREATE, {projectId, title, body, source, hints, clientRequestId}, {idempotent: true})`. Chỉ có dự án, tiêu đề, mô tả, loại, độ khẩn. `CreateRequestProjectRepoNote` hiển thị repo và host (từ `use-request-project-repos.ts`: `RequestProjectRepo`, trạng thái `idle|loading|failed|...`; `first` là repo đầu).
- Hộp thoại được dùng bởi tạo tay và bởi `CreateRequestIssueButton.tsx` (qua `create-request-from-issue.ts`: `jiraIssueToCreateParams`, `githubItemToCreateParams`).
- `components/settings/BaseRefPicker.tsx`: props `{repoId, currentBaseRef?, onSelect, onUsePrimary?}`; dùng `getRuntimeRepoBaseRefDefault`, `searchRuntimeRepoBaseRefs` (`runtime/runtime-repo-client`), `isRuntimeRepoRefSearchQueryWithinLimit` (`runtime/runtime-repo-search-bounds`), `getRuntimeEnvironmentIdForRepo`; có gợi ý nhiều remote; không bịa mặc định `origin/main`. Dùng ở Settings (`RepositoryWorktreeDefaultsSection`) và `SourceControl.tsx`.
- `shared/request-types.ts` `OrcaRequest` chưa có `baseRef`; `shared/request-wire-parsers.ts` `parseRequest` (không bao giờ ném lỗi); `shared/request-rpc-methods.ts` có `CREATE`, `FLOW_STATUS: 'request.flowStatus'`, chưa có `SET_BASE_REF`.
- Hỗ trợ runtime: `hooks/useRequestFlowSupport.ts` thăm dò một lần mỗi target và ghi `requestFlowSupport` vào store (`supported|unsupported`); chưa có cờ theo tính năng.
- `components/request/RequestOverviewTab.tsx` hiển thị thông tin Request; `components/request/execution/ExecutionFailureCard.tsx` hiển thị thẻ lỗi task với nguyên nhân (`execution-failure-cause.ts`).
- Cập nhật phiên bản lạc quan dùng `expectedVersion`; xung đột phải tải lại rồi hỏi người dùng (C12). Từ chối khi lỗi: `requestErrorMessageForError`, `backendCode`.
- i18n: khoá dạng `auto.components.request.<Component>.<key>`; lint chạy `verify:localization-catalog` và `verify:localization-coverage`.

### Correction relative to CR-REQ-040

1. CR nói "gateway cũ thì ẩn ô": thực tế gateway cũ **bỏ qua** tham số lạ, tức nếu không chặn trước thì người dùng chọn nhánh mà bị lặng lẽ bỏ. Cần cờ tính năng tường minh (`request.flowStatus.features` gồm `request.baseRef`, thêm ở TASK-REQ-039-05) và kiểm lại `baseRef` trả về sau khi tạo (phòng thủ chiều sâu).
2. CR đặt picker ở `components/git-ref-picker/`; `BaseRefPicker` hiện còn dùng ở `SourceControl.tsx`, nên task 02 giữ nguyên chữ ký công khai của `settings/BaseRefPicker`.

## 2. Giải pháp

### 2.1 Cây file

```
shared/request-types.ts, request-wire-parsers.ts, request-rpc-methods.ts            (sửa)
renderer/src/store/                                                                  (sửa nhỏ) requestFlowFeatures
renderer/src/hooks/useRequestFlowSupport.ts                                          (sửa) ghi features
renderer/src/components/git-ref-picker/RepoRefPicker.tsx, use-repo-ref-search.ts     (mới, tách từ settings/BaseRefPicker.tsx)
renderer/src/components/settings/BaseRefPicker.tsx                                   (sửa) lớp mỏng dùng RepoRefPicker
renderer/src/components/request/StartFromField.tsx, start-from-state.ts              (mới)
renderer/src/components/request/CreateRequestDialog.tsx                              (sửa)
renderer/src/components/request/RequestBaseRefRow.tsx                                (mới) + RequestOverviewTab.tsx (sửa)
renderer/src/components/request/execution/ExecutionFailureCard.tsx                   (sửa) hành động "Change starting branch"
renderer/src/i18n/locales/{en,es,ja,ko,zh}.json                                      (sửa)
tests/stages/e2e/orca-flows/ui-live/request-base-branch.web.e2e.ts                  (mới)
```

### 2.2 Dữ liệu và hỗ trợ

`OrcaRequest.baseRef?: string`, `baseRefLocked?: boolean`, `baseRefLockReason?: 'proposal'|'task'|'terminal'|''`, `baseRefUnverified?: boolean`. `parseRequest`: chuỗi thiếu → `undefined` (không `''`, để phân biệt "gateway cũ" với "mặc định"), cờ thiếu → `undefined`. Store: `requestFlowFeatures: ReadonlySet<string>` ghi bởi `useRequestFlowSupport` từ `request.flowStatus.features`. `supportsBaseRef = features.has('request.baseRef')`.

### 2.3 `StartFromField`

Trạng thái: `default` (chip "Default branch (`<tên>`)" khi phân giải được mặc định, ngược lại "Default branch") hoặc `custom(ref)`. Mở popover `RepoRefPicker` cho **repo đầu tiên của dự án** (`use-request-project-repos`). Đổi dự án → về `default`. Chỉ gửi `baseRef` khi `custom`. Ẩn ô khi `!supportsBaseRef`, khi dự án chưa có repo hoặc khi đang tải danh sách repo (không nhấp nháy). Host offline hoặc lỗi tìm ref: dòng giải thích và cho nhập tay (kiểm cú pháp bằng cùng bộ quy tắc backend: không bắt đầu `-`, không `..`, ...; thông điệp lỗi cụ thể).

### 2.4 Overview, khoá và lỗi

`RequestBaseRefRow`: "Worktrees start from `<ref>`" hoặc "Default branch"; bút khi `!baseRefLocked` và runtime hỗ trợ; ổ khoá và tooltip theo `baseRefLockReason` khi khoá. Lưu bằng `request.setBaseRef` kèm `expectedVersion`; `REQUEST_VERSION_CONFLICT` → tải lại và hỏi; `REQUEST_BASE_REF_LOCKED` → toast trung tính + làm mới; `INVALID`/`NOT_FOUND` → lỗi dưới ô (kèm gợi ý). `baseRefUnverified` → dòng cảnh báo "We couldn't reach the host to check this branch. It will be checked when the worktree is created.".

### 2.5 Lỗi lúc tạo worktree

`ExecutionFailureCard` (và thẻ lỗi plan nếu có): khi nguyên nhân là `base_ref_not_found` (từ `request.executionSummary` `failed[].lastCause` hoặc mã lỗi của Plan) và `!baseRefLocked`, hiện nút "Change starting branch" mở cùng popover.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| F1 | Tách `RepoRefPicker`, giữ `settings/BaseRefPicker` làm lớp mỏng | Không đổi Settings và SourceControl |
| F2 | Cờ tính năng tường minh từ `flowStatus` + kiểm `baseRef` echo | Gateway cũ không được lặng lẽ bỏ nhánh |
| F3 | `undefined` ≠ `''` trong parser | Phân biệt gateway cũ với mặc định |
| F4 | Mặc định là chip, không điền sẵn tên | Không gửi giá trị khi không chọn |
| F5 | Sửa ở Overview, không ở nơi khác | Một nơi, khoá giải thích được |

## 4. Phụ thuộc và thứ tự

Cần backend TASK-REQ-039-02 và 039-05 (kênh, cờ tính năng). Thứ tự: 01 → 02 → 03 và 04 (song song) → 05.

## 5. Kiểm thử

- Parser: thiếu/sai kiểu. Store/hook: ghi features. Component: `StartFromField` (mặc định, chọn, đổi dự án, offline, nhập tay, không hỗ trợ), `RequestBaseRefRow` (sửa được, khoá theo lý do, xung đột, các lỗi), thẻ lỗi.
- Picker dùng chung: test hiện có của `BaseRefPicker` giữ nguyên và xanh.
- E2E trực tiếp (`ui-live`, `run_ui_flow.py --ai-stub`) trên repo fixture tài chính: tạo Request với nhánh khác, kiểm `request.get.baseRef`; khoá sau khi có worktree; nhánh không tồn tại; gateway không hỗ trợ (giả lập bằng cờ tính năng tắt).

## 6. Rủi ro và điểm chưa kiểm chứng

- Tách picker có thể đụng `SourceControl.tsx` (file lớn); chạy `gitnexus_impact` cho `BaseRefPicker`.
- Hiệu năng tìm ref trên host SSH chưa đo.
- Dự án nhiều repo: ô chỉ cho repo đầu tiên; ghi rõ repo trong nhãn.

## 7. Câu hỏi mở

Như CR-REQ-040 mục 7 (SHA đã phân giải, nhớ lựa chọn gần nhất, hiện nhánh ở tab Execution). Mặc định tạm: không làm cả ba.

## 8. Tham chiếu

- Các file ở mục 1; [CR-REQ-040](../../../../../../docs/crs/v6/request-frontend/CR-REQ-040-request-base-branch-picker.md); [BE-REQ-SOL-039](../../../../../backend-go/crs/v6/plan-phase-task/solutions/BE-REQ-SOL-039-request-worktree-base-branch.md); kế hoạch tổng: [`IMPLEMENTATION-PLAN-CR-REQ-039-042`](../../../../../IMPLEMENTATION-PLAN-CR-REQ-039-042.md)
