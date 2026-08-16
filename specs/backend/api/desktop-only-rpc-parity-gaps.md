# Desktop-vs-Server RPC Parity — Gap Audit (2026-08-16)

## Vì sao có tài liệu này

Trong lúc xác minh live trên `172.20.2.39` sau khi fix bug crash user-process
(Postgres lowercase-alias — xem git log cùng ngày), người dùng thật lần lượt
gặp 5 lỗi console `Unknown method: X` khác nhau khi tải trang: `preflight.check`
(đã sẵn có, chỉ do process cũ crash), `rateLimits.get`, `onboarding.get`,
`crashReports.getLatestPending`, `telemetry.track`, `claudeUsage.*`, và cuối
cùng `starNag.onboardingCompleted`. Mỗi lần vá xong lại lộ ra lỗi tiếp theo —
vì trước đó user-process luôn crash ngay từ đầu (bug alias) nên các đường gọi
RPC này **chưa bao giờ thực sự chạy tới** ở server mode, kể cả khi method đã
tồn tại từ lâu ở phía desktop.

Thay vì tiếp tục vá từng lỗi theo báo cáo, đã rà soát **toàn bộ** danh sách
method để biết chính xác quy mô còn lại, tránh vá nhầm chỗ hoặc bỏ sót.

## Phương pháp

So sánh mọi `name: '<method>'` trong `desktop/src/main/runtime/rpc/methods/*.ts`
với `backend/src/main/runtime/rpc/methods/*.ts` (script Python đơn giản, regex
`name:\s*'([a-zA-Z0-9_.]+)'`, loại trừ file test). Đây là so sánh **tên
method**, không kiểm tra chữ ký/param — xem
[`specs/frontend/api/rpc-catalog.md`](../../frontend/api/rpc-catalog.md) cho
đối chiếu ở tầng frontend↔backend (namespace mà frontend web thực sự gọi tới,
cách generate/giới hạn khác — đọc `README.md` cùng thư mục đó).

**Trạng thái tại thời điểm viết** (sau khi đã port `rateLimits`/`onboarding`/
`crashReports`/`telemetry`/`claudeUsage`/`codexUsage`/`openCodeUsage` cùng
ngày — xem commit log `fix/pty-session-expired-on-pane-remount`):

- Desktop: 722 method. Backend: 578 method.
- **Còn thiếu ở backend: 147 method**, chia 31 nhóm theo namespace.

## Đã fix trong phiên này (không còn trong danh sách 147)

| Namespace | Method | Ghi chú |
|---|---|---|
| `rateLimits.*` | 11 | Port đầy đủ, nhưng **chưa có dữ liệu thật** — `ClaudeAccountService`/`CodexAccountService`/`RateLimitService` chưa từng được construct ở server mode (xem mục "Cần quyết định sản phẩm" bên dưới). Hiện trả state rỗng hợp lệ, không crash. |
| `onboarding.*` | 10 | Implement thật đầy đủ, kể cả `detectGhosttyConfig`/`openGhAuthTerminal` (proxy đúng tới Dev Server đang kết nối). |
| `crashReports.*` | 7 | Thiết kế lại cho server headless — không có native crash dump/clipboard, chỉ lưu crash được báo cáo tường minh (React error boundary + `process.on('uncaughtException')`). |
| `telemetry.*` | 4 | Wire RPC xong, nhưng `initTelemetry()`/`initCohortClassifier()` **chưa được gọi** ở `server-bootstrap.ts` — event bị `track()` tự no-op drop cho tới khi ai quyết định server mode có gửi telemetry không, gửi dưới identity nào. |
| `claudeUsage.*` / `codexUsage.*` | 16 | Port đầy đủ, có dữ liệu thật (Postgres, ADR-021). |
| `openCodeUsage.*` | 8 | Port đầy đủ + migration Postgres mới (`0025_opencode_usage_state_blob.ts`). |

| `starNag.*` | 12 | **Fixed 2026-08-16** — port đầy đủ (không chỉ rút gọn như đề xuất ban đầu; `checkOrcaStarred`/`starOrca`/`track()` đã có sẵn nên port full rẻ hơn dự kiến). 1 điều chỉnh thật: bỏ gate `if (!BrowserWindow)` trong `broadcastShow()` — server mode không bao giờ tạo `BrowserWindow` nên gate đó sẽ tắt hẳn tính năng. |
| `cache.getGitHub`/`.setGitHub`, `sparsePresets.*`, `feedback.submit`, `memory.getSnapshot`, `platform.get`, `diagnostics.*` (bundle, 6), `notifications.*` (5 còn thiếu) | 20 | **Fixed 2026-08-16** — toàn bộ nhóm B đã port, xem commit `a04a9ec59`. `diagnostics.*` bundle: RPC thật nhưng `initObservability()` chưa gọi ở `server-bootstrap.ts` (giống tình trạng `telemetry.*` — bundle rỗng cho tới khi wire). `notifications.*` 5 method còn lại + phần `playSound` của desktop: trả "not applicable"/lý do rõ ràng thay vì throw, không có hành vi thật (đúng theo bản chất OS-native của chúng). |

