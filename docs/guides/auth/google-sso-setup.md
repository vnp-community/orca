# Cấu hình đăng nhập SSO qua Google (backend-go, CR-LOGIN-001)

> **Phạm vi tài liệu này:** hướng dẫn vận hành (setup) để bật đăng nhập Google
> cho deploy **backend-go** (`deploy/dev/`) — đây là hệ thống backend hiện đang
> sống/triển khai thật (`b15.openledger.vn`), khác với backend Node/TS cũ mà
> [`authentication-login-process-methods.md`](./authentication-login-process-methods.md)
> mô tả (tài liệu đó liệt kê SSO là "⏳ Deferred Phase 2, endpoint trả 501" —
> đúng cho backend TS cũ, **không còn đúng** cho backend-go kể từ khi
> CR-LOGIN-001 được triển khai thật trong `backend-go/services/auth-service`
> và `backend-go/services/api-gateway`). Muốn hiểu code implementation chi
> tiết (usecase, account-linking policy, security invariant), đọc
> [`backend-go/services/auth-service/README.md`](../../../backend-go/services/auth-service/README.md)'s
> mục SSO.

Domain đang dùng cho deploy này: **`https://b15.openledger.vn`**. Mọi lệnh/giá
trị dưới đây dùng đúng domain này — nếu bạn deploy ở domain khác, thay thế
tương ứng.

---

## 1. Tổng quan luồng

```
Browser                    api-gateway                  auth-service              Google
  │  click "Continue with     │                              │                        │
  │  Google" (SsoButton.tsx)  │                              │                        │
  │─ GET /auth/sso/google ───►│                              │                        │
  │                           │─ StartSsoLogin(provider) ───►│                        │
  │                           │                              │ build authorize URL   │
  │                           │                              │ (PKCE + signed state)  │
  │                           │◄──── authorization_url ───────│                        │
  │◄── 302 redirect ──────────│                              │                        │
  │───────────────────────────────────────────────────────────────────────────────────►│
  │                                              (user đăng nhập + đồng ý trên Google)   │
  │◄──────────────── 302 redirect → https://b15.openledger.vn/auth/callback?code=...────│
  │─ GET /auth/callback?code=…&state=… ─────────►│                              │
  │                           │─ CompleteSsoLogin(code, state) ────────────────►│
  │                           │                              │ verify state, exchange │
  │                           │                              │ code, call Google      │
  │                           │                              │ userinfo endpoint      │
  │                           │◄──── session_token + user ────│                        │
  │◄── Set-Cookie: orca_session; redirect → "/" ─│                              │
```

Mã nguồn liên quan (không cần đọc để làm theo hướng dẫn này, chỉ để tham
khảo khi cần debug sâu):
- `backend-go/services/api-gateway/internal/adapter/httpgateway/auth_routes.go` — `GET /auth/sso/{provider}`, `GET /auth/callback`, `GET /auth/config`.
- `backend-go/services/auth-service/internal/adapter/oauth/oidc.go` — Google dùng chung client OIDC generic này (endpoint Google là hằng số cố định, không cần discovery URL).
- `backend-go/services/auth-service/internal/usecase/login_or_provision_sso_user.go` — chính sách chống trùng tài khoản (xem mục 6 bên dưới).

---

## 2. Tạo Google OAuth 2.0 Client

### 2.1 Tạo/chọn Google Cloud Project

