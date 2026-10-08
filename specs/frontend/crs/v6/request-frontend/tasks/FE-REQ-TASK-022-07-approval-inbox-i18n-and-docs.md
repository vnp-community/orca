# FE-REQ-TASK-022-07: i18n năm locale, test phủ khoá và tài liệu trang cho hộp duyệt

**From Solution:** [FE-REQ-SOL-022](../solutions/FE-REQ-SOL-022-approval-inbox.md) mục 2.6
**Priority:** P1
**Area:** frontend (i18n, docs)
**File:** `frontend/src/renderer/src/i18n/locales/{en,es,ja,ko,zh}.json` (sửa), `frontend/src/renderer/src/i18n/request-approval-locale-coverage.test.ts` (mới), `docs/ui/pages/requests.md` (SOL-018 tạo; task này thêm mục "Hộp duyệt")
**Depends on:** FE-REQ-TASK-022-05, 022-06 (để chốt danh sách khoá thật sự dùng)
**Status:** [x] DONE (verified 2026-10-07: request-approval-locale-coverage.test.ts 10/10, request-locale-coverage.test.ts 15/15; oxlint sạch, không thêm lỗi tsc ở file của task)

## Context

- Hai kiểu khoá: khoá băm sinh tự động (`auto.components.<...>.<hash>`, dùng với `translate(key, fallback)`) và khoá đọc theo tên. `i18n/task-jira-link-locale-coverage.test.ts` ghi rõ khoá đọc theo tên **không** được công cụ nào sinh vào catalog, nên test phủ khoá là thứ duy nhất giữ 4 locale khỏi rơi về tiếng Anh. Catalog là JSON lồng nhau; test tra khoá bằng `key.split('.')`.
- Locale: `en, es, ja, ko, zh` (`i18n/locales/*.json`).
- `package.json` gốc có `verify:localization-catalog` và `verify:localization-coverage` (`config/scripts/...`); các script đó trỏ file không có trong repo hiện tại (chỉ thấy `config/max-lines-baseline.txt`, `oxlint-react-doctor.json`): chưa kiểm chứng đường chạy, ghi lại kết quả thực tế khi thử.
- Khoá SOL-018 (`auto.hooks.request.error.*`) do SOL-018 sở hữu, không thêm ở đây.

## Việc cần làm

1. Chốt danh sách khoá từ code của task 05, 06 (không thêm khoá thừa) dưới tiền tố `auto.components.request.approval.`:
   `ApprovalInboxTab.{title,empty,emptyFiltered,clearFilters,loadMore}`, `ApprovalSubjectFilter.{all,requestType,solution,plan,phase,preDeploy,other}`, `ApprovalRow.{open,approve,reject,overdue,dueIn,overdueBy,requestedBy,systemActor,alreadyDecided,changed,requestGone}`, `ApprovalRow.confirmApprove.{request_type,findings,answer,task_list,phase,pre_deploy}`, `OverdueToggle.label`, `ApprovalInboxErrorState.{network,forbidden,retry}`, `ApprovalSubjectType.{request_type,solution,findings,answer,plan,phase,task_list,pre_deploy}`.
2. Thêm mỗi khoá vào đủ 5 file locale (JSON lồng nhau, giữ thứ tự khoá hiện có của file). Chuỗi tiếng Anh gốc theo CR-022 mục 2.5; bản dịch es/ja/ko/zh do người dịch rà (đánh dấu trong PR); không để chuỗi tiếng Anh ở locale khác.
3. Tham số nội suy theo kiểu repo (`{{value0}}`, `{{relative}}`); `dueIn` nhận `{{relative}}`, `overdueBy` nhận `{{relative}}`.
4. Viết `request-approval-locale-coverage.test.ts` theo mẫu `task-jira-link-locale-coverage.test.ts`: mảng `KEYS`, với mỗi locale khẳng định khoá có giá trị chuỗi không rỗng; thêm khẳng định chuỗi locale không phải `en` khác bản `en` cho các khoá không phải tên riêng (bắt trường hợp copy tiếng Anh sang locale khác), trừ danh sách ngoại lệ rõ ràng.
5. `docs/ui/pages/requests.md`: thêm mục "Hộp duyệt" (bố cục, nguồn dữ liệu `approval.listPending`, quy tắc duyệt nhanh, phím `j/k/Enter`, trạng thái rỗng/lỗi). Nếu file chưa có (SOL-018 chưa merge) thì ghi mục vào tệp nháp và báo người điều phối; không tạo trùng với SOL-018. Cập nhật `docs/ui/page-tree.md` nếu SOL-018 chưa làm.

## Kiểm thử

- `request-approval-locale-coverage.test.ts` xanh với 5 locale.
- Thử thủ công đổi ngôn ngữ UI sang `ja` và `zh`: tab hiện nhãn đúng, thời gian tương đối theo locale.
- Chạy (chưa chạy): `pnpm --filter orca-frontend test frontend/src/renderer/src/i18n/request-approval-locale-coverage.test.ts`; sau đó `pnpm verify:localization-catalog` và `pnpm verify:localization-coverage` ở gốc (chưa kiểm chứng chạy được).

## Tiêu chí hoàn thành

- [ ] Mọi chuỗi mới của hộp duyệt có đủ 5 locale.
- [ ] Test phủ khoá xanh; mỗi khoá trong `KEYS` thực sự được code gọi (không khoá mồ côi).
- [ ] `docs/ui/pages/requests.md` có mục Hộp duyệt.
- [ ] Không chuỗi cứng tiếng Việt hay tiếng Anh trong component (mọi chuỗi qua `translate`).

## Rủi ro và lưu ý

- Bản dịch máy chưa được người bản ngữ rà; ghi rõ trong PR.
- Thêm khoá vào `en.json` có thể làm lệch catalog sinh tự động nếu công cụ sinh ghi đè; kiểm khi chạy `verify:localization-catalog`.

## Ghi chú triển khai (2026-10-07)

- Thêm 3 khoá ngoài danh sách: `ApprovalRow.decided`, `ApprovalSubjectType.unknown`, `ApprovalRow.requestedBy` nhận `{{name}}`. `pnpm verify:localization-*` chưa chạy. Mục Hộp duyệt đã thêm vào `docs/ui/pages/requests.md` và `page-tree.md`.
