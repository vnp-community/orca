## ADDED Requirements

### Requirement: Đăng nhập bằng tài khoản công ty
Hệ thống SHALL cho phép người dùng đăng nhập bằng tài khoản công ty và SHALL trả về một phiên hợp lệ khi thông tin xác thực đúng.

#### Scenario: AC-1 Đăng nhập thành công bằng tài khoản công ty
- **WHEN** người dùng gửi tài khoản công ty hợp lệ tới màn hình đăng nhập
- **THEN** hệ thống tạo phiên và đưa người dùng tới trang chủ
- **AND** lần đăng nhập được ghi vào nhật ký kiểm toán

### Requirement: Khoá tài khoản khi nhập sai nhiều lần
Hệ thống MUST khoá tài khoản trong mười lăm phút sau năm lần nhập sai mật khẩu liên tiếp.

#### Scenario: AC-2 Tài khoản bị khoá sau năm lần nhập sai
- **WHEN** người dùng nhập sai mật khẩu năm lần liên tiếp
- **THEN** lần thử thứ sáu bị từ chối với thông báo tài khoản đang bị khoá
- **AND** không có phiên nào được tạo
