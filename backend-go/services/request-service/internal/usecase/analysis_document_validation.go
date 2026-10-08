package usecase

import (
	"fmt"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ValidateByKind checks a diagnosis, findings or answer document and returns its normalised JSON
// (typed fields only, so anything the model invented is dropped). minOptions applies to kind solution.
func ValidateByKind(kind domain.SolutionKind, raw []byte, minOptions int) ([]byte, error) {
	switch kind {
	case domain.SolutionKindSolution:
		o, err := domain.ParseSolutionOptions(raw)
		if err != nil {
			return nil, err
		}
		if err := o.Validate(minOptions); err != nil {
			return nil, err
		}
		return o.Marshal()
	case domain.SolutionKindDiagnosis:
		d, err := domain.ParseDiagnosisDocument(raw)
		if err != nil {
			return nil, err
		}
		if err := d.Validate(); err != nil {
			return nil, err
		}
		return d.Marshal()
	case domain.SolutionKindFindings:
		d, err := domain.ParseFindingsDocument(raw)
		if err != nil {
			return nil, err
		}
		if err := d.Validate(); err != nil {
			return nil, err
		}
		return d.Marshal()
	case domain.SolutionKindAnswer:
		d, err := domain.ParseAnswerDocument(raw)
		if err != nil {
			return nil, err
		}
		if err := d.Validate(); err != nil {
			return nil, err
		}
		return d.Marshal()
	}
	return nil, fmt.Errorf("unknown solution kind %q", kind)
}
