package workflows

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"linkedin-automation/internal/core"

	"go.uber.org/zap"
)

// ConnectWorkflow implements the connection workflow
type ConnectWorkflow struct {
	browser   core.BrowserPort
	repository core.RepositoryPort
	config    *core.Config
	logger    *zap.Logger
}

// NewConnectWorkflow creates a new connection workflow
func NewConnectWorkflow(
	browser core.BrowserPort,
	repository core.RepositoryPort,
	config *core.Config,
	logger *zap.Logger,
) *ConnectWorkflow {
	return &ConnectWorkflow{
		browser:    browser,
		repository: repository,
		config:     config,
		logger:     logger,
	}
}

// SendConnectionRequest sends a connection request with a personalized note
func (c *ConnectWorkflow) SendConnectionRequest(ctx context.Context, params *core.ConnectParams) error {
	if params == nil {
		return fmt.Errorf("connect params cannot be nil")
	}

	if params.ProfileURL == "" {
		return fmt.Errorf("profile URL is required")
	}

	// 1. Enforce Daily Limits
	dailyCount, err := c.repository.GetTodayActionCount(ctx, "Connect")
	if err != nil {
		c.logger.Warn("Failed to check daily limits", zap.Error(err))
	} else if dailyCount >= int64(c.config.Limits.MaxActionsPerDay) {
		return fmt.Errorf("daily connection limit reached (%d/%d)", dailyCount, c.config.Limits.MaxActionsPerDay)
	}

	c.logger.Info("Sending connection request", zap.String("profile_url", params.ProfileURL))

	// Navigate to profile page
	if err := c.browser.Navigate(ctx, params.ProfileURL); err != nil {
		return fmt.Errorf("failed to navigate to profile: %w", err)
	}

	// Wait for profile page to load
	c.browser.RandomSleep(ctx, 2.0, 4.0)

	// Extract profile name if not provided
	if params.Name == "" {
		name, err := c.ExtractProfileName(ctx)
		if err != nil {
			c.logger.Warn("Failed to extract profile name", zap.Error(err))
			params.Name = "there" // Fallback
		} else {
			params.Name = name
		}
	}

	// Check if we should skip this profile
	shouldSkip, err := c.ShouldSkipProfile(ctx, params.ProfileURL)
	if err != nil {
		c.logger.Warn("Failed to check if should skip profile", zap.Error(err))
		// Continue anyway
	}

	if shouldSkip {
		c.logger.Info("Skipping profile", zap.String("reason", "already connected or not available"))
		return nil
	}

	// Scroll down slightly to ensure content is loaded, but not too much to hide the top card
	// Reduced from 300 to 20 to avoid hiding the 'More' button behind the sticky header
	if err := c.browser.HumanScroll(ctx, "down", 20); err != nil {
		c.logger.Warn("Failed to scroll", zap.Error(err))
	}

	// Try to find Connect button directly
	connectBtnFound := false
	
	// Try the configured selector first
	if c.config.Selectors.ProfileConnectBtn != "" {
		if err := c.browser.WaitForElement(ctx, c.config.Selectors.ProfileConnectBtn, 3*time.Second); err == nil {
			connectBtnFound = true
			c.logger.Info("Found Connect button directly", zap.String("selector", c.config.Selectors.ProfileConnectBtn))
		}
	}

	// If configured selector failed, try fallback selectors including the one found by user
	if !connectBtnFound {
		// Use fallback selectors from config
		fallbackSelectors := c.config.Selectors.ProfileConnectButtonFallbacks

		for _, selector := range fallbackSelectors {
			if err := c.browser.WaitForElement(ctx, selector, 2*time.Second); err == nil {
				c.config.Selectors.ProfileConnectBtn = selector
				connectBtnFound = true
				c.logger.Info("Found Connect button using fallback", zap.String("selector", selector))
				break
			}
		}
	}

	if !connectBtnFound {
		// If not found, check if it's hidden under "More" actions
		c.logger.Info("Connect button not found directly, checking 'More' menu...")

		// Define fallback selectors for "More" button
		// We strictly scope this to the top card (.pv-top-card) to avoid clicking "More" buttons
		// in the "People also viewed" section or other parts of the page.
		moreSelectors := []string{
			// Use configured fallbacks first
			c.config.Selectors.ProfileMoreButton,
		}
		// Append configured fallbacks
		moreSelectors = append(moreSelectors, c.config.Selectors.ProfileMoreButtonFallbacks...)

		var foundMoreSelector string
		for _, selector := range moreSelectors {
			if selector == "" {
				continue
			}
			// Exclude sticky header if not already excluded
			if !strings.Contains(selector, ":not(.pvs-sticky-header") {
				selector = selector + ":not(.pvs-sticky-header-profile-actions__action)"
			}
			
			// Check if it exists and is visible
			// We use IsElementVisible to ensure we don't try to click something hidden
			if visible, _ := c.browser.IsElementVisible(ctx, selector); visible {
				foundMoreSelector = selector
				break
			}
		}

		if foundMoreSelector != "" {
			c.logger.Info("Found 'More' button", zap.String("selector", foundMoreSelector))
			
			// Try human click first
			if err := c.browser.HumanClick(ctx, foundMoreSelector); err != nil {
				c.logger.Warn("Human click failed, trying JS click", zap.Error(err))
				if err := c.browser.JSClick(ctx, foundMoreSelector); err != nil {
					c.logger.Error("JS click also failed", zap.Error(err))
				}
			}
			
			c.browser.RandomSleep(ctx, 1.0, 2.0)

			// Verify if the dropdown content is visible
			// This confirms the menu actually opened
			dropdownVisible, _ := c.browser.IsElementVisible(ctx, ".artdeco-dropdown__content")
			if !dropdownVisible {
				c.logger.Warn("Dropdown content not visible after clicking 'More', trying JS click...")
				// Retry with JS click
				if err := c.browser.JSClick(ctx, foundMoreSelector); err != nil {
					c.logger.Error("Retry JS click failed", zap.Error(err))
				}
				c.browser.RandomSleep(ctx, 1.0, 2.0)
				
				// Check again
				dropdownVisible, _ = c.browser.IsElementVisible(ctx, ".artdeco-dropdown__content")
				if !dropdownVisible {
					c.logger.Error("Dropdown still not visible after retry")
					// Dump HTML here to see why it's not opening
					if html, errHtml := c.browser.GetPageHTML(ctx); errHtml == nil {
						dumpPath := fmt.Sprintf("data/debug_more_click_fail_%d.html", time.Now().Unix())
						if errWrite := os.WriteFile(dumpPath, []byte(html), 0644); errWrite == nil {
							c.logger.Info("Dumped HTML after failed 'More' click", zap.String("path", dumpPath))
						}
					}
				}
			}

			// Define fallback selectors for "Connect" option in dropdown
			// CRITICAL: We must scope this to .artdeco-dropdown__content or similar
			// to ensure we don't click a "Connect" button elsewhere on the page (like "People also viewed")
			connectOptionSelectors := []string{
				c.config.Selectors.ProfileMoreConnectOption,
			}
			// Append configured fallbacks
			connectOptionSelectors = append(connectOptionSelectors, c.config.Selectors.ProfileConnectOptionFallbacks...)

			for _, selector := range connectOptionSelectors {
				if selector == "" {
					continue
				}
				// Use a short timeout because the menu is already open
				// We use IsElementVisible here too to be sure
				if visible, _ := c.browser.IsElementVisible(ctx, selector); visible {
					// Update selector to use the one we found for the click
					c.config.Selectors.ProfileConnectBtn = selector
					connectBtnFound = true
					c.logger.Info("Found Connect button in 'More' menu", zap.String("selector", selector))
					break
				}
			}
		}
	}

	if !connectBtnFound {
		// Dump HTML for debugging so user can find the correct selector
		if html, errHtml := c.browser.GetPageHTML(ctx); errHtml == nil {
			dumpPath := fmt.Sprintf("data/debug_connect_fail_%d.html", time.Now().Unix())
			if errWrite := os.WriteFile(dumpPath, []byte(html), 0644); errWrite == nil {
				c.logger.Info("Dumped profile page HTML for debugging", zap.String("path", dumpPath))
			}
		}
		return fmt.Errorf("connect button not found (even after checking 'More' menu)")
	}

	// Click Connect button with human-like mouse movement
	if err := c.browser.HumanClick(ctx, c.config.Selectors.ProfileConnectBtn); err != nil {
		return fmt.Errorf("failed to click connect button: %w", err)
	}

	// Wait for connection modal/dialog to appear
	c.browser.RandomSleep(ctx, 2.0, 3.0)

	// Handle Note
	if params.Note != "" {
		// Check for "Add a note" button
		addNoteSelector := c.config.Selectors.ConnectModalAddNoteButton
		if addNoteSelector == "" {
			addNoteSelector = "button[aria-label*='Add a note']"
		}

		// Wait for the "Add a note" button to be visible
		if err := c.browser.WaitForElement(ctx, addNoteSelector, 5*time.Second); err == nil {
			if err := c.browser.HumanClick(ctx, addNoteSelector); err != nil {
				c.logger.Warn("Failed to click 'Add a note'", zap.Error(err))
			} else {
				c.browser.RandomSleep(ctx, 1.0, 2.0)
				
				// Check if textarea appeared (it might not if monthly limit is reached)
				textareaSelector := c.config.Selectors.ConnectNoteTextarea
				if textareaSelector == "" {
					textareaSelector = "textarea[name='message']"
				}

				textareaExists, err := c.browser.ElementExists(ctx, textareaSelector)
				if err != nil {
					c.logger.Warn("Failed to check for note textarea", zap.Error(err))
				}

				if !textareaExists {
					c.logger.Warn("Note textarea not found after clicking 'Add a note'. Monthly limit for personalized invites might be reached. Sending without note.")
					
					// Check for potential "Got it" or dismissal button if a limit modal appeared
					dismissSelectors := []string{
						"button[aria-label='Got it']",
						"button[aria-label='Dismiss']",
						"button.artdeco-modal__dismiss",
					}
					
					for _, sel := range dismissSelectors {
						if exists, _ := c.browser.ElementExists(ctx, sel); exists {
							c.logger.Info("Found dismissal button, clicking it to proceed", zap.String("selector", sel))
							if err := c.browser.HumanClick(ctx, sel); err != nil {
								c.logger.Warn("Failed to click dismissal button", zap.Error(err))
							}
							c.browser.RandomSleep(ctx, 0.5, 1.0)
							break
						}
					}

					// Retry clicking Connect to open the modal again (without adding note this time)
					c.logger.Info("Retrying connection without note...")
					if err := c.browser.HumanClick(ctx, c.config.Selectors.ProfileConnectBtn); err != nil {
						c.logger.Warn("Failed to click connect button on retry", zap.Error(err))
					}
					c.browser.RandomSleep(ctx, 2.0, 3.0)
				} else {
					// Personalize note with name
					personalizedNote := strings.ReplaceAll(params.Note, "{{Name}}", params.Name)
					
					// Enforce character limit (300 chars)
					if len(personalizedNote) > 300 {
						c.logger.Warn("Note exceeds 300 characters, truncating", zap.Int("length", len(personalizedNote)))
						personalizedNote = personalizedNote[:297] + "..."
					}

					// Type note with human-like behavior
					if err := c.browser.HumanType(ctx, textareaSelector, personalizedNote); err != nil {
						c.logger.Warn("Failed to type note", zap.Error(err))
					}
					
					// Small delay before sending
					c.browser.RandomSleep(ctx, 1.0, 2.0)
				}
			}
		} else {
			c.logger.Info("Add a note button not found, sending without note")
		}
	}

	// Click Send button
	sendExists, err := c.browser.ElementExists(ctx, c.config.Selectors.ConnectSendButton)
	if err == nil && sendExists {
		if err := c.browser.HumanClick(ctx, c.config.Selectors.ConnectSendButton); err != nil {
			return fmt.Errorf("failed to click send button: %w", err)
		}
	} else {
		// Some LinkedIn flows might auto-send or use different button text
		// Try alternative selectors
		altSelectors := []string{
			"button[aria-label*='Send now']",
			"button[aria-label*='Send']",
			"button:contains('Send')",
		}
		
		clicked := false
		for _, selector := range altSelectors {
			if exists, _ := c.browser.ElementExists(ctx, selector); exists {
				if err := c.browser.HumanClick(ctx, selector); err == nil {
					clicked = true
					break
				}
			}
		}
		
		if !clicked {
			c.logger.Warn("Could not find send button, connection may have been sent automatically")
		}
	}

	// Wait a moment for the request to process
	c.browser.RandomSleep(ctx, 2.0, 4.0)

	// Record in database
	existing, err := c.repository.GetProfileByURL(ctx, params.ProfileURL)
	if err == nil && existing != nil {
		if err := c.repository.UpdateProfileStatus(ctx, params.ProfileURL, core.ProfileStatusRequestSent); err != nil {
			c.logger.Warn("Failed to update profile status", zap.Error(err))
		}
	} else {
		profile := &core.Profile{
			LinkedInURL: params.ProfileURL,
			Status:      core.ProfileStatusRequestSent,
		}
		if err := c.repository.CreateProfile(ctx, profile); err != nil {
			c.logger.Warn("Failed to save profile to database", zap.Error(err))
		}
	}

	// Record in history
	history := &core.History{
		ActionType: "Connect",
		Details:    fmt.Sprintf("Connected to %s", params.ProfileURL),
		Timestamp:  time.Now(),
	}

	if err := c.repository.CreateHistory(ctx, history); err != nil {
		c.logger.Warn("Failed to save history", zap.Error(err))
	}

	c.logger.Info("Connection request sent successfully", zap.String("profile_url", params.ProfileURL))

	return nil
}

