package stealth

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// Scroll implements human-like scrolling with acceleration/deceleration
type Scroll struct {
	rng *rand.Rand
}

// NewScroll creates a new Scroll instance
func NewScroll() *Scroll {
	return &Scroll{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// ScrollAction represents a scroll action
type ScrollAction struct {
	Distance int           // Pixels to scroll
	Delay    time.Duration // Delay after scrolling
}

// HumanScroll generates scroll actions with:
// - Chunked scrolling (not smooth, but in chunks)
// - Acceleration at start, deceleration at end
// - Random pauses between chunks
func (s *Scroll) HumanScroll(ctx context.Context, direction string, distance int, chunkMin, chunkMax int) ([]ScrollAction, error) {
	if distance < 0 {
		distance = -distance
	}
	if chunkMin < 1 {
		chunkMin = 1
	}
	if chunkMax < chunkMin {
		chunkMax = chunkMin
	}

	actions := make([]ScrollAction, 0)
	
	// Determine scroll direction multiplier
	multiplier := 1
	if direction == "up" || direction == "backward" {
		multiplier = -1
	}

	// Calculate number of chunks based on distance
	avgChunkSize := (chunkMin + chunkMax) / 2
	numChunks := int(math.Ceil(float64(distance) / float64(avgChunkSize)))
	if numChunks < 1 {
		numChunks = 1
	}

	remainingDistance := distance
	
	for i := 0; i < numChunks && remainingDistance > 0; i++ {
		// Check context
		select {
		case <-ctx.Done():
			return actions, ctx.Err()
		default:
		}

		// Calculate chunk size with acceleration/deceleration curve
		t := float64(i) / float64(numChunks-1)
		if numChunks == 1 {
			t = 0.5
		}
		
		// Ease-in-out curve: slower at start and end, faster in middle
		easeFactor := s.easeInOutCubic(t)
		
		// Base chunk size with easing
		baseChunkSize := float64(chunkMin) + easeFactor*float64(chunkMax-chunkMin)
		
		// Add random variation
		variation := 0.7 + s.rng.Float64()*0.6 // ±30% variation
		chunkSize := int(baseChunkSize * variation)
		
		// Ensure we don't exceed remaining distance
		if chunkSize > remainingDistance {
			chunkSize = remainingDistance
		}
		
		// Apply direction multiplier
		scrollDistance := chunkSize * multiplier
		
		// Calculate delay based on chunk size and position
		// Larger chunks = longer delay, middle chunks = shorter delay
		baseDelay := 50.0 + float64(chunkSize)*0.5 // Base delay increases with chunk size
		if i == 0 || i == numChunks-1 {
			// Longer delay at start and end (thinking/reading time)
			baseDelay *= 1.5 + s.rng.Float64()*0.5
		} else {
			// Shorter delay in middle (scrolling quickly)
			baseDelay *= 0.7 + s.rng.Float64()*0.3
		}
		
		// Add random jitter (never exact integers)
		jitter := s.rng.Float64() * 20.0 // 0-20ms jitter
		delay := time.Duration(baseDelay+jitter) * time.Millisecond

		actions = append(actions, ScrollAction{
			Distance: scrollDistance,
			Delay:    delay,
		})

		remainingDistance -= chunkSize
	}

	// Add final pause after scrolling (reading time)
	if len(actions) > 0 {
		finalPause := time.Duration(200+s.rng.Intn(300)) * time.Millisecond
		actions = append(actions, ScrollAction{
			Distance: 0,
			Delay:    finalPause,
		})
	}

	return actions, nil
}

// easeInOutCubic provides easing function for acceleration/deceleration
func (s *Scroll) easeInOutCubic(t float64) float64 {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	if t < 0.5 {
		return 4 * t * t * t
	}
	return 1 - math.Pow(-2*t+2, 3)/2
}

// SmoothScroll generates smooth scrolling actions (alternative to chunked)
func (s *Scroll) SmoothScroll(ctx context.Context, direction string, distance int) ([]ScrollAction, error) {
	multiplier := 1
	if direction == "up" || direction == "backward" {
		multiplier = -1
	}

	// Smooth scroll: many small increments
	numSteps := 10 + s.rng.Intn(10) // 10-20 steps
	stepSize := distance / numSteps
	if stepSize < 1 {
		stepSize = 1
	}

	actions := make([]ScrollAction, 0)
	for i := 0; i < numSteps; i++ {
		select {
		case <-ctx.Done():
			return actions, ctx.Err()
		default:
		}

		scrollDistance := stepSize * multiplier
		delay := time.Duration(10+s.rng.Intn(20)) * time.Millisecond

		actions = append(actions, ScrollAction{
			Distance: scrollDistance,
			Delay:    delay,
		})
	}

	return actions, nil
}

