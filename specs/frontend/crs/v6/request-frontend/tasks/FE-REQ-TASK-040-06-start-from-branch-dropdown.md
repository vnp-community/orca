# FE-REQ-TASK-040-06: Dropdown "Start from" hiện tên nhánh mặc định và mở danh sách nhánh

**From Solution:** [FE-REQ-SOL-040](../solutions/FE-REQ-SOL-040-request-base-branch-picker.md)
**Priority:** P1
**Area:** frontend / request
**File:** `components/request/{StartFromField.tsx,BaseRefPickerPopover.tsx,start-from-state.ts}`, `components/git-ref-picker/{RepoRefPicker.tsx,use-repo-branch-list.ts (mới)}`, `runtime/runtime-repo-client.ts` + `web/web-preload-api.ts` + `preload/api-types.ts` (`listRepoBranches`), locale 5 ngôn ngữ, test, spec UI `request-base-branch.web.e2e.ts`
**Depends on:** FE-REQ-TASK-040-03, 040-04, TASK-REQ-039-07
**Status:** [x] DONE (2026-10-20) — spec UI `request-base-branch` 3/3 xanh trên stack; bản dịch es/ja/ko/zh cần người bản ngữ rà

---

## Context

Yêu cầu duyệt 2026-10-20 (mockup đã duyệt): khi người dùng không chọn, dropdown hiển thị **tên chính xác của nhánh mặc định**; bấm vào dropdown hiện **danh sách nhánh của repo** để chọn. Bản 040-03 hiển thị nhãn chung "Default branch (main)" và chỉ liệt kê khi gõ từ 2 ký tự.

## Việc cần làm

1. Client `listRepoBranches(repoId)` gọi `repo.listBranches` (hợp đồng 2.12); gateway không có kênh thì trả "không hỗ trợ" và picker rơi về tìm kiếm như cũ (không báo lỗi).
2. Hook `use-repo-branch-list`: tải khi mở popover (một lần mỗi lần mở, có `retry`), gắn nhãn theo repo và runtime owner như `useRepoRefSearch`, không bao giờ hiện danh sách của repo trước.
3. Nút của dropdown: chưa chọn thì hiện **tên nhánh mặc định đúng như `repo.baseRefDefault` trả** (ví dụ `origin/main`) kèm nhãn nhỏ "default"; đã chọn thì hiện tên nhánh đã chọn. Chưa phân giải được thì "Default branch" như cũ. Phông mono, cắt giữa khi dài (`MiddleTruncatedRef`).
4. Popover: ô tìm kiếm trên cùng; khi ô trống liệt kê: nhánh mặc định (ghim đầu, nhãn "default"), rồi mục "Local branches", rồi "Remote branches" (nhánh remote-only, nhãn "remote only"). Gõ thì lọc danh sách đã tải ngay trong máy; từ 2 ký tự mà danh sách bị `truncated` thì gọi thêm `repo.searchRefs`. Mục đang dùng có dấu tick; bàn phím mũi tên/Enter/Esc như `RepoRefPicker`.
5. Chọn nhánh mặc định = "không chọn" (không gửi `baseRef`, như 040-03). Chọn nhánh khác: dòng gợi ý ghi nhánh đó và có liên kết "Use default (`<tên>`)" để quay về.
6. Giữ "Type a branch name instead" cho tag/commit và khi host offline. Trạng thái tải, rỗng ("No branches found") và lỗi (nút Retry) có mặt.
7. Dòng "Start from" ở tab Overview và thẻ lỗi thực thi dùng cùng popover nên nhận cùng hành vi.
8. Khoá i18n mới ở 5 locale (`defaultTag`, `remoteOnly`, `localHeading`, `remoteHeading`, `useDefaultNamed`, `noBranches`, `loadFailed`).

## Kiểm thử

- Hook: tải khi mở, lỗi + retry, kênh không có thì rơi về tìm kiếm, đổi repo không rò danh sách.
- `StartFromField`: chưa chọn hiện đúng tên mặc định kèm nhãn; chọn nhánh khác hiện tên đó; chọn lại nhánh mặc định không gửi `baseRef`; không phân giải được thì "Default branch".
- Popover: thứ tự mặc định/local/remote, lọc tại chỗ, `truncated` gọi tìm kiếm, bàn phím.
- Spec UI `request-base-branch.web.e2e.ts` (stub): mở dropdown thấy `main`, `feature/other`, `release/1.x` (repo LedgerLite), nút ban đầu ghi đúng tên mặc định, chọn `release/1.x`, quay về mặc định.

