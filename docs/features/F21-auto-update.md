# F21 — Auto Update

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F21 |
| **Tên** | Auto Update |
| **Ưu tiên** | P0 — Must Have |
| **Trạng thái** | ✅ Đã phát hành |
| **Tham chiếu PRD** | §6, §5.2 |
| **Tham chiếu URD** | UR-081 |
| **Tham chiếu SRS** | NFR-2.1 |
| **ADR References** | — |
| **HLD References** | C2 |

---

## Mô tả

Hệ thống tự động cập nhật Orca lên phiên bản mới, với fallback khi update thất bại, changelog inline, và tùy chọn kênh pre-release.

---

## Tính năng chi tiết

### Update Check
- Kiểm tra update tự động khi startup
- Background check định kỳ
- Thông báo in-app khi có bản mới

### Update Channels
- **Stable**: bản release chính thức
- **Pre-release**: bản RC, beta (opt-in)

### Changelog Display
- Hiển thị changelog của version mới trực tiếp trong app
- Markdown rendering với highlights

### Update Process

```
1. Check → Có version mới
2. Thông báo người dùng với changelog
3. Người dùng chọn "Update Now" hoặc "Later"
4. Download update in background
5. Verify download integrity
6. Prompt "Restart to apply update"
7. Restart và apply update
```

### Fallback Mechanism
- Nếu update thất bại → rollback về version cũ
- Watchdog: nếu app crash sau update → fallback
- Thông báo lỗi rõ ràng

### macOS Specifics
- macOS code signing verification
- Apple notarization
- DMG-based update

---

## Tiêu chí chấp nhận

- [ ] Auto-update success rate > 99%
- [ ] Fallback hoạt động khi update corrupt
- [ ] Changelog hiển thị trước khi apply update
- [ ] Pre-release channel có thể opt-in từ settings
- [ ] Không mất dữ liệu người dùng khi update

---

## Yêu cầu kỹ thuật

| Thành phần | Chi tiết |
|-----------|---------|
| **Library** | `electron-updater` v6.8.3 |
| **Updater events** | `src/main/updater-events.ts` |
| **Updater core** | `src/main/updater.ts` (~55K bytes) |
| **Changelog** | `src/main/updater-changelog.ts` |
| **Fallback** | `src/main/updater-fallback.ts` |
| **macOS install** | `src/main/updater-mac-install.ts` |
| **Nudge** | `src/main/updater-nudge.ts` |
| **Pre-release feed** | `src/main/updater-prerelease-feed.ts` |
| **Exit watchdog** | `src/main/update-install-exit-watchdog.ts` |

---

## Metrics

| KPI | Target |
|----|-------|
| Update success rate | > 99% |
| Download time (200MB) | < 2 phút (100 Mbps) |
| Restart time | < 10 giây |
