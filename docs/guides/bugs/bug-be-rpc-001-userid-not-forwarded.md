# BUG-BE-RPC-001: `ctx.userId` không bao giờ được forward cho phiên web — lỗi nền tảng, không phải lỗi riêng "Create Project"

**Phát hiện:** 2026-08-14, từ report của người dùng "You do not have permission to create a project."
khi bấm Create New Project. Điều tra ban đầu tưởng là lỗi RBAC riêng của `project.create`, nhưng
truy ngược tận gốc lộ ra đây là **lỗi transport nền tảng ảnh hưởng MỌI RPC method có kiểm tra
`ctx.userId`** — không chỉ `project.*`.

## Triệu chứng thật

Log server: `[DIAG runtime_error] method=project.list error=Error: UNAUTHENTICATED` và
`method=project.create error=Error: UNAUTHENTICATED` — dù `[WsSessionRouter] Connection accepted
for userId=5540a296-...` xác nhận phiên web đã xác thực đúng.

## Root cause — xác nhận bằng cách đọc trực tiếp source, không đoán

Chuỗi gọi thật khi user thao tác trên web:

```
Browser WS → WsSessionRouter (biết đúng userId, đã xác thực)
   → proxy sang Unix socket /data/orca/users/<userId>/orca.sock
      → UnixSocketTransport.dispatchMessage() (backend/src/main/runtime/rpc/unix-socket-transport.ts)
         → this.messageHandler(rawMessage, reply, { signal, startKeepalive })   ← KHÔNG có userId
            → OrcaRuntimeRpcServer.handleMessage() (backend/src/main/runtime/runtime-rpc.ts:937)
               → this.dispatcher.dispatchStreaming(request, reply, {
                   signal, clientId: request.authToken   ← KHÔNG có userId ở đây
                 })
                  → RpcDispatcher.dispatchStreaming() (backend/src/main/runtime/rpc/dispatcher.ts:130)
                     → method.handler(params, { ..., userId: options?.userId })  ← luôn undefined
                        → project-rpc-handler.ts: if (!ctx.userId) throw new Error('UNAUTHENTICATED')
```

**`OrcaRuntimeRpcServer.handleMessage()` chưa bao giờ forward `userId` vào `dispatchStreaming()`.**
Đây không phải lỗi mới — `RpcContext.userId`'s doc comment (`backend/src/main/runtime/rpc/core.ts`)
đã mô tả đúng ý định thiết kế từ trước: *"Each user-process in ORCA_MULTI_USER=1 mode has a
distinct userId injected via ORCA_USER_ID env var and forwarded here from the session router."* —
nhưng phần "forwarded here" chưa từng được implement ở `handleMessage()`.

`process.env['ORCA_USER_ID']` **có sẵn** trong mỗi user-process (do
`SessionManager.spawnUserProcess()` inject lúc `fork()`, xem `backend/src/main/session/
session-manager.ts:147`) — nhưng `OrcaRuntimeRpcServer`/`server-bootstrap.ts` chưa từng đọc nó ra
để gắn vào context dispatch.

## Mức độ ảnh hưởng thật — LỚN HƠN NHIỀU so với bug report ban đầu

Grep xác nhận các RPC method sau đều `if (!ctx.userId) throw new Error('UNAUTHENTICATED')` hoặc
tương đương, tức **đều bị lỗi này với MỌI phiên web** kể từ khi được xây (không phải chỉ hôm nay):

- `profile.getResolved`, `profile.updateUser` (`profile-rpc-handler.ts`)
- `project.list`, `project.create`, `project.update`, `project.delete`, ... (`project-rpc-handler.ts`)
- `team.create`, `team.addMember`, `team.removeMember` (qua `requireAdmin()`, `team-rpc-handler.ts`)
- `orcaProjects.linkSourceProject`, `getProjectData`, ... (`orca-project-sharing-rpc-handler.ts`)
- **Xác nhận thêm**: toàn bộ `task.*` (`task-rpc-handler.ts`) đọc `ctx.userId ?? ''` ở ≥10 chỗ
  (`task.create/update/delete/grant/addComment/...`) — không throw ngay như `project.*`, nhưng
  userId rỗng khiến `TaskGrantService`/quyền sở hữu resolve sai cho user thật, hỏng theo cách âm
  thầm hơn (không phải lúc nào cũng lộ ra bằng lỗi UNAUTHENTICATED rõ ràng).

