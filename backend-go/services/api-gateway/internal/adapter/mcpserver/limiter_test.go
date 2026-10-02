package mcpserver_test

import "github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

// denyAllLimiter: burst 1 consumed immediately would still allow one call, so
// use a zero-burst limiter that never admits.
func denyAllLimiter() *usecase.RateLimiter { return usecase.NewRateLimiter(0.0001, 0) }
