# CR-REQ-040 — Giao diện chọn nhánh gốc của worktree khi tạo và xem Request

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-REQ-040 |
| **Tên** | Ô "Start from" trong hộp thoại Create request (cả khi tạo từ issue Jira/GitHub), dòng nhánh gốc kèm nút sửa ở tab Overview của Request, thông báo lỗi và trạng thái khoá |
| **Loại** | Feature |
| **Priority** | 🟠 P1 |
| **Effort** | Small đến Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-19 |
| **Trạng thái** | 🟡 Đã triển khai (2026-10-20), chưa merge; còn: kiểm cập nhật thời gian thực giữa hai tab, hai lệnh verify i18n đỏ sẵn trên HEAD |
| **Phụ thuộc** | Backend [CR-REQ-039](../plan-phase-task/CR-REQ-039-request-worktree-base-branch.md) (`request.create` nhận `baseRef`, `request.setBaseRef`, `baseRef`/`baseRefLocked` trên Request); CR-REQ-018 đến 021 (trang Request) |
| **Mở khoá** | Không |
| **Tác động** | `frontend/src/renderer/src/components/request/{CreateRequestDialog,CreateRequestProjectRepoNote,RequestOverviewTab,create-request-from-issue}.tsx`, `use-request-project-repos.ts`, picker nhánh dùng chung (tách từ `components/settings/BaseRefPicker.tsx`), `frontend/src/shared/request-types.ts`, `request-wire-parsers.ts`, `request-rpc-methods.ts`, 5 file locale |

> Nguồn: yêu cầu "cho phép người dùng chỉ định nhánh mà worktree sẽ tạo từ đó, mặc định vẫn là nhánh mặc định" ngày 2026-10-19. Mô tả hiện trạng dựa trên đọc mã, chưa dựng giao diện thử.

---

## 1. Bối cảnh và vấn đề

Hộp thoại `CreateRequestDialog.tsx` chỉ có dự án, tiêu đề, mô tả và loại; không có chỗ nói nhánh nào sẽ được dùng. `CreateRequestProjectRepoNote` đã cho biết Request sẽ chạy trên repo đầu tiên của dự án (và host) nhưng không nói nhánh. Nút "Create request" trên issue Jira/GitHub (`CreateRequestIssueButton.tsx`) mở cùng hộp thoại đó, nên cùng thiếu. Backend hiện cũng chưa có trường để gửi (CR-REQ-039 mục 1).

Giao diện chọn nhánh đã có sẵn ở nơi khác: `components/settings/BaseRefPicker.tsx` (tìm ref qua `searchRuntimeRepoBaseRefs`, lấy mặc định qua `getRuntimeRepoBaseRefDefault`, có gợi ý nhiều remote). CR này dùng lại nó thay vì viết picker mới.

## 2. Giải pháp đề xuất

### 2.1a Sửa đổi 2026-10-20: dropdown hiện nhánh mặc định và danh sách nhánh

Theo duyệt mockup: (1) khi không chọn, dropdown ghi đúng tên nhánh mặc định (như `repo.baseRefDefault` trả) kèm nhãn "default", chưa phân giải được mới ghi "Default branch"; (2) bấm vào dropdown hiện ngay danh sách nhánh của repo (mặc định ghim đầu, nhánh local, nhánh remote-only), có ô tìm kiếm lọc tại chỗ; (3) vẫn cho nhập tay tag/commit; (4) chọn lại nhánh mặc định là "không chọn" và không gửi `baseRef`. Kênh dữ liệu mới `repo.listBranches` (TASK-REQ-039-07); việc giao diện ở FE-REQ-TASK-040-06. Không đổi hợp đồng `baseRef` của Request.

### 2.1 Ô "Start from" trong Create request

- Đặt dưới `CreateRequestProjectRepoNote`, chỉ hiện khi dự án có ít nhất một repo và runtime hỗ trợ (`request.flow` / capability của CR-REQ-039; runtime cũ thì ẩn ô và không gửi `baseRef`).
- Trạng thái mặc định (**sửa đổi 2026-10-20**, xem mục 2.1a): nút hiện **tên chính xác của nhánh mặc định** kèm nhãn "default"; bản trước ghi chip **"Default branch (`main`)"**, trong đó `main` là kết quả `getRuntimeRepoBaseRefDefault` của repo đầu tiên; nếu chưa phân giải được thì chỉ ghi "Default branch", không bịa tên (cùng nguyên tắc với `BaseRefPicker`: không hiển thị `origin/main` giả).
- Bấm chip mở picker tìm kiếm nhánh/tag của repo đó (debounce, giới hạn độ dài truy vấn theo `runtime-repo-search-bounds`). Chọn một ref thì chip hiện tên ref kèm nút "Use default branch" để quay lại mặc định.
- Gửi `baseRef` trong `request.create` **chỉ khi** người dùng chọn khác mặc định; không gửi gì nghĩa là dùng nhánh mặc định, khớp quy tắc backend (rỗng = mặc định).
- Runtime hỗ trợ được nhận biết bằng `request.flowStatus.features` có `request.baseRef` (gateway cũ bỏ qua tham số lạ, nên không thể chỉ dựa vào lỗi); sau khi tạo, so `baseRef` trả về với giá trị đã gửi và cảnh báo nếu lệch.
- Đổi dự án trong hộp thoại thì xoá lựa chọn nhánh (repo khác có tập nhánh khác) và trả về mặc định.
- Dev server của repo offline: picker không tìm được; hiện dòng "Branches can't be listed while the host is offline. The default branch will be used." và cho nhập tay tên ref có kiểm tra cú pháp (backend sẽ kiểm lại, CR-REQ-039 mục 2.4).

