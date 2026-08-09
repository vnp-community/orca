# Đánh giá `agent/` Code vs Thiết kế (Chỉ mục tổng)

**Ngày:** 2026-08-08
**Phạm vi:** `agent/src/**` (Dev Server Agent — "isolated copy, split from monorepo", `agent/package.json:4`) đối chiếu với toàn bộ tài liệu thiết kế liên quan: `docs/hld/dev-server-architecture.md`, `docs/crs/v2/agent/*` (CR-AG-001..004), `docs/crs/v2/dev-server/*` (CR-DS-001..005), ADR-004/005/013/014/015 (v1) và ADR-017/018/019 (v2), F04/F06/F07/F29/F35, và các business-logic doc `docs/logic/{agent-orchestration,agent-ws,profile,ai-providers,remote-development,worktree-management}/*`.
**Phương pháp:** 5 agent song song, mỗi agent đối chiếu một mảng thiết kế với code thật bằng CodeGraph/GitNexus (đọc symbol trực tiếp, kiểm tra caller/blast-radius, không suy đoán), trích dẫn bằng chứng `file:line`. Cùng phương pháp và văn phong với `audit/backend/backend-vs-design-review.md`.
**Trạng thái:** 5/5 mảng đã hoàn tất.

**Các báo cáo chi tiết:**
1. [`connection-wire-protocol-vs-design-review.md`](./connection-wire-protocol-vs-design-review.md) — Connection Modes & Wire Protocol (relay-websocket, direct-websocket, token lifecycle, reconnect)
2. [`rpc-dispatch-lifecycle-vs-design-review.md`](./rpc-dispatch-lifecycle-vs-design-review.md) — RPC Method Surface, Dispatch & Agent Lifecycle (spawn/stop/resume/switch-account/monitor)
3. [`pty-ai-cli-vs-design-review.md`](./pty-ai-cli-vs-design-review.md) — PTY / AI Agent CLI Integration (profile-aware execution, agent hooks)
4. [`git-ssh-external-api-vs-design-review.md`](./git-ssh-external-api-vs-design-review.md) — Git & External API Integration + SSH/Remote Execution
5. [`credential-fswatch-telemetry-vs-design-review.md`](./credential-fswatch-telemetry-vs-design-review.md) — AI Provider Credential Relay, Filesystem Watcher & Observability/Telemetry

---

## 1. Tổng kết mức độ khớp (chọn lọc các phát hiện quan trọng nhất mỗi mảng)

