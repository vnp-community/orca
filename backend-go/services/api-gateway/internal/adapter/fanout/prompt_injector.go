package fanout

import (
	"context"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

type GRPCPromptInjector struct {
	client infrafleetv1.InfraFleetServiceClient
}

func NewGRPCPromptInjector(client infrafleetv1.InfraFleetServiceClient) *GRPCPromptInjector {
	return &GRPCPromptInjector{client: client}
}

func (p *GRPCPromptInjector) InjectPrompt(ctx context.Context, connectionID, ptyID, prompt string) error {
	stream, err := p.client.AttachPty(ctx)
	if err != nil {
		return err
	}
	defer stream.CloseSend()
	if err := stream.Send(&infrafleetv1.PtyClientFrame{Frame: &infrafleetv1.PtyClientFrame_Attach{Attach: &infrafleetv1.AttachToSession{PtyId: ptyID}}}); err != nil {
		return err
	}
	return stream.Send(&infrafleetv1.PtyClientFrame{Frame: &infrafleetv1.PtyClientFrame_Input{Input: &infrafleetv1.PtyInput{Data: []byte(prompt + "\n")}}})
}