### 2.2 Tạo từ issue Jira/GitHub

`CreateRequestIssueButton` truyền `create-request-from-issue.ts` (`jiraIssueToCreateParams`, `githubItemToCreateParams`) vào cùng hộp thoại, nên ô "Start from" xuất hiện y như tạo tay. Không tự đoán nhánh từ issue hoặc PR (câu hỏi mở 5 của backend). Với issue GitHub là PR, nút bị ẩn như hiện nay.

### 2.3 Tab Overview: xem và sửa

- Thêm một dòng "Worktrees start from `release/1.4`" (hoặc "Default branch") trong `RequestOverviewTab`.
- Khi `baseRefLocked=false` và người dùng có quyền sửa: biểu tượng bút mở cùng picker; lưu bằng `request.setBaseRef` với `expectedVersion`. Xung đột phiên bản thì tải lại rồi hỏi người dùng (C12).
- Khi `baseRefLocked=true`: hiện khoá và tooltip "A worktree already exists for this request, so its starting branch can no longer change." Không có nút sửa.
- Hiển thị cho mọi loại Request, kể cả loại không bao giờ tạo worktree (`question`, `spike`): ẩn dòng này khi luồng của loại đó không có thực thi và không có Plan OpenSpec (dựa vào `request.flow`).

### 2.4 Lỗi và trạng thái

| Mã backend | Hiển thị |
|---|---|
| `REQUEST_BASE_REF_INVALID` | lỗi dưới ô: "That isn't a valid branch or tag name." |
| `REQUEST_BASE_REF_NOT_FOUND` | lỗi dưới ô kèm tối đa 3 gợi ý (nhánh gần đúng backend trả) |
| `REQUEST_BASE_REF_LOCKED` | toast trung tính, làm mới Request, ẩn nút sửa |
| Tạo worktree thất bại do `base_ref_not_found` (từ Activity/Execution) | thẻ lỗi ở tab Execution có nút "Change starting branch" mở picker (chỉ khi chưa khoá) |

Mọi chuỗi dùng khoá i18n mới cho cả 5 locale (en, es, ja, ko, zh); nhãn tiếng Anh theo quy ước của dự án. Bản dịch es/ja/ko/zh cần người bản ngữ rà như các CR trước.

### 2.5 Mã nguồn

- Tách `BaseRefPicker` thành thành phần dùng chung (đặt tên theo nội dung, ví dụ `components/git-ref-picker/RepoRefPicker.tsx`; không dùng tên `common`/`shared`/`utils`), giữ `settings/BaseRefPicker` làm lớp mỏng để không đổi hành vi Settings.
- `request-types.ts`: `OrcaRequest.baseRef?: string`, `baseRefLocked?: boolean`. `request-wire-parsers.ts` `parseRequest`: chuỗi thiếu → `undefined`, cờ thiếu → `false`, không bao giờ ném lỗi.
- `request-rpc-methods.ts`: thêm `SET_BASE_REF = 'request.setBaseRef'`; `request.create` nhận `baseRef`.
- Mọi file mới/đổi dưới 300 dòng mã; không thêm `max-lines` disable.
- Dùng token trong `assets/main.css` và primitive shadcn của `components/ui/` (xem `guides/STYLEGUIDE.md`); phím tắt và nhãn nếu có phải theo nền tảng (không cứng `metaKey`).
- Chạy được khi repo nằm trên host SSH: mọi tìm ref qua `runtime-repo-client` như `BaseRefPicker` hiện nay.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| F1 | Dùng lại picker của Settings thay vì viết mới | Đã xử lý nhiều remote, giới hạn truy vấn, host từ xa |
| F2 | Mặc định là chip "Default branch", không điền sẵn tên nhánh | Không gửi giá trị nào khi người dùng không chọn; tránh lệch khi nhánh mặc định đổi tên |
| F3 | Sửa nhánh gốc ở Overview, không ở Create dialog sau khi đã tạo | Một nơi để xem và sửa; khoá hiển thị rõ lý do |
| F4 | Không đoán nhánh từ issue/PR | Chưa có quy tắc chốt; tránh chọn sai âm thầm |
| F5 | Runtime cũ: ẩn ô, không gửi `baseRef` | Tương thích ngược khi frontend mới chạy trên gateway cũ |

## 4. Tiêu chí chấp nhận

