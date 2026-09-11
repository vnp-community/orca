ALTER TABLE accounts
  DROP COLUMN dev_server_id,
  DROP COLUMN deleted_at,
  DROP COLUMN label,
  DROP COLUMN model_hint,
  DROP COLUMN base_url;
