# TASK-FE-E2E-020: Browse folder có giới hạn, một lần gọi cho đường dẫn tuyệt đối, lỗi inline và tự nối lại

**From Solution:** SOL-FE-E2E-004 (mục 3.6) · **Resolves:** "Phát hiện phụ" của TASK-FE-E2E-015 (rớt agent, đóng im lặng)
**Priority:** P1 · **Area:** frontend
**Files:**
- frontend/src/renderer/src/components/sidebar/RemoteFileBrowser.tsx (`fetchListing` 120-148, `loadDir` 150-176, `resolvePathInput` 243-342)
- frontend/src/renderer/src/web/web-preload-api.ts (`devServer.browseDir` ~2574-2579: thêm `limit`, `query`, trả `truncated/totalEntries`)
- frontend/src/preload/api-types.ts (`devServer.browseDir`), runtime/runtime-browse (`browseRuntimeServerDirectory`, `files.browseServerDir`)
- frontend/src/renderer/src/components/sidebar/AddRepoServerStartStep.tsx (43-83)
**Depends on:** TASK-V2-030 (giới hạn + `truncated` ở backend; frontend tương thích ngược nếu backend chưa có: tự cắt phía client), TASK-FE-E2E-018 (host ngắt không thoát bước)
**Status:** `[x]` XONG (2026-10-11): mã, vitest, Playwright mock và kiểm live. Kiểm live (2026-10-11): `run_ui_flow.py --repro add-project-worktree` XANH (worktree test-02 tạo được, repo có devServerId = dev-local-mac); `--real all` 6/6 XANH (21.8 phút); backend `--strict --ai-stub` 114 PASS, 0 FAIL.

---

## Context

`resolvePathInput` liệt kê mọi thư mục cha của đường dẫn gõ vào; một thư mục ~1000 mục (như `$TMPDIR` macOS) làm khung agent vượt 32 KiB và rớt agent ~30 giây (SOL-V2-002 mục 1.3).

## Việc cần làm

- [x] `browseDir` gửi `limit` (500); đọc `truncated/totalEntries`; hiện dải "Hiển thị N/M, gõ để thu hẹp"; khi truncated, lọc dùng `query` phía máy chủ (debounce, huỷ yêu cầu cũ bằng `previewGenRef`).
- [x] `resolvePathInput`: đường dẫn tuyệt đối gọi MỘT `fetchListing(đường dẫn cuối)` trước; lỗi/không tồn tại mới quay về duyệt từng đoạn. Không liệt kê cha khi không cần.
- [x] Lỗi: giữ trình duyệt mở, hiển thị inline + nút Thử lại; với `INFRA_DEV_SERVER_NOT_CONNECTED`/`INFRA_AGENT_EXEC_FAILED` hiện "Host đang nối lại" và tự tải lại thư mục hiện tại khi host `connected` (đọc `devServers` store).
- [x] Tương thích backend cũ: nếu phản hồi không có `truncated`, frontend tự cắt theo `limit` để UI không đứng.

## Tiêu chí hoàn thành

- Duyệt tới thư mục lớn và gõ đường dẫn dưới `/private/var/folders/.../T` không làm rớt agent (kiểm trên stack thật cùng TASK-V2-030) và UI không đóng.
- Khi host ngắt, người dùng thấy thông báo và trình duyệt tự phục hồi.

## Kiểm thử

Vitest `RemoteFileBrowser` (mock `browseDir`): truncated, một lần gọi cho đường tuyệt đối, đường lỗi quay về từng đoạn, lỗi inline + retry; mock Playwright thêm ở 021.

## Rủi ro · Ước lượng

MEDIUM (`RemoteFileBrowser.tsx` đã 909 dòng: tách logic mới vào tệp theo nghiệp vụ, ví dụ `remote-browse-listing.ts`, KHÔNG thêm `max-lines` disable). 1 ngày.

## Kết quả thực thi

**Trạng thái:** mã + vitest + Playwright mock XANH. Backend của AA ĐÃ có `limit/query/directoriesOnly` và `truncated/totalEntries` trong `devServer.browseDir` (`wscompat/channels_browse_dir_limit.go`), tên trường khớp tên frontend dùng; chưa kiểm trên stack thật (chưa redeploy).

