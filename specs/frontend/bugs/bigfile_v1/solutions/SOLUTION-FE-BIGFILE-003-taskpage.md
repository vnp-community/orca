# SOLUTION-FE-BIGFILE-003 — Tách `TaskPage.tsx` (12,833 dòng)

**Bug:** `../BUG-FE-BIGFILE-003-taskpage.md`
**Trạng thái:** 🔄 In progress — Giai đoạn 1 done (TASK-BIGFILE-027..031), Giai
đoạn 2 investigation done (TASK-BIGFILE-032), Move con 235–241 chưa thực thi
**Thứ tự thực hiện:** #8 — xem `SOLUTION-FE-BIGFILE-001` mục 3
**Chiến lược:** Barrel/facade (xem `SOLUTION-FE-BIGFILE-001` mục 1)

---

## Giai đoạn 1 — DONE (TASK-BIGFILE-027..031)

Kết quả thật (đọc lại file sau khi tách, không phải ước tính ban đầu):
`TaskPage.tsx` 12,833 → **10,664 dòng**. Nhóm GitHub cell (dự kiến 1 file
~1,880 dòng) thực tế phải tách thành **2 file** vì vượt ngân sách oxlint
max-lines 400/`.tsx` ngay cả sau khi chia đôi (923 và 950 dòng — cả 2 đều cần
`eslint-disable max-lines` + baseline entry, giống pattern
`orca-runtime-tail-buffer.ts` ở TASK-BIGFILE-008).

| File mới | Nội dung | Dòng gốc (xác nhận thật) | Dòng thật |
|---|---|---|---|
| `task-page-types.ts` | `LinearProjectTab`, `LinearGroupSection`, `LinearIssueListRow` | 600–610 (KHÔNG phải 600–650 — vùng 611–650 là helper khác, ở lại) | 13 |
| `task-page-linear-cells.tsx` | `LinearStateCell` | 642–800 | 173 |
| `task-page-jira-banner.tsx` | `TaskPageJiraErrorBanner` | 717–761 (đã dịch chuyển do 2 bước trước) | 52 |
| `task-page-github-assignee-cells.tsx` | `GHStatusCell`, `GitHubAssigneeAvatar`, `GitHubIssueLabelSelector`, `GitHubIssueAssigneeSelector`, `GHAssigneesCell` | ~859–1799 (rải rác, xen giữa còn `formatPRDelta`/`sameOptionalGitHubOwnerRepo` KHÔNG chuyển — dùng bởi component chính, không phải 9 cell) | 949 (cần baseline max-lines) |
| `task-page-github-review-cells.tsx` | `ReviewChipAvatar`, `PRReviewCell`, `PRChecksCell`, `PRMergeCell` + helper riêng (`getChecksLabel`, `getChecksPillTone`, `mergeReviewerSuggestions`, `buildRequestedReviewUsers`) | ~1302–2715 (rải rác) | 986 (cần baseline max-lines) |
| `task-page-pagination.tsx` | `PaginationBar` + helper riêng `getPageNumbers` (task doc gốc không liệt kê `getPageNumbers` nhưng nó chỉ dùng bởi `PaginationBar` — chuyển theo) | 896–1000 | 111 |
| `TaskPage.tsx` (giữ nguyên tên) | `export default function TaskPage(...)` (component chính, dòng 887–cuối) + module-scope helper còn lại + `import`/`export` barrel cho 6 file trên | 1–10,664 | 10,664 |

**Bài học ranh giới** (giống TASK-BIGFILE-001..007): task doc gốc ước lượng
dòng bằng cách nhìn tổng khối liên tục, nhưng thực tế xen giữa các cell
component có nhiều helper module-scope KHÔNG thuộc về cell nào cả (dùng bởi
chính component `TaskPage`) — ví dụ `formatPRDelta`, `sameOptionalGitHubOwnerRepo`
nằm giữa các GitHub cell nhưng chỉ được gọi trong `TaskPage` (dòng ~9000+),
phải ở lại `TaskPage.tsx`. Tương tự `getPageNumbers` nằm ngay trước
`PaginationBar` nhưng KHÔNG có trong danh sách symbol gốc — xác nhận bằng
grep usage (chỉ `PaginationBar` gọi) rồi chuyển theo để tránh vòng import
ngược.

