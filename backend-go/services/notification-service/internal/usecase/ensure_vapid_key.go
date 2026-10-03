package usecase

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

const (
	vapidProvisionTimeout = 15 * time.Second
	// A missing Vault policy must not make every page load hit Vault again.
	vapidForbiddenCacheTTL = 30 * time.Second
	vapidErrorCacheTTL     = 5 * time.Second
	vapidKeyRefPrefix      = "vapid-signing-"
)

// EnsureVapidKey makes sure a tenant has an active VAPID key row, creating
// the Vault Transit key (through the broker) and the metadata row on first
// use. Concurrent callers in one process share a single attempt; replicas
// converge through InsertActiveIfAbsent.
type EnsureVapidKey struct {
	store    VapidKeyStore
	broker   VapidKeyProvisioner
	observer VapidProvisionObserver
	now      func() time.Time

	mu       sync.Mutex
	inflight map[string]*vapidCall
	negative map[string]negativeEntry
}

type vapidCall struct {
	done chan struct{}
	key  domain.VapidKeyMetadata
	err  error
}

type negativeEntry struct {
	err     error
	expires time.Time
}

func NewEnsureVapidKey(store VapidKeyStore, broker VapidKeyProvisioner, observer VapidProvisionObserver) *EnsureVapidKey {
	return &EnsureVapidKey{
		store: store, broker: broker, observer: observer, now: time.Now,
		inflight: map[string]*vapidCall{}, negative: map[string]negativeEntry{},
	}
}

// Execute returns the tenant's active key, provisioning it when missing.
func (uc *EnsureVapidKey) Execute(ctx context.Context, tenantID string) (domain.VapidKeyMetadata, error) {
	key, err := uc.store.GetPublicKey(ctx, tenantID)
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, domain.ErrNoActiveVapidKey) {
		return domain.VapidKeyMetadata{}, err
	}

	uc.mu.Lock()
	if neg, ok := uc.negative[tenantID]; ok {
		if uc.now().Before(neg.expires) {
			uc.mu.Unlock()
			return domain.VapidKeyMetadata{}, neg.err
		}
		delete(uc.negative, tenantID)
	}
	call, running := uc.inflight[tenantID]
	if !running {
		call = &vapidCall{done: make(chan struct{})}
		uc.inflight[tenantID] = call
	}
	uc.mu.Unlock()

	if running {
		select {
		case <-call.done:
			return call.key, call.err
		case <-ctx.Done():
			return domain.VapidKeyMetadata{}, ctx.Err()
		}
	}

	// The shared attempt must not die with the leader's request context.
	runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), vapidProvisionTimeout)
	defer cancel()
	call.key, call.err = uc.provision(runCtx, tenantID)

	uc.mu.Lock()
	delete(uc.inflight, tenantID)
	if call.err != nil && ctx.Err() == nil {
		ttl := vapidErrorCacheTTL
		if errors.Is(call.err, ErrVapidProvisionForbidden) {
			ttl = vapidForbiddenCacheTTL
		}
		uc.negative[tenantID] = negativeEntry{err: call.err, expires: uc.now().Add(ttl)}
	}
	uc.mu.Unlock()
	close(call.done)
	return call.key, call.err
}

func (uc *EnsureVapidKey) provision(ctx context.Context, tenantID string) (domain.VapidKeyMetadata, error) {
	// Another replica may have finished since the caller's check.
	if key, err := uc.store.GetPublicKey(ctx, tenantID); err == nil {
		uc.observe("existing")
		return key, nil
	} else if !errors.Is(err, domain.ErrNoActiveVapidKey) {
		uc.observe("error")
		return domain.VapidKeyMetadata{}, err
	}

	publicKey, err := uc.broker.EnsureVapidSigningKey(ctx, tenantID)
	if err != nil {
		if errors.Is(err, ErrVapidProvisionForbidden) {
			uc.observe("forbidden")
		} else {
			uc.observe("error")
		}
		return domain.VapidKeyMetadata{}, err
	}
	if publicKey == "" {
		uc.observe("error")
		return domain.VapidKeyMetadata{}, errors.New("usecase: credential broker returned an empty vapid public key")
	}

	stored, inserted, err := uc.store.InsertActiveIfAbsent(ctx, domain.VapidKeyMetadata{
		KeyID:       uuid.NewString(),
		TenantID:    tenantID,
		PublicKey:   publicKey,
		VaultKeyRef: vapidKeyRefPrefix + tenantID,
		Status:      domain.VapidKeyActive,
		CreatedAt:   uc.now().UTC(),
	})
	if err != nil {
		uc.observe("error")
		return domain.VapidKeyMetadata{}, fmt.Errorf("usecase: storing vapid key metadata: %w", err)
	}
	if inserted {
		uc.observe("created")
	} else {
		uc.observe("existing")
	}
	return stored, nil
}

func (uc *EnsureVapidKey) observe(outcome string) {
	if uc.observer != nil {
		uc.observer.ObserveVapidProvision(outcome)
	}
}
