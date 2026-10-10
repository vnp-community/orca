# TASK-FE-E2E-018: Chọn host trong hộp thoại: chọn sẵn host duy nhất, bắt chọn khi nhiều, chặn khi không có, giữ lựa chọn khi host ngắt

**From Solution:** SOL-FE-E2E-004 (mục 3.3) · **Resolves:** hành vi "rơi về local" và "đóng im lặng"
**Priority:** P0 · **Area:** frontend
**Files:**
- frontend/src/renderer/src/components/sidebar/use-add-repo-host-selection.ts (62-66 `defaultHostId`, 74-83 chọn host, 87-102 effect mở hộp thoại, 105-129 `handleSelectAddProjectHost`)
- frontend/src/renderer/src/components/sidebar/use-sidebar-host-scope-options.ts (42-45)
- frontend/src/renderer/src/components/sidebar/AddRepoDialog.tsx (66-74, 301-360), AddRepoServerStartStep.tsx (43, 88), AddRepoHostSelectorSlot.tsx (20-23), use-add-repo-host-change-reset.ts
**Depends on:** TASK-FE-E2E-016
**Status:** `[x]` XONG (2026-10-11): mã, vitest, Playwright mock và kiểm live. Kiểm live (2026-10-11): `run_ui_flow.py --repro add-project-worktree` XANH (worktree test-02 tạo được, repo có devServerId = dev-local-mac); `--real all` 6/6 XANH (21.8 phút); backend `--strict --ai-stub` 114 PASS, 0 FAIL.

---

## Context

Web chỉ liệt kê dev server `available`; mặc định lấy `hostOptions[0]` (thứ tự tuỳ ý) và khi danh sách rỗng thì rơi về `local` (`selectedHost?.id ?? defaultHostId`). Host ngắt giữa chừng làm `selectedHostId` đổi, `useAddRepoHostChangeReset` xoá trạng thái, bước Browse thoát im lặng (suy từ mã, chưa kiểm).

## Việc cần làm

- [x] Viết TEST TÁI HIỆN trước: host đang chọn biến mất khỏi `devServers` khi đang ở bước Browse; test đỏ với mã hiện tại, xác nhận hay bác bỏ giả thuyết trong SOL mục 1.3, ghi kết quả vào "Kết quả thực thi".
- [x] 0 host (web): trạng thái chặn, KHÔNG rơi về `local`; panel + nút "Mở Dev Servers" (`openSettingsTarget({pane:'servers'})`); Browse/Add vô hiệu.
- [x] 1 host: chọn sẵn và hiện tên host.
- [x] ≥2 host: không chọn sẵn; placeholder "Chọn host"; Browse/Add vô hiệu cho tới khi chọn; nhớ host gần nhất bằng `localStorage` (try/catch, chỉ áp nếu còn trong danh sách).
- [x] Host đã chọn ngắt: giữ lựa chọn với trạng thái `unavailable`, thông báo inline "host đang nối lại", vô hiệu Add, tự bật khi `connected`; không thoát bước Browse; không gọi `useAddRepoHostChangeReset` cho trường hợp này.
- [x] Desktop (`!isWeb`) giữ nguyên.

## Tiêu chí hoàn thành

- Ba tình huống 0/1/≥2 host đúng như trên; host ngắt giữa Browse không đóng hộp thoại.
- Không còn đường nào ở web chọn `local` làm host thêm repo.

## Kiểm thử

Vitest hook (`use-add-repo-host-selection.test.ts` mới hoặc mở rộng), `AddRepoDialogStepContent.test.tsx`, `AddRepoStartSteps.test.tsx`; Playwright mock ở 021.

## Rủi ro · Ước lượng

MEDIUM (hook dùng cho cả desktop; giữ nhánh `!isWeb`). 1 ngày.

## Kết quả thực thi

**Trạng thái:** mã + vitest + Playwright mock XANH; chưa kiểm live.

**Test tái hiện chạy TRƯỚC khi sửa (giả thuyết SOL mục 1.3 được XÁC NHẬN):** chép `use-add-repo-host-selection.test.ts` (khối web mới) vào bản sao sạch của HEAD và chạy: cả 5 ca web ĐỎ trên HEAD. Chi tiết: khi dev server đã chọn rơi khỏi danh sách (`hostOptions` rỗng, `devServers` còn nó với `disconnected`), HEAD trả `selectedHostId = 'local'` (`expected 'local' to be 'devServer:ds-1'`), nên `selectedDevServerId` thành `null` và điều kiện `browsing && (runtimeEnvironmentId || devServerId)` ở `AddRepoServerStartStep` sai, trình duyệt thư mục thoát im lặng; với 0 host HEAD cũng trả `local` (`expected 'local' to be null`), và với 2 host HEAD tự chọn host đầu (`expected 'devServer:ds-1' to be null`). Sau khi sửa cả 13 ca của tệp xanh (8 desktop có sẵn + 5 web).

