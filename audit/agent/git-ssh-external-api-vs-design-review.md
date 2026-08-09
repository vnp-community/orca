# Đánh giá `agent/` (Git & External API Integration + SSH/Remote Execution) vs Thiết kế

**Ngày:** 2026-08-08
**Phạm vi:** `agent/src/relay/git*.ts`, `agent/src/relay/agent-git-handler.ts`, `agent/src/relay/external-api-connector.ts`, `agent/src/main/git/*`, `agent/src/main/{wsl,git-bash,pwsh,win32-utils}.ts`, `agent/src/main/ssh/*` đối chiếu với `docs/hld/dev-server-architecture.md` §2‑§12, F06, F07, BL-SSH-01/03, BL-WT-01..04, BL-PI-03/04, ADR-003, `docs/reference/git-compatibility.md`.
**Phương pháp:** GitNexus/CodeGraph (đọc symbol trực tiếp), trích dẫn `file:line`. Đã loại `git-remote-handler*.ts` khỏi phạm vi (xóa gần đây, dead code, xác nhận qua GitNexus) — git engine thật là `GitHandler` (`git-handler.ts`).

---

## 1. Bảng tổng kết

| Mục thiết kế | Trạng thái | Vấn đề chính |
|---|---|---|
| Git Engine (§3a/§3b, §8) | ⚠️ Một phần | `GitHandler` (git-handler.ts) đúng vai trò "Git Engine" thật, nhưng có **3 bộ thực thi gh/git song song, không chia sẻ code** (`GitHandler`, `agent-git-handler.ts`, `external-api-connector.ts`) — trùng lặp `git.pr.create`/`github.pr.create` cùng chức năng |
| GitCapabilityCache pattern (AGENTS.md) | ✅ Khớp | Dùng đúng mẫu: 1 instance `GitCapabilityCache` sở hữu bởi `GitHandler`, threading qua `git-handler-worktree-list.ts`, `git-handler-worktree-remove.ts`, `git-handler-branch-cleanup.ts`, `git-handler-local-base-ref-refresh.ts`; predicate hẹp (`isUnsupportedWorktreeListZError`, `isUnsupportedRevParsePathFormatError`) |
| `gh-rate-limit-breaker.ts` + `runner.ts` (WSL/retry-aware git/gh runner) | ❌ Không được dùng bởi RPC git/gh chính | Circuit breaker + retry + WSL routing tinh vi trong `agent/src/main/git/runner.ts` chỉ có **1 caller** trong toàn bộ agent/ (`commit-message-text-generation.ts`) — `GitHandler`, `agent-git-handler.ts`, `external-api-connector.ts` đều tự viết lại `execFile`/`spawn` thô, KHÔNG dùng `ghExecFileAsync`/`gitExecFileAsync` |
| External API Connector — CLI-based, per-user isolation (§12.1, §12.2, §12.5) | ✅ Khớp phần lớn | `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` per-userId đúng path thiết kế; "Auth never through Gateway" đúng; nhưng RPC namespace sai (`github.*`/`gitlab.*` thay vì `git.*`) và thiếu operations (`pr.view`, `mr.view`, `issue.create` GitLab, `repo.clone`) |
| Agent Isolation — Git author injection (§7 "Git author") | ❌ Không tồn tại đúng như thiết kế | Thiết kế: "Injected từ `ctx.userEmail`, không thể bị override". Thực tế chỉ có `preflight.setGitIdentity` ghi **global** `git config user.name/user.email` — mutable, không per-call, không gắn với `RequestContext` |
| Wire Protocol 13-byte header (§5) | ✅ Khớp chính xác | `HEADER_LENGTH=13`, `MessageType.Regular=1/KeepAlive=9` khớp tuyệt đối |
| SSH connectivity (BL-SSH-01/03, F07) | ⚠️ Ngoài phạm vi thực tế của `agent/` | Toàn bộ SSH client-side (`ssh-connection.ts`, `ssh-relay-session.ts`, `ssh-config-parser.ts`, `ssh-port-forward.ts`...) nằm ở `backend/src/main/ssh/` và `desktop/src/main/ssh/`, **KHÔNG có trong `agent/`**. `agent/src/main/ssh/` chỉ chứa phía relay/remote-platform (multiplexer, protocol, stream reader) |
| Worktree Management (BL-WT-01..04) | ✅ Khớp phần lớn | `git worktree add/remove --force`, GitCapabilityCache fallback cho Git cũ đều đúng thiết kế |
| `docs/reference/git-compatibility.md` (baseline Git 2.25) | ❌ File không tồn tại | AGENTS.md và SRS.md đều tham chiếu file này nhưng nó **không có trên disk** |
| Cross-platform shims (WSL/git-bash/pwsh/win32) | ✅ Khớp | Đúng vai trò local-Windows detection/probe, cache TTL hợp lý; nhưng **không được `GitHandler` sử dụng** (chỉ dùng bởi `runner.ts`/`preflight-handler.ts`) |
| BL-PI-03/04 (update issue status, submit PR review) | ⚠️ Không xác nhận được trong `agent/` | Logic mapping event→status và submit review comment không nằm trong `agent/` (thuộc backend/Gateway) — hợp lý theo kiến trúc Control/Data Plane nhưng tài liệu không nói rõ ranh giới |

