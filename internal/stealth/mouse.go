package stealth

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// Mouse implements human-like mouse movement using Bézier curves
type Mouse struct {
	config *MouseConfig
	rng    *rand.Rand
}

// MouseConfig holds configuration for mouse behavior
type MouseConfig struct {
	SpeedMin      float64 // Minimum speed multiplier
	SpeedMax      float64 // Maximum speed multiplier
	OvershootChance float64 // Probability of overshooting target (0.0-1.0)
}

// NewMouse creates a new Mouse instance
func NewMouse(config *MouseConfig) *Mouse {
	return &Mouse{
		config: config,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Point represents a 2D coordinate
type Point struct {
	X, Y float64
}

// MoveMouse validates context for mouse movement
// Actual movement is handled by browser layer using GetPath()
func (m *Mouse) MoveMouse(ctx context.Context, startX, startY, endX, endY float64) error {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// generateControlPoints generates control points for a cubic Bézier curve
// Returns: [P0, P1, P2, P3] where P0 is start and P3 is end
func (m *Mouse) generateControlPoints(start, end Point) []Point {
	// Calculate perpendicular vector for curve
	dx := end.X - start.X
	dy := end.Y - start.Y
	perpendicularX := -dy
	perpendicularY := dx

	// Normalize and scale perpendicular vector
	perpLength := math.Sqrt(perpendicularX*perpendicularX + perpendicularY*perpendicularY)
	if perpLength > 0 {
		scale := (m.rng.Float64()*0.3 + 0.2) * math.Sqrt(dx*dx+dy*dy) // 20-50% of distance
		perpendicularX = (perpendicularX / perpLength) * scale
		perpendicularY = (perpendicularY / perpLength) * scale
	}

	// Generate two control points
	// P1 is offset from start, P2 is offset from end
	control1X := start.X + perpendicularX*(0.3+m.rng.Float64()*0.4)
	control1Y := start.Y + perpendicularY*(0.3+m.rng.Float64()*0.4)
	
	control2X := end.X - perpendicularX*(0.3+m.rng.Float64()*0.4)
	control2Y := end.Y - perpendicularY*(0.3+m.rng.Float64()*0.4)

	return []Point{
		{X: start.X, Y: start.Y},           // P0
		{X: control1X, Y: control1Y},       // P1
		{X: control2X, Y: control2Y},       // P2
		{X: end.X, Y: end.Y},               // P3
	}
}

// generateBezierPoints generates points along a cubic Bézier curve
func (m *Mouse) generateBezierPoints(controlPoints []Point, steps int) []Point {
	if len(controlPoints) != 4 {
		panic("cubic Bézier requires 4 control points")
	}

	points := make([]Point, steps)
	p0, p1, p2, p3 := controlPoints[0], controlPoints[1], controlPoints[2], controlPoints[3]

	for i := 0; i < steps; i++ {
		t := float64(i) / float64(steps-1)
		
		// Cubic Bézier formula: B(t) = (1-t)³P₀ + 3(1-t)²tP₁ + 3(1-t)t²P₂ + t³P₃
		mt := 1 - t
		mt2 := mt * mt
		mt3 := mt2 * mt
		t2 := t * t
		t3 := t2 * t

		x := mt3*p0.X + 3*mt2*t*p1.X + 3*mt*t2*p2.X + t3*p3.X
		y := mt3*p0.Y + 3*mt2*t*p1.Y + 3*mt*t2*p2.Y + t3*p3.Y

		points[i] = Point{X: x, Y: y}
	}

	return points
}

// easeInOutCubic provides easing function for variable speed
// Returns value between 0 and 1, with slower start and end
func (m *Mouse) easeInOutCubic(t float64) float64 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	return 1 - math.Pow(-2*t+2, 3)/2
}


// GetPath returns the path points for external use (for browser layer to execute)
func (m *Mouse) GetPath(startX, startY, endX, endY float64, shouldOvershoot bool) []Point {
	distance := math.Sqrt(math.Pow(endX-startX, 2) + math.Pow(endY-startY, 2))
	if distance < 1.0 {
		return []Point{{X: endX, Y: endY}}
	}

	var finalTarget Point
	var overshootTarget Point

	if shouldOvershoot && m.rng.Float64() < m.config.OvershootChance {
		overshootDistance := distance * (0.1 + m.rng.Float64()*0.2)
		angle := math.Atan2(endY-startY, endX-startX)
		overshootTarget = Point{
			X: endX + overshootDistance*math.Cos(angle),
			Y: endY + overshootDistance*math.Sin(angle),
		}
		finalTarget = Point{X: endX, Y: endY}
	} else {
		finalTarget = Point{X: endX, Y: endY}
		overshootTarget = finalTarget
	}

	controlPoints := m.generateControlPoints(
		Point{X: startX, Y: startY},
		overshootTarget,
	)

	speedMultiplier := m.config.SpeedMin + m.rng.Float64()*(m.config.SpeedMax-m.config.SpeedMin)
	steps := int(distance / (10.0 * speedMultiplier))
	if steps < 10 {
		steps = 10
	}
	if steps > 100 {
		steps = 100
	}

	points := m.generateBezierPoints(controlPoints, steps)

	if shouldOvershoot && overshootTarget.X != finalTarget.X || overshootTarget.Y != finalTarget.Y {
		correctionPoints := m.generateControlPoints(overshootTarget, finalTarget)
		correctionSteps := int(distance * 0.2)
		if correctionSteps < 5 {
			correctionSteps = 5
		}
		correctionPath := m.generateBezierPoints(correctionPoints, correctionSteps)
		points = append(points, correctionPath...)
	}

	return points
}

