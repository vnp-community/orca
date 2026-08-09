# TASK-FE2E-001 — Re-verify Discovery Audit trước khi implement

**Source Solution:** [SOL-FE2E-001](../solutions/SOL-FE2E-001-scope-and-discovery-audit.md)
**Priority:** P0 — Blocker cho mọi task sau
**Loại:** Verification (không sửa code)
**Depends on:** —
**Estimated:** 15 phút

---

## Context

SOL-FE2E-001 đã audit đầy đủ, nhưng thời gian giữa lúc viết solution và lúc implement có thể có commit mới. Task này re-chạy các lệnh xác nhận trước khi TASK-FE2E-002 trở đi bắt đầu sửa code — nếu có gì lệch so với solution, DỪNG LẠI và cập nhật lại solution trước khi tiếp tục.

## Việc cần làm

```bash
# 1. Xác nhận /auth/config và /auth/local vẫn chia sẻ 1 mount guard
grep -n "app.use('/auth'" -A3 -B3 backend/src/server/http-server.ts
sed -n '85,100p' backend/src/main/auth/auth-router.ts

# 2. Xác nhận inventory file chưa đổi
grep -rl "WebRuntimeClient\|WebPairingOffer\|parseWebPairingInput\|from '\.\./web-pairing'\|from '\./web-pairing'\|from '\./web-e2ee'\|StoredWebRuntimeEnvironment\|getPreferredWebPairingOffer\|AddInstanceForm\|OrcaInstanceSwitcher\|PairCodeFallback" frontend/src --include="*.ts" --include="*.tsx" | sort

# 3. Xác nhận canGeneratePairingUrl vẫn đúng như SOL-FE2E-004 mô tả
grep -n "canGeneratePairingUrl\|isWebClient" frontend/src/renderer/src/components/settings/Settings.tsx
cat frontend/src/renderer/src/lib/web-client-location.ts

# 4. Xác nhận lazyWithRetry vẫn được dùng cho WebConnect trong main-web-bootstrap.tsx
grep -n "lazyWithRetry\|WebConnect = lazy" frontend/src/renderer/src/web/main-web-bootstrap.tsx
```

## Definition of Done

- [x] Lệnh 1: output khớp — `/auth/config` (auth-router.ts:94) và `/auth/local` cùng nằm trong `if (options.authManager)` (http-server.ts:90-92)
- [x] Lệnh 2: danh sách file khớp inventory trong CR-FE2E-001 mục 2.2 — 2 file mới hợp lệ (`web-runtime-environment-crypto.ts` + `.test.ts`, đã dự đoán trước trong SOL-FE2E-001 mục 2 do BUG-FE-HLD-001), không có gì bất ngờ
- [x] Lệnh 3: `canGeneratePairingUrl={!isWebClient}` (Settings.tsx:1544) vẫn còn, `isWebClientLocation()` vẫn generic
- [x] Lệnh 4: `lazyWithRetry` vẫn import + dùng cho `WebConnect` (main-web-bootstrap.tsx:5,39)
- [x] Không có gì lệch — tiếp tục TASK-FE2E-002 trở đi

## Kết quả thực thi

**Status:** ✅ DONE — 2026-08-09. Toàn bộ 4 mục khớp chính xác với solution, không cần cập nhật lại solution nào trước khi tiếp tục.