---

## 2. Chi tiết theo mục

### 2.1 Git Engine — 3 bộ thực thi song song, không thống nhất

Thiết kế (`dev-server-architecture.md:60`: *"Git Engine: Full git ops + PR creation via `gh` CLI"*) mô tả **một** Git Engine duy nhất. Thực tế `agent/` có 3 lớp độc lập, mỗi lớp tự viết lại logic exec/env:

1. **`GitHandler`** (`agent/src/relay/git-handler.ts:260-428`) — engine chính, 35+ RPC methods (`agent/src/relay/relay.ts` gọi 13 chỗ). Có `private git()` riêng (dòng 393-428) dùng `buildRelayGitEnv()` (`agent/src/relay/relay-command-env.ts:165-170`) + `execFileAsync` thô — **không đi qua** `agent/src/main/git/runner.ts`.
2. **`agent-git-handler.ts`** — `handleGitPrCreate` (dòng 277-332), `handleGitWorktreeList/Add/Remove` (dòng 333+) — hàm độc lập, tự build `GH_CONFIG_DIR` inline (dòng 304-310), tự `execFileAsync`, **không có retry, không WSL, không GitCapabilityCache**.
3. **`external-api-connector.ts`** — `handleGitHubPrCreate` (dòng 130-191) với `execFileCaptured()` riêng (dòng 34-70, dùng `spawn()` thô), có idempotency check (`checkExistingPr`, dòng 108-126) nhưng **cũng không** dùng breaker/retry của `runner.ts`.

**Bằng chứng trùng lặp cụ thể:** RPC dispatch (`agent/src/relay/agent-rpc-dispatch.ts`) đăng ký **cả hai** `case 'git.pr.create'` (dòng 411, → `agent-git-handler.handleGitPrCreate`) **và** `case 'github.pr.create'` (dòng 488, → `external-api-connector.handleGitHubPrCreate`) — hai implementation khác nhau cho cùng một chức năng "tạo PR", một cái có idempotency-check, một cái không.

### 2.2 `gh-rate-limit-breaker.ts` + `runner.ts` — sức mạnh không được dùng ở đúng chỗ

`agent/src/main/git/runner.ts` là bộ chạy git/gh/glab tinh vi nhất trong `agent/`:
- WSL routing đầy đủ (`resolveCommand`, dòng 185-258)
- `ghExecFileAsync` (dòng 1405-1495): tích hợp circuit breaker (`classifyGhRateLimitBucket`, `getGhRateLimitBlockedUntilMs`, `notifyGhPrimaryRateLimit` — import từ `gh-rate-limit-breaker.ts:65-146`), retry transient 5xx/429 với backoff (`GH_RETRY_DELAYS_MS`, dòng 1367), idempotency-aware retry gate (`argsLookIdempotent`, dòng 1159-1226)
- `glabExecFileAsync` (dòng 1548-1605) tương tự cho GitLab

