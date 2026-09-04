# Mobile Emulator Agent — Technical Design Documents

**Cập nhật:** 2026-09-03 (v1.0 — initial)
**Phiên bản:** v1.0
**Source code:** `emulator/src/relay/*`
**CR Ref:** [CR-DS-009](../../../../docs/crs/v2/dev-server/CR-DS-009-mobile-emulator-agent-separation.md)
**Mirrors:** `specs/agent/tdd/v5/` (Dev Server Agent TDD) — cùng quy ước, khác domain

> Mobile Emulator Agent là một tiến trình/gói **hoàn toàn tách biệt** với Dev
> Server Agent (`agent/`). Nó không có git/fs/pty/browser/cli — chỉ có
> `device.*` (điều khiển Android Emulator qua ADB, iOS Simulator qua
> `xcrun simctl`).

## Tài liệu

| # | File | Nội dung |
|---|------|---------|
| 01 | [01-architecture.md](./01-architecture.md) | Vị trí trong hệ thống, ranh giới với Dev Server Agent |
| 02 | [02-device-rpc-catalog.md](./02-device-rpc-catalog.md) | Catalog method `device.*`, khớp `emulator_relay.go` |
| 03 | [03-transport-reuse-analysis.md](./03-transport-reuse-analysis.md) | Vì sao tái dùng transport của `agent/` khó hơn dự tính, kế hoạch tách |
| 04 | [04-deployment.md](./04-deployment.md) | Build, cài đặt, chế độ chạy (stdio debug vs WS thật) |

## Tech Stack (giống Dev Server Agent, không kéo theo phụ thuộc git/pty)

| Layer | Technology |
|-------|-----------|
| Language | TypeScript strict |
| Runtime | Node.js ≥ 22 |
| Bundler | esbuild (`emulator/build.mjs`, adapted từ `agent/build.mjs`) |
| Testing | Vitest |
| Wire protocol (khi TASK-EMU-001 xong) | dùng chung `packages/dev-agent-transport` với `agent/` — KHÔNG tự cài đặt lại |
