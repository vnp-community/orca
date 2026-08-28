package sshrelay

import (
	"context"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

// BulkProvisioner implements usecase.Provisioner by wrapping *Provisioner —
// a separate type rather than a same-named method on *Provisioner itself,
// since Provisioner.Provision already has a different, fixed signature
// (returns devserveragent.Transport) that devserveragent.SshProvisioner
// requires; Go doesn't allow two same-named methods with different
// signatures on one receiver.
type BulkProvisioner struct {
	inner *Provisioner
}

// NewBulkProvisioner wraps provisioner for use as a usecase.Provisioner —
// typically the same *Provisioner instance devserveragent.Client's
// WithRelaySSH option is configured with.
func NewBulkProvisioner(provisioner *Provisioner) *BulkProvisioner {
	return &BulkProvisioner{inner: provisioner}
}

// Provision runs the full SSH-connect -> prereq-check -> deploy -> handshake
// pipeline via the wrapped Provisioner and translates its result into
// usecase.Provisioner's shape (see that interface's doc comment for the
// prereqsMet-vs-err distinction). Closes whatever Provision opened
// immediately — BulkProvisionFleet doesn't keep a live agent session the
// way devserveragent.Client's normal getOrProvisionSession flow does, it
// just needs to know the outcome.
func (b *BulkProvisioner) Provision(ctx context.Context, devServer domain.DevServer) (usecase.HandshakeInfo, bool, error) {
	transport, info, err := b.inner.Provision(ctx, devServer)
	if err != nil {
		return usecase.HandshakeInfo{}, false, err
	}
	if transport != nil {
		_ = transport.Close("bulk provisioning check complete")
	}
	prereq, _ := b.inner.LastPrereqResult(devServer.ID)
	return usecase.HandshakeInfo{
		Platform:     info.Platform,
		Arch:         info.Arch,
		NodeVersion:  info.NodeVersion,
		AgentVersion: info.AgentVersion,
	}, prereq.Met(), nil
}
