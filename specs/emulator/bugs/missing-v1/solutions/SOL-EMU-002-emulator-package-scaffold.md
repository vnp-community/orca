# SOL-EMU-002: Khởi tạo workspace `emulator/`

**Resolves:** CR-DS-009 §2.3, Phase 2
**Service:** `emulator/` (mới)
**Affected files:** `emulator/package.json`, `emulator/tsconfig.json`, `emulator/build.mjs`, `emulator/vitest.config.ts`, `pnpm-workspace.yaml`
**Status:** ✅ Implemented — xem [TASK-EMU-002](../tasks/TASK-EMU-002-scaffold-emulator-workspace.md)

## Giải pháp

Tạo package độc lập `orca-emulator-agent`, thêm vào `pnpm-workspace.yaml`
ngang hàng `agent`. Không extend `@electron-toolkit/tsconfig` (emulator
không có gì liên quan Electron) — tsconfig tự định nghĩa compilerOptions
tương đương (ES2022, Node22, strict). `build.mjs` adapt trực tiếp từ
`agent/build.mjs` (esbuild, bundle `src/relay/emulator-entry.ts` →
`out/emulator.js`, external hoá các native module không cần cho emulator).

Package này **không khai báo phụ thuộc** vào `agent/` hay bất kỳ file nào
trong `agent/src/relay/` — đúng theo phân tích ở TDD-EMU-03 (transport thật
sẽ đến qua `packages/dev-agent-transport/` một khi TASK-EMU-001 xong, không
phải qua deep-import chéo package).
