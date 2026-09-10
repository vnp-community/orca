-- Bounded to 8KB via application-layer truncation (not a DB CHECK
-- constraint, since the truncation point is a product tradeoff — see
-- TASK-TG-04-07's Context note) before writing. Backs {{outputs.<taskId>.*}}
-- prompt interpolation for a later wave's ExecuteBatch dispatch — see that
-- task's note on the remaining buildExecutePrompt/CompleteExecution wiring
-- this column enables but does not itself complete.
ALTER TABLE task.tasks ADD COLUMN last_execution_output TEXT;
