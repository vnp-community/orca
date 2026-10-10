# TASK-FE-E2E-015: Web "Add Project" bỏ qua host đã chọn, repo không gắn dev server, nên tạo worktree chạy git trong container

**From:** báo lỗi của người dùng (dùng Orca web trên stack local) · **Resolves:** `Couldn't create worktree: WORKTREE_CREATE_FAILED ... chdir <repo>: no such file or directory`
**Priority:** P1 (không tạo được worktree cho mọi project thêm từ web) · **Area:** frontend (web preload + store) + backend (git-gateway/project-service, lỗi báo sai)
**Files (dự kiến, CHƯA sửa):**
- frontend/src/renderer/src/web/web-preload-api.ts (`createReposApi().add` và `.addRemote`: gọi `repo.add` không kèm `devServerId`)
- frontend/src/renderer/src/store/slices/repos.ts (`addRepoPath`: cả hai nhánh, `window.api.repos.add({ path, kind })` và `callRuntimeRpc('repo.add', { projectId, url, displayName })`, không truyền host)
- frontend/src/renderer/src/components/sidebar/AddRepoDialog.tsx và `useAddRepoServerPathFlow.ts` (host đã chọn trong hộp thoại không tới `addRepoPath`)
- backend-go/services/git-gateway-service/internal/usecase/ports.go (`dispatchExecutorForRepo`: `repo.DevServerID == ""` thì chạy git trong container)
- tests/stages/e2e/orca-flows/ui-live/add-project-worktree-repro.web.e2e.ts + `live-add-project-repro-steps.ts` (spec tái hiện, đã có), run_ui_flow.py (`--repro`)
**Depends on:** không
**Status:** `[x]` ĐÃ SỬA (2026-10-11) bằng TASK-FE-E2E-016..021 và TASK-V2-035; spec tái hiện chuyển xanh. Kiểm live (2026-10-11): `run_ui_flow.py --repro add-project-worktree` XANH (worktree test-02 tạo được, repo có devServerId = dev-local-mac); `--real all` 6/6 XANH (21.8 phút); backend `--strict --ai-stub` 114 PASS, 0 FAIL.

---

## Giải pháp và danh sách task (thêm 2026-10-10; nội dung tái hiện bên dưới KHÔNG đổi)

- Giải pháp: [SOL-FE-E2E-004](../solutions/SOL-FE-E2E-004-web-repo-host-binding.md) (frontend) và [SOL-V2-002](../../../../backend-go/bugs/e2e-acceptance/v2/solutions/SOL-V2-002-repo-host-binding-and-no-silent-local.md) (backend).
- Task frontend: [016](./TASK-FE-E2E-016-pass-dev-server-id-on-add-project.md), [017](./TASK-FE-E2E-017-pass-dev-server-id-on-other-creation-paths.md), [018](./TASK-FE-E2E-018-add-project-host-selection-ux.md), [019](./TASK-FE-E2E-019-show-and-change-repo-host.md), [020](./TASK-FE-E2E-020-bounded-remote-file-browser.md), [021](./TASK-FE-E2E-021-i18n-tests-and-live-spec.md).
- Task backend: TASK-V2-028 đến 034 ([README](../../../../backend-go/bugs/e2e-acceptance/v2/tasks/README.md)).
- Quyết định đã chọn: repo luôn gắn dev server, project gắn dev server và repo thừa hưởng, chọn sẵn host duy nhất, repo cũ không tự migrate, Browse folder có giới hạn.
- Trạng thái vẫn `[ ]` cho tới khi spec `--repro add-project-worktree` XANH.

## Context

Người dùng thao tác bình thường trên web (frontend `http://localhost:28080`, gateway `http://localhost:8081`) và gặp lỗi. Theo yêu cầu của người dùng, đây là bug sản phẩm, không phải thao tác sai: không được tránh bằng cách đổi cách dùng.

Các bước (mô tả của người dùng, đối chiếu với giao diện thật):

