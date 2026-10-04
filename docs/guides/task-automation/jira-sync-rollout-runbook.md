# Runbook — triển khai liên kết Jira ↔ task ↔ worktree trên server dev

**Áp dụng cho:** CR-TG-008 (`docs/crs/v4/task-graph/`). Người chạy: người có quyền SSH vào server deploy và quyền sửa Postgres của stack `deploy/dev`.
**Mức rủi ro:** thấp. Cả hai migration chỉ **thêm** (một bảng mới và các cột có mặc định); bản cũ của các service vẫn chạy được trên schema mới, nên có thể quay lại bằng cách đưa binary cũ lên mà không cần chạy `down`.

> Cần các biến `SERVER_HOST`, `SERVER_USER`, `SERVER_KEY`, `SERVER_PORT`, `SERVER_DEPLOY`, `POSTGRES_PASSWORD` trong `deploy/dev/.env`.
>
> Mọi lệnh bên dưới chạy từ thư mục gốc của repo trên máy bạn, trừ khi ghi "trên server". Biến `R` là cách gọi lệnh trên server:
>
> ```bash
> export $(grep -v '^#' deploy/dev/.env | xargs)   # như các script deploy: KHÔNG dùng `source`, vì shell sẽ mở rộng dấu ~ thành home trên máy bạn
> R="ssh -i ${SERVER_KEY/#\~/$HOME} -p ${SERVER_PORT:-22} ${SERVER_USER:-ubuntu}@${SERVER_HOST}"
> D="cd ${SERVER_DEPLOY:-~/orca-go-deploy} &&"
> ```

## 0. Có gì thay đổi

| Thành phần | Thay đổi |
|---|---|
| `task-service` DB `task` | migration `0012_task_sources` (bảng `task.task_sources`), `0013_execution_leases` (3 cột + 1 index trên `task.execution_links`) |
| `issue-status-sync` (service mới) | DB riêng `issuestatussync` (migration `0001`), tiêu thụ sự kiện NATS, gọi Jira thay mặt người dùng |
| `project-service`, `git-gateway-service`, `scm-integration-service`, `issue-tracking-service`, `api-gateway`, `task-service` | binary mới |
| Frontend | bản mới (cờ **Experimental → Jira task link** mặc định tắt) |

`migrate.sh` đã tự tạo DB `issuestatussync` nếu thiếu, và `sync-to-server.sh` bước 5 gọi `migrate.sh --remote` cho **toàn bộ** danh sách, nên chạy trọn quy trình ở mục 3 là đủ. Các mục 1–2 là bước an toàn nên làm trước.

## 1. Trước khi chạy (5 phút)

**1.1 Sao lưu hai DB sẽ bị ảnh hưởng** (trên server, nhờ `R`):

```bash
$R "$D mkdir -p backups && for db in task project; do docker compose exec -T postgres pg_dump -U orca -d \$db -Fc > backups/\$db-\$(date +%Y%m%d-%H%M).dump; done; ls -lh backups | tail -4"
```

**1.2 Xem phiên bản migration hiện tại của `task`** (ghi lại để so sánh):

```bash
$R "$D docker compose exec -T postgres psql -U orca -d task -c 'select version, dirty from schema_migrations;'"
```

Mong đợi `dirty = f`. Nếu `dirty = t` thì **dừng**: có một migration trước đó đang hỏng, xử lý trước (xem mục 6).

**1.3 (Khuyến nghị) Tắt đồng bộ trạng thái Jira cho mọi project, bật dần sau.** Cờ `issue_status_sync_enabled` mặc định bật cho project mới; khi service chạy, mọi worktree tạo từ issue Jira sẽ đẩy issue đang `To Do` sang "In Progress". Để chủ động:

```bash
$R "$D docker compose exec -T postgres psql -U orca -d project -c 'update project.projects set issue_status_sync_enabled = false;'"
```

Muốn bật lại cho một project thử nghiệm: thay `false` bằng `true` và thêm `where name = '<tên project>'`.

## 2. Kiểm tra bản build (trên máy bạn, không đụng server)

```bash
bash -n deploy/dev/scripts/*.sh && echo scripts-ok
POSTGRES_PASSWORD=x VAULT_TOKEN=x docker compose -f deploy/dev/docker-compose.yml --profile migrate config --services | grep -E 'issue-status-sync|migrate-issuestatussync|migrate-task'
```

Phải thấy cả ba tên. Nếu thiếu, bạn đang ở nhánh cũ: `git pull` rồi chạy lại.

## 3. Triển khai

Có hai cách; **cách A** gọn nhất, **cách B** cho phép dừng giữa chừng để kiểm tra.

### Cách A — một lệnh (build, rsync, migrate, khởi động lại)

```bash
./deploy/dev/scripts/sync-to-server.sh <phiên-bản>      # ví dụ 0.4.60
```

Script chạy theo thứ tự: build cục bộ → rsync → chuyển image git-gateway → pull image công khai → **migration (bước 5)** → khởi động lại toàn stack (bước 6). Migration luôn chạy **trước** khi binary mới lên.

### Cách B — tách bước

```bash
./deploy/dev/scripts/build-local.sh                      # tạo deploy/dev/bin/* và dist/
# rồi đồng bộ như bước [2/6]-[4/6] của sync-to-server.sh, hoặc chạy script ở cách A nhưng dừng sau bước 5:
./deploy/dev/scripts/migrate.sh --remote task            # áp dụng 0012 và 0013
./deploy/dev/scripts/migrate.sh --remote issuestatussync # tạo DB (nếu thiếu) và migrate
```

