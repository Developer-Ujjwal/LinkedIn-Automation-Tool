package workflows

import (
	"context"
	"fmt"
	"strings"
	"time"

	"linkedin-automation/internal/core"

	"go.uber.org/zap"
)

// AuthWorkflow implements the authentication workflow
type AuthWorkflow struct {
	browser   core.BrowserPort
	config    *core.Config
	logger    *zap.Logger
}

// NewAuthWorkflow creates a new authentication workflow
func NewAuthWorkflow(browser core.BrowserPort, config *core.Config, logger *zap.Logger) *AuthWorkflow {
	return &AuthWorkflow{
		browser: browser,
		config:  config,
		logger:  logger,
	}
}

// Authenticate performs login or loads existing session
func (a *AuthWorkflow) Authenticate(ctx context.Context) error {
	// Try to load existing cookies first
	if err := a.browser.LoadCookies(ctx, a.config.Session.CookiesPath); err != nil {
		a.logger.Warn("Failed to load cookies, will perform fresh login", zap.Error(err))
	}

	// Check if already authenticated
	isAuth, err := a.IsAuthenticated(ctx)
	if err != nil {
		return fmt.Errorf("failed to check authentication status: %w", err)
	}

	if isAuth {
		a.logger.Info("Already authenticated, using existing session")
		return nil
	}

	// Perform login
	a.logger.Info("Starting authentication process")

	// Navigate to login page
	if err := a.browser.Navigate(ctx, a.config.LinkedIn.LoginURL); err != nil {
		return fmt.Errorf("failed to navigate to login page: %w", err)
	}

	// Wait for login form to appear
	if err := a.browser.WaitForElement(ctx, a.config.Selectors.LoginEmailInput, 10*time.Second); err != nil {
		return fmt.Errorf("login form not found: %w", err)
	}

	// Type email with human-like behavior
	if err := a.browser.HumanType(ctx, a.config.Selectors.LoginEmailInput, a.config.Credentials.Email); err != nil {
		return fmt.Errorf("failed to type email: %w", err)
	}

	// Small delay between fields
	a.browser.RandomSleep(ctx, 0.5, 1.0)

	// Type password with human-like behavior
	if err := a.browser.HumanType(ctx, a.config.Selectors.LoginPasswordInput, a.config.Credentials.Password); err != nil {
		return fmt.Errorf("failed to type password: %w", err)
	}

	// Small delay before clicking submit
	a.browser.RandomSleep(ctx, 0.5, 1.0)

	// Click submit button
	if err := a.browser.HumanClick(ctx, a.config.Selectors.LoginSubmitButton); err != nil {
		return fmt.Errorf("failed to click submit button: %w", err)
	}

	// Wait for navigation (either success or 2FA challenge)
	a.browser.RandomSleep(ctx, 3.0, 5.0)

	// Check for 2FA challenge
	currentURL, err := a.browser.GetCurrentURL(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current URL: %w", err)
	}

	// Check for generic security challenge (Arkose/Captcha)
	if err := a.handleSecurityChallenge(ctx); err != nil {
		return fmt.Errorf("security challenge failed: %w", err)
	}

	// Refresh URL after potential security challenge resolution
	currentURL, err = a.browser.GetCurrentURL(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current URL: %w", err)
	}

	// Check if we're on a challenge/2FA page
	if strings.Contains(currentURL, "challenge") || strings.Contains(currentURL, "checkpoint") {
		a.logger.Warn("2FA challenge detected", zap.String("url", currentURL))
		return a.Handle2FA(ctx)
	}

	// Check if 2FA input field exists
	exists, err := a.browser.ElementExists(ctx, a.config.Selectors.TwoFactorChallenge)
	if err != nil {
		return fmt.Errorf("failed to check for 2FA field: %w", err)
	}

	if exists {
		a.logger.Warn("2FA challenge detected via input field")
		return a.Handle2FA(ctx)
	}

	// Wait a bit more and check if we're logged in
	a.browser.RandomSleep(ctx, 2.0, 4.0)
	isAuth, err = a.IsAuthenticated(ctx)
	if err != nil {
		return fmt.Errorf("failed to verify authentication: %w", err)
	}

	if !isAuth {
		return fmt.Errorf("authentication failed - still not logged in")
	}

	// Save cookies for future use
	if err := a.browser.SaveCookies(ctx, a.config.Session.CookiesPath); err != nil {
		a.logger.Warn("Failed to save cookies", zap.Error(err))
		// Don't fail the entire auth process if cookie save fails
	}

	a.logger.Info("Authentication successful")
	return nil
}

