# TDD-FE-03: Admin SPA

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/renderer/src/components/admin/`, `src/renderer/src/admin/`

---

## 1. Entry Point

```
src/renderer/admin-index.html
  → admin-main.tsx
      → AdminApp.tsx
          → React Router DOM
```

---

## 2. AdminApp Structure

```tsx
// src/renderer/src/components/admin/AdminApp.tsx

function AdminApp() {
  return (
    <BrowserRouter basename="/admin">
      <AdminNav />  {/* sidebar navigation */}
      <Routes>
        <Route path="/"         element={<AdminDashboard />} />
        <Route path="/users"    element={<UsersPage />} />
        <Route path="/sessions" element={<SessionsPage />} />
        <Route path="/policies" element={<PoliciesPage />} />
        <Route path="/audit"    element={<AuditPage />} />
      </Routes>
    </BrowserRouter>
  )
}
```

---

## 3. Admin API Client

```typescript
// src/renderer/src/components/admin/admin-api-client.ts

// Base: fetch('/admin/api/', { credentials: 'include' })

async function getStats(): Promise<AdminStats>
async function listUsers(): Promise<OrcaUser[]>
async function createUser(input: CreateUserInput): Promise<OrcaUser>
async function deactivateUser(userId: string): Promise<void>
async function listSessions(): Promise<OrcaSession[]>
async function killSession(sessionId: string): Promise<void>
async function killUserSessions(userId: string): Promise<void>
async function getAuditLog(params: AuditQueryParams): Promise<AuditPage>
```

---

## 4. AdminDashboard

```tsx
function AdminDashboard() {
  // Stats panel:
  //   - Total users
  //   - Active users (24h)
  //   - Active sessions
  //   - System version + uptime

  // Active sessions list (recent 10)
  // Quick links tới Users, Audit
}
```

---

## 5. UsersPage + UserForm

```tsx
function UsersPage() {
  // Table: email, name, role, provider, status (active/inactive)
  // Actions per row: Deactivate, Kill sessions
  // "Add User" button → UserForm modal (email/password/role)
}

function UserForm({ onSubmit, onCancel }) {
  // Fields: email, name, password, role (admin|user|viewer)
  // Validation: email format, password length >= 8
}
```

---

## 6. SessionsPage

```tsx
function SessionsPage() {
  // Table: user email, IP, user agent, created_at, last_seen_at
  // Actions: Kill session, Kill all sessions for user
  // Auto-refresh: 30s interval
}
```

---

## 7. PoliciesPage + PolicyForm

```tsx
function PoliciesPage() {
  // Table: name, effect (allow/deny), resource, action
  // CRUD: Create, Edit, Delete policies
}

function PolicyForm() {
  // effect: 'allow' | 'deny'
  // resource: string (e.g., 'dev-server:*', 'admin:users')
  // action: string (e.g., 'connect', 'read', 'write')
  // condition?: JSON object
}
```

---

## 8. AuditPage

```tsx
function AuditPage() {
  // Filters: userId, action, date range
  // Table: timestamp, user email, action, detail, IP
  // Pagination: 50 per page
  // Export: CSV download
}
```

---

## 9. Auth Guard (Admin SPA)

```typescript
// Admin SPA tự kiểm tra session khi load:
// GET /auth/me → nếu không phải admin → redirect to /
// Không dùng App.tsx auth flow — Admin SPA hoàn toàn độc lập
```

---

## 10. Tests (18 tests)

| File | Tests |
|------|-------|
| `admin-api-client.test.ts` | 7 |
| `AdminDashboard.test.tsx` | 3 |
| `UsersPage.test.tsx` | 5 |
| `SessionsPage.test.tsx` | 3 |
