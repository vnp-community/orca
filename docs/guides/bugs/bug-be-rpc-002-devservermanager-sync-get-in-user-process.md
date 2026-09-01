# BUG-BE-RPC-002: `DevServerManager.get()` đồng bộ luôn throw trong User Process (multi-user web mode)

**Phát hiện:** 2026-08-14, ngay sau khi deploy fix cho [BUG-BE-RPC-001](./bug-be-rpc-001-userid-not-forwarded.md)
— vừa hết lỗi `UNAUTHENTICATED`, `project.create` lộ ra lỗi kế tiếp:
`Error: Synchronous get() not supported in User Process`.

## Root cause

`backend/src/main/dev-server/gateway-proxy.ts`'s `GatewayDevServerManagerProxy` — object thay thế
`DevServerManager` thật trong mỗi user-process (`server-bootstrap.ts`: `if (options.isUserProcess)
{ devServerManager = new GatewayDevServerManagerProxy() as unknown as DevServerManager }`, vì mỗi
user-process không có kết nối trực tiếp tới dev server, phải round-trip IPC qua main process) —
`get(id)` **luôn throw**, có chủ đích, comment sẵn trong code: *"Synchronous get is trickier via
IPC... For now we'll throw... The RPC layer uses `list` and `connect`."* `list()`/`connect()` đã
được implement async đúng cách; chỉ `get()` bị bỏ lại dạng throw-stub.

## Đã sửa (phạm vi: unblock Create Project)

`ProjectService.create()`/`update()` (`backend/src/main/project/ProjectService.ts`) gọi
`devServerManager.get(devServerId)` đồng bộ chỉ để validate tồn tại — đổi sang
`await devServerManager.list()` rồi `.find()` theo id. An toàn ở cả 2 chế độ: `DevServerManager`
thật's `list()` trả mảng đồng bộ (await một giá trị không phải Promise chỉ resolve ngay lập tức),
proxy's `list()` là `Promise<DevServer[]>` thật.

## ⚠️ Còn nhiều chỗ khác gọi `.get()` đồng bộ tương tự — CHƯA sửa, cần theo dõi

Grep xác nhận các chỗ khác vẫn còn nguyên lỗi này (sẽ throw hệt vậy nếu chạy trong User Process
mode và đường code đó được thực thi):

| File | Dòng | Flow bị ảnh hưởng |
|---|---|---|
| `project/ProjectServerRouter.ts` | 42, 72 | `getProjectContext()` — dùng bởi `project.agentSpawn`, agent spawn nói chung |
| `ipc/repo-remote-ipc.ts` | 75, 106 | Repo remote IPC handlers |
| `ipc/onboarding-ipc.ts` | 278 | Onboarding flow |
| `runtime/rpc/methods/dev-server.ts` | 114 | RPC method `devServer.get` chính nó — mỉa mai là gọi `ctx.devServerManager.get()` trực tiếp |
| `ai-providers/AIProviderService.ts` | 309, 364, 433, 479 | AI provider account CRUD gắn với 1 dev server cụ thể |

**Chưa sửa các chỗ này** vì ngoài phạm vi "Create Project" người dùng đang test — sửa vội có rủi ro
side-effect ở các flow chưa được yêu cầu kiểm tra. Đề xuất: làm 1 lượt riêng, theo đúng cùng cách
(`get(id)` → `(await list()).find(s => s.id === id)`), verify từng chỗ bằng test có sẵn nếu có,
hoặc thêm test mới nếu chưa. `ProjectServerRouter.ts` nên ưu tiên trước (ảnh hưởng agent spawn —
tính năng lõi), `AIProviderService.ts` sau đó.

## Follow-up cân nhắc lâu dài

Cân nhắc thêm `get()` async thật vào `GatewayDevServerManagerProxy` (round-trip IPC như
`list()`/`connect()` đã làm) thay vì thay từng call site — giải quyết tận gốc, nhưng đổi type
signature `get()` từ sync sang async sẽ cascade sang MỌI call site sync hiện có (kể cả những chỗ
dùng `DevServerManager` thật, không phải proxy) — việc lớn hơn, cần thiết kế riêng, không làm vội.
