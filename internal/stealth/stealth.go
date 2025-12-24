package stealth

import (
	"context"

	"linkedin-automation/internal/core"
)

// Stealth implements the StealthPort interface
// It coordinates mouse, keyboard, jitter, and scroll components
type Stealth struct {
	mouse    *Mouse
	keyboard *Keyboard
	jitter   *Jitter
	scroll   *Scroll
	config   *core.StealthConfig
}

// NewStealth creates a new Stealth instance with the given configuration
func NewStealth(config *core.StealthConfig) *Stealth {
	return &Stealth{
		mouse: NewMouse(&MouseConfig{
			SpeedMin:              config.MouseSpeedMin,
			SpeedMax:              config.MouseSpeedMax,
			OvershootChance:       config.OvershootChance,
			OvershootDistMin:      config.OvershootDistMin,
			OvershootDistMax:      config.OvershootDistMax,
			ControlPointOffsetMin: config.ControlPointOffsetMin,
			ControlPointOffsetMax: config.ControlPointOffsetMax,
			ControlPointSpreadMin: config.ControlPointSpreadMin,
			ControlPointSpreadMax: config.ControlPointSpreadMax,
		}),
		keyboard: NewKeyboard(),
		jitter:   NewJitter(),
		scroll:   NewScroll(),
		config:   config,
	}
}

// MoveMouse moves the mouse using Bézier curves with optional overshoot
// Note: This requires a rod.Page instance, which will be provided by the browser layer
// For now, we return the path points that the browser layer can execute
func (s *Stealth) MoveMouse(ctx context.Context, startX, startY, endX, endY float64) error {
	// This is a placeholder - actual mouse movement will be handled by browser layer
	// which has access to rod.Page. The browser layer will call mouse.GetPath() and execute it.
	// We validate the call here but defer execution to browser layer.
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Path generation will be done by browser layer
		return nil
	}
}

// HumanType simulates human typing with variable speed and typos
func (s *Stealth) HumanType(ctx context.Context, text string, wpmMin, wpmMax int, typoProb float64) error {
	// Use config defaults if not provided
	if wpmMin == 0 {
		wpmMin = s.config.TypingSpeedMin
	}
	if wpmMax == 0 {
		wpmMax = s.config.TypingSpeedMax
	}
	if typoProb < 0 {
		typoProb = s.config.TypoProbability
	}

	// Generate typing actions
	_, err := s.keyboard.HumanType(ctx, text, wpmMin, wpmMax, typoProb)
	return err
}

// RandomSleep sleeps for a randomized duration (never exact integers)
func (s *Stealth) RandomSleep(ctx context.Context, baseSeconds, varianceSeconds float64) {
	// Use config defaults if not provided
	if baseSeconds == 0 {
		baseSeconds = s.config.BaseDelayMin
	}
	if varianceSeconds == 0 {
		varianceSeconds = s.config.BaseDelayMax - s.config.BaseDelayMin
	}

	s.jitter.RandomSleep(ctx, baseSeconds, varianceSeconds)
}

// HumanScroll scrolls with acceleration/deceleration and pauses
func (s *Stealth) HumanScroll(ctx context.Context, direction string, distance int, chunkMin, chunkMax int) error {
	// Use config defaults if not provided
	if chunkMin == 0 {
		chunkMin = s.config.ScrollChunkMin
	}
	if chunkMax == 0 {
		chunkMax = s.config.ScrollChunkMax
	}

	_, err := s.scroll.HumanScroll(ctx, direction, distance, chunkMin, chunkMax)
	return err
}

// GetMouse returns the mouse instance (for browser layer to use)
func (s *Stealth) GetMouse() *Mouse {
	return s.mouse
}

// GetKeyboard returns the keyboard instance (for browser layer to use)
func (s *Stealth) GetKeyboard() *Keyboard {
	return s.keyboard
}

// GetJitter returns the jitter instance (for browser layer to use)
func (s *Stealth) GetJitter() *Jitter {
	return s.jitter
}

// GetScroll returns the scroll instance (for browser layer to use)
func (s *Stealth) GetScroll() *Scroll {
	return s.scroll
}

// GetTypingActions returns keyboard actions for a text (for browser layer to execute)
func (s *Stealth) GetTypingActions(ctx context.Context, text string) ([]KeyAction, error) {
	return s.keyboard.HumanType(ctx, text, s.config.TypingSpeedMin, s.config.TypingSpeedMax, s.config.TypoProbability)
}

// GetScrollActions returns scroll actions (for browser layer to execute)
func (s *Stealth) GetScrollActions(ctx context.Context, direction string, distance int) ([]ScrollAction, error) {
	return s.scroll.HumanScroll(ctx, direction, distance, s.config.ScrollChunkMin, s.config.ScrollChunkMax)
}

// GetMousePath returns mouse movement path points (for browser layer to execute)
func (s *Stealth) GetMousePath(startX, startY, endX, endY float64) []Point {
	shouldOvershoot := true // Will be randomized inside GetPath
	return s.mouse.GetPath(startX, startY, endX, endY, shouldOvershoot)
}

