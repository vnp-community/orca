# FE-REQ-TASK-040-05: i18n 5 locale và e2e giao diện trên repo fixture tài chính

**From Solution:** [FE-REQ-SOL-040](../solutions/FE-REQ-SOL-040-request-base-branch-picker.md)
**Priority:** P1
**Area:** frontend / request
**File:** `frontend/src/renderer/src/i18n/locales/{en,es,ja,ko,zh}.json`, `tests/stages/e2e/orca-flows/ui-live/request-base-branch.web.e2e.ts` (mới) + `live-base-branch-steps.ts`, `run_ui_flow.py` (đăng ký spec)
**Depends on:** FE-REQ-TASK-040-03, 040-04, backend TASK-REQ-039-06
**Status:** [ ] PARTIAL (2026-10-20): spec live `request-base-branch` 3/3 xanh trên stack (stub); còn lại duy nhất tiêu chí hai lệnh verify i18n, đỏ sẵn trên HEAD (không do CR này)

---

## Context

- Lint chạy `verify:localization-catalog` và `verify:localization-coverage`.
- Spec UI dùng `runLiveFlow`, `configureProject`, fixture tạm; guard quiet-checkout khi chạy `--real`.
- Fixture `ledgerlite-finance` được sao vào repo git tạm bởi helper của backend task 06.

## Việc cần làm

1. Thêm mọi khoá mới vào 5 locale (en là nguồn; es/ja/ko/zh dịch có kiểm tra; ghi vào PR rằng cần người bản ngữ rà); chạy hai lệnh verify.
2. Spec `request-base-branch.web.e2e.ts` (`ORCA_E2E_LIVE=1`): (a) mở Create request, thấy chip "Default branch"; (b) chọn nhánh `release/1.x` của fixture, tạo, Overview hiện "Worktrees start from release/1.x"; (c) tạo không chọn: Overview "Default branch"; (d) sau khi chạy Plan có worktree (claude giả), dòng khoá với tooltip đúng; (e) nhập nhánh không tồn tại: lỗi dưới ô; (f) sửa nhánh ở Overview khi chưa khoá thành công; (g) cờ tính năng tắt (giả lập) thì ô ẩn.
3. Đăng ký trong `run_ui_flow.py` (tên chế độ `--ai-stub -- request-base-branch`).
4. Chụp ảnh các trạng thái chính vào thư mục ảnh của harness.

## Kiểm thử

- `pnpm run verify:localization-catalog && pnpm run verify:localization-coverage` xanh.
- `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- request-base-branch` xanh trên stack local.
- Chạy lại `qa-bug-task-native-flow` và `qa-regressions` (terminal cần agent thật) để chắc không hồi quy.

## Tiêu chí hoàn thành

- [ ] Đủ khoá ở 5 locale; hai lệnh verify xanh.
- [x] Spec UI xanh trên repo fixture thật: `request-base-branch` 3/3 (chọn → sửa → khóa; nhánh không tồn tại bị từ chối dưới trường và hộp thoại giữ nguyên; gateway không báo `request.baseRef` thì ẩn trường và dòng). Chạy thật phát hiện 2 lỗi trong spec (bộ chọn `dialog` bị mơ hồ vì popover đóng vẫn nằm trong DOM) và cần build lại container frontend.
- [x] Không hồi quy các spec QA hiện có: `qa-bug-task-native-flow` 3/3; `qa-regressions` QA-UI-TERM-01 đạt (không stub; dưới stub không tạo được PTY), QA-UI-GH-01 đỏ vì nguồn GitHub đang hoãn theo yêu cầu.

## Rủi ro và lưu ý

- `--ai-stub` không spawn được PTY: spec này không dùng terminal nên chạy được với stub.
- Bản dịch es/ja/ko/zh cần rà bởi người bản ngữ.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.

## Ghi chú triển khai (2026-10-20)

