# TASK-FE-E2E-017: Mọi đường tạo repo còn lại mang host: clone, create, folder không phải git, `addRuntimeRepoRemote`, onboarding, Create New Project

**From Solution:** SOL-FE-E2E-004 (mục 3.2) · **Resolves:** đường #4-#10 và #14 của bảng 1.2
**Priority:** P1 · **Area:** frontend
**Files:**
- frontend/src/renderer/src/components/sidebar/useAddRepoCloneFlow.ts (160-190), useCreateRepo.ts (155-197)
- frontend/src/renderer/src/web/web-preload-api.ts (`clone` 1555, `create` 1583, `addRemote` 1572)
- frontend/src/renderer/src/runtime/runtime-repo-client.ts (`addRuntimeRepoRemote` 75-100)
- frontend/src/renderer/src/components/onboarding/AddRepoStep.tsx (84, 106, 140), use-onboarding-flow.ts (860, 884, 922, 1142)
- frontend/src/renderer/src/store/slices/repos.ts (`setupProjectClone` 3329-3370)
- frontend/src/renderer/src/components/project/CreateProjectDialog.tsx (144-197)
- frontend/src/renderer/src/lib/init-repo-as-git.ts (38-47)
**Depends on:** TASK-FE-E2E-016
**Status:** `[x]` XONG (2026-10-11): mã, vitest, Playwright mock và kiểm live. Kiểm live (2026-10-11): `run_ui_flow.py --repro add-project-worktree` XANH (worktree test-02 tạo được, repo có devServerId = dev-local-mac); `--real all` 6/6 XANH (21.8 phút); backend `--strict --ai-stub` 114 PASS, 0 FAIL. `setupProjectClone` nhánh dev server vẫn hiện 'chưa hỗ trợ' (có i18n), chưa xác minh trên web.

---

## Context

Clone/create trong hộp thoại gọi `repo.clone`/`repo.create` với `devServerId` rồi `repo.add` THIẾU `devServerId` (repo nằm trên host nhưng lưu rỗng). `addRuntimeRepoRemote` gửi `repo.add {path,kind}` (sai dạng, thiếu `projectId`/`url`). `repos.clone/create` ở preload gửi `{url,destination}`/`{parentPath,name,kind}` còn gateway đọc `{devServerId,url,destPath}` và `{devServerId,destPath,...}` (`channels_repo_ssh_status_workspace.go:456-478, 524-551`) và trả `{worktreePath,defaultBranch}`, không `.repo`. `CreateProjectDialog` không gửi `devServerId` ở `project.create` và chỉ toast khi `repo.add` lỗi.

## Việc cần làm

- [x] `useAddRepoCloneFlow` và `useCreateRepo`: sau `repo.clone`/`repo.create`, `repo.add` mang `devServerId` (đã có biến `devServerId`); dùng project mặc định theo host (016).
- [x] `web-preload-api.ts` `clone`/`create`/`addRemote`: nhận `devServerId`, sửa tên trường theo gateway, đọc đúng dạng kết quả, rồi đăng ký bằng `repo.add` có host; web thiếu host thì lỗi trước RPC.
- [x] `addRuntimeRepoRemote`: thêm `devServerId`; nhánh environment gửi dạng Go `{projectId,url,displayName,devServerId}`; bỏ cặp `repo.add`+`repo.update` sai dạng.
- [x] Onboarding: `AddRepoStep.tsx` truyền `devServerId: effectiveDevServerId` (không nhét vào `connectionId`); `use-onboarding-flow.ts` clone dùng `repo.clone` đúng trường + `devServerId`.
- [x] `setupProjectClone`: nhánh `devServer` hiện ném "not supported yet" (3338-3341); nếu `devServerId` có thì dùng clone+add đúng, nếu chưa làm thì giữ thông báo nhưng bằng chuỗi i18n.
- [x] `CreateProjectDialog`: gửi `devServerId` (và `repoPath`) ở `project.create`; lỗi `repo.add` hiện trong hộp thoại với nút thử lại thay vì chỉ toast; sửa chú thích lỗi thời dòng 171-175.
- [x] `init-repo-as-git.ts`: repo không host thì trả lỗi `REPO_HOST_REQUIRED` rõ thay vì gửi `devServerId:''`.

