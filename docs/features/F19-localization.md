# F19 — Localization

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F19 |
| **Tên** | Localization (i18n) |
| **Ưu tiên** | P2 — Could Have |
| **Trạng thái** | ✅ Đang duy trì |
| **Tham chiếu PRD** | §3.10 (Localization) |
| **ADR References** | — |
| **HLD References** | C3.4 |

---

## Mô tả

Hỗ trợ giao diện Orca bằng nhiều ngôn ngữ — hiện tại: Tiếng Anh (mặc định), Tiếng Trung (Simplified), Nhật, Hàn, Tây Ban Nha, và Bồ Đào Nha.

---

## Ngôn ngữ hỗ trợ

| Locale | Ngôn ngữ | Trạng thái |
|--------|----------|-----------|
| `en` | English | ✅ Source |
| `zh-CN` | 中文 (Simplified) | ✅ Available |
| `ja` | 日本語 | ✅ Available |
| `ko` | 한국어 | ✅ Available |
| `es` | Español | ✅ Available |
| `pt` | Português | ✅ Available |

---

## Tính năng chi tiết

### i18n System
- `i18next` + `react-i18next` cho translation
- Lazy loading locale bundles (không load tất cả khi startup)
- Fallback về English nếu key không có trong locale
- Locale detection từ OS settings

### Translation Management
- **Localization catalog**: file JSON cho mỗi locale
- **Verification**: script kiểm tra coverage (không được thiếu key)
- **Bootstrap script**: tạo catalog mới cho locale mới
- **Repair script**: sửa catalog bị lỗi cấu trúc

### String Extraction
- Tất cả user-facing string phải qua `t()` function
- Linting rule cảnh báo khi có hardcoded string trong JSX

---

## Tiêu chí chấp nhận

- [ ] 100% user-facing strings qua i18n system
- [ ] Tất cả locale có coverage > 90%
- [ ] Locale switching không cần restart app
- [ ] Fallback về English khi key thiếu (không crash)

---

## Yêu cầu kỹ thuật

| Thành phần | Chi tiết |
|-----------|---------|
| **Library** | `i18next` v26.3.1, `react-i18next` v17.0.8 |
| **Main i18n** | `src/main/i18n/` |
| **Renderer i18n** | `src/renderer/src/i18n/` |
| **UI language** | `src/shared/ui-language.ts` |
| **UI locale** | `src/shared/ui-locale.ts` |
| **Verify catalog** | `config/scripts/verify-localization-catalog.mjs` |
| **Audit coverage** | `config/scripts/audit-localization-coverage.mjs` |
| **Bootstrap** | `config/scripts/bootstrap-locale-catalog.mjs` |
