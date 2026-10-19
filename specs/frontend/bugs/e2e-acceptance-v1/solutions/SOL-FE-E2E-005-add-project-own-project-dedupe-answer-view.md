# SOL-FE-E2E-005: "Add Project" tạo PROJECT RIÊNG cho mỗi repo; chặn thêm trùng repo; Create request chọn project rõ ràng; ô "Answer" đọc `answer_markdown`

**Resolves:** ba lỗi từ lượt test tạo Request #1136 (2026-10-11) · **Tasks:** [TASK-FE-E2E-022](../tasks/TASK-FE-E2E-022-answer-view-reads-answer-markdown.md) đến [026](../tasks/TASK-FE-E2E-026-i18n-tests-and-mock-specs.md) · **Backend đi kèm:** agent AE (`PROJECT_REPO_ALREADY_REGISTERED`, `project.create` luôn tạo repo hoặc báo lỗi; không thuộc tài liệu này, nhưng frontend KHÔNG phụ thuộc vào nó để chạy: xem mục 3.6)

Mọi `file:dòng` là mã THẬT đã đọc ngày 2026-10-11 (frontend ở `frontend/src/renderer/src/...`, trừ khi ghi khác). Tài liệu này được viết TRƯỚC khi sửa mã.

## Quyết định đã chọn (người dùng đã chốt, không hỏi lại)

1. Mỗi lần "Add Project" (thư mục có sẵn) tạo một project MỚI chứa đúng repo đó, gắn dev server đã chọn, tên mặc định = tên thư mục (sửa được trong hộp thoại). Luồng này không dùng "project mặc định theo host" nữa. Clone from URL và Create on host (cùng hộp thoại) làm tương tự, tên = tên repo.
2. Repo đã đăng ký (cùng dev server + cùng đường dẫn đã chuẩn hoá) thì CHẶN tạo trùng và chỉ đường tới project đang chứa nó ("Mở project đó").
3. Create request chọn được project như hiện có (không đổi backend Request); chỉ thêm hiển thị rõ repo mà Request sẽ chạy.
4. Dữ liệu cũ không tự động di chuyển. Các project mặc định cũ (ví dụ "Demo (this Mac)", "My Repos") và mọi nơi khác gọi `getOrCreateDefaultProject` giữ nguyên hành vi.

## 1. Ba lỗi và nguyên nhân đã xác minh

### 1.1 Lỗi 1: ô "Answer" ở tab Analysis hiện "No data" dù backend có `options.answer_markdown`

- Backend lưu tài liệu trả lời theo `backend-go/services/request-service/schemas/v1/answer.schema.json`: bắt buộc `schema_version`, `answer_markdown` (1..8000 ký tự); tuỳ chọn `kind:"answer"`, `confidence` (0..1), `citations[]` (`{ref, excerpt?}`), `limitations[]`, `suggested_follow_up {type: change_request|task, title}`, `open_questions[]`. Mẫu hợp lệ: `testdata/artifacts/valid/answer_ok.json`.
- `shared/request-wire-parsers.ts:276-312` `parseSolution`: với `kind` khác `solution`, `options` là một OBJECT (tài liệu AI); không có `options.options` nên `content = JSON.stringify(optObj)` (dòng 303-304). Vậy `solution.content` là JSON có khoá `answer_markdown`.
- `components/request/solution/AnswerView.tsx:12-15` chỉ khai báo bí danh `answer`/`response` cho phần trả lời và `citations`/`sources`/`references` cho nguồn. `solution-content-sections.ts:36-56` `readContentSections` chỉ giữ section có dòng; khi JSON có `citations` nhưng không khớp bí danh nào của `answer`, `sections` khác rỗng nên không rơi về `fallbackText`, và `SolutionSection.tsx:27-31` in "No data" cho Answer. Khi JSON chỉ có `answer_markdown` + `confidence` thì `sections` rỗng và màn hình in nguyên JSON thô (cũng sai).
- `toLines` (`solution-content-sections.ts:17-30`) ghép `title/text/summary/name/ref/url`, nên citation `{ref, excerpt}` mất `excerpt`.
- Hệ quả phụ: `confidence`, `limitations`, `suggested_follow_up` không bao giờ được hiển thị.

### 1.2 Lỗi 2: "Add Project" không tạo được project riêng

