-- Per-project switch: the Solution of a Request is written by the read-only agent (with the knowledge graph) instead of ai.complete.
ALTER TABLE request.project_engine_settings
    ADD COLUMN solution_agent BOOLEAN NOT NULL DEFAULT false;