// ExtractProfileName extracts the profile name from a profile page
func (c *ConnectWorkflow) ExtractProfileName(ctx context.Context) (string, error) {
	// LinkedIn profile pages have the name in various locations
	// Common selectors:
	selectors := []string{
		"h1.text-heading-xlarge",
		"h1[data-anonymize='person-name']",
		".pv-text-details__left-panel h1",
		"h1.inline",
	}

	for _, selector := range selectors {
		exists, err := c.browser.ElementExists(ctx, selector)
		if err != nil {
			continue
		}

		if exists {
			name, err := c.browser.GetText(ctx, selector)
			if err == nil && name != "" {
				// Clean up the name (remove extra whitespace)
				name = strings.TrimSpace(name)
				// Extract first name if full name
				parts := strings.Fields(name)
				if len(parts) > 0 {
					return parts[0], nil
				}
				return name, nil
			}
		}
	}

	return "", fmt.Errorf("could not extract profile name")
}

// ShouldSkipProfile checks if a profile should be skipped
func (c *ConnectWorkflow) ShouldSkipProfile(ctx context.Context, profileURL string) (bool, error) {
	// Check database first
	existingProfile, err := c.repository.GetProfileByURL(ctx, profileURL)
	if err != nil {
		return false, fmt.Errorf("failed to check database: %w", err)
	}

	if existingProfile != nil {
		// Already processed
		if existingProfile.Status == core.ProfileStatusConnected || 
		   existingProfile.Status == core.ProfileStatusIgnored || 
		   existingProfile.Status == core.ProfileStatusRequestSent {
			c.logger.Info("Profile already processed", 
				zap.String("url", profileURL),
				zap.String("status", existingProfile.Status),
			)
			return true, nil
		}
	}

	// Check if Connect button exists and is enabled
	// If button says "Connected" or "Pending", skip
	connectBtnText, err := c.browser.GetText(ctx, c.config.Selectors.ProfileConnectBtn)
	if err == nil {
		btnTextLower := strings.ToLower(connectBtnText)
		if strings.Contains(btnTextLower, "connected") ||
		   strings.Contains(btnTextLower, "pending") {
			c.logger.Info("Profile already connected or pending", zap.String("button_text", connectBtnText))
			return true, nil
		}
	}

	return false, nil
}

// GetRepository returns the repository instance (for rate limiting checks)
func (c *ConnectWorkflow) GetRepository() core.RepositoryPort {
	return c.repository
}