**Vì sao không ai thấy trước đây**: hầu hết call site frontend bọc RPC bằng `.catch(() => null)`
(ví dụ `WorkspaceContext.tsx`'s `profile.getResolved`) — lỗi bị nuốt âm thầm, `resolvedProfile`
chỉ đơn giản luôn `null`, không có gì "vỡ" nhìn thấy được. `ProjectSwitcher`/`CreateProjectDialog`
(Giai đoạn 2c) là UI ĐẦU TIÊN hiển thị lỗi RPC trực tiếp cho người dùng thay vì nuốt âm thầm — đó
là lý do bug này lần đầu tiên "lộ" ra, không phải vì nó mới phát sinh.

## Fix

1. `backend/src/main/runtime/runtime-rpc.ts`:
   - `OrcaRuntimeRpcServerOptions` thêm field `userId?: string`.
   - Constructor lưu vào `this.userId`.
   - `handleMessage()`: `dispatchStreaming(request, reply, { signal, clientId: request.authToken,
     userId: this.userId })`.
2. `backend/src/main/server-bootstrap.ts`: khởi tạo `OrcaRuntimeRpcServer` với
   `userId: process.env['ORCA_USER_ID']` (đã có sẵn từ `SessionManager`, không cần thêm wiring
   mới — chỉ đọc ra và truyền vào).

**Vì sao an toàn**: `this.userId` cố định tại thời điểm construct process (từ env var, không phải
từ nội dung request) — không request nào có thể tự xưng userId khác. Socket `/data/orca/users/
<userId>/orca.sock` đã có quyền 0600 + path scope theo đúng userId đó, chỉ `WsSessionRouter` (đã tự
xác thực session trước khi proxy) hoặc chính CLI của user đó mới kết nối được. Không mở thêm bề mặt
tấn công nào — chỉ sửa để request hợp lệ không còn bị từ chối sai.

**Chế độ single-user/desktop không đổi hành vi**: `process.env['ORCA_USER_ID']` không được set
ngoài multi-user mode (không có `SessionManager` fork) → `userId` vẫn `undefined` như trước, không
regression.

## Giới hạn của lượt sửa này

Không viết được test tự động — import `runtime-rpc.ts` trong sandbox hiện tại kéo theo `node-pty`
native module chưa build cho `linux-x64` trong môi trường test, lỗi ngay ở module-load time trước
khi chạy được dòng test nào (không do cách viết test, đã thử cô lập import). Đây có thể là lý do
`runtime-rpc.ts` chưa từng có test coverage nào trong repo (0 file test tồn tại trước lượt sửa
này). Verify thay thế: đọc trực tiếp source xác nhận root cause, tsc sạch (0 lỗi mới, so sánh qua
git worktree), test suite hiện có không đổi (117/117), và **verify sống sau deploy** (xem log
production không còn `UNAUTHENTICATED` cho các method này, và thao tác Create Project thật thành
công).

## Follow-up cần cân nhắc

- Xác nhận thêm `task.*`/các RPC method khác có gate `ctx.userId` tương tự đã hoạt động sai trước
  đây, giờ hoạt động đúng — cần 1 vòng test thủ công/E2E rộng hơn qua các tính năng v5.0 khác đã
  build (Team, Task pipeline) vì TẤT CẢ đều có thể đã bị ảnh hưởng bởi cùng gốc bệnh này.
- Cân nhắc bỏ bớt các `.catch(() => null)` silent-swallow ở frontend cho các RPC quan trọng (ít
  nhất log ra console) — nếu không, lớp lỗi tương tự trong tương lai sẽ tiếp tục bị che giấu y hệt
  như lần này.
- Viết test cho `runtime-rpc.ts` cần môi trường CI có `node-pty` build sẵn cho target platform,
  hoặc mock `node-pty` ở mức module trước khi import — việc riêng, ngoài phạm vi fix này.