| Mục thiết kế | Trạng thái | Vấn đề chính | Mảng |
|---|---|---|---|
| CR-AG-001 Wire Protocol Framing (13-byte header) | ⚠️ Khớp code, tài liệu tự mâu thuẫn | ADR-004/BL-AWS-01 mô tả bảng `TYPE` khác hẳn CR-AG-001/code thật | 1 |
| CR-AG-004 `direct-websocket` — port | ❌ Sai lệch | Doc ghi 6768; code thật (và cả comment sai trong chính backend) chạy trên **6769** — cùng phát hiện với backend audit | 1 |
| BL-AWS-03 Token lifecycle/renewal | ❌ Sai lệch mô hình | Doc: admin UI + DB `orca_agent_tokens`; thật: self-service `AgentTokenManager` qua `ORCA_AGENT_API_SECRET`, không DB, renew chủ động ở 80% TTL — không tài liệu nào mô tả cơ chế renew này | 1 |
| ADR-013/017/018/019 "Dev Server Agent v6.0" (`src/agent/` A0–A4, HMAC signed context) | ❌ Chưa implement — tự ADR ghi nhận | Package thật (`agent/src/relay/*`) là kiến trúc Phase-2 CR-AG-00x, không phải v6.0 | 1, 2 |
| ADR-014 JSON-RPC thuần (bỏ binary header) | ❌ Chưa triển khai | Code vẫn dùng framing 13-byte mà ADR-014 đề xuất loại bỏ | 2 |
| ADR-015 / CR-DS-005 Signed Execution Context | ❌ **Đã bị chủ động gỡ bỏ**, không chỉ thiếu | `context.ts` là stub rỗng có chủ đích (comment dẫn `docs/relay-fs-allowlist-removal.md`) — quyết định kiến trúc đối lập trực tiếp với ADR-015 | 2 |
| CR-DS-002 RPC Method Namespaces | ❌ Sai lệch nghiêm trọng | 40 method thật khác gần hết tên/namespace doc; nhóm `step.*`/`health.*` không tồn tại; thêm hẳn 1 lớp MCP (`tools/list`/`tools/call`) không tài liệu hoá | 2 |
| BL-AG-03 Resume Session (OpenCode/Gemini) | ❌ Sai lệch hành vi thật | Doc ghi OpenCode "✅ Full" nhưng `buildArgs` luôn trả mảng rỗng, bỏ qua `resumeId` hoàn toàn | 2 |
| BL-PRF-04 Profile hierarchy điều khiển agent spawn | ❌ Không xảy ra trong `agent/` | `OrcaProfile.ts` là type-only dead code; `buildAgentEnv()` không đọc field nào của nó — mọi env phải đến từ backend qua passthrough | 3 |
| `agent.exec` — 2 implementation, 1 chết | ❌ Nguy cơ bug-do-tài-liệu | Bản sống: inline case trong dispatcher; bản có đầy đủ docblock TG-001 (`handleAgentExec`) **không hề được gọi** | 2, 3 |
| 3 state machine "agent status" song song | ⚠️ Không hợp nhất | `AgentLifecycleState` (6-state), `AgentStatusState` (4-state), `AgentStatus` (3-state) — không cái nào khớp HLD §11.3/BL-AG-05 | 2, 3 |
| GitCapabilityCache pattern (AGENTS.md) | ✅ Khớp hoàn toàn | Instance duy nhất, predicate hẹp, lan tỏa nhất quán qua 5 module con của `GitHandler` — điểm sáng nhất toàn bộ audit | 4 |
| Git Engine — 3+1 lớp thực thi song song | ❌ Trùng lặp kiến trúc thật | `GitHandler`, `agent-git-handler.ts`, `external-api-connector.ts` đều tự viết lại exec; `runner.ts` (circuit-breaker+retry+WSL) chỉ 1 caller, bypass hoàn toàn bởi đường PR/MR chính | 4 |
| Agent Isolation §7 "Git author injected từ ctx.userEmail" | ❌ Không tồn tại | Chỉ có `git config --global` một lần qua `preflight.setGitIdentity`, không gắn RequestContext, có thể bị override | 4 |
| "Orca Server KHÔNG thấy plaintext key" | ❌ Chỉ đúng cho ghi | Luồng spawn: comment code tự thừa nhận Orca Server phải inject `resolvedApiKey` plaintext; nhánh fallback tự nhận không hoạt động đúng | 5 |
| Credential store path & thuật toán | ❌ Sai lệch | Doc: `~/.orca/ai-providers/`, salt=accountId; thật: `~/.orca/credentials/`, salt ngẫu nhiên. File khớp path doc nhất (`ai-provider-handler.ts`) lại là dead code (0 caller) | 5 |
| Cluster parcel-watcher (`agent/src/main/ipc/*`) | ⚠️ Đúng cơ chế, vô hình với HLD | Đứng sau `fs.watch`/`fs.changed` nhưng chỉ cho binary `relay.js` (SSH mode) — không được HLD §15.3 nhắc tới; không xác nhận được build pipeline đóng gói binary này | 5 |
| Observability/Telemetry/Diagnostics trong `agent/` | 📄 Không có thiết kế + dead code | 0 tài liệu; `initTelemetry` 0 caller; code trích dẫn "telemetry-error-tracking.md" không tồn tại; import `electron` vào gói headless Node | 5 |

---

## 2. Nhận định xuyên suốt (pattern lặp lại ở nhiều mảng)

