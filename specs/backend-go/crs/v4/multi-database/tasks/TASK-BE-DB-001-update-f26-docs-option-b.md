# TASK-BE-DB-001: Cập nhật tài liệu theo quyết định Option B (CR-DB-001)

**Solution:** không có (CR-DB-001 thuần tài liệu, xem [solutions/README.md](../solutions/README.md)) | **CR:** CR-DB-001
**Service:** không có — chỉ sửa markdown
**Depends on:** Không
**Status:** ✅ DONE (2026-09-09)

> **Kết quả thực tế:** Cả 3 file đã sửa đúng như mô tả, không lệch phạm vi.
> `docs/features/F26-multi-database.md` — thêm mục mới "Phạm vi mở rộng:
> SaaS platform (`backend-go`)" ngay cuối file (sau bảng Metrics), giữ
> nguyên 100% nội dung cũ. **Điều chỉnh so với bản nháp trong task doc**:
> link "task chi tiết" trỏ tới
> `specs/backend-go/crs/v4/multi-database/README.md` không tồn tại trên
> thực tế (chỉ có `tasks/README.md` và `solutions/README.md` dưới thư mục
> đó, không có `README.md` trực tiếp) — đã đổi thành
> `../../specs/backend-go/crs/v4/multi-database/tasks/README.md`, xác nhận
> bằng `ls` từ đúng vị trí tương đối của file. `docs/roadmap/feature-completion-matrix.md`
> dòng 64 và dòng 117 đổi đúng nội dung ô đã lên kế hoạch, không đổi cấu
> trúc bảng (`grep -c '|'` không đổi số cột — xác nhận bằng đếm `|` mỗi
> dòng liên quan trước/sau). `specs/backend-go/tdd/architecture/04-tech-stack.md`
> dòng 33 đổi đúng cột "Why", giữ nguyên câu "TiDB escape hatch" như yêu
> cầu. Đã Read lại cả 3 file sau khi sửa và `ls` xác nhận mọi link tương
> đối trỏ đúng file tồn tại. Không sửa code Go, không cần `impact()`.

---

## Mục tiêu

CR-DB-001 đã chốt quyết định chính thức (2026-09-09): **Option B —
`backend-go` hỗ trợ multi-dialect thật**. Cập nhật đúng 3 file tài liệu đã
liệt kê trong CR-DB-001's "Changes Required" để phản ánh quyết định này,
không còn mô tả F26/backend-go như "regression chưa xác nhận".

Đây là task **thuần tài liệu, không sửa code Go** — không cần chạy
`impact()`/gitnexus.

## Files cần sửa

1. `docs/features/F26-multi-database.md` (MODIFY)
2. `docs/roadmap/feature-completion-matrix.md` (MODIFY — dòng 64 và 117)
3. `specs/backend-go/tdd/architecture/04-tech-stack.md` (MODIFY — dòng 33)

## Nội dung sửa

### 1. `docs/features/F26-multi-database.md`

File hiện mô tả F26 hoàn toàn theo phạm vi `backend/` legacy (Electron TS,
`src/main/db/**`). Thêm 1 mục mới ngay sau phần mô tả hiện có (không xoá
nội dung cũ — nó vẫn đúng cho `backend/`), ví dụ:

```markdown
## Phạm vi mở rộng: SaaS platform (`backend-go`)

Quyết định 2026-09-09 (xem
[`docs/crs/v4/multi-database/CR-DB-001`](../crs/v4/multi-database/CR-DB-001-postgres-lockin-gap-analysis-and-decision.md)):
`backend-go` (kiến trúc microservices Go, khác `backend/` legacy TS ở
trên) cũng sẽ hỗ trợ multi-dialect thật — Postgres (mặc định) + MySQL/TiDB
— theo use case "Enterprise: dùng PostgreSQL/MySQL cluster sẵn có". Triển
khai qua [CR-DB-002](../crs/v4/multi-database/CR-DB-002-dialect-capability-layer-foundation.md)
(capability layer) và [CR-DB-003](../crs/v4/multi-database/CR-DB-003-mysql-tidb-adapter-backend-go.md)
(adapter MySQL/TiDB), giai đoạn đầu pilot 1 service
(`usage-service`) — xem
[`specs/backend-go/crs/v4/multi-database/`](../../specs/backend-go/crs/v4/multi-database/README.md)
cho thiết kế/task chi tiết. `backend-go` **không** hỗ trợ SQLite (khác
`backend/` legacy) — lý do loại trừ ở CR-DB-002.
```

