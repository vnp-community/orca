// Package sshrelay is relay-ssh mode's deploy+launch+handshake pipeline —
// the piece adapter/sshconn's own doc comment flags as deliberately not
// built there: SFTP-deploying agent/out/agent.js to a domain.SshTarget,
// launching it over the SSH exec channel in `--stdio` mode (a third
// agent-side connection mode, see agent/src/relay/agent-connection-stdio.ts,
// added specifically because relay-ssh's originally-spec'd deploy target —
// a separate `relay.js` binary, launched via `--detached --connect
// --sock-path` — has no buildable artifact anywhere in this repo; agent/'s
// actual, buildable Dev Server Agent only ever supported
// direct-websocket/relay-websocket before this), and running the
// receiver-side agent.handshake exchange that hands a live
// devserveragent.Transport back to Client.getOrProvisionSession.
//
// Scope, deliberately smaller than the TS reference's relay-ssh model (see
// launch.go's doc comment): one exec channel per session, foreground, no
// detach/nohup/Unix-socket-reattach — a dropped SSH connection just ends
// the session; the next call re-provisions from scratch. No multi-platform
// bundle resolution (Config.BundlePath is one local path, this scaffold
// runs one platform). No token/auth beyond the SSH connection itself —
// matches relay-ssh's spec'd trust model exactly ("implicit — trust
// boundary is the SSH connection itself").
package sshrelay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/devserveragent"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshconn"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// SshTargetResolver mirrors usecase.SshTargetResolver — declared here too
// (consumer-side, per this codebase's Dependency Inversion convention)
// rather than importing the usecase package directly, keeping this
// adapter's dependency graph pointed the same direction as its siblings
// (adapter -> domain, never adapter -> usecase). Implemented by
// postgres.SshTargetStore, same Get method as usecase.SshTargetResolver.
type SshTargetResolver interface {
	Get(ctx context.Context, tenantID, id string) (domain.SshTarget, error)
}

// Connector is the narrow slice of sshconn.Connector this package needs —
// declared here for the same Dependency Inversion reason as
// SshTargetResolver, satisfied by *sshconn.Connector as-is.
type Connector interface {
	Connect(ctx context.Context, target domain.SshTarget) (*sshconn.Connection, error)
}

// inboundHandshakeParams mirrors adapter/agentwsserver's identical type —
// the agent's agent.handshake request params, whichever transport it
// arrives over. Not shared as an exported type across packages since it's
// small and each package's copy documents its own transport's context.
type inboundHandshakeParams struct {
	DevServerID  string   `json:"devServerId"`
	Platform     string   `json:"platform"`
	Arch         string   `json:"arch"`
	NodeVersion  string   `json:"nodeVersion"`
	AgentVersion string   `json:"agentVersion"`
	Capabilities []string `json:"capabilities"`
}

// Provisioner implements devserveragent.SshProvisioner.
type Provisioner struct {
	connector Connector
	resolver  SshTargetResolver
	cfg       Config
}

// NewProvisioner builds a Provisioner. connector is typically
// sshconn.NewConnector(...); resolver is typically postgres.SshTargetStore.
func NewProvisioner(connector Connector, resolver SshTargetResolver, cfg Config) *Provisioner {
	return &Provisioner{connector: connector, resolver: resolver, cfg: cfg}
}

// Provision resolves devServer.SSHTargetID, dials it, deploys+launches
// agent.js --stdio, and completes the receiver-side handshake — see the
// package doc comment for the full pipeline. Any failure at any step
// closes whatever was opened so far; nothing is leaked on the error path.
func (p *Provisioner) Provision(ctx context.Context, devServer domain.DevServer) (devserveragent.Transport, devserveragent.HandshakeInfo, error) {
	if devServer.SSHTargetID == "" {
		return nil, devserveragent.HandshakeInfo{}, fmt.Errorf("sshrelay: dev server %q has no ssh_target_id", devServer.ID)
	}
	target, err := p.resolver.Get(ctx, devServer.TenantID, devServer.SSHTargetID)
	if err != nil {
		return nil, devserveragent.HandshakeInfo{}, fmt.Errorf("sshrelay: resolving ssh target %q: %w", devServer.SSHTargetID, err)
	}

	conn, err := p.connector.Connect(ctx, target)
	if err != nil {
		return nil, devserveragent.HandshakeInfo{}, fmt.Errorf("sshrelay: dialing ssh target %q: %w", devServer.SSHTargetID, err)
	}
	// BR-SSH-03: a plain SSH transport keepalive so a silently-dropped TCP
	// connection is detected promptly. Started against a Provisioner-lifetime
	// context (not ctx) since conn outlives this Provision call.
	conn.StartKeepAlive(context.Background(), 30*time.Second)

	// BR-SSH-07: skip the SFTP upload entirely when the already-deployed
	// bundle's AGENT_VERSION matches this backend's OrcaVersion. A
	// version-probe failure (verr != nil) or version mismatch falls through
	// to deployWithRetry as before — deploying is always the safe default,
	// skipping it is the optimization.
	if version, present, verr := remoteVersionAndPresence(ctx, conn); verr == nil && present && version == p.cfg.OrcaVersion {
		// already current — no deploy needed
	} else if _, derr := deployWithRetry(ctx, conn, p.cfg); derr != nil {
		_ = conn.Close()
		return nil, devserveragent.HandshakeInfo{}, derr
	}

	transport, sockPath, stderrBuf, err := launch(ctx, conn, remoteDir, devServer.ID)
	if err != nil {
		_ = conn.Close()
		return nil, devserveragent.HandshakeInfo{}, err
	}

	info, err := p.receiveHandshake(ctx, transport)
	if err != nil {
		_ = transport.Close("handshake failed")
		diag := collectDiagnostics(ctx, conn, stderrBuf)
		return nil, devserveragent.HandshakeInfo{}, fmt.Errorf("%w\n%s", err, diag)
	}
	// SockPath (SOL-SSH-03) is cached on the resulting *session so
	// relaySSHReconnect can call Reattach again later without re-resolving
	// the SshTarget or re-deploying — see devserveragent.HandshakeInfo's doc
	// comment.
	info.SockPath = sockPath

	return transport, info, nil
}

