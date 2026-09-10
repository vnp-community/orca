# backend-go Tasks — Orca CLI / F09 (v4)

**Solutions:** [../solutions/](../solutions/README.md)

## Track 1 — `wscompat` bearer-JWT identity (BE-CLI-SOL-001)

| Task | Depends on | Status |
|---|---|---|
| [TASK-BE-CLI-001](./TASK-BE-CLI-001-wscompat-handler-bearer-fallback.md) — `Handler.resolveIdentity` fallback bearer JWT | Không | ✅ DONE |
| [TASK-BE-CLI-002](./TASK-BE-CLI-002-wire-bearer-auth-into-wscompat-main.md) — wire `authValidator` có sẵn vào `main.go` | TASK-BE-CLI-001 | ✅ DONE |

## Track 2 — Credential headless CLI (BE-CLI-SOL-002) — ⚠️ chứa task security-sensitive

| Task | Depends on | Status |
|---|---|---|
| [TASK-BE-CLI-003](./TASK-BE-CLI-003-cli-token-mint-route.md) — route mint, ép `user_id` = caller | Không | ✅ DONE (merge sau 004, đúng thứ tự) |
| [TASK-BE-CLI-004](./TASK-BE-CLI-004-issue-service-token-caller-authorization.md) — ⚠️ vá KNOWN GAP authorization ở usecase | Không | ✅ DONE |
| [TASK-BE-CLI-005](./TASK-BE-CLI-005-cli-token-revocation.md) — ⚠️ domain revocation + check verify path | TASK-BE-CLI-004 (nên biết trước) | ✅ DONE |
| [TASK-BE-CLI-006](./TASK-BE-CLI-006-cli-token-audit-and-list-revoke-routes.md) — audit log + route list/revoke | TASK-BE-CLI-003, TASK-BE-CLI-005 | ✅ DONE |

## Track 3 — Rename `channels_cli.go` (BE-CLI-SOL-003)

| Task | Depends on | Status |
|---|---|---|
| [TASK-BE-CLI-007](./TASK-BE-CLI-007-rename-channels-cli-installer.md) — `gitnexus rename` + doc | Không | ✅ DONE |

## Thứ tự thực thi

```
Track 1: 001 → 002                              (tuyến tính)
Track 2: 003 ┐
             ├→ 006   (003, 005 xong mới tới 006)
         004 ┤
         005 ┘        (005 nên đợi kết luận 004, không bắt buộc — xem ghi chú)
Track 3: 007                                     (độc lập hoàn toàn)
```

3 track độc lập nhau, chạy song song được. Trong Track 2: 003 và 004 có thể bắt đầu
song song (khác file, khác mục tiêu — 003 sửa `api-gateway`, 004 sửa
`auth-service`), nhưng **003 không được MERGE trước khi 004 có kết luận** (xem cảnh
báo bảo mật ở đầu mỗi file); 005 kỹ thuật có thể làm độc lập với 004 nhưng nên chờ
kết luận 004 trước vì lược đồ bảng `issued_service_tokens` có thể cần thêm cột nếu
phương án B (mint hộ user khác) được chọn; 006 cần cả 003 (route mint tồn tại để
gắn audit) và 005 (usecase revoke tồn tại để gắn audit) xong trước.

## Nguyên tắc chung cho AI thực thi các task này

- **Luôn chạy `impact()`/`codegraph explore` trước khi sửa symbol** — mỗi task đã
  ghi rõ symbol cần kiểm tra ở mục "gitnexus", nhưng đây là yêu cầu bắt buộc chung
  theo `CLAUDE.md`/`AGENTS.md`, không chỉ khi task nhắc tới.
- **2 task đánh dấu ⚠️ (TASK-BE-CLI-004, và phần verify-path của TASK-BE-CLI-005)
  KHÔNG được coi là task code thông thường** — đây là các điểm CR-CLI-002 tự ghi
  "cần security review trước khi triển khai", cộng thêm 1 lỗ hổng authorization
  CÓ THẬT đã tồn tại sẵn trong code hiện tại mà việc re-verify khi viết bộ
  solution/task này phát hiện ra (xem BE-CLI-SOL-002 mục 1b). AI thực thi các task
  này KHÔNG được tự ý chọn phương án và merge — dừng lại đúng chỗ task ghi, báo cáo
  lại cho người yêu cầu.
- **`tenantID`/`userID`/`user_id` luôn lấy từ identity đã xác thực của caller**
  (`identityFromContext(ctx)` ở `httpgateway`, `usecase.Identity` sau
  `AuthValidator.Validate`) — không bao giờ từ request body/query param. Vi phạm
  nguyên tắc này ở Track 2 (đặc biệt TASK-BE-CLI-003/006) tạo lỗ hổng leo thang đặc
  quyền, không phải bug thông thường.
- **`buf generate` sau bất kỳ thay đổi `.proto` nào** (TASK-BE-CLI-006) — kiểm tra
  không phá `proto/gen/go` dùng chung với service khác trước khi commit.
- **`gitnexus rename`, không find-and-replace** cho mọi việc đổi tên symbol
  (TASK-BE-CLI-007 là ví dụ rõ nhất, nhưng áp dụng cho MỌI task nếu phát sinh nhu
  cầu đổi tên symbol khác trong lúc làm).
- **Test trước, không giả định pass** — mọi lệnh `go test`/`buf generate` trong mục
  "Verify" phải thực sự chạy và thấy kết quả, không suy đoán.
- **Không tự ý mở rộng phạm vi** — mỗi task chỉ sửa đúng file đã liệt kê; nếu phát
  hiện gap khác trong lúc làm, ghi nhận lại, không sửa luôn nếu ngoài phạm vi task.
