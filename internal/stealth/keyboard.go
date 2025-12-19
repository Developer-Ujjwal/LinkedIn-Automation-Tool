package stealth

import (
	"context"
	"math/rand"
	"time"
)

// Keyboard implements human-like typing with variable speed and typos
type Keyboard struct {
	rng *rand.Rand
}

// NewKeyboard creates a new Keyboard instance
func NewKeyboard() *Keyboard {
	return &Keyboard{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// HumanType simulates human typing with:
// - Variable WPM (words per minute)
// - Occasional typos (with probability typoProb)
// - Backspace and correction after typos
// - Natural delays between keystrokes
func (k *Keyboard) HumanType(ctx context.Context, text string, wpmMin, wpmMax int, typoProb float64) ([]KeyAction, error) {
	if wpmMin < 1 {
		wpmMin = 1
	}
	if wpmMax < wpmMin {
		wpmMax = wpmMin
	}
	if typoProb < 0 {
		typoProb = 0
	}
	if typoProb > 1 {
		typoProb = 1
	}

	actions := make([]KeyAction, 0)
	textRunes := []rune(text)
	
	// Calculate base delay per character based on WPM
	// Average word length is 5 characters + 1 space = 6 characters
	// WPM = (characters / 6) / (minutes)
	// So delay per character = (60 seconds / WPM) / 6
	wpm := wpmMin + k.rng.Intn(wpmMax-wpmMin+1)
	baseDelayPerChar := (60.0 / float64(wpm)) / 6.0 // seconds per character

	i := 0
	for i < len(textRunes) {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return actions, ctx.Err()
		default:
		}

		char := textRunes[i]
		
		// Decide if we should make a typo
		shouldTypo := k.rng.Float64() < typoProb
		
		if shouldTypo && i < len(textRunes)-1 {
			// Generate a typo: replace character with a nearby key
			typoChar := k.generateTypo(char)
			
			// Type the typo
			actions = append(actions, KeyAction{
				Type:      ActionTypeKey,
				Key:       string(typoChar),
				Delay:     k.calculateDelay(baseDelayPerChar, char),
			})
			
			// Small pause (humans notice typos quickly)
			actions = append(actions, KeyAction{
				Type:  ActionTypeDelay,
				Delay: time.Duration(100+k.rng.Intn(200)) * time.Millisecond,
			})
			
			// Backspace
			actions = append(actions, KeyAction{
				Type:  ActionTypeKey,
				Key:   "\b", // Backspace
				Delay: k.calculateDelay(baseDelayPerChar, '\b'),
			})
			
			// Type correct character
			actions = append(actions, KeyAction{
				Type:  ActionTypeKey,
				Key:   string(char),
				Delay: k.calculateDelay(baseDelayPerChar, char),
			})
		} else {
			// Type normally
			actions = append(actions, KeyAction{
				Type:  ActionTypeKey,
				Key:   string(char),
				Delay: k.calculateDelay(baseDelayPerChar, char),
			})
		}
		
		i++
	}

	return actions, nil
}

// KeyAction represents a single keyboard action
type KeyAction struct {
	Type  ActionType      // Type of action
	Key   string          // Key to press (for ActionTypeKey)
	Delay time.Duration   // Delay after this action
}

// ActionType represents the type of keyboard action
type ActionType int

const (
	ActionTypeKey ActionType = iota
	ActionTypeDelay
)

// generateTypo generates a typo character based on the intended character
// Uses QWERTY keyboard layout proximity
func (k *Keyboard) generateTypo(char rune) rune {
	// QWERTY keyboard layout (simplified)
	keyboardLayout := map[rune][]rune{
		'a': {'s', 'q', 'w', 'z', 'x'},
		'b': {'v', 'g', 'h', 'n'},
		'c': {'x', 'd', 'f', 'v'},
		'd': {'s', 'e', 'r', 'f', 'c', 'x'},
		'e': {'w', 'r', 'd', 's'},
		'f': {'d', 'r', 't', 'g', 'v', 'c'},
		'g': {'f', 't', 'y', 'h', 'b', 'v'},
		'h': {'g', 'y', 'u', 'j', 'n', 'b'},
		'i': {'u', 'o', 'k', 'j'},
		'j': {'h', 'u', 'i', 'k', 'm', 'n'},
		'k': {'j', 'i', 'o', 'l', ',', 'm'},
		'l': {'k', 'o', 'p', ';', '.', ','},
		'm': {'n', 'j', 'k', ','},
		'n': {'b', 'h', 'j', 'm'},
		'o': {'i', 'p', 'l', 'k'},
		'p': {'o', '[', ']', 'l', ';'},
		'q': {'w', 'a'},
		'r': {'e', 't', 'f', 'd'},
		's': {'a', 'w', 'e', 'd', 'x', 'z'},
		't': {'r', 'y', 'g', 'f'},
		'u': {'y', 'i', 'j', 'h'},
		'v': {'c', 'f', 'g', 'b'},
		'w': {'q', 'e', 's', 'a'},
		'x': {'z', 's', 'd', 'c'},
		'y': {'t', 'u', 'h', 'g'},
		'z': {'a', 's', 'x'},
	}

	// Convert to lowercase for lookup
	charLower := char
	if char >= 'A' && char <= 'Z' {
		charLower = char + 32
	}

	// Get nearby keys
	if nearby, ok := keyboardLayout[charLower]; ok && len(nearby) > 0 {
		typoRune := nearby[k.rng.Intn(len(nearby))]
		// Preserve case
		if char >= 'A' && char <= 'Z' {
			typoRune = typoRune - 32
		}
		return typoRune
	}

	// Fallback: return a random character if not found in layout
	// For non-letter characters, return a similar character
	if char == ' ' {
		return 'x' // Common typo for space
	}
	if char >= '0' && char <= '9' {
		// Adjacent number
		if char == '0' {
			return '9'
		}
		return char - 1
	}

	// Default: return the same character (no typo possible)
	return char
}

// calculateDelay calculates the delay for typing a character
// Adds natural variation based on character type
func (k *Keyboard) calculateDelay(baseDelay float64, char rune) time.Duration {
	// Base delay with small random variation (±20%)
	variance := 0.8 + k.rng.Float64()*0.4
	delay := baseDelay * variance

	// Longer delays for certain characters
	switch char {
	case ' ', '\n', '\t':
		// Spaces and newlines take longer
		delay *= 1.5 + k.rng.Float64()*0.5
	case '.', ',', '!', '?':
		// Punctuation takes longer (thinking time)
		delay *= 1.2 + k.rng.Float64()*0.3
	case '\b':
		// Backspace is quick
		delay *= 0.7 + k.rng.Float64()*0.2
	}

	// Convert to duration (never exact integers - add tiny jitter)
	jitter := k.rng.Float64() * 0.01 // 0-10ms jitter
	delaySeconds := delay + jitter

	return time.Duration(delaySeconds * float64(time.Second))
}

// GetTypingActions is a convenience method that returns actions ready to execute
func (k *Keyboard) GetTypingActions(ctx context.Context, text string, wpmMin, wpmMax int, typoProb float64) ([]KeyAction, error) {
	return k.HumanType(ctx, text, wpmMin, wpmMax, typoProb)
}