1. Đăng nhập Orca web.
2. Thanh Projects ở sidebar: nút `+` (aria-label `Add Project`; hai nút còn lại của hàng là bộ lọc và "tạo nhóm") mở hộp thoại "Add a project".
3. Hàng `Host`: chọn `dev-local-mac` (đang `Connected`).
4. `Browse folder` mở "Browse host filesystem".
5. Chọn thư mục repo git, `Select folder` (quay về bước "Open host project" với `Host path` đã điền), `Add Git Project`.
6. Di chuột vào project vừa thêm (cột trái), bấm `+` nhỏ ("Create new worktree for <project>"), hộp thoại "Create worktree", gõ tên `test-02`, `Create worktree`.

Kết quả: panel đỏ `Couldn't create worktree` với `rpc error: code = Internal desc = WORKTREE_CREATE_FAILED: git worktree add failed: git worktree add <repo>-test-02 -b test-02 : chdir <repo>: no such file or directory:` (thư mục repo có tồn tại trên máy Mac).

### Lệch giữa mô tả của người dùng và giao diện thật (ghi nhận, không phải lỗi)

- Ở bước 3 hộp thoại đã chọn sẵn `dev-local-mac` (host duy nhất đang kết nối được liệt kê; các dev server `disconnected` không có trong danh sách). Spec vẫn mở bộ chọn và bấm chọn `dev-local-mac` một cách tường minh.
- Ở bước 4 đường dẫn có thể gõ thẳng vào ô lọc ("Type to filter or enter a path"), Enter để vào thư mục.
- Ở bước 5 sau `Select folder` hộp thoại chuyển sang bước "Open host project" (ô `Host path`), nút `Add Git Project` nằm ở đó.
- Ở bước 6 nút `+` của hàng project CHỈ hiện khi rê chuột vào hàng; lỗi hiện trong panel chính (tab `test-02` có biểu tượng cảnh báo, nút `Retry`/`Dismiss`) kèm hàng "pending" đỏ ở sidebar, không phải toast.

## Cách tái hiện

```bash
python3 tests/stages/e2e/orca-flows/run_ui_flow.py --repro add-project-worktree
# hoặc
ORCA_E2E_LIVE=1 npx playwright test -c tests/configs/playwright.web.config.ts --project=request-live add-project-worktree-repro
```

Spec tạo repo git tạm `/private/tmp/e2e-ui-repro-XXXXXX` (không phải repo của người dùng), đi đúng 6 bước qua UI, chụp ảnh từng bước vào `test-results/` (ngoài git), bắt thông báo lỗi trên UI, rồi đọc bằng chứng chỉ đọc và dọn.

## Kết quả chạy (2026-10-10, stack local hiện có, 4 lượt liên tiếp giống nhau, ~8 giây mỗi lượt)

Thông báo nguyên văn trên UI (lượt cuối, đường dẫn rút gọn là `<repo>`):

```
Couldn’t create worktree
rpc error: code = Internal desc = WORKTREE_CREATE_FAILED: git worktree add failed: git worktree add /private/tmp/e2e-ui-repro-Ls5j7A-test-02 -b test-02 : chdir /private/tmp/e2e-ui-repro-Ls5j7A: no such file or directory:
```

Spec in `BUG REPRODUCED` (khớp `WORKTREE_CREATE_FAILED` + `chdir` + `no such file or directory`) và 4 assert của hành vi đúng đều đỏ:

| Assert (hành vi đúng) | Thực tế |
|---|---|
| UI không báo lỗi, worktree `test-02` được tạo | báo lỗi như trên |
| thư mục `<repo>-test-02` có trên đĩa | không có |
| `test-02` có trong `worktree.list` của repo | danh sách rỗng |
| `repo.list` cho repo có `devServerId` = id của `dev-local-mac` | `devServerId = ""` |

## Nguyên nhân (ĐÃ XÁC MINH bằng bằng chứng)

Gợi ý ban đầu đúng, kèm hai điều chỉnh (xem "Đính chính").

