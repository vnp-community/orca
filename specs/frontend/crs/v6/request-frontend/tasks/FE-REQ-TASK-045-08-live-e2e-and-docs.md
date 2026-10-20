# FE-REQ-TASK-045-08: Spec UI live (`plan-documents`, `plan-revision`) trên repo fixture tài chính và tài liệu

**From Solution:** [FE-REQ-SOL-045](../solutions/FE-REQ-SOL-045-plan-documents-and-revision-ui.md)
**Priority:** P1
**Area:** frontend / request / plan
**File:** `tests/stages/e2e/orca-flows/ui-live/plan-documents.web.e2e.ts`, `plan-revision.web.e2e.ts` (mới), `live-plan-documents-steps.ts`, `live-plan-revision-steps.ts`, `live-plan-revision-panel-recorder.ts` (mới; dùng lại `live-analysis-revision-panel-recorder.ts` và `live-push-frame-gate.ts` nếu phù hợp), `tests/stages/e2e/orca-flows/run_ui_flow.py` (đăng ký), `tests/stages/e2e/orca-flows/README.md` (dòng mới), dùng lại `tests/stages/e2e/orca-flows/plan_documents_seed.py` (BE-REQ-TASK-045-09)
**Depends on:** FE-REQ-TASK-045-03 đến 045-07, backend BE-REQ-TASK-045-09; FE-REQ-TASK-044-04 (mẫu spec `auto-plan-generation`, dùng lại bước chờ Plan)
**Status:** [ ] PARTIAL (2026-10-20): spec đã viết, kiểm kiểu và liệt kê được; chưa chạy trên stack (not run on the stack) theo yêu cầu của người điều phối; phụ thuộc backend CR-045 đã triển khai và `plan_documents_seed.py` (BE-045-09) cho ca OpenSpec.

---

## Context

- Spec UI mẫu: `analysis-revision.web.e2e.ts` (TASK-041-04): dùng `configureProject` (engine `native`, `solutionAgent=false`, khôi phục sau), claude giả trả cùng tài liệu bất kể phản hồi (chỉ kiểm cấu trúc, echo "revised, round N"), `MutationObserver` ghi dòng chạy chốc lát, hai tài khoản kiểm ở tầng API (ARV-07/08).
- Fake claude **không tác giả OpenSpec**: ca OpenSpec của UI dùng dữ liệu seed (`plan_documents_seed.py`) hoặc, nếu BE-045-09 việc 4 làm được, claude giả OpenSpec; ca OpenSpec thật là loại C (7 đến 13 USD mỗi lượt sinh, cần đồng ý, `ORCA_E2E_OPENSPEC=1`).
- Quy tắc `run_ui_flow.py --real`: chờ checkout orca yên 90 giây rồi coi mọi thay đổi là rò rỉ; không sửa/build trong repo khi chạy. Ghi chú stack local: Browser pane phải mở bằng `preview_start` có url trước khi chụp màn hình.
- Fixture `ledgerlite-finance`: kịch bản 1 `bug` (S, native) cho ca Proposal/chỉnh; kịch bản 3 `change_request` (OpenSpec, loại C).

## Việc cần làm

