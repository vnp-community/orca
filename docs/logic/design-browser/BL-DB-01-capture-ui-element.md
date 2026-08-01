# BL-DB-01 — Capture UI Element trong Browser

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-DB-01 |
| **Tên** | Capture UI Element Context |
| **Nhóm** | Design & Browser |
| **Actors** | Alex (Senior Dev), QA Engineer |
| **Ưu tiên** | P1 — Should Have |
| **Tính năng** | F05 Design Mode |
| **SRS** | FR-7.1 |

---

## Mô tả nghiệp vụ

Trong Design Mode, người dùng click vào bất kỳ UI element nào trong embedded browser — hệ thống tự động capture HTML, CSS, và screenshot của element đó.

---

## Luồng chính

```
1. Người dùng navigate tới URL trong embedded browser
2. Click "Design Mode" button → cursor chuyển sang inspect mode
3. Hover: highlight element (blue border overlay)
4. Click vào element:
   a. Capture outer HTML (element + relevant ancestors)
   b. Capture computed CSS styles
   c. Capture screenshot crop (viewport với padding)
5. Hiển thị preview panel:
   - HTML tab
   - CSS tab
   - Screenshot tab
6. Auto-inject vào agent prompt (BL-DB-02)
```

---

## Captured Data Structure

```typescript
interface ElementCapture {
  html: string;           // Outer HTML với ancestors
  css: {
    selector: string;
    rules: Record<string, string>;  // computed styles
  }[];
  screenshot: {
    data: string;         // base64 PNG
    rect: DOMRect;        // element position/size
  };
  url: string;            // page URL
  timestamp: Date;
}
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-DB-01 | CSS capture chỉ lấy non-inherited và computed styles (không tất cả) |
| BR-DB-02 | Screenshot crop có padding 20px quanh element |
| BR-DB-03 | HTML được sanitize (xóa script tags, event handlers inline) |
| BR-DB-04 | Capture phải hoàn thành trong < 500ms để không interrupt UX |