## 147 method còn thiếu — phân loại theo khuyến nghị

### A. Chỉ có ý nghĩa ở desktop — khuyến nghị KHÔNG port (99 method)

Đây là các tính năng gắn với "máy đang chạy Orca là máy của chính người
dùng" — giả định sai hoàn toàn ở server mode (máy chạy Orca là 1 server dùng
chung nhiều người, không phải máy cá nhân).

| Namespace | # | Lý do |
|---|---|---|
| ~~`starNag.*`~~ | ~~12~~ | **Fixed** — xem bảng "Đã fix trong phiên này" ở trên. |
| `ephemeralVm.*` | 9 | Điều tra sâu 2026-08-16 (xem `specs/backend/api/ephemeral-vm-server-mode-design.md`): KHÔNG port ngay được — cần Dev Server tự làm SSH client ra máy thứ 3 (agent chưa có) + bảng Postgres runtime state (chưa có). Đã đổi từ "vô nghĩa" sang "port được nhưng thiếu hạ tầng" — 3 phương án trong tài liệu thiết kế. |
| `mobile.*` | 9 | Pairing app mobile qua QR/LAN — cần biết địa chỉ mạng LAN của MÁY DESKTOP, vô nghĩa với server từ xa. |
| `app.*` | 6 | Keyboard input source id, dock badge count, floating-markdown file picker — API window/OS-level. |
| ~~`cli.*`~~ | ~~6~~ | **Fixed 2026-08-16** — port đầy đủ sang agent-relay, xem mục "Chuyển sang mô hình agent-proxy" bên dưới. Sửa lại nhận định sai ban đầu: đây là cài lệnh `orca` (của chính Orca), KHÔNG PHẢI CLI claude/codex. |
| `updater.*` | 6 | Electron auto-updater — server deploy qua Docker, không áp dụng. |
| `pet.*` | 4 | Tính năng "pet" cosmetic gắn với cửa sổ desktop. |
| `ui.*` | 4 | Đọc/ghi clipboard native (Electron `clipboard` module) — trình duyệt có `navigator.clipboard` riêng, không cần qua backend. |
| `computerUsePermissions.*` | 3 | Quyền Accessibility của macOS (cho tính năng Computer Use) — API hệ điều hành cục bộ. |
| `developerPermissions.*` | 3 | Prompt xin quyền cấp OS (ví dụ Full Disk Access) — chỉ có trên máy desktop. |
| `e2e.*` | 1 | Config test E2E, chỉ dùng khi dev/CI desktop. |
| `export.htmlToPdf` | 1 | Cần `BrowserWindow.webContents.printToPDF` — không có ở headless Node (có thể thay bằng headless-browser riêng nếu thật sự cần, effort riêng). |
| ~~`agentTrust.markTrusted`~~ | ~~1~~ | **Fixed 2026-08-16** — nhận định ban đầu sai: backend đã sẵn có SSH-filesystem-provider (`remote-agent-trust-presets.ts`), port trực tiếp không cần agent-relay. |
| `localhostWorktreeLabels.register` | 1 | **Xác nhận lại 2026-08-16, đúng như phân loại ban đầu, KHÔNG port được**: domain `*.localhost` theo RFC 6761 luôn resolve về máy trình duyệt của chính client, không bao giờ về server từ xa — port sẽ tạo URL không bao giờ truy cập được (không phải thiếu tính năng, mà là thiết kế không thể mang sang server mode với cơ chế URL này). |
| ~~`shell.pickDirectory/pickAttachment/pickImage/pickRepoIconImage/pickAudio/pathExists/copyFile`~~ | ~~7~~ | **Fixed 2026-08-16 theo hướng khác** — không port RPC `shell.pick*` (vẫn không dùng dialog OS được), mà xây UI file-picker mới trong app (`DevServerFilePickerDialog`) browse trực tiếp qua Dev Server Agent. 4/6 UI trigger point đã rewire; `shell.*` vẫn còn trong danh sách "im lặng lỗi" vì 2 điểm chưa rewire (chat attachment, rename-dialog). |
| `shell.openPath/openInFileManager/openInExternalEditor/openUrl/openFilePath/openFileUri` | 6 | Vẫn cần GUI của máy đang mở UI — không đổi phân loại. |

