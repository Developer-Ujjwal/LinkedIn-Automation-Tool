package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/go-rod/rod/lib/input"

	"linkedin-automation/internal/core"
	"linkedin-automation/internal/stealth"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	rodstealth "github.com/go-rod/stealth"
	"go.uber.org/zap"
)

// Instance wraps Rod browser with stealth features
type Instance struct {
	browser *rod.Browser
	page    *rod.Page
	stealth *stealth.Stealth
	config  *core.Config
	logger  *zap.Logger
}

// NewInstance creates a new browser instance
func NewInstance(cfg *core.Config, stealthEngine *stealth.Stealth, logger *zap.Logger) *Instance {
	return &Instance{
		stealth: stealthEngine,
		config:  cfg,
		logger:  logger,
	}
}

// Initialize sets up the browser instance with stealth features
func (b *Instance) Initialize(ctx context.Context) error {
	// Launch browser with stealth flags
	l := launcher.New().
		Headless(false). // Set to true for production
		Set("disable-blink-features", "AutomationControlled").
		Set("disable-features", "IsolateOrigins,site-per-process").
		Set("disable-web-security").
		Set("disable-features", "VizDisplayCompositor")

	browserPath, has := launcher.LookPath()
	if has {
		l = l.Bin(browserPath)
	}

	browserURL, err := l.Launch()
	if err != nil {
		return fmt.Errorf("failed to launch browser: %w", err)
	}

	b.browser = rod.New().ControlURL(browserURL)
	if err := b.browser.Connect(); err != nil {
		return fmt.Errorf("failed to connect to browser: %w", err)
	}

	// Create a new page with stealth
	b.page, err = rodstealth.Page(b.browser)
	if err != nil {
		return fmt.Errorf("failed to create stealth page: %w", err)
	}

	// Randomize viewport size
	width := b.config.Stealth.ViewportWidthMin
	if b.config.Stealth.ViewportWidthMax > b.config.Stealth.ViewportWidthMin {
		width = width + rand.Intn(b.config.Stealth.ViewportWidthMax-b.config.Stealth.ViewportWidthMin+1)
	}
	height := b.config.Stealth.ViewportHeightMin
	if b.config.Stealth.ViewportHeightMax > b.config.Stealth.ViewportHeightMin {
		height = height + rand.Intn(b.config.Stealth.ViewportHeightMax-b.config.Stealth.ViewportHeightMin+1)
	}

	// Set viewport using WindowSize
	b.page.MustSetViewport(width, height, 0, false)

	// Inject script to hide webdriver property
	_, err = b.page.Eval(`() => {
		try {
			Object.defineProperty(navigator, 'webdriver', {
				get: () => undefined
			});
		} catch (e) {}
	}`)
	if err != nil {
		b.logger.Debug("Failed to manually hide webdriver property (likely handled by stealth)", zap.Error(err))
	}

	// Randomize User-Agent (optional, Rod handles this)
	b.logger.Info("Browser initialized",
		zap.Int("width", width),
		zap.Int("height", height),
	)

	return nil
}

// Navigate navigates to a URL with human-like delays
func (b *Instance) Navigate(ctx context.Context, url string) error {
	if b.page == nil {
		return fmt.Errorf("browser not initialized")
	}

	// Random delay before navigation
	b.stealth.RandomSleep(ctx, 0.5, 1.0)

	if err := b.page.Navigate(url); err != nil {
		return fmt.Errorf("failed to navigate to %s: %w", url, err)
	}

	// Wait for page load with random delay
	if err := b.page.WaitLoad(); err != nil {
		return fmt.Errorf("failed to wait for page load: %w", err)
	}
	b.stealth.RandomSleep(ctx, 1.0, 2.0)

	return nil
}

