-- Live bug: the onboarding wizard's completion/progress state was never
-- persisted anywhere in backend-go (channels_onboarding.go always echoed
-- the caller's update back without storing it) — every page refresh reset
-- to "wizard not started", re-showing onboarding forever. tenant.user_profiles
-- is already the per-user (one row per user_id) state table this service
-- owns; onboarding progress is genuinely per-user, not a cascading
-- company/department/team default like settings_json, so it gets its own
-- column rather than being folded into that deep-merge blob.
ALTER TABLE tenant.user_profiles ADD COLUMN onboarding_state_json JSONB;
