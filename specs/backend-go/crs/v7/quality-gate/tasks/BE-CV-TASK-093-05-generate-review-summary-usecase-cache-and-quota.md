# BE-CV-TASK-093-05: Use case `GenerateReviewSummary`: cờ 4 lớp, `ai.complete` qua infra-fleet, cache 24 h, nền

**From Solution:** BE-CV-SOL-093-ai-review-summary
**Priority:** P2
**Service:** `code-intel-service`
**File:** `internal/usecase/generate_review_summary.go`, `ai_review_ports.go`, `internal/adapter/grpcclient/ai_complete_relay.go` (mới)
**Depends on:** BE-CV-TASK-093-01, 093-04; BE-CV-SOL-013-agent-call-gate-and-quotas; BE-CV-SOL-022-snapshot-cache; BE-CV-TASK-085-06
**Status:** [x] DONE

## Việc cần làm
1. Chuỗi: cờ (`CODEINTEL_ENABLED ∧ tenant ∧ quality_gate ∧ ai_review_level≠off ∧ CODEINTEL_AI_REVIEW_ENABLED`) → `level ≤ tenant` → model ∈ allowlist → dựng + manifest → `dry_run` thoát sớm (không agent) → cache (**quyền trước cache**) → `AgentCallGate` → `RelayByDevServer("ai.complete", {prompt, format:"json", model?})` → validate (thử lại một lần) → ghi cache → audit.
2. Cache `graph_snapshots` view `aiSummary`, `params_hash` theo SOL-093 §2.4, `expires_at=now_db+24h`, ≤ 32 KiB; `force_refresh` bỏ qua.
3. Singleflight theo khoá cache; context nền 100 s + 5 s; chờ đồng bộ ≤ 24 s rồi `CODEINTEL_TIMEOUT {"inProgress":true,"retryAfterMs":3000}`.
4. Hạn mức 10/giờ/người, 1 đồng thời/binding (giá trị khởi điểm chưa hiệu chỉnh) ⇒ `CODEINTEL_RATE_LIMITED`/`CODEINTEL_CONCURRENCY_LIMIT`; không dev server ⇒ `CODEINTEL_AI_NO_RELAY`.
5. Audit `codeintel.ai.summary` (level, model, số tệp, byte, số lần che; không nội dung).

## Kiểm thử
- `AiCompleter` giả đếm lời gọi (cờ tắt ⇒ 0; `dry_run` ⇒ 0); JSON hợp lệ/hỏng/độc; chậm > 24 s ⇒ `inProgress` rồi trúng cache; đổi 1 dòng diff ⇒ miss; `read_source` thiếu không đọc cache.

## Tiêu chí hoàn thành
- [x] khớp tiêu chí SOL-093 §4; [ ] cache theo tenant/binding.

## Rủi ro
- Cách xác nhận tệp untracked không ignore chưa chốt (Q3).