**File đổi**
- `components/sidebar/add-repo-web-host-gate.ts` (mới) + `.test.ts`: `AddRepoHostGate` (`ready | no-host | choose-host | host-unavailable{reason: connecting|disconnected|error|removed}`), `pickAutoSelectedWebHostId`, `deriveAddRepoHostGate`, nhớ host gần nhất bằng `localStorage` (try/catch, chỉ áp nếu host còn kết nối).
- `components/sidebar/use-add-repo-web-host-gate.ts` (mới): trạng thái chọn host của web (1 host: chọn sẵn và GHIM để host rớt thành `host-unavailable`, không tự đổi; ≥2: không chọn sẵn; 0: chặn); thêm host đã rớt vào danh sách bộ chọn dạng mờ.
- `use-add-repo-host-selection.ts`: nhánh `isWeb` dùng hook trên; `selectedHostId` có thể `null` (web chưa chọn), KHÔNG bao giờ rơi về `local`; trả `hostGate`; nhánh desktop giữ nguyên (8 test cũ không đổi).
- `AddRepoHostGateNotice.tsx` (mới, `data-testid="add-repo-host-gate"`, `data-gate-status`): panel inline (token có sẵn `border-border bg-muted text-muted-foreground`), nút "Mở Dev Servers" (đóng hộp thoại, `openSettingsTarget({pane:'dev-servers'})`).
- `AddRepoHostSelector.tsx`: `selectedHostId` nullable, placeholder "Choose a host". `AddRepoDialogStepContent.tsx` (`hostGateBlocking`), `AddRepoServerStartStep.tsx` (`hostBlocked`): Browse/Add vô hiệu khi chặn; ở bước Browse (RemoteFileBrowser) host rớt không thoát bước.
- `AddRepoDialog.tsx`: ghép notice vào `hostSelector`; tách `use-add-repo-dialog-reset.ts` (reset khi đóng/đổi host) vì tệp đã vượt max-lines sẵn (418 > 400), nay 401 dòng thô và oxlint sạch.
- `lib/settings-navigation-types.ts`: thêm `'dev-servers'` vào `SettingsNavTarget`. LỆCH SOLUTION: solution nói dùng `pane:'servers'` ("đã dùng ở AddRepoHostSelectorSlot"), nhưng `servers` là pane "Remote Orca Servers"; pane Dev Servers có id `dev-servers` (thấy qua ảnh Playwright). Đã sửa cả `AddRepoHostSelectorSlot.handleAddRemoteInWeb` sang `dev-servers`.

**Test:** `use-add-repo-host-selection.test.ts` (5 ca web), `add-repo-web-host-gate.test.ts` (10), `AddRepoHostGateNotice.test.tsx` (5), Playwright `add-project-host-binding.web.e2e.ts`: (a) 1 host chọn sẵn + `repo.add`/`project.create`/`projectGroup.scanNested` mang host; (b) 2 host: "Choose a host", Browse bị khoá tới khi chọn; (c) 0 host: panel + nút mở Dev Servers, không có `repo.add`; (d) host rớt giữa Browse: thông báo "host is not connected", vẫn ở bước Browse.

**Lệnh đã chạy và kết quả thật (2026-10-10)**
- `NODE_OPTIONS=--no-experimental-webstorage pnpm exec vitest run --config config/vitest.config.ts <các tệp đã đổi/mới + components/sidebar + ProjectDevServerSection>` (trong `frontend/`): 189 tệp, 1647 xanh, 4 đỏ. 4 đỏ là `Sidebar.test.tsx` (2) và `WorktreeCardMeta.interaction.test.tsx` (2), ĐỎ SẴN trên HEAD (đối chiếu bằng bản sao sạch của HEAD).
- Toàn bộ vitest frontend: HEAD 28 tệp đỏ / 85 test; sau thay đổi 25 tệp đỏ / 79 test; tập tệp đỏ sau thay đổi là tập con của tập đỏ trên HEAD (không test nào xanh ở HEAD mà đỏ bây giờ). Một phần chênh lệch do bản sao HEAD thiếu cây `backend-go` (`request-rpc-contract.test.ts`) và do tôi chép test mới vào bản sao để tái hiện.
- `./node_modules/.bin/tsc --noEmit -p frontend/tsconfig.json --composite false`: không có lỗi mới ở tệp tôi đụng; còn lại là lỗi CÓ SẴN (`ui.ts` `previousViewBefore*`, `use-onboarding-flow.ts` `StepId`, `web-preload-api.ts` 740/1506/2567 `onboarding`/`scanRemote`/`arch`, `CreateProjectDialog.test.tsx` `MockAppState`, `web-preload-api.test.ts` ~4281).
- `./node_modules/.bin/oxlint -c .oxlintrc.json <mọi tệp ts/tsx đã đổi/mới>`: sạch (exit 0), kể cả 3 tệp CÓ SẴN vượt max-lines mà tôi đã tách (xem 017/018); `./node_modules/.bin/oxfmt --list-different` trên cùng tập: sạch sau `--write`. Không thêm `max-lines` disable nào (`git diff` không có dòng `+...-disable`).
- `npx playwright test -c tests/configs/playwright.web.config.ts --project=request-web`: 67 xanh, 1 skipped (có sẵn); gồm 9 test mới ở `add-project-host-binding.web.e2e.ts` (a-f) và `repo-host-rebind.web.e2e.ts` (g-i). Đã xoá `test-results` ở gốc sau đó.

**Blast radius (codegraph):** `useAddRepoHostSelection`: chỉ `AddRepoDialog` (+ `AddRepoHostSelectorSlot` dùng kiểu trả về); `AddRepoHostSelector`: chỉ slot + test. Nhánh `!isWeb` không đổi. Electron: không có web gate (`hostGate` luôn `ready`).

**Chưa kiểm:** thao tác thật trên stack (host rớt thật ~30 s sau khi browse thư mục lớn); Electron bằng tay.
