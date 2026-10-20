-- Grant events were queued as task.grant_received / task.grant_revoked, outside the TASK stream (orca.task.>): JetStream refused them on
-- every poll and the relay stops at the first failure of a batch, so they held back every later event. Move the unsent ones under the stream.
UPDATE task.outbox_events SET subject = 'orca.' || subject WHERE published_at IS NULL AND subject IN ('task.grant_received', 'task.grant_revoked');