- [ ] Hộp thoại Create request hiện ô "Start from" với chip "Default branch" mặc định; gửi `request.create` không có `baseRef` khi không đổi.
- [ ] Chọn `release/1.4` thì `request.create` mang `baseRef="release/1.4"` và Request mới trả `baseRef` đúng ở Overview.
- [ ] Đổi dự án trong hộp thoại đưa ô về mặc định.
- [ ] Tạo từ issue Jira và issue GitHub dùng cùng ô; PR vẫn không có nút.
- [ ] Overview: sửa được khi chưa khoá; sau khi có worktree hiện khoá và không có nút sửa; xung đột phiên bản được xử lý.
- [ ] Lỗi `INVALID`, `NOT_FOUND`, `LOCKED` hiển thị đúng như bảng 2.4.
- [ ] Gateway cũ không có capability: ô ẩn, Create request vẫn hoạt động như trước.
- [ ] Dev server offline: ô báo không liệt kê được, nhập tay có kiểm cú pháp, Create request không bị chặn.
- [ ] Đủ khoá i18n cho 5 locale; kiểm tra coverage i18n xanh.
- [ ] Không file nào vượt `max-lines`; `pnpm lint` xanh; Settings → Repository vẫn dùng picker như cũ.

## 5. Kiểm thử

- Unit/component: parser (`baseRef`, `baseRefLocked` thiếu hoặc sai kiểu); `CreateRequestDialog` (không gửi `baseRef` khi mặc định, gửi khi chọn, xoá khi đổi dự án); Overview (trạng thái sửa được/khoá/quyền); ánh xạ lỗi; capability cũ.
- Picker dùng chung: test hiện có của `BaseRefPicker` vẫn xanh sau khi tách.
- E2E trực tiếp (`ui-live`, `run_ui_flow.py --ai-stub`): tạo Request qua hộp thoại với một nhánh khác mặc định trên repo tạm có hai nhánh, kiểm `request.get.baseRef`; ca khoá sau khi sinh Plan có worktree; ca nhánh không tồn tại. Ca kiểm nhánh thật của worktree thuộc suite backend của CR-REQ-039.
- Lưu ý khi chạy e2e thật: không sửa checkout trong lúc `--real` chạy (guard quiet-checkout).

## 6. Rủi ro và điểm chưa kiểm chứng

- Repo đầu tiên của dự án là repo duy nhất mà Request dùng (CR-REQ-039 R2); picker chỉ tìm trên repo đó. Dự án nhiều repo có thể gây hiểu nhầm nếu giao diện không nói rõ repo nào; `CreateRequestProjectRepoNote` đã nêu repo, đặt ô ngay dưới nó.
- Danh sách ref của repo lớn có thể chậm trên host SSH; dựa vào giới hạn và debounce của `BaseRefPicker`, chưa đo.
- Tách picker có thể đụng test Settings; tách thành lớp mỏng để giảm rủi ro.
- Chưa chạy hệ thống; mọi mô tả giao diện dựa trên đọc mã ngày 2026-10-19.

## 7. Câu hỏi còn mở

1. Có cần hiển thị SHA đã phân giải sau khi worktree được tạo (ví dụ "from `release/1.4` @ a1b2c3d") ở Overview không? Backend hiện không lưu SHA (CR-REQ-039 B4).
2. Có nhớ lựa chọn nhánh gần nhất theo dự án làm gợi ý (không phải mặc định) không? Đề xuất: không trong CR này.
3. Tab Execution có cần hiện nhánh gốc kèm tên worktree của từng task không?

## 8. Tác động tới CR hiện có

| CR | Cần sửa gì |
|---|---|
| CR-REQ-018 đến 021 | Hộp thoại tạo và tab Overview thêm ô/dòng nhánh gốc |
| CR-REQ-019-06 (nút tạo Request từ issue) | Dùng chung ô nhánh qua hộp thoại, không đổi nút |
| CR-REQ-037 | Thẻ lỗi ở tab Execution có hành động "Change starting branch" khi `base_ref_not_found` |
| CR-REQ-039 (backend) | Cung cấp trường, kênh và mã lỗi |

## 9. Tham chiếu

- `frontend/src/renderer/src/components/request/CreateRequestDialog.tsx`, `CreateRequestProjectRepoNote.tsx`, `CreateRequestIssueButton.tsx`, `create-request-from-issue.ts`, `RequestOverviewTab.tsx`, `use-request-project-repos.ts`
- `frontend/src/renderer/src/components/settings/BaseRefPicker.tsx`, `runtime/runtime-repo-client`, `runtime/runtime-repo-search-bounds`
- `frontend/src/shared/request-types.ts`, `request-wire-parsers.ts`, `request-rpc-methods.ts`
- [CR-REQ-039](../plan-phase-task/CR-REQ-039-request-worktree-base-branch.md), [CR-REQ-037](./CR-REQ-037-request-detail-redesign-execution-tab-and-activity.md)
- Mới: `components/git-ref-picker/RepoRefPicker.tsx`, test và 5 file locale
