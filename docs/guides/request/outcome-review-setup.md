# Bật Outcome Review (review cấp cao của một Request)

Outcome Review cho người duyệt thấy **Request đã/sẽ đổi kiến trúc và luồng thế nào so với thiết kế**, ở mức container (C4 mức 2), trước khi đọc diff. Nội dung đến từ đồ thị tri thức (KG) và SAD của project, cộng với dữ kiện Orca tự đo (tệp đổi, kiểm tra, bao phủ AC). Tab **Outcome review** nằm ngay sau Execution: trước khi có mã chỉ có phần thiết kế (pha `design`); sau khi xong có thêm phần sau mã và quyết định (pha `delivered`).

Mặc định **tắt**. Mỗi review tốn một lượt agent chỉ-đọc (và, khi bật vẽ, thêm các lượt vẽ, mỗi lượt tối đa 4 hình), nên người dùng chủ động bấm; không có chế độ tự động.

## 1. Cần gì trên dev server

| Thành phần | Dùng để | Kiểm |
|---|---|---|
| Agent Orca báo `agent.execPrompt.readonly` và `agent.execPrompt.knowledgeHub` | chạy lượt chỉ-đọc, nạp MCP `tdd-agent` | `orca engine show --project <id> --check` |
| `tdd-agent` trong `PATH` và `.tdd-agent/config.toml` trong repo | đọc KG của project | như trên (dòng `knowledge hub unused`) |
| Skill `tdd-knowledge` (chỉ các công cụ đọc) | lượt context | `npx skills add https://github.com/vnp-community/orca --skill tdd-knowledge --global` |
| Skill `diagram-design` (ghim theo mã băm) | lượt vẽ | xem mục 5: **bản này chưa kèm skill** |
| Python 3 | agent chạy `self_check.py` của `diagram-design` để tự kiểm hình; Orca còn kiểm lại hình bằng mã Go trước khi nhận | `python3 --version` trên dev server |

Skill `tdd-knowledge-writeback` **không** được dùng: review không ghi vào KG.

Thiếu KG thì review vẫn có phần dữ kiện của Orca và nói rõ `kg_unavailable`; pha `design` cần KG nên không chạy được.

## 2. Biến môi trường của `request-service`

Đặt trong `deploy/dev/.env`, đọc một lần lúc khởi động: `./deploy/dev/scripts/redeploy-local.sh request-service`. Giá trị sai không làm service chết; nó giữ mặc định an toàn và ghi cảnh báo.

| Biến | Mặc định | Ý nghĩa |
|---|---|---|
| `REQUEST_DEFAULT_OUTCOME_REVIEW` | `off` | `off` hoặc `manual`, cho project chưa lưu lựa chọn. Lựa chọn đã lưu của project luôn thắng |
| `REQUEST_REVIEW_KILL_SWITCH` | `false` | `true`: ẩn tab, mọi RPC review trả `REQUEST_REVIEW_DISABLED`; **không xoá dữ liệu** |
| `REQUEST_REVIEW_DRAW_ENABLED` | `true` | `false`: chỉ chữ và bảng, không bao giờ khởi chạy lượt vẽ |
| `REQUEST_REVIEW_DRAW_ENGINE` | `builtin` | Ai vẽ hình: `builtin` (Orca tự bố trí mô hình đã kiểm tra thành SVG: miễn phí, tức thì, không cần dev server hay skill), `agent` (agent chỉ-đọc dùng skill `diagram-design`, cần skill trên dev server), `auto` (agent khi chạy được, không thì `builtin`) |
| `REQUEST_REVIEW_CONTEXT_TTL` | 168h | thời gian dùng lại một kết quả context đã kiểm (1h–2160h) |
| `REQUEST_REVIEW_CONTEXT_TIMEOUT` | 10m | một lượt context (1m–15m; agent tự dừng lượt ở 15 phút) |
| `REQUEST_REVIEW_CONTEXT_MAX_FILES` | 60 | số tệp agent được mở trong một lượt (5–400) |
| `REQUEST_REVIEW_ARTIFACT_RETENTION` | 2160h | thời gian giữ hình không còn hiệu lực (24h–5 năm) |
| `REQUEST_REVIEW_ARTIFACT_QUOTA_MB` | 512 | dung lượng hình tối đa của một tenant (1–100000) |
| `REQUEST_REVIEW_REGEN_PER_DAY` | 5 | số lần sinh lại cho một Request mỗi ngày (1–1000) |
| `REQUEST_REVIEW_CONFIRM_COST` | không đặt | số USD ước tính trên đó sinh lại phải được người dùng xác nhận |

