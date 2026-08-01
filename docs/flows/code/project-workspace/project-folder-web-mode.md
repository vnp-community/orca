# Kiến Trúc và Luồng Xử Lý Project Folder (Web Mode / WebSocket)

Tài liệu này mô tả chi tiết cơ chế hoạt động, truy vấn và quản lý thư mục dự án (Project Folder) của hệ thống Orca IDE khi hoạt động ở chế độ Web (Web SPA) kết nối với Dev Server thông qua giao thức WebSocket.

## 1. Tổng Quan Kiến Trúc (RPC over WebSocket)

Ở chế độ Web Mode, Frontend (trình duyệt) không có quyền truy cập trực tiếp vào hệ thống file (File System - FS) của người dùng do cơ chế bảo mật sandbox. Để giải quyết vấn đề này, kiến trúc được chia làm 2 phần giao tiếp qua **WebSocket**:

- **Thin Client (Web SPA):** Đóng vai trò hiển thị giao diện (File Explorer, Editor). Toàn bộ thao tác với file được đóng gói thành các lệnh RPC (Remote Procedure Call) và gửi đi.
- **Execution Host (Backend Dev Server):** Đóng vai trò File System Provider. Chạy trên máy chủ, nhận lệnh RPC, thực thi các thao tác đọc/ghi thông qua Native API của Hệ điều hành (Node.js `fs`) và trả kết quả về cho Frontend.

## 2. Setup và Cấu Hình Host

Giao diện `Add a project -> Host` quản lý việc lựa chọn không gian lưu trữ và thực thi (Execution Host). 

### Các lựa chọn Host:
- **Local:** Không phải là ổ cứng laptop của trình duyệt đang mở, mà là hệ thống file nội bộ (Local Disk) của **máy chủ đang chạy Dev Server**. Thư mục dự án nằm trực tiếp trên ổ cứng hoặc container của backend.
- **Orca Session (Pair):** Kết nối vào một không gian làm việc (workspace) đang hoạt động từ xa thông qua Dev Server relay trung gian.
- **SSH (Add remote host):** Dev Server đóng vai trò là một Proxy. Web Client gửi lệnh SSH qua WebSocket, Dev Server sẽ thiết lập kết nối SSH tới máy chủ đích. Thư mục dự án lúc này nằm trên máy chủ SSH đó.

## 3. Quá Trình Truy Vấn Dữ Liệu (File/Folder Query)

Khi người dùng mở một thư mục dự án, luồng truy vấn diễn ra như sau:

1. **Khởi tạo Request:** Frontend tạo một gói tin JSON-RPC yêu cầu đọc danh sách file (ví dụ: `{ "method": "fs.readDirectory", "args": ["/path/to/project"] }`).
2. **Gửi qua WebSocket:** Gói khởi tạo được gửi từ Frontend qua kết nối WebSocket (`ws://` hoặc `wss://`) đến `OrcaRuntimeRpcServer` (hoặc `AgentWebSocketServer`) ở Backend.
3. **Thực thi Backend:** Dev Server nhận gói tin, sử dụng API native (như `fs.promises.readdir` hoặc các lệnh tương tự qua luồng SSH) để đọc dữ liệu từ ổ cứng.
4. **Phản hồi:** Dev Server đóng gói danh sách các thư mục, file cùng metadata (kích thước, quyền truy cập) thành JSON và gửi trả lại Frontend.
5. **Render UI:** Frontend nhận dữ liệu và vẽ lại cây thư mục (File Tree Explorer) trên giao diện.

## 4. Cơ Chế Lắng Nghe Thay Đổi (File Watcher)

Để giao diện luôn đồng bộ với trạng thái thực tế của ổ cứng (ví dụ: file bị thay đổi do `git pull` từ terminal, hoặc xóa file từ bên ngoài), hệ thống sử dụng cơ chế **Server-Push**:

1. **Đăng ký Watcher:** Khi Project Folder được mở, Dev Server tự động đăng ký một File Watcher sử dụng API native của OS (thường thông qua thư viện như `chokidar` hoặc `fs.watch`) trên thư mục dự án đó.
2. **Phát hiện sự thay đổi:** Khi có bất kỳ sự kiện tạo mới (Create), chỉnh sửa (Update), hoặc xóa (Delete) file trên ổ cứng, Watcher ở Backend sẽ bắt được sự kiện này.
3. **Push Event:** Dev Server không chờ Frontend hỏi (không dùng cơ chế polling), mà chủ động đẩy (push) thông báo sự kiện (kèm đường dẫn file bị thay đổi) qua kết nối WebSocket xuống Web SPA.
4. **Cập nhật UI:** Frontend nhận sự kiện, tự động cập nhật lại File Tree Explorer hoặc hiển thị cảnh báo nếu file đang được mở trong Editor bị sửa đổi bởi một tiến trình khác.

## 5. Chỉnh Sửa và Cập Nhật File (Editor)

Luồng chỉnh sửa dữ liệu diễn ra theo cơ chế đồng bộ hóa chủ động:

1. **Đọc File:** Khi click mở file, Frontend gửi lệnh `fs.readFile` qua WebSocket. Backend đọc nội dung file dưới dạng text hoặc buffer và gửi về Frontend để hiển thị vào Editor.
2. **Chỉnh sửa trong bộ nhớ:** Người dùng gõ code, Frontend giữ trạng thái "dirty" (chưa lưu) trong bộ nhớ trình duyệt, tối ưu bằng các công nghệ như virtual DOM hoặc bộ đệm của IDE.
3. **Cập nhật (Save):** 
   - Khi người dùng nhấn Save (hoặc hệ thống Auto-save), Frontend đóng gói toàn bộ nội dung file mới (hoặc các khối diff/patch nếu được tối ưu) và gửi qua lệnh `fs.writeFile` tới Backend.
   - Backend sử dụng `fs.promises.writeFile` để ghi đè dữ liệu xuống ổ cứng thực.
   - Khi quá trình ghi thành công, Backend gửi tín hiệu xác nhận (ACK) về Frontend để trình duyệt xóa cờ "dirty".
   - File Watcher cũng có thể kích hoạt event Update ở bước này (nhưng Frontend thường sẽ bỏ qua/lọc event này nếu phát hiện ID hoặc cờ thay đổi là do chính thao tác save của nó).
