# TASK-FE-E2E-014: "Open in Git" mở Source Control nhưng danh sách thay đổi trống (G5) và spawn-helper của node-pty (G6)

**From Solution:** SOL-FE-E2E-003 · **Resolves:** TASK-FE-E2E-011 G5 (nguyên nhân gốc phía frontend), G6 (độ bền của quyền thực thi `spawn-helper`)
**Priority:** P1 · **Area:** frontend (web preload) + agent
**Files:**
- frontend/src/renderer/src/web/runtime-worktree-path-owner.ts (mới) + `.test.ts` (6 test), web-preload-task-worktree-git-status.test.ts (mới, 3 test)
- frontend/src/renderer/src/web/web-preload-api.ts (`listAllRuntimeDetectedWorktrees`, `listAllBookkeptRuntimeWorktrees` mới, `resolveRuntimeWorktreeByPath`)
- agent/src/relay/node-pty-spawn-helper-permissions.ts (mới) + `.test.ts` (5 test), pty-handler.ts và agent-spawner.ts (mỗi file một dòng gọi)
**Depends on:** TASK-FE-E2E-011 (khảo sát G5/G6), TASK-FE-E2E-013 (điều hướng "Open in Git")
**Status:** `[x]` XONG (2026-10-10): sửa `resolveRuntimeWorktreeByPath` (allSettled + dự phòng) và agent tự sửa quyền `spawn-helper`; kiểm trên stack thật: terminal worktree mở được, lỗi `No runtime worktree owns` hết. Còn G5b (`SCM_EMPTY_BRANCH`) chưa sửa.

---

## Gốc rễ THẬT của G5

Lỗi `No runtime worktree owns <path>` do `resolveRuntimeWorktreeByPath` (web-preload-api.ts) sinh ra, và mọi lệnh `git.*`/`files.*` theo đường dẫn của web đều đi qua nó. Hàm này đối chiếu đường dẫn với hàng của `worktree.detectedList` của TẤT CẢ repo trong TẤT CẢ project người dùng nhìn thấy, gom bằng `Promise.all`. Chỉ cần MỘT repo làm `worktree.detectedList` lỗi (dev server của repo đó offline, hoặc thư mục repo đã bị xoá: backend trả `WORKTREE_DETECT_FAILED: git worktree list failed`, mã này không nằm trong danh sách lỗi mềm) thì `Promise.all` từ chối, `listAllRuntimeDetectedWorktrees` bắt lỗi và trả `[]`, nên worktree khoẻ của task cũng "không ai sở hữu". `git.status`/`git.upstreamStatus` lỗi, Source Control không có dòng nào, console có `fetchUpstreamStatus failed: No runtime worktree owns ...`.

Bằng chứng:
- Stack thật (chỉ đọc, `project.list` + `repo.list` + `worktree.detectedList` từng repo): 5 trong 6 repo hiện có lỗi `WORKTREE_DETECT_FAILED` (dev server `local-mac`, `dev-local-mac-stub-in` đang `disconnected`); chỉ repo trên dev server đang kết nối trả đúng danh sách worktree (đường dẫn dạng `/private/...`). Với cấu hình này mã cũ luôn trả `[]`.
- Test mới `web-preload-task-worktree-git-status.test.ts` chạy trên mã CŨ (đã kiểm: khôi phục bản `HEAD` của `web-preload-api.ts`) ra đúng lỗi `No runtime worktree owns /private/var/folders/...` kèm `Failed to list runtime detected worktrees: Error: WORKTREE_DETECT_FAILED`; trên mã mới xanh.
- Giới hạn trung thực: tôi không tái hiện lại nguyên trạng lượt e2e cũ (project `e2e-ui-*` đã bị dọn, không có log RPC của lượt đó), nên "repo nào đã lỗi lúc đó" là suy luận từ trạng thái stack hiện tại. Mọi mắt xích khác đã kiểm.

Các giả thuyết khác đã loại bằng chứng:
- (a) `/private/var` so với `/var`: KHÔNG phải nguyên nhân của G5. Fixture `real-repo-fixture.ts` dùng `realpathSync`, repo đăng ký dạng `/private/var/...` và `git worktree list` in đúng dạng đó. Thử `git worktree add` trong `$TMPDIR` (git 2.54): git luôn in realpath (`/private/var/...`) cho cả worktree chính lẫn worktree liên kết. Tuy nhiên đây là mối nguy TIỀM ẨN có thật: `git-gateway` ghi đường dẫn worktree là `repoPath + "-" + name` nguyên văn (không realpath); nếu repo được đăng ký bằng đường dẫn có symlink (`/var/...`, `/tmp/...`, thư mục dự án là symlink, kể cả Linux), hàng của project-service mang `/var/...` còn `detectedList` mang `/private/var/...`, hai bên không khớp. Đã thêm đường dự phòng và test cho trường hợp này (xem dưới).
- (c) `SCM_EMPTY_BRANCH: head_branch is required` KHÔNG liên quan tới danh sách thay đổi và không phải do thiếu branch của worktree. Đây là lỗi hợp đồng riêng, xem "Còn lại".