Nhưng **chỉ một caller trong toàn bộ `agent/`**: `agent/src/main/text-generation/commit-message-text-generation.ts`. `GitHandler`, `agent-git-handler.ts`, `external-api-connector.ts` — tức toàn bộ bề mặt RPC `git.*`/`github.*`/`gitlab.*` mà user thực sự dùng để tạo PR/MR/issue — đều bypass module này. Hệ quả: circuit breaker được thiết kế riêng để chặn "90-repo Tasks-page fan-out storm" (comment `gh-rate-limit-breaker.ts:4-9`) **không bảo vệ** đường PR/MR create thực tế mà chỉ bảo vệ generate-commit-message.

### 2.3 GitCapabilityCache — tuân thủ đúng pattern AGENTS.md yêu cầu

✅ Đây là điểm khớp tốt nhất: `GitCapabilityCache` (`agent/src/shared/git-capability-cache.ts:14-115`) — probe dedupe, negative-cache retry interval 30 phút (dòng 3), narrow predicate injection qua `isUnsupportedError` callback (dòng 42).

- Một instance duy nhất: `private readonly gitCapabilities = new GitCapabilityCache()` (`git-handler.ts:264`), threading xuyên suốt qua tham số vào các module con:
  - `readRepoLocation()` — `runWithFallback('rev-parse-path-format', ...)` (`git-handler.ts:1388-1409`), remember-unsupported khi phát hiện Git cũ echo option lạ (dòng 1395-1399)
  - `listWorktrees()` — `runWithFallback('worktree-list-z', ...)` (`git-handler.ts:1450-1476`), fallback sang parser line-block cho Git <2.36 (comment dòng 1463-1464)
  - `git-handler-worktree-list.ts:16-18`, `git-handler-worktree-remove.ts:76,127`, `git-handler-branch-cleanup.ts:15`, `git-handler-local-base-ref-refresh.ts:8` — đều nhận `GitCapabilityCache` qua tham số, dùng chung instance của `GitHandler`.

Không có git command mới nào trong `agent/relay` tự retry/probe capability ngoài cơ chế này — đúng khuyến nghị AGENTS.md.

### 2.4 `docs/reference/git-compatibility.md` — file được tham chiếu nhưng không tồn tại

`AGENTS.md:39` và `docs/SRS.md:1141` đều yêu cầu bám theo `docs/reference/git-compatibility.md` làm baseline Git 2.25, nhưng:
```
find /opt/repos/orca/docs/reference -type f  → (không tồn tại thư mục/file)
```
Không có cách nào xác minh chính sách baseline-compatibility bằng tài liệu — chỉ có thể suy ra từ code (VD: `git-handler.ts:1463-1464` fallback cho Git <2.36 khi dùng `-z`). Đây là khoảng trống tài liệu, không phải lỗi code.

### 2.5 External API Connector — đúng nguyên tắc thiết kế nhưng sai namespace và thiếu method

Header comment tại `external-api-connector.ts:1-11` liệt kê đúng 6 nguyên tắc thiết kế của §12.5 dev-server-architecture.md ("CLI-based not SDK", "Per-user isolation", "No shell injection: spawn() with array args, shell:false", "Auth never through Gateway"). Đối chiếu:

| Nguyên tắc thiết kế | Trạng thái | Bằng chứng |
|---|---|---|
| CLI-based (gh/glab, không SDK) | ✅ | `execFileCaptured('gh', ...)`/`'glab'` khắp file |
| `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` per-userId | ✅ | `buildGhEnv()` (dòng 74-81): `GH_CONFIG_DIR: \`${homedir()}/.config/gh/${userId}/\`` — khớp chính xác path thiết kế (`dev-server-architecture.md:475,509`) |
| `shell: false` | ✅ | `external-api-connector.ts:44` |
| Idempotency PR create | ✅ | `checkExistingPr()` (dòng 108-126) trước khi `gh pr create` |
| "Auth never through Gateway" | ✅ | Token nằm trong `~/.config/gh/<userId>/` trên filesystem Dev Server, không đi qua payload RPC |

