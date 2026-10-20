## Why

Người dùng hiện không thể đăng nhập bằng tài khoản của công ty: biểu mẫu đăng nhập chỉ nhận email và mật khẩu cục bộ. Thay đổi này thêm đăng nhập bằng tài khoản công ty và khoá tài khoản sau nhiều lần nhập sai, để đáp ứng AC-1 và AC-2.

## What Changes

- Thêm đường đăng nhập bằng tài khoản công ty cạnh đường email và mật khẩu hiện có.
- Khoá tài khoản trong mười lăm phút sau năm lần nhập sai liên tiếp.
- Không đổi định dạng token phiên và không đổi màn hình đăng ký.

## Capabilities

### New Capabilities

- `dang-nhap`: đăng nhập bằng tài khoản công ty và khoá tài khoản khi nhập sai nhiều lần.

## Impact

- Dịch vụ `auth-service`: thêm bộ xử lý đăng nhập mới và bộ đếm lần nhập sai.
- Không có thay đổi dữ liệu không tương thích ngược.
