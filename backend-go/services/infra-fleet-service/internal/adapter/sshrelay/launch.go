package sshrelay

import (
	"fmt"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshconn"
)

// launch opens a fresh SSH exec session over conn and starts
// `node agent.js --stdio` in remoteDir (foreground, tied to this exec
// channel — no detach/nohup/Unix-socket reattach model, unlike the TS
// reference's relay.js daemon; see package doc comment on why this pass's
// scope is deliberately the simpler one-shot model). DEV_SERVER_ID is
// passed inline in the command rather than via session.Setenv, since many
// sshd configs reject arbitrary SetEnv requests via AcceptEnv restrictions
// and this needs to work without assuming the target's sshd_config allows
// it.
func launch(conn *sshconn.Connection, remoteDir, devServerID string) (*sshExecTransport, error) {
	session, err := conn.NewSession()
	if err != nil {
		return nil, fmt.Errorf("sshrelay: opening launch session: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		return nil, fmt.Errorf("sshrelay: opening stdin pipe: %w", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		_ = session.Close()
		return nil, fmt.Errorf("sshrelay: opening stdout pipe: %w", err)
	}

	cmd := fmt.Sprintf(
		"cd %s && DEV_SERVER_ID=%s node %s --stdio",
		shellQuote(remoteDir), shellQuote(devServerID), shellQuote(remoteAgentFile),
	)
	if err := session.Start(cmd); err != nil {
		_ = session.Close()
		return nil, fmt.Errorf("sshrelay: starting relay process: %w", err)
	}

	return newSSHExecTransport(conn, session, stdin, stdout), nil
}
