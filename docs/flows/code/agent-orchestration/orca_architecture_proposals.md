# Đề Xuất Cải Tiến Kiến Trúc & Trải Nghiệm (Desktop vs Web Mode)

Dựa trên phân tích về cơ chế hoạt động của Orca IDE hiện tại (đặc biệt là sự khác biệt giữa Desktop App và Web SPA kết nối qua Dev Server), dưới đây là các đề xuất thay đổi nhằm tối ưu hóa UX/UI, giải quyết sự nhầm lẫn về mặt khái niệm và tăng cường tính năng hỗ trợ kết nối Remote.

---

## 1. Cải tiến UI/UX về thuật ngữ (Terminology) theo ngữ cảnh (Context-Aware UI)

Vấn đề lớn nhất hiện tại là sự nhập nhằng của từ **"Local"** trong chế độ Web Mode. Người dùng web thường hiểu "Local" là máy tính cá nhân của họ, nhưng thực tế nó lại là ổ cứng của Dev Server.

**Đề xuất:** Thay đổi nhãn (label) hiển thị trong mục `Add a project -> Host` tùy thuộc vào môi trường (Platform Environment).

- **Khi chạy trên Desktop App (Electron/Native):**
  - Cấu trúc giữ nguyên: **"Local Machine"** (Máy cá nhân), **"SSH"** (Kết nối từ máy cá nhân tới Server), **"Orca Session / Pair"**.
  
- **Khi chạy trên Web Mode (Trình duyệt):**
  - Đổi tên "Local" thành **"Dev Server (Host)"** hoặc **"Cloud Workspace"**.
  - Bổ sung chú thích (tooltip/subtext) nhỏ: *Thư mục nằm trên máy chủ đang chạy Orca Backend.*
  - Chuyển "SSH" thành **"SSH (Relay via Dev Server)"** để người dùng hiểu rằng quá trình kết nối SSH xuất phát từ Dev Server chứ không phải từ trình duyệt máy tính của họ (điều này liên quan trực tiếp đến việc cấu hình SSH Key).

## 2. Hỗ trợ Local Folder thực sự trên Web Mode (Trình duyệt)

**Vấn đề:** Nhiều người dùng muốn mở một thư mục trên laptop cá nhân của họ ngay trên trình duyệt mà không cần cài đặt Desktop App hay chạy Dev Server nội bộ.

**Đề xuất:**
- Tích hợp **HTML5 File System Access API**. 
- Thêm một option trong mục Host (chỉ hiển thị ở Web Mode) tên là: **"Browser Local (No Terminal)"**.
- **Cách hoạt động:** Trình duyệt sẽ xin quyền truy cập vào một thư mục trên máy tính người dùng. Orca Web có thể đọc/ghi trực tiếp vào ổ cứng người dùng mà không cần Dev Server. 
- **Hạn chế cần làm rõ:** Khi dùng chế độ này, người dùng sẽ không có Terminal (vì không có backend thực thi lệnh) và không chạy được Language Server (LSP) nặng. Nó chỉ đóng vai trò như một Text Editor thuần túy.

## 3. Quản lý SSH Keys và Xác thực trong môi trường Dev Server

**Vấn đề:** Khi dùng "Add remote host -> SSH" trên Web Mode, người dùng thường bị bối rối vì SSH Key phải nằm trên Dev Server (thay vì trên laptop cá nhân).

**Đề xuất:**
- Xây dựng luồng (flow) quản lý Credentials riêng cho Dev Server:
  - Cho phép người dùng upload an toàn Private Key từ máy cá nhân lên Dev Server thông qua WebSocket.
  - Hoặc tích hợp **SSH Agent Forwarding** qua WebSocket, cho phép Dev Server mượn SSH Key từ máy cá nhân của người dùng để kết nối tới Remote Host khác. Cơ chế này an toàn hơn vì Key không bao giờ rời khỏi laptop người dùng.

## 4. Tối ưu kiến trúc "Disconnected State" (Chống rớt mạng cho Web Mode)

**Vấn đề:** Môi trường Web phụ thuộc 100% vào WebSocket. Nếu mạng chập chờn, kết nối WebSocket bị đứt, người dùng có thể mất dữ liệu đang gõ dở hoặc không thể thao tác tiếp.

**Đề xuất:**
- **Offline Editing:** Lưu tạm trạng thái "dirty" (các thay đổi chưa save) vào `IndexedDB` của trình duyệt. 
- Khi WebSocket mất kết nối, UI hiện cảnh báo *"Reconnecting..."* nhưng **không khóa hoàn toàn Editor**. Người dùng vẫn có thể tiếp tục gõ code.
- Khi có mạng trở lại (WebSocket kết nối lại), hệ thống tự động đẩy (sync) bộ đệm từ `IndexedDB` lên Dev Server để lưu file. Nếu Dev Server thông báo file đã bị đổi bởi người khác trong lúc mất mạng, kích hoạt giao diện **Conflict Resolution** (như Git merge conflict).

## 5. Phân tách rõ ràng giữa "Compute" và "Storage" (Định hướng tương lai)

Thay vì Dev Server ôm đồm cả Storage (File system) và Compute (Terminal, LSP), Orca có thể tiến tới kiến trúc linh hoạt hơn:
- **Storage Host:** Nơi chứa Source Code (ví dụ Github Codespaces, Dev Server A, hoặc Browser Local).
- **Compute Host:** Nơi chạy terminal và build (ví dụ Server B có GPU hoặc cấu hình mạnh).
- **Lợi ích:** Người dùng có thể để Source Code ở một máy (thậm chí trên Browser Local), nhưng khi chạy lệnh `npm start`, lệnh đó được đẩy sang một Execution Server khác thông qua Dev Server điều phối. 

---

### Bảng Tóm Tắt (Summary Matrix)

| Tính Năng / Chế độ | Desktop Mode (Electron) | Web Mode (Trình duyệt) | Web Mode + File System API |
| :--- | :--- | :--- | :--- |
| **Bản chất "Local"** | Máy cá nhân của user | Ổ cứng của Dev Server | Máy cá nhân của user (qua Browser) |
| **Quyền truy cập File** | Native FS (Full access) | Native FS (trên Server) | Giới hạn bởi Browser Sandbox |
| **Kết nối SSH** | Trực tiếp từ máy cá nhân | Proxy qua Dev Server | Không hỗ trợ (hoặc cần extension) |
| **Terminal / Bash** | Hỗ trợ (Chạy local) | Hỗ trợ (Chạy trên Server) | ❌ Không hỗ trợ |
| **Độ trễ (Latency)** | Bằng 0 | Phụ thuộc mạng (Ping) | Bằng 0 (nhưng tính năng hạn chế) |
