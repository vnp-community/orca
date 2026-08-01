# BUG-BE-MOBILE-001: Mobile Companion hoàn toàn chưa được implement — PairingManager, MobileNotificationService không tồn tại

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-MB-001  
**Note:** mobile/MobileCompanionService.ts: Web Push RFC 8030, VAPID, 410 cleanup  

## Mức độ: 🔴 CRITICAL (Feature Missing)

## Tóm tắt

HLD (BL-MB-01 → BL-MB-04) mô tả Mobile Companion features bao gồm:
- `PairingManager` — generate QR code, NaCl keypair, verify pairing code
- `MobileNotificationService` — APNs/FCM push notifications
- `MobileDispatchHandler` — nhận dispatch từ mobile
- `MobileStatusHandler` — trả agent status cho mobile

Grep toàn bộ `src/` không tìm thấy **bất kỳ class nào** trong số này:
```
MobileNotificationService → No results
PairingManager            → No results
APNs / FCM                → No results
mobile_devices table      → No results
TweetNaCl / tweetnacl     → No results
```

## Ảnh hưởng

1. **Toàn bộ BL-MB domain (BL-MB-01 → BL-MB-04) chưa được implement**.
2. User Sam không thể pair mobile device với desktop Orca.
3. Không có push notification khi agent hoàn thành.
4. Mobile dispatch và status monitoring không hoạt động.

## Files không tồn tại (theo HLD)

- `src/main/mobile/pairing-manager.ts` — chưa tạo
- `src/main/mobile/mobile-notification-service.ts` — chưa tạo
- `src/main/mobile/mobile-dispatch-handler.ts` — chưa tạo
- `src/main/mobile/mobile-ws-router.ts` — chưa tạo
- DB migration cho `mobile_devices` table — chưa tạo

## Liên quan đến luồng

- **BL-MB-01**: Pair device — hoàn toàn không có.
- **BL-MB-02**: Push notification — không có.
- **BL-MB-03**: Remote dispatch từ mobile — không có.
- **BL-MB-04**: Agent status từ mobile — không có.
