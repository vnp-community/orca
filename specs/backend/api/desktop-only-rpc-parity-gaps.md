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

**Đang báo lỗi live, chưa fix**: `starNag.onboardingCompleted` (và 11 method
`starNag.*` khác cùng nhóm) — xem phân loại bên dưới, đề xuất port rút gọn
(bỏ phần `BrowserWindow` native) chứ không port 1:1.

## 147 method còn thiếu — phân loại theo khuyến nghị

### A. Chỉ có ý nghĩa ở desktop — khuyến nghị KHÔNG port (99 method)

Đây là các tính năng gắn với "máy đang chạy Orca là máy của chính người
dùng" — giả định sai hoàn toàn ở server mode (máy chạy Orca là 1 server dùng
chung nhiều người, không phải máy cá nhân).

| Namespace | # | Lý do |
|---|---|---|
| `shell.*` | 13 | Native file/folder dialog, mở file trong file manager/external editor — cần OS window của máy người dùng. |
| `starNag.*` | 12 | Nhắc star repo GitHub qua overlay `BrowserWindow` gắn với desktop app. **Ngoại lệ**: phần lõi (`checkOrcaStarred` + `track()`) không phụ thuộc Electron, có thể port rút gọn nếu muốn giữ tính năng — xem mục C. |
| `ephemeralVm.*` | 9 | Quản lý VM tạm trên máy local (attach/suspend/resume workspace, doctor/cleanup) — khái niệm máy ảo cục bộ. |
| `mobile.*` | 9 | Pairing app mobile qua QR/LAN — cần biết địa chỉ mạng LAN của MÁY DESKTOP, vô nghĩa với server từ xa. |
| `app.*` | 6 | Keyboard input source id, dock badge count, floating-markdown file picker — API window/OS-level. |
| `cli.*` | 6 | Cài/gỡ CLI tool (claude/codex...) TRÊN MÁY đang chạy Orca — server không phải máy làm việc của user. |
| `updater.*` | 6 | Electron auto-updater — server deploy qua Docker, không áp dụng. |
| `pet.*` | 4 | Tính năng "pet" cosmetic gắn với cửa sổ desktop. |
| `ui.*` | 4 | Đọc/ghi clipboard native (Electron `clipboard` module) — trình duyệt có `navigator.clipboard` riêng, không cần qua backend. |
| `computerUsePermissions.*` | 3 | Quyền Accessibility của macOS (cho tính năng Computer Use) — API hệ điều hành cục bộ. |
| `developerPermissions.*` | 3 | Prompt xin quyền cấp OS (ví dụ Full Disk Access) — chỉ có trên máy desktop. |
| `e2e.*` | 1 | Config test E2E, chỉ dùng khi dev/CI desktop. |
| `export.htmlToPdf` | 1 | Cần `BrowserWindow.webContents.printToPDF` — không có ở headless Node (có thể thay bằng headless-browser riêng nếu thật sự cần, effort riêng). |
| `agentTrust.markTrusted` | 1 | Quyết định "trust" 1 binary AI agent cục bộ trên máy — khái niệm máy-cụ-thể. |
| `localhostWorktreeLabels.register` | 1 | Gắn nhãn cho port localhost — chỉ có ý nghĩa khi worktree chạy trên chính máy đang mở UI. |

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
| `orcaProfiles.*` | 16 | Hệ thống profile/org **cloud** của Orca (auth, mời/xoá member org, chuyển project giữa org) — nghe như tính năng thật sự quan trọng, KHÔNG chỉ là desktop-local. Cần xác nhận: server mode có nên tự làm client của chính cloud API này, hay đã có cơ chế khác (`profile.*`/`ProfileService` hiện tại) đảm nhiệm vai trò tương đương rồi? |
| `claudeAccounts.*` / `codexAccounts.*` | 11 | CHÍNH LÀ gap `rateLimits.*` đã nói ở trên — quản lý account CLI Claude/Codex đăng nhập TRÊN MÁY. Với server multi-user + remote Dev Server, "máy" nào mới đúng? Tương đương đã có sẵn và đang hoạt động là `AIProviderService`/`aiProvider.*` (đăng ký key theo tổ chức, không phải theo máy) — khả năng cao đây là 11 method KHÔNG nên port nguyên bản, mà nên hướng người dùng sang `aiProvider.*`. |
| `agentStatus.*` | 5 | Theo dõi trạng thái agent đang chạy — có thể đã có phần tương đương qua `orchestration.*`/terminal agent-status hiện tại (session trước đã cố tình để `getAgentStatusOrchestrationContextForHandle` trả `undefined` ở server mode — xem ghi chú trong `orca-runtime-terminal-agent-status.ts`). Cần xác nhận có trùng lặp không trước khi port thêm. |
| `remoteWorkspace.*` | 6 | "Remote workspace" + `listConnectedClients` — nghe giống khái niệm liên quan `mobile.*`/pairing, cần đọc kỹ trước khi quyết định có áp dụng được cho multi-user server không. |
| `workspaceCleanup.*` | 4 | `hasKillableLocalProcesses` gợi ý đang thao tác process TRÊN MÁY chạy Orca — nếu đúng, thuộc nhóm A (desktop-only); cần đọc code xác nhận trước khi xếp loại chắc chắn. |
| `workspaceSpace.*` | 2 | Phân tích dung lượng đĩa — của máy nào? Nếu là máy chạy Orca (server dùng chung) thì thông tin này không giúp ích cho user cá nhân; nếu ý là dung lượng trong worktree/Dev Server thì có thể port được. Cần đọc code. |
| `minimaxCredentials.*` / `grokAccounts.getStatus` | 4 | Quản lý credential cho AI provider ngoài luồng chính — có thể nên gộp vào `aiProvider.*`/`AIProviderService` đã có, giống nhận định ở `claudeAccounts`/`codexAccounts`. |
| `starNag.*` (phần lõi) | 3 | Nếu quyết định GIỮ tính năng nhắc star ở server mode: `starNag.starOrca`/`.dismiss`/`.later` không cần `BrowserWindow`, chỉ cần `checkOrcaStarred()` (GitHub API) + `track()` (đã có). Phần `subscribe`/`forceShow`/`agentValueMoment`... gắn UI-trigger cụ thể hơn, cần xem frontend có gọi thật hay chỉ desktop dùng. |