**Cạm bẫy phát hiện khi thực thi**: pattern `export { X } from './module'`
tại vị trí định nghĩa cũ CHỈ hoạt động nếu không còn nơi nào trong
`TaskPage.tsx` dùng `X` trực tiếp trong JSX/type annotation cùng file — vì
`export ... from` là re-export thuần, KHÔNG tạo local binding. Cả 9/9 GitHub
cell + `PaginationBar` + `TaskPageJiraErrorBanner` + `LinearStateCell` đều
được `TaskPage` tự render trong JSX của chính nó, nên mỗi file cần THÊM
`import { X } from './module'` ở đầu file (giữ `export { X } from './module'`
song song cho khả năng import từ ngoài, dù hiện tại chưa ai import) — xác
nhận bằng oxlint `jsx-no-undef` sau mỗi bước, không chỉ tsc (tsc không luôn
bắt được JSX-undefined-value do JSX compile ra `React.createElement(X, ...)`
với X unresolved chỉ báo runtime, nhưng oxlint's `react/jsx-no-undef` bắt
được tĩnh).

Test phụ thuộc vào layout cũ: `feature-interaction-writer-boundaries.test.ts`
đọc **source text thô** của `TaskPage.tsx` bằng `indexOf('function X', ...)`
để xác nhận `recordFeatureInteraction(...)` nằm trong đúng khối — đây là
loại phụ thuộc mà `gitnexus impact`/`tsc` KHÔNG phát hiện được (không phải
symbol reference, là string matching trên nội dung file). Phải cập nhật các
`sourceBetween(...)` liên quan để đọc từ file mới thay vì `TaskPage.tsx` —
xem diff của TASK-BIGFILE-028/030.

## Giai đoạn 2 — Investigation (TASK-BIGFILE-032, DONE) — số liệu thật thay ước đoán

### ⚠️ Con số 83 state / 58 effect trong bản gốc là SAI — thực tế 153 / 57

Task doc TASK-BIGFILE-032 (viết trước khi Giai đoạn 1 chạy) ước đoán 83
`useState`/58 `useEffect` bằng cách nhìn sơ bộ. Sau khi Giai đoạn 1 hoàn tất
và đọc lại bằng script (không đọc tuyến tính 9,778 dòng — dùng regex
multi-line qua Python để bắt đúng cả các khai báo `useState<...>` xuống
dòng, giống cách `TASK-BIGFILE-035` dùng field-span thay vì đọc code trực
tiếp):

- **`useState`: 153** (gần gấp đôi ước tính gốc — lý do: đếm tay/grep nông
  bỏ sót các khai báo `useState<Type>(` mà `<Type>` xuống dòng riêng, ví dụ
  `const [x, setX] =\n    useState<Foo | null>(null)`; `grep -c "useState("`
  chỉ đếm được 70 vì literal `"useState("` không xuất hiện trên cùng 1 dòng
  ở nhiều khai báo).
- **`useEffect`: 57** (khớp gần đúng ước tính gốc 58 — pattern `useEffect(`
  luôn nguyên vẹn trên 1 dòng nên đếm nông vẫn đúng ở đây).

### Phân nhóm 153 `useState` theo domain (dòng 988–3722, xác nhận bằng regex)

