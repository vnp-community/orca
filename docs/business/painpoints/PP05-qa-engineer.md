# PP05 — Painpoints: QA Engineer

| Thuộc tính | Giá trị |
|-----------|---------|
| **Actor ID** | PP05 |
| **Actor** | QA Engineer |
| **Đại diện** | QA Engineer kiểm thử UI và automation test |
| **Quote** | *"Tôi cần test UI mà agent tạo ra mà không cần phải setup lại môi trường từ đầu mỗi lần."* |
| **Tham chiếu giải pháp** | [SOL05](../solutions/SOL05-qa-engineer.md) |

---

## Bối cảnh

QA Engineer chịu trách nhiệm kiểm thử chất lượng code mà AI agent tạo ra. Với tốc độ AI sinh code ngày càng nhanh, QA phải test nhiều hơn và thường xuyên hơn. Đặc biệt với UI changes, việc test thủ công trở nên bottleneck của cả pipeline.

---

## Danh sách Painpoints

### PP05-01: Thiếu Công Cụ Để Capture UI Context Cho Test Case

**Mức độ nghiêm trọng:** 🟠 High  
**Tần suất:** Mỗi khi agent tạo UI change  

**Mô tả:**
Khi agent thay đổi UI, QA cần document test case với element selectors, expected appearance, và behavior. Hiện tại phải mở DevTools thủ công, tìm selector, copy HTML, chụp screenshot riêng. Rất tốn thời gian và dễ bỏ sót.

**Biểu hiện cụ thể:**
- Phải mở browser DevTools riêng để inspect element
- Copy selector thủ công, hay copy sai (tên class thay đổi theo build)
- Screenshot chỉ chụp được toàn màn hình, không crop đúng element
- Test case documentation không đủ chi tiết → test miss cases

---

### PP05-02: Không Có Cách Giao Task Sửa Bug Cho Agent Với Context Đầy Đủ

**Mức độ nghiêm trọng:** 🟠 High  
**Tần suất:** Mỗi khi phát hiện bug UI  

**Mô tả:**
Khi QA phát hiện bug UI, phải mô tả bug bằng lời cho agent: "Button ở trang X không responsive trên mobile". Agent không có context về selector, CSS, và actual vs expected behavior. Phải mô tả rất chi tiết để agent hiểu đúng.

**Biểu hiện cụ thể:**
- Mô tả bug bằng lời mơ hồ → agent fix sai chỗ 40% trường hợp
- Phải attach screenshot thủ công, không có element-level context
- Agent không biết expected design spec là gì
- Cần 2-3 vòng feedback để agent fix đúng bug đơn giản

---

### PP05-03: Môi Trường Test Không Nhất Quán Giữa Các Worktree

**Mức độ nghiêm trọng:** 🟡 Medium  
**Tần suất:** Khi test nhiều worktrees song song  

**Mô tả:**
Khi QA test nhiều worktrees cùng lúc (từ parallel agent chạy), mỗi worktree có thể chạy dev server trên port khác nhau. Không có cách clear biết worktree nào đang serve port nào.

**Biểu hiện cụ thể:**
- Không biết localhost:3000 là worktree A hay worktree B
- Phải track thủ công port assignment trong notes
- Hay test nhầm worktree → kết quả test sai

---

### PP05-04: Không Có Computer Use Để Automate Repetitive UI Test

**Mức độ nghiêm trọng:** 🟡 Medium  
**Tần suất:** Regression test sau mỗi agent change  

**Mô tả:**
QA muốn dùng agent để chạy automated UI testing — click through user flows, fill forms, verify visual output. Không có cơ chế để agent tương tác với desktop UI apps.

**Biểu hiện cụ thể:**
- Playwright/Selenium test cần setup riêng, không tích hợp với agent
- Không thể dùng agent để test desktop apps (Electron, native)
- Manual regression test tốn 2-4 giờ mỗi lần có major change

---

## Tổng hợp Impact

| Painpoint | Mức độ | Thời gian mất/ngày | Tần suất |
|-----------|--------|-------------------|---------|
| PP05-01: Capture UI context cho test | 🟠 High | 30-60 phút | Mỗi UI change |
| PP05-02: Giao bug với context đủ | 🟠 High | 20-40 phút | Mỗi bug phát hiện |
| PP05-03: Môi trường không nhất quán | 🟡 Medium | 10-20 phút | Khi test song song |
| PP05-04: Không có UI test automation | 🟡 Medium | 2-4 giờ | Mỗi major change |

---

*Tham chiếu: URD §2.2 (QA Engineer), PRD §3.4 (Design Mode), §3.10 (Computer Use)*
