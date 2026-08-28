# TASK-PRF-03-03: Implement `DevServerHealthChecker`/`ProfileResolver` grpcclient adapters and new `adapter/eventbus` package

**From Solution:** SOL-PRF-03
**Priority:** P0
**Service:** `project-service`
**File:** `backend-go/services/project-service/internal/adapter/grpcclient/dev_server_health_checker.go`
**Depends on:** TASK-PRF-03-01, TASK-PRF-03-02
**Status:** `[x]` DONE — InfraFleetHealthChecker (adapted to real GetFleetHealthRequest{tenant_id}/statuses[] shape) + TenantProfileResolver + adapter/eventbus package added; go build clean; publisher_test.go payload-shape tests pass

---

## Context

Implements the two new outbound ports against real RPCs, and stands up
`project-service`'s first `adapter/eventbus` package (this service currently
has none — `find internal/adapter` today lists only
`postgres/grpc/opaclient/grpcclient`) for the audit/rebind-notify outbox,
mirroring `tenant-service`'s existing `adapter/eventbus` shape.

## Changes to make

Create `backend-go/services/project-service/internal/adapter/grpcclient/dev_server_health_checker.go`,
following `infra_fleet_dev_server_lister.go`'s existing dial pattern in this
same package:

```go
package grpcclient

import (
	"context"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// InfraFleetHealthChecker implements usecase.DevServerHealthChecker by
// calling infra-fleet-service's GetFleetHealth RPC — already exists on the
// proto (infrafleet.proto), just never called by project-service before
// this task (see infrafleet.proto's DevServerHealth message: reachable,
// cpu_percent, ram_percent, disk_percent, latency_ms).
type InfraFleetHealthChecker struct {
	client infrafleetv1.InfraFleetServiceClient
}

// NewInfraFleetHealthChecker wraps an already-dialed client — reuse the
// same *grpc.ClientConn InfraFleetDevServerLister/DevServerRelay dial in
// cmd/server/main.go rather than opening a second connection.
func NewInfraFleetHealthChecker(client infrafleetv1.InfraFleetServiceClient) *InfraFleetHealthChecker {
	return &InfraFleetHealthChecker{client: client}
}

func (c *InfraFleetHealthChecker) IsReachable(ctx context.Context, tenantID, devServerID string) (bool, error) {
	resp, err := c.client.GetFleetHealth(ctx, &infrafleetv1.GetFleetHealthRequest{DevServerId: devServerID})
	if err != nil {
		return false, fmt.Errorf("grpcclient: infra-fleet-service GetFleetHealth: %w", err)
	}
	return resp.GetReachable(), nil
}
```

Confirm `GetFleetHealthRequest`'s actual field name (`DevServerId` above is
this task's best guess from SOL-PRF-03's citation of `infrafleet.proto:17`)
against the generated `infrafleet.pb.go` before wiring — adjust if the real
field differs.

Create `backend-go/services/project-service/internal/adapter/grpcclient/profile_resolver.go`,
implementing `usecase.ProfileResolver` against `tenantv1.TenantServiceClient`
(`GetResolvedProfile`, decoding `resolved_settings_json`'s
`fleet.allowedServerTags`) and `infrafleetv1.InfraFleetServiceClient`
(`ListDevServers`, matching by id for `DevServerTags`) — dial both the same
way `InfraFleetDevServerLister`/`InfraFleetHealthChecker` already do.

Create the new package `backend-go/services/project-service/internal/adapter/eventbus/publisher.go`,
mirroring `tenant-service/internal/adapter/eventbus/publisher.go`'s shape:

```go
package eventbus

import (
	"encoding/json"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
)

const StreamName = "PROJECT"
const AuditSubject = "orca.project.audit.recorded"
const DevServerChangedSubject = "orca.project.devserver.changed"

type Publisher struct {
	pub *commoneventbus.Publisher
}

func New(pub *commoneventbus.Publisher) *Publisher {
	return &Publisher{pub: pub}
}

type auditPayload struct {
	Action  string `json:"action"`
	ActorID string `json:"actor_id"`
	Target  string `json:"target"`
}

// PublishAuditEvent implements usecase.AuditPublisher.
func (p *Publisher) PublishAuditEvent(ctx context.Context, tenantID, actorID, action, target string) error {
	payload, err := json.Marshal(auditPayload{Action: action, ActorID: actorID, Target: target})
	if err != nil {
		return fmt.Errorf("eventbus: marshal audit payload: %w", err)
	}
	return p.pub.Publish(ctx, AuditSubject, commoneventbus.Event{
		ID: uuid.NewString(), TenantID: tenantID, OccurredAt: time.Now().UTC(), Version: 1, Payload: payload,
	})
}

// notificationEventPayload matches notification-service's generic
// EventPayload{UserIDs, Title, Body, DeepLink} shape exactly
// (notification_event.go:39-45) so it translates without a dedicated
// subjectRules entry even before TASK-PRF-03-08 adds one.
type notificationEventPayload struct {
	UserIDs  []string `json:"user_ids"`
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	DeepLink string   `json:"deep_link"`
}

// NotifyDevServerChanged implements usecase.MemberNotifier.
func (p *Publisher) NotifyDevServerChanged(ctx context.Context, tenantID string, userIDs []string, projectID, oldDevServerID, newDevServerID string) error {
	payload, err := json.Marshal(notificationEventPayload{
		UserIDs: userIDs, Title: "Dev server changed",
		Body:     "This project's dev server binding was changed.",
		DeepLink: "/projects/" + projectID,
	})
	if err != nil {
		return fmt.Errorf("eventbus: marshal devserver-changed payload: %w", err)
	}
	return p.pub.Publish(ctx, DevServerChangedSubject, commoneventbus.Event{
		ID: uuid.NewString(), TenantID: tenantID, OccurredAt: time.Now().UTC(), Version: 1, Payload: payload,
	})
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/project-service/internal/adapter/...
```

Add `adapter/eventbus/publisher_test.go` asserting the marshaled payload
shape (especially `user_ids` vs a typo like `userIds` — a mismatch here fails
silently downstream, only caught by TASK-PRF-03-08's cross-service contract
test).
