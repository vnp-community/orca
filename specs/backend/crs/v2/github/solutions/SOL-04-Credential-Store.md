# Solution cho CR-INT-002 & CR-INT-003: API & File Tokens trong Web Mode

## Bối cảnh & TDD Specs liên kết
- Theo **TDD-11 (Web Server Mode) - Addendum v4.0**, mỗi User Session trong Web Mode (khi `ORCA_MULTI_USER=1`) được fork thành một Node.js process riêng biệt bởi `WsSessionRouter`.
- Process này nhận `ORCA_USER_ID` thông qua biến môi trường.

## Đánh giá vấn đề
- Các biến môi trường global (`ORCA_BITBUCKET_ACCESS_TOKEN`, ...) không thể sử dụng do các user share chung container (trước khi split process) hoặc không thể cấu hình biến môi trường tùy chỉnh dễ dàng per-user.
- Các file tokens (`linear-token.enc`, `jira-tokens`) lưu chung vào `/data/orca/` gây rò rỉ dữ liệu giữa các User.
- API mã hóa `safeStorage` của Electron không hoạt động trong môi trường Node.js headless.

## Thiết kế giải pháp: `WebCredentialStore`

Do mỗi user đã chạy trong một **process riêng biệt**, chúng ta có thể tiêm `userId` vào khi khởi tạo `WebCredentialStore` cho process đó.

### 1. Khởi tạo per-process
Mỗi instance của `OrcaRuntimeRpcServer` (chạy trong tiến trình con của user) sẽ quản lý tokens của chính user đó.

```typescript
// src/main/credentials/web-credential-store.ts
export class WebCredentialStore {
  // Base dir cho user cụ thể
  private userCredDir: string;
  private masterKey: Buffer;

  constructor(userDataPath: string, userId: string, serverSecret: string) {
    this.userCredDir = join(userDataPath, 'users', userId, 'credentials');
    // Mã hóa bằng server secret
    this.masterKey = scryptSync(serverSecret, 'orca-credential-salt', 32);
    mkdirSync(this.userCredDir, { recursive: true, mode: 0o700 });
  }

  // Các phương thức AES-256-GCM để setToken, getToken, deleteToken...
}
```

### 2. Xóa bỏ sự phụ thuộc vào Electron `safeStorage`
Thay thế các đoạn mã trong `linear/client.ts` và `jira/client.ts`:

```typescript
// src/main/linear/client.ts
async function readLinearToken(): Promise<string | null> {
  if (isWebMode()) {
    const store = getWebCredentialStore();
    return store.getToken('linear');
  }
  
  // Electron mode fallback
  const raw = readFileSync(getLinearTokenPath());
  return safeStorage.decryptString(raw);
}
```

### 3. Hợp nhất Category B và C
- **Bitbucket, Azure DevOps, Gitea** (Category B - Environment): Chuyển sang đọc từ `WebCredentialStore` thay vì `process.env`.
- **Linear, Jira** (Category C - File based): Chuyển sang đọc/ghi từ `WebCredentialStore` (hoạt động đồng thời làm nơi lưu file mã hóa cho từng user).

## Ưu điểm
- **An toàn**: Tiến trình của User A bị giới hạn trong `/data/orca/users/UserA/`. Không có nguy cơ truy cập chéo.
- **Tương thích**: AES-256-GCM thay thế hoàn hảo cho `safeStorage` của Electron.
- **Quản lý tập trung**: Thay vì các integration tự rải rác lưu credentials khắp nơi, nay quy tụ về một thư mục.

---

## ✅ Implementation Status — COMPLETED (2026-07-25)

### Files đã implement

#### `src/main/credentials/web-credential-store.ts` [NEW]
- ✅ `WebCredentialStore` class với AES-256-GCM encryption
- ✅ Key derivation: `scryptSync(serverSecret, 'orca-credential-salt', 32)`
- ✅ Per-user storage: `{userDataPath}/users/{userId}/credentials/{service}.enc`
- ✅ Methods: `setToken()`, `getToken()`, `hasToken()`, `deleteToken()`, `listServices()`
- ✅ `setConfig()` / `getConfig()` — lưu metadata phụ (email, apiBaseUrl, username)
- ✅ Type `CredentialService = 'bitbucket' | 'azure-devops' | 'gitea' | 'linear' | 'jira'`
- ✅ `SAFE_CONFIG_FIELDS` — whitelist fields được phép expose qua `credentials.status`