// IsAuthenticated checks if the current session is valid by looking for a key element on the feed page.
func (a *AuthWorkflow) IsAuthenticated(ctx context.Context) (bool, error) {
	// Check if we are already on the feed or have the feed container
	// This avoids unnecessary navigation which might trigger security checks
	currentURL, err := a.browser.GetCurrentURL(ctx)
	if err == nil && strings.Contains(currentURL, "/feed") {
		a.logger.Info("User is already logged in (URL contains /feed)")
		return true, nil
	}

	if a.config.Selectors.FeedContainer != "" {
		exists, _ := a.browser.ElementExists(ctx, a.config.Selectors.FeedContainer)
		if exists {
			a.logger.Info("User is already logged in (feed container found)")
			return true, nil
		}
	}

	// Navigate to the main feed page to check for a logged-in state
	if err := a.browser.Navigate(ctx, a.config.LinkedIn.BaseURL); err != nil {
		return false, fmt.Errorf("failed to navigate to feed page: %w", err)
	}

	// Check if the main feed container is present
	if a.config.Selectors.FeedContainer != "" {
		exists, err := a.browser.ElementExists(ctx, a.config.Selectors.FeedContainer)
		if err != nil {
			return false, fmt.Errorf("failed to check for feed container: %w", err)
		}

		if exists {
			a.logger.Info("User is already logged in (feed container found)")
			return true, nil
		}
	}

	// Check if the URL indicates we are on the feed
	currentURL, err = a.browser.GetCurrentURL(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get current URL: %w", err)
	}

	if strings.Contains(currentURL, "/feed") {
		a.logger.Info("User is already logged in (URL contains /feed)")
		return true, nil
	}

	return false, nil
}

// handleSecurityChallenge checks for security challenges and pauses for manual intervention
func (a *AuthWorkflow) handleSecurityChallenge(ctx context.Context) error {
	// Check for common security challenge indicators using element visibility
	challengeReason := ""

	// Check 1: Human Security Enforcer Iframe
	if visible, _ := a.browser.IsElementVisible(ctx, "#humanSecurityEnforcerIframe"); visible {
		challengeReason = "Visible #humanSecurityEnforcerIframe"
	}

	// Check 2: Internal Captcha
	if challengeReason == "" {
		if visible, _ := a.browser.IsElementVisible(ctx, "#captcha-internal"); visible {
			challengeReason = "Visible #captcha-internal"
		}
	}

	// Check 3: Security Check Text
	if challengeReason == "" {
		if visible, _ := a.browser.IsElementVisible(ctx, "//*[contains(text(), \"Let's do a quick security check\")]"); visible {
			challengeReason = "Visible text 'Let's do a quick security check'"
		}
	}

	if challengeReason != "" {
		a.logger.Warn("⚠️ SECURITY CHALLENGE DETECTED! ⚠️", zap.String("reason", challengeReason))
		a.logger.Warn("The bot has been presented with a security check (CAPTCHA/Arkose).")
		a.logger.Warn("Please switch to the browser window and solve the challenge MANUALLY.")
		a.logger.Warn("Waiting for up to 5 minutes...")

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		timeout := time.After(5 * time.Minute)

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timeout:
				return fmt.Errorf("timed out waiting for manual security challenge resolution")
			case <-ticker.C:
				// Check if we are back to a normal page (feed)
				currentURL, err := a.browser.GetCurrentURL(ctx)
				if err != nil {
					continue
				}

				// If we are on the feed, the challenge is resolved
				if strings.Contains(currentURL, "/feed") {
					a.logger.Info("Security challenge resolved (on feed). Resuming workflow...")
					a.browser.RandomSleep(ctx, 3.0, 5.0)
					return nil
				}

				// Also check if the challenge elements are gone
				html, err := a.browser.GetPageHTML(ctx)
				if err == nil {
					stillHasChallenge := strings.Contains(html, "humanSecurityEnforcerIframe") ||
						strings.Contains(html, "grecaptcha-badge") ||
						strings.Contains(html, "security-challenge")

					if !stillHasChallenge {
						a.logger.Info("Security challenge elements gone. Resuming workflow...")
						a.browser.RandomSleep(ctx, 3.0, 5.0)
						return nil
					}
				}
			}
		}
	}

	return nil
}

// Handle2FA waits for manual 2FA intervention
func (a *AuthWorkflow) Handle2FA(ctx context.Context) error {
	a.logger.Warn("2FA challenge detected - waiting for manual intervention")
	a.logger.Info("Please complete 2FA manually in the browser window")
	a.logger.Info("Press ENTER in the console once 2FA is completed...")

	// Wait for user to complete 2FA manually
	// In a real implementation, you might want to poll for authentication success
	// For now, we'll wait indefinitely (or until context cancellation)
	
	// Check every 2 seconds if authentication succeeded
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Check if we are on the feed page WITHOUT navigating
			// Navigating here would interrupt the user's 2FA process
			currentURL, err := a.browser.GetCurrentURL(ctx)
			if err != nil {
				continue
			}

			if strings.Contains(currentURL, "/feed") {
				a.logger.Info("2FA completed successfully (URL check)")
				
				// Save cookies
				if err := a.browser.SaveCookies(ctx, a.config.Session.CookiesPath); err != nil {
					a.logger.Warn("Failed to save cookies after 2FA", zap.Error(err))
				}
				return nil
			}

			// Also check for feed element on current page
			if a.config.Selectors.FeedContainer != "" {
				exists, _ := a.browser.ElementExists(ctx, a.config.Selectors.FeedContainer)
				if exists {
					a.logger.Info("2FA completed successfully (Element check)")
					
					// Save cookies
					if err := a.browser.SaveCookies(ctx, a.config.Session.CookiesPath); err != nil {
						a.logger.Warn("Failed to save cookies after 2FA", zap.Error(err))
					}
					return nil
				}
			}
		}
	}
}