### B. Đáng cân nhắc port — rủi ro thấp, giá trị rõ (17 method)

Backend đã có phần lớn hạ tầng nền hoặc method độc lập, không phụ thuộc thứ
gì đặc thù desktop.

| Namespace | # | Ghi chú |
|---|---|---|
| `cache.getGitHub` / `cache.setGitHub` | 2 | Cache generic, không phụ thuộc Electron — nhìn qua có vẻ port gần như copy-paste. |
| `sparsePresets.*` | 4 | Preset sparse-checkout cho git worktree — thuần logic + Store, giống pattern `onboarding.*` vừa port. |
| `feedback.submit` | 1 | Namespace RIÊNG, KHÁC với `crashReports.submit` đã port hôm nay (không trùng, không tự động có) — desktop's `ipc/feedback.ts`, cần kiểm tra còn phụ thuộc gì Electron không. |
| `memory.getSnapshot` | 1 | Rất có thể chỉ là `process.memoryUsage()` — nếu đúng thì port gần như tức thời. |
| `platform.get` | 1 | Thông tin OS/arch — với server nghĩa là thông tin máy server, vẫn có thể hữu ích cho debug, cần xác nhận ý nghĩa hiển thị ở UI có đổi không. |
| `diagnostics.*` (bundle) | 6 | Riêng biệt với `diagnostics.ts` đã có sẵn (0 gap) — nằm ở `desktop/.../diagnostics-crash-bundle.ts`, một tính năng "thu thập bundle chẩn đoán để gửi hỗ trợ" khác với `crashReports.*` đã port. Có thể trùng lặp một phần với crash-report-store.ts mới viết — cần review trước khi port để tránh 2 cơ chế song song. |
| `notifications.*` (5 còn thiếu) | 5 | `dispatch/getPermissionStatus/openSystemSettings/playSound/probeDelivery` — phần notification KHÔNG phải Web Push (đã có `NOTIFICATION_METHODS` cho web push). Các method này là OS-native notification, cần xem lại có ý nghĩa gì ở server hay chỉ là phần còn sót của desktop mode. |

### C. Cần quyết định sản phẩm trước khi port (31 method)

Về mặt kỹ thuật port được, nhưng đụng câu hỏi kiến trúc: server đa người
dùng có nên có khái niệm này theo đúng cách desktop hiểu nó không, hay cần
thiết kế lại?

