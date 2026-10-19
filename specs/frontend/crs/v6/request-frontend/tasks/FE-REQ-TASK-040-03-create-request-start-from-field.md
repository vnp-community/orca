# FE-REQ-TASK-040-03: Ô "Start from" trong Create request (cả từ issue Jira/GitHub)

**From Solution:** [FE-REQ-SOL-040](../solutions/FE-REQ-SOL-040-request-base-branch-picker.md)
**Priority:** P1
**Area:** frontend / request
**File:** `components/request/{StartFromField.tsx,start-from-state.ts}` (mới), `CreateRequestDialog.tsx` (sửa), `create-request-from-issue.ts` (kiểm tra, không đổi hợp đồng), test
**Depends on:** FE-REQ-TASK-040-01, 040-02
**Status:** [x] DONE (2026-10-19)

---

## Context

- `CreateRequestDialog.submit` dựng tham số tạo; hộp thoại dùng chung cho tạo tay và `CreateRequestIssueButton`.
- `CreateRequestProjectRepoNote` đã nêu repo/host của dự án (repo đầu).

## Việc cần làm

1. `start-from-state.ts`: reducer thuần `default | custom(ref)` + hành động `select`, `useDefault`, `projectChanged`, `hostOffline`.
2. `StartFromField`: chip "Default branch (`<tên>`)" (tên từ `getRuntimeRepoBaseRefDefault` của repo đầu; chưa có thì "Default branch"), popover `RepoRefPicker`, nút "Use default branch", nhập tay khi offline (kiểm `validateBaseRefSyntax`, lỗi cụ thể), dòng giải thích offline.
3. Hiển thị ngay dưới `CreateRequestProjectRepoNote`; ẩn khi `!supportsBaseRef`, dự án chưa có repo, hoặc đang tải repo.
4. `submit`: thêm `baseRef` **chỉ khi** `custom`; sau thành công gọi `verifyBaseRefEcho`; nếu `mismatch` thì toast cảnh báo rõ "This server didn't apply the starting branch.".
5. `baseRefUnverified` trong kết quả tạo → toast thông tin kèm nút mở Request.
6. Lỗi `INVALID`/`NOT_FOUND` từ backend hiển thị dưới ô (kèm gợi ý), giữ hộp thoại mở, không mất dữ liệu đã nhập.
7. Đổi dự án trong hộp thoại đặt lại về mặc định.
8. Nhãn tiếng Anh, khoá i18n mới (hoàn thiện ở task 05), `aria-label` đầy đủ; Tab/Enter/Esc hoạt động.

## Kiểm thử

- Component: mặc định không gửi `baseRef`; chọn thì gửi; đổi dự án reset; `NOT_FOUND` giữ dữ liệu; offline nhập tay; ẩn theo `supportsBaseRef`; `mismatch` hiện cảnh báo.
- Đường issue: `CreateRequestIssueButton` mở hộp thoại có ô (Jira và GitHub); PR vẫn không có nút.
- Test `CreateRequestDialog` hiện có xanh.

## Tiêu chí hoàn thành

- [x] Tạo Request với nhánh khác mặc định gửi đúng `baseRef`; không chọn thì không gửi gì.
- [x] Gateway cũ: ô ẩn, tạo Request không đổi.
- [x] Cả tạo tay và từ issue dùng chung ô.
- [x] Lỗi hiển thị đúng, không mất dữ liệu nhập.

## Rủi ro và lưu ý

- Hộp thoại có thể quá dài trên màn hình thấp; kiểm bố cục ở 3 kích thước.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.

## Ghi chú triển khai (2026-10-19)

