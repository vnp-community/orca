package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// SubjectDevServerStatusChanged carries dev server status snapshots. Direct publish, no outbox: a missed snapshot is
// replaced by the next one and a list call always shows the truth, so durability would only add stale history.
const SubjectDevServerStatusChanged = "orca.infra.devserver.statusChanged"

// devServerStatusPayload uses the camelCase names of the gateway's devServer.list view.
type devServerStatusPayload struct {
	DevServerID      string `json:"devServerId"`
	Connected        bool   `json:"connected"`
	ActiveExecutions int    `json:"activeExecutions"`
	Busy             bool   `json:"busy"`
}

type DevServerStatusPublisher struct {
	pub *commoneventbus.Publisher
}

func NewDevServerStatusPublisher(pub *commoneventbus.Publisher) *DevServerStatusPublisher {
	return &DevServerStatusPublisher{pub: pub}
}

func (p *DevServerStatusPublisher) PublishDevServerStatus(ctx context.Context, tenantID string, ev domain.DevServerStatusEvent) error {
	raw, err := json.Marshal(devServerStatusPayload{
		DevServerID: ev.DevServerID, Connected: ev.Connected, ActiveExecutions: ev.ActiveExecutions, Busy: ev.Connected && ev.ActiveExecutions > 0,
	})
	if err != nil {
		return fmt.Errorf("eventbus: marshal dev server status: %w", err)
	}
	return p.pub.Publish(ctx, SubjectDevServerStatusChanged, commoneventbus.Event{
		ID: uuid.NewString(), TenantID: tenantID, OccurredAt: time.Now().UTC(), Version: 1, Payload: raw,
	})
}
