package infrafleetclient

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/usecase"
)

// decodeRawCodeIntelResult decodes an agent result JSON string into a RawCodeIntelResult.
// It detects embedded JSON-RPC error envelopes and validates payload size and presence of data.
func decodeRawCodeIntelResult(resultJSON string, maxBytes int64) (usecase.RawCodeIntelResult, error) {
	if int64(len(resultJSON)) > maxBytes {
		return usecase.RawCodeIntelResult{}, apperrors.New(
			apperrors.KindFailedPrecondition,
			"CODEINTEL_OUTPUT_TOO_LARGE",
			fmt.Sprintf("result size %d exceeds limit %d", len(resultJSON), maxBytes),
			nil,
		)
	}

	// 1. Detect JSON-RPC error envelope embedded in result_json (the relay bug pattern)
	var envelope struct {
		Error *struct {
			Code    json.Number `json:"code"`
			Message string      `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &envelope); err == nil && envelope.Error != nil {
		return usecase.RawCodeIntelResult{}, apperrors.New(
			apperrors.KindInternal,
			"CODEINTEL_TOOL_FAILED",
			fmt.Sprintf("agent error envelope: %s (%s)", envelope.Error.Message, envelope.Error.Code),
			nil,
		)
	}

	// 2. Decode RawCodeIntelResult with UseNumber
	var raw struct {
		Sources    []string        `json:"sources"`
		HeadCommit string          `json:"headCommit"`
		Stale      bool            `json:"stale"`
		Truncated  bool            `json:"truncated"`
		TotalCount *int64          `json:"totalCount"`
		Warnings   []string        `json:"warnings"`
		Perf       map[string]any  `json:"perf"`
		Data       json.RawMessage `json:"data"`
	}
	dec := json.NewDecoder(strings.NewReader(resultJSON))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return usecase.RawCodeIntelResult{}, apperrors.New(
			apperrors.KindInternal,
			"CODEINTEL_RESULT_INVALID",
			"failed to decode agent result",
			err,
		)
	}

	if len(raw.Data) == 0 {
		return usecase.RawCodeIntelResult{}, apperrors.New(
			apperrors.KindInternal,
			"CODEINTEL_RESULT_INVALID",
			"agent result missing data field",
			nil,
		)
	}

	return usecase.RawCodeIntelResult{
		Sources:    raw.Sources,
		HeadCommit: raw.HeadCommit,
		Stale:      raw.Stale,
		Truncated:  raw.Truncated,
		TotalCount: raw.TotalCount,
		Warnings:   raw.Warnings,
		Perf:       raw.Perf,
		Data:       raw.Data,
	}, nil
}
