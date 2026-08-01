# PP03 — Painpoints: Remote Developer (Carlos)

| Thuộc tính | Giá trị |
|-----------|---------|
| **Actor ID** | PP03 |
| **Actor** | Remote Developer |
| **Đại diện** | Carlos — Freelancer, làm việc trên nhiều dự án từ laptop |
| **Quote** | *"Máy local của tôi yếu. Tôi cần chạy agent trên server mạnh nhưng vẫn kiểm soát từ laptop."* |
| **Tham chiếu giải pháp** | [SOL03](../solutions/SOL03-remote-developer.md) |

---

## Bối cảnh

Carlos là freelancer làm việc trên nhiều dự án cùng lúc. Laptop local không đủ RAM/CPU để chạy nhiều AI agent song song với các build tools nặng (Docker, large ML models). Carlos phải thuê cloud server mạnh hơn, nhưng việc quản lý remote development environment cực kỳ phức tạp và không ổn định.

---

## Danh sách Painpoints

### PP03-01: SSH Session Mất Kết Nối Liên Tục

**Mức độ nghiêm trọng:** 🔴 Critical  
**Tần suất:** 5-10 lần/ngày  

**Mô tả:**
SSH session bị drop khi: laptop sleep, WiFi chuyển mạng, NAT timeout, VPN reconnect. Mỗi lần drop, Carlos phải reconnect thủ công, tmux session có thể bị mất, agent process bị kill.

**Biểu hiện cụ thể:**
- Mất kết nối trung bình 5-10 lần/ngày khi làm việc di chuyển
- Phải reconnect thủ công: `ssh user@host` → attach tmux → tìm lại context
- Mỗi reconnect tốn 2-5 phút
- Agent bị killed khi SSH drop → mất progress nếu không dùng tmux
- tmux session expire sau 8 giờ trên một số server

**Chi phí ước tính:** 10-50 phút/ngày bị gián đoạn do SSH drop

---

### PP03-02: Setup Môi Trường Remote Phức Tạp và Dễ Break

**Mức độ nghiêm trọng:** 🔴 Critical  
**Tần suất:** Mỗi khi đổi server hoặc server được rebuild  

**Mô tả:**
Mỗi khi setup môi trường mới trên remote server, Carlos phải thực hiện hàng chục bước thủ công: install Node.js đúng version, cài AI agent CLI tools, configure PATH, setup SSH keys, clone repo. Bất kỳ bước nào fail là phải debug.

**Biểu hiện cụ thể:**
- Mỗi lần setup môi trường mới tốn 2-4 giờ
- Node.js version mismatch với project requirements
- AI agent CLI không installed hoặc outdated trên server mới
- PATH không configure đúng → agent không tìm được binary
- Khi server bị rebuild (snapshot restore, OS upgrade) → phải setup lại từ đầu

**Chi phí ước tính:** 2-4 giờ mỗi lần setup, 1-2 lần/tháng

---

### PP03-03: Port Forwarding Thủ Công Cồng Kềnh

**Mức độ nghiêm trọng:** 🟠 High  
**Tần suất:** Mỗi ngày, nhiều lần  

**Mô tả:**
Khi agent khởi động dev server trên remote (port 3000, 8080), Carlos phải thiết lập port forwarding thủ công bằng command line: `ssh -L 3000:localhost:3000 user@server`. Phải biết trước port nào sẽ được mở. Nếu quên forward thì không test được.

**Biểu hiện cụ thể:**
- Phải nhớ và gõ SSH tunnel command mỗi khi cần forward
- Hay quên forward → mở browser → connection refused → phải reconnect
- Nhiều agent mở nhiều port → cần nhiều tunnel command khác nhau
- Khi SSH session drop, tunnel cũng drop → phải re-establish

**Chi phí ước tính:** 15-30 phút/ngày setup và re-setup port forwarding

---

### PP03-04: Không Có File Editing Trực Tiếp Trên Remote

**Mức độ nghiêm trọng:** 🟠 High  
**Tần suất:** Nhiều lần mỗi ngày  

**Mô tả:**
Carlos muốn xem và sửa file trực tiếp trên remote server từ laptop. Các giải pháp hiện tại không ổn: `nano`/`vim` trong SSH (clunky), SFTP mount (lag cao), VSCode Remote SSH (tốt nhưng cần VSCode, không tích hợp với agent workflow).

