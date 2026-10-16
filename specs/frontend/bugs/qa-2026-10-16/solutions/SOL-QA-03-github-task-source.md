# SOL-QA-03: Nguồn task GitHub hoạt động và báo lỗi có hướng xử lý

**Bug:** [BUG-QA-03](../BUG-QA-03-github-task-source-unavailable.md)

## Bước 1 (gateway): chấp nhận selector `id:` và `repoId`

`channels_scm.go`: `listWorkItemsArgs` thêm `RepoID string json:"repoId"`. Hàm `normalizeRepoRef(repo, repoID)`: ưu tiên `repoID`; nếu rỗng thì cắt tiền tố `id:` (và hậu tố `::<path>`) của `repo`; nếu không có tiền tố thì giữ nguyên (tương thích cũ). Dùng cho `github.listWorkItems`, `github.countWorkItems` và mọi kênh `github.*` nhận "repo từ giao diện" (rà `github.issues`, `github.prForBranch`, `github.repoSlug`). Test Go bảng: `repo` ∈ {`<uuid>`, `id:<uuid>`, `id:<uuid>::/path`, rỗng} × `repoId` có/không.

Lưu ý lỗi vẫn rơi vào `errors.issues` (không phải lỗi kênh) nên giao diện thấy "không có dữ liệu" thay vì lỗi; sau sửa, `WORKTREE_REPO_NOT_FOUND` chỉ còn khi repo thật sự không tồn tại.

## Bước 2 (gateway + scm): phân loại lỗi thông tin xác thực

- Thêm `auth_required` vào `ClassifiedError.type` (`frontend/src/shared/types.ts`) và vào view của gateway; ánh xạ `SCM_CREDENTIAL_NOT_FOUND`/401/403 thiếu scope → `auth_required` kèm thông điệp "GitHub is not connected for this account."; 404 → `not_found`; 429 → `rate_limited`.
- Giao diện: `auth_required` hiện nút **Connect GitHub** (gọi `github.startAuthLogin`, đã có) hoặc ô nhập token cá nhân (PAT) lưu qua credential-broker (hỗ trợ GitHub Enterprise host riêng bằng cấu hình `githubApiBaseUrl`). Không đọc token từ biến môi trường của trình duyệt; tuyệt đối không ghi token vào log.
- Triển khai dev: `deploy/dev/.env.example` ghi rõ `SSO_GITHUB_CLIENT_ID/SECRET` (đã có cho đăng nhập) dùng cho OAuth; thêm hướng dẫn kết nối.

## Bước 3 (frontend): trạng thái nguồn đúng

- `getRepoBackedProviderAvailability`: nguồn GitHub **không** phụ thuộc `gh`. Điều kiện khả dụng = host có kết nối dev server (từ `devServer.list`, không từ bảng SSH) **và** thông tin xác thực GitHub tồn tại. `gh` chỉ còn là gợi ý cho các thao tác chạy trên host (Open PR…).
- `getHostLabel`: tra tên từ `devServer.list` (nhãn `dev-local-mac`), không in UUID; bỏ trùng ("disconnected" và "source tool unavailable" của cùng host gộp một dòng).
- Dải cảnh báo nêu nguyên nhân và hành động: "GitHub is not connected — Connect" / "Host dev-local-mac is offline — Reconnect".

## Bước 4 (agent/gateway): preflight có hạn thời gian

- `onboarding.getPreflightStatus`: hạn 5 giây, quá hạn trả `status: "timeout"` thay vì treo; giao diện coi là "chưa biết", không chặn nguồn.
- `preflight.check`: bỏ `skip` cố định của `github-cli-auth`; hoặc báo rõ "không dùng `gh`".

## Kiểm thử cùng bản sửa

| Tầng | Test |
|---|---|
| Gateway Go | bảng chuẩn hoá `repo`; `auth_required` cho `SCM_CREDENTIAL_NOT_FOUND` |
| FE unit | `task-source-provider-availability.test.ts` (không phụ thuộc `gh`), `task-source-context-summary.test.ts` (tên host, gộp trùng) |
| Acceptance | QA-GH-01 (`repo:"id:<repoId>"` giải ra đúng owner/repo), QA-GH-02 (không có credential → `auth_required`, không `not_found`/`unknown`), QA-GH-03 (`repo` thô vẫn chạy) |
| UI live | QA-UI-GH-01: tab Tasks → GitHub không hiện "unavailable", hiện hướng "Connect GitHub" khi chưa có credential, nhãn host là tên |

## Rủi ro
- Token cá nhân là dữ liệu nhạy cảm: chỉ lưu qua credential-broker, mã hoá, không trả lại giao diện.
- Giới hạn tốc độ GitHub: giữ `rate_limited` và `noCache` theo hợp đồng cũ.