1. **Hai "thế hệ" tài liệu thiết kế cùng tồn tại, không phân biệt rõ trạng thái.** CR-AG-001..004 (Phase 2, đã triển khai) mô tả khá sát code thật. Nhưng ADR-013/014/015/017/018/019 và CR-DS-001..005 mô tả kiến trúc **"Dev Server Agent v6.0"** — tự các tài liệu này đã ghi `Trạng thái: Proposed` / `❌ Chưa implement` — trỏ tới đường dẫn code (`src/agent/rpc/*`, `src/agent/pty/*`...) **không tồn tại** trong monorepo thật. Vấn đề là các doc "cầu nối" (`docs/flows/code/agent-ws/agent-connection-modes.md`, HLD §11/§15) trộn lẫn cả 2 thế hệ trong cùng file mà không gắn nhãn rõ ràng, khiến người đọc dễ tưởng nhầm kiến trúc v6.0 đã có thật. Đây là loại sai lệch khác về bản chất so với backend audit (ở đó phần lớn là "tài liệu quên cập nhật theo code đã đổi"; ở đây phần lớn là "tài liệu mô tả roadmap chưa tới").
2. **Một khoảng trống bảo mật thật, không chỉ là lệch tài liệu, xuất hiện lặp lại ở 3 mảng độc lập:** ADR-015 Signed Execution Context bị chủ động gỡ bỏ (mảng 2); Git author injection theo `ctx.userEmail` không tồn tại, chỉ có `git config --global` mutable (mảng 4); credential plaintext phải được Orca Server cung cấp ở bước spawn agent, trái với lời hứa "Gateway không thấy plaintext" (mảng 5). Cả 3 đều cùng một chủ đề: **trust boundary thật đã dịch chuyển lên trên (renderer/Orca Server), nhưng tài liệu bảo mật vẫn mô tả agent như một trust boundary độc lập tự verify mọi thứ.** Nên được xác nhận có chủ đích với đội bảo mật, không tự động coi là bug.
3. **Trùng lặp/dead code xuất hiện có hệ thống, thường đi kèm docblock gây hiểu nhầm.** `agent.exec` (2 bản, bản có tài liệu đầy đủ lại chết — mảng 2&3), `git.pr.create` vs `github.pr.create` (2 implementation cùng chức năng — mảng 4), `ai-provider-handler.ts` (khớp đúng path tài liệu nhất nhưng là code chết — mảng 5), `runner.ts`'s circuit-breaker (chỉ 1 caller, bị toàn bộ đường git/gh chính bypass — mảng 4), telemetry/observability/diagnostics (copy nguyên xi từ `desktop/`, chưa từng wire vào `agent-entry.ts` — mảng 5). Pattern chung: **code "đã viết xong nhưng chưa/không còn được gọi" tồn tại phổ biến hơn dự kiến**, và trong nhiều trường hợp bản chết lại là bản có tài liệu/docblock đầy đủ hơn — rủi ro cao khi bảo trì nhầm bản.
4. **3 state machine "agent status" không hợp nhất được xác nhận độc lập bởi cả mảng 2 và mảng 3** (cùng tìm ra `AgentLifecycleState`/`AgentStatusState`/`AgentStatus`), và trùng với phát hiện tương tự ở backend audit (§6.3) — đây là nợ kiến trúc xuyên suốt nhiều package, không phải trùng hợp cục bộ.
5. **Sai lệch namespace RPC là loại lỗi tài liệu phổ biến nhất**, lặp lại giống hệt pattern đã thấy ở backend audit (§9): `aiProvider.*` vs `ai.provider.*`, `worktree.*` vs `git.worktree.*`, `git.*` phẳng vs `github.*`/`gitlab.*` tách namespace — luôn là quy ước đặt tên (camelCase liền vs dot-separated, gộp vs tách namespace) chứ không phải khác biệt hành vi.
6. **Port Agent WS 6768 vs 6769** — phát hiện đã có ở backend audit (§2.2/§2.4) được xác nhận lại độc lập từ phía `agent/` (mảng 1): cấu hình mặc định phía agent (`wss://.../agent`, không có port tường minh, dựa vào TLS 443 + Nginx `:6769`) củng cố thêm bằng chứng 6769 mới là port thật.

## 3. Khuyến nghị ưu tiên cao (tổng hợp — chi tiết đầy đủ xem từng file mảng)

- **Gắn nhãn rõ trạng thái triển khai ngay đầu mỗi tài liệu v6.0** (ADR-013/014/015/017/018/019, CR-DS-001..005) — hiện chỉ có 1 dòng trạng thái ẩn cuối file — để tránh đọc nhầm thành hệ thống hiện hành. Ưu tiên cao nhất vì ảnh hưởng cách hiểu toàn bộ các mảng 1 & 2.
- **Xác nhận chính thức mô hình trust hiện tại** (renderer/SSH-user là trust boundary, agent tin tưởng hoàn toàn) — hoặc viết ADR mới thay ADR-015, hoặc khôi phục `ContextVerifier`/path-allowlist như thiết kế — không để tài liệu bảo mật và code mâu thuẫn nhau khi review.
- **Dọn dead code có docblock gây hiểu nhầm trước tiên**: `handleAgentExec()` (mảng 2/3), `ai-provider-handler.ts` (mảng 5), hợp nhất `git.pr.create`/`github.pr.create` (mảng 4) — đây là nhóm rủi ro bảo trì cao nhất vì dễ khiến người sửa nhầm bản chết.
- **Sửa port 6768→6769** trong mọi tài liệu Agent WS liên quan (đồng bộ với khuyến nghị backend audit) — bao gồm sửa luôn message lỗi runtime sai trong `backend/src/main/dev-server/agent-ws-server.ts:103`.
- **Viết lại CR-DS-002 theo method surface thật** (40 method, namespace `ai.provider.*`, generic passthrough `git.exec`/`agent.exec`, lớp MCP `tools/*`) thay vì method chi tiết theo doc hiện tại.
- **Hợp nhất 3 state machine agent-status** hoặc tài liệu hoá rõ phạm vi từng cái — khuyến nghị lặp lại từ backend audit, càng có thêm bằng chứng ưu tiên cao.

---

*Đây là chỉ mục tổng hợp. Mọi trích dẫn `file:line` chi tiết nằm trong 5 file mảng liệt kê ở đầu tài liệu này.*