// HumanType types text into an element with human-like behavior
func (b *Instance) HumanType(ctx context.Context, selector string, text string) error {
	if b.page == nil {
		return fmt.Errorf("browser not initialized")
	}

	// Wait for element to appear (with timeout)
	if _, err := b.page.Timeout(10 * time.Second).Element(selector); err != nil {
		return fmt.Errorf("element not found: %s: %w", selector, err)
	}

	// Get element for interaction (without timeout)
	elem, err := b.page.Element(selector)
	if err != nil {
		return fmt.Errorf("failed to get element: %w", err)
	}

	// Click to focus
	if err := b.HumanClick(ctx, selector); err != nil {
		return fmt.Errorf("failed to click element: %w", err)
	}

	// Get typing actions from stealth engine
	actions, err := b.stealth.GetTypingActions(ctx, text)
	if err != nil {
		return fmt.Errorf("failed to generate typing actions: %w", err)
	}

	// Execute typing actions
	for _, action := range actions {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		switch action.Type {
		case stealth.ActionTypeKey:
			if action.Key == "\b" {
				// Backspace - Use Keyboard.Press for special keys to ensure correct behavior
				if err := b.page.Keyboard.Press(input.Backspace); err != nil {
					return fmt.Errorf("failed to press backspace: %w", err)
				}
			} else {
				// Type character
				if err := elem.Input(action.Key); err != nil {
					return fmt.Errorf("failed to input key: %w", err)
				}
			}
		case stealth.ActionTypeDelay:
			// Delay
		}

		// Apply delay
		if action.Delay > 0 {
			// If debug mode is on, slow down the delay to make it observable
			delay := action.Delay
			if b.config.Stealth.DebugStealth {
				delay *= 5
				b.logger.Info("Stealth Debug: Typing delay", zap.Duration("delay", delay))
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	return nil
}

// HumanClick clicks an element with Bézier curve mouse movement
func (b *Instance) HumanClick(ctx context.Context, selector string) error {
	if b.page == nil {
		return fmt.Errorf("browser not initialized")
	}

	// Wait for element to appear (with timeout)
	if _, err := b.page.Timeout(10 * time.Second).Element(selector); err != nil {
		return fmt.Errorf("element not found: %s: %w", selector, err)
	}

	// Get element for interaction (without timeout)
	elem, err := b.page.Element(selector)
	if err != nil {
		return fmt.Errorf("failed to get element: %w", err)
	}

	// Get element position using JavaScript
	boxResult, err := elem.Eval(`() => {
		const rect = this.getBoundingClientRect();
		return {
			x: rect.left + rect.width / 2,
			y: rect.top + rect.height / 2
		};
	}`)
	if err != nil {
		return fmt.Errorf("failed to get element position: %w", err)
	}

	// Extract coordinates from result
	var box struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}
	// Use MarshalJSON and Unmarshal to extract values
	boxJSON, err := boxResult.Value.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal element position: %w", err)
	}
	if err := json.Unmarshal(boxJSON, &box); err != nil {
		return fmt.Errorf("failed to parse element position: %w", err)
	}

	centerX := box.X
	centerY := box.Y

	// Get current mouse position (approximate - start from viewport center)
	startX := float64(b.config.Stealth.ViewportWidthMin) / 2
	startY := float64(b.config.Stealth.ViewportHeightMin) / 2

	// Get mouse path from stealth engine
	points := b.stealth.GetMousePath(startX, startY, centerX, centerY)

	// In debug mode, log the points and slow down the movement
	mouseMoveDelay := 10 // Default delay
	if b.config.Stealth.DebugStealth {
		mouseMoveDelay = 50 // Slower delay for observation
		b.logger.Info("Stealth Debug: Mouse path", zap.Int("points", len(points)))
	}

	// Execute mouse movement using CDP (Chrome DevTools Protocol)
	// This generates 'isTrusted: true' events which are indistinguishable from real hardware input,
	// unlike JavaScript-generated events which are easily detected.
	for _, p := range points {
		// Move mouse to the next point in the Bezier curve
		// We use CDP directly via proto.InputDispatchMouseEvent
		err := proto.InputDispatchMouseEvent{
			Type:       proto.InputDispatchMouseEventTypeMouseMoved,
			X:          p.X,
			Y:          p.Y,
		}.Call(b.page)
		if err != nil {
			b.logger.Debug("Failed to move mouse", zap.Error(err))
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Add micro-delays between movements to simulate human speed
		delay := time.Duration(mouseMoveDelay) * time.Millisecond
		if !b.config.Stealth.DebugStealth {
			// Add random jitter to the delay (5-15ms)
			jitter := rand.Intn(11) + 5
			delay = time.Duration(jitter) * time.Millisecond
		}
		time.Sleep(delay)
	}

	// Small delay before actual click
	b.stealth.RandomSleep(ctx, 0.1, 0.2)

	// Perform click
	if err := elem.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("failed to click element: %w", err)
	}

	return nil
}


// HumanScroll scrolls the page with human-like acceleration/deceleration
func (b *Instance) HumanScroll(ctx context.Context, direction string, distance int) error {
	if b.page == nil {
		return fmt.Errorf("browser not initialized")
	}

	// Get scroll actions from stealth engine
	actions, err := b.stealth.GetScrollActions(ctx, direction, distance)
	if err != nil {
		return fmt.Errorf("failed to generate scroll actions: %w", err)
	}

	// Execute scroll actions
	for _, action := range actions {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if action.Distance != 0 {
			// Use CDP to dispatch a mouse wheel event, which is stealthier than JS scrollBy
			// and works even if the page overrides window.scrollBy
			err := proto.InputDispatchMouseEvent{
				Type:   proto.InputDispatchMouseEventTypeMouseWheel,
				X:      100, // Arbitrary point inside viewport
				Y:      100,
				DeltaX: 0,
				DeltaY: float64(action.Distance),
			}.Call(b.page)

			if err != nil {
				b.logger.Debug("Failed to execute CDP scroll, falling back to keyboard", zap.Error(err))
				// Fallback to keyboard scrolling
				if action.Distance > 0 {
					_ = b.page.Keyboard.Press(input.ArrowDown)
				} else {
					_ = b.page.Keyboard.Press(input.ArrowUp)
				}
			}
		}

		if action.Delay > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(action.Delay):
			}
		}
	}

	return nil
}