**Biểu hiện cụ thể:**
- SFTP mount lag 1-3 giây mỗi keystroke khi kết nối chậm
- VSCode Remote SSH không share context với AI agent terminal
- Phải mở 2 ứng dụng riêng: VSCode (edit) và terminal (agent)
- Thao tác file thủ công qua vim trong SSH làm chậm workflow

---

### PP03-05: Không Biết Trạng Thái Agent Khi Mất Kết Nối

**Mức độ nghiêm trọng:** 🟠 High  
**Tần suất:** 5-10 lần/ngày  

**Mô tả:**
Khi SSH drop, Carlos không biết agent đang làm gì trên server: đang chạy, đã xong, hay gặp lỗi và đang chờ input. Phải reconnect để check — nhưng reconnect có thể mất vài phút.

**Biểu hiện cụ thể:**
- Reconnect để check → agent đã xong từ lâu nhưng không biết
- Reconnect → agent đang chờ input → đã chờ 10 phút không ai response
- Không có cách biết agent status khi offline
- Hay reconnect nhiều lần không cần thiết chỉ để check

---

### PP03-06: Nhiều Dự Án Trên Nhiều Server, Khó Quản Lý

**Mức độ nghiêm trọng:** 🟡 Medium  
**Tần suất:** Hằng ngày  

**Mô tả:**
Carlos làm việc trên 3-5 client projects cùng lúc, mỗi project trên một remote server khác nhau. Phải nhớ hostname, credentials, và current state của mỗi server. Không có unified dashboard.

**Biểu hiện cụ thể:**
- Không nhớ hostname của các server → phải tìm trong notes
- Nhầm lẫn giữa dev server và production server
- Không biết server nào đang có agent chạy
- Phải mở nhiều terminal tab, mỗi tab một SSH session

---

### PP03-07: Git Operations Chậm Qua SSH Relay

**Mức độ nghiêm trọng:** 🟡 Medium  
**Tần suất:** Nhiều lần mỗi ngày  

**Mô tả:**
Git fetch, clone, và push từ remote server thường chậm hơn local vì phụ thuộc vào network từ server tới GitHub. Carlos không thể kiểm soát network path của server.

**Biểu hiện cụ thể:**
- `git fetch` tốn 5-15 giây thay vì 1-2 giây trên local
- Large repo clone tốn hàng chục phút trên server với uplink yếu
- Push lớn bị timeout → phải retry
- Không có visual progress khi git operation đang chạy

---

## Tổng hợp Impact

| Painpoint | Mức độ | Thời gian mất/ngày | Tần suất |
|-----------|--------|-------------------|---------|
| PP03-01: SSH drop liên tục | 🔴 Critical | 10-50 phút | 5-10 lần/ngày |
| PP03-02: Setup môi trường | 🔴 Critical | 2-4 giờ | 1-2 lần/tháng |
| PP03-03: Port forwarding thủ công | 🟠 High | 15-30 phút | Hằng ngày |
| PP03-04: Không có file editor remote | 🟠 High | 20-40 phút | Hằng ngày |
| PP03-05: Không biết agent status khi offline | 🟠 High | 10-20 phút | 5-10 lần/ngày |
| PP03-06: Quản lý nhiều server | 🟡 Medium | 10-20 phút | Hằng ngày |
| PP03-07: Git operations chậm | 🟡 Medium | 5-15 phút | Nhiều lần/ngày |

**Tổng thời gian lãng phí ước tính:** **1-3 giờ/ngày** — tương đương 15-35% năng suất của Carlos

---

## Nguyên nhân gốc rễ

1. **Không có abstraction layer giữa local UI và remote execution** — developer phải trực tiếp quản lý SSH session
2. **SSH protocol không được thiết kế cho interactive development** — không có auto-reconnect, không có state persistence
3. **Tool ecosystem không tích hợp với nhau** — editor, terminal, agent, port forwarding đều riêng biệt
4. **Không có visibility vào remote agent khi không có kết nối** — true offline-first monitoring thiếu

---

*Tham chiếu: URD §2.1 (Persona Carlos), §3.3 (UR-013, UR-020 đến UR-022)*