**File đổi/tạo**
- Spec (mới, `tests/stages/e2e/orca-flows/ui-live/`): `request-base-branch.web.e2e.ts` (3 test), `live-base-branch-steps.ts` (hộp thoại Create, dòng Overview, khoá), `live-base-branch-gateway-features.ts` (+ `.test.ts`, 5 ca: proxy WebSocket bỏ `request.baseRef` khỏi `features` của `request.flowStatus`), `live-ledgerlite-project-flow.ts` (`runLedgerliteFlow`: project trên repo LedgerLite `createLedgerliteRepo` có `release/1.x` và nhánh thứ ba `feature/wip`, repo xoá sau khi project đã xoá). `run_ui_flow.py`: chỉ thêm hai dòng vào docstring (tên spec là bộ lọc của Playwright nên `-- request-base-branch` chạy được ngay, không cần danh sách riêng).
- Frontend (theo việc tồn đọng của 040-04): `RequestDetailPane.tsx` truyền `onChanged={refetch}` cho Overview và gọi `useRequestRefreshOnReturn(refetch)`; `ExecutionTab.tsx` truyền `request` và `onChanged` cho `ExecutionFailureCard`; **ngoài danh sách file được giao nhưng bắt buộc** để `onChanged` có chỗ đi: `RequestOverviewTab.tsx` (prop `onChanged?`), `RequestBaseRefRow.tsx` (prop `onChanged?`), `use-request-base-ref-save.ts` (tham số thứ hai `onChanged?`, gọi sau lưu thành công, sau từ chối khoá đã tải lại, sau xung đột đã tải lại; không gọi khi bị từ chối mà không đổi gì), `execution/ExecutionFailureCard.tsx` (prop `onChanged?`). Hook mới `hooks/useRequestRefreshOnReturn.ts` (làm mới khi cửa sổ lấy lại focus / tab hiện lại, tối thiểu 5 s giữa hai lần).
- Test mới: `useRequestRefreshOnReturn.test.ts` (4), `RequestDetailPane.refresh.test.tsx` (2), `RequestBaseRefRow.on-changed.test.tsx` (4), `execution/ExecutionTab.failure-card.test.tsx` (1).
- i18n: không thêm khoá nào mới cho task này (khoá của hộp thoại/dòng/thẻ lỗi đã ở 040-03). Thêm 1 khoá ở `activity` (xem 041-04: `cause.baseRefNotFound`) cho nhãn nguyên nhân `base_ref_not_found` trong History.

**Sự kiện thời gian thực (việc tồn đọng của 040-04)**: đọc `backend-go/services/api-gateway/internal/adapter/wscompat/channels_request_stream.go`: `requestEventRegistry` có 14 hàng và **không có** `orca.request.request.base_ref_changed` (subject có trong `request-service/internal/domain/outbox_subjects.go`). Sự kiện đó **không được chuyển tới `request.subscribe`**, nên UI không thể làm mới dòng khi người khác đổi nhánh. Đã làm phần bù mà frontend làm được: làm mới khi cửa sổ lấy lại focus (`useRequestRefreshOnReturn`), cộng `onChanged` sau mỗi lần chính người dùng lưu. Tiêu chí "Cập nhật thời gian thực" của 040-04 **để nguyên chưa tick**. Việc cho gateway: thêm một hàng `{Subject: "orca.request.request.base_ref_changed", EventType: "request.base_ref_changed"}` vào registry (và cập nhật CONTRACT mục 3 + test `channels_request_stream_test.go`); khi đó chỉ cần `RequestDetailPane` đã refetch theo mọi sự kiện của Request (`useRequestSubscription`), không cần sửa gì thêm ở frontend ngoài thêm vào bản đồ kiểu `RequestEvent` nếu có.

**Spec live (chưa chạy)**
1. `a chosen branch is stored and shown, can be changed until the Request is finished, then it is locked`: Create dialog có chip "Default branch"; chọn `release/1.x` (popover tìm 3 ký tự), có nút "Use default branch"; tạo; `request.get.baseRef = release/1.x`, chưa khoá; Overview "Worktrees start from release/1.x"; chờ `awaiting_type_confirmation` và màn hình theo kịp; sửa bằng bút sang `feature/wip` (đối chiếu API, tải lại trang vẫn còn), sửa về "Default branch" (ref rỗng); huỷ Request bằng giao diện; dòng có "Locked" + tooltip "finished, so the branch can no longer be changed", không còn bút, `baseRefLockReason=terminal`, và `request.setBaseRef` trả `REQUEST_BASE_REF_LOCKED`; checkout chính vẫn ở `feature/wip`.
2. `without a choice the default branch applies; a branch that does not exist...`: gõ tay `release/9.99`, Create: lỗi "not found in the repository" dưới ô, hộp thoại còn mở, tiêu đề và nhánh gõ còn nguyên, không có Request nào được tạo; "Use default branch" rồi Create: `baseRef=''`, Overview "Worktrees start from the default branch".
3. `a gateway that does not advertise the feature hides the field and the row`: đối chứng ở cửa sổ chính (ô và dòng có); cửa sổ thứ hai (cùng phiên đăng nhập) qua `page.routeWebSocket` (cùng kỹ thuật `tests/e2e/workflow-web/support/workflow-socket-fault-injection.ts`) bỏ `request.baseRef` khỏi `features`: kiểm proxy đã viết lại ≥ 1 câu trả lời, hộp thoại (sau khi `create-request-repo-note` hiện, tức repo đã biết) không có `start-from-field`, Overview không có `request-base-ref-row`.
- Ảnh chụp: `create-dialog-default-chip`, `create-dialog-branch-chosen`, `overview-branch-row`, `overview-branch-changed`, `overview-branch-locked`, `create-dialog-branch-not-found`, `overview-default-branch`, `flag-off-create-dialog` (thư mục output của Playwright).
- **Khác với task text**: (d) "sau khi có Plan và worktree thì khoá": bản `--ai-stub` không bao giờ duyệt Plan và engine native không tạo worktree trước khi task chạy, nên không thể có khoá `proposal`/`task`/`execution` mà không có agent chạy thật; spec kiểm khoá `terminal` (Request đã kết thúc), còn ba khoá kia do `RequestBaseRefRow.test.tsx` phủ (5 lý do). Spec chỉ chạy với stub (`aiMode=stub`), vì loại của Request do classifier giả định (title/body chọn `change_request`, loại có worktree).

