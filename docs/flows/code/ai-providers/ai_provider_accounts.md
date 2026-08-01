# Giải thích về AI Provider Accounts trong Orca

Dưới đây là phần giải thích chi tiết về cách hoạt động của AI Provider Accounts trong Orca, luồng xử lý khi bấm "Add account", nguyên nhân gây ra lỗi hiển thị `"System default -> System default"`, và hành vi đặc thù khi sử dụng Remote Dev Server qua Web mode.

## 1. Các AI Provider Accounts hoạt động thế nào?
Trong dự án Orca, tính năng **AI Provider Accounts** (nằm ở `AccountsPane.tsx` và `ClaudeAccountService.ts`) hoạt động như một trình quản lý phiên đăng nhập (Session Manager) cho các công cụ AI CLI (như `claude`). 
- **Cách ly (Isolation):** Thay vì chỉ dùng một config mặc định của hệ thống (nằm ở `~/.anthropic`), Orca cho phép bạn thêm nhiều tài khoản khác nhau. Mỗi tài khoản sẽ được Orca lưu cấu hình (credentials) vào một thư mục riêng biệt (`managedAuthPath`).
- **Chuyển đổi (Switching):** Bạn có thể gán các tài khoản khác nhau cho từng môi trường chạy (Host, WSL, Remote Server). Khi Orca khởi chạy các terminal cho AI CLI, nó sẽ tự động tiêm (inject) credentials tương ứng của tài khoản đã chọn vào môi trường đó để CLI gọi đúng API của tài khoản.
- **System Default:** Nếu bạn không chọn tài khoản nào (giá trị ID là `null`), hệ thống sẽ dùng "System default" – tức là config mặc định có sẵn trên máy của bạn (nếu có).

## 2. Luồng yêu cầu khi bấm vào "Add account" (Môi trường Local)
Khi bạn bấm nút "Add account" trên giao diện ở môi trường local, luồng sự kiện diễn ra qua các lớp của ứng dụng Electron như sau:

1. **Frontend (UI - Browser):** Nút bấm trong `AccountsPane.tsx` gọi hàm `window.api.claudeAccounts.add(...)` thông qua Context Bridge.
2. **IPC (Inter-Process Communication):** Lệnh được truyền qua IPC qua hàm `ipcRenderer.invoke('claudeAccounts:add')` trong file preload.
3. **Main Process (Node.js):** File `src/main/ipc/claude-accounts.ts` nhận IPC và gọi hàm `addAccount()` của `ClaudeAccountService` (nằm ở `src/main/claude-accounts/service.ts`).
4. **Execution (Chạy lệnh CLI):**
   - Orca tạo một ID mới và cấp phát một thư mục tạm (`tempConfig`) để tránh ghi đè cấu hình hiện tại của máy.
   - Nó gọi lệnh shell ẩn (spawn subprocess): `claude auth login --claudeai` với môi trường trỏ vào thư mục tạm này.
   - Lệnh CLI này của Anthropic sẽ **tự động mở Default Browser** (trình duyệt web mặc định của OS) của bạn để yêu cầu đăng nhập bằng luồng OAuth.
5. **Capture (Ghi nhận kết quả):**
   - Sau khi bạn đăng nhập thành công trên browser, lệnh CLI ẩn sẽ kết thúc.
   - Ngay lập tức, Orca gọi tiếp `claude auth status --json` trên cùng thư mục tạm đó để lấy thông tin (Email, Organization UUID...).
6. **Lưu trữ (Storage):** Orca copy file auth từ thư mục tạm vào thư mục quản lý chính thức của nó, rồi cập nhật danh sách `claudeManagedAccounts` vào Settings store nội bộ. Trả kết quả thành công về lại Frontend.

## 3. Web mode và Remote Dev Server: CLI chạy ở đâu?
Đối với môi trường **Web mode** và khi sử dụng **Remote dev server**, lệnh CLI (`claude auth login`) sẽ **KHÔNG chạy ở đâu cả** thông qua giao diện Web. 

Lý do là Orca đã **vô hiệu hóa (disable)** hoàn toàn nút **"Add Account"** khi bạn đang trỏ tới một Remote Server. 

### Tại sao lại Disable trong Web mode?
Bản chất của lệnh CLI `claude auth login` là một **Interactive Login** (đăng nhập tương tác). Khi lệnh này chạy, nó sẽ tìm cách mở một trình duyệt web mặc định của hệ điều hành để bắt đầu luồng đăng nhập OAuth.

