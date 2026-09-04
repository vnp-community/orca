# TASK-EMU-001: Trích `packages/dev-agent-transport/` từ `agent/`

**Solution:** [SOL-EMU-001](../solutions/SOL-EMU-001-shared-transport-extraction.md)
**Priority:** P1
**Status:** `[x]` DONE — verify thật (xem bên dưới), zero regression trên `agent/`'s test suite

## Việc đã làm (phạm vi thu hẹp — KHÔNG cần refactor DI trên `agent-session.ts`)

1. `git mv agent/src/main/ssh/relay-protocol.ts
   packages/dev-agent-transport/src/relay-protocol.ts` +
   `git mv agent/src/relay/agent-wire.ts
   packages/dev-agent-transport/src/agent-wire.ts`.
2. `packages/dev-agent-transport/package.json` (tên `orca-dev-agent-transport`,
   `private: true`), `tsconfig.json` (mẫu `emulator/tsconfig.json`:
   ES2022/CommonJS/`moduleResolution: Bundler`/strict — không extend
   `@electron-toolkit/tsconfig`), thêm `packages/dev-agent-transport` vào
   `pnpm-workspace.yaml`. `src/index.ts` re-export cả hai module.
3. `relay-protocol.ts`'s import `DEFAULT_SSH_RELAY_GRACE_PERIOD_SECONDS` từ
   `agent/src/shared/ssh-types.ts` **không** move theo, và **không** import
   chéo package. Quyết định: copy hằng số này (giá trị `0`) trực tiếp vào
   `relay-protocol.ts` với comment giải thích lý do. Lý do chọn phương án
   này (ít rủi ro nhất trong 3 lựa chọn cân nhắc):
   - Move cả `ssh-types.ts` theo: SAI — file này 238 dòng, phần lớn là domain
     type `SshTarget`/port-forward/v.v. dùng bởi `persistence.ts`,
     `port-scan-handler.ts`, `pty-handler.ts`, `relay.ts` — không liên quan
     transport codec, sẽ kéo domain SSH-target-management vào package
     transport dùng chung.
   - Import chéo package (`packages/dev-agent-transport` phụ thuộc `agent/`):
     SAI — tạo circular workspace dependency, vì `agent/` đã phụ thuộc
     ngược lại `orca-dev-agent-transport` cho chính codec này.
   - Copy 1 hằng số thuần (không logic, giá trị `0`, đã không đổi từ khi tạo):
     rủi ro thấp nhất, tách rời 2 module hoàn toàn, không đổi hành vi runtime.
4. Cập nhật toàn bộ import trong `agent/` trỏ tới 2 file cũ sang package name
   `orca-dev-agent-transport` (không phải relative path xuyên package) — 15
   file: 4 file trong `agent/src/main/ssh/` (`ssh-remote-platform.ts`,
   `ssh-channel-multiplexer.ts`, `ssh-filesystem-stream-reader.ts`,
   `ssh-git-response-stream-reader.ts`), và 11 file trong `agent/src/relay/`
   (bao gồm `agent-session.ts`, `agent-rpc-dispatch.ts` — **chỉ sửa dòng
   import**, không đụng logic dispatcher/domain của các file này, đúng ràng
   buộc AGENTS.md/SOL-EMU-001). `agent/package.json` thêm
   `"orca-dev-agent-transport": "workspace:*"`.
5. `grep -rn` toàn repo (không chỉ `agent/`) theo tên file lẫn theo export
   (`MessageType`, `encodeFrame`, `FrameDecoder`, `createWireState`,
   `encodeDataFrame`, `decodeFrame`) xác nhận `desktop/`, `backend/`,
   `frontend/` có **bản copy độc lập riêng** của `relay-protocol.ts`/
   `agent-wire.ts` (khác commit lịch sử với bản `agent/`) — không import từ
   `agent/`, nên hoàn toàn không bị ảnh hưởng bởi việc move này.
6. `pnpm install --filter orca-agent --filter orca-dev-agent-transport` (và
   sau đó `--filter orca-emulator-agent` cho TASK-EMU-006) để pnpm tạo
   symlink workspace thật (`agent/node_modules/orca-dev-agent-transport ->
   ../../packages/dev-agent-transport`) — môi trường sandbox này CÓ mạng nên
   không cần thủ thuật thay thế; `pnpm-lock.yaml` diff chỉ có dòng thêm mới
   (869 insertions, 0 deletions), không sửa version của package nào khác.

## Không làm

- Không đụng `agent-session.ts`, `agent-rpc-dispatch.ts` ngoài đúng dòng
  import bị ảnh hưởng bởi việc move — không sửa dispatcher/domain logic.
- Không sửa hành vi wire protocol trong lúc move (frame header, seq/ack,
  keepalive interval, error code) — cơ học move + đổi import path.

## Verify (chạy thật trong pass này, có bằng chứng)

**Baseline (trước move)**, `cd agent && node node_modules/vitest/vitest.mjs run`:
```
Test Files  3 failed | 336 passed | 2 skipped (341)
     Tests  2 failed | 3816 passed | 30 skipped (3848)
```
(3 lỗi pre-existing, không liên quan: esbuild không resolve được
`parcel-watcher-process-entry.ts`, `pty-handler.test.ts` một assertion cũ
sai lệch, `feature-interactions.test.ts` thiếu thư mục `src/renderer/src` —
cả ba tồn tại từ trước, không liên quan transport.)

**Sau move**, cùng lệnh:
```
Test Files  3 failed | 336 passed | 2 skipped (341)
     Tests  2 failed | 3816 passed | 30 skipped (3848)
```
Giống hệt baseline — **zero regression**, không có test nào bị sửa nội dung
để pass (chỉ sửa import path trong 5 file test).

**tsc --noEmit cho `packages/dev-agent-transport/`:**
```
$ node agent/node_modules/typescript/bin/tsc --noEmit -p packages/dev-agent-transport/tsconfig.json
(không có output — sạch)
```

**tsc --noEmit cho `agent/` (dùng `agent/tsconfig.json` hiện có, phạm vi hẹp
chỉ relay/shared/types):** môi trường sandbox dùng TypeScript 7.0.2 (rất mới
so với `devDependencies` ghi `^7.0.2` nhưng hành vi TS6307 "file not listed
within project" khác các bản cũ) khiến lệnh này vốn **đã không sạch từ
trước khi có bất kỳ thay đổi nào của task này** — baseline (stash về commit
gốc) có **107 lỗi TS6307**, toàn bộ là các file không liên quan
(`parcel-watcher-*.ts`, `telemetry/*.ts`, `text-generation/*.ts`, …) nằm
ngoài `tsconfig.json`'s `include` list nhưng được kéo vào qua import bắc
cầu. Sau move: **106 lỗi** — diff chính xác từng dòng lỗi giữa trước/sau xác
nhận **zero lỗi mới**, và move thực ra loại bỏ đúng 1 lỗi cũ (chính lỗi
TS6307 về `relay-protocol.ts` không nằm trong include list — biến mất vì
file giờ ở package khác, có `tsconfig.json` riêng). Không sửa
`agent/tsconfig.json` để "làm sạch" các lỗi pre-existing này — ngoài phạm vi
task, có thể ảnh hưởng nhiều file không liên quan.

**pnpm-lock.yaml**: diff chỉ thêm dòng (869 insertions, 0 deletions) —
`git diff pnpm-lock.yaml | grep -c '^-[^-]'` = 0.
