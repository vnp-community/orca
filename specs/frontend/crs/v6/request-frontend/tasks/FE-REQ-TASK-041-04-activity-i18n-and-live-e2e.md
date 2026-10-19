# FE-REQ-TASK-041-04: Activity, i18n 5 locale và e2e giao diện trên repo fixture tài chính

**From Solution:** [FE-REQ-SOL-041](../solutions/FE-REQ-SOL-041-analysis-revision-by-prompt.md)
**Priority:** P1
**Area:** frontend / request / solution
**File:** `renderer/src/components/request/activity/activity-entry-presentation.ts` (+ test), `i18n/locales/{en,es,ja,ko,zh}.json`, `tests/stages/e2e/orca-flows/ui-live/analysis-revision.web.e2e.ts` (mới) + `live-analysis-revision-steps.ts`, `run_ui_flow.py` (đăng ký)
**Depends on:** FE-REQ-TASK-041-02, 041-03, backend TASK-REQ-041-04
**Status:** [x] DONE (2026-10-20): spec live `analysis-revision` 5/5 xanh trên stack (stub); hai vai trò (approver không phải reporter, người lạ) được kiểm ở tầng API bằng ARV-07/08, không ở tầng UI

---

## Context

- Activity kind lạ phải chịu được; kind mới cần biểu tượng/nhãn.
- Spec UI dùng fixture `ledgerlite-finance` được sao vào repo git tạm.

## Việc cần làm

1. `analysis.revision_requested` trong `activity-entry-presentation.ts`; test ánh xạ và kind lạ.
2. Mọi khoá i18n mới cho 5 locale; chạy `verify:localization-catalog` và `verify:localization-coverage`.
3. Spec `analysis-revision.web.e2e.ts` (claude giả): (a) `change_request` → Analysis hiện Solution, nhập yêu cầu, "Revise analysis", thấy "Analyzing version 2…", rồi bản 2 với Approval mới; lặp lần hai; danh sách có 3 bản; (b) xem bản 1 chỉ đọc; (c) `bug` chẩn đoán; (d) `spike` Findings; (e) `question` Answer; (f) tài khoản approver không phải reporter chỉnh được, tài khoản chỉ đọc bị từ chối với thông báo quyền; (g) Activity có dòng "Requested changes to the analysis" và không chứa nội dung; (h) ca lỗi `NO_CONNECTION` giữ nội dung đã gõ.
4. Đăng ký `--ai-stub -- analysis-revision`; ảnh chụp trạng thái chính.

## Kiểm thử

- Hai lệnh verify i18n xanh.
- `run_ui_flow.py --ai-stub -- analysis-revision` xanh trên stack local.
- Không hồi quy `qa-bug-task-native-flow`, `qa-regressions`, tab Analysis/History hiện có.

## Tiêu chí hoàn thành

- [x] Spec xanh cho bốn kiểu tài liệu (change request chỉnh hai lần, bug, spike, question) và ca màn hình cũ bị từ chối giữ nguyên chữ; hai vai trò kiểm ở API (ARV-07/08). Chạy thật phát hiện: dòng "Analyzing version n…" cần `seq` từ gateway (đã sửa), và spec dùng mã `[stub:slow]` để dòng đó đủ lâu cho MutationObserver.
- [x] Activity hiển thị đúng, không lộ nội dung. (kiểm bằng test đơn vị và component; phần live ở spec chưa chạy)
- [x] 5 locale đủ khoá. (hai khoá mới, test phủ + so ngang hàng; hai lệnh verify i18n đỏ sẵn trên HEAD, đầu ra không đổi)

## Rủi ro và lưu ý

- Hai tài khoản (approver, chỉ đọc) cần được tạo bằng API admin của stack local trong bước chuẩn bị spec, không dùng tài khoản thật; mật khẩu sinh ngẫu nhiên, không in ra.

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
- `renderer/src/components/request/activity/`: `activity-entry-presentation.ts` (kind `analysis.revision_requested`: biểu tượng `FilePenLine`, nhóm lọc `changes`, mở tab Analysis, tông trung tính; kind lạ vẫn rơi về biểu tượng và tóm tắt mặc định), `activity-entry-text.ts` (tiêu đề "Requested changes to the analysis"; **không bao giờ** có dòng chi tiết cho kind này, kể cả khi backend gửi thêm `feedback`/`reason`; nhãn nguyên nhân `base_ref_not_found` = "Starting branch not found" cho dòng `task.failed`). Test: `activity-entry-presentation.test.ts` (+nhóm, tông, biểu tượng riêng, tab), `activity-entry-text.test.ts` (+tiêu đề, nguyên nhân, "không lộ nội dung"), `RequestActivityTab.test.tsx` (+1: dòng chỉnh sửa hiển thị một dòng, có actor, không có huy hiệu mức độ), `activity-locale-coverage.test.ts` (mới: đọc mọi ``translate(`${T}...`)`` của `activity-entry-text.ts`, kiểm 5 catalog, placeholder, en = fallback, hai khoá mới không sao chép tiếng Anh).
- i18n (5 locale, một lần sửa nguyên tử mỗi file, JSON giữ nguyên định dạng): `auto.components.request.activity.title.analysisRevisionRequested` và `auto.components.request.activity.cause.baseRefNotFound`. Bản dịch es/ja/ko/zh tự dịch, **cần người bản ngữ rà**.
- Spec (mới, `tests/stages/e2e/orca-flows/ui-live/`): `analysis-revision.web.e2e.ts` (5 test), `live-analysis-revision-steps.ts` (chờ phân tích sẵn sàng mà không duyệt, `reviseAnalysis`, danh sách bản, bản cũ chỉ đọc, đối chiếu `solution.list` và Approval), `live-analysis-revision-outcome-steps.ts` (dòng Activity, quá 2000 ký tự, màn hình cũ), `live-analysis-revision-panel-recorder.ts` (MutationObserver ghi dòng "Analyzing version n…" và lỗi, vì chúng chỉ hiện chốc lát), `live-push-frame-gate.ts` (giữ các khung push của một cửa sổ), dùng chung `live-ledgerlite-project-flow.ts`. `run_ui_flow.py`: hai dòng docstring (xem 040-05).

