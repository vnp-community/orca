# TASK-EMU-002: Khởi tạo workspace `emulator/`

**Solution:** [SOL-EMU-002](../solutions/SOL-EMU-002-emulator-package-scaffold.md)
**Priority:** P1
**Status:** `[x]` DONE

## Việc đã làm

- `emulator/package.json` (`name: orca-emulator-agent`, `bin.orca-emulator-agent`, scripts `build`/`start`/`test`)
- `emulator/tsconfig.json` (tự định nghĩa compilerOptions — ES2022/CommonJS/Bundler resolution/strict, không extend `@electron-toolkit/tsconfig`)
- `emulator/build.mjs` (esbuild, bundle `src/relay/emulator-entry.ts` → `out/emulator.js`, `external: []` — không cần external hoá native module nào vì không có git/fs/pty)
- `emulator/vitest.config.ts`
- Thêm `'emulator'` vào `pnpm-workspace.yaml`

## Verify (chạy thật trong pass này)

```
$ node <agent's local typescript>/bin/tsc --noEmit -p emulator/tsconfig.json
(không có output — sạch)

$ node build.mjs
out/emulator.js  10.9kb
⚡ Done in 8ms
```

Ghi chú: `tsconfig.json` ban đầu dùng `"moduleResolution": "Node"` nhưng
TypeScript cài sẵn trong sandbox (7.0.2) đã bỏ giá trị này (`node10`) —
đổi sang `"Bundler"` để tương thích cả TS hiện tại lẫn TS mới hơn.
