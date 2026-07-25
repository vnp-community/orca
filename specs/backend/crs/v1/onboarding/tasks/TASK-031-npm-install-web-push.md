# TASK-031: Cài đặt npm package `web-push`

**Phase:** 3 — Web Push Notifications  
**Solution:** [SOL-007-008-009](../solutions/SOL-007-008-009-windows-notifications-checklist.md) §B checklist  
**Depends on:** (không)  
**Blocks:** TASK-032

---

## Mục tiêu

Cài package `web-push` và type definitions, đảm bảo import hoạt động trong TypeScript.

---

## Lệnh cần chạy

```bash
# Từ thư mục root của project (nơi chứa package.json của orca):
npm install web-push
npm install --save-dev @types/web-push
```

---

## Acceptance Criteria

- [x] `web-push` có trong `dependencies` của `package.json`
- [x] `@types/web-push` có trong `devDependencies`
- [x] `import webPush from 'web-push'` compile thành công (TypeScript)
- [x] `webPush.generateVAPIDKeys()` available theo types

---

## Lưu ý cho AI

1. Xác định đúng `package.json` cần thêm — trong monorepo, có thể là `orca/package.json`
2. Chạy lệnh từ đúng working directory
3. Verify: `node -e "require('web-push')"` không báo lỗi
