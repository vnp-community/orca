# BUG-BE-PROJECT-001: Tạo `OrcaProject` + `repo.add` không liên kết gì với `Project`/`Repo` cũ (sidebar chính) — 2 hệ dữ liệu song song, người dùng dễ tưởng nhầm mất dữ liệu / tạo trùng

**Phát hiện:** 2026-09-01, từ câu hỏi "khi thêm OrcaProject và bổ sung repo cho OrcaProject có tự
động thêm Project để thực hiện kết nối không?" — điều tra trực tiếp qua CodeGraph xác nhận: không,
và hệ quả UX của việc "không" đó chưa được xử lý ở đâu cả.

> Bối cảnh khái niệm: xem [Project vs OrcaProject](../authorization/asset-hierarchy-and-permission-model.md#project-vs-orcaproject--vì-sao-tồn-tại-cả-2-và-có-nên-gộp-không)
> trong `docs/guides/authorization/`. Bug này không phải do 2 model cùng tồn tại (đó là quyết định
> kiến trúc hợp lý) — mà do **cầu nối giữa 2 model dừng lại ở tầng RPC, không tới được UI/UX**.

## Triệu chứng thật

1. User đã dùng sidebar chính, có sẵn Repo `my-service` (path `/home/dev/my-service`,
   `executionHostId: devServer:dev-01`) trong `Project` "Backend" của họ.
2. User mở nút "Project Switcher" (thanh sidebar, render bởi `App.tsx` → `ProjectSwitcher.tsx`,
   xác nhận đây là UI thật đang chạy, không phải code chết) → "Create New Project" →
   `CreateProjectDialog.tsx`.
3. User chọn dev server `dev-01`, nhập lại đúng path `/home/dev/my-service`, tên "My Service", bấm
   Create.
4. Chuỗi RPC thật chạy: `project.create` (tạo 1 `OrcaProject` mới, id khác hoàn toàn) →
   `project.rebindDevServer` → `repo.add({projectId: <orcaProjectId>, url: '/home/dev/my-service'})`
   (tạo 1 **`Repo` Go-native mới**, FK `projectId` trỏ vào `OrcaProject` vừa tạo — proto
   `orca.project.v1.Repo`, khác hoàn toàn `Repo` cũ ở bước 1).
5. Kết quả: **2 bản ghi hoàn toàn độc lập** cùng trỏ tới cùng 1 thư mục vật lý trên cùng 1 dev
   server — không có FK, không có tag, không gì liên hệ chúng với nhau. Sidebar chính (Project
   "Backend") không thấy gì thay đổi. `OrcaProject` "My Service" mới không "biết" gì về Project
   "Backend" hay lịch sử worktree/terminal đã có trên repo đó.

## Root cause

`CreateProjectDialog.tsx` (`frontend/src/renderer/src/components/project/CreateProjectDialog.tsx`)
luôn đi theo nhánh "tạo repo Go-native mới" (`repo.add`), không bao giờ kiểm tra hoặc gợi ý nhánh
"link 1 Project đã có sẵn" (`orcaProjects.linkSourceProject`, xem
[BUG-BE-PROJECT-002](./bug-be-project-002-orcaprojectsourceproject-no-ui.md)) — vì bản thân nhánh
đó **không có UI nào để gọi tới** (dialog chỉ có field `repoPath` dạng text, không có lựa chọn nào
để chọn 1 Project có sẵn từ sidebar). Đây là hệ quả trực tiếp của Bug 002, không phải lỗi độc lập —
nhưng tách thành bug riêng vì **impact khác hẳn**: Bug 002 là "tính năng chia sẻ không dùng được",
Bug 001 là "người dùng bình thường (không cần biết gì về sharing) cũng bị lẫn lộn/trùng lặp dữ
liệu ngay ở luồng tạo project cơ bản nhất".

Không có bất kỳ cơ chế phát hiện trùng lặp nào ở tầng backend (`repo.add`
(`backend-go/services/api-gateway/internal/adapter/httpgateway/project_routes.go` →
`project-service`) không kiểm tra `url` đã tồn tại ở Repo/Project cũ nào của cùng user/host hay
chưa trước khi insert).

## Mức độ ảnh hưởng

- **Nhầm lẫn UX**: người dùng không có cách nào biết 2 luồng ("Add Repo" ở sidebar chính vs "New
  Project" ở Workspace Beta) tạo ra 2 thực thể độc lập trên cùng 1 dữ liệu vật lý — dễ tưởng là
  bug ("tôi add rồi sao không thấy trong Project mới tạo?" hoặc ngược lại).
- **Giá trị cốt lõi của OrcaProject bị chặn cho use case phổ biến nhất**: mục tiêu chính của
  `OrcaProject` là chia sẻ RBAC cho project đang làm việc — nhưng project "đang làm việc" thường
  đã tồn tại từ trước trong sidebar chính. Không có đường tạo OrcaProject TỪ 1 Project có sẵn (chỉ
  có đường tạo Repo Go-native hoàn toàn mới), nên use case thật (chia sẻ cái đang có) khó dùng hơn
  use case ít gặp hơn (tạo project mới từ đầu chỉ để chia sẻ).
- Không phải data-loss thật (không ghi đè, không xoá gì) — nhưng là data **phân mảnh không kiểm
  soát được**, tích luỹ theo thời gian nếu không sửa.

## Đề xuất fix

**Bước 1 (chặn trước, low-effort, làm ngay được):** Trong `CreateProjectDialog.tsx`, khi user
nhập `repoPath` + chọn `devServerId`, so khớp với danh sách Repo hiện có trong store (`repos.ts`
slice, đã có `state.repos` với `path` + `executionHostId`). Nếu trùng, hiện cảnh báo inline:
"Repo này đã có trong sidebar của bạn (Project: <tên>). Tạo Project mới ở đây sẽ KHÔNG liên kết
với dữ liệu đó — chúng sẽ độc lập." — không chặn hành động, chỉ minh bạch hoá hệ quả.

**Bước 2 (giải pháp đúng, phụ thuộc Bug 002 được fix trước):** Thêm 1 lựa chọn thứ 2 trong
`CreateProjectDialog` (hoặc tách hẳn thành 2 tab "New Repo" / "Link Existing Project"):
- Tab "Link Existing Project": dropdown liệt kê `Project[]` của chính user (từ `repos.ts` store,
  đã có sẵn ở client, không cần RPC mới) → khi chọn, gọi `project.create` (không kèm
  devServerId/repoPath) rồi `orcaProjects.linkSourceProject({orcaProjectId, projectId})` — không
  gọi `repo.add` ở nhánh này.
- Tab "New Repo" (hành vi hiện tại): giữ nguyên `repo.add`, dùng cho trường hợp thật sự muốn tạo
  Repo Go-native mới (ví dụ: repo chưa từng add vào sidebar chính).

**Bước 3 (dài hạn, không bắt buộc để fix bug này):** Cân nhắc hợp nhất tầng hiển thị worktree/
terminal cho cả 2 loại Repo (cũ và Go-native) — hiện `Worktree`/`Terminal` (mục 1.4/1.6,
`docs/guides/authorization/`) vẫn mô tả gắn với `Repo` cũ; cần xác minh riêng (ngoài phạm vi bug
này) xem Repo Go-native tạo qua `repo.add` có luồng tạo worktree/terminal tương đương hay không —
nếu không, đó là 1 bug/gap khác, nghiêm trọng hơn (OrcaProject tạo ra rồi không thao tác được),
cần điều tra tiếp bằng CodeGraph trước khi kết luận.

## Việc CHƯA làm (cần xác nhận riêng trước khi code)

- Chưa xác minh: Repo Go-native (tạo qua `repo.add`, thuộc `orca.project.v1.Repo`) có API tạo
  Worktree/mở Terminal tương ứng hay không (proto `Worktree` tồn tại ở
  `backend-go/proto/gen/go/orca/project/v1/project.pb.go`, nhưng chưa xác nhận có RPC
  `worktree.create` nào thật sự dùng nó từ UI Workspace). Nếu không có, đây là bug chặn hoàn toàn
  luồng "tạo Project mới từ Workspace Beta" chứ không chỉ là vấn đề đồng bộ dữ liệu — cần 1 lượt
  điều tra riêng trước khi ưu tiên hoá fix nào ở trên.
- Chưa code bất kỳ thay đổi nào ở trên — đây là bug report + đề xuất, chờ xác nhận phạm vi trước
  khi implement (đặc biệt Bước 2, đụng vào UI đa file + cần Bug 002 xong trước).

## Spec thực thi đầy đủ — ✅ ĐÃ FIX (2026-09-01)

Bug này đã được tách thành spec chi tiết (bám theo `specs/frontend/tdd`), có solution + task
thực thi từng bước, **đã code xong và có test pass**, lưu tại
`specs/frontend/bugs/project-workspace/`:
- Bug: `BUG-FE-PW-001-create-project-dialog-no-duplicate-repo-warning.md`
- Solution: `solutions/SOL-FE-PW-001-duplicate-repo-detection-and-link-existing-project.md`
- Tasks: `tasks/TASK-FE-PW-001-A-detect-duplicate-repo-warning.md`,
  `tasks/TASK-FE-PW-001-B-add-link-existing-project-mode.md`
