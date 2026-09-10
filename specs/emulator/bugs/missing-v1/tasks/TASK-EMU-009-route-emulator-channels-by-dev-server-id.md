# TASK-EMU-009: Định tuyến `emulator.*` theo `mobileEmulatorAgentId`

**Solution:** [SOL-EMU-006](../solutions/SOL-EMU-006-project-binding-and-routing.md)
**Priority:** P2
**Depends on:** TASK-EMU-008
**Status:** `[x]` DONE

## Việc đã làm

`backend-go/services/api-gateway/internal/adapter/wscompat/channels_emulator_folderworkspace_host.go`:

1. **Đổi wire contract của mọi channel `emulator.*`**: tham số đầu vào từ
   client đổi từ `connectionId` (kiểu git/worktree, sai ngữ nghĩa cho Mobile
   Emulator Agent — agent này không có khái niệm repo/worktree) sang
   `projectId`. Hàm mới `resolveEmulatorConnectionID(ctx, id, projectClient,
   infraClient, projectID)` thực hiện 2 bước:
   - `projectClient.GetProject(projectId)` → đọc
     `project.mobileEmulatorAgentId`
   - `infraClient.ResolveConnection(&ResolveConnectionRequest{DevServerId:
     mobileEmulatorAgentId})` → `connectionId` thật

   Mọi handler (`emulator.listDevices/availability/attach/tap/gesture/
   button/rotate/shutdown`) gọi hàm này trước, rồi dùng `connectionId` TRẢ
   VỀ (không phải giá trị client gửi) cho RPC `infra-fleet-service` phía
   sau — test `TestRegisterEmulatorChannels_WithProjectID_ResolvesAndRelays`
   xác nhận rõ: `connectionId` forward xuống là giá trị
   `ResolveConnection` trả về (`"resolved-conn-1"`), không phải `projectId`
   client gửi (`"proj-1"`).

2. **Giữ đường đi cũ (honest stub) hoạt động đúng** — `resolveEmulatorConnectionID`
   trả `("", nil)` (không phải lỗi) cho 3 trường hợp, tất cả đều rơi vào
   nhánh `errEmulatorNotSupported` sẵn có (không đổi logic đó):
   - `projectId` rỗng (test
     `TestRegisterEmulatorChannels_NoProjectID_ReturnsHonestNotSupportedError`
     — 7 channel, y hệt bộ test cũ, chỉ đổi tên phản ánh "no projectId"
     thay vì "no connectionId")
   - project tồn tại nhưng `mobileEmulatorAgentId` rỗng (test mới
     `TestRegisterEmulatorChannels_ProjectWithNoMobileEmulatorAgentID_ReturnsHonestNotSupportedError`)
   - `ResolveConnection` trả `connectionId` rỗng (dev server không có kết
     nối sống) — cùng nhánh, không cần test riêng vì logic y hệt trường hợp
     trên (return "" từ `resolveResp.GetConnectionId()`)

   Một lỗi RPC thật (`GetProject`/`ResolveConnection` fail) propagate
   nguyên trạng, không bị nuốt thành honest stub.

3. `emulator.availability` giữ đúng hành vi "luôn relay kể cả không resolve
   được connectionId" (không có nhánh honest-stub riêng của wscompat — để
   `infra-fleet-service`'s `GetEmulatorAvailability` tự trả lời
   `available=false` với connectionId rỗng, đúng thiết kế cũ) — test
   `TestRegisterEmulatorChannels_Availability_AlwaysRelaysEvenWithoutProjectID`.

4. `usecase.EmulatorRelay` ở `infra-fleet-service`: **không sửa gì** — đúng
   như dự đoán trong SOL-EMU-006, `resolveDevServer`/`callAgent` vốn đã
   generic theo `connectionId` + method string, không quan tâm connectionId
   đến từ nguồn nào.

5. Test file `channels_emulator_folderworkspace_host_test.go` viết lại toàn
   bộ phần `emulator.*`: `fakeEmulatorHostClient` thêm mock
   `ResolveConnection`; `fakeProjectClient` (đã có sẵn cho
   `folderWorkspace.*` test) thêm mock `GetProject`. 4 test case (2 cũ đổi
   tên phản ánh contract mới + 2 mới) đều pass.

## Verify (chạy thật trong pass này, có bằng chứng)

```
$ cd backend-go/services/api-gateway && go build ./... && go vet ./...
(exit 0, no output)

$ go test ./internal/adapter/wscompat/... -run 'TestRegisterEmulatorChannels' -v
=== RUN   TestRegisterEmulatorChannels_NoProjectID_ReturnsHonestNotSupportedError
    (7 sub-test: attach/button/gesture/listDevices/rotate/shutdown/tap) --- PASS (mỗi cái)
--- PASS: TestRegisterEmulatorChannels_NoProjectID_ReturnsHonestNotSupportedError (0.00s)
=== RUN   TestRegisterEmulatorChannels_ProjectWithNoMobileEmulatorAgentID_ReturnsHonestNotSupportedError
--- PASS (0.00s)
=== RUN   TestRegisterEmulatorChannels_WithProjectID_ResolvesAndRelays
--- PASS (0.00s)
=== RUN   TestRegisterEmulatorChannels_Availability_AlwaysRelaysEvenWithoutProjectID
--- PASS (0.00s)
PASS
ok  	github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat	0.008s

$ go test ./...   # toàn bộ api-gateway
ok  	.../adapter/authclient
ok  	.../adapter/httpgateway
ok  	.../adapter/wsbridge
ok  	.../adapter/wscompat   (24.492s)
ok  	.../usecase
```

## Chưa làm (ngoài phạm vi backend-go này)

Frontend chưa gọi `emulator.*` qua `{ kind: 'remote' }`/wscompat ở web mode
— mọi lời gọi hiện tại (`frontend/src/renderer/src/components/emulator-pane/**`)
dùng `{ kind: 'local' }` (Electron desktop runtime), không đi qua
`api-gateway` — xác nhận bằng grep, không có nơi nào trong `frontend/src`
gọi `emulator.*` kèm `connectionId`/`projectId` qua remote channel. Nghĩa là
thay đổi wire contract trong task này (đổi `connectionId` → `projectId`)
AN TOÀN — không có caller thật nào đang dùng shape cũ để phá. Nối frontend
thật vào `emulator.*` (kèm `projectId`) là Phase 5, chưa bắt đầu.
