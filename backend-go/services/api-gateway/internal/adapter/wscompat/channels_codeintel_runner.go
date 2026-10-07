package wscompat

import (
	"context"
	"encoding/json"

	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
	"google.golang.org/protobuf/proto"
)

type codeIntelDeps struct {
	core    codeintelv1.CodeIntelServiceClient
	quality codeintelv1.QualityGateServiceClient
	limits  CodeIntelLimits
}

func (d codeIntelDeps) configured(spec codeIntelChannelSpec) bool {
	if spec.Quality {
		return d.quality != nil
	}
	return d.core != nil
}

func (d codeIntelDeps) caller() codeIntelCaller {
	return codeIntelCaller{
		Core:    d.core,
		Quality: d.quality,
	}
}

type codeIntelCaller struct {
	Core    codeintelv1.CodeIntelServiceClient
	Quality codeintelv1.QualityGateServiceClient
}

// finishCodeIntelResponse checks response size against limits and serializes to JSON.
func finishCodeIntelResponse(spec codeIntelChannelSpec, limits CodeIntelLimits, resp proto.Message, encode func() (json.RawMessage, error)) (any, error) {
	if resp == nil {
		return json.RawMessage("null"), nil
	}
	limit := spec.MaxResponse
	if limit <= 0 {
		limit = limits.MaxResponseBytes
	}
	if limit <= 0 {
		limit = 2 << 20 // 2 MiB default
	}

	size := proto.Size(resp)
	if size > limit {
		return nil, errCodeIntelResponseTooLarge(size, limit)
	}

	if encode != nil {
		return encode()
	}
	return encodeCodeIntelWire(resp)
}

// registerCodeIntelUnary wires an RPC channel into the gateway registry with uniform guards,
// timeouts, and payload limits across all 45 unary code intelligence channels.
func registerCodeIntelUnary[A codeIntelArgs](r *Registry, d codeIntelDeps, name string,
	call func(ctx context.Context, c codeIntelCaller, id Identity, in A) (any, error)) {
	spec := mustCatalogSpec(name)
	r.Register(name, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		// 1. Client configured check: unconfigured service returns UNAVAILABLE immediately
		// so frontend can hide the feature without showing error toasts (UI-API U7/6).
		if !d.configured(spec) {
			return nil, errCodeIntelUnavailable
		}
		// 2. Identity presence check (defensive).
		if id.TenantID == "" || id.UserID == "" {
			return nil, errCodeIntelNotFound
		}
		// 3. Device session check: device tokens are forbidden except for settings.get (UI-API U8).
		if id.DeviceID != "" && !spec.AllowDevice {
			return nil, errCodeIntelNotAuthorized
		}
		// 4. Strict argument decoding and parameter validation (UI-API U1/U3).
		in, err := decodeCodeIntelArgs[A](spec, args)
		if err != nil {
			return nil, err
		}
		// 5. Per-channel timeout: 8s/20s/24s.
		cctx, cancel := context.WithTimeout(ctx, spec.Timeout)
		defer cancel()
		// 6. Invoke downstream service without retry (write channels are non-idempotent;
		// read timeouts return inProgress:true for graceful client-side singleflight polling).
		out, err := call(cctx, d.caller(), id, in)
		if err != nil {
			return nil, codeIntelChannelError(err)
		}
		return out, nil
	})
}