| Namespace | # | Câu hỏi cần quyết định |
|---|---|---|
| `orcaProfiles.*` | 16 | Hệ thống profile/org **cloud** của Orca (auth, mời/xoá member org, chuyển project giữa org) — nghe như tính năng thật sự quan trọng, KHÔNG chỉ là desktop-local. Cần xác nhận: server mode có nên tự làm client của chính cloud API này, hay đã có cơ chế khác (`profile.*`/`ProfileService` hiện tại) đảm nhiệm vai trò tương đương rồi? **Chưa điều tra thêm** — vẫn treo. |
| `claudeAccounts.*` / `codexAccounts.*` | 11 | Điều tra sâu 2026-08-16: KHÔNG đơn giản như `cli.*`. Gắn với ~3500 dòng logic đồng bộ credential real-time (`ClaudeRuntimeAuthService`/`CodexRuntimeHomeService`, theo dõi PTY đang chạy để tránh race khi CLI tự refresh token) — port kiểu relay 1-call sẽ tái tạo đúng race condition code gốc tránh. Cũng phát hiện `add`/`reauthenticate` không dùng PTY như tưởng, chỉ `spawn()` đơn giản, bỏ qua URL OAuth CLI in ra — cần UI streaming PTY riêng (giống `WebModeCliAuthSection.tsx` đã có cho GitHub) mới dùng được ở server mode headless. **Cần 1 ADR riêng**, không phải quyết định "route sang aiProvider.*" đơn giản như đoán ban đầu. |
| ~~`agentStatus.*`~~ | ~~5~~ | **Fixed 2026-08-16** — backend đã có `agentHookServer` byte-identical với desktop, port trực tiếp không cần agent-relay. Gap còn lại: hook event từ Dev Server Agent (WS) chưa wire vào `agentHookServer` (không thuộc phạm vi 5 method RPC này). |
| `remoteWorkspace.*` | 6 | **Chưa điều tra** — vẫn treo. |
| ~~`workspaceCleanup.*`~~ | ~~4~~ | **Fixed 3/4 (2026-08-16)** — `hasKillableLocalProcesses` đã connectionId-aware sẵn từ desktop, dùng lại `DevServerPtyProvider.listProcesses()` đã relay agent sẵn có. `.scan` cần port thêm 4 file phụ trợ (subsystem riêng), còn treo. |
| `workspaceSpace.*` | 2 | Điều tra 2026-08-16: đúng là hợp lý port (không phải máy chạy Orca, mà dung lượng worktree trên Dev Server — desktop's `workspace-space-analysis.ts` đã hỗ trợ cả local lẫn SSH-remote). Nhưng cần port cả subsystem 960 dòng, không phải 1-2 method đơn lẻ — còn treo, để riêng thành việc lớn hơn. |
| `minimaxCredentials.*` / `grokAccounts.getStatus` | 4 | **Chưa điều tra** — vẫn treo. |

## Trạng thái (cập nhật 2026-08-16)

1. ~~starNag.onboardingCompleted đang lỗi live~~ — **Fixed**, port đầy đủ 12
   method (commit `a04a9ec59`).
2. ~~Nhóm B (17-20 method)~~ — **Fixed**, toàn bộ đã port (commit `a04a9ec59`).
3. **Nhóm C (31 method)** — vẫn KHÔNG nên port trước khi có quyết định rõ
   ràng, đặc biệt `orcaProfiles.*` (16 method, nghe như tính năng lớn) và
   `claudeAccounts.*`/`codexAccounts.*` (11 method, nhiều khả năng trùng vai
   trò với `aiProvider.*` đã hoạt động).
4. **Nhóm A (99 method)** — **Fixed theo hướng khác**: không port, thay vào
   đó thêm 1 listener `unhandledrejection` toàn cục ở frontend
   (`desktop-only-rpc-error-suppressor.ts`, commit `b647b5119`) im lặng
   console error cho đúng các namespace đã liệt kê ở đây khi backend trả
   `method_not_found` — không viết 99 method backend không có tác dụng thật.

**Còn treo, cần quyết định sản phẩm**: toàn bộ Nhóm C, cộng thêm 2 việc phát
sinh trong lúc port Nhóm B — `initObservability()` (diagnostics bundle) và
`initTelemetry()`/`initCohortClassifier()` (telemetry) đều CHƯA được gọi ở
`server-bootstrap.ts`, nên 2 tính năng này hoạt động ở tầng RPC nhưng chưa
có dữ liệu thật cho tới khi ai đó quyết định wire chúng (giống gap
`rateLimits.*`/`ClaudeAccountService`).

## Đợt "chuyển sang mô hình agent-proxy" (2026-08-16, sau khi rà lại Nhóm A/C)

Theo yêu cầu: những tính năng mà "máy chạy code" mới có ý nghĩa (không phải
backend container dùng chung) nên chuyển thành backend-proxy-tới-agent thay
vì port thẳng hoặc bỏ qua. Rà lại cả Nhóm A lẫn Nhóm C với góc nhìn này:

**Port thành công theo mô hình mới** (commit `7df884e91`):
- `cli.*` (6) — agent-relay thật, agent tự thực thi trên máy nó đứng.
- `agentTrust.markTrusted` (1) — hoá ra dùng SSH-filesystem-provider backend
  đã có sẵn, không cần agent-relay.
- `agentStatus.*` (5), `workspaceCleanup.*` (3/4) — hoá ra backend đã có sẵn
  hạ tầng (agentHookServer, DevServerPtyProvider.listProcesses relay), chỉ
  thiếu RPC layer — port trực tiếp, không cần agent-relay mới.
- `shell.pickDirectory/pickAttachment/pickImage/pickRepoIconImage/pickAudio/pathExists/copyFile`
  (7) — không port RPC `shell.pick*` gốc, mà xây UI file-picker mới trong
  app + relay `devServer.pathExists/readFile/copyFile` tới agent.

**Điều tra sâu, quyết định KHÔNG port ngay (không ép code sai)**:
- `ephemeralVm.*` (9) — thiếu SSH-client outbound ở agent + bảng Postgres.
  Tài liệu thiết kế riêng: `specs/backend/api/ephemeral-vm-server-mode-design.md`.
- `claudeAccounts.*`/`codexAccounts.*` (11) — sâu hơn nhận định ban đầu rất
  nhiều (~3500 dòng logic đồng bộ credential), cần ADR riêng.
- `workspaceSpace.*` (2), `workspaceCleanup.scan` (1) — hợp lý về mặt kiến
  trúc nhưng cần port cả subsystem lớn (960+ dòng), không phải 1 method.

**Xác nhận lại đúng, giữ nguyên phân loại KHÔNG port**:
- `localhostWorktreeLabels.register` — lý do kỹ thuật chắc chắn (RFC 6761).

**Chưa điều tra, vẫn treo**: `orcaProfiles.*` (16), `remoteWorkspace.*` (6),
`minimaxCredentials.*`/`grokAccounts.getStatus` (4).