**File đổi/tạo** (dưới `frontend/src/renderer/src/`):
- `components/request/start-from-state.ts` (mới): reducer thuần `{ref: string|null, hostOffline}` với `select | useDefault | projectChanged | hostOffline`; `baseRefToSend` chỉ trả `ref` khi người dùng đã chọn (lựa chọn rỗng thành "mặc định", không bao giờ thành ref rỗng đã ghim).
- `components/request/StartFromField.tsx` (mới): nhãn "Start from", chip "Default branch (<tên>)" (tên từ `useRepoDefaultBaseRef`, không bịa), nút "Use default branch" khi đã chọn, dòng nêu repo (repo đầu của dự án), lỗi `role=alert` dưới ô kèm gợi ý bấm được. Ẩn khi `!supportsBaseRef`, dự án chưa có repo hoặc đang tải/lỗi danh sách repo. Chọn đúng tên nhánh mặc định = không chọn (không gửi).
- `components/request/BaseRefPickerPopover.tsx` (mới, dùng chung với task 04): Popover + `RepoRefPicker` + nhập tay. Khi host offline (tra mặc định thất bại) chỉ còn dòng giải thích + ô nhập tay; luôn có nút "Type a branch name instead". `onEscapeKeyDown` chặn Radix đóng popover khi ô tìm kiếm còn chữ, nên Esc xoá chữ trước, Esc lần hai mới đóng; hộp thoại không đóng (có test).
- `components/request/StartFromManualEntry.tsx` (mới): nhập tay, kiểm bằng `validateBaseRefSyntax`, thông điệp cụ thể cho cả 17 mã (`base-ref-violation-message.ts`, mới), `aria-invalid` + `role=alert`, Enter không submit hộp thoại.
- `components/request/use-repo-default-base-ref.ts` (mới): tra tên nhánh mặc định theo repo + runtime owner, có `retry`.
- `components/request/base-ref-write-error.ts` (mới, dùng chung với 04): phân loại lỗi `request.create/setBaseRef` (`invalid`, `notFound` + gợi ý, `locked` + lý do, `conflict`, `forbidden`, `other`); đọc phần đuôi JSON ` {"suggestions":[...]}` / ` {"reason":"..."}` của gateway (tối đa 5 gợi ý, đuôi hỏng thì bỏ qua); văn bản lý do khoá.
- `components/request/create-request-submit.ts` (mới): gọi `request.create` (thêm `baseRef` chỉ khi có), `finishCreate` (store, đóng, điều hướng, rồi `verifyBaseRefEcho`: `mismatch` thì toast cảnh báo "This server didn't apply the starting branch." kèm nút Open; `baseRefUnverified` thì toast thông tin kèm nút Open). Không kiểm echo khi `created === false` (đó là Request cũ của issue, có nhánh riêng).
- `components/request/CreateRequestDialog.tsx` (sửa): tách phần gọi RPC ra `create-request-submit.ts` (giữ dưới 300 dòng), thêm reducer + ô; đổi dự án gọi `projectChanged`; `INVALID`/`NOT_FOUND` chỉ hiện dưới ô khi có gửi `baseRef`, hộp thoại giữ nguyên mọi dữ liệu đã nhập; các lỗi khác vẫn ở dòng lỗi cũ (`REQUEST_PENDING_LIMIT` giữ nguyên).
- `components/request/create-request-from-issue.ts` (sửa): `CreateRequestParams` có thêm `baseRef?` (kiểu `RequestCreateBaseRefParams`). Mapper Jira/GitHub không đổi; hộp thoại không lấy `baseRef` từ `initial` (ô tự giữ lựa chọn; nếu seed được thì có thể gửi nhánh mà ô đang ẩn).
- `runtime/request-rate-limit.ts` (+ test): `SET_BASE_REF` thuộc lớp `write` và nằm trong `IDEMPOTENT_WRITES` (có `expectedVersion`; hook 04 truyền `idempotent:false` khi không biết phiên bản).
- i18n: 55 khoá mới ở cả 5 locale (nhóm `StartFromField`, `BaseRefViolation`, `BaseRefError`, `RequestBaseRefRow`, `execution.cause.baseRefNotFound/adviceBaseRefNotFound`, `execution.ExecutionFailureCard.changeBranch`, `CreateRequestDialog.baseRefNotAppliedDetail`). Bản es/ja/ko/zh là dịch tay, **cần người bản ngữ rà soát** (đặc biệt thuật ngữ "worktree" giữ nguyên ở es, "ワークツリー"/"워크트리"/"工作树" ở ja/ko/zh).

