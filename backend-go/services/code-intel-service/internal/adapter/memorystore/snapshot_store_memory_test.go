package memorystore

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stablyai/orca-go/services/code-intel-service/internal/adapter/snapshotstorecontract"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/usecase"
)

func TestMemorySnapshotStore(t *testing.T) {
	var tenantCounter int32
	
	newStore := func() usecase.SnapshotStore {
		return NewMemorySnapshotStore()
	}
	
	seedTenant := func() string {
		val := atomic.AddInt32(&tenantCounter, 1)
		return fmt.Sprintf("tenant-%d", val)
	}
	
	snapshotstorecontract.Run(t, newStore, seedTenant)
}
