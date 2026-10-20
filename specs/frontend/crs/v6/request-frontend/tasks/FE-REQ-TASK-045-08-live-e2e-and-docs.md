# FE-REQ-TASK-045-08: Spec UI live (`plan-documents`, `plan-revision`) trên repo fixture tài chính và tài liệu

**From Solution:** [FE-REQ-SOL-045](../solutions/FE-REQ-SOL-045-plan-documents-and-revision-ui.md)
**Priority:** P1
**Area:** frontend / request / plan
**File:** `tests/stages/e2e/orca-flows/ui-live/plan-documents.web.e2e.ts`, `plan-revision.web.e2e.ts` (mới), `live-plan-documents-steps.ts`, `live-plan-revision-steps.ts`, `live-plan-revision-panel-recorder.ts` (mới; dùng lại `live-analysis-revision-panel-recorder.ts` và `live-push-frame-gate.ts` nếu phù hợp), `tests/stages/e2e/orca-flows/run_ui_flow.py` (đăng ký), `tests/stages/e2e/orca-flows/README.md` (dòng mới), dùng lại `tests/stages/e2e/orca-flows/plan_documents_seed.py` (BE-REQ-TASK-045-09)
**Depends on:** FE-REQ-TASK-045-03 đến 045-07, backend BE-REQ-TASK-045-09; FE-REQ-TASK-044-04 (mẫu spec `auto-plan-generation`, dùng lại bước chờ Plan)
**Status:** [ ] TODO

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

- [ ] Spec xanh cho Proposal native, Documents OpenSpec (seed hoặc claude giả OpenSpec), chỉnh hai lần, bản cũ chỉ đọc, "Try again", Activity.
- [ ] Hai vai trò (approver, chỉ đọc) được kiểm ở API (PRV-05) và, nếu tạo được tài khoản, ở UI.
- [ ] Ca loại B và C có lệnh, điều kiện, chi phí ghi rõ; không tự chạy.
- [ ] Không hồi quy các spec UI hiện có.

## Rủi ro và lưu ý

- Dòng "Writing version n…" chỉ hiện chốc lát: dùng `MutationObserver` và mã `[stub:slow]` như TASK-041-04.
- Dữ liệu seed là giả: dọn sau khi chạy; không dùng ở stack dùng chung.
- Gateway sập sau `redeploy-local.sh`: `docker compose up -d --force-recreate api-gateway`, khởi động lại các dịch vụ theo thứ tự đã ghi.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