**Sai lệch namespace:** thiết kế bảng §12.1/§12.2 định nghĩa RPC namespace phẳng `git.pr.create`, `git.pr.view`, `git.pr.merge`, `git.issue.list`, `git.issue.create`, `git.repo.clone`, `git.mr.create`, `git.mr.view`, `git.mr.list`, `git.pipeline.status`. Thực tế `agent-rpc-dispatch.ts` đăng ký: `github.pr.create` (dòng 488), `github.pr.merge` (499), `github.issue.list` (510), `github.issue.create` (521), `gitlab.mr.create` (532), `gitlab.pipeline.status` (543) — namespace `github.*`/`gitlab.*`, không phải `git.*` như tài liệu.

**Thiếu operations có sẵn trong code nhưng không được wire:** `handleGitLabMrList` (`external-api-connector.ts:394-423`), `handleGitHubAuthStatus` (dòng 311-339), `handleGitLabAuthStatus` (dòng 460-488) tồn tại trong file nhưng **không có `case` tương ứng** trong `agent-rpc-dispatch.ts` (đã kiểm tra toàn bộ các `case 'git...`/`'gh...'` — chỉ có 13 case, không có `gitlab.mr.list`/`github.auth.status`/`gitlab.auth.status`). `git.pr.view`, `git.mr.view`, `git.repo.clone` không hề có implementation nào trong `agent/` — clone dùng `GitCloneHandler` (`git-handler-clone.ts:8`) qua path khác, không qua RPC `repo.clone` như thiết kế đặt tên.

### 2.6 Agent Isolation Model §7 — "Git author injected từ ctx.userEmail" không tồn tại

Thiết kế (`dev-server-architecture.md:192`): *"Git author | Injected từ `ctx.userEmail`, không thể bị override"* — ngụ ý mỗi git operation tự động gắn author = user hiện tại của RPC context, không cho override.

Thực tế: grep toàn bộ `git-handler.ts` và `agent-git-handler.ts` cho `userEmail`/`GIT_AUTHOR`/`GIT_COMMITTER` → **0 kết quả**. Cơ chế duy nhất liên quan đến identity là `preflight.setGitIdentity` (`preflight-handler.ts:290-293`):
```ts
private async setGitIdentity(params: { name: string; email: string }): Promise<void> {
  await execFileAsync('git', ['config', '--global', 'user.name', params.name])
  await execFileAsync('git', ['config', '--global', 'user.email', params.email])
}
```
Đây là ghi **global git config một lần**, không gắn với `RequestContext`, không "không thể override" — bất kỳ lệnh `git.exec` nào cũng có thể tự thay đổi `user.name`/`user.email` qua `git config` thường. Đây là khoảng trống thiết kế thật (không chỉ sai tên) nếu multi-user isolation trên cùng Dev Server Agent instance là yêu cầu bảo mật nghiêm túc.

### 2.7 Wire Protocol (§5) — khớp chính xác

`agent/src/main/ssh/relay-protocol.ts:14` `HEADER_LENGTH = 13`; `MessageType.Regular = 1` (dòng 19), `MessageType.KeepAlive = 9` (dòng 20) — khớp tuyệt đối với thiết kế (`dev-server-architecture.md:150-155`: `TYPE[1B]|SEQ[4B]|ACK[4B]|LEN[4B]`, `TYPE = 0x01 Regular | 0x09 KeepAlive`). `FrameDecoder` (dòng 168-295) implement đúng: peek/take/discard theo chunk-list, không dùng `Buffer.concat` O(n²) (comment dòng 169-173 giải thích lý do). File này ghi chú tham chiếu tới `design-ssh-support.md` (dòng 3) — một tài liệu KHÔNG nằm trong danh sách tài liệu audit được giao, nhưng khớp với mô tả §5 của `dev-server-architecture.md`.

