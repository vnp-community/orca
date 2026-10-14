-- What an agent run says about itself: tool names with call counts and claude's usage figures. Names and numbers only, never
-- what went into or came out of a tool.
ALTER TABLE request.analysis_runs ADD COLUMN run_stats JSONB NULL;