// Reattach re-dials devServer's SshTarget and bridges onto the already-
// running detached relay process at sockPath — the cheap path
// relaySSHReconnect takes on every retry after the first. Returns
// ErrDetachedProcessGone (wrapped) when the detached process itself is no
// longer alive, the caller's cue to fall back to a full Provision instead.
func (p *Provisioner) Reattach(ctx context.Context, devServer domain.DevServer, sockPath string) (devserveragent.Transport, error) {
	target, err := p.resolver.Get(ctx, devServer.TenantID, devServer.SSHTargetID)
	if err != nil {
		return nil, fmt.Errorf("sshrelay: resolving ssh target %q: %w", devServer.SSHTargetID, err)
	}
	conn, err := p.connector.Connect(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("sshrelay: dialing ssh target %q: %w", devServer.SSHTargetID, err)
	}
	conn.StartKeepAlive(context.Background(), 30*time.Second)
	transport, _, _, err := reattach(ctx, conn, remoteDir, sockPath)
	if err != nil {
		_ = conn.Close()
		if errors.Is(err, ErrDetachedProcessGone) {
			// devserveragent must not import adapter/sshrelay (wrong
			// dependency direction) — wrap in its own local sentinel so
			// session.relaySSHReconnect can detect this cause via
			// errors.Is(err, devserveragent.ErrRelayDetachedProcessGone)
			// without depending on this package's error type.
			return nil, fmt.Errorf("%w: %w", devserveragent.ErrRelayDetachedProcessGone, err)
		}
		return nil, err
	}
	return transport, nil
}

// receiveHandshake waits for the launched agent's agent.handshake request
// and replies {ok:true, orcaVersion, sessionId} — the receiver side of the
// same exchange devserveragent's session.go runs as INITIATOR for
// relay-websocket, and adapter/agentwsserver runs as receiver (with a token
// check this has none of — the SSH connection is already the trust
// boundary by the time this runs). No dedicated Handshake frame type
// exists for this exchange (unlike relay-ssh's originally-spec'd Stack B
// protocol) — this is a plain Regular-framed JSON-RPC agent.handshake,
// identical in shape to the other two modes', since agent-session.ts (the
// shared TS code all three agent-side modes now use) sends the same
// request regardless of transport.
func (p *Provisioner) receiveHandshake(ctx context.Context, t devserveragent.Transport) (devserveragent.HandshakeInfo, error) {
	hctx, cancel := context.WithTimeout(ctx, p.cfg.HandshakeTimeout)
	defer cancel()

	frame, err := t.ReadFrame(hctx)
	if err != nil {
		return devserveragent.HandshakeInfo{}, fmt.Errorf("sshrelay: waiting for agent.handshake: %w", err)
	}
	if frame.Type != devserveragent.MessageTypeRegular {
		return devserveragent.HandshakeInfo{}, fmt.Errorf("sshrelay: protocol violation: first frame is not a regular JSON-RPC frame")
	}

	var req devserveragent.JSONRPCRequest
	if err := json.Unmarshal(frame.Payload, &req); err != nil || req.Method != "agent.handshake" {
		return devserveragent.HandshakeInfo{}, fmt.Errorf("sshrelay: protocol violation: first message must be agent.handshake")
	}

	var params inboundHandshakeParams
	if len(req.Params) > 0 {
		_ = json.Unmarshal(req.Params, &params) // best-effort, matching the other two modes' per-field fallback tolerance
	}

	sessionID := fmt.Sprintf("sess-%d", time.Now().UnixMilli())
	result, err := json.Marshal(map[string]any{"ok": true, "orcaVersion": p.cfg.OrcaVersion, "sessionId": sessionID})
	if err != nil {
		return devserveragent.HandshakeInfo{}, err
	}
	resp := devserveragent.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
	respFrame, err := devserveragent.EncodeJSONRPCFrame(resp, 1, frame.ID)
	if err != nil {
		return devserveragent.HandshakeInfo{}, err
	}
	if err := t.WriteFrame(hctx, respFrame); err != nil {
		return devserveragent.HandshakeInfo{}, fmt.Errorf("sshrelay: sending handshake ack: %w", err)
	}

	return devserveragent.HandshakeInfo{
		Platform:     firstNonEmpty(params.Platform, "linux"),
		Arch:         firstNonEmpty(params.Arch, "x64"),
		NodeVersion:  firstNonEmpty(params.NodeVersion, "unknown"),
		AgentVersion: firstNonEmpty(params.AgentVersion, "unknown"),
		SessionID:    sessionID,
		Capabilities: params.Capabilities,
	}, nil
}

func firstNonEmpty(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
