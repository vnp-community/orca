# AG-CV-TASK-091-02: Che bí mật, bộ dụng cụ canary (không giữ giá trị)

**From Solution:** [AG-CV-SOL-091-security-and-dependency-profiles](../solutions/AG-CV-SOL-091-security-and-dependency-profiles.md) mục 5.2
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-secret-redaction.ts` (mới), `agent/src/relay/quality-secret-canary.test.ts` (mới)
**Depends on:** AG-CV-TASK-081-02
**Status:** [x] DONE

## Context

CR-091 2.5; hợp đồng §9.4. Nền tảng cho task 03, 07, 08.

## Việc cần làm

1. `assertNoSecretFields(record)`: từ chối (ném) bản ghi công cụ có khoá `Secret|Match|RawSecret|secret|match|Fingerprint(gitleaks)` mang giá trị chưa che; dùng ở parser gitleaks.
2. `makeCanary()` (chỉ trong test): sinh chuỗi dạng `AKIA` + 16 ký tự ngẫu nhiên ghép lúc chạy; `scanTreeForCanary(dirs, canary)` quét đệ quy tệp trong thư mục tạm/log; hàm kiểm khẳng định chuỗi vắng trong mọi đầu ra.
3. Tài liệu hoá: không bao giờ băm/độ dài/tiền tố từ giá trị.

## Kiểm thử

Test harness tự kiểm: canary có mặt trong dữ liệu mẫu thì `scanTreeForCanary` thấy; sau redaction thì không. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-secret-canary.test.ts`.

## Tiêu chí hoàn thành

- [x] Harness tái dùng được ở task 03, 07, 08 và bởi AG-CV-SOL-072.

## Rủi ro

Mã test không được chứa chuỗi bí mật nguyên văn (kể cả dạng giả) để không tự kích hoạt bộ quét bí mật của repo.
