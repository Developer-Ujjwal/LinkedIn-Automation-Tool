package stealth

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// Jitter implements randomized timing that never sleeps for exact integers
type Jitter struct {
	rng *rand.Rand
}

// NewJitter creates a new Jitter instance
func NewJitter() *Jitter {
	return &Jitter{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// RandomSleep sleeps for a randomized duration
// baseSeconds: base delay in seconds
// varianceSeconds: maximum variance to add/subtract (±varianceSeconds)
// CRITICAL: Never sleeps for exact integers - always adds fractional milliseconds
func (j *Jitter) RandomSleep(ctx context.Context, baseSeconds, varianceSeconds float64) {
	if baseSeconds < 0 {
		baseSeconds = 0
	}
	if varianceSeconds < 0 {
		varianceSeconds = 0
	}

	// Calculate random variance (±varianceSeconds)
	variance := (j.rng.Float64() * 2 - 1) * varianceSeconds // Range: [-varianceSeconds, +varianceSeconds]
	
	// Total delay
	totalSeconds := baseSeconds + variance
	
	// Ensure minimum delay
	if totalSeconds < 0.001 {
		totalSeconds = 0.001
	}

	// Convert to duration
	// Add tiny fractional jitter to ensure never exact integer milliseconds
	fractionalJitter := j.rng.Float64() * 0.0001 // 0-0.1ms additional jitter
	totalSeconds += fractionalJitter
	
	duration := time.Duration(totalSeconds * float64(time.Second))

	// Sleep with context support
	select {
	case <-ctx.Done():
		return
	case <-time.After(duration):
		return
	}
}

// RandomSleepRange sleeps for a random duration between min and max seconds
func (j *Jitter) RandomSleepRange(ctx context.Context, minSeconds, maxSeconds float64) {
	if minSeconds < 0 {
		minSeconds = 0
	}
	if maxSeconds < minSeconds {
		maxSeconds = minSeconds
	}

	// Random value between min and max
	randomSeconds := minSeconds + j.rng.Float64()*(maxSeconds-minSeconds)
	
	// Add fractional jitter to ensure never exact integer
	fractionalJitter := j.rng.Float64() * 0.0001
	randomSeconds += fractionalJitter
	
	duration := time.Duration(randomSeconds * float64(time.Second))

	select {
	case <-ctx.Done():
		return
	case <-time.After(duration):
		return
	}
}

// RandomInt returns a random integer between min and max (inclusive)
// with optional jitter to avoid patterns
func (j *Jitter) RandomInt(min, max int) int {
	if min > max {
		min, max = max, min
	}
	if min == max {
		return min
	}
	return min + j.rng.Intn(max-min+1)
}

// RandomFloat returns a random float64 between min and max
func (j *Jitter) RandomFloat(min, max float64) float64 {
	if min > max {
		min, max = max, min
	}
	return min + j.rng.Float64()*(max-min)
}

// GaussianDelay returns a delay sampled from a Gaussian (normal) distribution
// Useful for more natural timing patterns
func (j *Jitter) GaussianDelay(ctx context.Context, meanSeconds, stdDevSeconds float64) {
	if meanSeconds < 0 {
		meanSeconds = 0
	}
	if stdDevSeconds < 0 {
		stdDevSeconds = 0
	}

	// Generate Gaussian random number
	// Box-Muller transform for normal distribution
	u1 := j.rng.Float64()
	u2 := j.rng.Float64()
	z0 := math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
	
	// Scale to desired mean and std dev
	delaySeconds := meanSeconds + z0*stdDevSeconds
	
	// Ensure non-negative
	if delaySeconds < 0.001 {
		delaySeconds = 0.001
	}
	
	// Add fractional jitter
	fractionalJitter := j.rng.Float64() * 0.0001
	delaySeconds += fractionalJitter
	
	duration := time.Duration(delaySeconds * float64(time.Second))

	select {
	case <-ctx.Done():
		return
	case <-time.After(duration):
		return
	}
}