// WaitForElement waits for an element to appear with timeout
func (b *Instance) WaitForElement(ctx context.Context, selector string, timeout time.Duration) error {
	if b.page == nil {
		return fmt.Errorf("browser not initialized")
	}

	_, err := b.page.Timeout(timeout).Element(selector)
	return err
}

// GetText extracts text content from an element
func (b *Instance) GetText(ctx context.Context, selector string) (string, error) {
	if b.page == nil {
		return "", fmt.Errorf("browser not initialized")
	}

	elem, err := b.page.Timeout(10 * time.Second).Element(selector)
	if err != nil {
		return "", fmt.Errorf("element not found: %s: %w", selector, err)
	}

	text, err := elem.Text()
	if err != nil {
		return "", fmt.Errorf("failed to get text: %w", err)
	}

	return text, nil
}

// GetAttribute gets an attribute value from an element
func (b *Instance) GetAttribute(ctx context.Context, selector string, attr string) (string, error) {
	if b.page == nil {
		return "", fmt.Errorf("browser not initialized")
	}

	elem, err := b.page.Timeout(10 * time.Second).Element(selector)
	if err != nil {
		return "", fmt.Errorf("element not found: %s: %w", selector, err)
	}

	value, err := elem.Attribute(attr)
	if err != nil {
		return "", fmt.Errorf("failed to get attribute %s: %w", attr, err)
	}

	if value == nil {
		return "", nil
	}

	return *value, nil
}