| Domain | Số state | Dòng | Ghi chú |
|---|---|---|---|
| Bootstrap/chung (KHÔNG tách được) | 4 | 988, 1130, 1135, 1508 | `repoSelection`, `taskSource`, `runtimePreflightStatusByHostId`, `taskResumeApplied` — đọc bởi effect của MỌI domain khác |
| GitHub (issues/PR list, pagination, new-issue draft) | 24 | 1548–1939 | `githubMode`, `taskSearchInput`...`taskRefreshNonce` (9), `pages`...`totalItemCount` (5), `dialogInitialTab`, `retryingSourceKeys`, `newIssueOpen`...`newIssueRepoId` (7) |
| GitLab | 9 | 1554–1570 | `gitlabFilter`, `gitlabItems`, `gitlabLoading`, `gitlabError`, `gitlabRefreshNonce`, `gitlabDialogItem`, `gitlabView`, `gitlabTodos`, `gitlabTodosLoading` — **domain KHÔNG có trong đề xuất 4-hook gốc** (bị bỏ sót hoàn toàn) |
| Linear — chọn/mở issue | 3 | 2051–2054 | `selectedLinearIssueId`, `selectedLinearIssueFallback`, `selectedLinearIssueCanFloat` |
| Linear — browse (issues/projects/custom-views/board/pagination) | 48 | 2225–2303 | Khối lớn nhất trong toàn bộ component |
| Linear — teams | 2 | 2645–2646 | `availableTeams`, `linearTeamRefreshNonce` |
| Linear — team filter | 1 | 2903 | `linearTeamSelection` |
| Linear — new-project draft | 12 | 3533–3544 | |
| Linear — new-issue draft | 13 | 3568–3586 | |
| Linear — connect dialog | 1 | 3668 | `linearConnectOpen` |
| **Linear tổng** | **80** | | **52% tổng 153 state — 1 hook duy nhất `useTaskPageLinearState()` như đề xuất gốc SẼ vẫn là 1 file to (tái tạo vấn đề bigfile ở quy mô nhỏ hơn), cần tách tiếp theo sub-domain** |
| Jira — chọn/mở issue | 2 | 2164–2165 | |
| Jira — browse (issues/priorities/status order) | 12 | 2420–2434 | |
| Jira — projects | 2 | 2690–2691 | |
| Jira — new-issue draft | 15 | 3699–3714 | |
| Jira — connect dialog | 5 | 3717–3722 | |
| **Jira tổng** | **36** | | 24% tổng |
| **Tổng** | **153** | | |

### Phân tích 57 `useEffect` — coupling chéo domain (trích dependency array thật)

Trích toàn bộ 57 dependency array bằng script (bracket-depth matching qua
nhiều dòng, không dùng regex 1 dòng vì phần lớn effect có dep array xuống
dòng theo từng phần tử). Phát hiện chính:

1. **`taskSource` xuất hiện trong dependency array của ≥20/57 effect**, trải
   đều cả 4 domain (GitHub, GitLab, Linear, Jira) — đây là biến "provider
   đang active", mọi domain đều phải đọc để biết có nên fetch/render hay
   không. Không thể là state riêng của domain nào.
2. **`taskResumeApplied` xuất hiện trong ≥15/57 effect**, cũng trải đều 4
   domain — cờ "đã áp dụng resume-from-persisted-UI hay chưa", dùng để mỗi
   domain effect tự quyết có nên chạy fetch lần đầu hay đợi resume xong.
   Cùng nhóm với `taskSource` — 2 biến này PHẢI được truyền vào mỗi hook
   domain như tham số đọc (không sở hữu), giống cách `OrcaRuntimeService`
   inject lại field lõi qua constructor cho các domain class tách ra
   (TASK-BIGFILE-036..040).
