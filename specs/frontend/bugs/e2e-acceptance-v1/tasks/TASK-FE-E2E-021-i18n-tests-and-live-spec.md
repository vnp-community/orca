# TASK-FE-E2E-021: i18n 5 ngôn ngữ, test hợp đồng, Playwright mock, cập nhật spec live

**From Solution:** SOL-FE-E2E-004 (mục 3.7, 4) · **Resolves:** tiêu chí xong của TASK-FE-E2E-015
**Priority:** P1 · **Area:** frontend + test
**Files:**
- frontend/src/renderer/src/i18n/locales/{en,es,ja,ko,zh}.json; frontend/src/renderer/src/i18n/repo-host-binding-locale-coverage.test.ts (mới, theo mẫu `task-open-in-git-locale-coverage.test.ts`)
- tests/stages/e2e/legacy/request-web/add-project-host-binding.web.e2e.ts (mới; dùng `support/mock-request-ws.ts`: `on`, `callsTo`)
- tests/stages/e2e/orca-flows/ui-live/add-project-worktree-repro.web.e2e.ts (đổi tên nội dung đoạn 140-155 không cần; thêm xác nhận), live-add-project-repro-steps.ts, tests/stages/e2e/orca-flows/run_ui_flow.py (11, 37-38 `REPRO_SPECS`)
**Depends on:** TASK-FE-E2E-016 đến 020
**Status:** `[x]` XONG (2026-10-11): mã, vitest, Playwright mock và kiểm live. Kiểm live (2026-10-11): `run_ui_flow.py --repro add-project-worktree` XANH (worktree test-02 tạo được, repo có devServerId = dev-local-mac); `--real all` 6/6 XANH (21.8 phút); backend `--strict --ai-stub` 114 PASS, 0 FAIL. Chưa kiểm Electron bằng tay.

---

## Việc cần làm

- [x] i18n: mọi chuỗi mới của 016 đến 020 vào đủ 5 ngôn ngữ; test phủ khoá (lấy khoá bằng regex trên nguồn, như mẫu). Các script `verify:localization-*` không chạy được trong cây này (thiếu `config/scripts/*`); ghi rõ trong kết quả.
- [x] Test hợp đồng "mọi `repo.add`/`repo.clone`/`repo.create` từ web có `devServerId` và đúng tên trường gateway" (đối chiếu bảng kênh `channel_inventory.json` hoặc hằng tên trường), mở rộng tinh thần TASK-FE-E2E-001.
- [x] Playwright mock: (a) 1 host: chọn sẵn, `repo.add` có `devServerId`; (b) 2 host: bắt chọn; (c) 0 host: chặn + nút tới Settings; (d) clone/create gọi `repo.add` có host; (e) repo không host: "Đổi host" gửi `repo.rebindDevServer`; (f) host ngắt khi đang Browse: không đóng.
- [x] Spec live `add-project-worktree-repro`: giữ nguyên các assert hành vi đúng (đã đúng); sau khi sửa phải XANH. Thêm assert `project holding the repo` có `devServerId` = host đã chọn (project tạo theo host) và đổi nhật ký "BUG REPRODUCED" thành chỉ in khi còn lỗi. KHÔNG xoá spec; nó thành spec hồi quy. Cập nhật mô tả trong `run_ui_flow.py` (dòng 11, 37) từ "RED while the bug exists" sang hồi quy.
- [x] Chạy `--repro add-project-worktree` (xanh) và `--real all` (không hồi quy, 6/6) trên stack local với repo tạm.

## Tiêu chí hoàn thành

- Vitest các file đã đổi xanh; `tsc --noEmit -p config/tsconfig.tc.web.json` và `oxlint` sạch trên file đã đổi (lỗi có sẵn trên HEAD ghi như README).
- `request-web` Playwright xanh, spec live xanh, `--real all` 6/6.

## Rủi ro · Ước lượng

LOW. 1 ngày.

## Kết quả thực thi

**Trạng thái:** i18n, test hợp đồng, Playwright mock và cập nhật spec live XONG; CHƯA chạy spec live (`--repro add-project-worktree`, `--real all`) vì người điều phối chưa redeploy và đã dặn không chạy.

