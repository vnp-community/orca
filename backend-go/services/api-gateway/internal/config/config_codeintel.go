package config

import (
	"fmt"

	"github.com/stablyai/orca-go/common/config"
)

// CodeIntelConfig holds settings for dialing code-intel-service.
type CodeIntelConfig struct {
	ServiceAddr      string
	MaxResponseBytes int
	MaxStreams       int
}

func loadCodeIntel() (CodeIntelConfig, error) {
	addr := config.StringEnv("CODE_INTEL_SERVICE_ADDR", "")
	
	maxRespBytes, err := intEnv("CODE_INTEL_MAX_RESPONSE_BYTES", 2<<20)
	if err != nil {
		return CodeIntelConfig{}, err
	}
	if maxRespBytes < 64<<10 || maxRespBytes > 3<<20 {
		return CodeIntelConfig{}, fmt.Errorf("CODE_INTEL_MAX_RESPONSE_BYTES invalid")
	}

	maxStreams, err := intEnv("CODE_INTEL_MAX_STREAMS", 500)
	if err != nil {
		return CodeIntelConfig{}, err
	}
	if maxStreams < 1 || maxStreams > 10000 {
		return CodeIntelConfig{}, fmt.Errorf("CODE_INTEL_MAX_STREAMS invalid")
	}

	return CodeIntelConfig{
		ServiceAddr:      addr,
		MaxResponseBytes: maxRespBytes,
		MaxStreams:       maxStreams,
	}, nil
}
