# AG-CV-TASK-091-08: Canary đầu-cuối và dọn tệp tạm khi lỗi

**From Solution:** [AG-CV-SOL-091-security-and-dependency-profiles](../solutions/AG-CV-SOL-091-security-and-dependency-profiles.md) mục 5.2
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-secret-canary.test.ts` (mở rộng), `quality-run-manager` (kiểm dọn)
**Depends on:** AG-CV-TASK-091-03, 091-07 (nếu có), AG-CV-TASK-081-06, 081-07
**Status:** [x] DONE

## Context

CR-091 2.5 và CR-072 (che secret). Một test chạy cả pipeline: diff chứa canary → `quality.run security-secrets-diff` → `results`, `view=log`, `runStatus`, `quality.progress/finished`.

## Việc cần làm

1. Test đầu-cuối với run manager thật + executor giả: canary trong diff; thu mọi `ws.send`, log (`AgentLogger` giả), tệp tạm sau khi kết thúc (`finished`, `failed`, `timeout`, `cancelled`).
2. Đường lỗi: công cụ crash giữa chừng → thư mục tạm của run bị dọn khi hết TTL, tệp diff tạm xoá ngay.
3. Khẳng định chuỗi canary không xuất hiện ở bất kỳ bản ghi nào.

## Kiểm thử

Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-secret-canary.test.ts`; `pnpm test`.

## Tiêu chí hoàn thành

- [x] Canary vắng ở finding, `results` thô, log, payload thông báo, tệp tạm, thông điệp lỗi.

## Rủi ro

Không phát hiện được bí mật dạng chưa có mẫu: giới hạn của bộ quét, không phải của test.
