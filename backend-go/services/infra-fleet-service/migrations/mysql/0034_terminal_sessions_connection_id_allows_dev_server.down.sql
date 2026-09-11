ALTER TABLE terminal_sessions
    ADD CONSTRAINT terminal_sessions_connection_id_fkey
    FOREIGN KEY (connection_id) REFERENCES connections(id);