Nếu Orca cho phép chạy lệnh này trên Remote Dev Server thông qua nút bấm trên Web mode:
- **Trường hợp 1 (Server Headless):** Máy chủ Linux trên cloud thường không có trình duyệt. Lệnh `claude auth login` sẽ thất bại (crash) hoặc bị treo do không thể mở được browser, hoặc chỉ in ra đường link URL mà giao diện Web Orca hiện tại không bắt được.
- **Trường hợp 2 (Server có giao diện màn hình):** Trình duyệt web sẽ được mở bung lên trên **màn hình vật lý của cái Server đó**. Người đang ngồi ở máy tính cá nhân (client) truy cập qua Web mode sẽ không nhìn thấy trang đăng nhập. 

### Cách hệ thống hoạt động thực tế
Vì những giới hạn kỹ thuật trên, luồng hoạt động dành cho Remote Dev Server được thiết kế theo hướng **quản lý cục bộ**:
- **Để thêm tài khoản (Add):** Bạn phải truy cập trực tiếp vào máy Remote Server đó (thông qua SSH, hoặc mở app Orca desktop trực tiếp trên máy đó) để cấu hình. Lệnh CLI sẽ chạy trực tiếp trên môi trường của server và sử dụng browser của server.
- **Trên Web mode:** Giao diện chỉ hiển thị thông báo *"Showing accounts managed by [Remote Server]"*. Nó chỉ đọc danh sách cấu hình đã được tạo sẵn trên Server và cho phép bạn **chọn (select)** giữa các account đã có sẵn, chứ không cho phép thêm mới (add).

## 4. Tại sao lại bị lỗi hiển thị "System default -> System default"?
Việc thông báo hiện lên dòng `"System default -> System default. Restart live Claude terminals before continuing old sessions."` **là một lỗi logic trong UI (bug hiển thị)** khi bạn add tài khoản thành công ở môi trường Local. 

Nguyên nhân cụ thể:

- **Ở Main Process (`service.ts`):** Khi một tài khoản mới được thêm vào thành công, Orca **cố tình không tự động chuyển (auto-select)** tài khoản đang dùng sang tài khoản mới. Tài khoản đang kích hoạt (active account) vẫn giữ nguyên như cũ (thường là `null` - ứng với "System default").
- **Ở Frontend (`AccountsPane.tsx`):**
  Khi API trả về thành công, code kiểm tra xem có cần hiện Toast thông báo Restart hay không:
  ```typescript
  const shouldPromptRestart =
    action === 'adding' ||
    previousActiveAccountId !== nextActiveAccountId || ...
  ```
  Do `action === 'adding'` (hành động là Add Account), cờ `shouldPromptRestart` luôn bị ép thành `true` bất chấp việc tài khoản Active có thay đổi hay không.

- **Ghép chuỗi thông báo:** Do cả tài khoản cũ (`previousActiveAccountId`) và tài khoản mới (`nextActiveAccountId`) **vẫn đang là `null`**, hàm sinh nhãn (label) trả về string `'System default'` cho cả hai bên.
  
👉 **Kết quả:** Toast UI hiển thị việc chuyển đổi trạng thái giữa hai account y hệt nhau: `System default -> System default`.

**Cách để fix lỗi này:**
Trong `AccountsPane.tsx`, nên bỏ điều kiện ép buộc `action === 'adding'` ra khỏi `shouldPromptRestart`, hoặc kiểm tra thêm: nếu `previousActiveAccountId === nextActiveAccountId` thì chỉ hiện thông báo dạng "Account Added Successfully" thay vì thông báo bắt Restart mang chuỗi mũi tên `A -> B`.

## 5. Đề xuất cải tiến thiết kế (Design Recommendation)
- **Ẩn tab AI Provider Accounts trong Web mode:** Vì trong chế độ web mode, tài khoản AI provider không được phép thay đổi (nút Add Account bị disable do giới hạn trình duyệt/remote CLI), tốt nhất là nên ẩn hẳn mục này đi để tránh gây bối rối cho người dùng.
- **Hỗ trợ tuỳ chọn hiển thị (Tương lai):** Nếu phần Settings thiết kế một cờ (flag) cho phép hiển thị mục này trong web mode, thì bắt buộc toàn bộ luồng thông tin tương tác (ví dụ khi thêm account) phải được thiết kế lại để gửi và thực thi trực tiếp trên Remote Dev Server (cần giải quyết được bài toán xác thực OAuth/CLI mà không phụ thuộc vào trình duyệt vật lý của remote server).
