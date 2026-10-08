// Package stubs fakes the services request-service reaches over gRPC so the end-to-end tests exercise the real
// binary with no AI and no dev server. The seam is the dev-server relay of infra-fleet-service: request-service
// asks the agent for ai.complete (and, from the execution CRs, agent.execPrompt) through Relay, so answering
// Relay here is how the "agent" is stubbed. Answers are deterministic and driven by markers in the request text.
package stubs