1. Spec `plan-documents.web.e2e.ts` (claude giả): (a) **native** `bug`: tạo, phân loại, phân tích, duyệt phân tích, Plan sinh xong; tab Plan có mục "Proposal"; mở: `summary`, `notes`, Phase/task, phụ thuộc; không có mục "Documents"; (b) **OpenSpec bằng seed**: Request có dòng `openspec_changes`: mục "Documents" hiện bốn nhóm tab theo thứ tự, mở từng tab (chỉ tải khi mở: kiểm số lần gọi `request.planBundleFile` qua observer mạng/RPC log), Rendered hiện checkbox của tasks, Source hiện `orca:begin plan`, file bị lược có placeholder với kích thước, bí mật cắm sẵn đã bị che trong thân; dải kiểm tra có `STRUCTURE_OK`; (c) hotfix không có mục nào (nếu dựng được; không thì ghi `skipped` lý do).
2. Spec `plan-revision.web.e2e.ts` (claude giả, native `bug`): (a) Plan sẵn sàng, một Approval `pending`; nhập yêu cầu, thấy bộ đếm `n / 2000`, bấm "Revise plan", observer thấy "Writing version 2…", Request qua `planning` rồi về `awaiting_plan_approval`, Approval mới khác Approval cũ (đối chiếu qua API), phiên bản 2; lặp lần hai: 3 phiên bản, 3 Approval khác nhau; (b) bản 1 chỉ đọc: banner "You're viewing an older version.", không panel chỉnh; bản 2 hiện đúng chữ vòng 1 trong "What was asked for this version"; "Back to current"; (c) History/Activity: đúng 2 dòng "Requested changes to the plan", không có chữ phản hồi trên màn hình, `request.activity` có 2 kind `plan.revision_requested` mà JSON không chứa chữ phản hồi; (d) quá 2000 ký tự: lỗi "Keep the request to 2000 characters or fewer.", ô giữ chữ, không thêm bản; (e) màn hình cũ: cửa sổ thứ hai giữ khung push, API huỷ Request, bấm Revise: lỗi "Someone else is already revising…" khi ô còn chữ; (f) lượt thất bại (`[stub:fail-once]` trong phản hồi): thẻ lỗi có "Try again", bấm: chạy lại với cùng phản hồi và thành công, Approval mới; (g) `NO_CONNECTION` (nếu có công cụ ngắt dev server) giữ chữ và Approval; (h) cuối cùng huỷ Request bằng giao diện.
3. Ca người dùng: approver không phải reporter và người chỉ đọc kiểm ở tầng API bằng PRV-05 (BE-045-09); UI chỉ kiểm thông báo quyền khi tạo được hai tài khoản bằng API admin của stack; nếu không thì ghi vào Ghi chú triển khai là không làm (mật khẩu sinh ngẫu nhiên, không in ra).
4. Ca OpenSpec chạy thật (loại C) viết sẵn nhưng `skip` trừ khi `ORCA_E2E_OPENSPEC=1`: kịch bản 3 trên `release/1.x`: Documents đủ nhóm, chỉnh một lần, hai bản, bản 2 `Changed` ở file cần đổi, không bí mật. Ghi lệnh chạy và ước lượng chi phí vào README; **không tự chạy**.
5. Ca loại B (`--real`, native, repo tạm, khoảng 3 phút, chi phí nhỏ): `run_ui_flow.py --real plan-revision` một vòng chỉnh `bug`; chỉ chạy khi người dùng yêu cầu; không sửa checkout trong lúc chạy.
6. Đăng ký `plan-documents` và `plan-revision` trong `run_ui_flow.py` (docstring + bảng ca) và cho chạy với `--ai-stub`; ảnh chụp trạng thái chính (`plan-documents-openspec`, `plan-proposal-native`, `revision-n-typed`, `revision-n-done`, `older-version-1`, `revision-failed`).
7. Tài liệu: dòng trong `README.md` của `orca-flows`; ghi chú triển khai: lệnh cho người điều phối, điều chưa kiểm chứng.

## Kiểm thử

- `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- plan-documents` và `-- plan-revision` xanh trên stack local.
- `ORCA_E2E_LIVE=1 npx playwright test -c tests/configs/playwright.web.config.ts --project=request-live --list` liệt kê đủ test mới.
- Không hồi quy `qa-bug-task-native-flow`, `qa-regressions`, `analysis-revision`, `request-base-branch`.
- tsc tạm sạch cho spec và file import; vitest/lint của vùng đã đổi xanh.

## Tiêu chí hoàn thành

- [ ] Spec xanh cho Proposal native, Documents OpenSpec (seed hoặc claude giả OpenSpec), chỉnh hai lần, bản cũ chỉ đọc, "Try again", Activity. (còn lại: chưa chạy trên stack (not run on the stack))
- [ ] Hai vai trò (approver, chỉ đọc) được kiểm ở API (PRV-05) và, nếu tạo được tài khoản, ở UI. (còn lại: chưa chạy trên stack (not run on the stack))
- [x] Ca loại B và C có lệnh, điều kiện, chi phí ghi rõ; không tự chạy. Bằng chứng: `README.md` của `orca-flows` + docstring `run_ui_flow.py` + đầu hai spec; không chạy ca nào.
- [ ] Không hồi quy các spec UI hiện có. (còn lại: chưa chạy trên stack (not run on the stack))

## Rủi ro và lưu ý

- Dòng "Writing version n…" chỉ hiện chốc lát: dùng `MutationObserver` và mã `[stub:slow]` như TASK-041-04.
- Dữ liệu seed là giả: dọn sau khi chạy; không dùng ở stack dùng chung.
- Gateway sập sau `redeploy-local.sh`: `docker compose up -d --force-recreate api-gateway`, khởi động lại các dịch vụ theo thứ tự đã ghi.

## Ghi chú triển khai (2026-10-20)

