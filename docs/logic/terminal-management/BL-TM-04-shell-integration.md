# BL-TM-04 — Shell Integration (OSC 133)

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-TM-04 |
| **Tên** | Shell Integration — OSC 133 Command Tracking |
| **Nhóm** | Terminal Management |
| **Actors** | Alex, Maya |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F02 Terminal Splits |
| **SRS** | FR-3.4 |

---

## Mô tả nghiệp vụ

Theo dõi các command trong terminal qua OSC 133 escape sequences — hiển thị exit code, thời gian thực thi, và navigation giữa các commands.

---

## OSC 133 Sequence Protocol

| Sequence | Ý nghĩa |
|---------|---------|
| `OSC 133 ; A ST` | Command start (shell prompt) |
| `OSC 133 ; B ST` | Command line start (user input area) |
| `OSC 133 ; C ST` | Command executed (output start) |
| `OSC 133 ; D ; <exit-code> ST` | Command finished |

---

## Luồng chính

```
1. Shell autoconfigure OSC 133 (via bootstrap script)
2. Khi người dùng run command:
   a. Detect OSC 133 A → đánh dấu start của command block
   b. Detect OSC 133 C → command đang chạy
   c. Detect OSC 133 D;<code> → command xong, hiển thị exit code
3. UI hiển thị:
   - ✅ Exit code 0
   - ❌ Exit code ≠ 0 (màu đỏ với code)
   - ⏱ Thời gian thực thi
4. Người dùng có thể jump tới command trước/sau
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-TM-13 | Shell integration là opt-in, không bắt buộc |
| BR-TM-14 | PowerShell cần bootstrap script để inject OSC 133 |
| BR-TM-15 | OSC sequences phải được strip khỏi copy-to-clipboard |
