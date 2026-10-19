package grpcclient

import (
	"fmt"

	"github.com/stablyai/orca-go/prompts/classify_request"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// RenderClassificationPrompt maps the classification input to prompts/classify_request.
func RenderClassificationPrompt(in usecase.ClassificationInput) (string, error) {
	text, err := classify_request.Render(classify_request.Input{Title: in.Title, IssueType: in.IssueType, Labels: in.Labels, Body: in.Body})
	if err != nil {
		return "", fmt.Errorf("build the classification prompt: %w", err)
	}
	return text, nil
}
