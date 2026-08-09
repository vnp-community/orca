# Orca Mobile — Build & Release

Build và phát hành app di động (React Native/Expo) từ package [`mobile/`](../../mobile/) — vốn **đã độc lập từ trước** (có `pnpm-lock.yaml`, `tsconfig.json`, `metro.config.js` riêng, không nằm trong pnpm workspace của repo — xem lý do trong lịch sử trao đổi hoặc hỏi lại nếu cần).

## Build local (dev/simulator)

```bash
./deploy/mobile/scripts/build.sh --ios       # expo run:ios     (cần macOS + Xcode)
./deploy/mobile/scripts/build.sh --android   # expo run:android (cần Android SDK)
```

Lệnh trên tương đương chạy trực tiếp trong `mobile/`: `pnpm run ios` / `pnpm run android`.

## Release — iOS (TestFlight, đã tự động hoá qua Fastlane)

```bash
cd mobile
bundle exec fastlane ios release
```

Lane `release` (`mobile/fastlane/Fastfile`):
1. Build prebuilt iOS workspace (`ios/Orca.xcworkspace`, scheme `Orca`)
2. Ký bằng distribution identity đã import vào CI keychain + provisioning profile lấy qua App Store Connect API key
3. Upload `.ipa` lên TestFlight (nhóm tester: `peeps`)

**Secrets cần thiết** (không có sẵn trong sandbox này, phải cấu hình ở máy build/CI thật):
- `ASC_KEY_ID`, `ASC_ISSUER_ID`, `ASC_API_KEY_P8` — App Store Connect API key
- Distribution certificate (.p12) đã import vào keychain

Lane phụ `prepare_release_version` tự tính version/build number tiếp theo dựa trên trạng thái App Store hiện tại (bỏ qua các version đã "closed" — approved/released/removed).

## Release — Android (chưa tự động hoá)

`mobile/fastlane/Fastfile` **chỉ có lane cho iOS** — không có lane Android/Play Store trong repo hiện tại. Build Android hiện dừng ở `expo run:android` (local/emulator). Nếu cần tự động hoá release Play Store, cần thêm:
- `fastlane supply` config (Play Console service account JSON)
- Lane Android riêng trong `mobile/fastlane/Fastfile`

## Yêu cầu môi trường

| Platform | Yêu cầu |
|----------|---------|
| iOS build/release | **macOS** + Xcode + CocoaPods |
| Android build | Android SDK + JDK |
| Cả hai | Node.js, pnpm (cài riêng trong `mobile/`, không dùng workspace root) |

## Không build được trong sandbox này

Môi trường hiện tại không có Xcode/Android SDK/fastlane — các script trên **chưa được chạy thử thật** ở đây, chỉ được viết đúng theo cấu trúc `mobile/package.json` + `mobile/fastlane/Fastfile` đã khảo sát. Cần verify trên máy có đủ toolchain trước khi dùng cho release thật.