3. **4 effect thực sự cross-domain** (đọc/ghi state từ ≥2 domain khác nhau
   trong CÙNG 1 effect — không thể gán cho 1 hook domain nào mà không kéo
   theo phụ thuộc ngược):
   - Dòng 3726–3762: dep gồm `newJiraIssueOpen` VÀ `newLinearIssueOpen` —
     đóng draft của bên kia khi bên này mở (mutual-exclusion giữa 2
     create-dialog khác provider).
   - Dòng 4523–4567: dep gồm `newIssueOpen` (GitHub), `newLinearProjectOpen`
     + `newLinearIssueOpen` (Linear), `newJiraIssueOpen` (Jira), `activeModal`,
     `dialogWorkItem`, `githubMode`, `taskSource` — effect quản lý
     "modal nào đang mở" xuyên suốt cả 3 provider.
   - Dòng 5114–5157: tương tự — xử lý phím Escape đóng dialog, đọc
     `newIssueOpen`/`newLinearIssueOpen`/`newJiraIssueOpen`/`selectedLinearIssue`
     cùng lúc.
   - Dòng 5167–5177: `checkJiraConnection` + `checkLinearConnection` +
     preflight-status context key của cả Jira lẫn Linear trong 1 effect.
   - **Kết luận**: 4 effect này là lớp điều phối UI xuyên-provider
     ("chỉ 1 dialog mở tại 1 thời điểm", "Esc đóng dialog đang mở", "check
     preflight cả 2 provider cùng lúc") — PHẢI ở lại `TaskPage.tsx` (hoặc 1
     hook điều phối riêng nhận tất cả state mở/đóng làm tham số), KHÔNG
     Move vào hook domain đơn lẻ. Giống hệt phát hiện ở
     `TASK-BIGFILE-054` cho `ptyTitleTrackersByPtyId` — field/state trông
     cô lập nhưng thân effect gọi chéo domain khác.

### Đề xuất hook — điều chỉnh từ 4 xuống 7 theo domain thật (không đoán theo tên type)

Đề xuất gốc (`useTaskPageLinearState`/`useTaskPageGitHubState`/
`useTaskPageJiraState`/`useTaskPageFilters`) **không dùng được trực tiếp**:
bỏ sót GitLab hoàn toàn, và gộp Linear (80 state) / Jira (36 state) thành 1
hook mỗi domain sẽ tái tạo vấn đề file to ở quy mô hook. Đề xuất mới, đã
sinh task Move con TASK-BIGFILE-235..241 (dải số riêng của nhóm xử lý task
này, xem `TASKS-INDEX.md`):

| # | Hook | Domain | State | Effect liên quan (dòng) | Input tham số chia sẻ |
|---|---|---|---|---|---|
| 235 | `useTaskPageGitLabState()` | GitLab | 9 | 1767–1769, 2749–2900 | `taskSource`, `selectedRepos`/`primaryRepo` |
| 236 | `useTaskPageGitHubState()` | GitHub issues/PR + new-issue draft | 24 | 1666–2040, 4052–4383 (gồm 1 effect ~196 dòng) | `taskSource`, `taskResumeApplied`, `selectedRepos` |
| 237 | `useTaskPageJiraDraftState()` | Jira new-issue draft + connect dialog | 20 | 3726(chung)–3957 | `settings`, `jiraConnected`, `jiraTaskSourceContext` |
| 238 | `useTaskPageJiraBrowseState()` | Jira issues/priorities/projects/chọn issue | 16 | 2206–2208, 2446–2472, 2693–2724, 5693–5812 | `taskSource`, `taskResumeApplied`, `jiraConnected` |
| 239 | `useTaskPageLinearDraftState()` | Linear new-project + new-issue draft + connect dialog | 26 | 3561–3666 | `linearConnected`, `settings`, `linearTaskSourceContext` |
| 240 | `useTaskPageLinearTeamsState()` | Linear teams + team-filter + attribute-filter reconcile | 3 (+ dùng chung `linearAttributeFilter` từ nhóm browse) | 2648–2679, 3045–3099 | `taskSource`, `taskResumeApplied`, `linearConnected` |
| 241 | `useTaskPageLinearBrowseState()` | Linear issues/projects/custom-views/board/pagination/chọn-issue | 51 | 2106–2112, 3212–3251, 5194–5683 (nhiều effect, khối lớn nhất còn lại) | `taskSource`, `taskResumeApplied`, `linearConnected`, output của 240 |

