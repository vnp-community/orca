DROP TABLE IF EXISTS user_workspace_sessions;

ALTER TABLE user_profiles
  DROP COLUMN keybindings_json,
  DROP COLUMN ui_local_state_json,
  DROP COLUMN saved_runtime_environments_json,
  DROP COLUMN client_settings_json,
  DROP COLUMN accounts_dev_server_json;
