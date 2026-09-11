-- BL-MB-02: links a push subscription to a paired mobile device (SOL-MB-01)
-- so DeliverPush can resolve a shared secret (auth-service's
-- ResolveDeviceSharedSecret) for the E2E-sealed push payload. Nullable — a
-- standard (non-paired) Web Push subscription has no device_id. No FK to
-- auth-service's own paired_devices table: that table lives in a different
-- service's database (database-per-service rule), so this is a plain
-- pointer, resolved at delivery time, not join-enforced.
ALTER TABLE notification.push_subscriptions ADD COLUMN device_id UUID;
