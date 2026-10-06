# AG-CV-TASK-091-04: Diff phụ thuộc từ `pnpm-lock.yaml`, `go.mod`, drift `package.json`

**From Solution:** [AG-CV-SOL-091-security-and-dependency-profiles](../solutions/AG-CV-SOL-091-security-and-dependency-profiles.md) mục 5.3
**Priority:** P2
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-dependency-diff-lockfiles.ts` (mới), `.test.ts`
**Depends on:** AG-CV-TASK-081-14 (base), AG-CV-TASK-082-04
**Status:** [ ] TODO

## Context

Không cần công cụ ngoài. `git show <base>:<path>` nội bộ ở agent. Phiên bản lockfile pnpm (v6/v9) cần xác nhận khi viết.

## Việc cần làm

1. `diffPnpmLock(baseText, headText) → {added, removed, bumped[{name, from, to, major:boolean}], downgraded, newSource[]}` bằng `yaml` (`packages`/`importers`), tập `name@version`; nguồn git/tarball (không phải registry) → `DEP-NEW-SOURCE` (warning).
2. `diffGoMod(base, head)` cho từng module trong `go.work`: `require`, `replace`, `exclude`; `replace` mới hoặc tới đường dẫn cục bộ → warning.
3. Drift: `package.json` đổi phần phụ thuộc mà lockfile không đổi (và ngược lại) → `DEP-LOCKFILE-DRIFT` (warning).
4. `ruleId`: `DEP-ADDED|DEP-REMOVED|DEP-MAJOR-BUMP|DEP-DOWNGRADE|DEP-LOCKFILE-DRIFT|DEP-NEW-SOURCE` (info trừ hai warning); `category:"dependency"`; `anchorOverride:"<package>@<from>-><to>"`.
5. Trần 2 000 thay đổi, `truncated`; lockfile > 5 MiB đọc theo luồng nếu cần; không ghi tệp.

## Kiểm thử

Fixture lockfile nhỏ (hai phiên bản nếu cần), `go.mod` mẫu; thêm/bớt/major/downgrade/replace/drift/git URL; lockfile hỏng → không ném (`failure`). Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-dependency-diff-lockfiles.test.ts`.

## Tiêu chí hoàn thành

- [ ] Đúng cho mọi ca ở bảng; không gọi mạng/công cụ ngoài.

## Rủi ro

Monorepo nhiều `importers` và `mobile/` lockfile riêng có thể làm sai; chưa chạy trên lockfile Orca thật (13 k dòng).