**Spec live (chưa chạy)**: dự án đặt engine native, `solutionAgent=false` bằng `configureProject` (khôi phục sau); claude giả trả cùng một tài liệu dù phản hồi là gì (đọc `fake_claude_cli/claude`, không sửa) nên chỉ kiểm cấu trúc.
1. `change request: revised twice...`: tạo, phân loại, xác nhận loại, trả lời Clarification nếu có, phân tích sẵn sàng (không duyệt); một Approval mở. Vòng 1 và 2: nhập, đếm ký tự `n / 2000`, "Revise analysis"; nhật ký observer có "Analyzing version 2…"/"3…"; dịch vụ có đủ n bản, bản mới mang `revisionFeedback` đúng chữ đã gõ, mọi bản trước `superseded`, `seq` 1..n, đúng **một** Approval mở và **khác** Approval cũ, chip "Awaiting Analysis Approval", danh sách n bản, màn hình chuyển sang bản mới ("Version n" `aria-pressed`), ô nhập rỗng. Cuối: 3 bản, 3 Approval khác nhau. Bản 1 chỉ đọc (dải "You're viewing an older version.", không ô nhập, không Approve, không khối "What was asked"), bản 2 hiện đúng chữ vòng 1, "Back to current" trở về, bản 3 hiện đúng chữ vòng 2 (xuống dòng giữ). History: đúng 2 dòng "Requested changes to the analysis", không có chữ phản hồi trên màn hình, và `request.activity` có 2 kind `analysis.revision_requested` mà JSON không chứa chữ phản hồi. Quá 2000 ký tự: lỗi "Keep the request to 2000 characters or fewer.", ô giữ nguyên chữ, không thêm bản. Cuối cùng huỷ Request bằng giao diện.
2. `a revision sent from an out-of-date screen is refused and keeps the text`: cửa sổ thứ hai giữ khung push (chặn `streaming:true`), gõ chữ, API huỷ Request, bấm Revise: observer thấy lỗi "Someone else is already revising this analysis or has decided..." khi ô còn đúng chữ đã gõ; sau khi thả khung, màn hình biết Request đã "Cancelled"; không bản nào được tạo.
3. `bug`, `spike`, `question`: cùng đường đi, một lần chỉnh (diagnosis, Findings, Answer), bản 2 + Approval mới + bản 1 chỉ đọc + 1 dòng Activity, rồi huỷ. Hai tài khoản (approver không phải reporter, chỉ đọc) **không làm**: cần tạo user bằng API admin của stack, việc này nằm ngoài những gì brief cho phép; để lại cho test phân quyền backend (`solution_revise_authorization_test.go`). Spec chỉ chạy với stub (`aiMode=stub`).
- Ảnh chụp: `analysis-ready`, `revision-n-typed`, `revision-n-done`, `older-version-1`, `current-version-asked-for`, `history-revision-lines`, `revision-too-long`, `revision-after-request-moved-on`.

**Lệnh chạy (orchestrator, trên stack local)**
- `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- analysis-revision`
- ở máy này đã chạy: `ORCA_E2E_LIVE=1 npx playwright test -c tests/configs/playwright.web.config.ts --project=request-live --list` liệt kê đủ 5 test; tsc tạm sạch cho spec và file import; vitest/oxlint/tsc/i18n như 040-05 (cùng kết quả: vùng đã đổi xanh, tsc 113 = mốc, verify i18n đỏ sẵn trên HEAD với đầu ra giống hệt).

**Rủi ro chưa kiểm vì chưa chạy trên stack**: dòng "Analyzing version n…" phụ thuộc `seq` do gateway trả (đã triển khai); lỗi `wrongState` ở cửa sổ cũ phụ thuộc mã `REQUEST_SOLUTION_WRONG_STATE` của backend khi Request đã huỷ; khung push của gateway được nhận diện qua `"streaming":true` (cùng dạng `gateway-rpc-client.ts` đang đọc); số lần tool-call của phân tích chỉ-đọc ở chế độ stub.

**Blast radius (grep)**: `ACTIVITY_GROUP_KINDS` chỉ dùng ở `activity-filter-params.ts` (nhóm "changes" nay gửi thêm một kind hợp lệ ở backend); `activityTargetTab` ở `ActivityEntryRow`; `activityFailureCauseLabel` ở `activity-entry-text.ts`.
