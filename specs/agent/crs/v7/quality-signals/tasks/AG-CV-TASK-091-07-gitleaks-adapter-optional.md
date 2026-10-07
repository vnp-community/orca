# AG-CV-TASK-091-07: Bộ chuyển đổi `gitleaks` (tuỳ chọn, cần duyệt) với từ chối bản ghi chưa che

**From Solution:** [AG-CV-SOL-091-security-and-dependency-profiles](../solutions/AG-CV-SOL-091-security-and-dependency-profiles.md) mục 5.3
**Priority:** P2
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-parser-gitleaks.ts` (mới), `.test.ts`
**Depends on:** duyệt `gitleaks`; AG-CV-TASK-091-01, 091-02
**Status:** [x] DONE

## Context

CR-091 2.5: `gitleaks` bắt buộc chế độ che; parser **từ chối** bản ghi có `Secret`/`Match` chưa che.

## Việc cần làm

1. Profile `security-secrets-diff` `engine: "gitleaks"` chỉ bật khi công cụ có và được duyệt; đầu vào là **tệp diff tạm** `0600` chứa dòng thêm; xoá ngay sau chạy (cả khi lỗi/timeout/huỷ).
2. Parser: map `RuleID` → `SEC-SECRET/<loại>`; gọi `assertNoSecretFields`; bỏ `Secret`, `Match`, `Entropy`, `Fingerprint` của công cụ; `anchorOverride:""`.
3. Tham số CLI từ hằng (chưa xác nhận: `--redact`).

## Kiểm thử

Fixture giả có `Secret` chưa che → parser ném lỗi rõ và bước `failed (parser_error)`; có che → finding không chứa gì từ giá trị; tệp tạm bị xoá khi lỗi. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-parser-gitleaks.test.ts`.

## Tiêu chí hoàn thành

- [x] Canary vắng; tệp tạm biến mất mọi đường thoát.

## Rủi ro

Cờ `--redact` chưa xác nhận; gitleaks ồn.