## 3. Bật cho một project

Chỉ admin. Mặc định của deploy là `off`.

```bash
orca engine set  --project <id> --outcome-review manual     # off | manual | default (theo triển khai)
orca engine show --project <id> --check
```

`--check` hỏi lại dev server và nêu **vì sao review không chạy được** (agent quá cũ, thiếu skill, không có KG). Lưu cài đặt không bị chặn bởi kết quả kiểm.

Một Request chỉ review được từ `awaiting_plan_approval` trở đi; trước đó mã `REQUEST_REVIEW_NOT_READY`.

## 4. Lỗi `REQUEST_REVIEW_*` thường gặp

| Mã | Nghĩa | Cách xử lý |
|---|---|---|
| `DISABLED` | project để `off`, hoặc công tắc khẩn đang bật | bật ở mục 3; kiểm `REQUEST_REVIEW_KILL_SWITCH` |
| `NOT_READY` | Request chưa có Plan để review | chờ tới `awaiting_plan_approval` |
| `AGENT_TOO_OLD` | agent không báo đủ khả năng (chỉ-đọc, nạp hub) | cập nhật agent, `orca engine show --check` |
| `SKILL_MISSING` | thiếu `tdd-knowledge`, hoặc `diagram-design` không đúng bản đã ghim | cài/cập nhật skill, kiểm lại |
| `NO_DEV_SERVER` | project chưa có dev server hoặc repo chưa có đường dẫn | nối dev server |
| `KNOWLEDGE_REQUIRED` | pha `design` cần KG nhưng hub không tải được | sửa `.tdd-agent/config.toml` / `tdd-agent` |
| `RESULT_INVALID` | agent trả kết quả sai lược đồ hai lần | thử lại; mở run để xem mã chi tiết |
| `RUN_IN_PROGRESS` | đã có một lượt cùng loại đang chạy cho Request | chờ lượt đó; tự giải phóng khi quá hạn |
| `STALE_REVISION` | ai đó đã đổi review trước bạn | tải lại rồi làm lại |
| `DECISIONS_OPEN` | duyệt khi còn mục cần quyết định | quyết định hết các mục |
| `CONFIRM_COST_REQUIRED` | chi phí ước tính vượt `REQUEST_REVIEW_CONFIRM_COST` | gửi lại kèm xác nhận |
| `FORBIDDEN` | chỉ người báo hoặc admin được quyết định/duyệt | đăng nhập đúng vai |
| `DUPLICATE_REQUEST` | cùng một yêu cầu được gửi hai lần cùng lúc | bỏ qua; kết quả của lần đầu là câu trả lời |

Agent MCP chỉ **đọc** review (`request_review_get|estimate|list|runs|decisions`). Sinh, sinh lại, quyết định và duyệt luôn do người.

## 5. Skill `diagram-design` và cập nhật bản ghim

Lượt vẽ chỉ chạy với đúng bản skill mà Orca ghim (mã băm 16 ký tự của `SKILL.md` do agent báo; hằng `PinnedDiagramDesignSHA` trong `backend-go/services/request-service/internal/domain/dev_server_capability_review.go`). Bản khác bị coi là thiếu (`SKILL_MISSING`).

