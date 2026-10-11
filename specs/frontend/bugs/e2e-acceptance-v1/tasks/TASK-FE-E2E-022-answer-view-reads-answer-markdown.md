# TASK-FE-E2E-022: AnswerView đọc `answer_markdown` (+ citations, confidence, assumptions, limitations) theo schema

**From Solution:** SOL-FE-E2E-005 (mục 1.1, 3.1) · **Resolves:** ô "Answer" ở tab Analysis hiện "No data" dù backend có `options.answer_markdown` (Request #1136)
**Priority:** P0 · **Area:** frontend (request / solution)
**Files:**
- frontend/src/renderer/src/components/request/solution/solution-answer-document.ts (mới) + `.test.ts`
- frontend/src/renderer/src/components/request/solution/AnswerView.tsx (sửa) + `AnswerView.test.tsx` (mới)
- frontend/src/renderer/src/i18n/locales/{en,es,ja,ko,zh}.json (khoá `auto.components.request.AnswerView.*`)
**Depends on:** không (schema: `backend-go/services/request-service/schemas/v1/answer.schema.json`)
**Status:** `[x]` XONG (2026-10-11): mã, vitest, Playwright mock và kiểm live. Kiểm live (2026-10-11): e2e `--real addproject` XANH 2/2 (Claude thật); `--real all` 7/7 XANH (24.1 phút); `--repro add-project-worktree` xanh; dữ liệu thật của người dùng không đổi.

---

## Context

`parseSolution` (`shared/request-wire-parsers.ts:303-304`) đặt `content = JSON.stringify(options)` cho kind khác `solution`. `AnswerView.tsx:12-15` chỉ có bí danh `answer`/`response`, nên `answer_markdown` bị bỏ qua: ô Answer "No data" (khi có `citations`) hoặc JSON thô (khi chỉ có `answer_markdown`).

## Việc cần làm

- [x] `solution-answer-document.ts`: `readAnswerDocument(content)`; khoá `answer_markdown`/`answerMarkdown` rồi `answer`/`response`; `citations`/`sources`/`references` (chuỗi hoặc `{ref|url|title|text, excerpt}`); `confidence` (0..1 hoặc 0..100); `assumptions`; `limitations`; `suggested_follow_up`/`suggestedFollowUp`. JSON hỏng hoặc không phải object thì trả `raw` = nguyên văn.
- [x] `AnswerView.tsx`: Answer qua `RequestMarkdownContent` (an toàn theo cách repo đang render Markdown: bỏ HTML thô, liên kết chỉ http/https); Confidence `NN%`; Citations danh sách có excerpt; Assumptions; Limitations; Suggested follow-up (chỉ văn bản). `data-testid` giữ `answer-view`; thêm `answer-confidence`, `answer-citations`, `answer-limitations`, `answer-assumptions`, `answer-follow-up`.
- [x] i18n: khoá mới ở 5 ngôn ngữ (đã có `answer`, `citations`).
- [x] Không đổi `readContentSections`, `DiagnosisView`, `FindingsView`.

## Tiêu chí hoàn thành

- Với `{"schema_version":1,"kind":"answer","answer_markdown":"Trả lời **ngắn**","confidence":0.7}` ô Answer hiện chữ in đậm và "70%"; không có "No data" ở Answer.
- `answer`/`response` cũ vẫn hiện; chuỗi không phải JSON vẫn hiện như Markdown.
- `<script>`/HTML thô và liên kết `javascript:` không được render thành phần tử.

## Kiểm thử

Vitest: `solution-answer-document.test.ts`, `AnswerView.test.tsx`, `SolutionPanel.test.tsx` (kind `answer`), `solution-locale-coverage.test.ts`. Lệnh trong `frontend/`: `NODE_OPTIONS=--no-experimental-webstorage pnpm exec vitest run --config config/vitest.config.ts src/renderer/src/components/request/solution`.

## Blast radius (codegraph)

`AnswerView` được gọi duy nhất từ `SolutionBody.tsx` (case `answer`). `readContentSections` (dùng bởi Diagnosis/Findings) không đổi. Rủi ro THẤP.

## Rủi ro · Ước lượng

THẤP. 0.5 ngày.

## Kết quả thực thi

**Trạng thái:** XONG (mã, vitest, Playwright mock; chưa kiểm live).

**Tệp đổi (frontend/src/renderer/src/...)**
- `components/request/solution/solution-answer-document.ts` (mới) + `.test.ts` (8 test): `readAnswerDocument`.
- `components/request/solution/AnswerView.tsx` (viết lại; `data-testid`: `answer-view`, `answer-confidence`, `answer-citations`, `answer-assumptions`, `answer-limitations`, `answer-follow-up`) + `AnswerView.test.tsx` (6 test, happy-dom vì `jsdom` không có trong cây này).
- i18n: `auto.components.request.AnswerView.{confidence,assumptions,limitations,followUp,followUpWithType}` (5 ngôn ngữ).
- Playwright mock: `tests/stages/e2e/legacy/request-web/request-answer-and-project-note.web.e2e.ts` (a).

**Lệch so với tài liệu:** không. `confidence` chuẩn hoá cả thang 0..1 lẫn 0..100. Ô Answer vẫn hiện "No data" khi tài liệu JSON thật sự không có văn bản trả lời.

**Blast radius (codegraph):** `AnswerView` chỉ gọi từ `SolutionBody.tsx`; `readContentSections` không đổi (Diagnosis/Findings nguyên vẹn). `SolutionPanel.test.tsx` (kind `answer`) xanh.
