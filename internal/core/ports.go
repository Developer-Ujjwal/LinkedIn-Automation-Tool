package core

import (
	"context"
	"time"
)

// BrowserPort defines the interface for browser operations
type BrowserPort interface {
	// Initialize sets up the browser instance with stealth features
	Initialize(ctx context.Context) error
	
	// Navigate navigates to a URL with human-like delays
	Navigate(ctx context.Context, url string) error
	
	// HumanType types text into an element with human-like behavior
	HumanType(ctx context.Context, selector string, text string) error
	
	// HumanClick clicks an element with Bézier curve mouse movement
	HumanClick(ctx context.Context, selector string) error
	
	// HumanScroll scrolls the page with human-like acceleration/deceleration
	HumanScroll(ctx context.Context, direction string, distance int) error
	
	// WaitForElement waits for an element to appear with timeout
	WaitForElement(ctx context.Context, selector string, timeout time.Duration) error
	
	// GetText extracts text content from an element
	GetText(ctx context.Context, selector string) (string, error)
	
	// GetAttribute gets an attribute value from an element
	GetAttribute(ctx context.Context, selector string, attr string) (string, error)
	
	// ElementExists checks if an element exists on the page
	ElementExists(ctx context.Context, selector string) (bool, error)
	
	// GetCurrentURL returns the current page URL
	GetCurrentURL(ctx context.Context) (string, error)
	
	// SaveCookies saves browser cookies to a file
	SaveCookies(ctx context.Context, path string) error
	
	// LoadCookies loads browser cookies from a file
	LoadCookies(ctx context.Context, path string) error
	
	// Close closes the browser instance
	Close(ctx context.Context) error
}

// RepositoryPort defines the interface for data persistence
type RepositoryPort interface {
	// Profile operations
	CreateProfile(ctx context.Context, profile *Profile) error
	GetProfileByURL(ctx context.Context, url string) (*Profile, error)
	UpdateProfileStatus(ctx context.Context, url string, status string) error
	GetProfilesByStatus(ctx context.Context, status string) ([]*Profile, error)
	
	// History operations
	CreateHistory(ctx context.Context, history *History) error
	GetTodayActionCount(ctx context.Context, actionType string) (int64, error)
	GetHistoryByDateRange(ctx context.Context, start, end time.Time) ([]*History, error)
	
	// Rate limiting
	CanPerformAction(ctx context.Context, actionType string, dailyLimit int) (bool, error)
	
	// Database management
	Migrate(ctx context.Context) error
	Close() error
}

// StealthPort defines the interface for stealth/humanization operations
type StealthPort interface {
	// MoveMouse moves the mouse using Bézier curves with optional overshoot
	MoveMouse(ctx context.Context, startX, startY, endX, endY float64) error
	
	// HumanType simulates human typing with variable speed and typos
	HumanType(ctx context.Context, text string, wpmMin, wpmMax int, typoProb float64) error
	
	// RandomSleep sleeps for a randomized duration (never exact integers)
	RandomSleep(ctx context.Context, baseSeconds, varianceSeconds float64)
	
	// HumanScroll scrolls with acceleration/deceleration and pauses
	HumanScroll(ctx context.Context, direction string, distance int, chunkMin, chunkMax int) error
}

// AuthWorkflowPort defines the interface for authentication workflow
type AuthWorkflowPort interface {
	// Authenticate performs login or loads existing session
	Authenticate(ctx context.Context) error
	
	// IsAuthenticated checks if the current session is valid
	IsAuthenticated(ctx context.Context) (bool, error)
	
	// Handle2FA waits for manual 2FA intervention
	Handle2FA(ctx context.Context) error
}

// SearchWorkflowPort defines the interface for search workflow
type SearchWorkflowPort interface {
	// Search performs a LinkedIn search and returns profile URLs
	Search(ctx context.Context, params *SearchParams) ([]string, error)
	
	// ExtractProfileURLs extracts profile URLs from search results
	ExtractProfileURLs(ctx context.Context) ([]string, error)
}

// ConnectWorkflowPort defines the interface for connection workflow
type ConnectWorkflowPort interface {
	// SendConnectionRequest sends a connection request with a personalized note
	SendConnectionRequest(ctx context.Context, params *ConnectParams) error
	
	// ExtractProfileName extracts the profile name from a profile page
	ExtractProfileName(ctx context.Context) (string, error)
	
	// ShouldSkipProfile checks if a profile should be skipped (already connected, etc.)
	ShouldSkipProfile(ctx context.Context, profileURL string) (bool, error)
}

