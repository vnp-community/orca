# TASK-REQ-046-06: Cấu hình triển khai và tài liệu cho `REQUEST_DEFAULT_OUTPUT_LANGUAGE`

**From Solution:** [BE-REQ-SOL-046](../solutions/BE-REQ-SOL-046-ai-output-language.md)
**Priority:** P1
**Service:** `deploy/dev`, `docs`
**File:** `deploy/dev/docker-compose.yml` (sửa, khối `request-service` quanh dòng 463-475), `deploy/dev/.env.example` (sửa, khối "Request defaults" quanh dòng 221-228), `deploy/dev/README.md` (sửa), `deploy/dev/RELEASE-2026-10-DEPLOY.md` (sửa mục 4.1) hoặc tài liệu phát hành của bản kế tiếp nếu bản 2026-10 đã phát hành, `docs/guides/request/admin-enable-request-flow.md` (thêm một mục ngắn), `docs/guides/request/knowledge-hub-setup.md` (một dòng về kỹ năng)
**Depends on:** TASK-REQ-046-02
**Status:** [x] DONE (2026-10-20): hai kiểm thủ công (a) và (b) đã chạy trên stack

---

## Context

- `request-service` chưa đọc biến này (task 02 làm). Họ biến cùng loại ở compose: `REQUEST_DEFAULT_ENGINE`, `REQUEST_DEFAULT_SOLUTION_AGENT`, `REQUEST_DEFAULT_EXECUTION_MODE` (`docker-compose.yml:470-473`), `.env.example:225-228`; bảng biến ở `RELEASE-2026-10-DEPLOY.md` mục 4.1 viết rằng giá trị sai "không làm service chết". Biến mới là **ngoại lệ có chủ ý**: giá trị sai làm `request-service` không khởi động.
- Đổi biến cần `redeploy-local.sh request-service`; giá trị đọc một lần lúc khởi động.
- Kỹ năng OpenSpec cài từ nhánh mặc định bằng `npx skills` (`knowledge-hub-setup.md:19-24`); mục Language mới ở `SKILL.md` chỉ có hiệu lực sau khi cập nhật trên dev server.

## Việc cần làm

1. `docker-compose.yml`: dưới `REQUEST_DEFAULT_EXECUTION_MODE` thêm `REQUEST_DEFAULT_OUTPUT_LANGUAGE: ${REQUEST_DEFAULT_OUTPUT_LANGUAGE:-vi}` kèm chú thích một đến hai dòng: ngôn ngữ AI viết phân tích và Plan (`vi` hoặc `en`), áp cho cả hệ thống, giá trị khác làm service không khởi động.
2. `.env.example`: thêm `REQUEST_DEFAULT_OUTPUT_LANGUAGE=vi` trong khối "Request defaults", chú thích tương tự.
3. `deploy/dev/README.md`: một đoạn trong phần "Luồng Request": biến, giá trị hợp lệ, mặc định `vi`, hiệu lực sau `redeploy-local.sh request-service`, và lỗi khởi động mẫu (`REQUEST_DEFAULT_OUTPUT_LANGUAGE "fr" is not supported; use one of: vi, en`).
4. Tài liệu phát hành: thêm hàng vào bảng 4.1 (`vi`/`en`, mặc định `vi`) và sửa câu "Giá trị sai không làm service chết" thành nêu rõ ngoại lệ này. Ghi **tác động**: sau lần deploy, phân tích và Plan mới của mọi tenant ra tiếng Việt, cái đã tạo giữ nguyên; muốn tiếng Anh đặt `en` (với `en` chỉ thị tiếng Anh vẫn được gửi, ép tiếng Anh ngay cả khi Request bằng tiếng Việt); phân tích tiếng Việt dưới giao diện tiếng Anh trộn hai ngôn ngữ vì giao diện chưa có locale `vi`; kỹ năng `tdd-openspec-author` cần `npx skills update`. Cách quay lại: đặt `en`, `redeploy-local.sh request-service`.
5. `admin-enable-request-flow.md`: mục ngắn "Ngôn ngữ đầu ra của AI" (biến, hợp lệ, mặc định, nằm ngoài cài đặt theo dự án, hạn chế: không theo ngôn ngữ giao diện, không phát hiện ngôn ngữ của Request). `knowledge-hub-setup.md`: một dòng cạnh lệnh `npx skills update` nhắc cập nhật `tdd-openspec-author` sau khi hợp nhất mục Language.
6. Kiểm thủ công trên stack local (ghi vào PR): (a) `redeploy-local.sh request-service` không đặt biến → log khởi động có `output language code=vi`; (b) đặt `REQUEST_DEFAULT_OUTPUT_LANGUAGE=fr` → container dừng với thông báo rõ và `docker compose logs request-service` hiện đúng thông báo; trả lại `vi`. **Không** chạy bước này trên stack của người khác mà không báo.

## Kiểm thử

