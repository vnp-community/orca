# TASK-FE-E2E-019: Hiển thị host của repo, trạng thái "chưa gắn host", hành động "Đổi host" (`repo.rebindDevServer`), lỗi có mã khi tạo worktree

**From Solution:** SOL-FE-E2E-004 (mục 3.4, 3.5) · **Resolves:** người dùng không thấy/không sửa được repo cũ không có host
**Priority:** P1 · **Area:** frontend
**Files:**
- frontend/src/renderer/src/hooks/use-repo-host-rebind.ts (mới) + test (tách từ components/project/ProjectDevServerSection.tsx:73-87)
- frontend/src/renderer/src/components/sidebar/WorktreeList.tsx (214), repo-header-create-state.ts (15-), hộp thoại mới `RepoHostPickerDialog.tsx`
- frontend/src/renderer/src/components/settings/RepositoryPane.tsx, RepositoryHostSetupsSection.tsx (hàng "Host" + "Đổi host")
- frontend/src/renderer/src/lib/workspace-create-error-format.ts (14-24, 30-62), components/worktree-creation/WorktreeCreationPanel.tsx (114-140)
- frontend/src/renderer/src/store/slices/repos.ts (1317: thêm cờ `devServerInherited`)
**Depends on:** TASK-FE-E2E-016; mã lỗi từ TASK-V2-028/029 (dùng chuỗi con nên không chặn)
**Status:** `[x]` XONG (2026-10-11): mã, vitest, Playwright mock và kiểm live. Kiểm live (2026-10-11): `run_ui_flow.py --repro add-project-worktree` XANH (worktree test-02 tạo được, repo có devServerId = dev-local-mac); `--real all` 6/6 XANH (21.8 phút); backend `--strict --ai-stub` 114 PASS, 0 FAIL.

---

## Context

Repo cũ không có `dev_server_id` hiện như "local" và không có chỗ sửa ở luồng chính; backend nay trả mã lỗi rõ (SOL-V2-002 mục 3.5) mà UI phải hiển thị đúng.

## Việc cần làm

- [x] Nhãn host của repo (web): tên dev server + trạng thái; nếu kế thừa từ project thì ghi "(từ project)"; `isUnboundWebRepo` thì biểu tượng cảnh báo "Chưa gắn host".
- [x] Nút "+" tạo worktree của repo không host mở `RepoHostPickerDialog` (không phải nút chết).
- [x] `use-repo-host-rebind.ts`: gọi `repo.rebindDevServer {repoId,newDevServerId}`, xử lý `PROJECT_HAS_ACTIVE_WORKFLOWS`, `PROJECT_DEV_SERVER_NOT_FOUND`, xoá cache worktree (`invalidate...`), lấy lại repo; chỉ liệt kê host `connected` làm chọn được, host khác hiện mờ kèm trạng thái. `ProjectDevServerSection.tsx` dùng hook này.
- [x] Settings → Repository: hàng "Host" + nút "Đổi host".
- [x] `formatWorkspaceCreateError`: thêm kind `host-unbound` và `host-unreachable` TRƯỚC `NOT_A_GIT_REPO_ANCHORS`; giữ anchor `worktree_detect_failed` (lỗi detect không còn chứa "not a git repository"). `WorktreeCreationPanel`: nút "Chọn host" cho `host-unbound`, Retry cho `host-unreachable`; sau khi đổi host tự Retry tạo worktree đang chờ.
- [x] Chuỗi mới qua i18n (xem 021).

## Tiêu chí hoàn thành

- Repo không host có cảnh báo và "Đổi host" hoạt động; tạo worktree trên repo đó dẫn thẳng tới chọn host.
- Lỗi `GITGATEWAY_DEV_SERVER_UNREACHABLE` hiện "host đang offline" với Retry, không phải `chdir`.
- Repo của người dùng (ví dụ `codex`) KHÔNG bị tự đổi; chỉ đổi khi người dùng bấm.

## Kiểm thử

