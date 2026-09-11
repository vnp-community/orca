# mobile-companion (F03) solutions — index (agent)

Implementation solutions/assessments for the `mobile-companion` CR series
([`docs/crs/v4/mobile-companion/`](../../../../../../docs/crs/v4/mobile-companion/README.md))
scoped to `agent/`. Both CRs in this series conclude with **zero `agent/`
code change** — this directory exists to make that a verified, evidence-backed
conclusion rather than a silent omission.

## Đánh giá trạng thái hiện tại (bắt buộc trước khi thiết kế — theo yêu cầu)

`docs/crs/v4/mobile-companion/README.md`'s own conclusion table already states
*"`agent/` có logic mobile thật không? Không, và không cần có"* — but that
conclusion was reached while auditing `desktop/`/`mobile/`/`backend-go` for
the CR series overall, not from a dedicated `agent/`-side pass. Re-auditing
`agent/src/` directly for this solution set confirms the conclusion holds,
and surfaces one thing the original audit didn't check: `agent/src/main/persistence.ts`
carries its own orphaned Web Push (`WebPushSubscription`/`getVapidKeys`,
"Phase 3 — TASK-033") API, unreachable from `agent/`'s real build — the same
class of dead code as `desktop/src/main/mobile/MobileCompanionService.ts`
(which CR-MOBILE-002 does retire), just not caught by that CR's
`desktop/`-only dead-code audit. See
[SOL-AG-MOBILE-001 §4](./SOL-AG-MOBILE-001-zero-agent-scope-notification-delivery.md#4-incidental-finding-out-of-scope-for-cr-mobile-001)
for the full evidence trail.

## Solutions

| CR | Solution | Status | Note |
|----|----------|--------|------|
| [CR-MOBILE-001](../../../../../../docs/crs/v4/mobile-companion/CR-MOBILE-001-notification-service-native-push-delivery.md) | [SOL-AG-MOBILE-001](./SOL-AG-MOBILE-001-zero-agent-scope-notification-delivery.md) | 📐 Assessment | **Zero code change** — push delivery is entirely `notification-service` (backend-go); the agent reports task/workflow/automation completion up its existing wire protocol and never touches NATS or `notification-service` |
| [CR-MOBILE-002](../../../../../../docs/crs/v4/mobile-companion/CR-MOBILE-002-mobile-app-device-registration-and-dead-code-retirement.md) | [SOL-AG-MOBILE-002](./SOL-AG-MOBILE-002-zero-agent-scope-device-registration.md) | 📐 Assessment | **Zero code change** — device-token registration and auth route entirely through `mobile/` ↔ `desktop/` ↔ `api-gateway`; the real `DeviceRegistry` and the dead `MobileCompanionService.ts` both live in `desktop/`, not `agent/` |

## Why neither CR has tasks in this directory

Both solutions are explicitly assessments, not designs — each states plainly
in its "Not in scope" section that it is "a confirmation, not a solution to
implement." Creating implementation tasks for a solution whose entire content
is "no code change required" would be inventing work the solution itself says
isn't there. (This mirrors the precedent in
`specs/agent/crs/v4/task-graph/tasks/README.md`'s "Why SOL-AG-TG-001 has no
tasks" section, itself following `specs/backend-go/crs/v3/flow-task/tasks/README.md`'s
"Why BE-SOL-004 has no tasks.")

## Why this reads differently from the `feature-completion-matrix.md` gap it originated from

`docs/roadmap/feature-completion-matrix.md`'s F03 row (as of 2026-09-09) read
*"AG chỉ có filename constants... thiếu pairing/QR/E2E ở backend-go & agent"*
— phrased as a gap to close. `docs/crs/v4/mobile-companion/README.md`'s own
audit (§"Kết luận khảo sát") already re-framed this: pairing/QR/E2E is
peer-to-peer by design (`docs/features/F03-mobile-companion.md:57`), needs no
backend-go or agent involvement, and was never a gap. The real gap the CR
series closes is **native push delivery** (APNs/FCM) for the Mobile Companion
app specifically — entirely a `notification-service` (backend-go) + `mobile/`
+ `desktop/` concern, confirmed here to have zero `agent/` surface either.
`docs/roadmap/feature-completion-matrix.md` itself is not updated by this
solution set (out of scope — see that CR README's own "Việc chưa làm ngoài
bộ CR này").

## Nguyên tắc chung khi đọc 2 solution này

Cả 2 đều đọc code thật (`agent/src/**`, `desktop/src/main/runtime/device-registry.ts`,
`specs/agent/tdd/v5/*`) trước khi kết luận "không cần sửa" — không suy diễn
từ bảng "Changes Required" của CR gốc. Nếu một CR tương lai trong series này
(hoặc một CR mới) thực sự cần `agent/` làm gì đó (ví dụ: agent tự relay một
sự kiện hoàn thành task tới thẳng notification-service thay vì qua
task-service trung gian), điều đó là một thay đổi kiến trúc cần CR riêng, không
phải một task ẩn trong 2 solution "zero change" này.
