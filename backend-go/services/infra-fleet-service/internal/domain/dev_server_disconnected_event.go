package domain

// DevServerDisconnectedSubject is published on a fleet-health
// reachable=true -> false transition (see usecase.PollFleetHealth). Not yet
// consumed by notification-service — see this feature's own doc comment in
// poll_fleet_health.go for why: resolving which tenant admins to notify
// requires calling auth-service.ListUsers, which today can only be invoked
// by an authenticated admin actor already in ctx (requireAdminActor) — a
// background poller has none. Deliberately scoped out rather than inventing
// a new service-to-service authorization primitive in this pass; the event
// is still durably recorded and published so a future consumer (once that
// primitive exists) has real data to react to, and it's independently
// visible today via the WARN log line PollFleetHealth also emits on the
// same transition.
const DevServerDisconnectedSubject = "orca.infrafleet.dev_server.disconnected"

// DevServerDisconnectedPayload is DevServerDisconnectedSubject's payload
// shape — deliberately does NOT include user_id/user_ids (see the subject
// const's doc comment for why no recipient can be resolved yet).
type DevServerDisconnectedPayload struct {
	DevServerID string `json:"dev_server_id"`
	Host        string `json:"host"`
	TenantID    string `json:"tenant_id"`
}
