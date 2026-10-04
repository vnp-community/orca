package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

// SubjectCredentialRotated is consumed by notification-service (critical
// "credential rotated" alert); the CREDENTIAL stream is created in main.go.
const SubjectCredentialRotated = "orca.credential.credential.rotated"

// EventEnqueuer is implemented by the transaction-scoped repository so an
// event commits atomically with the change that caused it. It is discovered by
// type assertion on the repo RunInTx hands back, which keeps TxRunner's
// signature (and every usecase using it) unchanged; repos without an outbox
// simply publish nothing.
type EventEnqueuer interface {
	EnqueueEvent(ctx context.Context, e domain.OutboxEvent) error
}

type credentialRotatedPayload struct {
	CredentialID string `json:"credential_id"`
	Category     string `json:"category"`
	OwnerID      string `json:"owner_id"`
	RotatedAt    string `json:"rotated_at"`
}

func enqueueCredentialRotated(ctx context.Context, repo any, m domain.CredentialMetadata, at time.Time) error {
	enq, ok := repo.(EventEnqueuer)
	if !ok {
		return nil
	}
	payload, err := json.Marshal(credentialRotatedPayload{
		CredentialID: m.ID, Category: string(m.Category), OwnerID: m.OwnerID, RotatedAt: at.UTC().Format("2006-01-02T15:04:05Z07:00"),
	})
	if err != nil {
		return err
	}
	return enq.EnqueueEvent(ctx, domain.OutboxEvent{
		ID: uuid.NewString(), TenantID: m.TenantID, Subject: SubjectCredentialRotated, OccurredAt: at, Payload: payload,
	})
}