1. **Repo lưu không có dev server.** Sau khi thêm qua UI, `repo.list` (qua gateway, chỉ đọc) trả `devServerId: ""` cho repo vừa thêm. Cùng tình trạng với repo `codex` của người dùng (`devServerId` rỗng; không thao tác gì trên nó) và mọi repo thêm từ web. Các repo tạo bằng `project.create {devServerId}` (ví dụ `probe-proj`, `Demo (this Mac)`) thì có `devServerId`.
2. **Mã frontend bỏ host.** `createReposApi().add` và `.addRemote` (`web-preload-api.ts`) gọi `repo.add {projectId, url, displayName}` và chỉ nhận `{path}` hay `{remotePath, displayName}`; `addRepoPath` trong `store/slices/repos.ts` cũng gửi `{projectId, url, displayName}` ở nhánh runtime và `{path, kind}` ở nhánh local. Host chọn trong hộp thoại chỉ dùng để duyệt thư mục (`devServer.browseDir` có `id` của dev server), không bao giờ đến `repo.add`.
3. **Gateway nhận được trường này nhưng không ai gửi.** Handler `repo.add` (`channels_repo_ssh_status_workspace.go`) đọc `devServerId` ("empty = local repo, validated against infra-fleet server-side when non-empty") và chuyển cho project-service.
4. **git-gateway coi "không có dev server" là "chạy cục bộ".** `dispatchExecutorForRepo` (`ports.go`): `if repo.DevServerID == "" { return ctx, local, repo.URL, nil }`; executor `local` là tiến trình git trong container `orca-go-git-gateway`.
5. **Container không thấy đường dẫn.** `docker exec orca-go-git-gateway test -d <repo>` trả không tồn tại (spec in `git-gateway container sees <repo>: false`). Log container có đúng dòng: `WORKTREE_CREATE_FAILED ... git worktree add <repo>-test-02 -b test-02 : chdir <repo>: no such file or directory` và `GITGATEWAY_CHECK_HOOKS_FAILED: read hooks dir: open <repo>/.git/hooks: no such file or directory`. Cùng nguyên nhân gây `WORKTREE_DETECT_FAILED`/`GITGATEWAY_CHECK_HOOKS_FAILED` lặp lại trên sidebar cho mọi repo kiểu này (đã thấy trong console của trang, xem TASK-FE-E2E-014).
6. **Project mặc định.** Repo vào project `My Repos` (tạo/tìm bởi `getOrCreateDefaultProjectId`). Project đó có `devServerId` riêng (`local-mac`, đang `disconnected`) KHÁC `dev-local-mac`; repo không thừa hưởng nó (`repo.list` vẫn trả rỗng), nên ngay cả khi project có dev server thì repo vẫn rơi vào nhánh "cục bộ".

### Đính chính so với gợi ý ban đầu

- Gợi ý nói project "My Repos" không có dev server. Thực tế `My Repos` CÓ `devServerId` (`local-mac`, disconnected); điều quyết định là `repos.dev_server_id` của chính repo, và nó rỗng. Kết luận không đổi: repo rơi vào nhánh `DevServerID == ""`.
- Gợi ý chỉ nêu `repos.add`/`repos.addRemote` của preload. Cả `addRepoPath` trong store (cả hai nhánh) cũng bỏ host, nên sửa chỉ ở preload là chưa đủ nếu nhánh runtime còn được dùng.

## Phát hiện phụ (không phải nguyên nhân của lỗi này, cùng luồng Browse folder)

- **Duyệt thư mục lớn làm agent bị ngắt kết nối.** Hộp thoại Browse folder liệt kê MỌI thư mục cha của đường dẫn gõ vào (`resolvePathInput` đi qua từng đoạn). Gõ đường dẫn nằm dưới `$TMPDIR` của macOS (`/private/var/folders/.../T`, ~1000 mục) làm `devServer.browseDir` trả `INFRA_AGENT_EXEC_FAILED: failed to relay to dev server agent`; log `orca-go-infra-fleet`: `devserveragent: connection lost: failed to read: websocket: message too big: read limited at 32769 bytes`, rồi `dev server disconnected` (dev-local-mac về `connected` lại sau khoảng 30 giây, agent tự nối lại, không ai khởi động lại). Hộp thoại duyệt cũng đóng im lặng. Người dùng duyệt tới thư mục có hơn khoảng 450 mục sẽ gặp lại. Đề xuất: giới hạn/phân trang kết quả `browseDir` ở agent hoặc nâng giới hạn đọc frame của infra-fleet, và hiện lỗi thay vì đóng hộp thoại. Chưa có task riêng. Spec tránh bằng cách đặt repo tạm ở thư mục có mọi thư mục cha nhỏ.
- Trong lúc khảo sát (lần chạy đầu tôi chọn nhầm thư mục `/`), sidebar còn hiện banner "Git is not installed" và hộp "Add a setup script / Couldn't verify this repo's setup script". Cả hai nhiều khả năng cùng nguyên nhân (git chạy trong container); chưa kiểm riêng.

