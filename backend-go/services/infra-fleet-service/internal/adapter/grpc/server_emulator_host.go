// Handlers for the emulator.* relay (TASK-048) and the per-target host
// capability probe (TASK-070) — split out of server.go per this package's
// growing size, not because these RPCs differ structurally from any other
// handler here (translate request -> usecase -> translate response,
// apperrors.ToGRPCStatus on error, same as every other method in
// server.go).
package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stablyai/orca-go/common/apperrors"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

func (s *Server) ListEmulatorDevices(ctx context.Context, req *infrafleetv1.ListEmulatorDevicesRequest) (*infrafleetv1.ListEmulatorDevicesResponse, error) {
	devices, err := s.emulatorRelay.ListDevices(ctx, req.GetConnectionId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*infrafleetv1.EmulatorDevice, 0, len(devices))
	for _, d := range devices {
		out = append(out, &infrafleetv1.EmulatorDevice{Id: d.ID, Name: d.Name, Platform: d.Platform, State: d.State})
	}
	return &infrafleetv1.ListEmulatorDevicesResponse{Devices: out}, nil
}

func (s *Server) GetEmulatorAvailability(ctx context.Context, req *infrafleetv1.GetEmulatorAvailabilityRequest) (*infrafleetv1.GetEmulatorAvailabilityResponse, error) {
	available, reason, err := s.emulatorRelay.GetAvailability(ctx, req.GetConnectionId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.GetEmulatorAvailabilityResponse{Available: available, Reason: reason}, nil
}

func (s *Server) AttachEmulatorSession(ctx context.Context, req *infrafleetv1.AttachEmulatorSessionRequest) (*infrafleetv1.EmulatorSession, error) {
	session, err := s.emulatorRelay.AttachSession(ctx, req.GetConnectionId(), req.GetDeviceId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.EmulatorSession{
		SessionId: session.SessionID, DeviceId: session.DeviceID,
		ConnectionId: session.ConnectionID, Platform: session.Platform,
	}, nil
}

func (s *Server) SendEmulatorTap(ctx context.Context, req *infrafleetv1.SendEmulatorTapRequest) (*emptypb.Empty, error) {
	err := s.emulatorRelay.SendCommand(ctx, req.GetConnectionId(), "device.tap", map[string]any{
		"sessionId": req.GetSessionId(), "x": req.GetX(), "y": req.GetY(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) SendEmulatorGesture(ctx context.Context, req *infrafleetv1.SendEmulatorGestureRequest) (*emptypb.Empty, error) {
	err := s.emulatorRelay.SendCommand(ctx, req.GetConnectionId(), "device.gesture", map[string]any{
		"sessionId": req.GetSessionId(),
		"startX":    req.GetStartX(), "startY": req.GetStartY(),
		"endX": req.GetEndX(), "endY": req.GetEndY(),
		"durationMs": req.GetDurationMs(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) SendEmulatorButton(ctx context.Context, req *infrafleetv1.SendEmulatorButtonRequest) (*emptypb.Empty, error) {
	err := s.emulatorRelay.SendCommand(ctx, req.GetConnectionId(), "device.button", map[string]any{
		"sessionId": req.GetSessionId(), "button": req.GetButton(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) RotateEmulator(ctx context.Context, req *infrafleetv1.RotateEmulatorRequest) (*emptypb.Empty, error) {
	err := s.emulatorRelay.SendCommand(ctx, req.GetConnectionId(), "device.rotate", map[string]any{
		"sessionId": req.GetSessionId(), "orientation": req.GetOrientation(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ShutdownEmulator(ctx context.Context, req *infrafleetv1.ShutdownEmulatorRequest) (*emptypb.Empty, error) {
	err := s.emulatorRelay.SendCommand(ctx, req.GetConnectionId(), "device.shutdown", map[string]any{
		"sessionId": req.GetSessionId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

// GetHostCapabilities backs the frontend's per-target host.wsl.isAvailable/
// host.wsl.listDistros/host.pwsh.isAvailable/host.gitBash.isAvailable
// channels — one RPC covers all 4, see usecase.GetHostCapabilities's doc
// comment and the proto's rpc doc comment for why.
func (s *Server) GetHostCapabilities(ctx context.Context, req *infrafleetv1.GetHostCapabilitiesRequest) (*infrafleetv1.GetHostCapabilitiesResponse, error) {
	result, err := s.getHostCapabilities.Execute(ctx, req.GetConnectionId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.GetHostCapabilitiesResponse{
		WslAvailable:     result.WslAvailable,
		WslDistros:       result.WslDistros,
		PwshAvailable:    result.PwshAvailable,
		GitBashAvailable: result.GitBashAvailable,
	}, nil
}
