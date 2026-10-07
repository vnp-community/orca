package grpc

import (
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type EngineSettingsServer struct {
	manageUC *usecase.ManageProjectEngineSettings
}

func NewEngineSettingsServer(manageUC *usecase.ManageProjectEngineSettings) *EngineSettingsServer {
	return &EngineSettingsServer{manageUC: manageUC}
}

// grpc methods here