## Sửa

1. `listAllRuntimeDetectedWorktrees`: gom bằng `Promise.allSettled` (`collectWorktreeListsIgnoringFailures`); repo lỗi chỉ bị bỏ qua kèm `console.warn`, không làm mất hàng của repo khác.
2. `resolveRuntimeWorktreeByPath`: nếu danh sách phát hiện từ git không nhận đường dẫn, dùng hàng đã đăng ký của project-service (`worktree.list {projectId}` theo từng project; gateway không nhận `worktree.list` thiếu `projectId`, đã đo: `PROJECT_LIST_WORKTREES_FAILED`). Đường dẫn trong UI vốn lấy từ chính hàng này nên khớp đúng, kể cả khi git in realpath khác hoặc host của repo offline (lúc đó lỗi thật hiện ra từ backend thay vì "không ai sở hữu"). So khớp đường dẫn dùng `relativePathInsideRoot` sẵn có (đa nền tảng: dấu phân cách, Windows không phân biệt hoa thường, đường dẫn runtime/SSH theo kiểu của host).
3. `runtime-worktree-path-owner.ts` tách logic thuần để test được, không thêm dòng vào `web-preload-api.ts` ngoài phần nối.

## G6: `spawn-helper` của node-pty mất bit thực thi

- Trạng thái: sau `chmod +x` của người điều phối, hai `spawn-helper` (darwin-arm64, darwin-x64) là `-rwxr-xr-x`. Kiểm bằng `node-pty` thật của agent (cùng module và prebuild): `spawn('/bin/echo')` trả `pty-ok`, thoát 0. Chưa thử qua kênh terminal của gateway (cần phiên terminal thật trên dev server của người dùng).
- Bền vững: `ensureNodePtySpawnHelperExecutable()` (agent) tự thêm bit thực thi cho `build/{Release,Debug}/spawn-helper` và `prebuilds/<platform>-<arch>/spawn-helper` lần đầu nạp `node-pty` (cả `pty.create` của terminal và `agent.spawn`); bỏ qua Windows, thiếu file, hoặc cài đặt chỉ đọc (khi đó node-pty tự báo lỗi thật). Không sửa `node_modules` thủ công. Hiệu lực sau khi build lại và khởi động lại agent.

## Còn lại (không sửa ở task này)

- **G5b [frontend + api-gateway + scm-integration]:** `hostedReview.getCreationEligibility`, `hostedReview.forBranch`, `github.prForBranch` gửi `branch`/`base` (và `repo` là id repo của Orca) trong khi kênh gateway đọc `headBranch`/`baseBranch`, `provider` và `repo` dạng slug `owner/name`; vì vậy `head_branch` luôn rỗng và scm-integration trả `SCM_EMPTY_BRANCH` (ba lần trong console, đúng với Source Control trên MỌI worktree, không riêng task). Chỉ gây nhiễu console/trạng thái "failed" của thẻ PR, không chặn danh sách thay đổi. Sửa đúng cần thiết kế: gateway phải suy ra provider + slug từ remote của repo (project-service/git-gateway), nên để task riêng; repo cục bộ không có remote thì không có PR để hỏi.
- `worktrees.listAll` (web-preload) vẫn gọi `worktree.list` không có `projectId` nên lỗi trên backend hiện tại; không nằm trong luồng Source Control.

## Kiểm

| Lệnh | Kết quả |
|---|---|
| `vitest run --config config/vitest.config.ts src/renderer/src/web/runtime-worktree-path-owner.test.ts src/renderer/src/web/web-preload-task-worktree-git-status.test.ts src/renderer/src/web/web-preload-api.test.ts` | 91 xanh. Bản `HEAD` của `web-preload-api.ts` làm 2/3 ca `task-worktree-git-status` đỏ với đúng thông báo của G5 |
| `vitest run` (agent) `node-pty-spawn-helper-permissions.test.ts`, `pty-handler`, `agent-spawner` | xanh (85) |
| `oxfmt --check`, `oxlint -c .oxlintrc.json` các file mới | sạch, không tắt `max-lines` |
| `tsc --noEmit` (frontend, agent) | không lỗi mới ở file đụng; còn lỗi CÓ SẴN (`web-preload-api.ts` onboarding/repos, `agent-spawner.test.ts`) |
| `vitest run src/renderer/src/web` | còn 8 ca đỏ CÓ SẴN, không đụng (`preload-no-change`, `web-index-html`, `main-web-bootstrap`, `web-runtime-environment`: phụ thuộc cwd/localStorage) |

## Kiểm lại trên stack thật (việc của người điều phối / agent N)

Dựng lại frontend (và build lại + khởi động lại agent), chạy lại `--real monitor`: sau "Open in Git" phải có dòng `source-control-entry` cho `src/calc.py` và `tests/test_calc.py`; console không còn `No runtime worktree owns`. `SCM_EMPTY_BRANCH` sẽ còn (G5b), nên thêm vào allowlist console của spec live hoặc chờ sửa G5b. Không cần redeploy backend.
