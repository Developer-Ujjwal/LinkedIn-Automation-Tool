package stealth

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// Constants for mouse movement physics
const (
	minDistanceForMovement = 1.0
	minSteps               = 10
	maxSteps               = 100
	stepDivisor            = 10.0
	correctionStepFactor   = 0.2
	minCorrectionSteps     = 5
)

// Mouse implements human-like mouse movement using Bézier curves
type Mouse struct {
	config *MouseConfig
	rng    *rand.Rand
}

// MouseConfig holds configuration for mouse behavior
type MouseConfig struct {
	SpeedMin              float64 // Minimum speed multiplier
	SpeedMax              float64 // Maximum speed multiplier
	OvershootChance       float64 // Probability of overshooting target (0.0-1.0)
	OvershootDistMin      float64 // Min overshoot distance factor
	OvershootDistMax      float64 // Max overshoot distance factor
	ControlPointOffsetMin float64 // Min control point offset
	ControlPointOffsetMax float64 // Max control point offset
	ControlPointSpreadMin float64 // Min control point spread
	ControlPointSpreadMax float64 // Max control point spread
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
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// GetPath generates a human-like mouse path using Bézier curves
func (m *Mouse) GetPath(startX, startY, endX, endY float64, shouldOvershoot bool) []Point {
	start := Point{X: startX, Y: startY}
	end := Point{X: endX, Y: endY}
	
	distance := m.calculateDistance(start, end)
	if distance < minDistanceForMovement {
		return []Point{end}
	}

	// 1. Determine targets (main target vs overshoot target)
	target, overshootTarget := m.determineTargets(start, end, distance, shouldOvershoot)

	// 2. Generate main path
	points := m.generateCurvePath(start, overshootTarget, distance)

	// 3. Generate correction path if we overshot
	if m.hasOvershot(target, overshootTarget) {
		correctionPath := m.generateCorrectionPath(overshootTarget, target, distance)
		points = append(points, correctionPath...)
	}

	return points
}

// calculateDistance returns Euclidean distance between two points
func (m *Mouse) calculateDistance(p1, p2 Point) float64 {
	return math.Sqrt(math.Pow(p2.X-p1.X, 2) + math.Pow(p2.Y-p1.Y, 2))
}

// determineTargets calculates the final target and potential overshoot target
func (m *Mouse) determineTargets(start, end Point, distance float64, shouldOvershoot bool) (final Point, overshoot Point) {
	final = end
	overshoot = end

	if shouldOvershoot && m.rng.Float64() < m.config.OvershootChance {
		overshootFactor := m.config.OvershootDistMin + m.rng.Float64()*(m.config.OvershootDistMax-m.config.OvershootDistMin)
		overshootDist := distance * overshootFactor
		angle := math.Atan2(end.Y-start.Y, end.X-start.X)
		
		overshoot = Point{
			X: end.X + overshootDist*math.Cos(angle),
			Y: end.Y + overshootDist*math.Sin(angle),
		}
	}
	return final, overshoot
}

// hasOvershot checks if the overshoot target differs from the final target
func (m *Mouse) hasOvershot(final, overshoot Point) bool {
	return final.X != overshoot.X || final.Y != overshoot.Y
}

// generateCurvePath creates the points for a single Bézier curve segment
func (m *Mouse) generateCurvePath(start, end Point, totalDistance float64) []Point {
	controlPoints := m.generateControlPoints(start, end)
	steps := m.calculateSteps(totalDistance)
	return m.generateBezierPoints(controlPoints, steps)
}

// generateCorrectionPath creates the path from overshoot point back to real target
func (m *Mouse) generateCorrectionPath(start, end Point, originalDistance float64) []Point {
	controlPoints := m.generateControlPoints(start, end)
	steps := int(originalDistance * correctionStepFactor)
	if steps < minCorrectionSteps {
		steps = minCorrectionSteps
	}
	return m.generateBezierPoints(controlPoints, steps)
}

// calculateSteps determines the number of steps based on distance and speed
func (m *Mouse) calculateSteps(distance float64) int {
	speedMultiplier := m.config.SpeedMin + m.rng.Float64()*(m.config.SpeedMax-m.config.SpeedMin)
	steps := int(distance / (stepDivisor * speedMultiplier))
	
	if steps < minSteps {
		return minSteps
	}
	if steps > maxSteps {
		return maxSteps
	}
	return steps
}

// generateControlPoints generates control points for a cubic Bézier curve
// Returns: [P0, P1, P2, P3] where P0 is start and P3 is end
func (m *Mouse) generateControlPoints(start, end Point) []Point {
	// Calculate perpendicular vector for curve
	dx := end.X - start.X
	dy := end.Y - start.Y
	
	// Perpendicular vector (-y, x)
	perpX := -dy
	perpY := dx

	// Normalize and scale perpendicular vector
	perpLength := math.Sqrt(perpX*perpX + perpY*perpY)
	if perpLength > 0 {
		// Random scale for the curve arc
		scale := (m.rng.Float64()*(m.config.ControlPointOffsetMax-m.config.ControlPointOffsetMin) + m.config.ControlPointOffsetMin) * math.Sqrt(dx*dx+dy*dy)
		perpX = (perpX / perpLength) * scale
		perpY = (perpY / perpLength) * scale
	}

	// Generate two control points with random spread
	// P1 is offset from start, P2 is offset from end
	spread1 := m.config.ControlPointSpreadMin + m.rng.Float64()*(m.config.ControlPointSpreadMax-m.config.ControlPointSpreadMin)
	spread2 := m.config.ControlPointSpreadMin + m.rng.Float64()*(m.config.ControlPointSpreadMax-m.config.ControlPointSpreadMin)

	control1 := Point{
		X: start.X + perpX*spread1,
		Y: start.Y + perpY*spread1,
	}

	control2 := Point{
		X: end.X - perpX*spread2,
		Y: end.Y - perpY*spread2,
	}

	return []Point{start, control1, control2, end}
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
		
		// Apply easing for human-like acceleration/deceleration
		easedT := m.easeInOutCubic(t)
		
		points[i] = m.cubicBezier(p0, p1, p2, p3, easedT)
	}

	return points
}

// cubicBezier calculates a point on the curve for time t
func (m *Mouse) cubicBezier(p0, p1, p2, p3 Point, t float64) Point {
	mt := 1 - t
	mt2 := mt * mt
	mt3 := mt2 * mt
	t2 := t * t
	t3 := t2 * t

	// B(t) = (1-t)³P0 + 3(1-t)²tP1 + 3(1-t)t²P2 + t³P3
	x := mt3*p0.X + 3*mt2*t*p1.X + 3*mt*t2*p2.X + t3*p3.X
	y := mt3*p0.Y + 3*mt2*t*p1.Y + 3*mt*t2*p2.Y + t3*p3.Y

	return Point{X: x, Y: y}
}

// easeInOutCubic provides easing function for variable speed
// Returns value between 0 and 1, with slower start and end
func (m *Mouse) easeInOutCubic(t float64) float64 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	return 1 - math.Pow(-2*t+2, 3)/2
}