**Test mới**: `start-from-state.test.ts` (5), `base-ref-write-error.test.ts` (9), `base-ref-violation-message.test.ts` (2, đủ 17 mã), `start-from-locale-coverage.test.ts` (đọc mọi lời gọi `translate` kể cả dạng `${T}khoá`, kiểm 5 catalog + placeholder + không sao chép tiếng Anh), `CreateRequestDialog.start-from.test.tsx` (17: mặc định không gửi, chọn thì gửi, chọn đúng tên mặc định, Use default, đổi dự án reset, `supportsBaseRef` false, không repo, NOT_FOUND giữ dữ liệu + gợi ý + gửi lại, INVALID, lỗi khác, offline nhập tay hợp lệ/không hợp lệ/rỗng, Enter không submit, Esc hai bước, mismatch, unverified, `created=false`), `CreateRequestIssueButton.start-from.test.tsx` (3: Jira, GitHub, PR không nút).

**Blast radius** (grep, không có GitNexus): `CreateRequestDialog` được dùng bởi `CreateRequestIssueButton` và `RequestPage` (props không đổi); `CreateRequestParams` chỉ thêm trường tuỳ chọn; `request-rate-limit` được `request-rpc-client` dùng, chỉ đổi phân lớp của `request.setBaseRef` (trước đó rơi vào `read`).

**Kiểm chứng đã chạy** (từ `frontend/`)
- `npx vitest run --config config/vitest.config.ts src/renderer/src/components/request src/shared src/renderer/src/components/git-ref-picker src/renderer/src/components/settings/BaseRefPicker.test.tsx src/renderer/src/runtime/request-rate-limit.test.ts src/renderer/src/store/slices/request.test.ts`: xanh, trừ 3 test không liên quan trong `src/shared` (`feature-interactions`, hai test `remote-runtime-shared-control-boundary`); `settings/mcp/McpExternalServerDialog.test.tsx` (7 test) cũng đỏ, cũng không liên quan.
- `npx oxlint -c ../.oxlintrc.json <31 file đổi/tạo>` và `--config config/oxlint-react-doctor.json`: sạch (không `max-lines`).
- `npx tsc --noEmit -p tsconfig.json --composite false --incremental false`: 113 lỗi, bằng số sau task 02, không lỗi mới (không lỗi nào ở file của task này).
- i18n: chạy `desktop/config/scripts/verify-localization-catalog.mjs` và `audit-localization-coverage.mjs --check` trên bản sao `src/renderer/src` của HEAD (`git archive`) và của cây làm việc (scratchpad `i18nverify040/{head,now}`, cùng cách task 02: script đòi `<root>/src/{main,renderer/src}` và `config/localization-coverage-allowlist.json`). Cả hai vẫn thất bại sẵn trên HEAD (lỗi có sẵn của admin/web); đầu ra **giống hệt** giữa HEAD và hiện tại, không dòng nào nhắc file/khoá của task này.

**Chưa kiểm / lưu ý**
- Chưa xem trực quan: bố cục hộp thoại ở 3 kích thước (rủi ro nêu trong task), popover trong `Dialog` với chuột thật (Radix `onPointerDownOutside`/khoá cuộn chỉ có thể kiểm bằng ứng dụng thật; test dùng `fireEvent.click`). Cần mắt người hoặc spec live ở task 05.
- Chưa chạy với gateway thật: tên `features`, đuôi JSON lỗi theo mã Go và CONTRACT 2.11.
- `useRequestProjectRepos` được gọi hai lần trong hộp thoại (ghi chú repo và ô), tức hai lần `repo.list`; chấp nhận được, không sửa hook (ngoài phạm vi).
- `RepoRefPicker` không công khai trạng thái tìm kiếm, nên "host offline" ở hộp thoại dựa vào việc tra nhánh mặc định thất bại; lỗi tìm kiếm riêng lẻ vẫn do picker hiển thị kèm Retry, và nút "Type a branch name instead" luôn có.