**Trạng thái bản này:** skill `diagram-design` chưa được đưa vào thư mục `skills/` của repo và hằng ghim đang rỗng. Trong lúc rỗng, lượt vẽ chấp nhận mọi bản đã cài và ghi lại bản nó thấy; nếu chưa cài, review vẫn hiển thị đầy đủ chữ, bảng, dữ kiện và hình nằm ở trạng thái "chưa vẽ". Có thể chạy giai đoạn 1 (chỉ chữ, mục 7) mà không cần skill.

Khi cập nhật skill: vendor bản mới vào `skills/diagram-design/` kèm `LICENSE`/`NOTICE`, đặt lại mã băm ghim, triển khai `request-service`, rồi `npx skills update diagram-design --global` trên từng dev server. Hình đã vẽ bằng bản cũ vẫn còn và đọc được; chúng chỉ không được dùng lại cho review mới (khoá bộ nhớ đệm có phiên bản skill).

## 6. Dữ liệu, xoá và giới hạn

- Hình lưu theo nội dung (một nội dung một lần cho mỗi tenant) và có thể sinh lại; phiên bản cũ giữ cho đến hết `ARTIFACT_RETENTION`, sau đó vòng dọn nền xoá. Vượt `ARTIFACT_QUOTA_MB` thì lượt vẽ mới bị từ chối cho đến khi dọn.
- `EraseRequest` xoá **nội dung** review: mô hình, ghi chú quyết định và duyệt, kết quả context đã lưu, chữ SVG, tiêu đề, mô tả và lý do của hình. Các dòng còn lại chỉ giữ id, trạng thái và mã băm.
- Công tắc khẩn không xoá gì; bỏ công tắc là các review trở lại nguyên vẹn.

## 7. Triển khai từng bước

| Giai đoạn | Cấu hình | Điều kiện qua |
|---|---|---|
| 0. Thử ở `vnp-qa-platform` | project `manual`, `.tdd-agent/config.toml` có sẵn | `orca engine show --check` không báo lỗi review; một review `design` ra được |
| 1. Chỉ chữ | `REQUEST_REVIEW_DRAW_ENABLED=false` | review có chữ, bảng, dữ kiện; không có lượt vẽ nào trong `request_review_run_duration_seconds{kind="draw"}` |
| 2. Có hình | `REQUEST_REVIEW_DRAW_ENABLED=true`, hồ sơ vẽ mặc định, skill đã ghim | tỉ lệ hình vẽ hỏng (`request_review_diagrams_total{result="failed"}`) chấp nhận được sau khi đo |
| 3. Hồ sơ vẽ, thêm project | lưu hồ sơ theo project; bật thêm project | hồ sơ lưu và áp dụng đúng; chi phí sinh lại nằm trong ngưỡng |

Ngưỡng số **chưa được đặt**: đo trên 10 Request thử trước khi quyết định.

## 8. Số liệu và log

Prometheus của `request-service`: `request_review_context_lookups_total{result}` (tỉ lệ trúng bộ nhớ đệm), `request_review_artifact_lookups_total{result}`, `request_review_run_duration_seconds{kind,result}`, `request_review_tokens_total{kind,direction}` (chỉ khi agent báo), `request_review_cost_estimate_error_ratio{kind}`, `request_review_diagrams_total{result}`, `request_review_draw_failures_total{error_code}`, `request_review_regenerations_total{reason}`, `request_review_time_to_first_facts_seconds`, `request_review_agent_items_total{verified}`, `request_review_design_reviewed_ratio`.

Log có `audit=true` cho sinh, sinh lại, quyết định, duyệt, kèm `inputs_digest`, **không có nội dung**. Sự kiện outbox: `orca.request.review.generated|artifact_regenerated|decided|progress` (chỉ có id và enum); gateway đẩy `request.event` với `eventType` `review.changed` hoặc `review.progress`.
