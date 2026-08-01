# F03 — Mobile Companion App

| Thuộc tính | Giá trị |
|-----------|---------|
| **ID** | F03 |
| **Tên** | Mobile Companion App |
| **Ưu tiên** | P0 — Must Have |
| **Trạng thái** | ✅ Đã phát hành (iOS 0.0.27 / Android 0.0.27) |
| **Tham chiếu PRD** | §3.3 |
| **Tham chiếu URD** | UR-030, UR-031, UR-032 |
| **Tham chiếu SRS** | FR-5.1, FR-5.2, FR-5.3 |
| **ADR References** | — |
| **HLD References** | C3.4 |

---

## Mô tả

Ứng dụng companion mobile (iOS/Android) giúp người dùng giám sát và điều khiển các AI agent đang chạy trên desktop từ bất kỳ đâu — nhận thông báo khi agent xong việc, gửi follow-up instructions từ điện thoại.

---

## Vấn đề cần giải quyết

Developer không thể ngồi trước máy tính chờ agent xong. Với agent chạy hàng giờ, người dùng cần biết ngay khi agent hoàn thành để tiếp tục workflow — dù đang họp, di chuyển, hoặc làm việc khác.

---

## Tính năng chi tiết

### Device Pairing
- Desktop hiển thị mã QR (encode: server URL + one-time token)
- Mobile scan QR để pair trong < 30 giây
- E2E encryption sau khi pair (TweetNaCl key exchange)
- One-time token hết hạn sau 5 phút
- Kết nối bền vững khi chuyển mạng WiFi/4G

### Monitoring
- Xem danh sách tất cả agent đang chạy
- Trạng thái real-time: idle, running, waiting, completed, error
- Xem worktree nào đang active

### Push Notifications
- Notification khi agent hoàn thành task
- Notification khi agent gặp lỗi
- Notification khi agent chờ user input
- Delivery < 5 giây sau khi sự kiện xảy ra
- Hoạt động khi app mobile ở background

### Remote Dispatch
- Gõ follow-up prompt từ điện thoại và gửi về agent
- Agent nhận và xử lý prompt từ mobile
- Xem trạng thái agent cập nhật real-time sau khi dispatch

### Security
- E2E encryption toàn bộ traffic (TweetNaCl)
- Không có server trung gian — kết nối peer-to-peer qua local network
- Session key rotation

---

## Platform

| Platform | Phân phối | Version tối thiểu |
|----------|-----------|------------------|
| iOS | App Store, TestFlight | iOS 15+ |
| Android | APK download | Android 8+ |

---

## Luồng người dùng

```
[Pairing]
1. Mở Orca Desktop → Settings → Mobile
2. Desktop hiển thị QR code
3. Mở Orca Mobile → Scan QR
4. Pair thành công → kết nối được mã hóa

[Nhận notification]
5. Agent hoàn thành task trên desktop
6. Push notification tới điện thoại (< 5 giây)
7. Tap notification → mở Orca Mobile
8. Xem trạng thái agent

[Gửi follow-up]
9. Nhập prompt trong Orca Mobile
10. Tap Send → Desktop nhận prompt
11. Agent xử lý và tiếp tục làm việc
```

---

## Tiêu chí chấp nhận

- [ ] Pairing hoàn thành trong < 30 giây
- [ ] Notification delivery trong < 5 giây khi agent kết thúc
- [ ] Notification hoạt động khi app mobile ở background
- [ ] Follow-up prompt được nhận và xử lý bởi agent
- [ ] Mã QR hết hạn sau 5 phút nếu chưa pair
- [ ] Hỗ trợ iOS 15+ và Android 8+

---

## Yêu cầu kỹ thuật

| Thành phần | Chi tiết |
|-----------|---------|
| **Encryption** | TweetNaCl (tweetnacl v1.0.3) |
| **QR Code** | `qrcode` library |
| **Transport** | WebSocket (local network) |
| **Pairing module** | `src/shared/pairing.ts` |
| **Mobile markdown** | `src/shared/mobile-markdown-document.ts` |
| **Mobile firewall (Windows)** | `src/shared/windows-mobile-firewall.ts` |
