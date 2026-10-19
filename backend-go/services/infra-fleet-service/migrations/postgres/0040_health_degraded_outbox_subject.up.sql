-- Rows enqueued under the bare subject matched no JetStream stream and were retried forever; move the unpublished ones under the INFRAFLEET stream.
UPDATE infra.outbox_events
SET subject = 'orca.infrafleet.dev_server.health_degraded'
WHERE subject = 'dev_server.health_degraded' AND published_at IS NULL;
