package tools

import "context"

type RateLimiter struct {}

func (l *RateLimiter) Allow(ctx context.Context, tenantID string) bool {
	return true
}
