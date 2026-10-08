package schemas

import (
	"sync"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

var (
	defaultOnce sync.Once
	defaultReg  *domain.SchemaRegistry
	defaultErr  error
)

// Default compiles the embedded schemas once per process.
func Default() (*domain.SchemaRegistry, error) {
	defaultOnce.Do(func() { defaultReg, defaultErr = domain.NewSchemaRegistry(V1) })
	return defaultReg, defaultErr
}