## Đề xuất bước tiếp theo

1. **starNag.onboardingCompleted đang lỗi live** — mức độ thấp (không chặn
   luồng chính), nhưng vẫn là 1 uncaught promise rejection trên console mỗi
   lần có user mới hoàn tất onboarding. Khuyến nghị: port tối thiểu 3 method
   không phụ thuộc `BrowserWindow` (`.dismiss`/`.later`/`.onboardingCompleted`)
   để hết lỗi, hoặc bọc try/catch phía frontend nếu quyết định không cần
   tính năng nhắc star ở server mode.
2. **Nhóm B (17 method)** — an toàn để port khi có thời gian, độ ưu tiên
   theo giá trị sử dụng thực tế (không có gì khẩn cấp).
3. **Nhóm C (31 method)** — KHÔNG nên port trước khi có quyết định rõ ràng,
   đặc biệt `orcaProfiles.*` (16 method, nghe như tính năng lớn) và
   `claudeAccounts.*`/`codexAccounts.*` (11 method, nhiều khả năng trùng vai
   trò với `aiProvider.*` đã hoạt động).
4. **Nhóm A (99 method)** — khuyến nghị không port. Nếu muốn hết sạch lỗi
   console "Unknown method" cho các tính năng này khi có, cách rẻ hơn là
   guard chung phía frontend (coi "Unknown method" cho các namespace này là
   "tính năng không khả dụng ở chế độ hiện tại", không log lỗi đỏ) thay vì
   viết 99 method backend không có tác dụng thật.
