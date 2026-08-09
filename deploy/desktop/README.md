# Orca Desktop — Build & Release

Build và đóng gói ứng dụng Electron desktop từ package [`desktop/`](../../desktop/) (đã tách khỏi monorepo gốc — tự chứa, có `package.json`/`node_modules`/`electron.vite.config.ts`/`config/electron-builder.config.cjs` riêng).

## Build local

```bash
./deploy/desktop/scripts/build.sh                # platform hiện tại, tạo installer
./deploy/desktop/scripts/build.sh --dir           # unpacked only (nhanh — để test)
./deploy/desktop/scripts/build.sh --mac           # macOS (.dmg + .zip)
./deploy/desktop/scripts/build.sh --win           # Windows (.exe NSIS)
./deploy/desktop/scripts/build.sh --linux         # Linux (AppImage + .deb)
```

Output: `desktop/dist/`

## Các bước bên trong

1. `pnpm install` trong `desktop/` (nếu chưa có `node_modules`)
2. `pnpm run build:relay` — build Dev Server Relay binaries đa nền tảng (`out/relay/`), cần cho tính năng "Deploy Relay" của SSH Targets trong app
3. `pnpm run build` (`electron-vite build`) — build main + preload + renderer
4. `electron-builder --config config/electron-builder.config.cjs` — đóng gói installer

## Ký code & Publish (chưa tự động hoá trong repo này)

`config/electron-builder.config.cjs` đã cấu hình mac/win/linux targets nhưng **ký code (code signing) và notarization macOS/Windows cần secrets riêng** (Apple Developer ID, Windows cert) — không được đóng gói sẵn trong sandbox này. Chạy build thật cần:

- macOS: `CSC_LINK`, `CSC_KEY_PASSWORD`, Apple notarization credentials
- Windows: cert `.pfx` + password
- Xem `config/electron-builder.config.cjs` (mục `mac`/`win`) và CI workflow gốc (`.github/workflows/`) để biết chi tiết secrets cần thiết

## Yêu cầu môi trường

- Build cho macOS **phải chạy trên macOS** (Xcode command line tools)
- Build cho Windows nên chạy trên Windows hoặc dùng Wine trên Linux CI
- Build cho Linux chạy được trên Linux/macOS
- `node-pty` cần biên dịch native — máy build cần `python3`/`make`/`g++` (Linux/macOS) hoặc Visual Studio Build Tools (Windows)

## CI

`.github/workflows/computer-e2e.yml` (và các workflow release khác trong `.github/workflows/`) hiện vẫn trỏ vào cấu trúc monorepo cũ (`electron.vite.config.ts` ở root) — **cần cập nhật để trỏ vào `desktop/` sau khi merge thay đổi này.**
