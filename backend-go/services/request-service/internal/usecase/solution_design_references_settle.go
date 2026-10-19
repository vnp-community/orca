package usecase

import (
	"encoding/json"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// settleDesignReferences makes design_references say what the run could actually do: every agent Solution carries one, and a
// run without the graph can only say it was unavailable, whatever the agent claimed.
func settleDesignReferences(doc []byte, hadKnowledge bool) ([]byte, error) {
	opts, err := domain.ParseSolutionOptions(doc)
	if err != nil {
		return nil, err
	}
	if !hadKnowledge || opts.DesignReferences == nil {
		gaps := []string{}
		if opts.DesignReferences != nil {
			gaps = opts.DesignReferences.Gaps
		}
		if len(gaps) == 0 && !hadKnowledge {
			gaps = []string{"the knowledge graph was not available in this run"}
		}
		if len(gaps) == 0 {
			gaps = []string{"the agent did not report what it read in the knowledge graph"}
		}
		opts.DesignReferences = &domain.DesignReferences{Available: false, Gaps: gaps}
	}
	return json.Marshal(opts)
}
