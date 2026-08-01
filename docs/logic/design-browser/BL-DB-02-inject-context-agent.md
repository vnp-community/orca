# BL-DB-02 — Inject UI Context vào Agent Prompt

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-DB-02 |
| **Tên** | Inject UI Context vào Agent Prompt |
| **Nhóm** | Design & Browser |
| **Actors** | Alex, QA Engineer |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F05 Design Mode |
| **SRS** | FR-7.1 |

---

## Mô tả nghiệp vụ

Sau khi capture element (BL-DB-01), tự động format và inject context vào agent prompt field — người dùng chỉ cần thêm yêu cầu cụ thể.

---

## Luồng chính

```
1. Element captured (BL-DB-01)
2. Format context thành structured prompt:
   ---
   [UI Context]
   URL: https://app.example.com/login
   Element: button.submit-btn

   HTML:
   ```html
   <button class="submit-btn" type="submit">
     Sign In
   </button>
   ```

   CSS (relevant):
   background: #1a73e8;
   color: white;
   padding: 8px 16px;
   border-radius: 4px;
   
   [Screenshot attached]
   ---
3. Append vào agent prompt field (không overwrite)
4. Focus vào prompt field để người dùng thêm yêu cầu
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-DB-05 | Context phải được append, không replace prompt hiện tại |
| BR-DB-06 | Screenshot được encode base64 và inline (nếu agent hỗ trợ vision) |
| BR-DB-07 | Context format phải consistent để agent có thể parse |
