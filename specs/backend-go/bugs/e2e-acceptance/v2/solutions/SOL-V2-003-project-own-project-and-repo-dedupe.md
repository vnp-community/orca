# SOL-V2-003: "Add Project" tạo project riêng đúng mọi trường hợp; chặn đăng ký trùng repo; xác nhận hình dạng tài liệu `answer`

**Giải quyết:** ba lỗi từ lượt test Request #1136: (1) UI `AnswerView` không đọc `answer_markdown`; (2) "Add Project" phải tạo được PROJECT RIÊNG chứa repo, gắn dev server đã chọn; (3) cùng một repo (cùng dev server, cùng đường dẫn) bị đăng ký nhiều lần ở nhiều project.
**Service:** project-service (chính), api-gateway (wscompat `project.create`, REST), request-service (chỉ đọc/xác nhận), proto `project.v1` (chỉ cộng). Frontend (`frontend/**`) do agent khác làm song song, KHÔNG nằm trong tài liệu này.
**Phối hợp với:** [SOL-V2-002](./SOL-V2-002-repo-host-binding-and-no-silent-local.md) (repo luôn gắn host; đợt này dựa trên `RepoHostPolicy`, `AddRepo`, `CreateProject`, `RebindRepoDevServer` đã có).
**Tasks:** TASK-V2-036 đến TASK-V2-040 (xem [README](../tasks/README.md))

Mọi trích dẫn `file:dòng` là mã THẬT đã đọc ngày 2026-10-11 trên nhánh `feat/e2e-acceptance-fixes`. gitnexus không có trong môi trường này; phạm vi ảnh hưởng dùng `codegraph` (tên chính xác), ghi ở mục 8 và trong từng task.

## Quyết định đã chọn

