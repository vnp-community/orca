# Frontend Solutions — Onboarding CR v1

Thư mục này chứa **Frontend Technical Solutions** cho từng Change Request trong [`docs/crs/v1/onboarding/`](../../../../../docs/crs/v1/onboarding/).

Mỗi solution theo chuẩn Frontend TDD của Orca (xem [`specs/frontend/tdd/`](../../../tdd/)).

---

## Trạng thái

| Solution | CR Coverage | Status | Phase |
|----------|------------|--------|-------|
| [FE-SOL-A-dev-server-ui.md](./FE-SOL-A-dev-server-ui.md) | CR-OB-002 | ✅ COMPLETED | Phase 1 |
| [FE-SOL-B-agent-wizard.md](./FE-SOL-B-agent-wizard.md) | CR-OB-003, CR-OB-004 | ✅ COMPLETED | Phase 1–2 |
| [FE-SOL-C-preflight-repo.md](./FE-SOL-C-preflight-repo.md) | CR-OB-005, CR-OB-006 | ✅ COMPLETED | Phase 2 |
| [FE-SOL-D-platform-polish.md](./FE-SOL-D-platform-polish.md) | CR-OB-007, CR-OB-008, CR-OB-009 | ✅ COMPLETED | Phase 3 |

---

## TDD References

| TDD | File | Liên quan |
|-----|------|-----------|
| TDD-FE-02 | [02-state-management.md](../../../tdd/02-state-management.md) | Zustand slices, selector pattern |
| TDD-FE-05 | [05-ui-components.md](../../../tdd/05-ui-components.md) | Component tree, App shell, lazy loading |
| TDD-FE-06 | [06-web-client.md](../../../tdd/06-web-client.md) | Web-mode bootstrapping, RPC client |
| TDD-FE-07 | [07-hooks-and-ipc.md](../../../tdd/07-hooks-and-ipc.md) | Hook patterns, IPC event subscription |

---

## Nguyên tắc thiết kế (áp dụng cho tất cả solutions)

1. **Không sửa `App.tsx`** — Chỉ thêm components mới, không đụng App shell
2. **Zustand slices** — Mọi global state qua slice, không component-local state phức tạp
3. **Cleanup required** — Mọi `window.api.on*()` phải có `off*()` trong `useEffect` cleanup
4. **`useShallow`** — Dùng khi selector trả về object để tránh re-render thừa
5. **Lazy loading** — Components onboarding heavy → `React.lazy()` + `<Suspense>`
6. **Platform guard** — Detect web vs electron qua `import.meta.env.ORCA_PLATFORM`
