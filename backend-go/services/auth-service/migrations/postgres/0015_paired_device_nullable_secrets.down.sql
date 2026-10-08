ALTER TABLE auth.paired_devices ALTER COLUMN shared_secret_ciphertext SET NOT NULL;
ALTER TABLE auth.paired_devices ALTER COLUMN vault_key_ref SET NOT NULL;