**File đổi**
- i18n: `i18n/locales/{en,es,ja,ko,zh}.json` thêm 45 khoá (`auto.components.sidebar.{AddRepoHostGateNotice,RepoHostBadge,RepoHostPickerDialog,AddRepoHostSelector.chooseHost,RemoteFileBrowser.*}`, `auto.components.settings.RepositoryDevServerHostSection.*`, `auto.components.project.CreateProjectDialog.repoAddFailed`, `auto.components.worktree.creation.WorktreeCreationPanel.chooseHost`, `auto.lib.{repoHostBinding,repoHostRebind}.*`, `auto.lib.workspace.create.error.format.{hostUnbound,hostUnreachable,hostCheckFailed}.*`, `auto.store.slices.repos.setupProjectCloneDevServerUnsupported`), giữ nguyên `{{placeholder}}`.
- `i18n/repo-host-binding-locale-coverage.test.ts` (mới, theo mẫu `task-open-in-git-locale-coverage.test.ts`): khoá liệt kê tường minh cho tệp dùng chung + quét mọi khoá của 6 tệp thuộc tính năng (khoá mới thêm vào đó mà thiếu bản dịch sẽ đỏ); kiểm cả 5 ngôn ngữ không rỗng, giữ placeholder, không chép nguyên tiếng Anh (12 test xanh). LƯU Ý: nhiều khoá CÓ SẴN trên HEAD cũng thiếu trong catalog (ví dụ `ProjectDevServerSection.noRepos`, `WorktreeCreationPanel.initAsGitRepo`, `lib.init.repo.as.git.*`, `notAGitRepo.*`); tôi KHÔNG bổ sung các khoá không thuộc tính năng này. Các script `verify:localization-*` ở `package.json` trỏ `config/scripts/...` không có trong cây nên không chạy được; dựa vào test phủ.
- Test hợp đồng: `web-preload-api.test.ts` khối `host binding (devServerId)` đọc `backend-go/.../channels_repo_ssh_status_workspace.go` (bỏ qua nếu thiếu cây backend), lấy tag JSON của `repo.add/repo.clone/repo.create` và kiểm mọi khóa payload ⊆ tag đó và `devServerId` có mặt; kèm test từng đường thiếu host thì ném `REPO_HOST_REQUIRED` TRƯỚC mọi RPC (không để project mồ côi).
- Playwright mock (request-web): `add-project-host-binding.web.e2e.ts` (a-f), `repo-host-rebind.web.e2e.ts` (g-i), `support/mock-project-host-gateway.ts` (cổng giả project/repo/browse; tách theo trách nhiệm để mỗi tệp < 300 dòng). Ca d dùng `devServerStatus.push` để giả host rớt; ca c kiểm nút tới Dev Servers.
- Spec live `ui-live/add-project-worktree-repro.web.e2e.ts`: giữ NGUYÊN các assert hành vi đúng, thêm assert `project.list devServerId` của project chứa repo = host đã chọn; chú thích đổi sang "regression", dòng `BUG REPRODUCED` chỉ in khi lỗi cũ quay lại (vốn đã có điều kiện). `tests/stages/e2e/orca-flows/run_ui_flow.py` (dòng 11, 37) và `orca-flows/README.md` mô tả lại thành spec hồi quy.

**Lệnh đã chạy và kết quả thật (2026-10-10)**
- `NODE_OPTIONS=--no-experimental-webstorage pnpm exec vitest run --config config/vitest.config.ts <các tệp đã đổi/mới + components/sidebar + ProjectDevServerSection>` (trong `frontend/`): 189 tệp, 1647 xanh, 4 đỏ. 4 đỏ là `Sidebar.test.tsx` (2) và `WorktreeCardMeta.interaction.test.tsx` (2), ĐỎ SẴN trên HEAD (đối chiếu bằng bản sao sạch của HEAD).
- Toàn bộ vitest frontend: HEAD 28 tệp đỏ / 85 test; sau thay đổi 25 tệp đỏ / 79 test; tập tệp đỏ sau thay đổi là tập con của tập đỏ trên HEAD (không test nào xanh ở HEAD mà đỏ bây giờ). Một phần chênh lệch do bản sao HEAD thiếu cây `backend-go` (`request-rpc-contract.test.ts`) và do tôi chép test mới vào bản sao để tái hiện.
- `./node_modules/.bin/tsc --noEmit -p frontend/tsconfig.json --composite false`: không có lỗi mới ở tệp tôi đụng; còn lại là lỗi CÓ SẴN (`ui.ts` `previousViewBefore*`, `use-onboarding-flow.ts` `StepId`, `web-preload-api.ts` 740/1506/2567 `onboarding`/`scanRemote`/`arch`, `CreateProjectDialog.test.tsx` `MockAppState`, `web-preload-api.test.ts` ~4281).
- `./node_modules/.bin/oxlint -c .oxlintrc.json <mọi tệp ts/tsx đã đổi/mới>`: sạch (exit 0), kể cả 3 tệp CÓ SẴN vượt max-lines mà tôi đã tách (xem 017/018); `./node_modules/.bin/oxfmt --list-different` trên cùng tập: sạch sau `--write`. Không thêm `max-lines` disable nào (`git diff` không có dòng `+...-disable`).
- `npx playwright test -c tests/configs/playwright.web.config.ts --project=request-web`: 67 xanh, 1 skipped (có sẵn); gồm 9 test mới ở `add-project-host-binding.web.e2e.ts` (a-f) và `repo-host-rebind.web.e2e.ts` (g-i). Đã xoá `test-results` ở gốc sau đó.

**Chưa kiểm:** `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --repro add-project-worktree` (phải XANH sau khi redeploy frontend + backend AA) và `--real all` (6/6, kiểm không hồi quy); đóng TASK-FE-E2E-015 sang `[x]` và README (việc của người điều phối).
