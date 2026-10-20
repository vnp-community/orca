-- How many secrets were masked in the stored copy of an OpenSpec change (CR-REQ-045). Rows written before this keep 0: they
-- hold the raw text and are masked when read, so nothing is backfilled.
ALTER TABLE request.openspec_changes ADD COLUMN redacted_secrets INT NOT NULL DEFAULT 0;
