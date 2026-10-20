Chỉ vùng được rào bên dưới mới được bộ phân tích chặt của Orca đọc; phần còn lại dành cho người đọc.

<!-- orca:begin plan request=REQ-901 schema=1 -->
## PH-1 Đăng nhập bằng tài khoản công ty
> Giao đường đăng nhập mới: người dùng có tài khoản công ty vào được hệ thống. Xong khi kiểm thử của riêng nó đạt, trước khi phần khoá tài khoản phụ thuộc vào nó.
- [ ] T1.1 Thêm bộ xử lý đăng nhập bằng tài khoản công ty [type=feature] [h=3] [ac=AC-1]
  Serves: FR-AUTH-02
  Code: create `services/auth-service/internal/handler/company_login.go::CompanyLoginHandler`
  Verify: AC-1 đăng nhập thành công qua `cd services/auth-service && go test ./internal/handler/... -run TestCompanyLogin`
  Done when: một tài khoản công ty hợp lệ nhận được phiên và lần đăng nhập được ghi vào nhật ký kiểm toán.
- [ ] T1.2 Chạy bộ kiểm thử hồi quy của đăng nhập [type=task] [h=1.5] [ac=AC-1] [labels=test:regression] [depends=T1.1]
  Serves: FR-AUTH-02
  Code: create `services/auth-service/internal/handler/company_login_test.go::TestCompanyLoginRegression`
  Verify: AC-1 qua `cd services/auth-service && go test ./internal/handler/...`
  Done when: `go test ./internal/handler/...` đạt, kể cả ca tài khoản công ty không tồn tại.

## PH-2 Khoá tài khoản khi nhập sai
> Giao bộ đếm lần nhập sai và việc khoá mười lăm phút. Xong khi mọi kịch bản của AC-2 đạt ở mức tuyến đường.
- [ ] T2.1 Thêm bộ đếm lần nhập sai mật khẩu [type=feature] [h=2.5] [ac=AC-2]
  Serves: FR-AUTH-05
  Code: modify `services/auth-service/internal/handler/company_login.go::CompanyLoginHandler` (đếm lần nhập sai theo tài khoản)
  Verify: AC-2 khoá sau năm lần qua `cd services/auth-service && go test ./internal/handler/... -run TestLoginLockout`
  Done when: lần nhập sai thứ năm liên tiếp đặt trạng thái khoá và lần thử thứ sáu bị từ chối.
- [ ] T2.2 Cập nhật tài liệu SRS cho FR-AUTH-05 [type=task] [h=1] [ac=AC-2] [depends=T2.1]
  Serves: FR-AUTH-05
  Code: modify `docs/SRS.md` (dòng FR-AUTH-05)
  Verify: rà soát thủ công, mô tả khoá tài khoản khớp hành vi mới
  Done when: SRS mô tả khoá mười lăm phút sau năm lần nhập sai.
<!-- orca:end plan -->