1. **`project.create {name, devServerId, repoPath, visibility}` là thao tác NGUYÊN TỬ**: project + hội viên chủ sở hữu (người tạo) + repo đầu tiên được ghi trong MỘT giao dịch. Không còn "tạo project rồi cố gắng gắn repo, lỗi thì nuốt". Nếu repo không ghi được (kể cả vì trùng), project KHÔNG được tạo. Phản hồi trả thêm repo vừa tạo (`repo` + `repoId`) để frontend không phải gọi `repo.list` đoán lại.
2. **Khoá "một vị trí repo" = (tenant, dev server, đường dẫn đã chuẩn hoá).** Chuẩn hoá tối thiểu và không phá nghĩa: bỏ khoảng trắng hai đầu, bỏ dấu phân cách cuối, gộp dấu phân cách lặp; KHÔNG đổi hoa/thường, KHÔNG giải `..`/symlink. Kiểu đường dẫn theo dạng của chính chuỗi (`C:\...`, `\\server\...` là Windows: `/` và `\` tương đương; còn lại POSIX; URL có `://` chỉ bỏ `/` cuối), không theo hệ điều hành của project-service.
3. **Chống trùng hai lớp**: (a) kiểm trong ứng dụng, đọc cột thật (`dev_server_id`, `url`) nên thấy cả repo cũ chưa có khoá và bản ghi khác định dạng; (b) cột mới `repos.registration_key` + chỉ mục UNIQUE trên cột đó làm hàng rào chống đua. `registration_key` chỉ được ghi cho bản ghi MỚI (NULL ở dữ liệu cũ; cả Postgres lẫn MySQL cho nhiều NULL trong chỉ mục unique), nên migration KHÔNG đụng dữ liệu trùng sẵn có (ví dụ repo `/Users/datbeohbbh/Desktop/stuff/agentic/codex` ở hai project) và không cần migrate/xoá chúng.
4. **Mã lỗi `PROJECT_REPO_ALREADY_REGISTERED`, `KindAlreadyExists` (gRPC `AlreadyExists`)**, đúng quy ước repo cho "tài nguyên đã được đăng ký/chiếm" (`PROJECT_FOLDER_WORKSPACE_PATH_TAKEN` cùng service, `TENANT_EMAIL_DOMAIN_TAKEN`, `AIPROVIDER_LABEL_TAKEN`); `FailedPrecondition` dành cho "trạng thái hệ thống chưa cho phép" (`PROJECT_HAS_ACTIVE_WORKFLOWS`). Thông điệp CỐ ĐỊNH để frontend parse:
   `PROJECT_REPO_ALREADY_REGISTERED: repo <url> on dev server <devServerId> is already registered in project "<tên>" (<projectId>)`
   (`<url>` là chuỗi người gọi gửi; repo không gắn dev server in `local`). Người gọi KHÔNG xem được project đó (không là hội viên, không là admin toàn cục, visibility `private`) nhận cùng mã với thông điệp rút gọn `... is already registered in another project` để không lộ tên/id project riêng tư.
5. **Phạm vi áp dụng**: `AddRepo`, `CreateProject` (có `repo_path`), `SetupExistingFolder` (tạo project + repo từ host setup), `ImportNested` (mỗi ứng viên, và trùng trong chính lô chọn), `RebindRepoDevServer` (gắn lại sang dev server đã có repo cùng đường dẫn), `UpdateRepo` khi đổi `url`. `AssignRepoToProject` (chuyển repo giữa project cùng tenant) không đổi vị trí nên không cần kiểm. `repo.clone`/`repo.create` của git-gateway chỉ chạy git, không đăng ký repo (đăng ký xảy ra ở `repo.add`), nên không có đường tạo repo thứ hai cần chặn.
6. **Dữ liệu cũ giữ nguyên**: repo trùng sẵn có không bị migrate, không bị xoá, mọi thao tác khác trên chúng (đổi tên, hook, worktree, rebind về chính vị trí cũ) vẫn chạy. Chỉ thao tác TẠO vị trí mới (thêm, đổi url, đổi dev server sang vị trí mới) bị kiểm. Công cụ `report-duplicate-repos` chỉ liệt kê (dry-run, không ghi).
7. **`project.create` không bắt buộc project có tên duy nhất**: `projects` không có ràng buộc tên (`migrations/postgres/0001`, `0002`); hai project cùng tên là hợp lệ và có id khác nhau. Chỉ vị trí repo phải duy nhất.
8. **Hình dạng tài liệu `answer` qua `solution.list`** đã xác minh (mục 3.5): gateway KHÔNG đổi tên/mất trường; `options` là JSON thô của request-service, khoá snake_case. Không sửa backend; thêm test chốt hợp đồng (TASK-V2-038).

## 1. Nguyên nhân đã xác minh

| # | Triệu chứng | Bằng chứng trong mã |
|---|---|---|
| C1 | `project.create` có thể trả thành công nhưng project KHÔNG có repo (bị bỏ im lặng) | `create_project.go:143-148`: `_, _ = uc.repos.AddRepo(...)` bỏ cả lỗi lẫn kết quả ("best-effort attach"); project đã được `uc.repo.Create` ở dòng 138 trước đó, hai thao tác không chung giao dịch |
| C2 | Người tạo có thể mất quyền vào project vừa tạo | `channels_tenant_project.go:483-488`: hội viên owner thêm ở lần gọi `AddMember` thứ hai của gateway, sau `CreateProject`; lỗi ở giữa để lại project không có hội viên (chỉ owner thấy được project trong `List`) |
| C3 | Frontend không biết id repo vừa tạo | `CreateProjectResponse` chỉ có `project` (`project.proto:216-218`); `toProjectView` (`channels_tenant_project.go:382`) không có repo |
| C4 | Cùng repo đăng ký nhiều lần | `add_repo.go:85-93`, `create_project.go:143-148`, `import_nested.go:60`, `rebind_repo_dev_server.go:99`, `setup_existing_folder.go:101-108`, `update_repo.go` (đổi `url`): không nơi nào kiểm vị trí đã có; bảng `repos` chỉ có `PRIMARY KEY (id)` và chỉ mục không-unique `idx_repos_project`, `idx_repos_dev_server` (`migrations/postgres/0003_repos.up.sql`, `0017_repo_dev_server.up.sql`); đối chiếu `folder_workspaces` đã có `UNIQUE (tenant_id, dev_server_id, path)` (`0006_folder_workspaces.up.sql:20`) |
| C5 | `project.importNested` hỏng ở Postgres | `postgres/project_group_repository.go:181`: `Scan` 10 đích cho `projectColumns` 14 cột (mã MySQL đã tránh lỗi này, ghi ở `mysql/project_group_repository.go` doc của `ImportNested`); phải sửa vì đợt này đụng đúng hàm này |
| C6 | Thứ tự kiểm lộn xộn trong `CreateProject` | tên rỗng/visibility sai chỉ bị bắt SAU khi đã gọi infra-fleet và relay `fs.checkPath` (`create_project.go:116-131`); lỗi đầu vào rẻ phải đứng trước lời gọi mạng |

Ngoài phạm vi (đã đọc, không phải nguyên nhân): `repo_path` không phải kho git. `create_project.go:106-112` chỉ đòi tồn tại và là thư mục (`fs.checkPath`); project-service không có client git-gateway và Orca có chế độ thư mục không-git (`folder_workspace`), nên KHÔNG chặn ở đây; thao tác git sau đó trả lỗi git có mã. Ghi nhận, không đổi.

## 2. Thiết kế

### 2.1 Miền (`internal/domain/repo_location.go`, mới)

- `NormalizeRepoLocation(url string) string`: quy tắc ở quyết định 2; bảng ca trong test (`/a/b/`, `/a//b`, `C:\a\b\`, `C:/a/b`, `\\srv\share\x\`, `https://h/x/y/`, `/` giữ nguyên, hoa/thường giữ nguyên).
- `RepoRegistrationKey(tenantID, devServerID, url string) string`: SHA-256 hex của `tenant \x00 devServer(lower) \x00 normalized`; luôn 64 ký tự nên không vướng giới hạn độ dài khoá của MySQL. Tenant nằm trong băm để hai tenant không bao giờ đụng nhau dù dev server id giống.
- `Repo.RegistrationKey` (đọc/ghi qua repository), `ErrRepoAlreadyRegistered` (sentinel adapter trả khi chạm chỉ mục unique), `RepoConflict{RepoID, ProjectID, ProjectName, ProjectVisibility, URL, DevServerID}`.

### 2.2 Cổng lưu trữ

- `RepoRepository.FindRepoAtLocation(ctx, tenantID, devServerID, normalizedURL, excludeRepoID) (RepoConflict, bool, error)`: `SELECT ... FROM repos JOIN projects ON ... WHERE projects.tenant_id = ? AND repos.dev_server_id = ?` (hoặc `IS NULL` cho local), sau đó so `NormalizeRepoLocation(row.url)` trong Go. So trong Go vì chuẩn hoá Windows/POSIX không biểu diễn được bằng SQL chung hai dialect; số repo trên một dev server nhỏ và đã có `idx_repos_dev_server`. Luôn lọc tenant qua `projects.tenant_id` (bảng `repos` không có cột tenant; MySQL không có RLS, `mysql/repo_repository.go` doc `ListReposForTenant`).
- `RepoRepository.UpdateDevServerID(ctx, repoID, devServerID, registrationKey)`: nhận khoá mới cùng lúc đổi host. `Update` ghi `registration_key` từ `repo.RegistrationKey` (usecase tính lại khi đổi `url`).
- `ProjectRepository.CreateWithOwner(ctx, project, owner, *repo)`: một giao dịch ghi project, hội viên owner và (nếu có) repo đầu tiên với khoá; chạm chỉ mục unique thì quay lui toàn bộ và trả `ErrRepoAlreadyRegistered`.
- `ProjectGroupRepository.ImportNested`: chữ ký giữ nguyên; adapter tự tính khoá từ `tenantID, devServerID, c.Path` (đã có đủ), chạm chỉ mục thì `ErrRepoAlreadyRegistered`.

### 2.3 Chống đua và dữ liệu trùng sẵn có

Quy trình mỗi lần TẠO vị trí mới: (1) usecase gọi `FindRepoAtLocation`, có kết quả thì từ chối ngay với thông điệp đầy đủ (đường chính, không đua); (2) không có thì ghi với `registration_key`; (3) hai request đồng thời cùng qua bước 1: đúng MỘT `INSERT` thắng, bên kia nhận `ErrRepoAlreadyRegistered` từ chỉ mục unique (Postgres `23505` tên ràng buộc `uq_repos_registration_key`; MySQL lỗi `1062` tên khoá đó), usecase tra lại `FindRepoAtLocation` để dựng thông điệp có tên/id project của bên thắng (nếu bên thắng vừa bị xoá thì trả `PROJECT_REPO_REGISTRATION_CONFLICT`, `KindAborted`, thử lại được).

Dữ liệu cũ: các hàng cũ có `registration_key = NULL`, không vào chỉ mục (NULL không xung đột); bước (1) vẫn thấy chúng vì đọc cột thật, nên repo `codex` ở hai project vẫn chặn được lần đăng ký thứ ba. Khoảng trống duy nhất: một repo CŨ (không khoá) bị đăng ký lại bởi hai request ĐỒNG THỜI trong khi chưa có bản nào có khoá, nhưng trường hợp đó hai request mới đều có khoá và chỉ mục chặn. Hai thao tác đổi vị trí của hai repo cũ khác nhau về cùng một vị trí cũng đều mang khoá nên bị chặn. Tức là mọi đua giữa các bản ghi MỚI/được ghi lại đều bị chỉ mục chặn; bản ghi cũ tĩnh nên bước (1) đủ.

MySQL: `INSERT ... SELECT` của `AddRepo` đọc cùng bảng nên khi nhiều client cùng khoá thì InnoDB báo deadlock (1213, test đua thật đã gặp); nạn nhân bị lùi nguyên câu lệnh nên `AddRepo` thử lại tối đa 5 lần, lần sau thấy bên thắng và trả `ErrRepoAlreadyRegistered`. Postgres chờ giao dịch bên thắng rồi báo `23505`, không cần thử lại.

Thao tác KHÔNG đổi vị trí không kiểm và không ghi khoá: `RebindRepoDevServer` về cùng dev server trả nguyên repo; `UpdateRepo` với `url` chuẩn hoá giống cũ chỉ đổi tên/hook; `AssignRepoToProject` không đổi.

### 2.4 `CreateProject` (usecase)

Thứ tự mới: tenant/user → kiểm tên, visibility (rẻ) → chính sách host (`requireHost`) → dev server tồn tại + tới được → `repo_path` bắt buộc khi có host (giữ `PROJECT_REPO_PATH_REQUIRED`) → vị trí repo chưa đăng ký → relay `fs.checkPath` → `CreateWithOwner`. Lỗi `AddRepo` không còn bị nuốt vì không còn lời gọi `AddRepo` riêng. `repo_path` có hay không có `/` cuối đều tạo cùng vị trí; `repo` trả về dùng chuỗi người gọi gửi (không đổi dữ liệu người dùng). Project không gắn dev server và không `repo_path` vẫn tạo được (hành vi cũ).

### 2.5 Gateway

- `project.create` trả `{...projectView, repo: repoView|null, repoId: string}` (cộng trường, camelCase như `repo.add`); vẫn gọi `AddMember` sau đó (idempotent, upsert) để triển khai lệch phiên bản không vỡ.
- REST `POST /v1/projects` nhận thêm `dev_server_id`, `repo_path` (trước đây bị bỏ, `project_routes.go:85-91`), trả cùng cấu trúc proto.
- Lỗi đi nguyên vẹn dạng `rpc error: code = AlreadyExists desc = PROJECT_REPO_ALREADY_REGISTERED: ...`.

### 2.6 Proto (chỉ cộng)

`CreateProjectResponse.repo = 2` (message `Repo` đã có). Không đổi/xoá trường nào; số trường mới, `buf breaking` xanh, `buf lint` giữ 307 cảnh báo, header sinh mã về `v6.33.4`.

### 2.7 Migration `0032_repo_registration_key` (postgres, mysql)

```
ALTER TABLE project.repos ADD COLUMN registration_key VARCHAR(64);
CREATE UNIQUE INDEX uq_repos_registration_key ON project.repos (registration_key);
```
MySQL: cùng nội dung với `repos`, `CHAR(64) NULL`. Không backfill (cố ý, quyết định 3), không khoá dài (cột mới toàn NULL). Down: bỏ chỉ mục và cột. Kiểm: up/down/up trên Postgres và MySQL thật, và dữ liệu có bản trùng sẵn có vẫn nguyên sau up.

### 2.8 Công cụ báo cáo (TASK-V2-040)

`deploy/dev/scripts/report-duplicate-repos.sh` + `report_duplicate_repos.py`: đọc `project.list`/`repo.list` qua kênh WS giống `rebind_unbound_repos.py`, gom theo (tenant của phiên, `devServerId`, đường dẫn chuẩn hoá — chuẩn hoá y hệt Go, có bảng ca chung), in nhóm có từ hai repo trở lên (project, repo, ngày). Chỉ liệt kê; không có cờ ghi/xoá.

## 3. Hình dạng tài liệu `answer` qua kênh gateway (TASK-V2-038, xác minh)

3.1 request-service lưu `solutions.options_json` đã chuẩn hoá bởi `ValidateByKind` (`usecase/analysis_document_validation.go:41-49`) → `AnswerDocument.Marshal()` (`domain/answer_document.go`).
3.2 gRPC `ListSolutions` đặt nguyên chuỗi vào `Solution.options_json` (`adapter/grpc/server_solution.go:132`).
3.3 Gateway `solution.list` (`wscompat/channels_solution.go:17-52`) → `SolutionViewOf` (`channels_request_views.go:112-123`): `options` là JSON thô (`json.RawMessage`), KHÔNG đổi khoá; chỉ khi không phải JSON hợp lệ mới thành `null`.
3.4 Kết quả: mỗi phần tử `solutions[]` có `kind: "answer"`, `status`, `options` =
```json
{"schema_version":1,"kind":"answer","answer_markdown":"...","confidence":0.0,
 "citations":[{"ref":"path:line","excerpt":""}],"limitations":["..."],
 "suggested_follow_up":{"type":"change_request|task","title":"..."}}
```
Khoá là snake_case ngay trong `options` (khác phần bọc `SolutionView` là camelCase). Không có trường `assumptions`; điều tương đương là `limitations`. `citations[].excerpt`, `limitations`, `suggested_follow_up` có thể vắng (`omitempty`).
3.5 `chosenOption`/`chosenOptionId` không áp dụng cho `kind: "answer"` (chỉ kind `solution` có mảng `options`; `chosenOptionId` tự rỗng khi không có `options[]`).
Frontend phải đọc `solution.options.answer_markdown`, không phải `solution.answer_markdown`/`solution.options.answerMarkdown`.

## 4. Phương án đã cân nhắc và loại

| Phương án | Lý do loại |
|---|---|
| UNIQUE INDEX toàn cục `(dev_server_id, url)` trên cột sẵn có | Dữ liệu thật đã có bản trùng (`codex` ở hai project): `CREATE UNIQUE INDEX` thất bại, hoặc buộc xoá/migrate dữ liệu người dùng; ngoài ra `url` chưa chuẩn hoá và bảng không có cột tenant |
| Chỉ chặn trong ứng dụng (không chỉ mục) | Hai request đồng thời cùng qua kiểm và cùng ghi; không đúng yêu cầu chống đua |
| Khoá tư vấn (`pg_advisory_xact_lock`) / `SELECT ... FOR UPDATE` theo băm | Postgres-only; MySQL cần `GET_LOCK` gắn session (khó với pool) và gap lock; chỉ mục unique là cơ chế chung của cả hai dialect, bền qua mọi đường ghi (kể cả công cụ khác ghi trực tiếp) |
| Chỉ mục unique một phần (`WHERE ... IS NOT NULL`) | MySQL không hỗ trợ chỉ mục một phần; unique thường trên cột nullable đã cho nhiều NULL ở cả hai dialect, nên không cần |
| Chuẩn hoá bằng `path.Clean`/`filepath.Clean` | Giải `..` theo chữ nghĩa sai khi có symlink; `filepath` phụ thuộc hệ điều hành của project-service (Linux trong container) nên hiểu sai `C:\x`; đúng yêu cầu là so theo kiểu của dev server |
| Backfill `registration_key` cho dữ liệu cũ không trùng | Không cần (bước 1 đọc cột thật), và backfill đòi chuẩn hoá hàng loạt trong migration SQL; để NULL là an toàn nhất, rollback tầm thường |
| Giữ `AddRepo` best-effort + dọn project mồ côi | Cửa sổ hỏng giữa hai ghi; giao dịch đơn giản và đúng hơn |
| Trả `FailedPrecondition` | Không đúng ngữ nghĩa "đã tồn tại"; cùng service đã dùng `AlreadyExists` cho đường dẫn bị chiếm |
| Chặn `repo_path` không phải git ngay ở project-service | Phá chế độ thư mục không-git; project-service không có client git; xem mục 1 "ngoài phạm vi" |

## 5. Rủi ro

- Tên project/id trong thông điệp có thể lộ project riêng tư của người khác cùng tenant: đã rút gọn khi người gọi không xem được project (quyết định 4); kiểm bằng test.
- Tra `FindRepoAtLocation` quét mọi repo của một dev server trong tenant: nhỏ (hàng chục-hàng trăm), đã có chỉ mục `dev_server_id`; đo ở TASK-V2-039.
- `e2e orca-flows` dùng `ORCA_E2E_REPO_PATH` cố định cho nhiều lượt: nếu một lượt trước chết giữa chừng để lại project cùng đường dẫn, lượt sau bị chặn đúng thiết kế (`run_acceptance.py` DEV-04 và `live-project-fixture.ts` dùng repo tạm duy nhất khi biến không đặt; các suite request/task/ui-live không tái dùng đường dẫn). Ghi ở TASK-V2-037.
- Tab/triển khai lệch phiên bản: gateway cũ + project-service mới vẫn chạy (gateway cũ chỉ bỏ qua trường `repo` mới và vẫn `AddMember`); gateway mới + project-service cũ: `repo` rỗng, `repoId` rỗng (frontend phải chịu được).
- Phía MySQL không có RLS: mọi truy vấn mới tự lọc `projects.tenant_id`; test cô lập tenant ở cả hai dialect.
- Có sẵn, không do đợt này: `ListReposForTenant` ở MySQL không lọc tenant (`mysql/repo_repository.go:66-71` tự ghi chú); Postgres dựa RLS. Ghi nhận cho người rà soát, không sửa trong SOL này.

## 6. Lộ trình triển khai và kiểm chứng

### 6.1 Thứ tự (mỗi bước build/vet/test xanh)

1. Tài liệu (SOL + task + README).
2. Miền: chuẩn hoá + khoá (TASK-V2-037), test bảng.
3. Migration `0032` hai dialect + adapter hai dialect + test tích hợp hai dialect (TASK-V2-039).
4. Usecase `AddRepo`/`Rebind`/`UpdateRepo`/`ImportNested`/`SetupExistingFolder` (TASK-V2-037), `CreateProject` nguyên tử (TASK-V2-036).
5. Proto + gateway + REST (TASK-V2-036), test gateway `answer` (TASK-V2-038).
6. Công cụ báo cáo (TASK-V2-040).

### 6.2 Redeploy (người điều phối; KHÔNG chạy trong đợt viết mã)

Thứ tự: chạy migration `0032` của project-service trước (cộng cột, không khoá dài), rồi `project-service`, rồi `api-gateway` (gateway mới dùng trường proto mới; nếu gateway lên trước thì `repo` chỉ rỗng). Không cần dựng lại `request-service`, `git-gateway-service`, `infra-fleet-service`.

### 6.3 Kiểm chứng (không động dữ liệu thật của người dùng)

`go build ./... && go vet ./... && go test ./...` ở project-service và api-gateway; `go test -tags integration ./internal/adapter/...` project-service (Postgres + MySQL, test đua nhiều goroutine, cô lập tenant, khác dev server không trùng, dữ liệu cũ trùng vẫn sống); `buf lint`, `buf breaking`, mã sinh khớp. Chạy `report-duplicate-repos.sh` (chỉ đọc) trên stack thật do người điều phối.

### 6.4 Rollback

Migration down bỏ chỉ mục + cột (an toàn: cột chỉ chứa băm). Nhị phân cũ bỏ qua cột mới. Không có cờ cấu hình: quy tắc trùng là hành vi cố ý; lùi bằng triển khai lại bản trước.

## 7. Định nghĩa xong

- Mã lỗi/định dạng thông điệp đúng như quyết định 4, có test chuỗi chính xác.
- Test đua (nhiều goroutine, mỗi dialect) đúng một thành công.
- Dữ liệu trùng sẵn có sống sót qua migration và mọi thao tác không tạo vị trí mới.
- `go test ./...` xanh ở project-service và api-gateway (trừ lỗi có sẵn đã ghi), tích hợp hai dialect xanh, `buf` xanh.

## 8. Phạm vi ảnh hưởng (codegraph, tên chính xác)

Ghi chi tiết ở từng task (số người gọi thật từ `codegraph callers`). Tóm tắt: `NewAddRepo`, `NewCreateProject`, `NewImportNested`, `NewRebindRepoDevServer`, `NewUpdateRepo`, `NewSetupExistingFolder` chỉ được dựng ở `project-service/cmd/server/main.go` (cộng test); `RepoRepository.UpdateDevServerID` có một nơi gọi (`rebind_repo_dev_server.go`); `ProjectRepository.Create` giữ nguyên cho `SetupExistingFolder` dùng.

## 9. Chưa xác minh (nói thẳng)

- `project.importNested` không thêm hội viên owner cho người nhập (`ImportNested` ở hai adapter chỉ ghi project/repo/group, `channels_tenant_project.go` gateway cũng không gọi `AddMember` sau đó): project nhập về có thể không hiện trong `project.list` của người nhập. Có sẵn từ trước, KHÔNG sửa trong đợt này (ngoài phạm vi); nên mở task riêng.
- Postgres `ImportNested` trước đây `Scan` 10 đích cho 14 cột; đợt này đã sửa (dùng `scanProject`) và có test tích hợp đầu tiên cho hàm này. Kết luận "hỏng trước đó" suy ra từ đọc mã, chưa chạy lại mã cũ.

- Chưa chạy trên stack thật (cấm redeploy trong đợt này); phản hồi `project.create` mới chưa kiểm với frontend thật.
- Trường hợp dev server Windows thật (`C:\...`) chỉ kiểm bằng test đơn vị chuẩn hoá, chưa có agent Windows thật.
- Hành vi của frontend khi nhận thông điệp rút gọn (người gọi không xem được project kia) do agent frontend xử lý theo mã.