## Tiêu chí hoàn thành

- [x] Nút hiện tên nhánh mặc định thật khi chưa chọn.
- [x] Bấm mở danh sách nhánh local và remote-only của repo, lọc được (kiểm thử với `listRuntimeRepoBranches` giả theo hợp đồng 2.12).
- [x] Không hồi quy 040-03/04; gateway cũ vẫn dùng được (rơi về tìm kiếm, không báo lỗi).
- [ ] Gateway thật phục vụ `repo.listBranches` (phụ thuộc TASK-REQ-039-07; chưa kiểm trên stack).
- [ ] Spec UI xanh trên repo fixture thật (đã sửa spec, **chưa chạy**).

## Ghi chú triển khai

**Tệp mới** (`frontend/src/renderer/src/`):
- `components/git-ref-picker/use-repo-branch-list.ts` — hook tải danh sách khi gắn kết (popover gắn nội dung khi mở nên mỗi lần mở tải một lần), khoá theo runtime owner + repo + lần thử (`retry`), trả `loading | ready | unsupported | error{offline}`.
- `components/git-ref-picker/branch-list-sections.ts` — hàm thuần dựng các nhóm: mặc định ghim đầu, Local, Remote-only, "More matches" (kết quả `repo.searchRefs` khi `truncated`); lọc cục bộ không phân biệt hoa thường; nhánh mặc định chỉ hiện một lần (ở dòng ghim).
- `components/git-ref-picker/RepoBranchListPicker.tsx` + `repo-branch-list-parts.tsx` — giao diện danh sách (ô tìm, listbox có nhóm, tick mục đang dùng, thẻ "default"/"remote only", trạng thái tải/rỗng/lỗi + Retry, phím mũi tên/Enter/Esc như `RepoRefPicker`).
- `components/git-ref-picker/RepoBranchPicker.tsx` — chọn giữa danh sách và `RepoRefPicker` cũ (khi kênh không có).
- Test: `use-repo-branch-list.test.tsx`, `branch-list-sections.test.ts`, `RepoBranchPicker.test.tsx`, `components/request/StartFromField.branch-list.test.tsx`, `runtime/runtime-repo-client-list-branches.test.ts`.

**Tệp sửa:**
- `shared/types.ts` (`RepoBranchEntry`, `RepoBranchListResult`), `preload/api-types.ts` (`repos.listBranches?` tuỳ chọn), `web/web-preload-api.ts` (`listBranches` gọi `repo.listBranches`), `runtime/runtime-repo-client.ts` (`listRuntimeRepoBranches` trả `{status:'ok',...}` hoặc `{status:'unsupported'}`; đường Electron không có IPC tương đương nên trả "unsupported", không tạo IPC mới).
- `git-ref-picker/use-repo-ref-search.ts`: thêm tuỳ chọn `{searchEnabled, lookupDefault}` (mặc định như cũ nên Settings và các màn khác không đổi); `RepoRefPicker.tsx` không bị sửa.
- `request/BaseRefPickerPopover.tsx` (dùng `RepoBranchPicker`, prop mới `defaultName`), `request/BaseRefChangeControl.tsx` (truyền `defaultName`), `request/StartFromField.tsx` (nút hiện tên mặc định mono + thẻ "default"; đã chọn thì hiện tên nhánh; nút "Use default (<tên>)"; dòng gợi ý nêu nhánh đã chọn).
- 5 file locale: `StartFromField` thêm `defaultTag`, `useDefaultNamed`, `hintPicked`; nhóm mới `gitRefPicker.RepoBranchPicker` (`localHeading`, `remoteHeading`, `moreHeading`, `defaultTag`, `remoteOnly`, `optionDefault`, `optionRemote`, `noBranches`, `loading`, `loadFailed`, `listLabel`). Bản dịch es/ja/ko/zh cần người bản ngữ rà như các CR trước.
- Test cũ: thêm `listRuntimeRepoBranches` (trả `unsupported`) vào mock của `CreateRequestDialog.start-from`, `CreateRequestIssueButton.start-from`, `RequestBaseRefRow`, `RequestBaseRefRow.on-changed`, `ExecutionFailureCard.base-ref`; đổi nút "Use default branch" thành "Use default (origin/main)"; thêm 2 test danh sách vào `RequestBaseRefRow.test.tsx`; thêm tệp mới vào `repo-ref-picker-locale-coverage.test.ts`.
- Spec UI (`tests/stages/e2e/orca-flows/ui-live/`): `request-base-branch.web.e2e.ts` + `live-base-branch-steps.ts` — kiểm nút ban đầu ghi `main` + "default" (không phải "Default branch"), mở dropdown không gõ, thấy `main (default)`, `feature/wip` (nhánh thứ ba của spec, fixture Python dùng `feature/other`), `release/1.x`; chọn `release/1.x`, đặt lại về mặc định, chọn lại rồi tạo. Hộp thoại được chọn theo tên (`Create request`), danh sách theo `getByRole('listbox', {name:'Branches'})`. **Chưa chạy.**

