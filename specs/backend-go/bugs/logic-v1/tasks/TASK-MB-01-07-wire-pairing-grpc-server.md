# TASK-MB-01-07: Wire pairing usecases into `auth-service`'s gRPC server + composition root

**From Solution:** SOL-MB-01
**Priority:** P0
**Service:** `auth-service`
**File:** `backend-go/services/auth-service/internal/adapter/grpc/server.go`, `backend-go/services/auth-service/cmd/server/main.go`
**Depends on:** TASK-MB-01-05, TASK-MB-01-06
**Status:** `[ ]` TODO

---

## Context

`Server` (in `internal/adapter/grpc/server.go`) already holds one field per
usecase (`deactivateUser`, `listSessionsForUser`, etc. — see the admin RPCs
added by `missing-v1/TASK-002`). This task adds the 6 pairing usecases the
same way, translating wire messages with no business logic in this layer.

## Changes to make

In `Server` struct, add:

```go
initiateDevicePairing    *usecase.InitiateDevicePairing
completeDevicePairing    *usecase.CompleteDevicePairing
listPairedDevices        *usecase.ListPairedDevices
unpairDevice              *usecase.UnpairDevice
resolveDeviceSharedSecret *usecase.ResolveDeviceSharedSecret
```

Extend the `New(...)` constructor's parameter list and assignment to match.

Add handler methods:

```go
func (s *Server) InitiateDevicePairing(ctx context.Context, req *authv1.InitiateDevicePairingRequest) (*authv1.InitiateDevicePairingResponse, error) {
	result, err := s.initiateDevicePairing.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.InitiateDevicePairingResponse{
		PairingToken:     result.PairingToken,
		DesktopPublicKey: result.DesktopPublicKey,
		ServerAddress:    result.ServerAddress,
		ExpiresAtUnixMs:  result.ExpiresAt.UnixMilli(),
	}, nil
}

func (s *Server) CompleteDevicePairing(ctx context.Context, req *authv1.CompleteDevicePairingRequest) (*authv1.CompleteDevicePairingResponse, error) {
	result, err := s.completeDevicePairing.Execute(ctx, req.GetPairingToken(), req.GetMobilePublicKey(), req.GetDeviceLabel())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.CompleteDevicePairingResponse{
		DeviceId:                     result.DeviceID,
		DesktopPublicKeyConfirmation: result.DesktopPublicKeyConfirmation,
		AccessToken:                  result.AccessToken,
		RefreshToken:                 result.RefreshToken,
	}, nil
}

func (s *Server) ListPairedDevices(ctx context.Context, req *authv1.ListPairedDevicesRequest) (*authv1.ListPairedDevicesResponse, error) {
	devices, err := s.listPairedDevices.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*authv1.PairedDevice, 0, len(devices))
	for _, d := range devices {
		out = append(out, &authv1.PairedDevice{
			Id: d.ID, DeviceLabel: d.DeviceLabel,
			PairedAtUnixMs: d.PairedAt.UnixMilli(), LastUsedAtUnixMs: d.LastUsedAt.UnixMilli(),
			Status: string(d.Status),
		})
	}
	return &authv1.ListPairedDevicesResponse{Devices: out}, nil
}

func (s *Server) UnpairDevice(ctx context.Context, req *authv1.UnpairDeviceRequest) (*emptypb.Empty, error) {
	if err := s.unpairDevice.Execute(ctx, req.GetDeviceId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ResolveDeviceSharedSecret(ctx context.Context, req *authv1.ResolveDeviceSharedSecretRequest) (*authv1.ResolveDeviceSharedSecretResponse, error) {
	secret, err := s.resolveDeviceSharedSecret.Execute(ctx, req.GetDeviceId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.ResolveDeviceSharedSecretResponse{SharedSecret: secret}, nil
}
```

In `cmd/server/main.go`, construct the two new adapters (`nacl.New()`,
`vault.NewSharedSecretSealer(secretsClient)`, calling `.Ensure(ctx)` at
startup alongside `TokenSigner.Ensure`), the two new repositories
(`postgres.NewPairingSessionStore`/`NewPairedDeviceStore`), the 5 usecases,
and pass them into `grpc.New(...)`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/auth-service/... && go vet ./services/auth-service/...
go test ./services/auth-service/...
```
