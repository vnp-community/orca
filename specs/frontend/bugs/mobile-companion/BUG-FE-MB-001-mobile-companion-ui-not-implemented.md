# BUG-FE-MOBILE-001: Mobile Companion UI hoàn toàn không có trong Renderer — không có QR pairing, không có status panel

## Mức độ: 🔴 CRITICAL (Feature Missing)

## Tóm tắt

HLD (BL-MB-01 → BL-MB-04) mô tả UI components:
```
[Renderer Desktop] Settings → Mobile → "Pair New Device"
    → Hiển thị QR code trong Renderer, timeout 5 phút
    
[Renderer Desktop] agent status push từ mobile
    → Mobile notification service

[Admin SPA] Settings → Mobile Devices (manage paired devices)
```

Grep toàn bộ `src/renderer/` không tìm thấy:
```
mobile.pair        → No results
PairingManager     → No results
QR code            → No results (mobile-specific)
mobile.*device     → No results
mobile companion   → No results
```

## Ảnh hưởng

1. **Toàn bộ Mobile Companion UI (BL-MB-01 → BL-MB-04)** không có trong frontend.
2. User Sam không thể vào Settings → Mobile để pair device.
3. Không có UI để xem/revoke paired devices.
4. Push notification preferences không có UI.

## Files không tồn tại (theo HLD)

- `src/renderer/src/components/mobile/mobile-pairing-dialog.tsx`
- `src/renderer/src/components/mobile/paired-devices-panel.tsx`
- `src/renderer/src/components/mobile/qr-code-display.tsx`
- Notification preferences UI component

## Liên quan đến luồng

- **BL-MB-01**: Pair device UI — không có.
- **BL-MB-04**: Mobile Companion UI → tất cả không có.
