# AG-REQ-TASK-033-10: Đăng ký RPC `agent.capabilities` và chốt hợp đồng JSON với backend

**From Solution:** [AG-REQ-SOL-033-capability-report-handshake-and-ai-complete](../solutions/AG-REQ-SOL-033-capability-report-handshake-and-ai-complete.md) mục 2.2 và 3.5
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/agent-rpc-dispatch-misc.ts` (sửa), `agent/src/relay/agent-rpc-dispatch-misc.test.ts` (sửa), `agent/src/relay/__fixtures__/agent-capabilities-golden.json` (mới, mẫu hợp đồng dùng chung với backend)
**Depends on:** [09](./AG-REQ-TASK-033-09-capability-report-core.md)
**Status:** [x] DONE

## Context

CR-033 mục 2.6 đặt đăng ký `agent.capabilities` ở `agent-rpc-dispatch-misc.ts`. Đã đọc file này (287 dòng): `dispatchMiscRpc(rpc, tools, config, log, ws, state)` là một `switch` với mẫu mỗi `case`: `try { const { fn } = await import('./module'); return (await fn(rpc.id, rpc.params ?? {}, ...)) as JsonRpcResponse } catch (err) { return makeError(rpc.id, AgentErrorCode.ServerError, \`<method> unavailable: ${msg}\`) }`; `host.capabilities` (dòng 150 trở đi) là mẫu gần nhất. Dispatcher gốc `agent-rpc-dispatch.ts` đi qua các `dispatch*Rpc` theo thứ tự rồi trả `MethodNotFound` khi không ai nhận, nên thêm `case` ở file này là đủ, không phải sửa `route()`.

Phía backend: agent chưa có method trả `-32601 MethodNotFound`, `devserveragent/client.go:434-437` bọc thành `domain.ErrAgentMethodNotFound`; `usecase/get_host_capabilities.go` là mẫu degradation. Với agent mới, `GetDevServerCapabilities` (infra-fleet, ngoài phạm vi task này) gọi `agent.capabilities` rồi lưu nguyên JSON vào `profile_json`. Hợp đồng JSON phải khớp giữa agent (TypeScript) và decoder Go: task này tạo tệp mẫu để cả hai bên dùng làm golden.

## Việc cần làm

1. Trong `dispatchMiscRpc` thêm, ngay sau `case 'host.capabilities'`:
   ```ts
   case 'agent.capabilities': {
     try {
       const { handleAgentCapabilities } = await import('./agent-capability-report')
       return (await handleAgentCapabilities(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
     } catch (err: unknown) {
       const msg = err instanceof Error ? err.message : String(err)
       return makeError(rpc.id, AgentErrorCode.ServerError, `agent.capabilities unavailable: ${msg}`)
     }
   }
   ```
2. Trong `agent-capability-report.ts` thêm `handleAgentCapabilities(id, params, config, log)`: gọi `validateCapabilityParams`; lỗi thì trả `{ jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'agent.capabilities: <message> (<code>)', data: { reason: code } } }`; thành công thì `{ jsonrpc: '2.0', id, result: await buildCapabilityReport(...) }`. Tự dựng đối tượng lỗi, không import `makeError` (tránh vòng với `agent-rpc-dispatch.ts`, như task 01).
3. Ghi log một dòng info: số công cụ đã cài, `partial`, thời gian dò. KHÔNG log tên hay giá trị biến môi trường.
4. Tạo `agent/src/relay/__fixtures__/agent-capabilities-golden.json`: một phản hồi mẫu đầy đủ đúng sơ đồ solution mục 2.2, và một mẫu `partial: true` có `installed: null`. Backend sao chép tệp này làm golden cho decoder Go (README tasks, mục "Hợp đồng chung với backend", đã nêu).
5. Thêm `agent.capabilities` vào danh sách phương thức được ghi trong chú thích đầu `agent-rpc-dispatch-misc.ts` (một dòng).
6. Không thêm `agent.capabilities` vào `buildCapabilities`/`STATIC_CAPABILITIES_FALLBACK` (mảng `capabilities` cũ giữ nguyên; tính năng mới đi qua `features` ở task 12).
7. Phát hiện rủi ro dispatcher có tự xác thực: kiểm `agent-rpc-dispatch.ts` có cổng chặn RPC trước khi handshake xong ("Gate RPC dispatch behind successful handshake" ở `agent-session.ts`): `agent.capabilities` tự nhiên đi qua cổng đó; không cần làm gì thêm, ghi vào test.

## Kiểm thử

Thêm vào `agent-rpc-dispatch-misc.test.ts` (khuôn sẵn: `MockWs`, `createWireState`, `LOG`, `vi.mock` cho module phụ thuộc):
- `routes agent.capabilities to the report builder and returns its result under the same id` (mock `./agent-capability-report` trả báo cáo giả).
- `returns InvalidParams with data.reason INVALID_CAPABILITY_PARAMS for tools given as a string`.
- `returns InvalidParams TOO_MANY_ENV_NAMES for 65 envNames`.
- `returns ServerError "agent.capabilities unavailable" when the module throws on import` (mock ném).
- `passes refresh through to the builder`.
- `result matches the golden contract keys` (đọc `__fixtures__/agent-capabilities-golden.json` bằng `fs.readFileSync` tương đối từ `import.meta.dirname`, so tập khoá cấp trên và cấp `host`/`claude`/`agent` với báo cáo giả; không so giá trị; `import.meta.dirname` đã được `vitest.config.ts` dùng nên có sẵn trên Node 22).
- Hồi quy: các `it` hiện có của file (`connection.teardown`...) xanh.
- Dispatcher thật: thêm một `it` ở `src/relay/__tests__/agent-rpc-dispatch.test.ts` gọi `route` qua `handleRpcFrame` hoặc hàm công khai tương đương (đọc file để chọn đúng điểm vào) với `method: 'agent.capabilities'` và khẳng định KHÔNG trả `MethodNotFound`.

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run src/relay/agent-rpc-dispatch-misc.test.ts src/relay/__tests__/agent-rpc-dispatch.test.ts`; rồi `pnpm test`.

Phía backend (ghi để phối hợp, KHÔNG làm ở đây): test Go decoder đọc cùng tệp golden; `GetDevServerCapabilities` với agent cũ (`-32601`) trả `source=handshake_only`, `degraded=true`.

## Tiêu chí hoàn thành

- [x] Gọi `agent.capabilities` qua dispatcher thật cho kết quả đúng sơ đồ; không còn `MethodNotFound`.
- [x] Tham số sai trả `InvalidParams` kèm `data.reason`, không chạy lệnh nào.
- [x] Tệp golden có mặt và test so khoá khớp với báo cáo giả.
- [x] `capabilities`/`STATIC_CAPABILITIES_FALLBACK` không đổi.
- [x] Không log giá trị hay tên biến môi trường.

## Rủi ro và lưu ý

- Một lần dò mất tới 8 giây; RPC trả lời chậm trong lúc đó. Dispatcher xử lý đồng thời nhiều RPC trên một kết nối (chưa kiểm chứng việc dispatch có tuần tự hoá hay không); nếu tuần tự, một lần `agent.capabilities` chậm sẽ chặn RPC sau nó 8 giây. Kiểm trong `agent-rpc-dispatch.ts` (vòng `handleMessage`) trước khi chốt; nếu tuần tự, backend cần gọi `agent.capabilities` ở kết nối nền hoặc ngoài giờ cao điểm (CR: sau `attachTransport`, tối đa 1 lần mỗi 5 phút).
- Backend cũ không gọi RPC này: không ảnh hưởng. Backend mới gọi agent cũ: `-32601`, xử lý ở infra-fleet.
- Golden JSON dùng chung với Go: đổi khoá phải đổi cả hai nơi; thêm khoá mới chỉ được thêm (không đổi tên, không đổi kiểu), `schemaVersion` tăng khi thay đổi không tương thích.
- Dò `claude auth status` có thể có tác dụng phụ (làm mới token, ghi file cấu hình) của `claude`; chưa kiểm chứng; nếu có, tách sau cờ.

## Không làm trong task này

- Không viết `GetDevServerCapabilities` hay migration `0039` (phía `infra-fleet-service`, thuộc solution backend).
- Không thêm tên vào handshake `features` (task 12).
- Không thêm lệnh `claude` nào ngoài `--version`, `--help` (qua `detectClaudeFlags`) và `auth status --json`.
