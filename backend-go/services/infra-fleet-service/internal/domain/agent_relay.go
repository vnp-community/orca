package domain

import "errors"

// ErrAgentMethodNotFound is returned by DevServerAgentClient.Exec when the
// resolved Dev Server Agent's JSON-RPC dispatcher rejects the call with the
// standard JSON-RPC "method not found" response (code -32601) — the agent
// is reachable and answered, it simply does not implement the requested
// method on this build, as opposed to a connection/transport failure.
//
// Shared by every "relay to a capability agent/ doesn't have yet" usecase
// (TASK-048's emulator.*, TASK-070's host.capabilities) rather than one
// sentinel per capability: the shape every caller needs is identical
// (errors.Is this, then translate to an apperrors.KindFailedPrecondition
// naming the missing capability), so one sentinel keeps that translation
// in the usecase layer instead of multiplying near-identical domain errors.
// Mirrors git-gateway-service's domain.ErrForceDeleteBranchUnsupported /
// grpcclient.isMethodNotFoundError pattern, but detected from the real
// JSON-RPC error code (see devserveragent.JSONRPCError.Code) rather than a
// string heuristic, since this service's adapter has direct access to it.
var ErrAgentMethodNotFound = errors.New("infra-fleet-service: dev server agent does not implement the requested method")