1. Vào [console.cloud.google.com](https://console.cloud.google.com/).
2. Tạo project mới (hoặc dùng project có sẵn của tổ chức) — ví dụ tên `orca-sso`.

### 2.2 Cấu hình OAuth consent screen

Vào **APIs & Services → OAuth consent screen**:

1. **User Type:**
   - Nếu toàn bộ user đăng nhập đều thuộc Google Workspace của tổ chức bạn → chọn **Internal** (không cần Google review, chỉ user trong domain Workspace mới đăng nhập được).
   - Nếu user dùng Gmail cá nhân bất kỳ → chọn **External**. Ở trạng thái **Testing**, chỉ các email được thêm vào mục **Test users** mới đăng nhập được — muốn mở cho tất cả Gmail thì phải submit Google verify app (không bắt buộc nếu chỉ dùng nội bộ, cứ giữ ở Testing + thêm test users là đủ cho môi trường b15.openledger.vn hiện tại).
2. **App information:** điền App name (ví dụ "Orca"), User support email, Developer contact email.
3. **Scopes:** thêm 3 scope sau (bắt buộc — đây chính xác là scope `internal/adapter/oauth/oidc.go`'s `defaultOidcScope` yêu cầu):
   - `openid`
   - `.../auth/userinfo.email`
   - `.../auth/userinfo.profile`
4. Lưu lại.

### 2.3 Tạo OAuth Client ID

Vào **APIs & Services → Credentials → Create Credentials → OAuth client ID**:

1. **Application type:** `Web application`.
2. **Name:** ví dụ `orca-b15-web`.
3. **Authorized JavaScript origins:** thêm
   ```
   https://b15.openledger.vn
   ```
4. **Authorized redirect URIs** — **quan trọng, phải khớp byte-for-byte** với
   những gì `api-gateway` tự build (`PUBLIC_BASE_URL + "/auth/callback"`, xem
   `auth_routes.go`'s `handleSsoStart` — cố tình **không bao giờ** suy ra từ
   header `Host` của request, để chống tấn công redirect_uri):
   ```
   https://b15.openledger.vn/auth/callback
   ```
5. Bấm **Create** → Google hiện **Client ID** và **Client secret**. Copy lại
   2 giá trị này — sẽ dùng ở bước 3.

---

## 3. Cấu hình `.env` cho deploy

File thao tác: `deploy/dev/.env` (copy từ `.env.example` nếu chưa có — **không**
commit file `.env` thật, đã có trong `.gitignore`).

```bash
# ── Public base URL (bắt buộc để SSO hoạt động) ──
PUBLIC_BASE_URL=https://b15.openledger.vn

# ── Auth mode ── "both" = hiện cả local password lẫn nút SSO (mặc định, an toàn)
AUTH_MODE=both

# ── Google OAuth (từ bước 2.3) ──
SSO_GOOGLE_CLIENT_ID=<Client ID vừa tạo>
SSO_GOOGLE_CLIENT_SECRET=<Client secret vừa tạo>

# ── HMAC key ký state token (PKCE flow, auth-service) — bắt buộc set giá trị
# ngẫu nhiên dài cho production; để trống thì auth-service vẫn chạy nhưng
# state token có thể bị giả mạo (không an toàn để dùng thật) ──
SSO_STATE_SECRET=<chuỗi ngẫu nhiên dài, ví dụ: openssl rand -hex 32>
```

Không cần set `SSO_GITHUB_*`/`SSO_OIDC_*` nếu chỉ dùng Google — để trống thì
2 provider đó tự động không hiện trên trang login (`GET /auth/config` chỉ
liệt kê provider có `CLIENT_ID` khác rỗng, xem `auth_routes.go`).

> `PUBLIC_BASE_URL`/`AUTH_MODE`/`SSO_GOOGLE_CLIENT_ID` cần set **cả ở
> `api-gateway`** (build redirect_uri + hiện nút SSO) **lẫn `auth-service`**
> (`SSO_GOOGLE_CLIENT_ID/SECRET`, thực hiện exchange thật với Google) —
> `deploy/dev/docker-compose.yml` đã tự động đọc đúng các biến này cho cả
> hai service từ cùng file `.env`, bạn chỉ cần set một lần.

---

## 4. Deploy

### 4.1 Nếu deploy qua SSH lên server (cách chuẩn cho b15.openledger.vn)

```bash
cd /path/to/orca
nano deploy/dev/.env     # đã set ở bước 3 chưa quên SERVER_HOST/SERVER_KEY
bash deploy/dev/scripts/sync-to-server.sh 0.1.1
```

Lệnh này tự build binary, sync lên server, **chạy migration mới**
(migration `0003_sso_identities` — tạo bảng `auth.sso_identities` + cột
`auth.users.sso_provider`), rồi restart toàn bộ container với env mới.

### 4.2 Nếu chỉ redeploy lại container (không có thay đổi binary)

```bash
cd deploy/dev
docker compose up -d --force-recreate api-gateway auth-service
```

### 4.3 Kiểm tra migration đã chạy

```bash
docker compose exec postgres psql -U orca -d auth -c "\d auth.sso_identities"
```

Nếu bảng chưa tồn tại, chạy migration thủ công:

```bash
bash deploy/dev/scripts/migrate.sh
```

---

## 5. Kiểm tra hoạt động (verify)

1. Mở `https://b15.openledger.vn/` (ẩn danh/cửa sổ mới để tránh cache session cũ).
2. Trang login phải hiện nút **"Continue with Google"** bên cạnh form
   email/password (nếu không hiện → xem mục Troubleshooting bên dưới, đây
   là dấu hiệu `GET /auth/config` chưa thấy `SSO_GOOGLE_CLIENT_ID`).
3. Bấm nút → phải redirect sang trang đăng nhập Google (đúng domain
   `accounts.google.com`).
4. Đăng nhập + đồng ý cấp quyền → phải redirect ngược về
   `https://b15.openledger.vn/` và **vào thẳng app** (không phải trang lỗi).
5. Mở DevTools → Application → Cookies → phải thấy cookie `orca_session`
   với cờ `HttpOnly`, `Secure`, `SameSite=Strict`.
6. Vì đây là **user hoàn toàn mới** trong hệ thống, ngay sau khi vào app sẽ
   hiện overlay **"Chọn phòng ban"** (`DepartmentGate`, CR-DS-008) — đây là
   hành vi **đúng như thiết kế**, không phải lỗi. Chọn phòng ban → nếu chưa
   có quyền truy cập dev server nào, tiếp tục hiện form **yêu cầu quyền
   truy cập** (access request) — admin duyệt trong **Settings → Admin →
   Dev Servers → Access Requests** (`AdminDevServerConsole.tsx`).
7. Đăng xuất, đăng nhập lại bằng đúng tài khoản Google đó → lần này phải
   vào thẳng app, **không** hiện lại `DepartmentGate` (đã có department từ
   lần trước) — xác nhận identity được nhận diện là "returning", không tạo
   user trùng.

---

## 6. Lưu ý bảo mật quan trọng — email phải verified

`auth-service` **từ chối thẳng** mọi lần đăng nhập SSO mà Google báo
`email_verified: false` (rất hiếm với Google — hầu như luôn `true` — nhưng
vẫn được kiểm tra tường minh), **kể cả khi đây là lần tạo tài khoản đầu
tiên**, không chỉ khi trùng với tài khoản local có sẵn. Đây là fix chủ đích
cho một lỗ hổng chiếm đoạt tài khoản: nếu cho phép tạo tài khoản bằng email
chưa xác thực, kẻ tấn công có thể "giữ chỗ" trước email của nạn nhân, khiến
lần đăng nhập thật (đã verified) sau này của nạn nhân bị tự động gộp vào
tài khoản kẻ tấn công đã tạo trước đó.

→ Với Google thì gần như không bao giờ gặp lỗi này (Google luôn verify
email trước khi cấp OAuth token), nhưng nếu gặp lỗi
`AUTH_SSO_EMAIL_NOT_VERIFIED` hoặc `AUTH_SSO_EMAIL_UNVERIFIED_COLLISION`,
đây **không phải bug** — không có "UI admin để bypass" theo thiết kế (xem
`backend-go/services/auth-service/README.md`'s mục SSO để hiểu đầy đủ lý
do). Người dùng cần xác minh lại email đó thẳng trong Google Account của
họ.

---

## 7. Giới hạn hiện tại — single-tenant

Một user SSO **hoàn toàn mới** (chưa có `sso_identities` row, chưa có local
account trùng email) sẽ được auto-tạo tài khoản, nhưng tenant được gán tự
động **chỉ hoạt động khi deployment này có đúng 1 company/tenant**
(`TenantResolver.ResolveDefaultTenant` gọi `tenant-service.ListCompanies` —
nếu có 0 hoặc >1 company, đăng nhập bị từ chối với lỗi
`AUTH_SSO_AMBIGUOUS_TENANT`). Với deploy `b15.openledger.vn` hiện tại (single
tenant), điều này không phải vấn đề — chỉ ghi chú lại nếu sau này deployment
này phục vụ nhiều tổ chức (multi-tenant) cùng lúc.

---

## 8. Troubleshooting

| Triệu chứng | Nguyên nhân khả dĩ | Cách kiểm tra/sửa |
|---|---|---|
| Nút "Continue with Google" không hiện trên trang login | `SSO_GOOGLE_CLIENT_ID` rỗng hoặc chưa restart container | `curl https://b15.openledger.vn/auth/callback... ` không áp dụng — thay vào đó: `curl https://b15.openledger.vn/auth/config`, kiểm tra `"providers"` có `"google"` không |
| Bấm nút → lỗi `501 AUTH_SSO_NOT_CONFIGURED` | `PUBLIC_BASE_URL` rỗng | Set `PUBLIC_BASE_URL=https://b15.openledger.vn` trong `.env`, redeploy |
| Google báo lỗi `redirect_uri_mismatch` | Redirect URI đăng ký trên Google Cloud Console không khớp | Vào lại bước 2.3, kiểm tra đúng `https://b15.openledger.vn/auth/callback` (không có dấu `/` thừa cuối, đúng scheme `https`) |
| Google báo `Error 400: invalid_client` | Sai `SSO_GOOGLE_CLIENT_ID`/`SECRET`, hoặc client bị xoá | Kiểm tra lại giá trị trong `.env` khớp với Google Cloud Console |
| Đăng nhập xong quay về `/?ssoError=1` | `CompleteSsoLogin` lỗi phía `auth-service` — state hết hạn (>15 phút), hoặc `SSO_STATE_SECRET` khác nhau giữa các lần deploy | Xem log: `docker compose logs auth-service --tail=100`; thử lại từ đầu (đừng để tab login mở quá lâu trước khi bấm) |
| `AUTH_SSO_AMBIGUOUS_TENANT` | Deployment có 0 hoặc nhiều hơn 1 company trong `tenant-service` | Kiểm tra `docker compose exec postgres psql -U orca -d tenant -c "SELECT id, name FROM tenant.companies"` |
| Google Workspace user báo "app chưa xác minh" (unverified app warning) | Consent screen đang ở **External + Testing** và email đó chưa được thêm vào **Test users** | Thêm email vào Test users (mục 2.2), hoặc chuyển User Type sang **Internal** nếu cùng Google Workspace |

---

## 9. Tài liệu liên quan

| File | Nội dung |
|---|---|
| [`backend-go/services/auth-service/README.md`](../../../backend-go/services/auth-service/README.md) | Chi tiết implementation: account-linking policy, PKCE, state token, known gaps |
| [`deploy/dev/.env.example`](../../../deploy/dev/.env.example) | Toàn bộ biến môi trường, giải thích từng biến |
| [`deploy/dev/README.md`](../../../deploy/dev/README.md) | Quick start deploy, "Known limitations" mục SSO |
| [`docs/crs/v1/login/CR-LOGIN-001-auth.md`](../../crs/v1/login/CR-LOGIN-001-auth.md) | Change request gốc — thiết kế SSO ban đầu (lưu ý: mô tả backend TS cũ, phần OAuth/OIDC nay đã triển khai thật trong backend-go) |
| [`authentication-login-process-methods.md`](./authentication-login-process-methods.md) | Tổng quan mọi cơ chế auth của Orca (PairCode, Local Login, SSO...) — tài liệu về backend TS cũ |
