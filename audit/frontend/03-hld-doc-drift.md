# 03 — Doc Drift: `docs/hld/web-server-architecture.md` vs Code Thật

Đối chiếu §5 (Transport), §9–§12 (UI Components, Admin SPA, Routing), §14. Đây là phần phát hiện nhiều lệch nhất trong toàn bộ audit — doc mô tả một hệ thống mà một phần không còn khớp với code hiện tại.

---

## 1. §5.1 Wire Protocol — sai hoàn toàn

**Mức độ: 🟡 Medium (doc drift, không phải bug code)**

Doc ([web-server-architecture.md:219-225](../../docs/hld/web-server-architecture.md#L219)) mô tả:

```
Frame: TYPE[1B] | SEQ[4B BE] | ACK[4B BE] | LEN[4B BE] | PAYLOAD[LEN bytes]
TYPE:  0x01 = Regular | 0x09 = KeepAlive (every 30s)
PAYLOAD: UTF-8 JSON-RPC 2.0
```

**Code thật** ([frontend/src/platform/adapters/web/rpc-client.ts](../../frontend/src/platform/adapters/web/rpc-client.ts)) gửi **JSON text thuần** qua `WebSocket` — không có `ArrayBuffer`/binary framing nào, không SEQ/ACK, **không có keepalive logic nào trong file này**. Envelope thực tế cũng không phải JSON-RPC 2.0 mà là dạng ipcRenderer-style: `{ id, type: 'invoke'|'result'|'error'|'push', channel, args/result }` — comment đầu file tự mô tả: *"replaces Electron ipcRenderer with the same invoke/on/once surface"*.

**Phát hiện thú vị:** protocol nhị phân 13-byte trong doc **có thật trong code** — nhưng thuộc về subsystem khác hoàn toàn: [frontend/src/main/ssh/relay-protocol.ts:12-14](../../frontend/src/main/ssh/relay-protocol.ts#L12) (`HEADER_LENGTH = 13`, `MessageType.Regular=1`, `MessageType.KeepAlive=9`), dùng bởi SSH remote-relay multiplexer (`ssh-channel-multiplexer.ts`, style VS Code `PersistentProtocol`) cho F23/F24 remote dev server — **không liên quan gì tới `WebSocketRpcClient` của browser**. Ngay cả ở đúng chỗ, keepalive interval thật là **5s** (`KEEPALIVE_SEND_MS = 5_000`), không phải 30s như doc ghi. Có vẻ đoạn §5.1 bị copy nhầm từ tài liệu mô tả SSH relay sang mục nói về browser transport.

API thực tế (`IRpcClient`, [rpc-client-interface.ts:3-12](../../frontend/src/platform/rpc-client-interface.ts#L3)) cũng khác doc: `connect/disconnect/isConnected/invoke/send/on/off/once` — không có `call<T>()`/`callStream()` như doc mô tả. Reconnect backoff thật `[500,1000,2000,5000,10000,30000]` không giới hạn số lần thử, khác "maxRetries=3, delay=2s" trong doc.

**Gợi ý:** viết lại toàn bộ §5.1 theo code thật; xoá đoạn binary-frame protocol khỏi mục này (chuyển sang 1 mục riêng mô tả `relay-protocol.ts` nếu muốn tài liệu hoá nó).

---

## 2. §12 Routing — không có router nào tồn tại

**Mức độ: 🟡 Medium**

Doc ([dòng 928-940](../../docs/hld/web-server-architecture.md#L928)) trình bày 1 bảng route (`/`, `/login`, `/workspace/:id`, `/admin`, `/admin/users`…) như thể có 1 client-side router thống nhất.

**Thực tế:** không có router nào cho app chính. `main-web-bootstrap.tsx`'s `WebRoot()` quyết định render gì bằng **boolean/state branching thuần**, không match URL — không `useParams`, không `/workspace/:id`, không `react-router` ở bất kỳ đâu trong `src/renderer/src`. Điều hướng sau login là `window.location.href = '/'` cứng, không phải router navigation. Admin SPA (xem mục 3) là bundle HTML riêng biệt (`admin-index.html`), không nằm chung 1 route tree như bảng doc gợi ý.

**Gợi ý:** sửa §12 thành mô tả đúng cơ chế state-branching hiện tại, hoặc nếu router hoá là dự định tương lai, đánh dấu rõ "🚧 Planned" thay vì trình bày như đã có.

---

## 3. §11 Admin SPA — doc nói React Router, code nói ngược lại

**Mức độ: 🟡 Medium**

Doc ([dòng 904](../../docs/hld/web-server-architecture.md#L904)): *"Root component: AdminApp.tsx (React Router, riêng biệt với App.tsx)"*.

Code tự mâu thuẫn với câu này ở **2 chỗ**:
- [components/admin/AdminApp.tsx:1-2](../../frontend/src/renderer/src/components/admin/AdminApp.tsx#L1): *"Uses prop-driven state routing (no react-router-dom)"*.
- [components/admin/AdminLayout.tsx:1-2](../../frontend/src/renderer/src/components/admin/AdminLayout.tsx#L1): *"Uses simple hash-based routing (no react-router-dom dependency required)"*.

`react-router`/`react-router-dom` **không phải dependency ở bất kỳ `package.json` nào** trong repo. Routing thật là `useState<AdminRoute>` + so sánh string.

**Route list cũng lệch:** doc liệt kê 10 trang gồm cả **SSH Hosts** (`/admin/ssh-hosts`), **Departments** (`/admin/departments`), Company Profile ở `/admin/company`. `AdminRoute` union type thật ([AdminLayout.tsx:5-15](../../frontend/src/renderer/src/components/admin/AdminLayout.tsx#L5)) chỉ có 8 route: `/`, `/users`, `/users/new`, `/policies`, `/policies/new`, `/sessions`, `/audit`, `/profile`, `/ai-providers`, `/fleet`. **Không có trang Departments nào** (grep `DepartmentsPage` dưới `components/admin/` — 0 kết quả) — đáng chú ý vì F33 (User Profile Hierarchy) có concept "Department" ở tầng dữ liệu; SSH Hosts gộp chung vào `/fleet`; Company Profile ở route `/profile`, không phải `/admin/company`.

**Gợi ý:** cập nhật lại bảng route trong §11 theo `AdminRoute` thật; xác nhận với product owner liệu trang Departments có thực sự cần (nếu F33 cần quản lý department mà UI chưa có, đây là gap chức năng, không chỉ doc drift).

---

## 4. §10.7 GitPanel — sai path, thiếu 1 component đã tài liệu hoá

**Mức độ: 🟡 Medium**

Doc ([dòng 854](../../docs/hld/web-server-architecture.md#L854)): `File: src/renderer/src/components/workspace/GitPanel/`.

Thực tế: `frontend/src/renderer/src/components/workspace/git/GitPanel.tsx` (thư mục tên `git`, không phải `GitPanel`).

| Sub-component doc nhắc | Thực tế |
|---|---|
| `DiffViewer.tsx`, `CommitForm.tsx`, `BranchManager.tsx`, `PullRequestForm.tsx` | ✅ Khớp |
| `GitLog` ("Last 50 commits + ASCII branch graph") | Tên thật là `GitHistory.tsx` |
| **`ConflictPanel`** ("Conflict files + AI resolve") | ❌ **Không tồn tại** — grep toàn repo 0 kết quả. Có thể là component F39 chưa được implement, không chỉ đổi tên. |

Có 2 file tồn tại trong `workspace/git/` mà doc không nhắc: `StagingArea.tsx`, `PullRequestList.tsx`.

**Gợi ý ưu tiên:** xác nhận với team liệu `ConflictPanel` (AI-assisted conflict resolution cho F39 Remote Git UI) có nằm trong scope hiện tại hay bị bỏ — nếu đã bị bỏ khỏi roadmap, xoá khỏi doc; nếu vẫn cần, đây là 1 feature gap thật, không chỉ doc drift.

---

## 5. Path/naming mismatch khác (§9.1, §9.6, §9.8)

| Doc | Thực tế |
|---|---|
| `components/worktree-sidebar/` (§9.1) | `components/sidebar/` |
| `components/QuickOpen/` (thư mục, §9.6) | `components/QuickOpen.tsx` (1 file phẳng) |
| `components/fleet/` với `FleetHealthDashboard`, `BulkProvisioningWizard`, `BootstrapStatusPanel`, `UserProfileBadge` (§9.8) | Không có `components/fleet/`. Thật ra rải ở `components/settings/ssh/` (`FleetHealthDashboard.tsx`, `FleetProvisionWizard.tsx` — không phải `BulkProvisioningWizard`, `ServerBootstrapPanel.tsx`/`BootstrapStepList.tsx` — không phải `BootstrapStatusPanel`) và `components/admin/fleet/`. `UserProfileBadge` tồn tại nhưng ở `components/activity/`. 4 hook doc nhắc (`useFleetImport`, `useBootstrapAutomation`, `useServerGroups`, `useBulkProvisioning`) **không tìm thấy** ở đâu trong `hooks/` |

---

## 6. Những phần vẫn khớp tốt — ✅

- **File size** của các component lớn (App.tsx ~127KB, TaskPage.tsx ~542KB, PullRequestPage.tsx ~259KB, GitHubItemDialog.tsx ~285KB, LinearItemDrawer.tsx ~59KB, NewWorkspaceComposerCard.tsx ~60KB, TerminalPane.tsx ~127KB) — **khớp gần như chính xác byte-for-byte** với số liệu trong doc. Phần này của doc rõ ràng được cập nhật gần đây, khác hẳn các mục routing/GitPanel/fleet ở trên.
- §9.9 Onboarding & Dev Server UI, §9.10 SSH Provisioning UI, §9.11 Web Push, §10.1 WorkspaceContext (có thêm `WorkspaceContextV6.tsx`/`WorkspaceContextBridge.ts` chưa được doc nhắc — bổ sung chứ không mâu thuẫn), §10.2 Workspace Components, §10.3–§10.6 (Profile/AI Provider/Task/Workflow UI), §10.8 Remote File Explorer, §11 entry point (`admin-index.html` → `admin-main.tsx` → `AdminApp`) — tất cả khớp cấu trúc thật, chỉ có một vài component phụ chưa được doc nhắc tên (bổ sung, không phải sai).

---

## Xếp hạng mức độ lệch (nghiêm trọng nhất trước)

1. §5.1 Wire protocol — mô tả sai hoàn toàn/nhầm subsystem.
2. §12 Routing — mô tả 1 cơ chế không tồn tại.
3. §11 Admin SPA — khẳng định trái ngược với chính comment trong code; thiếu 2 trang thật (SSH Hosts, Departments).
4. §10.7 GitPanel — sai path, 1 sub-component (`ConflictPanel`) có thể là feature gap thật chứ không chỉ doc drift.
5. §9.1/§9.6/§9.8 — path/tên component lệch, một số hook được doc nhắc nhưng không tồn tại trong code.
