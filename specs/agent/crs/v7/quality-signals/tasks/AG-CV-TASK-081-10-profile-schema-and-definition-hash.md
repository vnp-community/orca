# AG-CV-TASK-081-10: Schema profile, kiểm hợp lệ, mẫu thay argv, `definitionHash`

**From Solution:** [AG-CV-SOL-081-quality-profile-catalog-and-preflight](../solutions/AG-CV-SOL-081-quality-profile-catalog-and-preflight.md) mục 5.2
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-profile-schema.ts` (mới), `.test.ts` (mới)
**Depends on:** không
**Status:** [ ] TODO

## Context

Hợp đồng §5.1 (`definitionHash`), §9.5 (mẫu thay chỉ `{bin}`, `{files}`, `{files|x}`, `{tmp}`, `{base}`). Không thêm thư viện (viết tay).

## Việc cần làm

1. Kiểu `QualityCheckProfile`, `QualitySuite` đúng solution 5.2.
2. `validateProfile(raw: unknown): { ok: true; value } | { ok: false; errors: string[] }`: `id` `^[a-z0-9][a-z0-9-]{0,47}$`; `argv[0]` là `{bin:<tên>}` hoặc `node`; mọi token `{...}` thuộc 6 mẫu (kể cả `{gitCommonDir}`); `{files}` phải là cả một phần tử; `env.set` không có tên khớp pattern secret; `timeoutMs` 1..2_700_000; `maxOutputBytes` ≤ 64 MiB; `cwd` tương đối, không `..`, không tuyệt đối, không `\`.
3. `parseArgvTemplate(token) → Literal | Bin | Files | Tmp | Base | GitCommonDir`.
4. `definitionHashOf(profile): string` = `sha256:` + hex của JSON chuẩn tắc (khoá sắp xếp) theo 5.2; loại `title`.
5. `displayOf(profile): string` — chuỗi người đọc, không chứa env, không đường dẫn tuyệt đối (thay `{tmp:x}` bằng `<tmp>`).

## Kiểm thử

Bảng ca hợp lệ/không hợp lệ; token lạ; `{files}` nằm trong chuỗi lớn; `argv[0]` là `sh`; `cwd:"../x"`; hash ổn định theo thứ tự khoá và đổi khi đổi một phần tử argv; `title` đổi không đổi hash. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-profile-schema.test.ts`.

## Tiêu chí hoàn thành

- [ ] Mọi ca từ chối có thông điệp lỗi rõ; hash xác định.
- [ ] `display` không lộ đường dẫn ngoài worktree.

## Rủi ro

Chuẩn tắc JSON tự viết: cần test chống khác biệt khoá Unicode.