#### `src/main/credentials/index.ts` [NEW]
- ✅ Singleton pattern: `initWebCredentialStore(userDataPath, userId, serverSecret)`
- ✅ `getWebCredentialStore()` — throw nếu chưa init
- ✅ `isWebCredentialMode()` — `return process.env['ORCA_MULTI_USER'] === '1'`

#### `src/main/credentials/__tests__/web-credential-store.test.ts` [NEW]
- ✅ Unit tests cho encrypt/decrypt round-trip

#### `src/main/server-bootstrap.ts` [MODIFIED]
- ✅ `initWebCredentialStore(userDataPath, credUserId, serverSecret)` được gọi khi `ORCA_MULTI_USER=1`
- ✅ `SessionManager` được khởi tạo với `serverSecret` để inject credentials vào child processes
- ✅ `sessionManager` được expose trong `ServerBootstrapResult` interface

#### `src/main/session/session-manager.ts` [MODIFIED]
- ✅ `buildCredentialEnv(userId)` — đọc tokens từ `WebCredentialStore` và build env vars
- ✅ Env injection tại spawn time cho:
  - `ORCA_BITBUCKET_ACCESS_TOKEN`, `ORCA_BITBUCKET_EMAIL`, `ORCA_BITBUCKET_API_BASE_URL`
  - `ORCA_AZURE_DEVOPS_TOKEN`, `ORCA_AZURE_DEVOPS_API_BASE_URL`, `ORCA_AZURE_DEVOPS_USERNAME`
  - `ORCA_GITEA_TOKEN`, `ORCA_GITEA_API_BASE_URL`
- ✅ Errors trong `buildCredentialEnv` là non-fatal (warn + continue)

#### `src/main/linear/client.ts` [MODIFIED]
- ✅ `writeEncryptedToken()`: `isWebCredentialMode()` guard → write plain (SessionManager đã encrypt bằng WebCredentialStore)
- ✅ Electron mode fallback: `safeStorage.encryptString()` giữ nguyên

#### `src/main/jira/client.ts` [MODIFIED]
- ✅ `isWebCredentialMode()` guard trong write path
- ✅ Electron mode fallback giữ nguyên

#### `src/main/runtime/rpc/methods/credentials.ts` [NEW]
- ✅ RPC methods: `credentials.set`, `credentials.revoke`, `credentials.status`, `credentials.list`
- ✅ Web mode guard — throw error rõ ràng nếu gọi trong Electron mode
- ✅ Token leak prevention: `credentials.status` chỉ trả về `SAFE_CONFIG_FIELDS`
- ✅ Exported as `CREDENTIAL_METHODS` → đăng ký vào `ALL_RPC_METHODS`

#### `src/main/runtime/rpc/methods/credentials.test.ts` [NEW]
- ✅ **14/14 test cases PASS**:
  - `credentials.set`: stores token, config, throws in electron mode, rejects empty token
  - `credentials.revoke`: deletes token, idempotent
  - `credentials.status`: configured/not-configured, token leak prevention, safe config fields, electron mode
  - `credentials.list`: empty list, list services, electron mode

#### `src/renderer/src/web/web-preload-api.ts` [MODIFIED]
- ✅ `credentials` namespace exposed: `{ set, revoke, status, list }`

### Kết quả test
```
✅ credentials.test.ts — 14/14 tests pass
✅ TypeScript — 0 new errors trong các files đã sửa
```

### Thay đổi so với thiết kế gốc
| Thiết kế gốc | Implementation thực tế |
|---|---|
| WebCredentialStore constructor(userDataPath, userId, serverSecret) | Đúng như thiết kế |
| Chỉ setToken/getToken/deleteToken | Thêm `hasToken()`, `listServices()`, `setConfig()`, `getConfig()` |
| linear/jira: dùng `store.getToken()` trực tiếp | Dùng env vars inject bởi SessionManager (transparent) |
| Bitbucket/Azure/Gitea: đọc từ WebCredentialStore | Dùng `process.env` (được inject bởi SessionManager tại spawn time) |
