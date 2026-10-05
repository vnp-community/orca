# Thử nghiệm tạo task qua MCP (project Vnp-asm)

Bộ test chạy trên server qua công cụ MCP `task_create` và các công cụ liên quan, tạo task trong project
**Vnp-asm** (đổi bằng `ORCA_MCP_TASK_PROJECT=<tên hoặc uuid>` trong `tests/mcp/.env`) và **xoá hết ở cuối**
bằng kênh `/ws` `task.delete`.

## Chạy
```bash
cd tests/mcp
python3 -m venv --system-site-packages .venv && .venv/bin/pip install websocket-client   # một lần
cd task
../.venv/bin/python run_all.py                  # tất cả suite (~3 phút, có giãn nhịp)
../.venv/bin/python run_all.py fields update    # một số suite
../.venv/bin/python check_task_create_fields.py # mỗi file chạy độc lập
```
Điều kiện: gateway bật pack ghi (`MCP_TOOL_PACKS_ENABLED=1,2`), tài khoản admin trong `.env` (tự cấp PAT
`orca:read`+`orca:write`), và `websocket-client` để dọn dẹp. Thiếu `websocket-client` thì test báo FAIL kèm danh sách
id cần xoá tay.

## Suite
| Suite | File | Kiểm tra |
|---|---|---|
| fields | `check_task_create_fields.py` | title/project_id, biên 1/500/501 ký tự, rỗng, sai kiểu, tham số ngoài hợp đồng, Việt/emoji/HTML, đọc lại, `taskNumber` tăng |
| hierarchy | `check_task_create_hierarchy.py` | `parent_id` (con, cháu), cha không tồn tại/sai định dạng, `depends_on` (vòng, tự phụ thuộc, loại sai) |
| update | `check_task_create_update.py` | labels, title, máy trạng thái (blocked/open/review/done, `in_progress` bị chặn, `done` là cuối), bình luận |
| from-source | `check_task_create_from_source.py` | `task_createFromSource`: `created` true/false, trùng khoá, provider hợp lệ/không hợp lệ, `task_getSource` |
| concurrency-access | `check_task_create_concurrency_access.py` | 8 lời gọi song song (id/số thứ tự không trùng), token chỉ-đọc và không token không tạo được |

## Kết quả: PASS / FAIL / XFAIL
`XFAIL` là **lỗi đã biết**: hành vi đúng mà hệ thống hiện chưa đạt. Hiện rõ trong báo cáo nhưng không làm
lần chạy thất bại; khi lỗi được sửa nó tự chuyển thành `PASS`. Hiện có 4:
- title toàn khoảng trắng vẫn tạo được task (`domain.NewTask` chỉ chặn `title == ""`);
- `project_id` không tồn tại vẫn tạo được task (task-service cố ý không kiểm tra project);
- `task_getSubtree` trả `{}` dù cây có con (cả kênh `/ws` lẫn MCP);
- `totalSubtasks` của cha vẫn 0 sau khi tạo con và gọi `task_recalculateProgress`.

## Bộ chặn lời gọi lặp
mcp-service chặn lời gọi **giống hệt** lặp lại (mặc định ≥5 trong 60 giây thì chậm lại, ≥20 trong 5 phút thì bị chặn vài
phút). Bộ test tự giãn nhịp và thử lại (`task_test_bed.py`), nên các dòng `[info] giãn nhịp` là bình thường. Đừng tự gọi
lặp `task_get`/`task_list` cùng tham số khi đang chạy.

## Lưu ý về dữ liệu
Task thử nghiệm mang tiền tố `ORCA_TEST_PREFIX` (mặc định `mcptest`). `taskNumber` là bộ đếm toàn cục nên các số bị
bỏ trống sau khi xoá; đó là bình thường.