- **File mới** (`tests/stages/e2e/orca-flows/ui-live/`): `plan-revision.web.e2e.ts`, `plan-documents.web.e2e.ts`, `live-plan-revision-steps.ts`, `live-plan-revision-outcome-steps.ts`, `live-plan-revision-panel-recorder.ts`, `live-plan-documents-steps.ts`, `live-plan-documents-openspec-real-steps.ts`. **Sửa**: `run_ui_flow.py` (docstring hai ca stub + ca `--real plan-revision`; `REAL_SPECS["plan-revision"]`), `README.md` (khối lệnh CR-REQ-045). Dùng lại `live-auto-plan-steps.ts` (chờ Plan, `reachAnalysisApproval`), `live-push-frame-gate.ts`, `live-flow-steps.ts`, `real-flow-harness.ts`; không sửa các file đó.
- **Đã làm**: `tsc` (tsconfig tạm trong scratchpad, `--strict`) sạch cho hai spec và mọi import; `ORCA_E2E_LIVE=1 npx playwright test -c tests/configs/playwright.web.config.ts --project=request-live --list` liệt kê 8 test mới (tổng 50 test, 22 file); `oxlint` và `oxfmt` sạch; `python3 -m py_compile run_ui_flow.py` ok. **Không chạy** spec, không build container, không đụng stack.
- **Ca stub**: `plan-revision`: (1) chỉnh hai lần -> 3 phiên bản, 3 Approval khác nhau, dòng "Writing version n…" (MutationObserver, mã `[stub:slow]`), `request.planRevisions` đúng (1 `current`, 2 `superseded`, `revisionFeedback` đúng chữ, đánh số liên tục), trình duyệt chỉ gửi `generatePlan` với `mode: propose` (đếm khung WebSocket) và không `approval.reject`, bản cũ chỉ đọc + "What was asked for this version" + "Back to current", History đúng 2 dòng "Requested changes to the plan" không chữ phản hồi (màn hình và `request.activity`), quá 2000 ký tự; (2) màn hình cũ (cửa sổ thứ hai giữ khung push, API huỷ Request) -> lỗi "Someone else is already revising this plan or has decided", chữ còn trong ô; (3) `[stub:fail-once]`: thẻ lỗi "The plan could not be revised" + "Try again", Request ở `planning` không còn Approval, Try again gọi `request.generatePlan` (không `retryPlan`), bản mới mang đúng chữ cũ, Approval mới. `plan-documents`: (a) native `bug`: mục "Proposal", không "Documents", summary/notes/task/phụ thuộc/"Agent prompt", chỉ đọc; (b) OpenSpec bằng seed: thứ tự tab, số lần gọi `request.planBundleFile` (1 lúc đầu, +1 mỗi tab mới, 0 khi mở lại), tasks Rendered có checkbox chỉ đọc và Source có `orca:begin plan`, file bị lược có kích thước, không dạng khoá bí mật nào trên mọi tab, dải kiểm tra có `STRUCTURE_OK`; (c) `test.fixme` hotfix (cần luồng urgent + chẩn đoán; phủ bằng test component "offers neither for a hotfix").
- **Giả định về seed** (chưa có file khi tôi làm): `python3 plan_documents_seed.py seed <request-id>` in ra một dòng JSON cuối (`secret`, `omittedPath`, `omittedSize`; thiếu trường chỉ bỏ qua kiểm tra tương ứng) và `clean <request-id>` dọn; ca (b) tự `skip` khi file không tồn tại. Cần đối chiếu với seed của BE-045-09 khi chạy; nếu CLI khác, chỉ sửa `plantOpenspecChange`/`removePlantedChange` trong `live-plan-documents-steps.ts`.
- **Ca B** (`run_ui_flow.py --real plan-revision`, native, repo calc tạm, một vòng, chi phí nhỏ): viết trong `plan-revision.web.e2e.ts` (qua `runRealFlow`), không tự chạy; kiểm version 2, Approval mới, trình duyệt chỉ `propose`, repo không đổi. **Ca C** (`ORCA_E2E_OPENSPEC=1 … plan-documents`, kịch bản 3 trên `release/1.x`, 7-13 USD mỗi lượt sinh cộng một lượt chỉnh): đủ các nhóm tài liệu, chỉnh một lần ("Add a task for the fee rounding tests."), phiên bản 2 có `changed` và `unchanged`, không dạng khoá bí mật; skip khi thiếu cờ, khi `--ai-stub`, hoặc thiếu `tdd-agent`; không tự chạy.
- **Vai trò (approver, chỉ đọc)**: không làm ở UI (stack không cho tạo tài khoản từ spec; mật khẩu sinh ngẫu nhiên không được in); kiểm ở API bằng PRV-05 của BE-045-09. Dev server ngắt (`NO_CONNECTION`, ca g): không làm (không có công cụ ngắt dev server); PRV-09 của backend phủ.
- **Lệnh cho người điều phối**: sau khi triển khai backend CR-045 và build lại container frontend: `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- plan-documents` rồi `-- plan-revision` (ca (b) cần `plan_documents_seed.py`), rồi hồi quy `qa-bug-task-native-flow`, `qa-regressions`, `analysis-revision`, `request-base-branch`, `auto-plan-generation`. Sau lần chạy, xem ảnh `plan-documents-openspec`, `plan-proposal-native`, `revision-2-typed`, `revision-2-done`, `older-version-1`, `revision-failed`.
- **Chưa kiểm**: toàn bộ hành vi trên stack; tên ảnh theo spec ở `f.shots.take(...)` (tiền tố số do harness thêm).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