- `store/slices/repos.ts:3103` `addRepoPath` → nhánh `target.kind==='local'` gọi `window.api.repos.add` (preload web `web/web-preload-api.ts:1511` `add` → `getOrCreateDefaultProjectId(hostId)` dòng 1515), nhánh `environment` gọi `getOrCreateDefaultProject(target, devServerId)` (`repos.ts:3134`). Cả hai gom repo vào project "mặc định theo dev server": `lib/repo-host-binding.ts:90-120` `resolveProjectIdForDevServer` chọn project SỚM NHẤT gắn host đó; chỉ khi chưa có mới tạo. Với người dùng đã có "Demo (this Mac)" gắn `dev-local-mac`, mọi repo mới rơi vào đó.
- Đường clone (`web-preload-api.ts:1572-1597` `clone`, `lib/clone-repo-onto-dev-server.ts:28`), create (`web-preload-api.ts:1615-1640` `create`, `components/sidebar/useCreateRepo.ts:182`), `addRemote` (`web-preload-api.ts:1602`, `runtime/runtime-repo-client.ts:96`) cũng dùng project mặc định.
- Project riêng chỉ tạo được qua "Project Workspace (Beta)" → `ProjectSwitcher` → "Create New Project" (`components/project/CreateProjectDialog.tsx:144-197`: Name, Dev Server, Repo Path, Visibility; không có Browse folder). Người dùng bị ép đi vòng.
- Hệ quả: Request chạy trên repo ĐẦU TIÊN của project (theo `position`): `backend-go/services/request-service/internal/adapter/grpcclient/ai_connection_resolver.go:54-62` và `task-service/internal/adapter/grpcclient/project_execution_resolver.go:132` đều lấy `repos[0]`. Một project chứa nhiều repo thì Request chạy trên repo không như người dùng nghĩ.

### 1.3 Lỗi 3: sidebar có hai mục cùng đường dẫn

- `CreateProjectDialog.tsx:116-128` `findDuplicateRepo` chỉ so `r.path` (bỏ `/` cuối) và `r.executionHostId`; khi trùng chỉ hiện cảnh báo amber `cp-duplicate-repo-warning` (dòng 321-331: "will NOT link ... independent") và VẪN cho tạo (`submitDisabled` dòng 245-248 không xét `duplicateRepo`). Kết quả: hai mục cùng đường dẫn trong sidebar.
- `repos.ts:3196-3239` `addRepoPath` có `alreadyAdded` nhưng so theo `getRepoHostIdentity` = (host, **repo id**): repo mới luôn có id mới nên không bao giờ coi là trùng khi backend cho phép hai hàng cùng `url`.
- Đường "Add Project" dùng `repo.add` / `project.create` mà backend (trước AE) không chặn trùng `url + devServerId`.

## 2. Nguyên nhân gốc

(1) Hợp đồng đọc tài liệu AI không bám schema (bí danh chọn tay, thiếu `answer_markdown`). (2) "Project" ở luồng Add Project được hiểu là "thùng chứa theo host" thay vì "đơn vị công việc một-repo"; mọi đường tạo repo đều đi qua hàm "lấy hoặc tạo project mặc định". (3) Không có bước kiểm "repo này đã có ở đâu" ở phía người dùng; backend cũng không chặn.

## 3. Thiết kế

### 3.1 AnswerView đọc đúng schema (TASK-FE-E2E-022)

- Tệp mới `components/request/solution/solution-answer-document.ts`: `readAnswerDocument(content)` trả `{ answerMarkdown, confidence, citations[{ref, excerpt?}], limitations[], assumptions[], followUp?, raw: string | null }`. Khoá nhận: `answer_markdown`/`answerMarkdown`, rồi `answer`/`response` (tương thích cũ); `citations`/`sources`/`references` (phần tử chuỗi hoặc `{ref|url|title|text, excerpt}`); `confidence` (số 0..1 hoặc 0..100, chuẩn hoá thành %); `limitations`; `assumptions` (chuỗi hoặc `{text}`); `suggested_follow_up`/`suggestedFollowUp`. JSON hỏng hoặc không phải object thì `raw` = nguyên văn và `AnswerView` in như Markdown (giữ hành vi cũ của `fallbackText`). Không có `answer_markdown` nhưng có các khoá khác thì vẫn hiện các phần đó và để trống ô Answer với "No data" đúng nghĩa.
- `AnswerView.tsx` hiển thị: khối Answer qua `RequestMarkdownContent` (đã an toàn: bỏ HTML thô, chỉ liên kết http/https, ảnh thành văn bản; `RequestMarkdownContent.tsx:1-50`), huy hiệu Confidence (`NN%`), Citations dạng danh sách (ref + excerpt trích dẫn), Assumptions, Limitations, "Suggested follow-up" (chỉ văn bản; không tạo hành động mới). Mọi chuỗi qua `translate('auto.components.request.AnswerView.*')`, thêm khoá mới ở 5 ngôn ngữ.
- `readContentSections` và `Diagnosis/FindingsView` KHÔNG đổi (không có bằng chứng lỗi ở hai view đó trong lượt test; xem 5 "chưa xác minh").

