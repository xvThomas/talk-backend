package ratelimit

import (
	"context"

	"golang.org/x/time/rate"
)

// Limiter enforces a per-minute rate limit on API calls.
type Limiter struct {
	limiter *rate.Limiter
}

// NewLimiter creates a rate limiter.
// perMinute is the maximum number of calls allowed per minute.
func NewLimiter(perMinute int) *Limiter {
	// Steady rate = perMinute/60 req/s, burst = perMinute.
	r := rate.Limit(float64(perMinute) / 60.0)
	return &Limiter{limiter: rate.NewLimiter(r, perMinute)}
}

// Noop returns a limiter that never blocks (for testing).
func Noop() *Limiter {
	return &Limiter{limiter: rate.NewLimiter(rate.Inf, 0)}
}

// Wait blocks until the rate limit allows the request or the context is cancelled.
func (l *Limiter) Wait(ctx context.Context) error {
	return l.limiter.Wait(ctx)
}