Điều chỉnh đường dẫn tương đối cho đúng vị trí thật của file khi sửa (kiểm
tra bằng cách đọc lại các link tương đối khác đã có sẵn trong chính file
này).

### 2. `docs/roadmap/feature-completion-matrix.md`

**Dòng 64** — đổi cột "Ghi chú" của hàng F26 từ:

```
**BG chỉ hỗ trợ Postgres — chưa có SQLite/MySQL/TiDB dialect abstraction như spec mô tả (regression so với `docs/features/F26`)**
```

thành (giữ nguyên cấu trúc bảng, chỉ đổi nội dung ô cuối):

```
Option B đã quyết định (CR-DB-001, 2026-09-09) — `backend-go` sẽ hỗ trợ multi-dialect thật qua CR-DB-002/CR-DB-003 (pilot `usage-service`), không còn là regression chưa xác nhận
```

**Dòng 117** (bảng "Khoảng trống cần xử lý", cột "Đề xuất" của hàng #1) —
đổi từ:

```
Xác nhận đây có phải là quyết định kiến trúc có chủ đích (chuẩn hoá về Postgres cho SaaS) hay là thiếu sót cần bổ sung
```

thành:

```
Đã xác nhận (CR-DB-001, 2026-09-09): Option B — multi-dialect thật, không phải chuẩn hoá Postgres theo chủ đích. Triển khai qua CR-DB-002/CR-DB-003, xem specs/backend-go/crs/v4/multi-database/
```

Cột "Ảnh hưởng" của cùng hàng #1 giữ nguyên (mô tả ảnh hưởng vẫn đúng).

### 3. `specs/backend-go/tdd/architecture/04-tech-stack.md`

**Dòng 33** hiện tại (trong bảng "Data access", hàng "Migrations"):

```
| Migrations | `golang-migrate`, one migration directory per service, numbered sequentially (mirrors the TS system's own `0001, 0002, …` convention for continuity) | Battle-tested, dialect-agnostic enough to keep a TiDB escape hatch open the way ADR-002/ADR-021 did for the TS system |
```

Sửa cột "Why" — **không xoá câu "keep a TiDB escape hatch open"**, làm rõ
đây nay là mục tiêu đang triển khai thật, không phải tuyên bố mâu thuẫn
với thiết kế:

```
| Migrations | `golang-migrate`, one migration directory per service (further split into `<dialect>/` subdirectories where a service supports >1 dialect — see `usage-service`'s pilot), numbered sequentially per dialect | Battle-tested; the "TiDB escape hatch" is no longer aspirational — CR-DB-001 (2026-09-09) confirmed Option B (real multi-dialect support), being implemented via CR-DB-002/CR-DB-003, piloted on `usage-service` |
```

## Verify

Không có lệnh build/test — verify bằng cách **Read lại 3 file sau khi
sửa** và xác nhận đúng nội dung đã lên kế hoạch ở trên, đúng cú pháp
markdown (bảng không lệch cột), và mọi link tương đối trỏ đúng file tồn
tại (kiểm tra bằng `ls`/`Read` đường dẫn đích, không chỉ nhìn cú pháp).

```bash
# Xác nhận không phá cấu trúc bảng markdown (số cột mỗi hàng phải khớp)
grep -c '|' docs/roadmap/feature-completion-matrix.md
```

## gitnexus

Không áp dụng — task này không sửa code, chỉ sửa markdown.
