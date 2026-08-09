# BUG-BE-HLD-010 — FleetHealthMonitor vẫn không thu thập CPU/RAM/disk/latency (re-verify: tracker cũ đánh dấu FIXED nhưng code hiện tại KHÔNG khớp)

**Mức độ:** 🟡 MEDIUM (Data integrity — Fleet Monitoring)
**Status:** 🔴 Open (⚠️ tracker cũ `BUG-BE-FLEET-002` đánh dấu "✅ FIXED — 2026-08-01", nhưng audit 2026-08-09 xác nhận vẫn broken)
**Module:** `backend/src/main/ssh/fleet-health-monitor.ts`, `fleet-health-store.ts`, `backend/src/main/runtime/rpc/fleet-metrics-handler.ts`
**Phát hiện:** 2026-08-09 (audit `backend/` code vs thiết kế — `audit/backend/backend-vs-design-review.md` §5.6/F27, xác nhận lại từ Vòng 1 §2.6)

---

## Mô tả

⚠️ **Lưu ý quan trọng:** [`specs/backend/bugs/fleet/BUG-BE-FLEET-002-health-monitor-no-relay-metrics.md`](../fleet/BUG-BE-FLEET-002-health-monitor-no-relay-metrics.md) đã ghi nhận đúng bug này từ trước và được đánh dấu **"✅ FIXED — 2026-08-01"** trong `BUG-SOLUTION-STATUS.md`. Audit lần này (đọc trực tiếp toàn bộ `fleet-health-monitor.ts`/`fleet-health-store.ts` hiện tại) xác nhận **status "FIXED" đó không chính xác** — code hiện tại vẫn giữ nguyên hành vi bug mô tả ban đầu. Ticket này mở lại với bằng chứng mới, đề nghị cập nhật lại `BUG-SOLUTION-STATUS.md`.

`docs/features/F27-fleet-health-monitoring.md` yêu cầu poll CPU/RAM/disk/latency mỗi server. Thực tế:
- `runHealthCheck()` (`fleet-health-monitor.ts:52-86`) chỉ đọc `SshConnectionStatus` đã có sẵn (callback `getConnectionState`) — không exec lệnh đo CPU/RAM/disk, không ping đo latency.
- Field `pingLatencyMs` tồn tại trong type `HealthRecord` (`fleet-health-store.ts:7-15`) nhưng **không bao giờ được ghi giá trị** trong `recordConnectionState()` (`fleet-health-monitor.ts:61-67`) — dead field.
- Khái niệm CPU/RAM/disk **hoàn toàn không tồn tại** trong bất kỳ type nào (`HealthRecord`, `FleetServerStatus`/`FleetStatusReport`).
- Prometheus `/metrics` (`fleet-metrics-handler.ts:21-92`) chỉ có `orca_server_connected/uptime_seconds/uptime_24h_percent/reconnect_attempts/fleet_health_score/fleet_servers_total/connected/error` — **không có `orca_server_cpu_percent`** hay bất kỳ metric RAM/disk/latency nào.
- `DEFAULT_PING_INTERVAL_MS = 60_000` (60s) — tài liệu yêu cầu 30s.

## Hậu quả

- Admin dashboard không có dữ liệu CPU/RAM/disk thật — chỉ biết connected/disconnected.
- Threshold alerting (cpu>90%, disk>95%) không thể hoạt động vì không có input.
- Webhook alert (đã có, format Slack-style) không bao giờ trigger vì lý do resource — chỉ trigger theo connection state.

## Bằng chứng

- `backend/src/main/ssh/fleet-health-monitor.ts:52-86` — `runHealthCheck()` không exec lệnh đo resource.
- `backend/src/main/ssh/fleet-health-monitor.ts:8` — `DEFAULT_PING_INTERVAL_MS = 60_000`.
- `backend/src/main/ssh/fleet-health-store.ts:7-15` — `pingLatencyMs?: number` không bao giờ set.
- `backend/src/main/runtime/rpc/fleet-metrics-handler.ts:34-83` — danh sách metric Prometheus thật, không có cpu/ram/disk.

## Đề xuất fix

(Giữ nguyên đề xuất gốc của BUG-BE-FLEET-002, chưa được áp dụng thật vào code)
1. Thêm `relay.call('health.get')`/SSH exec `top`/`df` để lấy CPU/RAM/disk thật từ Dev Server.
2. Đo `pingLatencyMs` bằng round-trip time của chính request `health.get`.
3. Đổi `DEFAULT_PING_INTERVAL_MS` về 30s theo doc, hoặc cập nhật lại doc nếu 60s là quyết định có chủ đích (cần xác nhận với team).
4. Thêm metric Prometheus `orca_server_cpu_percent`/`ram_percent`/`disk_percent`/`latency_ms`.
5. **Cập nhật `BUG-SOLUTION-STATUS.md`**: đổi status của `BE-FLEET-002` từ ✅ về 🔴, ghi chú rõ "solution đề xuất chưa được áp dụng vào code, xác nhận lại 2026-08-09".

## Tham khảo

- Audit: `audit/backend/backend-vs-design-review.md` §5.6 (F27), §2.6, §6 mục 7 (Top 10)
- Doc gốc: `docs/features/F27-fleet-health-monitoring.md`
- Bug gốc (status cần sửa): [`specs/backend/bugs/fleet/BUG-BE-FLEET-002-health-monitor-no-relay-metrics.md`](../fleet/BUG-BE-FLEET-002-health-monitor-no-relay-metrics.md)
