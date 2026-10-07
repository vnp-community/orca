# BE-CV-TASK-040-09: Dòng loại trừ MCP `codeIntel.*`, kiểm parity, README gateway, biến compose

**From Solution:** BE-CV-SOL-040-codeintel-channel-foundation
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/excluded_channels.yaml`, `backend-go/services/api-gateway/README.md`, `deploy/dev/docker-compose.yml`
**Depends on:** TASK-040-07 (kênh đã đăng ký); compose cần BE-CV-SOL-010 (tên service)
**Status:** [x] DONE

---

## Context

`TestChannelInventory` (`parity_test.go:38`) đỏ khi kênh không có `ToolSpec` lẫn dòng loại trừ; `TestToolParity` (`:79`) đỏ khi dòng loại trừ không khớp kênh nào. `Exclusion.Match` (`excluded.go:51`) cho `codeIntel.*` khớp mọi cấp. `LoadExclusions` đòi `reason` >= 20 ký tự. Hợp đồng §8.3 điểm 2 và UI-API §9: dòng này nằm **cùng PR** với đăng ký kênh.

## Việc cần làm

1. `excluded_channels.yaml` thêm cuối file:
```yaml
- pattern: "codeIntel.*"
  category: code-intel-v1
  reason: "Code intelligence views carry repository source text, reindex runs analyzers on a dev server and quality runs execute checks; not MCP tools in v1 (README v7 O2, CR-CV-041 may replace this line)."
```
2. Chạy `go test ./internal/adapter/mcpserver/tools/...`: `TestChannelInventory` và `TestToolParity` xanh; xoá tạm dòng => `TestChannelInventory` liệt kê 46 kênh `codeIntel.*` (ghi vào PR như bằng chứng). `tools_list.golden.json` không đổi.
3. README gateway: thêm mục "Code intelligence channels (CR-CV-040)": downstream `code-intel-service`; biến `CODE_INTEL_SERVICE_ADDR`, `CODE_INTEL_MAX_RESPONSE_BYTES`, `CODE_INTEL_MAX_STREAMS`, `CODEINTEL_INTERNAL_CALLER_TOKEN`; 46 kênh (45 + 1); `SetReadLimit` 320 KiB; dùng `invoke`, không `send` (U2); lỗi ở tiền tố `message`; không log args.
4. `docker-compose.yml`: `CODE_INTEL_SERVICE_ADDR: code-intel-service:9090` cạnh `MCP_SERVICE_ADDR` (dòng 71) và `CODEINTEL_INTERNAL_CALLER_TOKEN` cho gateway **chỉ khi** service đã có trong compose (BE-CV-SOL-010); nếu chưa, bỏ bước này và ghi vào PR.

## Kiểm thử

`cd backend-go/services/api-gateway && go test ./internal/adapter/mcpserver/tools/... ./internal/adapter/wscompat/ -run 'Parity|ChannelInventory|CodeIntel'`; `docker compose config` hợp lệ (nếu sửa compose).

## Tiêu chí hoàn thành

- [x] Parity xanh với một dòng `codeIntel.*`; bỏ dòng thì đỏ.
- [x] README ghi đủ biến và kênh.
- [x] Không thêm `max-lines` disable.

## Rủi ro và lưu ý

- CR-CV-041 thay dòng này bằng danh sách tường minh; đừng thêm `ToolSpec` codeIntel ở task này.
- Biến token trong compose dev là bí mật giả; không dùng giá trị thật.
