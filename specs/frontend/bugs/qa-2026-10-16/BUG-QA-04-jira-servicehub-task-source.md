# BUG-QA-04: Cấu hình nguồn task Jira `https://jr.servicehub.vn` bằng email + PAT trong `.env`

**Mức độ:** P1 · **Khu vực:** issue-tracking-service (adapter Jira), frontend (hộp thoại kết nối), triển khai (`.env`) · **Trạng thái:** đã sửa 2026-10-16 (Jira Server/DC: Bearer PAT, API v2); QA-JIRA-01..03 và QA-JIRA-LIVE-01 xanh, `check-jira-from-env.sh` OK trên jr.servicehub.vn

## Yêu cầu

Tab Tasks → nguồn Jira phải lấy được task từ `https://jr.servicehub.vn` bằng **email và Personal Access Token (PAT) lấy từ `.env`**, không phải gõ tay trong giao diện mỗi lần.

## Hiện trạng (rà soát 2026-10-16)

- `.env` ở gốc repo (gitignored) có ba biến `JR_URL`, `JR_EMAIL`, `JR_PAT_TOKEN`. **Không dịch vụ nào đọc chúng**: `docker compose` của `deploy/dev` chỉ đọc `deploy/dev/.env`, và không service nào có biến Jira.
- `jira.status` trả `{"connected": false}`; tab Tasks → Jira hiện "Connect your Jira site". Hộp thoại kết nối yêu cầu nhập URL, email, API token.
- `jr.servicehub.vn` là **Jira Server/Data Center**: `GET /rest/api/2/serverInfo` không kèm thông tin xác thực trả `401` (Cloud trả thông tin công khai). PAT của Server/DC chỉ dùng được dạng `Authorization: Bearer <PAT>`; dạng Basic `email:PAT` bị từ chối.

## Nguyên nhân gốc

1. **Đúng cấu hình người dùng muốn (có email + PAT) lại chọn sai cách xác thực.** `jira/client.go` `authHeaderValue` chọn Basic khi `Email != ""`, Bearer khi trống. Có email (như `.env`) → Basic với PAT làm mật khẩu → `401`. Ghi chú CR-JIRA-001/BUG-013 và hộp thoại tự thừa nhận "chỉ Bearer (email trống) chạy được trên site này": người dùng phải biết bỏ trống email.
2. **Dò phiên bản API thất bại thì rơi về Cloud.** `probeAPIVersion` gọi `/rest/api/2/serverInfo` bằng cùng tiêu đề; với Basic sai bị `401` nên trả `"3"`, mọi lời gọi sau đi `/rest/api/3/…` (không tồn tại trên Server/DC) → 404. Kết quả đệm 10 phút nên lỗi kéo dài ngay cả sau khi sửa thông tin xác thực.
3. **Không có đường cấu hình từ `.env`.** Không biến môi trường, không script, không kết nối khởi tạo; giao diện không biết giá trị.

## Giải pháp và task

[solutions/SOL-QA-04-jira-servicehub.md](./solutions/SOL-QA-04-jira-servicehub.md); task T-QA-04-xx; test QA-JIRA-01..03 (Python, có máy chủ Jira Server giả), QA-JIRA-LIVE-01 (thật, bật bằng cờ), QA-UI-JIRA-01.

> An toàn: PAT chỉ đọc từ `.env`/môi trường, không in, không ghi log, không đưa vào tham số dòng lệnh. Bộ test thật chỉ chạy khi `ORCA_E2E_JIRA_LIVE=1`.
