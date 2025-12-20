package workflows

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"linkedin-automation/internal/core"

	"go.uber.org/zap"
)

// MessagingWorkflow implements the messaging and follow-up workflow
type MessagingWorkflow struct {
	browser    core.BrowserPort
	repository core.RepositoryPort
	config     *core.Config
	logger     *zap.Logger
}

// NewMessagingWorkflow creates a new messaging workflow
func NewMessagingWorkflow(
	browser core.BrowserPort,
	repository core.RepositoryPort,
	config *core.Config,
	logger *zap.Logger,
) *MessagingWorkflow {
	return &MessagingWorkflow{
		browser:    browser,
		repository: repository,
		config:     config,
		logger:     logger,
	}
}

// ScanNewConnections checks for new connections and updates their status in the DB
func (m *MessagingWorkflow) ScanNewConnections(ctx context.Context) error {
	m.logger.Info("Scanning for new connections...")

	connectionsURL := "https://www.linkedin.com/mynetwork/invite-connect/connections/"
	if err := m.browser.Navigate(ctx, connectionsURL); err != nil {
		return fmt.Errorf("failed to navigate to connections page: %w", err)
	}

	// Wait for the list to load
	// The list container usually has a class like 'scaffold-finite-scroll__content' or specific connection cards
	listSelector := ".mn-connection-card"
	if err := m.browser.WaitForElement(ctx, listSelector, 10*time.Second); err != nil {
		m.logger.Warn("Could not find connection cards (maybe no connections yet?)", zap.Error(err))
		return nil
	}

	// Scroll down a bit to ensure we get the most recent ones
	// We don't need to scroll infinitely, just enough to catch recent accepts
	if err := m.browser.HumanScroll(ctx, "down", 500); err != nil {
		m.logger.Warn("Failed to scroll connections list", zap.Error(err))
	}
	m.browser.RandomSleep(ctx, 2.0, 3.0)

	// Extract all profile URLs from the visible list
	// Selector targets the main link in the connection card
	linkSelector := ".mn-connection-card__link"
	urls, err := m.browser.GetAttributes(ctx, linkSelector, "href")
	if err != nil {
		return fmt.Errorf("failed to extract connection URLs: %w", err)
	}

	m.logger.Info("Found connections on page", zap.Int("count", len(urls)))

	newConnectionsCount := 0
	
	for _, rawURL := range urls {
		// Clean the URL
		profileURL := m.cleanProfileURL(rawURL)
		if profileURL == "" {
			continue
		}

		// Check if we know this profile
		profile, err := m.repository.GetProfileByURL(ctx, profileURL)
		if err != nil {
			m.logger.Error("Failed to query profile", zap.String("url", profileURL), zap.Error(err))
			continue
		}

		if profile != nil {
			// If we sent a request and now they appear here, they accepted!
			if profile.Status == core.ProfileStatusRequestSent || 
			   profile.Status == core.ProfileStatusScanned || 
			   profile.Status == core.ProfileStatusDiscovered {
				
				m.logger.Info("Detected new connection acceptance", 
					zap.String("url", profileURL),
					zap.String("previous_status", profile.Status),
				)

				if err := m.repository.MarkAsConnected(ctx, profileURL); err != nil {
					m.logger.Error("Failed to mark profile as connected", zap.Error(err))
				} else {
					newConnectionsCount++
				}
			} else if profile.Status == core.ProfileStatusConnected {
				// Already marked, likely from a previous run
				m.logger.Debug("Profile already marked as connected", zap.String("url", profileURL))
			}
		} else {
			// Profile not in our DB. 
			// Option: Add them as 'Connected' so we can potentially message them later?
			// For now, we'll just log it.
			m.logger.Debug("Found connection not in DB", zap.String("url", profileURL))
			
			// TODO: Uncomment to auto-add unknown connections
			/*
			newProfile := &core.Profile{
				LinkedInURL: profileURL,
				Status:      core.ProfileStatusConnected,
			}
			if err := m.repository.CreateProfile(ctx, newProfile); err == nil {
				newConnectionsCount++
			}
			*/
		}
	}

	m.logger.Info("Scan complete", zap.Int("newly_marked_connected", newConnectionsCount))
	return nil
}

// cleanProfileURL removes query parameters and ensures standard format
func (m *MessagingWorkflow) cleanProfileURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	
	// Handle relative URLs
	if strings.HasPrefix(rawURL, "/") {
		rawURL = "https://www.linkedin.com" + rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	// Keep only scheme, host, and path
	// Example: https://www.linkedin.com/in/username/
	return fmt.Sprintf("%s://%s%s", parsed.Scheme, parsed.Host, parsed.Path)
}

