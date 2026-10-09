# TASK-FE-E2E-005: Thông báo cấu hình AI có hướng khắc phục

**From Solution:** SOL-FE-E2E-003 · **Resolves:** BUG-E2E-010
**Priority:** P1 · **Area:** frontend
**Files:**
- frontend/src/renderer/src/components/request/ (banner trong chi tiết Request và panel Solution/Plan)
- i18n
**Depends on:** TASK-FE-E2E-004, TASK-E2E-024, 034
**Status:** `[ ]` TODO

---

## Context

Hiện chỉ có "classifier error".

## Việc cần làm

- [ ] Banner theo mã lỗi/`classificationReason`: `REQUEST_AI_NOT_CONFIGURED` nêu "Dev server chưa đăng nhập Claude Code, chạy `claude auth login` trên dev server"; `REQUEST_AI_SESSION_EXPIRED` nêu đăng nhập lại; liên kết tới Settings → AI providers (admin) hoặc "liên hệ quản trị".
- [ ] Hiển thị ở phân loại, Solution, Plan.
- [ ] Không hiển thị khi AI chạy tốt.

## Tiêu chí hoàn thành

- [ ] Test render theo từng mã; Playwright (mock) cho banner.

## Kiểm thử

Vitest + Playwright.
