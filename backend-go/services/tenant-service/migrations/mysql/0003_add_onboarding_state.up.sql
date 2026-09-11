-- Mirrors postgres/0003_add_onboarding_state.up.sql. JSONB -> JSON, nullable
-- (no DEFAULT — same as the Postgres column, which also has none; "no row
-- ever saved" and "column NULL" are both valid found=false states per
-- UserProfileRepository.GetOnboardingState's doc comment).
ALTER TABLE user_profiles ADD COLUMN onboarding_state_json JSON;