Vitest: hook rebind, `workspace-create-error-format` (thứ tự ưu tiên), `repo-header-create-state`, `WorktreeCreationPanel.test.tsx`; Playwright mock ở 021 (kiểm payload `repo.rebindDevServer`).

## Rủi ro · Ước lượng

LOW-MEDIUM. 1 đến 1.5 ngày.

## Kết quả thực thi

**Trạng thái:** mã + vitest + Playwright mock XANH; chưa kiểm live (mã lỗi backend của AA đã có trong cây: `PROJECT_REPO_HOST_REQUIRED`, `GITGATEWAY_REPO_HOST_UNBOUND`, `GITGATEWAY_DEV_SERVER_UNREACHABLE`, `GITGATEWAY_HOST_CHECK_FAILED`; frontend khớp bằng chuỗi con không phân biệt hoa thường).

**File đổi**
- `lib/repo-host-rebind.ts` (mới) + test: `rebindRepoToDevServer(repoId,newDevServerId)` gọi `repo.rebindDevServer {repoId,newDevServerId}`, map `PROJECT_HAS_ACTIVE_WORKFLOWS` ("dừng task đang chạy trước"), `PROJECT_DEV_SERVER_NOT_FOUND`, `FORBIDDEN`, rồi `fetchRepos` + `fetchWorktrees(repoId)`. `hooks/use-repo-host-rebind.ts` (mới, tên theo nhiệm vụ): danh sách host (chỉ host `connected` chọn được, host khác hiện mờ kèm trạng thái qua `lib/dev-server-status-label.ts`). `components/project/ProjectDevServerSection.tsx` dùng lại hàm này (test cũ `ProjectDevServerSection.test.tsx` vẫn xanh, payload RPC y nguyên).
- `components/sidebar/RepoHostPickerDialog.tsx` (mới; modal `repo-host-picker` thêm vào `store/slices/ui.ts` + `App.tsx` theo mẫu `init-repo-as-git`): chọn dev server + "Use this dev server"; lỗi inline (`data-testid="repo-host-picker-error"`); không host kết nối thì panel + nút Mở Dev Servers; thành công thì toast, đóng, và nếu mở từ panel lỗi tạo worktree (`creationId`) thì tự Retry tạo worktree đang chờ.
- `components/sidebar/RepoHostBadge.tsx` (mới) hiển thị ở hàng project trong `WorktreeList.tsx` (chỉ web): tên host (muted, `repo-host-label`, `data-host-inherited` + title khi kế thừa từ project) hoặc nút cảnh báo "No dev server" (`repo-host-unbound`, `text-destructive`) mở bộ chọn. `WorktreeList.handleCreateForRepo`: repo không host thì "+" mở bộ chọn (không phải nút chết, không mở composer chắc chắn lỗi).
- `components/settings/RepositoryDevServerHostSection.tsx` (mới) trong `RepositoryPane.tsx` (chỉ web, repo git): hàng "Dev server" (tên, trạng thái, "from the project" khi kế thừa) hoặc cảnh báo inline `repo-host-unbound-warning`, nút "Change dev server" (`repo-host-change`).
- `shared/types.ts` `Repo.devServerInherited?` + `store/slices/repos.ts`: cờ chỉ đặt `true` ở nhánh environment khi `devServerId` lấy từ project (không thêm khóa khi false để các test `toEqual` cũ giữ nguyên).
- `lib/workspace-create-error-format.ts`: kind mới `host-unbound` (`GITGATEWAY_REPO_HOST_UNBOUND`/`PROJECT_REPO_HOST_REQUIRED`/`GITGATEWAY_MISSING_DEV_SERVER_ID`), `host-unreachable` (`GITGATEWAY_DEV_SERVER_UNREACHABLE`/`INFRA_DEV_SERVER_NOT_CONNECTED`), `host-check-failed` (`GITGATEWAY_HOST_CHECK_FAILED`), kiểm TRƯỚC `NOT_A_GIT_REPO_ANCHORS` (giữ anchor `worktree_detect_failed`); `lib/pending-worktree-creation.ts` `errorKind` dùng kiểu chung; `WorktreeCreationPanel.tsx`: nút "Choose dev server" cho `host-unbound` (Retry có sẵn cho hai kind còn lại).
- Test: `workspace-create-error-format.test.ts` (+5 nhóm), `WorktreeCreationPanel.test.tsx` (nút theo kind), `RepoHostBadge.test.tsx`, `RepositoryDevServerHostSection.test.tsx`, `repo-host-rebind.test.ts`, Playwright `repo-host-rebind.web.e2e.ts`: (g) repo không host có cờ, "+" mở bộ chọn, chọn Mac Studio gửi đúng `repo.rebindDevServer {repoId,newDevServerId}`, cờ biến thành nhãn host; (h) backend từ chối `PROJECT_HAS_ACTIVE_WORKFLOWS` thì giải thích trong hộp thoại, không đổi gì; (i) repo kế thừa host hiện nhãn "inherited".
- LƯU Ý: repo `codex` của người dùng KHÔNG bị đổi; chỉ đổi khi người dùng bấm xác nhận trong bộ chọn.