// SendFollowUpMessages sends personalized follow-up messages to new connections
func (m *MessagingWorkflow) SendFollowUpMessages(ctx context.Context) error {
	// 1. Get pending follow-ups
	// Limit to configured batch limit
	limit := m.config.Messaging.BatchLimit
	if limit <= 0 {
		limit = 5 // Default fallback
	}
	profiles, err := m.repository.GetPendingFollowups(ctx, limit)
	if err != nil {
		return fmt.Errorf("failed to get pending follow-ups: %w", err)
	}

	if len(profiles) == 0 {
		m.logger.Info("No pending follow-up messages found")
		return nil
	}

	m.logger.Info("Starting follow-up sequence", zap.Int("count", len(profiles)))

	for i, profile := range profiles {
		// Check context
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		m.logger.Info("Processing follow-up", 
			zap.Int("index", i+1), 
			zap.String("url", profile.LinkedInURL),
		)

		// 2. Navigate to profile
		if err := m.browser.Navigate(ctx, profile.LinkedInURL); err != nil {
			m.logger.Error("Failed to navigate to profile", zap.String("url", profile.LinkedInURL), zap.Error(err))
			continue
		}
		
		// Wait for load
		m.browser.RandomSleep(ctx, 3.0, 5.0)

		// 3. Extract Name for personalization
		firstName := m.extractFirstName(ctx)
		if firstName == "" {
			firstName = "there" // Fallback
		}

		// 4. Find and Click Message Button
		if err := m.clickMessageButton(ctx); err != nil {
			m.logger.Warn("Failed to click message button", zap.Error(err))
			// Dump HTML for debugging
			if html, errHtml := m.browser.GetPageHTML(ctx); errHtml == nil {
				dumpPath := fmt.Sprintf("data/debug_msg_fail_%d.html", time.Now().Unix())
				_ = os.WriteFile(dumpPath, []byte(html), 0644)
			}
			continue
		}

		// 5. Wait for chat overlay/window
		// The chat input usually has role='textbox' and is contenteditable
		chatInputSelector := "div.msg-form__contenteditable[role='textbox']"
		if err := m.browser.WaitForElement(ctx, chatInputSelector, 5*time.Second); err != nil {
			m.logger.Warn("Chat input not found", zap.Error(err))
			continue
		}

		// 6. Prepare Message
		template := m.config.Messaging.FollowUpTemplate
		if template == "" {
			template = "Hi {{FirstName}}, thanks for connecting! I'd love to keep in touch."
		}
		
		messageBody := strings.ReplaceAll(template, "{{FirstName}}", firstName)

		// 7. Type Message
		if err := m.browser.HumanClick(ctx, chatInputSelector); err != nil {
			m.logger.Warn("Failed to focus chat input", zap.Error(err))
			continue
		}
		
		if err := m.browser.HumanType(ctx, chatInputSelector, messageBody); err != nil {
			m.logger.Error("Failed to type message", zap.Error(err))
			continue
		}

		// 8. Click Send
		sendBtnSelector := "button.msg-form__send-button"
		if err := m.browser.WaitForElement(ctx, sendBtnSelector, 2*time.Second); err != nil {
			m.logger.Warn("Send button not found", zap.Error(err))
			continue
		}

		if err := m.browser.HumanClick(ctx, sendBtnSelector); err != nil {
			m.logger.Error("Failed to click send button", zap.Error(err))
			continue
		}

		// 9. Log Success
		if err := m.repository.LogMessageSent(ctx, profile.ID, messageBody); err != nil {
			m.logger.Error("Failed to log message sent", zap.Error(err))
		} else {
			m.logger.Info("Follow-up message sent successfully")
		}

		// 10. Cooldown
		if i < len(profiles)-1 {
			// Random delay 2-5 minutes
			delay := time.Duration(120 + time.Now().Unix()%180) * time.Second
			m.logger.Info("Sleeping before next message", zap.Duration("duration", delay))
			
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	return nil
}

// extractFirstName extracts the first name from the profile page
func (m *MessagingWorkflow) extractFirstName(ctx context.Context) string {
	// Try standard profile name selector
	// Usually h1.text-heading-xlarge
	nameSelector := "h1.text-heading-xlarge"
	name, err := m.browser.GetText(ctx, nameSelector)
	if err != nil || name == "" {
		return ""
	}

	// Split by space and take first part
	parts := strings.Fields(name)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// clickMessageButton attempts to find and click the message button
func (m *MessagingWorkflow) clickMessageButton(ctx context.Context) error {
	// 1. Try primary/secondary Message button
	// "Message" button is often a secondary button if "Connect" is primary, 
	// or primary if already connected.
	selectors := []string{
		"button.artdeco-button--primary[aria-label*='Message']",
		"button.artdeco-button--secondary[aria-label*='Message']",
		"button[aria-label*='Message']",
	}

	for _, sel := range selectors {
		if exists, _ := m.browser.ElementExists(ctx, sel); exists {
			return m.browser.HumanClick(ctx, sel)
		}
	}

	// 2. Check "More" menu
	moreBtn := "button[aria-label*='More actions']"
	if exists, _ := m.browser.ElementExists(ctx, moreBtn); exists {
		if err := m.browser.HumanClick(ctx, moreBtn); err != nil {
			return err
		}
		m.browser.RandomSleep(ctx, 1.0, 2.0)

		// Look for Message in dropdown
		msgOption := "div[role='button'][aria-label*='Message']"
		if err := m.browser.WaitForElement(ctx, msgOption, 2*time.Second); err == nil {
			return m.browser.HumanClick(ctx, msgOption)
		}
	}

	return fmt.Errorf("message button not found")
}