Chỉ sau khi mục 4 đạt mới khởi động lại service:

```bash
$R "$D docker compose up -d"                             # hoặc up -d <service> theo thứ tự ở mục 5
```

## 4. Kiểm tra sau migration (trước khi khởi động service mới)

```bash
# phiên bản và trạng thái sạch
$R "$D docker compose exec -T postgres psql -U orca -d task -c 'select version, dirty from schema_migrations;'"          # version 13, dirty = f
$R "$D docker compose exec -T postgres psql -U orca -d issuestatussync -c 'select version, dirty from schema_migrations;'" # version 1, dirty = f

# cấu trúc mới đã có
$R "$D docker compose exec -T postgres psql -U orca -d task -c '\d task.task_sources' -c '\d task.execution_links'"
# task.task_sources có unique index idx_task_sources_unique; execution_links có lease_expires_at, lease_owner, previous_status

# dữ liệu cũ nguyên vẹn (so với số đếm trước khi chạy nếu bạn có ghi lại)
$R "$D docker compose exec -T postgres psql -U orca -d task -c 'select count(*) from task.tasks;' -c 'select count(*) from task.execution_links;'"
```

## 5. Khởi động lại và xác nhận

Thứ tự khuyến nghị nếu bạn tự khởi động từng service (script ở cách A làm tất cả cùng lúc): `task-service` → `project-service` → `git-gateway-service` → `issue-tracking-service` → `scm-integration-service` → `api-gateway` → `issue-status-sync` → frontend.

```bash
$R "$D docker compose ps issue-status-sync task-service"
$R "$D docker compose logs --tail=40 issue-status-sync"      # không có lỗi kết nối NATS/DB
$R "$D docker compose logs --tail=40 task-service"           # không có "column ... does not exist"
```

## 6. Thử luồng thật (khoảng 10 phút)

Dùng một project có Jira đã kết nối và `issue_status_sync_enabled = true` (xem 1.3), và **một issue thử nghiệm đang ở "To Do"**.

1. **Jira → worktree:** Settings → Experimental → bật **Jira task link**. Trang Tasks → nguồn Jira → "Start work" trên issue thử → tạo workspace.
   Kỳ vọng: trong vài giây issue chuyển **In Progress** (đúng một lần). Nếu không: `docker compose logs issue-status-sync` sẽ nói lý do (không có người thực hiện, provider không hỗ trợ, issue không còn `To Do`, hoặc workflow Jira không có đường tới "In Progress").
2. **Task:** mở task vừa tạo — thấy badge "Jira ABC-1". Bấm **Execute with Agent**: agent chạy trong đúng worktree vừa tạo (không sinh worktree thứ hai).
3. **Xoá worktree không đổi Jira:** xoá workspace đó → issue vẫn ở nguyên trạng thái, **không** thành Cancelled.
4. **Khôi phục sau khi service chết:** trong lúc agent đang chạy, trên server `docker compose kill -s KILL task-service` rồi `up -d task-service`. Sau khoảng 90–120 giây task trở về trạng thái trước đó (xem `select id, status_mirror from task.execution_links order by started_at desc limit 3;` — link phải là `failed`).

## 7. Quay lại (rollback)

Vì migration chỉ thêm, **quay lại bằng cách đưa bản cũ lên là đủ**:

```bash
git checkout <commit-trước>  &&  ./deploy/dev/scripts/sync-to-server.sh <phiên-bản-cũ>
```

Chỉ khi cần gỡ hẳn cấu trúc mới (mất dữ liệu trong `task_sources`, `lease_*` — nhỏ):

```bash
# trên server, gỡ 0013 rồi 0012 (mỗi lần một bước)
$R "$D docker compose run --rm migrate-task -path /migrations -database 'postgresql://orca:${POSTGRES_PASSWORD}@postgres:5432/task?sslmode=disable' down 1"
$R "$D docker compose run --rm migrate-task -path /migrations -database 'postgresql://orca:${POSTGRES_PASSWORD}@postgres:5432/task?sslmode=disable' down 1"
```

Khôi phục dữ liệu từ bản sao lưu ở 1.1 chỉ cần khi có sự cố khác (`pg_restore -d task --clean`).

**Nếu migration báo `dirty = t`** (chạy dở): xem lỗi trong log `migrate-task`, sửa nguyên nhân, kiểm tra thủ công cấu trúc đã áp dụng đến đâu, rồi `... force <version>` đúng với trạng thái thực tế trước khi chạy lại `up`.

## 8. Những điều cần biết

- **Chỉ Jira** được đồng bộ trạng thái; Linear/GitHub bị bỏ qua có log. Chỉ **người đã kết nối Jira** mới có issue được cập nhật (mỗi người dùng một kết nối).
- Xoá worktree **không bao giờ** đổi issue. PR được tạo/merge chuyển issue sang "In Review"/"Done" chỉ khi issue đang ở nhóm "đang làm" và workflow Jira có đúng tên trạng thái đó; không thì ghi log rồi bỏ.
- Khoá Jira được lấy từ tên branch, tiêu đề PR, hoặc từ mô tả PR khi đứng sau `fixes`/`closes`/`resolves`.
- Task còn `in_progress` mà không có link nào sẽ được trả về `open` sau 30 phút.
- Chưa thử trên Jira thật: bước 6 là lần chạy thật đầu tiên. Nếu bước 1 của mục 6 không chuyển trạng thái, kiểm tra tên trạng thái "In Progress" trong workflow Jira của project.
