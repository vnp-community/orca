# BL-DB-03 — Viewport Testing

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-DB-03 |
| **Tên** | Viewport Testing trong Embedded Browser |
| **Nhóm** | Design & Browser |
| **Actors** | QA Engineer, Alex |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F05 Design Mode |
| **SRS** | FR-7.1 |

---

## Mô tả nghiệp vụ

Thay đổi kích thước viewport của embedded browser để test responsive design ở nhiều breakpoints — mobile, tablet, desktop.

---

## Viewport Presets

| Preset | Width | Height | Device |
|--------|-------|--------|--------|
| Mobile S | 320 | 568 | iPhone SE |
| Mobile M | 375 | 667 | iPhone 12 |
| Mobile L | 414 | 896 | iPhone XR |
| Tablet | 768 | 1024 | iPad |
| Laptop | 1280 | 800 | MacBook |
| Desktop | 1440 | 900 | — |
| Custom | User-defined | — | — |

---

## Luồng chính

```
1. Người dùng chọn viewport preset từ toolbar
2. Hoặc nhập custom dimensions
3. Browser resize tức thì
4. Responsive design reflow
5. Người dùng test ở kích thước mới
6. Có thể capture element ở viewport này (BL-DB-01)
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-DB-08 | Viewport change phải xảy ra tức thì (< 100ms) |
| BR-DB-09 | Custom viewport không được vượt quá window size |
| BR-DB-10 | Viewport dimensions được lưu vào session (không reset khi navigate) |