### 3.2 "Add Project" tạo project riêng (TASK-FE-E2E-023)

Một đơn vị logic dùng chung cho hai transport (preload web `callRuntimeResult` và store `callRuntimeRpc(target, ...)`):

- Tệp mới `lib/own-project-registration.ts`: `registerRepoInOwnProject(rpc, {devServerId, path, projectName, kind?})`:
  1. Kiểm trùng (3.4) trước mọi ghi; trùng thì ném `RepoAlreadyRegisteredError`.
  2. `project.create {name: projectName, devServerId, repoPath: path, visibility: 'private'}` (gateway `channels_tenant_project.go:449-480` nhận cả `devServerId` lẫn `repoPath`; project-service `usecase/create_project.go` kiểm host, health và `fs.checkPath` TRƯỚC khi ghi, rồi tạo repo).
  3. `repo.list {projectId}` để lấy `RemoteRepoView` của repo vừa tạo (so theo đường dẫn đã chuẩn hoá). Nếu backend cũ không tạo repo thì bù bằng `repo.add {projectId, url, displayName, devServerId}` (chính là bước mà `CreateProjectDialog.tsx:189-196` đã làm); nếu bù lỗi thì ném lỗi có tên project (không tự xoá project: thao tác xoá dữ liệu không do mã UI tự quyết định).
  4. Lỗi `PROJECT_REPO_ALREADY_REGISTERED: ... already in project <name> (<id>)` được ánh xạ thành `RepoAlreadyRegisteredError`.
- Tên mặc định: `getRuntimePathBasename(path)` bỏ hậu tố `.git` (`shared/cross-platform-path.ts:49-55`; không giả định `/`), người dùng sửa được.
- `useAddRepoServerPathFlow` thêm trạng thái `projectName` (+ cờ đã sửa tay) và truyền xuống `addRepoPath(path, kind, {devServerId, ownProject: {name}})`; `AddRepoServerStartStep` (bước "Open host project") có ô "Project name" (`data-testid="add-project-name"`). Clone/Create tự dùng tên repo (không thêm ô).
- `store/slices/repos.ts` `addRepoPath`: tuỳ chọn `ownProject?: {name}`; khi có (và có `devServerId`) cả hai nhánh (`local`→preload web, `environment`→`callRuntimeRpc`) dùng `registerRepoInOwnProject`; thiếu thì giữ nguyên đường project mặc định (onboarding, `AddProjectFromFolderDialog`, desktop không đổi). Preload `repos.add/addRemote/clone/create` nhận `projectName?: string` (api-types) với cùng ý nghĩa.
- `useAddRepoCloneFlow` / `useCreateRepo` / `cloneRepoOntoDevServer` / `addRuntimeRepoRemote`: truyền `projectName` (tên repo) để đi đường project riêng khi có `devServerId`.
- Sau khi tạo: repo mới vào store (đã có `upsertAddedRepoWithProjectHostSetup`/`set repos`) và luồng hiện tại `onGitRepoReady → finishProjectAddWithDefaultCheckout` chọn và mở worktree mặc định của project mới (không đổi). Thông báo thành công nêu tên project.
- "Mở project đó" ở Add Project = `lib/open-registered-repo.ts` `openRegisteredRepo({repoId, projectId})` (nạp worktree rồi `activateAndRevealWorktree` worktree chính); ở Create New Project = `switchProject` của `WorkspaceContext` (qua prop `onOpenExistingProject`).
- `addRuntimeRepoRemote` và preload `repos.addRemote` KHÔNG đổi: chỉ dùng cho SSH/onboarding, không có caller nào của hộp thoại Add Project (thêm tham số sẽ là mã chết).
- Tách `AddRepoDialog.tsx` (vượt max-lines sau khi thêm dây nối): phần suy ra "host đã chọn nghĩa là duyệt/nhập đường dẫn kiểu nào" chuyển sang `components/sidebar/add-repo-host-routing.ts` (`deriveAddRepoHostRouting`), hành vi giữ nguyên.
- Nested import (`projectGroup.importNested`): backend đã tạo group + project cho TỪNG repo (`repos.ts:2570-2600` ghi chú), không đổi.

### 3.3 Create request chọn project (TASK-FE-E2E-024)

