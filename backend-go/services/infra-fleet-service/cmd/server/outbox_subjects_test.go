package main

import (
	"strings"
	"testing"

	infraeventbus "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/eventbus"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

// ensuredStreamPrefixes mirrors the JetStream filters main.go creates for the outbox relay. An outbox row whose
// subject matches none of them is never acked, so the relay retries it forever (dev_server.health_degraded did).
var ensuredStreamPrefixes = []string{"orca.infrafleet.", "orca.infra_fleet.", "orca.infra."}

func TestOutboxSubjectsAreCoveredByAnEnsuredStream(t *testing.T) {
	subjects := map[string]string{
		"health degraded":    infraeventbus.HealthDegradedSubject,
		"ssh connected":      usecase.SSHConnectedSubject,
		"capabilities":       usecase.CapabilitiesChangedSubject,
		"dev server dropped": domain.DevServerDisconnectedSubject,
		"terminal completed": infraeventbus.SubjectAgentCompleted,
		"terminal error":     infraeventbus.SubjectAgentError,
		"terminal waiting":   infraeventbus.SubjectAgentWaiting,
	}
	for name, subject := range subjects {
		covered := false
		for _, prefix := range ensuredStreamPrefixes {
			if strings.HasPrefix(subject, prefix) {
				covered = true
			}
		}
		if !covered {
			t.Errorf("%s: subject %q is not under any ensured JetStream stream", name, subject)
		}
	}
}
