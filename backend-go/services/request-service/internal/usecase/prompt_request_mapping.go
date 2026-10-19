package usecase

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/stablyai/orca-go/prompts/request_context"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// promptRequest maps a Request to what the prompts show. Only active acceptance criteria are listed, so the model
// cannot cite one that was retired; unreadable content lists none.
func promptRequest(r domain.Request) request_context.Request {
	out := request_context.Request{
		Title: r.Title, Type: string(r.Type), Size: string(r.Size), Urgency: string(r.Urgency), SourceProvider: string(r.SourceProvider),
		ClassificationReason: r.ClassificationReason, Body: r.Body,
	}
	c, err := domain.ContentFromRequest(r)
	if err != nil {
		return out
	}
	for _, it := range c.AcceptanceCriteria.Items {
		if it.Status == domain.ACStatusActive {
			out.AcceptanceCriteria = append(out.AcceptanceCriteria, request_context.Criterion{ID: it.ID, Text: it.Text})
		}
	}
	return out
}

func promptProject(name, repoURL string) request_context.Project {
	return request_context.Project{Name: name, RepoURL: repoURL}
}

// errPromptBuild marks a prompt that could not be rendered. It is a fault of this service, never of the model or the agent.
var errPromptBuild = errors.New("the prompt could not be built")

func promptBuildError(err error) error { return fmt.Errorf("%w: %w", errPromptBuild, err) }

// promptBuildFailure ends a run whose prompt could not be built. The log names the template error, never the Request text.
func promptBuildFailure(run domain.AnalysisRun, err error) *AnalysisFailure {
	slog.Warn("prompt could not be built", slog.String("run_id", run.ID), slog.Any("error", err))
	return newFailure(run, domain.RunErrInternal, "the prompt could not be built")
}

// builtPrompt tags a render error so callers can tell it from a model or agent failure.
func builtPrompt(text string, err error) (string, error) {
	if err != nil {
		return "", promptBuildError(err)
	}
	return text, nil
}
