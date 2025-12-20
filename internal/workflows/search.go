package workflows

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"linkedin-automation/internal/core"

	"go.uber.org/zap"
)

// SearchWorkflow implements the search workflow
type SearchWorkflow struct {
	browser    core.BrowserPort
	repository core.RepositoryPort
	config     *core.Config
	logger     *zap.Logger
}

// NewSearchWorkflow creates a new search workflow
func NewSearchWorkflow(browser core.BrowserPort, repo core.RepositoryPort, config *core.Config, logger *zap.Logger) *SearchWorkflow {
	return &SearchWorkflow{
		browser:    browser,
		repository: repo,
		config:     config,
		logger:     logger,
	}
}

// Search performs a LinkedIn search and returns profile URLs
func (s *SearchWorkflow) Search(ctx context.Context, params *core.SearchParams) ([]string, error) {
	if params == nil {
		return nil, fmt.Errorf("search params cannot be nil")
	}

	if params.Keyword == "" {
		return nil, fmt.Errorf("search keyword is required")
	}

	s.logger.Info("Starting LinkedIn search",
		zap.String("keyword", params.Keyword),
		zap.Int("max_results", params.MaxResults),
	)

	// Build search URL
	searchURL := s.buildSearchURL(params)

	// Navigate to search page
	if err := s.browser.Navigate(ctx, searchURL); err != nil {
		return nil, fmt.Errorf("failed to navigate to search page: %w", err)
	}

	// Check for security challenge
	if err := s.handleSecurityChallenge(ctx); err != nil {
		return nil, fmt.Errorf("security challenge failed: %w", err)
	}

	allProfileURLs := make([]string, 0)
	page := 1

	for len(allProfileURLs) < params.MaxResults {
		// Wait for search results to load
		time.Sleep(2 * time.Second)

		// Scroll down to load more results
		// Scroll multiple times to ensure all lazy-loaded elements appear
		for i := 0; i < 3; i++ {
			if err := s.browser.HumanScroll(ctx, "down", 800); err != nil {
				s.logger.Warn("Failed to scroll", zap.Error(err))
			}
			time.Sleep(1 * time.Second)
		}

		// Extract profile URLs from current page
		profileURLs, err := s.ExtractProfileURLs(ctx)
		if err != nil {
			s.logger.Warn("Failed to extract profile URLs from current page", zap.Error(err))
			// If we fail to extract on the first page, it's a critical error
			if page == 1 {
				return nil, fmt.Errorf("failed to extract profile URLs: %w", err)
			}
			break // Stop if we can't extract anymore
		}

		// Add new unique URLs
		for _, url := range profileURLs {
			// Check DB for duplicate
			existingProfile, err := s.repository.GetProfileByURL(ctx, url)
			if err == nil && existingProfile != nil {
				s.logger.Debug("Skipping duplicate profile (already in DB)", zap.String("url", url))
				continue
			}

			// Create new profile in DB
			newProfile := &core.Profile{
				LinkedInURL: url,
				Status:      core.ProfileStatusDiscovered,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			if err := s.repository.CreateProfile(ctx, newProfile); err != nil {
				s.logger.Warn("Failed to save profile to DB", zap.String("url", url), zap.Error(err))
				// Continue anyway, maybe we can still process it in this session
			} else {
				s.logger.Debug("Saved new profile to DB", zap.String("url", url))
			}

			isDuplicate := false
			for _, existing := range allProfileURLs {
				if existing == url {
					isDuplicate = true
					break
				}
			}
			if !isDuplicate {
				allProfileURLs = append(allProfileURLs, url)
			}
		}

		s.logger.Info("Extracted profiles", 
			zap.Int("page", page), 
			zap.Int("new_profiles", len(profileURLs)), 
			zap.Int("total_profiles", len(allProfileURLs)),
		)

		// Check if we have enough results
		if len(allProfileURLs) >= params.MaxResults {
			break
		}

		// Go to next page
		page++
		nextPageButton := fmt.Sprintf("button[aria-label='Page %d']", page)
		
		// Check if next page button exists
		exists, err := s.browser.ElementExists(ctx, nextPageButton)
		if err != nil || !exists {
			s.logger.Info("No more pages found", zap.Int("last_page", page-1))
			break
		}

		// Click next page
		s.logger.Info("Navigating to next page", zap.Int("page", page))
		if err := s.browser.HumanClick(ctx, nextPageButton); err != nil {
			s.logger.Warn("Failed to click next page", zap.Error(err))
			break
		}
	}

	// Limit results if needed
	if params.MaxResults > 0 && len(allProfileURLs) > params.MaxResults {
		allProfileURLs = allProfileURLs[:params.MaxResults]
	}

	s.logger.Info("Search completed",
		zap.Int("profiles_found", len(allProfileURLs)),
	)

	return allProfileURLs, nil
}

// buildSearchURL constructs the LinkedIn search URL with parameters
func (s *SearchWorkflow) buildSearchURL(params *core.SearchParams) string {
	baseURL := s.config.LinkedIn.SearchURL

	// Build query parameters
	queryParams := url.Values{}
	queryParams.Set("keywords", params.Keyword)

	if params.Location != "" {
		queryParams.Set("geoUrn", params.Location)
	}

	// Note: Industry filtering might require different parameter format
	// LinkedIn search URL format: /search/results/people/?keywords=...
	fullURL := baseURL + "?" + queryParams.Encode()

	return fullURL
}

// ExtractProfileURLs extracts profile URLs from search results
func (s *SearchWorkflow) ExtractProfileURLs(ctx context.Context) ([]string, error) {
	// Wait for search results container (use extended timeout + retry and include current URL on failure)
	if err := s.browser.WaitForElement(ctx, s.config.Selectors.SearchResults, 20*time.Second); err != nil {
		s.logger.Debug("Initial wait for search results failed, retrying with shorter timeout", zap.Error(err))
		if err2 := s.browser.WaitForElement(ctx, s.config.Selectors.SearchResults, 10*time.Second); err2 != nil {
			curURL, _ := s.browser.GetCurrentURL(ctx)

			// Dump HTML for debugging
			if html, errHtml := s.browser.GetPageHTML(ctx); errHtml == nil {
				dumpPath := filepath.Join("data", fmt.Sprintf("debug_search_fail_%d.html", time.Now().Unix()))
				if errWrite := os.WriteFile(dumpPath, []byte(html), 0644); errWrite == nil {
					s.logger.Info("Dumped page HTML for debugging", zap.String("path", dumpPath))
				}
			}

			return nil, fmt.Errorf("search results not found (current_url=%s): %w", curURL, err2)
		}
	}

	// Extract profile URLs using the selector from config
	// We append the anchor tag selector to target the profile link within the result container
	// This uses the robust data-view-name selector defined in config
	selector := fmt.Sprintf("%s a[href*='/in/']", s.config.Selectors.SearchResults)
	
	rawURLs, err := s.browser.GetAttributes(ctx, selector, "href")
	if err != nil {
		// Fallback to legacy selectors if the new one fails
		s.logger.Warn("Failed to extract URLs with primary selector, trying fallbacks", zap.Error(err))
		return s.extractProfileURLsFallback(ctx)
	}

	// Filter and clean URLs
	cleanedURLs := make([]string, 0, len(rawURLs))
	seen := make(map[string]bool)

	for _, urlStr := range rawURLs {
		// Ensure it's a valid LinkedIn profile URL
		if !strings.Contains(urlStr, "/in/") || strings.Contains(urlStr, "/search") {
			continue
		}

		// Ensure full URL
		if !strings.HasPrefix(urlStr, "http") {
			urlStr = s.config.LinkedIn.BaseURL + urlStr
		}

		// Remove query parameters
		urlStr = strings.Split(urlStr, "?")[0]
		urlStr = strings.Split(urlStr, "#")[0]

		// Remove duplicates
		if seen[urlStr] {
			continue
		}
		seen[urlStr] = true

		cleanedURLs = append(cleanedURLs, urlStr)
	}

	s.logger.Info("Extracted profile URLs", zap.Int("count", len(cleanedURLs)))

	return cleanedURLs, nil
}

// extractProfileURLsFallback uses legacy iteration method
func (s *SearchWorkflow) extractProfileURLsFallback(ctx context.Context) ([]string, error) {
	profileURLs := make([]string, 0)

	// Try to find profile links by checking multiple result containers
	// LinkedIn typically shows 10 results per page
	for i := 1; i <= 20; i++ {
		// Try multiple potential selectors for the link
		potentialSelectors := []string{
			fmt.Sprintf("%s a[href*='/in/']:nth-of-type(%d)", s.config.Selectors.SearchResults, i),
			fmt.Sprintf(".entity-result__item:nth-of-type(%d) .entity-result__title-text a", i),
			fmt.Sprintf(".reusable-search__result-container:nth-of-type(%d) a.app-aware-link", i),
			fmt.Sprintf("li:nth-of-type(%d) .entity-result__title-text a", i),
		}

		var href string
		var err error

		for _, selector := range potentialSelectors {
			exists, _ := s.browser.ElementExists(ctx, selector)
			if exists {
				href, err = s.browser.GetAttribute(ctx, selector, "href")
				if err == nil && href != "" {
					break
				}
			}
		}
		
		if href == "" {
			continue
		}
		
		// Clean and validate URL
		if strings.Contains(href, "/in/") && !strings.Contains(href, "/search") {
			// Make sure it's a full URL
			if !strings.HasPrefix(href, "http") {
				href = s.config.LinkedIn.BaseURL + href
			}

			// Remove query parameters
			href = strings.Split(href, "?")[0]
			href = strings.Split(href, "#")[0]

			// Check for duplicates
			isDuplicate := false
			for _, existing := range profileURLs {
				if existing == href {
					isDuplicate = true
					break
				}
			}

			if !isDuplicate {
				profileURLs = append(profileURLs, href)
			}
		}
	}
	return profileURLs, nil
}

// handleSecurityChallenge checks for security challenges and pauses for manual intervention
func (s *SearchWorkflow) handleSecurityChallenge(ctx context.Context) error {
	_, err := s.browser.GetPageHTML(ctx)
	if err != nil {
		return fmt.Errorf("failed to get page HTML for security check: %w", err)
	}

	// Check for common security challenge indicators using element visibility
	// This avoids false positives where the code exists in the source (e.g. scripts) but is not active
	challengeReason := ""

	// Check 1: Human Security Enforcer Iframe
	if visible, _ := s.browser.IsElementVisible(ctx, "#humanSecurityEnforcerIframe"); visible {
		challengeReason = "Visible #humanSecurityEnforcerIframe"
	}

	// Check 2: Internal Captcha
	if challengeReason == "" {
		if visible, _ := s.browser.IsElementVisible(ctx, "#captcha-internal"); visible {
			challengeReason = "Visible #captcha-internal"
		}
	}

	// Check 3: Security Check Text
	if challengeReason == "" {
		// Use XPath to find text content
		if visible, _ := s.browser.IsElementVisible(ctx, "//*[contains(text(), \"Let's do a quick security check\")]"); visible {
			challengeReason = "Visible text 'Let's do a quick security check'"
		}
	}

	if challengeReason != "" {
		s.logger.Warn("⚠️ SECURITY CHALLENGE DETECTED! ⚠️", zap.String("reason", challengeReason))
		s.logger.Warn("The bot has been presented with a security check (CAPTCHA/Arkose).")
		s.logger.Warn("Please switch to the browser window and solve the challenge MANUALLY.")
		s.logger.Warn("The bot will check every 5 seconds if the challenge is resolved.")
		s.logger.Warn("Waiting for up to 5 minutes...")

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
				// Check if we are back to a normal page
				// We can check if the challenge elements are gone, or if search results are present
				html, err := s.browser.GetPageHTML(ctx)
				if err != nil {
					s.logger.Error("Failed to check page status", zap.Error(err))
					continue
				}

				// If challenge markers are gone, we assume success
				stillHasChallenge := strings.Contains(html, "humanSecurityEnforcerIframe") ||
					strings.Contains(html, "grecaptcha-badge") ||
					strings.Contains(html, "security-challenge")

				if !stillHasChallenge {
					s.logger.Info("Security challenge appears to be resolved. Resuming workflow...")
					// Give it a moment to fully load the target page
					time.Sleep(3 * time.Second)
					return nil
				}
			}
		}
	}

	return nil
}