**Lệnh chạy (orchestrator, trên stack local)**
- `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- request-base-branch`
- đã chạy được ở máy này: `ORCA_E2E_LIVE=1 npx playwright test -c tests/configs/playwright.web.config.ts --project=request-live --list` liệt kê đủ 3 test (tổng 34 test / 18 file); `npx vitest run tests/stages/e2e/orca-flows/ui-live/live-base-branch-gateway-features.test.ts` xanh (5); kiểm kiểu bằng tsconfig tạm (`tsc -p` với `moduleResolution: bundler`) sạch cho hai spec và mọi file import.

**Rủi ro chưa kiểm vì chưa chạy trên stack**: popover tìm nhánh trong Radix Dialog với chuột thật; tìm nhánh qua gateway cho repo trên dev server (`searchRuntimeRepoBaseRefs` ở chế độ web); hành vi tooltip Radix sau `hover()`; `connectToServer()` của proxy chuyển cookie phiên (precedent `workflow-web` cho thấy chạy được với cùng gateway); văn bản lỗi NOT_FOUND ở đường `request.create` khớp `/not found in the repository/`.

**Kiểm chứng đã chạy (frontend, từ `frontend/`)**
- `npx vitest run --config config/vitest.config.ts src/renderer/src/components/request src/shared src/renderer/src/hooks src/renderer/src/components/git-ref-picker src/renderer/src/runtime/request-rate-limit.test.ts src/renderer/src/store/slices/request.test.ts`: mọi test của vùng đã đổi xanh; 17 test đỏ ở 5 file **không liên quan, có sẵn** (`useLogout`, `useClarifications`, `useIpcEvents` thiếu `window.api`/`localStorage`; `shared/feature-interactions`, `shared/remote-runtime-shared-control-boundary` thiếu `src/main`).
- `npx tsc --noEmit -p tsconfig.json --composite false --incremental false`: **113 lỗi, bằng mốc**, không lỗi nào ở file đã đổi.
- `npx oxlint -c .oxlintrc.json frontend/src/renderer/src/components/request frontend/src/renderer/src/hooks/useRequestRefreshOnReturn*.ts`: sạch; `oxlint -c frontend/config/oxlint-react-doctor.json` không có cảnh báo ở file đã đổi.
- i18n: chạy `desktop/config/scripts/verify-localization-catalog.mjs` và `audit-localization-coverage.mjs --check` trên bản sao `src/renderer/src` của HEAD (`git archive`) và của cây làm việc (scratchpad `i18nverify040-05/{head,now}` + `config/localization-coverage-allowlist.json`): cả hai **đỏ sẵn trên HEAD** (284 tham chiếu khoá thiếu, 1227 chuỗi cứng, ở `web/`, `DepartmentGate`...), đầu ra **giống hệt** nhau (`diff` rỗng), không dòng nào thuộc file đã đổi. Thêm kiểm ngang hàng: hai khoá mới có ở es/ja/ko/zh, cùng placeholder.
- Blast radius (grep): `useRequestBaseRefSave` được gọi ở `RequestBaseRefRow` và `ExecutionFailureCard` (tham số thêm tuỳ chọn); `RequestOverviewTab` chỉ ở `RequestDetailPane`; `ExecutionFailureCard` chỉ ở `ExecutionTab`.

**Chưa làm**: chạy spec live, "không hồi quy `qa-bug-task-native-flow` / `qa-regressions`" (không đụng stack); `shared/request-errors.ts` không ánh xạ `REQUEST_BASE_REF_*` (UI dựa vào mã backend, không cần); `README.md` của `orca-flows` chưa liệt kê hai spec (không thuộc file được giao).