- `CreateRequestDialog.tsx:66-98,193-216` đã liệt kê `project.list` và bắt chọn (chặn tạo khi `projectId === ''`). Thêm tệp `components/request/CreateRequestProjectRepoNote.tsx` + hook `use-request-project-repos.ts`: với project đang chọn, gọi `repo.list {projectId}`, hiển thị "Request sẽ chạy trên repo: <tên> (<đường dẫn>) · <host>" kèm "(repo đầu tiên trong N repo của project)" khi N > 1, và cảnh báo khi project không có repo/host (đúng `REQUEST_NO_DEV_SERVER` về sau). Chỉ hiển thị, không đổi backend, không chặn.
- Tên project trong Select giữ `p.name`; thêm hậu tố số repo khi > 1 không cần thiết (bỏ để tránh thêm chuỗi).

### 3.4 Chống trùng repo (TASK-FE-E2E-025)

- Tệp mới `lib/registered-repo-lookup.ts`: `normalizeRepoPathKey` (bọc `normalizeRuntimePathForComparison` `shared/cross-platform-path.ts:22-25`: đổi `\`→`/`, gộp `//`, bỏ `/` cuối, chữ thường cho đường dẫn kiểu Windows), `findRegisteredRepo(repos, {devServerId, path})`, `RepoAlreadyRegisteredError` (`code: 'PROJECT_REPO_ALREADY_REGISTERED'`, `projectId`, `projectName`, `repoId?`), `parseRepoAlreadyRegistered(error)` (đọc mã `PROJECT_REPO_ALREADY_REGISTERED` ở `error.code` hoặc trong thông điệp; regex `/already (?:registered )?in project "?(.+?)"? \(([^()\s]+)\)/`). Thông điệp THẬT của backend (đọc ở `project-service/internal/usecase/repo_registration.go`) là `repo <path> on dev server <host> is already registered in project "<name>" (<id>)`, hoặc `... is already registered in another project` khi project riêng tư của người khác: khi đó không có tên/id nên khối chặn không có nút "Mở project đó".
- Hai lớp chặn: (a) tại hộp thoại Add Project, TRƯỚC khi gọi: tra `repos` của store (đã nạp từ mọi project) theo (devServerId, đường dẫn) + `project.list` để lấy tên project; trùng thì hiện khối inline `data-testid="add-project-duplicate"` (văn bản destructive nêu tên project) với nút "Open that project" (`data-testid="add-project-open-existing"`) mở project/repo đó và KHÔNG gọi RPC ghi nào; (b) trong `registerRepoInOwnProject` (kiểm `repo.list` mọi project, cho trường hợp store cũ) và ánh xạ mã lỗi backend (cuộc đua giữa hai tab); lỗi này hiện thành toast "Project already added" có nút "Open project" (không phải khối inline, vì hộp thoại đã chạy xong bước kiểm).
- `CreateProjectDialog.tsx`: bỏ cảnh báo amber "will NOT link"; khi trùng (từ store hoặc từ lỗi backend) hiện cùng khối chặn, nút Submit bị vô hiệu, kèm nút "Open that project" (prop mới `onOpenExistingProject?(projectId)`; `ProjectSwitcher` nối vào `switchProject`).
- `addRepoPath` (store): lỗi trùng không thành toast chung chung "Failed to add project" mà toast có tên project và hành động "Open project".

### 3.5 i18n, STYLEGUIDE, đa nền tảng, SSH/Electron

- Mọi chuỗi mới qua `translate()` với khoá `auto.components.request.AnswerView.*`, `auto.components.sidebar.AddRepoServerStartStep.projectName*`, `auto.components.sidebar.AddRepoDuplicateNotice.*`, `auto.components.project.CreateProjectDialog.duplicate*`, `auto.components.request.CreateRequestProjectRepoNote.*`, `auto.lib.ownProjectRegistration.*`; thêm đủ en/es/ja/ko/zh và một test phủ `i18n/project-own-and-dedupe-locale-coverage.test.ts` (mẫu `repo-host-binding-locale-coverage.test.ts`).
- STYLEGUIDE (`guides/STYLEGUIDE.md`; AGENTS.md ghi sai `docs/STYLEGUIDE.md`): lỗi chặn inline `text-destructive`, có hành động trực tiếp, không tooltip cho thông báo quan trọng; chỉ dùng token/primitive sẵn có; không màu/cỡ chữ/bóng mới.
- Tên tệp theo nghiệp vụ (không `helpers/utils/common`); không thêm `max-lines` disable.
- Đa nền tảng: so sánh đường dẫn dùng tiện ích `shared/cross-platform-path.ts`; không phím tắt mới.
- SSH/Electron: nhánh mới chỉ chạy khi có `devServerId` (host dev server của web hoặc desktop nối runtime). `repos.add` desktop (local/SSH) không đổi; tuỳ chọn `ownProject` vắng thì không đổi đường nào. `desktop/src/renderer/` là bản sao riêng, không thuộc phạm vi.

### 3.6 Phụ thuộc backend và thoái lui an toàn

Frontend tự kiểm trùng (store + `repo.list` mọi project) nên vẫn chặn được khi backend chưa có `PROJECT_REPO_ALREADY_REGISTERED`. Nếu backend cũ không gắn repo trong `project.create`, bước 3 của 3.2 bù bằng `repo.add`. Nếu backend mới trả `PROJECT_REPO_ALREADY_REGISTERED` thì thông điệp được ánh xạ như trên.

## 4. Phương án đã loại

- Tạo project riêng ở TẦNG backend `repo.add` (đổi ngữ nghĩa API): vi phạm "không đổi backend" cho phần Request/Project, và phá các caller khác (onboarding, script e2e).
- Biến mọi `addRepoPath` thành "project riêng" (không opt-in): đổi hành vi onboarding/desktop và `AddProjectFromFolderDialog`; rủi ro hồi quy cao hơn lợi ích.
- Sửa `CreateProjectDialog` thành luồng chính, bỏ luồng sidebar: người dùng không muốn đi qua "Project Workspace (Beta)" và hộp thoại đó không có Browse folder.
- Chuyển/gộp dữ liệu cũ vào project riêng: loại theo quyết định (không tự di chuyển).
- Tự xoá project bù khi `repo.add` thất bại: thao tác xoá không do UI tự quyết.
- Cho phép trùng nhưng cảnh báo (hành vi cũ): đúng nguyên nhân của lỗi 3.

## 5. Kiểm thử và lộ trình kiểm chứng

- Vitest: `solution-answer-document.test.ts`, `AnswerView.test.tsx`; `registered-repo-lookup.test.ts`, `own-project-registration.test.ts`; `web-preload-api.test.ts` (khối `host binding`, thêm `projectName` → `project.create` thay vì chọn project mặc định); `store/slices/repos-own-project.test.ts`; `useAddRepoServerPathFlow.test.ts`, `AddRepoServerStartStep` (ô tên, khối trùng); `CreateProjectDialog.test.tsx` (chặn trùng, ánh xạ lỗi); `CreateRequestProjectRepoNote.test.tsx`; test phủ i18n.
- Playwright mock `tests/stages/e2e/legacy/request-web` (`support/mock-request-ws.ts`): spec mới `add-project-own-project.web.e2e.ts` (payload `project.create` có `devServerId`, `repoPath`, tên sửa được; chặn trùng không gọi RPC ghi; mã lỗi backend thành khối chặn) và `request-answer-view.web.e2e.ts` (Answer hiện từ `answer_markdown`); chạy lại các spec Add Project hiện có đã bị ảnh hưởng (`add-project-host-binding.web.e2e.ts`).
- Live (agent AF viết, KHÔNG thuộc tài liệu này): e2e thật với repo tạm; Add Project 2 lần hai thư mục khác nhau → 2 project riêng; thêm lại cùng thư mục → bị chặn; Create request chọn project; Analysis `question` hiện Answer. Cần người điều phối dựng lại frontend (`BUILD_FRONTEND=1 ./deploy/dev/scripts/redeploy-local.sh frontend`) sau khi AE xong backend.
- Không thao tác lên dữ liệu thật của người dùng ("Demo (this Mac)", "codex", Request #1136).

## 6. Rủi ro, chưa xác minh

- Rủi ro: (a) tab frontend cũ vẫn dùng project mặc định (chấp nhận, dữ liệu cũ giữ nguyên); (b) `project.list` + N lần `repo.list` khi kiểm trùng (chấp nhận, giới hạn bởi số project của người gọi; chạy song song); (c) project riêng nhân lên số mục trong `ProjectSwitcher` (theo yêu cầu); (d) `RequestPageHeader.tsx:30-36` đặt tên project theo repo đầu tiên: khớp tên project mới vì `project.create` đặt tên repo = tên project, nhưng nếu người dùng sửa tên về sau thì nhãn lọc lệch (không sửa ở đợt này).
- Chưa xác minh: định dạng chính xác thông điệp `PROJECT_REPO_ALREADY_REGISTERED` của AE (dựa vào mô tả: `... already in project <name> (<id>)`); `Diagnosis/FindingsView` có cùng lỗi bí danh với schema `diagnosis`/`findings` hay không (chưa có mẫu); `switchProject` của WorkspaceContext có sẵn để dùng cho "Mở project đó" ở `CreateProjectDialog` (có, `ProjectSwitcher.tsx:31`).
