package usecase

import "context"

type ManageDecisions struct {}

func (uc *ManageDecisions) RecordDecision(ctx context.Context, reqID, decision string) error {
	return nil
}

func (uc *ManageDecisions) ConfirmDecision(ctx context.Context, id string) error {
	return nil
}
