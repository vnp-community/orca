ALTER TABLE auth.paired_devices ALTER COLUMN shared_secret_ciphertext DROP NOT NULL;
ALTER TABLE auth.paired_devices ALTER COLUMN vault_key_ref DROP NOT NULL;
