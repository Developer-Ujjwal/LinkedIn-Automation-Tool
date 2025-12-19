package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	rodstealth "github.com/go-rod/stealth"
	"go.uber.org/zap"
	"linkedin-automation/internal/core"
	"linkedin-automation/internal/stealth"
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
	b.page.MustEval(`
		Object.defineProperty(navigator, 'webdriver', {
			get: () => undefined
		});
	`)

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
	b.page.MustWaitLoad()
	b.stealth.RandomSleep(ctx, 1.0, 2.0)

	return nil
}

// HumanType types text into an element with human-like behavior
func (b *Instance) HumanType(ctx context.Context, selector string, text string) error {
	if b.page == nil {
		return fmt.Errorf("browser not initialized")
	}

	// Wait for element
	elem, err := b.page.Timeout(10 * time.Second).Element(selector)
	if err != nil {
		return fmt.Errorf("element not found: %s: %w", selector, err)
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
				// Backspace
				elem.MustInput("\b")
			} else {
				// Type character
				elem.MustInput(action.Key)
			}
		case stealth.ActionTypeDelay:
			// Delay
		}

		// Apply delay
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

// HumanClick clicks an element with Bézier curve mouse movement
func (b *Instance) HumanClick(ctx context.Context, selector string) error {
	if b.page == nil {
		return fmt.Errorf("browser not initialized")
	}

	elem, err := b.page.Timeout(10 * time.Second).Element(selector)
	if err != nil {
		return fmt.Errorf("element not found: %s: %w", selector, err)
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

	// Execute mouse movement using JavaScript
	jsCode := fmt.Sprintf(`
		(function() {
			const points = %s;
			let currentIndex = 0;
			
			function moveMouse() {
				if (currentIndex >= points.length) {
					// Click at final position
					const event = new MouseEvent('click', {
						view: window,
						bubbles: true,
						cancelable: true
					});
					document.elementFromPoint(points[points.length-1].X, points[points.length-1].Y).dispatchEvent(event);
					return;
				}
				
				const point = points[currentIndex];
				const event = new MouseEvent('mousemove', {
					view: window,
					bubbles: true,
					cancelable: true,
					clientX: point.X,
					clientY: point.Y
				});
				document.dispatchEvent(event);
				currentIndex++;
				setTimeout(moveMouse, 10);
			}
			
			moveMouse();
		})();
	`, b.pointsToJSON(points))

	// Execute mouse movement
	b.page.MustEval(jsCode)

	// Small delay before actual click
	b.stealth.RandomSleep(ctx, 0.1, 0.2)

	// Perform click
	elem.MustClick()

	return nil
}

// pointsToJSON converts Point slice to JSON string
func (b *Instance) pointsToJSON(points []stealth.Point) string {
	type PointJSON struct {
		X float64 `json:"X"`
		Y float64 `json:"Y"`
	}
	jsonPoints := make([]PointJSON, len(points))
	for i, p := range points {
		jsonPoints[i] = PointJSON{X: p.X, Y: p.Y}
	}
	data, _ := json.Marshal(jsonPoints)
	return string(data)
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
			b.page.MustEval(fmt.Sprintf("window.scrollBy(0, %d);", action.Distance))
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

// GetCurrentURL returns the current page URL
func (b *Instance) GetCurrentURL(ctx context.Context) (string, error) {
	if b.page == nil {
		return "", fmt.Errorf("browser not initialized")
	}

	return b.page.MustInfo().URL, nil
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

