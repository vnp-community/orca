-- port_forwards was schema-only (§5's DDL) until SOL-SSH-04 gave it a
-- real writer (usecase.PollWorkspacePorts). process_name/status are the two
-- columns the domain entity needs beyond the original DDL.
-- MySQL 8.0.13+ requires TEXT column defaults to be a parenthesized
-- expression — see migrations/mysql/0002_connections.up.sql's comment.
ALTER TABLE port_forwards ADD COLUMN process_name TEXT NOT NULL DEFAULT ('');
ALTER TABLE port_forwards ADD COLUMN status TEXT NOT NULL DEFAULT ('active');
