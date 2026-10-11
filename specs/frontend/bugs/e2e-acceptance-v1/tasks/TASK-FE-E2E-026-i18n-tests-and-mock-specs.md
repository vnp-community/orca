# TASK-FE-E2E-026: i18n 5 ngôn ngữ, vitest, Playwright mock và cập nhật spec/mock bị ảnh hưởng

**From Solution:** SOL-FE-E2E-005 (mục 3.5, 5) · **Priority:** P1 · **Area:** frontend (i18n, test)
**Files:** `i18n/locales/{en,es,ja,ko,zh}.json`, `i18n/project-own-and-dedupe-locale-coverage.test.ts` (mới), `tests/stages/e2e/legacy/request-web/{add-project-own-project,request-answer-view}.web.e2e.ts` (mới), spec/mock đã có bị ảnh hưởng (`add-project-host-binding.web.e2e.ts`, `support/mock-request-ws.ts` nếu cần handler `project.create`/`repo.list`)
**Depends on:** 022 đến 025
**Status:** `[x]` XONG (2026-10-11): mã, vitest, Playwright mock và kiểm live. Kiểm live (2026-10-11): e2e `--real addproject` XANH 2/2 (Claude thật); `--real all` 7/7 XANH (24.1 phút); `--repro add-project-worktree` xanh; dữ liệu thật của người dùng không đổi. Chưa kiểm Electron bằng tay.

---

## Việc cần làm

- [x] Mọi chuỗi mới của 022 đến 025 có khoá ở en/es/ja/ko/zh; test phủ quét các tệp sở hữu.
- [x] Playwright mock: payload `project.create` (devServerId, repoPath, tên sửa được); chặn trùng không gọi RPC ghi; mã lỗi backend thành khối chặn; Answer hiện từ `answer_markdown`; ghi chú repo ở Create request.
- [x] Chạy lại spec/mock Add Project hiện có (đã đổi nếu cần) và dọn `test-results` ở gốc.
- [x] Không sửa `tests/stages/e2e/orca-flows/ui-live/**` (AF); nếu spec live hiện có vỡ vì thay đổi này thì báo lại.

## Tiêu chí hoàn thành

Vitest các tệp đã đổi xanh (trừ lỗi có sẵn trên HEAD, đối chiếu); `tsc`/`oxlint`/`oxfmt` sạch trên tệp đã đổi; Playwright `request-web` xanh.

## Rủi ro · Ước lượng

THẤP. 0,5 ngày.

## Kết quả thực thi

**Trạng thái:** XONG.

- i18n: 20 khoá mới × 5 ngôn ngữ (`en/es/ja/ko/zh.json`), test phủ `i18n/project-own-and-dedupe-locale-coverage.test.ts` (khoá có trong nguồn, không rỗng, đủ `{{placeholder}}`, không chép tiếng Anh).
- Playwright (từ gốc, `npx playwright test -c tests/configs/playwright.web.config.ts --project=request-web`): 74 xanh, 1 skipped (có sẵn); gồm `add-project-own-project.web.e2e.ts` (5), `request-answer-and-project-note.web.e2e.ts` (2) và `add-project-host-binding.web.e2e.ts` đã cập nhật (a, e, f). Đã xoá `test-results` ở gốc.
- Vitest (trong `frontend/`, `NODE_OPTIONS=--no-experimental-webstorage pnpm exec vitest run --config config/vitest.config.ts <tệp>`): 77 tệp / 831 test xanh trên tập tệp đã đổi + tệp liên quan; toàn bộ frontend 25 tệp đỏ / 79 test, tập đỏ KHÔNG có tệp nào xanh ở HEAD (đối chiếu bằng bản sao sạch của HEAD: 27 tệp đỏ / 80 test; HEAD không có `backend-go` nên hai tệp `request-rpc-contract`/`web-preload-api` không so được).
- `oxfmt --list-different` và `oxlint -c .oxlintrc.json` sạch trên mọi tệp đã đổi; `tsc --noEmit -p frontend/tsconfig.json --composite false` không thêm lỗi nào (còn lại là lỗi CÓ SẴN ở `fleet-dashboard`, `use-onboarding-flow`, `web-preload-api.ts` 741/1531/2586, `CreateProjectDialog.test.tsx` 155 ...).
- Không sửa `tests/stages/e2e/orca-flows/ui-live/**`. Lưu ý cho agent AF: `live-add-project-repro-steps.ts` dọn bằng `repo.rm`, nhưng Add Project nay tạo thêm MỘT project cùng tên thư mục (`e2e-ui-repro-*`) cho mỗi repo: phần dọn phải xoá cả project đó; ô tên project `add-project-name` mặc định = tên thư mục; thêm lại cùng thư mục sẽ hiện `add-project-duplicate`.