**Lệnh và kết quả:**
- `cd frontend && npx vitest run --config config/vitest.config.ts src/renderer/src/components/request src/renderer/src/components/git-ref-picker src/renderer/src/runtime` → 160 file xanh; 1 file đỏ có sẵn, không liên quan (`runtime/runtime-cli-client.test.ts`, 3 test: `window.api.cli`). Cộng thêm `settings/BaseRefPicker.test.tsx`, `automations/CreateFromPicker.test.tsx` xanh.
- `npx oxlint` (từ thư mục gốc, trên các thư mục/tệp đã đổi, cả cấu hình react-doctor) → sạch. `npx oxfmt --write` từng tệp đã đổi.
- `cd frontend && npx tsc --noEmit -p tsconfig.json --composite false --incremental false` → 113 lỗi, bằng mốc; không lỗi mới (các lỗi sẵn có ở `web-preload-api.ts` dòng 741/1531/2587 không do thay đổi này).

**Chênh lệch so với mô tả:**
- Khi đang tải, popover chỉ hiện dòng "Loading branches..." (chưa có ô tìm) để nếu gateway không có kênh thì chuyển sang picker tìm kiếm mà không mất chữ người dùng đã gõ; sau đó ô tìm được focus tự động.
- Nhận biết "kênh không có": mã `method_not_found`/`-32601`, hoặc thông điệp "is not yet implemented"/"unknown method|channel" (gateway Go trả mã `internal` kèm câu này cho kênh chưa đăng ký; đường web preload mất mã nên dựa vào thông điệp). `WORKTREE_REPO_NOT_FOUND` và host không tới được vẫn là lỗi thật.
- Danh sách tự viết (cùng kiểu `RepoRefPicker`) thay vì `Command` của STYLEGUIDE, để giữ đúng hành vi Esc/Enter/mũi tên và `aria-activedescendant` của picker hiện có.

## Kết quả chạy trên stack (2026-10-20, orchestrator)

- `run_ui_flow.py --ai-stub -- request-base-branch`: 3/3 đạt; `run_acceptance.py --ai-stub devserver request_base_ref`: BRF-01..08 và BRF-10 xanh; hồi quy `platform devserver request matrix task aistub`: 72 đạt, 0 lỗi.
- Chạy thật phát hiện hai lỗi và đã sửa: (1) `repo.baseRefDefault` trả `{ref}` nhưng màn hình đọc `defaultBaseRef`, nên nút "Start from" chưa bao giờ biết tên nhánh mặc định ở chế độ web (sửa ở `web/base-ref-default-wire.ts`, có test); (2) gateway bỏ khoá `remote` khi `false`, bộ dựng danh sách đòi `remote === false` nên mọi nhánh local biến mất (nay vắng mặt = local, có test).
- Tiêu chí đã tick: nút hiện tên nhánh mặc định thật, bấm mở danh sách local/remote-only có lọc, gateway cũ rơi về tìm kiếm (test đơn vị), spec UI xanh.
