# BE-CV-TASK-035-01: Re-verify nguồn lưu trữ và chữ ký `RepoSourceReader`, dựng bộ fixture gốc

**From Solution:** BE-CV-SOL-035-storage-map
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/testdata/storage/PROVENANCE.txt` (mới); `backend-go/services/code-intel-service/testdata/storage/{compose-dev.yml, compose-prod.yml, compose-old.yml, backend-go-compose.yml, init-databases.sh}` (mới, bản chụp đã che tay); kết quả đối chiếu ghi vào mô tả PR
**Depends on:** BE-CV-SOL-030 (đã có chữ ký `RepoSourceReader`; nếu chưa, task này chỉ làm phần compose/config và ghi phần reader là "chưa kiểm chứng")
**Status:** [ ] TODO

---

## Context

Solution 035 dựa vào số liệu do CR đọc ngày 2026-10-05 và vào một cổng đọc file chưa tồn tại. Bước này khoá sự thật trước khi viết parser. Chỉ lệnh đọc; không chạy compose, không chạy `docker`.

## Việc cần làm

1. Đọc lại và ghi vào mô tả PR bảng đối chiếu (đúng/lệch) cho: số khối service ở `deploy/dev/docker-compose.yml` (mong đợi 19 khối ứng dụng; 17 job `migrate-*`), 17 tên trong `DATABASES` của `deploy/dev/docker/postgres/init-databases.sh`, literal `POSTGRES_PASSWORD: orca` và `VAULT_DEV_ROOT_TOKEN_ID: dev-root-token` ở `backend-go/docker-compose.yml`, `container_name` của khối `postgres` (mong đợi `orca-go-postgres`), `VAULT_ADDR` literal ở `x-go-common-env`.
2. So hai bản danh sách DB: `deploy/dev/docker/postgres/init-databases.sh` và `backend-go/deploy/postgres-init-databases.sh`; ghi chênh lệch (nếu có) vào PR; **chỉ** bản `deploy/dev/...` là nguồn của solution.
3. Với từng service trong `backend-go/services/*`, đọc `internal/config/*.go` (không `_test.go`) và ghi: có dùng mẫu `os.Getenv` / `StringEnv` / hàm `*Env` cục bộ nào. Liệt kê service **không** theo mẫu để `service_config_env_keys.go` (TASK-035-06) có ca thử. Ghi riêng `api-gateway/internal/config/config_mcp.go`.
4. Đọc chữ ký thực của `RepoSourceReader` (solution `BE-CV-SOL-030`) và xác nhận bốn khả năng: đọc một đường dẫn có hạn mức; liệt kê theo tiền tố/glob; grep literal; `ContentID` rẻ cho từng tệp. Ghi kết quả "có / không / chưa kiểm chứng" cho từng khả năng; **khả năng nào thiếu thì tạo mục trong "Rủi ro" của solution 035 §7 và một câu hỏi cho chủ `BE-CV-SOL-030`**, không tự thêm vào cổng.
5. Tạo `testdata/storage/*` bằng cách sao chép tối thiểu từ các tệp thật rồi **che tay**: thay mọi giá trị thật bằng literal giả có tiền tố `CANARY_` (ví dụ `POSTGRES_PASSWORD: CANARY_pg_pass_0001`); giữ nguyên cấu trúc (anchor `<<: *go-defaults`, `${POSTGRES_PASSWORD:?msg}`, DSN có `${…}` trong userinfo, comment chứa IP, `command: ["-js"]`). Không chép `.env`. `PROVENANCE.txt` ghi từng tệp fixture lấy từ đường dẫn thật nào, dòng nào, và các chỗ đã che.
6. Tạo dưới `testdata/storage/services/` cây tối thiểu cho 4 service mẫu (`infra-fleet-service`, `task-service`, `git-gateway-service`, `mcp-service`): `internal/config/config.go` rút gọn, danh sách thư mục `internal/adapter/*` (tệp `.keep` hoặc `doc.go`), vài tệp `.go` chứa literal subject (trong `const`, `SubjectBinding{}`, comment). Tệp Go fixture phải biên dịch được hoặc ít nhất parse được bằng `go/parser`.

## Kiểm thử

- Không có test Go ở task này. Kiểm tra thủ công: `grep -rn "CANARY_" services/code-intel-service/testdata/storage | wc -l` > 0 và `grep -rEn "dev-root-token|POSTGRES_PASSWORD: orca$" services/code-intel-service/testdata/storage` rỗng (không còn literal thật).
- Kiểm tra `go vet` không áp dụng; fixture nằm trong `testdata` nên Go bỏ qua khi build.

## Tiêu chí hoàn thành

- [ ] Bảng đối chiếu số liệu trong PR; mọi chỗ lệch với CR được ghi.
- [ ] Bốn khả năng của `RepoSourceReader` có nhãn rõ ràng.
- [ ] Fixture không chứa giá trị thật nào; `PROVENANCE.txt` đủ để người khác tái tạo.
- [ ] Danh sách service không theo mẫu config được ghi lại.

## Rủi ro và lưu ý

- Fixture chụp từ repo sẽ lỗi thời; `PROVENANCE.txt` cho phép cập nhật có kiểm soát (nightly golden, TASK-035-08).
- Đừng đọc `deploy/dev/.env`: dù tồn tại cục bộ, đọc nó vi phạm chính quy tắc của solution; chỉ `.env.example` (tên khoá).