## Việc cần làm (đề xuất, CHƯA làm)

1. **Frontend:** truyền `devServerId` của host chọn trong hộp thoại Add Project xuống `repo.add`: `AddRepoDialog` / `useAddRepoServerPathFlow` giữ `selectedHostId`; `addRepoPath(path, kind, { devServerId })`; `createReposApi().add` và `.addRemote` nhận và chuyển `devServerId` (host `local` thật của app desktop thì để trống). Áp dụng cả clone/create nếu cùng lỗ hổng (`repo.clone`, `repo.create` đã có `devServerId` ở gateway).
2. **Backend, không âm thầm quay về local:** `repo.add` từ web không kèm `devServerId` thì project-service dùng `devServerId` của project nếu có, ngược lại trả lỗi rõ ("repo chưa gắn dev server, chọn host") thay vì lưu repo rỗng; `dispatchExecutorForRepo` với repo rỗng dev server khi git-gateway chạy trong container (không có host cục bộ thật) phải báo `REPO_NOT_BOUND_TO_DEV_SERVER` thay vì chạy git trong container và trả `chdir: no such file or directory`.
3. **Dữ liệu cũ:** repo đã có `devServerId` rỗng (ví dụ `codex` của người dùng) cần cách gắn lại (`repo.rebindDevServer` đã có trong gateway: `newDevServerId`) hoặc migration chọn dev server mặc định; không tự động đổi dữ liệu của người dùng ở task này.
4. **Duyệt thư mục:** xem "Phát hiện phụ".

## Tiêu chí

- Chạy lại `run_ui_flow.py --repro add-project-worktree`: không còn `BUG REPRODUCED`; worktree `test-02` được tạo, thư mục `<repo>-test-02` có trên đĩa, có trong danh sách, `repo.list` trả `devServerId` = id của `dev-local-mac`; spec XANH và vẫn dọn sạch.
- Thêm project từ web mà không có host nào chọn được thì UI báo lỗi rõ, không lưu repo "mồ côi".
- Thêm repo qua gateway không kèm `devServerId` (kênh tắt) không còn dẫn tới `chdir: no such file or directory`; trả lỗi có mã.

## Kiểm thử

- Spec live: `ui-live/add-project-worktree-repro.web.e2e.ts` (đã có, ĐỎ).
- Vitest cần thêm khi sửa: `web-preload-api` `repos.add` truyền `devServerId`; `addRepoPath` truyền host; mock Playwright của hộp thoại Add Project kiểm payload `repo.add` có `devServerId`.
- Go test cần thêm: `dispatchExecutorForRepo` repo rỗng dev server trả lỗi có mã trong chế độ container; project-service `AddRepo` không `devServerId`.

## Kết quả thực thi

Chưa sửa mã sản phẩm (task này chỉ tái hiện và ghi nhận). Dọn dẹp mỗi lượt: repo tạm bị `repo.rm` khỏi Orca, thư mục tạm (`e2e-ui-repro-*`, `<repo>-test-02`) bị xoá; kiểm 0 repo/project `e2e-ui-repro-*` còn lại, 0 thư mục tạm còn lại. Trong lúc khảo sát tôi đã tự thêm rồi tự gỡ (`repo.rm`) hai repo tên `/` và một repo `e2e-ui-repro-explore-*` vào project `My Repos`; không đụng repo `codex` hay các project `probe-proj`/`Demo (this Mac)` có sẵn.