**File đổi**
- `components/sidebar/remote-browse-listing.ts` (mới, tên theo nghiệp vụ, tránh thêm dòng vào `RemoteFileBrowser.tsx` vốn đã có disable max-lines từ trước) + test: `REMOTE_BROWSE_ENTRY_LIMIT=500`, `normalizeBrowseListing` (backend cũ không trả `truncated` thì frontend tự cắt theo `limit` và đặt `truncated`/`totalEntries`), `isHostReconnectingBrowseError` (`INFRA_DEV_SERVER_NOT_CONNECTED`, `INFRA_AGENT_EXEC_FAILED`, ...), `directListingTarget`.
- `RemoteFileBrowser.tsx`: (1) `browseDir`/`files.browseServerDir` gửi `limit` (500) và `query`; CHỈ bật cho dev server và runtime ở web (SSH và runtime desktop giữ danh sách đầy đủ, không nhận tham số mới); (2) dải "Showing N of M entries. Type to narrow the list." (`remote-file-browser-truncated`); khi bị cắt, bộ lọc gọi `query` phía host sau 300 ms (huỷ yêu cầu cũ bằng thế hệ `queryGenRef`); (3) `resolvePathInput` gọi MỘT `browseDir(đường dẫn cuối)` trước, chỉ khi lỗi (không phải rớt host) mới duyệt từng đoạn để giải thích; lỗi rớt host thì dừng, không duyệt thêm; (4) lỗi nằm inline (`remote-file-browser-error`, `role="alert"`) với nút "Try again", thông báo "host is reconnecting" cho lỗi rớt agent; (5) đọc `devServers` store: host không `connected` thì hiện `remote-file-browser-host-offline`, và khi trở lại `connected` tự tải lại thư mục (hoặc gõ lại đường dẫn) đã lỗi; (6) bỏ `<p>` "No matches" bị lặp đôi sẵn có.
- `runtime/runtime-server-directory-browser.ts`: tham số `options {limit,query}` tuỳ chọn (mặc định không gửi gì, test cũ giữ nguyên); `preload/api-types.ts` + `web-preload-api.ts` `devServer.browseDir` nhận/chuyển `limit`, `query`, trả `truncated`, `totalEntries`.
- Test: `RemoteFileBrowser.dev-server.test.tsx` (9 ca: cap, đường tuyệt đối chỉ gọi 1 lần `/private/var/folders/xy/T/repo` không liệt kê cha, quay về từng đoạn khi lỗi, notice cắt từ backend và từ backend cũ, lọc phía host, lỗi inline + Try again, tự tải lại khi host về, thông báo host offline mà không đóng); `RemoteFileBrowser.paste.test.tsx` (SSH) vẫn xanh; Playwright (d) host rớt khi đang Browse.

**Lệnh đã chạy và kết quả thật (2026-10-10)**
- `NODE_OPTIONS=--no-experimental-webstorage pnpm exec vitest run --config config/vitest.config.ts <các tệp đã đổi/mới + components/sidebar + ProjectDevServerSection>` (trong `frontend/`): 189 tệp, 1647 xanh, 4 đỏ. 4 đỏ là `Sidebar.test.tsx` (2) và `WorktreeCardMeta.interaction.test.tsx` (2), ĐỎ SẴN trên HEAD (đối chiếu bằng bản sao sạch của HEAD).
- Toàn bộ vitest frontend: HEAD 28 tệp đỏ / 85 test; sau thay đổi 25 tệp đỏ / 79 test; tập tệp đỏ sau thay đổi là tập con của tập đỏ trên HEAD (không test nào xanh ở HEAD mà đỏ bây giờ). Một phần chênh lệch do bản sao HEAD thiếu cây `backend-go` (`request-rpc-contract.test.ts`) và do tôi chép test mới vào bản sao để tái hiện.
- `./node_modules/.bin/tsc --noEmit -p frontend/tsconfig.json --composite false`: không có lỗi mới ở tệp tôi đụng; còn lại là lỗi CÓ SẴN (`ui.ts` `previousViewBefore*`, `use-onboarding-flow.ts` `StepId`, `web-preload-api.ts` 740/1506/2567 `onboarding`/`scanRemote`/`arch`, `CreateProjectDialog.test.tsx` `MockAppState`, `web-preload-api.test.ts` ~4281).
- `./node_modules/.bin/oxlint -c .oxlintrc.json <mọi tệp ts/tsx đã đổi/mới>`: sạch (exit 0), kể cả 3 tệp CÓ SẴN vượt max-lines mà tôi đã tách (xem 017/018); `./node_modules/.bin/oxfmt --list-different` trên cùng tập: sạch sau `--write`. Không thêm `max-lines` disable nào (`git diff` không có dòng `+...-disable`).
- `npx playwright test -c tests/configs/playwright.web.config.ts --project=request-web`: 67 xanh, 1 skipped (có sẵn); gồm 9 test mới ở `add-project-host-binding.web.e2e.ts` (a-f) và `repo-host-rebind.web.e2e.ts` (g-i). Đã xoá `test-results` ở gốc sau đó.

**Blast radius (codegraph):** `RemoteFileBrowser`: callers `CloneStep`, `RemoteStep` (SSH), `AddRepoServerPathStartStep`, `CreateProjectParentBrowser`; `browseRuntimeServerDirectory`: `RemoteFileBrowser`, `useCreateProjectDefaults` (gọi `~`, không đổi). SSH (`targetId`) KHÔNG nhận `limit/query` và không bị cắt (tránh làm hỏng desktop).

**Chưa kiểm:** gõ đường dẫn dưới `$TMPDIR` macOS trên stack thật và xác nhận `orca-go-infra-fleet` không có `dev server disconnected` (cần redeploy infra-fleet/gateway/agent của AA + frontend); thư mục >2000 mục (gateway trần `browseDirMaxLimit`).
