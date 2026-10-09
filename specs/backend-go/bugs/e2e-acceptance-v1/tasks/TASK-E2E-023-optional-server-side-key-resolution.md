# TASK-E2E-023: Tuỳ chọn: phân giải khoá AI phía server (`resolvedApiKey`)

**From Solution:** SOL-E2E-005 · **Resolves:** BUG-E2E-010
**Priority:** P2 · **Service/Area:** request-service
**Files:**
- backend-go/services/request-service/internal/adapter/grpcclient/ai_credential_resolver.go (mới)
- backend-go/services/request-service/internal/adapter/grpcclient/request_classifier.go (`relayAIComplete`)
- backend-go/services/request-service/internal/usecase (cổng `AICredentialResolver`)
- backend-go/services/request-service/cmd/server (wiring: aiprovider, credentialbroker)
**Depends on:** TASK-E2E-006, 033
**Status:** `[ ]` CHƯA LÀM (cập nhật 2026-10-09, chưa commit)

---

## Context

Sau quyết định dùng phiên CLI, đây không còn là điều kiện thông luồng. Giữ lại cho đội muốn khoá quản trị tập trung hoặc dev server không đăng nhập CLI. `ProviderAccount.credential_ref` trỏ Vault; `credentialbroker.ResolveCredential` đọc được.

## Việc cần làm

- [-] Cổng + adapter: `ResolveProvider` → `ResolveCredential` → khoá, chỉ giữ trong bộ nhớ trong lời gọi. — _không làm (tuỳ chọn, không cần với phiên Claude đã đăng nhập)_
- [-] `relayAIComplete` thêm `resolvedApiKey` vào `params` **chỉ khi có tài khoản provider được cấu hình**; nếu không thì để agent tự chọn nguồn (env/CLI). — _không làm (tuỳ chọn, không cần với phiên Claude đã đăng nhập)_
- [-] Che khoá ở mọi lớp ghi: log, `request_audit_outbox`, kết quả trả về, `channel_args_redaction` (G8). — _không làm (tuỳ chọn, không cần với phiên Claude đã đăng nhập)_
- [-] Dùng chung cho phân loại, Solution, Plan (cùng `relayAIComplete`). — _không làm (tuỳ chọn, không cần với phiên Claude đã đăng nhập)_

## Tiêu chí hoàn thành

- [-] Có tài khoản provider: phân loại chạy khi agent không có khoá env và không đăng nhập CLI (stub HTTP). — _không làm (tuỳ chọn, không cần với phiên Claude đã đăng nhập)_
- [-] Không có chuỗi khoá trong log/audit. — _không làm (tuỳ chọn, không cần với phiên Claude đã đăng nhập)_

## Kiểm thử

Unit test + test log.

## Kết quả thực thi

Không làm (P2, tuỳ chọn). Nguồn khoá phía server không cần vì dev server dùng phiên Claude Code đã đăng nhập (quyết định 2026-10-09).