// ElementExists checks if an element exists on the page
func (b *Instance) ElementExists(ctx context.Context, selector string) (bool, error) {
	if b.page == nil {
		return false, fmt.Errorf("browser not initialized")
	}

	elem, err := b.page.Timeout(2 * time.Second).Element(selector)
	if err != nil {
		return false, nil // Element doesn't exist, not an error
	}

	return elem != nil, nil
}

// IsElementVisible checks if an element is visible on the page
func (b *Instance) IsElementVisible(ctx context.Context, selector string) (bool, error) {
	if b.page == nil {
		return false, fmt.Errorf("browser not initialized")
	}

	// Use a short timeout to check for visibility
	elem, err := b.page.Timeout(2 * time.Second).Element(selector)
	if err != nil {
		// Element not found, so it's not visible
		return false, nil
	}

	visible, err := elem.Visible()
	if err != nil {
		return false, fmt.Errorf("failed to check visibility: %w", err)
	}

	if !visible {
		return false, nil
	}

	// Check dimensions to avoid 1x1 tracking pixels or hidden iframes
	// The security challenge iframe should be substantial
	validSize, err := elem.Eval(`() => {
		const rect = this.getBoundingClientRect();
		return rect.width > 50 && rect.height > 50;
	}`)
	if err != nil {
		// If eval fails, assume it's not a valid visible element for our purposes
		return false, nil
	}

	return validSize.Value.Bool(), nil
}

// GetCurrentURL returns the current page URL
func (b *Instance) GetCurrentURL(ctx context.Context) (string, error) {
	if b.page == nil {
		return "", fmt.Errorf("browser not initialized")
	}

	info, err := b.page.Info()
	if err != nil {
		return "", fmt.Errorf("failed to get page info: %w", err)
	}

	return info.URL, nil
}

// GetPageHTML returns the full HTML content of the current page
func (b *Instance) GetPageHTML(ctx context.Context) (string, error) {
	if b.page == nil {
		return "", fmt.Errorf("browser not initialized")
	}

	return b.page.HTML()
}

// SaveCookies saves browser cookies to a file
func (b *Instance) SaveCookies(ctx context.Context, path string) error {
	if b.page == nil {
		return fmt.Errorf("browser not initialized")
	}

	cookies, err := b.page.Cookies([]string{})
	if err != nil {
		return fmt.Errorf("failed to get cookies: %w", err)
	}

	// Convert to JSON
	data, err := json.MarshalIndent(cookies, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cookies: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll("data", 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write cookies file: %w", err)
	}

	b.logger.Info("Cookies saved", zap.String("path", path))
	return nil
}

// LoadCookies loads browser cookies from a file
func (b *Instance) LoadCookies(ctx context.Context, path string) error {
	if b.page == nil {
		return fmt.Errorf("browser not initialized")
	}

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		b.logger.Info("Cookies file not found, skipping load", zap.String("path", path))
		return nil
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read cookies file: %w", err)
	}

	// Parse JSON - use the same type that Cookies() returns
	var cookies []*proto.NetworkCookie
	if err := json.Unmarshal(data, &cookies); err != nil {
		return fmt.Errorf("failed to unmarshal cookies: %w", err)
	}

	// Convert NetworkCookie to NetworkCookieParam using helper function
	cookieParams := proto.CookiesToParams(cookies)

	// Set cookies
	if err := b.page.SetCookies(cookieParams); err != nil {
		return fmt.Errorf("failed to set cookies: %w", err)
	}

	b.logger.Info("Cookies loaded", zap.String("path", path), zap.Int("count", len(cookies)))
	return nil
}

// Close closes the browser instance
func (b *Instance) Close(ctx context.Context) error {
	if b.browser == nil {
		return nil
	}

	if err := b.browser.Close(); err != nil {
		return fmt.Errorf("failed to close browser: %w", err)
	}

	b.logger.Info("Browser closed")
	return nil
}

// GetPage returns the underlying Rod page (for advanced usage)
func (b *Instance) GetPage() *rod.Page {
	return b.page
}

