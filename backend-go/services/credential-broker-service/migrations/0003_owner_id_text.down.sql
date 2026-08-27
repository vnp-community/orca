-- Reverts owner_id to UUID. Only safe if every existing row's owner_id is
-- still UUID-shaped — will fail loudly (not silently truncate/corrupt) on
-- any non-UUID owner_id value written while this migration was up, which is
-- the correct behavior: a down-migration should not silently discard data
-- it cannot represent.
ALTER TABLE credential.credential_metadata ALTER COLUMN owner_id TYPE UUID USING owner_id::uuid;
