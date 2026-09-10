-- infra.port_forwards was schema-only (§5's DDL) until SOL-SSH-04 gave it a
-- real writer (usecase.PollWorkspacePorts). process_name/status are the two
-- columns the domain entity needs beyond the original DDL.
ALTER TABLE infra.port_forwards ADD COLUMN process_name TEXT NOT NULL DEFAULT '';
ALTER TABLE infra.port_forwards ADD COLUMN status TEXT NOT NULL DEFAULT 'active';