- `docker compose -f deploy/dev/docker-compose.yml config` hợp lệ và hiện biến với mặc định `vi`.
- Đọc lại từng tài liệu: không còn câu nào nói "mọi `REQUEST_DEFAULT_*` đều không làm service chết".
- Liên kết trong tài liệu đã sửa trỏ đúng.

## Tiêu chí hoàn thành

- [x] Compose, `.env.example`, README, tài liệu phát hành, hướng dẫn quản trị nêu biến, giá trị, mặc định, hiệu lực và ngoại lệ "sai thì dừng". Bằng chứng ở "Ghi chú triển khai": cả năm tài liệu nêu biến, giá trị, mặc định, hiệu lực sau `redeploy-local.sh request-service` và ngoại lệ "sai thì dừng".
- [x] Ghi chú phát hành nêu tác động lên tenant hiện có và cách quay lại. Mục "Tác động" ở `RELEASE-2026-10-DEPLOY.md` 4.1: tenant hiện có, `en` vẫn ép tiếng Anh, giao diện chưa có locale `vi`, kỹ năng cần `npx skills update`, cách quay lại.
- [x] Hai kiểm thủ công (a) và (b) có kết quả trong PR. (a) `docker logs orca-go-request | grep '"msg":"output language"'`: `{"level":"INFO","msg":"output language","service":"request-service","code":"vi"}` ngay sau lần khởi động hiện tại (container `StartedAt` 05:19:56Z, dòng 05:19:57Z; không đặt biến nên là mặc định compose `vi`). (b) container dùng một lần: `docker compose -f docker-compose.yml -f docker-compose.local.yml run --rm --no-deps -T -e REQUEST_DEFAULT_OUTPUT_LANGUAGE=fr --name ara-olg-throwaway request-service`: thoát mã 1 với đúng một dòng `error: REQUEST_DEFAULT_OUTPUT_LANGUAGE "fr" is not supported; use one of: vi, en`, container `--rm` không còn, `orca-go-request` thật không bị đụng tới và vẫn `healthy` (stack không bao giờ ở giá trị sai). `docker compose logs request-service` của dịch vụ thật không có dòng này vì phép thử chạy ở container riêng.

## Rủi ro và lưu ý

- Đổi mặc định của mọi triển khai sang tiếng Việt là có chủ ý (quyết định của người dùng); không thêm cờ "giữ hành vi cũ" ngoài `en`.
- `RELEASE-2026-10-DEPLOY.md` là tài liệu của một bản phát hành cụ thể: nếu bản đó đã phát hành, thêm vào tài liệu phát hành kế tiếp thay vì sửa lịch sử.

## Ghi chú triển khai

**Tệp:** `deploy/dev/docker-compose.yml` (khối `request-service`, ngay dưới `REQUEST_DEFAULT_EXECUTION_MODE`), `deploy/dev/.env.example`, `deploy/dev/README.md` (mục mới "Ngôn ngữ đầu ra của AI" trong "Luồng Request trên stack local"), `deploy/dev/RELEASE-2026-10-DEPLOY.md` (hàng mới ở mục 1 và bảng 4.1, câu "giá trị sai không làm service chết" nay chỉ nói ba biến cũ và nêu ngoại lệ, đoạn "Tác động", dòng quay lại ở mục 7), `docs/guides/request/admin-enable-request-flow.md` (mục "Ngôn ngữ đầu ra của AI"), `docs/guides/request/knowledge-hub-setup.md` (một câu cạnh `npx skills update`).

**Lệnh và kết quả:** `docker compose -f deploy/dev/docker-compose.yml config` hợp lệ và hiện `REQUEST_DEFAULT_OUTPUT_LANGUAGE: vi`; `grep` toàn `deploy/` và `docs/` không còn câu nói chung "mọi `REQUEST_DEFAULT_*` đều không làm service chết" (chỉ còn câu giới hạn ở ba biến cũ). Liên kết tương đối mới (`../../../deploy/dev/README.md` từ `docs/guides/request/`) cùng mẫu với các liên kết sẵn có trong tệp đó.

**Quyết định:** bản 2026-10 chưa có tag/phát hành (chỉ một commit tài liệu và nhánh đang phát triển), nên thêm vào `RELEASE-2026-10-DEPLOY.md` thay vì mở tài liệu bản kế tiếp. Nếu bản đó đã phát hành thật thì chuyển các đoạn này sang tài liệu bản sau.

**Chưa chạy (cần stack, do bên điều phối):** (a) `redeploy-local.sh request-service` không đặt biến: log khởi động có `output language code=vi`; (b) đặt `REQUEST_DEFAULT_OUTPUT_LANGUAGE=fr`: container thoát với thông báo đúng và `docker compose logs request-service` hiện nó, rồi trả lại `vi`. Phần mã của (b) đã có test đơn vị (`TestRun_UnsupportedOutputLanguageStopsBeforeAnythingIsOpened`).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
