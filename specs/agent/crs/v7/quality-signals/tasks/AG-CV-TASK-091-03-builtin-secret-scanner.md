# AG-CV-TASK-091-03: Bộ quét bí mật tích hợp trên dòng thêm của diff

**From Solution:** [AG-CV-SOL-091-security-and-dependency-profiles](../solutions/AG-CV-SOL-091-security-and-dependency-profiles.md) mục 5.2
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-secret-scanner-diff.ts` (mới), `.test.ts`
**Depends on:** AG-CV-TASK-091-02, AG-CV-TASK-083-03, AG-CV-TASK-082-04
**Status:** [x] DONE

## Context

Mặc định của profile `security-secrets-diff` (không cần công cụ ngoài).

## Việc cần làm

1. `scanAddedLines(files, opts) → RawQualityFinding[]` theo 5.2: bảng mẫu `{type, regex, messageKey}`; entropy tắt; `ruleId="SEC-SECRET/<type>"`, `severity:"error"`, `category:"security"`, `anchorOverride:""`.
2. Loại đường dẫn mặc định (cấu hình được ở hằng); bỏ nhị phân, `node_modules`, `*.lock`, `pnpm-lock.yaml`, `go.sum`.
3. Không trả `matchedText`; giá trị chỉ dùng trong hàm; không log; không ném thông điệp chứa giá trị.
4. `message`/`fixHint` cố định theo loại (vd "Có vẻ là AWS access key ID"; "Thu hồi khoá, chuyển sang Vault/biến môi trường").
5. Giới hạn: ≤ 5 000 tệp, 20 MiB diff; `truncated`.

## Kiểm thử

Bảng mẫu: mỗi loại 1 dương tính + 1 âm tính (dùng canary dựng lúc chạy); dòng cũ không bị báo; fixture/test/docs bị loại; fingerprint không đổi khi chèn dòng phía trên, không đổi theo giá trị; harness canary khẳng định giá trị vắng khỏi finding và log. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-secret-scanner-diff.test.ts`.

## Tiêu chí hoàn thành

- [x] Finding không chứa độ dài/tiền tố/ngữ cảnh; canary vắng mọi nơi.

## Rủi ro

Bắt ít hơn công cụ chuyên dụng; tỉ lệ báo nhầm chưa đo.