**KHÔNG sinh task Move cho**: state bootstrap 4 biến
(`repoSelection`/`taskSource`/`runtimePreflightStatusByHostId`/
`taskResumeApplied`) và 4 effect cross-domain liệt kê ở trên (dòng
3726–3762, 4523–4567, 5114–5157, 5167–5177) — đây là lõi điều phối dùng
chéo, giống nhóm field lõi `ptyController`/`notifier`/`graphStatus`... của
`OrcaRuntimeService` (TASK-BIGFILE-035). Tách các effect này vào hook domain
bất kỳ sẽ tạo phụ thuộc ngược (hook A phải import setter của hook B) — cần
1 lớp điều phối riêng (`useTaskPageModalCoordination()` hoặc giữ nguyên
trong `TaskPage`) mà bản thân việc thiết kế nó nằm ngoài phạm vi Investigate
này, để lại cho 1 task riêng SAU KHI 235–241 hoàn tất và có thể nhìn rõ hình
dạng phần lõi còn sót lại.

## Nguyên tắc bắt buộc cho MỖI task Move sinh ra (235–241)

1. Đây KHÔNG phải Move cơ học thuần (khác 027–031) — tách state ra custom
   hook đổi call site từ `const [x, setX] = useState(...)` thành
   `const { x, setX } = useTaskPageXState(...)` tại HÀNG CHỤC điểm dùng
   trong JSX/handler của `TaskPage`. Đọc kỹ toàn bộ điểm tham chiếu (grep
   tên biến qua TOÀN FILE, không chỉ trong khối state/effect) trước khi tách.
2. `gitnexus impact` cho tên state đại diện của domain (vd `linearIssues`)
   trước khi tách — dừng nếu risk HIGH/CRITICAL.
3. Truyền `taskSource`/`taskResumeApplied` (và các biến bootstrap khác cần)
   làm THAM SỐ đọc vào hook, KHÔNG cố sở hữu chúng trong hook.
4. Chạy lại TOÀN BỘ `feature-interaction-writer-boundaries.test.ts` sau mỗi
   task — nhiều marker string trong test này neo vào tên hàm/biến trong
   đúng domain đang tách.
5. 1 domain = 1 commit riêng. Thứ tự đề xuất: 235 (GitLab, nhỏ nhất, ít
   effect nhất) trước để xác nhận pattern hoạt động đúng, giống khuyến nghị
   ở TASK-BIGFILE-035 (làm domain nhỏ nhất trước).

## Xác minh

- `pnpm exec tsc --noEmit -p frontend/tsconfig.json` (dùng `--composite
  false` nếu chạy song song nhiều nhóm khác — tsbuildinfo dùng chung gây
  nhiễu kết quả giữa các phiên chạy đồng thời, đã xác nhận thực tế khi làm
  TASK-BIGFILE-027..031)
- `pnpm exec oxlint <đúng file đã đổi>`
- `pnpm exec vitest run --config config/vitest.config.ts
  src/renderer/src/components/feature-interaction-writer-boundaries.test.ts`
- `gitnexus detect_changes({scope: "all"})` (lưu ý: gitnexus CLI/MCP có thể
  segfault khi scope theo `repo` trong môi trường nhiều repo được index —
  xác nhận lại bằng grep thủ công nếu gitnexus không khả dụng)
- `node scripts/find-frontend-bigfiles.mjs`

## Rủi ro

- Giai đoạn 1: **Thấp, đã xác nhận** — done, không phát sinh vấn đề ngoài
  việc phải thêm `import` song song với `export { ... } from` (JSX cần local
  binding) và cập nhật 1 test đọc source text thô.
- Giai đoạn 2 (235–241): **Trung bình-Cao** — ranh giới domain đã xác nhận
  rõ bằng dependency-array thật (không còn là phỏng đoán), nhưng khối lượng
  điểm tham chiếu cần sửa trong `TaskPage` chính rất lớn (hàng trăm chỗ dùng
  state/setter trong JSX + handler). Phần lõi cross-domain (4 effect +
  4 state bootstrap) CHƯA có thiết kế — để lại cho task riêng sau 235–241,
  giống cách `RuntimeGraphStore` (TASK-BIGFILE-041) được để lại sau khi tách
  5 domain an toàn của `OrcaRuntimeService`.