### 2.8 SSH connectivity (BL-SSH-01/03, F07) — phần lớn nằm ngoài `agent/`

F07 (`docs/features/F07-ssh-worktrees.md:143-151`) liệt kê `src/main/ssh/ssh-connection.ts`, `ssh-relay-session.ts`, `ssh-relay-deploy.ts`, `ssh-config-parser.ts`, `ssh-port-forward.ts`, `ssh-port-scanner.ts`, `system-ssh-file-transfer.ts` làm "Yêu cầu kỹ thuật". Khảo sát thực tế:
```
find ... -iname "ssh-connection.ts" -o ... → chỉ có trong backend/src/main/ssh/, desktop/src/main/ssh/, frontend/src/main/ssh/
```
**Không file nào trong số này tồn tại trong `agent/`.** `agent/src/main/ssh/` chỉ có 6 file: `relay-protocol.ts`, `ssh-channel-multiplexer.ts`, `ssh-remote-platform.ts`, `ssh-filesystem-stream-reader.ts`, `ssh-git-response-stream-reader.ts`, `ssh-target-id-migration.ts` — đây là phía **client library dùng để nói chuyện với relay đã deploy** (multiplexer/protocol/stream reader), KHÔNG phải phía thiết lập kết nối SSH (auth, `~/.ssh/config` parsing, exponential-backoff reconnect loop của BL-SSH-01/03). Logic BL-SSH-01 (auth negotiation, ProxyJump, keepalive) và BL-SSH-03 (reconnect loop, buffer 10MB) nằm hoàn toàn ở `backend/src/main/ssh/ssh-connection.ts` và `ssh-relay-session.ts` — **ngoài phạm vi `agent/`** đúng theo mô hình Control Plane (khởi tạo kết nối) vs Data Plane (thực thi lệnh) của kiến trúc, nhưng F07 gộp chung cả hai phía dưới một feature không phân biệt rõ package nào implement phần nào.

`ssh-remote-platform.ts:1-40` (`RemoteHostPlatform`, `RemotePathFlavor`, `RemoteCommandDialect`) đúng vai trò "phát hiện platform remote" cần cho việc dịch đường dẫn/command khi target là Windows/macOS/Linux — hỗ trợ đúng yêu cầu AGENTS.md "File paths: dùng path utilities, không giả định `/` hay `\`".

### 2.9 WSL/git-bash/pwsh/win32 shims — đúng vai trò nhưng bị cô lập khỏi `GitHandler`

- `agent/src/main/wsl.ts` — `parseWslPath`, `listWslDistros` (cache negative TTL 15s, dòng 120-121), `getWslHome` — đầy đủ, đúng comment "native Windows binaries... đều routed qua wsl.exe".
- `agent/src/main/git-bash.ts:42-92` — `getGitBashCandidatePaths()` dò `ProgramFiles`/`ProgramW6432`/`LOCALAPPDATA` + PATH scan — đúng yêu cầu cross-platform.
- `agent/src/main/pwsh.ts:44-65` — cache dương vĩnh viễn/âm có TTL 30s, xử lý cold-start timeout riêng (dòng 29-37) — hợp lý.
- `agent/src/main/win32-utils.ts:24-38` — dùng full path `System32\icacls.exe`/`whoami.exe` vì Electron main process có thể có PATH bị cắt — đúng pattern phòng thủ.

Tất cả các shim này chỉ được `agent/src/main/git/runner.ts` (dòng 31-32 import) và `preflight-handler.ts` (dòng 9-11, dùng cho `detectWindowsTerminalCapabilities`) sử dụng. Như đã nêu ở 2.2, `runner.ts` không phải đường đi chính cho git RPC — nên hiệu lực thực tế của toàn bộ lớp WSL-awareness này với `GitHandler`/`git.exec` là **0**; `GitHandler.git()` chỉ gọi `buildRelayGitEnv()` (chỉnh PATH + locale, `relay-command-env.ts:165-170`), không có bước `resolveCommand`/WSL-routing nào.

