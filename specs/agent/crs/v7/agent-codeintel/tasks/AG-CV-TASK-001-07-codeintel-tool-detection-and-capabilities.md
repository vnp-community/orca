# AG-CV-TASK-001-07: Phát hiện công cụ và capability `codeintel*` trong handshake

**From Solution:** [AG-CV-SOL-001-codeintel-agent-foundation](../solutions/AG-CV-SOL-001-codeintel-agent-foundation.md) mục 2.7
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-tool-detection.ts` (mới), `agent/src/relay/codeintel-tool-detection.test.ts` (mới), `agent/src/relay/agent-session-capabilities.ts` (sửa), `agent/src/relay/agent-session-capabilities-codeintel.test.ts` (mới), `agent/src/shared/agent-wire-protocol.ts` (sửa)
**Depends on:** [01](./AG-CV-TASK-001-01-codeintel-errors-and-strict-params.md) (mã lỗi `unsupported_*`)
**Status:** [ ] TODO

## Context

Contract §1.3: `codeintel` khi >= 1 binary có trong `toolPath` (chỉ kiểm tồn tại, **không** `--version` trong handshake vì cuộc đua 5 s ở `agent-session-handshake.ts:44-54`); `codeintel.gitnexus`, `codeintel.codegraph` theo từng binary; `STATIC_CAPABILITIES_FALLBACK` không thêm; `tools[]` giữ nguyên (PQ-18: backend tự thêm `Tools`). Dải phiên bản (giả định, đã thử 1.6.9/1.4.1): `gitnexus >=1.6.0 <2`, `codegraph >=1.4.0 <2`. Đã đọc `agent-session-capabilities.ts` (`checkGitAvailable` quét `config.toolPath.split(':')`, `buildCapabilities` `:58-96`) và `shared/agent-wire-protocol.ts:58` (`AgentCapability` chỉ là kiểu của `AgentHandshakeParams`; grep không thấy nơi nào khác dùng).

## Việc cần làm

1. **Impact trước khi sửa:** `gitnexus impact -r /opt/repos/orca buildCapabilities --direction upstream` (và `AgentCapability`); ghi vào PR.
2. `codeintel-tool-detection.ts`: `hasCodeIntelBinary(config, 'gitnexus'|'codegraph')` (`fs.access(X_OK)` trên mỗi thư mục của `config.toolPath.split(path.delimiter)`, không spawn); `detectCodeIntelTools(config, deps?)` -> `{gitnexus:{available, version, supported, binary}, codegraph:{…}}` chạy `<binary> --version` (`execFile`, timeout 5 s, env con), cache 60 s, parse semver, so với dải (`supported:false` ngoài dải). `process.platform === 'win32'` -> đối tượng `unsupportedPlatform:true` mà handler đổi thành `CODEINTEL_TOOL_UNAVAILABLE reason='unsupported_platform'`.
3. `agent-session-capabilities.ts`: trong `buildCapabilities`, sau khối `hasPty`, `const intel = await detectCodeIntelBinaries(config)` (gọi `hasCodeIntelBinary` hai lần song song); nếu có ít nhất một: `caps.push('codeintel')`, thêm `codeintel.gitnexus`/`codeintel.codegraph` theo từng binary. Không động `STATIC_CAPABILITIES_FALLBACK`. Không thêm `quality` (AG-CV-SOL-081).
4. `agent-wire-protocol.ts`: `AgentCapability` thêm `'codeintel' | 'codeintel.gitnexus' | 'codeintel.codegraph' | 'quality'` (additive; chỉ ảnh hưởng kiểu).

## Kiểm thử

`codeintel-tool-detection.test.ts`: binary giả có/không; `--version` in `1.6.9`/`1.3.0`/`2.0.0`/rác -> `supported`/`unsupported`; cache 60 s (đồng hồ giả); timeout 5 s -> `available:true, version:null, supported:false`; `win32` giả (`Object.defineProperty(process,'platform')`).
`agent-session-capabilities-codeintel.test.ts` (khuôn mock `checkPtyAvailable` bằng `vi.mock('node-pty')` như test hiện có nếu có; **chưa kiểm** file test hiện có của capabilities): có cả hai binary -> `codeintel`, `codeintel.gitnexus`, `codeintel.codegraph`; chỉ một -> `codeintel` + một; không -> không có cái nào; mọi capability cũ (`fs`, `git`, `pty*`…) vẫn có; `STATIC_CAPABILITIES_FALLBACK` không chứa `codeintel*` (snapshot).

Lệnh: `pnpm exec vitest run src/relay/codeintel-tool-detection.test.ts src/relay/agent-session-capabilities-codeintel.test.ts src/relay/__tests__/agent-session.test.ts`

## Tiêu chí hoàn thành

- [ ] `agent.handshake` có `codeintel*` khi binary tồn tại và không khi vắng.
- [ ] Không có `--version` nào chạy trong đường handshake.
- [ ] Test handshake hiện có (`agent-session.test.ts`) xanh không sửa.

## Rủi ro và lưu ý

- Dải phiên bản là giả định (chỉ đã thử 1.6.9 và 1.4.1).
- Backend phải gọi lại `codeintel.status` sau mỗi lần nối: capability chỉ tính một lần tại handshake (đã ghi ở TDD v5/08 mục 8 cho node-pty; cùng giới hạn).
- Telemetry CodeGraph: gọi `codegraph --version` không nên bật telemetry; chưa kiểm chứng.
