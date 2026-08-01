# F05 — Design Mode

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F05 |
| **Tên** | Design Mode |
| **Ưu tiên** | P1 — Should Have |
| **Trạng thái** | ✅ Đã phát hành |
| **Tham chiếu PRD** | §3.4 |
| **Tham chiếu URD** | UR-050 |
| **Tham chiếu SRS** | FR-7.1 |
| **ADR References** | — |
| **HLD References** | C3.4 |

---

## Mô tả

Tích hợp browser Chromium thực trong IDE, cho phép người dùng click vào bất kỳ UI element nào để tự động trích xuất HTML, CSS, và screenshot — gửi thẳng vào agent prompt mà không cần copy/paste thủ công.

---

## Vấn đề cần giải quyết

Khi muốn agent sửa UI, developer phải tự mở DevTools, tìm element, copy HTML và CSS, mô tả bằng lời cho agent. Quá trình này tốn thời gian, dễ sai sót, và thiếu context (screenshot). Design Mode tự động hóa toàn bộ bước này.

---

## Tính năng chi tiết

### Embedded Browser
- Browser Chromium thực tích hợp trong Orca (không phải webview đơn giản)
- Điều hướng URL như browser thông thường
- JavaScript execution đầy đủ
- Cookie import từ Chrome/Safari profile bên ngoài

### Element Picker
- Activate Design Mode → cursor chuyển sang chế độ inspect
- Hover: highlight element đang hover
- Click: capture element và thoát picker mode

### Context Capture
Khi click vào element, hệ thống tự động thu thập:
- **Outer HTML**: toàn bộ HTML của element và ancestors liên quan
- **Computed CSS**: style computed sau khi apply cascade
- **Screenshot**: crop viewport quanh element (với padding)

### Prompt Injection
- Context được format thành Markdown/code block
- Tự động append vào agent prompt đang soạn
- Người dùng chỉ cần thêm yêu cầu cụ thể

### Viewport Presets
- Mobile (375×667, 390×844, 414×896)
- Tablet (768×1024, 820×1180)
- Desktop (1280×800, 1440×900, 1920×1080)
- Custom viewport size

### Cookie Import
- Import cookies từ Chrome profile
- Import cookies từ Safari profile
- Hữu ích để test với session đã đăng nhập

---

## Luồng người dùng

```
1. Mở browser tab trong Orca → điều hướng tới app cần sửa
2. Click "Design Mode" button
3. Hover lên button "Submit" → thấy highlight
4. Click "Submit" button
5. Orca capture: HTML, CSS, screenshot của button
6. Context được inject vào agent prompt
7. Người dùng thêm: "Sửa button này thành màu xanh và bo tròn góc"
8. Agent nhận đầy đủ context và sửa code chính xác
```

---

## Tiêu chí chấp nhận

- [ ] Người dùng capture được UI element context trong < 3 clicks
- [ ] HTML, CSS, và screenshot đều được capture chính xác
- [ ] Context được inject đúng vào agent prompt
- [ ] Viewport presets thay đổi kích thước browser ngay lập tức
- [ ] Cookie import hoạt động với Chrome và Safari

---

## Yêu cầu kỹ thuật

| Thành phần | Chi tiết |
|-----------|---------|
| **Browser** | Electron BrowserView / WebContentsView |
| **Annotation overlay** | `src/shared/browser-annotation-viewport-bridge.ts` |
| **Grab types** | `src/shared/browser-grab-types.ts` |
| **Viewport presets** | `src/shared/browser-viewport-presets.ts` |
| **Cookie import** | `src/shared/browser-cookie-import-sources.ts` |
| **Screencast** | `src/shared/browser-screencast-protocol.ts` |
| **URL handling** | `src/shared/browser-url.ts` |