---

## 3. Nhận định tổng quan

1. **`GitCapabilityCache` là điểm sáng nhất** của phạm vi audit này — triển khai đúng 100% pattern AGENTS.md yêu cầu (narrow predicate, single shared instance, retry-interval cache), lan tỏa nhất quán qua 5 module con của `GitHandler`.
2. **Trùng lặp kiến trúc nghiêm trọng nhất**: 3 lớp thực thi git/gh riêng biệt (`GitHandler`, `agent-git-handler.ts`, `external-api-connector.ts`) cộng thêm lớp thứ 4 (`agent/src/main/git/runner.ts`) có sẵn circuit-breaker + retry + WSL-routing nhưng gần như không ai gọi tới cho đường git/PR/MR chính. Đây không chỉ là sai lệch tài liệu mà là nợ kiến trúc thật — hai implementation `git.pr.create` và `github.pr.create` cùng tồn tại, một có idempotency-check một không.
3. **Khoảng trống bảo mật cần lưu ý**: lời hứa "Git author injected từ ctx.userEmail, không thể override" trong Agent Isolation Model không có code tương ứng — chỉ có `git config --global` một lần, không ràng buộc theo RPC context.
4. **Namespace RPC git/gh sai lệch có hệ thống** giống pattern đã thấy ở audit backend: tài liệu dùng `git.*` phẳng cho mọi external-API method, code thực tế tách `github.*`/`gitlab.*`, và một số method có sẵn trong code (`handleGitLabMrList`, `handleGitHubAuthStatus`, `handleGitLabAuthStatus`) chưa được wire vào dispatcher.
5. **SSH scope trong `agent/` hẹp hơn F07 mô tả nhiều** — F07 liệt kê cả phần thiết lập kết nối (thuộc `backend`/`desktop`), khiến người đọc dễ tưởng `agent/` sở hữu toàn bộ SSH stack trong khi thực tế nó chỉ là phía relay/multiplexer chạy trên remote host.

## 4. Khuyến nghị

- **Hợp nhất 3 lớp git/gh exec** thành một: hoặc route `GitHandler`/`agent-git-handler.ts`/`external-api-connector.ts` qua `agent/src/main/git/runner.ts` để mọi gh call đều được circuit-breaker/retry bảo vệ, hoặc xóa `runner.ts`'s gh/git runner nếu không còn định dùng cho relay path.
- **Loại bỏ trùng `git.pr.create` vs `github.pr.create`** — giữ một implementation (khuyến nghị giữ bản có idempotency-check của `external-api-connector.ts`), deprecate bản kia.
- **Wire nốt `gitlab.mr.list`/`github.auth.status`/`gitlab.auth.status`** vào `agent-rpc-dispatch.ts` hoặc xóa code chết nếu không cần.
- **Bổ sung `docs/reference/git-compatibility.md`** — file được 2 nơi khác trong repo tham chiếu nhưng không tồn tại.
- **Làm rõ trong `dev-server-architecture.md` §7** cơ chế "Git author injection" thực tế là gì (`preflight.setGitIdentity` global config) — hoặc triển khai đúng như tài liệu hứa (per-call `GIT_AUTHOR_NAME`/`GIT_AUTHOR_EMAIL` env từ `RequestContext`, không cho `git.exec` override).
- **Sửa F07** để phân định rõ `src/main/ssh/*` được liệt kê thuộc `backend`/`desktop` (kết nối) khác với `agent/src/main/ssh/*` (relay-side protocol) — tránh nhầm lẫn phạm vi package.

---

*Phạm vi: Git & External API Integration + SSH/Remote Execution của `agent/` — một trong 5 mảng của audit tổng `agent/`, xem chỉ mục tại `audit/agent/agent-vs-design-review.md`.*