## Tiêu chí hoàn thành

- Clone/Create từ hộp thoại: repo lưu với `devServerId` của host; `worktree.create` trên repo đó thành công.
- Không còn lời gọi `repo.add`/`repo.clone`/`repo.create` từ web mà thiếu host hoặc sai tên trường (kiểm bằng test hợp đồng ở 021).

## Kiểm thử

Vitest: mở rộng `useAddRepoCloneFlow`/`useCreateRepo` test (nếu chưa có thì tạo), `web-preload-api.test.ts`, `runtime-repo-client.test.ts`, `CreateProjectDialog` test. Playwright mock ở 021.

## Rủi ro · Ước lượng

MEDIUM (nhiều nơi nhỏ; gateway `repo.clone/create` chưa tự đăng ký repo, giữ hai bước). 1 đến 1.5 ngày.

## Kết quả thực thi

**Trạng thái:** mã + vitest + Playwright mock XANH; chưa kiểm live.

**File đổi**
- `lib/clone-repo-onto-dev-server.ts` (mới): `cloneRepoOntoDevServer(target,{devServerId,url,destPath})` = `repo.clone {devServerId,url,destPath}` rồi `repo.add` mang cùng `devServerId` trong project theo host (đọc `worktreePath` kết quả). Dùng cho `useAddRepoCloneFlow.ts` và clone ở onboarding (`use-onboarding-flow.ts`, sửa sai tên trường `destination` và dạng kết quả `{repo}` không tồn tại).
- `useCreateRepo.ts`: `repo.add` mang `devServerId`, project theo host; nhánh `window.api.repos.create` truyền `devServerId` khi có.
- `web/web-preload-api.ts`: `clone` (`repo.clone {devServerId,url,destPath}` rồi `repo.add` với `worktreePath`), `create` (`repo.create {devServerId,destPath,defaultBranch:''}` rồi `repo.add`; `kind:'folder'` dùng `devServer.mkdir`), `addRemote`; đọc đúng dạng kết quả gateway (`{worktreePath,defaultBranch}`/`{path,defaultBranch}`).
- `runtime/runtime-repo-client.ts`: `addRuntimeRepoRemote` nhận `devServerId`; nhánh environment gửi dạng Go `{projectId,url,displayName,devServerId}` (bỏ cặp `repo.add {path,kind}`+`repo.update` sai dạng).
- `components/onboarding/AddRepoStep.tsx` (hiện không được import ở đâu): truyền `devServerId` thay vì nhét vào `connectionId`. `use-onboarding-flow.ts`: `scanNestedRepos`/`addRepoPath` mang `settings.activeDevServerId` (host chọn ở bước DevServer).
- `components/project/CreateProjectDialog.tsx`: `project.create` gửi `devServerId` + `repoPath`; sau đó `repo.list` và chỉ `repo.add` (mang `devServerId`) nếu project chưa có repo; lỗi `repo.add` hiện INLINE (`auto.components.project.CreateProjectDialog.repoAddFailed`), giữ hộp thoại, lần bấm lại dùng lại project đã tạo (không tạo trùng); sửa chú thích lỗi thời.
- `lib/init-repo-as-git.ts`: web + repo không host thì trả `REPO_HOST_REQUIRED` rõ (toast + thông điệp) thay vì gửi `devServerId:''`.
- `store/slices/repos.ts` `setupProjectClone`: nhánh `devServer` GIỮ thông báo "chưa hỗ trợ" nhưng bằng chuỗi i18n (`auto.store.slices.repos.setupProjectCloneDevServerUnsupported`); chưa xác minh `projectHostSetup.*` có chạy ở web (#13 của solution), nên không mở rộng.
- Test: `web-preload-api.test.ts` (clone/create/addRemote/hợp đồng tên trường), `useAddRepoCloneFlow.test.ts` (+ ca tạo project theo host), `useCreateRepo.default-checkout.test.ts`, `runtime-repo-client-add-remote.test.ts` (mới), `CreateProjectDialog.test.tsx` (đổi kỳ vọng: gửi host ở `project.create`, lỗi inline + thử lại không tạo trùng), `lib/init-repo-as-git.test.ts` (mới), `add-project-host-binding.web.e2e.ts` ca e (clone) và f (create).
- Tách file để không vượt max-lines (không thêm disable): `useAddRepoNestedRootFolder.ts` (tách `handleOpenNestedRootFolder` khỏi `useAddRepoNestedImportFlow.ts`, vốn 311 dòng nên oxlint đã đỏ sẵn khi tôi thêm tham số).

**Lệnh đã chạy và kết quả thật (2026-10-10)**
- `NODE_OPTIONS=--no-experimental-webstorage pnpm exec vitest run --config config/vitest.config.ts <các tệp đã đổi/mới + components/sidebar + ProjectDevServerSection>` (trong `frontend/`): 189 tệp, 1647 xanh, 4 đỏ. 4 đỏ là `Sidebar.test.tsx` (2) và `WorktreeCardMeta.interaction.test.tsx` (2), ĐỎ SẴN trên HEAD (đối chiếu bằng bản sao sạch của HEAD).
- Toàn bộ vitest frontend: HEAD 28 tệp đỏ / 85 test; sau thay đổi 25 tệp đỏ / 79 test; tập tệp đỏ sau thay đổi là tập con của tập đỏ trên HEAD (không test nào xanh ở HEAD mà đỏ bây giờ). Một phần chênh lệch do bản sao HEAD thiếu cây `backend-go` (`request-rpc-contract.test.ts`) và do tôi chép test mới vào bản sao để tái hiện.
- `./node_modules/.bin/tsc --noEmit -p frontend/tsconfig.json --composite false`: không có lỗi mới ở tệp tôi đụng; còn lại là lỗi CÓ SẴN (`ui.ts` `previousViewBefore*`, `use-onboarding-flow.ts` `StepId`, `web-preload-api.ts` 740/1506/2567 `onboarding`/`scanRemote`/`arch`, `CreateProjectDialog.test.tsx` `MockAppState`, `web-preload-api.test.ts` ~4281).
- `./node_modules/.bin/oxlint -c .oxlintrc.json <mọi tệp ts/tsx đã đổi/mới>`: sạch (exit 0), kể cả 3 tệp CÓ SẴN vượt max-lines mà tôi đã tách (xem 017/018); `./node_modules/.bin/oxfmt --list-different` trên cùng tập: sạch sau `--write`. Không thêm `max-lines` disable nào (`git diff` không có dòng `+...-disable`).
- `npx playwright test -c tests/configs/playwright.web.config.ts --project=request-web`: 67 xanh, 1 skipped (có sẵn); gồm 9 test mới ở `add-project-host-binding.web.e2e.ts` (a-f) và `repo-host-rebind.web.e2e.ts` (g-i). Đã xoá `test-results` ở gốc sau đó.

**Blast radius (codegraph):** `addRuntimeRepoRemote`: callers `AddRepoStep` (onboarding, không được import), `AddProjectFromFolderDialog`, `AddRepoSteps` (`useRemoteRepo`, SSH desktop), `NonGitFolderDialog`; nhánh desktop/SSH (`target.kind !== 'environment'` → `window.api.repos.addRemote(args)`) giữ nguyên, có test. `useAddRepoNestedImportFlow`: chỉ `AddRepoDialog`.

**Chưa kiểm:** clone/create thật trên stack (cần redeploy); `setupProjectClone` cho `devServer` (chưa làm, đã ghi lý do); onboarding bằng giao diện thật.
