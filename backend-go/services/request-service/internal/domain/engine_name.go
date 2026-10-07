package domain

import (
	"github.com/stablyai/orca-go/common/apperrors"
)

type EngineName string

const (
	EngineNative   EngineName = "native"
	EngineOpenSpec EngineName = "openspec"
)

var (
	ErrEngineInvalid = apperrors.New(apperrors.KindInvalidArgument, "REQUEST_ENGINE_INVALID", "invalid engine name", nil)
)

func ParseEngineName(s string) (EngineName, error) {
	switch EngineName(s) {
	case EngineNative, EngineOpenSpec:
		return EngineName(s), nil
	default:
		return "", ErrEngineInvalid
	}
}

func AllEngineNames() []EngineName {
	return []EngineName{EngineNative, EngineOpenSpec}
}
