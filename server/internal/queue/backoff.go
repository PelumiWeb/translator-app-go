package queue

import (
	"math/rand/v2"
	"time"
)

// ExponentialBackoff returns a function that says how long to wait before
// retrying after the given attempt: base after the first, twice that after
// the second, and so on, never more than max.
//
// Each wait is then moved by up to 20% either way (jitter). Without it, every
// job that failed in the same outage would retry at the same instant and hit
// the recovering provider together.
func ExponentialBackoff(base, max time.Duration) func(attempt int) time.Duration {
	return func(attempt int) time.Duration {
		delay := base
		for i := 1; i < attempt && delay < max; i++ {
			delay *= 2
		}
		if delay > max {
			delay = max
		}
		jitter := 0.8 + 0.4*rand.Float64() // a factor from 0.8 to 1.2
		return time.Duration(float64(delay) * jitter)
	}
}