**Lệnh đã chạy và kết quả thật (2026-10-10)**
- `NODE_OPTIONS=--no-experimental-webstorage pnpm exec vitest run --config config/vitest.config.ts <các tệp đã đổi/mới + components/sidebar + ProjectDevServerSection>` (trong `frontend/`): 189 tệp, 1647 xanh, 4 đỏ. 4 đỏ là `Sidebar.test.tsx` (2) và `WorktreeCardMeta.interaction.test.tsx` (2), ĐỎ SẴN trên HEAD (đối chiếu bằng bản sao sạch của HEAD).
- Toàn bộ vitest frontend: HEAD 28 tệp đỏ / 85 test; sau thay đổi 25 tệp đỏ / 79 test; tập tệp đỏ sau thay đổi là tập con của tập đỏ trên HEAD (không test nào xanh ở HEAD mà đỏ bây giờ). Một phần chênh lệch do bản sao HEAD thiếu cây `backend-go` (`request-rpc-contract.test.ts`) và do tôi chép test mới vào bản sao để tái hiện.
- `./node_modules/.bin/tsc --noEmit -p frontend/tsconfig.json --composite false`: không có lỗi mới ở tệp tôi đụng; còn lại là lỗi CÓ SẴN (`ui.ts` `previousViewBefore*`, `use-onboarding-flow.ts` `StepId`, `web-preload-api.ts` 740/1506/2567 `onboarding`/`scanRemote`/`arch`, `CreateProjectDialog.test.tsx` `MockAppState`, `web-preload-api.test.ts` ~4281).
- `./node_modules/.bin/oxlint -c .oxlintrc.json <mọi tệp ts/tsx đã đổi/mới>`: sạch (exit 0), kể cả 3 tệp CÓ SẴN vượt max-lines mà tôi đã tách (xem 017/018); `./node_modules/.bin/oxfmt --list-different` trên cùng tập: sạch sau `--write`. Không thêm `max-lines` disable nào (`git diff` không có dòng `+...-disable`).
- `npx playwright test -c tests/configs/playwright.web.config.ts --project=request-web`: 67 xanh, 1 skipped (có sẵn); gồm 9 test mới ở `add-project-host-binding.web.e2e.ts` (a-f) và `repo-host-rebind.web.e2e.ts` (g-i). Đã xoá `test-results` ở gốc sau đó.

**Blast radius (codegraph):** `formatWorkspaceCreateError`: callers `useComposerState` (x3), `repo-git-status-check`, `worktree-creation-flow`; kind mới chỉ xuất hiện khi chuỗi lỗi chứa mã host, các kind/đường cũ giữ nguyên (test cũ xanh). `WorktreeList.tsx` (6800+ dòng, đã có disable max-lines từ trước): chỉ thêm 1 badge và 1 nhánh trong `handleCreateForRepo`.

**Chưa kiểm:** luồng "Choose dev server" từ panel lỗi tạo worktree trên stack thật (chỉ vitest cho panel; Playwright mock không dựng composer tạo worktree); Electron bằng tay (các thành phần web-only tự trả `null` ngoài web).
