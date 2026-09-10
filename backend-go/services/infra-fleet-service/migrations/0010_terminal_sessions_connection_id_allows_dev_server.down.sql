ALTER TABLE infra.terminal_sessions
    ADD CONSTRAINT terminal_sessions_connection_id_fkey
    FOREIGN KEY (connection_id) REFERENCES infra.connections(id);
