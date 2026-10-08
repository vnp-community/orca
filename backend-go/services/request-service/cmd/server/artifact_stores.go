package main

import (
	mysqladapter "github.com/stablyai/orca-go/services/request-service/internal/adapter/mysql"
	postgresadapter "github.com/stablyai/orca-go/services/request-service/internal/adapter/postgres"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// artifactStores are the repositories of the artifact model and the clarification and decision records
// (CR-REQ-027, CR-REQ-028), kept apart from requestStores so those features own their own wiring.
type artifactStores struct {
	content        usecase.RequestContentWriter
	revisions      usecase.RequestRevisionRepository
	index          usecase.ArtifactIndexRepository
	relations      usecase.ArtifactRelationRepository
	coverage       usecase.RequestCoverageRepository
	clarifications usecase.ClarificationRepository
	decisions      usecase.DecisionRepository
}

func postgresArtifactStores(base *postgresadapter.Repository) artifactStores {
	return artifactStores{
		content:        postgresadapter.NewRequestContentRepository(base),
		revisions:      postgresadapter.NewRequestRevisionRepository(base),
		index:          postgresadapter.NewArtifactIndexRepository(base),
		relations:      postgresadapter.NewArtifactRelationRepository(base),
		coverage:       postgresadapter.NewRequestCoverageRepository(base),
		clarifications: postgresadapter.NewClarificationRepository(base),
		decisions:      postgresadapter.NewDecisionRepository(base),
	}
}

func mysqlArtifactStores(base *mysqladapter.Repository) artifactStores {
	return artifactStores{
		content:        mysqladapter.NewRequestContentRepository(base),
		revisions:      mysqladapter.NewRequestRevisionRepository(base),
		index:          mysqladapter.NewArtifactIndexRepository(base),
		relations:      mysqladapter.NewArtifactRelationRepository(base),
		coverage:       mysqladapter.NewRequestCoverageRepository(base),
		clarifications: mysqladapter.NewClarificationRepository(base),
		decisions:      mysqladapter.NewDecisionRepository(base),
	}
}
