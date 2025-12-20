package core

import "time"

// Profile represents a LinkedIn profile in the database
type Profile struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	LinkedInURL string   `gorm:"uniqueIndex;not null" json:"linkedin_url"`
	Status     string    `gorm:"index;not null" json:"status"` // Scanned, Connected, Ignored
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// History represents an action log entry
type History struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ActionType string   `gorm:"index;not null" json:"action_type"` // Login, Search, Connect
	Details   string    `gorm:"type:text" json:"details"`
	Timestamp time.Time `gorm:"index;not null" json:"timestamp"`
}

// Task represents a workflow task
type Task struct {
	Type        string                 `json:"type"`         // Auth, Search, Connect
	Params      map[string]interface{} `json:"params"`       // Task-specific parameters
	Priority    int                    `json:"priority"`     // Task priority (higher = more important)
	RetryCount  int                    `json:"retry_count"`  // Number of retries attempted
	MaxRetries  int                    `json:"max_retries"`  // Maximum retries allowed
}

// SearchParams holds parameters for a search operation
type SearchParams struct {
	Keyword     string `json:"keyword"`
	MaxResults  int    `json:"max_results"`
	Location    string `json:"location,omitempty"`
	Industry    string `json:"industry,omitempty"`
}

// ConnectParams holds parameters for a connection request
type ConnectParams struct {
	ProfileURL string `json:"profile_url"`
	Note       string `json:"note"`
	Name       string `json:"name,omitempty"`
}

// StealthConfig holds stealth/humanization parameters
type StealthConfig struct {
	TypingSpeedMin   int     `mapstructure:"typing_speed_min"`   // WPM minimum
	TypingSpeedMax   int     `mapstructure:"typing_speed_max"`   // WPM maximum
	TypoProbability  float64 `mapstructure:"typo_probability"`    // Probability of typo (0.0-1.0)
	MouseSpeedMin    float64 `mapstructure:"mouse_speed_min"`     // Minimum mouse speed multiplier
	MouseSpeedMax    float64 `mapstructure:"mouse_speed_max"`     // Maximum mouse speed multiplier
	OvershootChance  float64 `mapstructure:"overshoot_chance"`    // Chance of mouse overshoot (0.0-1.0)
	ScrollChunkMin   int     `mapstructure:"scroll_chunk_min"`    // Minimum scroll chunk size
	ScrollChunkMax   int     `mapstructure:"scroll_chunk_max"`    // Maximum scroll chunk size
	BaseDelayMin     float64 `mapstructure:"base_delay_min"`      // Minimum base delay in seconds
	BaseDelayMax     float64 `mapstructure:"base_delay_max"`      // Maximum base delay in seconds
	ViewportWidthMin int     `mapstructure:"viewport_width_min"`  // Minimum viewport width
	ViewportWidthMax int     `mapstructure:"viewport_width_max"`  // Maximum viewport width
	ViewportHeightMin int    `mapstructure:"viewport_height_min"` // Minimum viewport height
	ViewportHeightMax int    `mapstructure:"viewport_height_max"` // Maximum viewport height
	DebugStealth      bool   `mapstructure:"debug_stealth"`       // Enable stealth debugging (slows down actions)
}

// LimitsConfig holds rate limiting and working hours configuration
type LimitsConfig struct {
	MaxActionsPerDay int    `mapstructure:"max_actions_per_day"`
	WorkingHoursStart string `mapstructure:"working_hours_start"` // Format: "09:00"
	WorkingHoursEnd   string `mapstructure:"working_hours_end"`   // Format: "17:00"
	ConnectCooldownMin int   `mapstructure:"connect_cooldown_min"` // Minutes
	ConnectCooldownMax int   `mapstructure:"connect_cooldown_max"` // Minutes
}

// SelectorsConfig holds CSS/XPath selectors
type SelectorsConfig struct {
	LoginEmailInput    string `mapstructure:"login_email_input"`
	LoginPasswordInput string `mapstructure:"login_password_input"`
	LoginSubmitButton  string `mapstructure:"login_submit_button"`
	SearchInput        string `mapstructure:"search_input"`
	SearchResults      string `mapstructure:"search_results"`
	ProfileConnectBtn  string `mapstructure:"profile_connect_button"`
	ConnectNoteTextarea string `mapstructure:"connect_note_textarea"`
	ConnectSendButton  string `mapstructure:"connect_send_button"`
	TwoFactorChallenge string `mapstructure:"two_factor_challenge"`
	FeedContainer      string `mapstructure:"feed_container"`
}

// Config represents the application configuration
type Config struct {
	Credentials struct {
		Email    string `mapstructure:"email"`
		Password string `mapstructure:"password"`
	} `mapstructure:"credentials"`
	
	Stealth  StealthConfig  `mapstructure:"stealth"`
	Limits   LimitsConfig   `mapstructure:"limits"`
	Selectors SelectorsConfig `mapstructure:"selectors"`
	
	LinkedIn struct {
		BaseURL      string `mapstructure:"base_url"`
		SearchURL    string `mapstructure:"search_url"`
		LoginURL     string `mapstructure:"login_url"`
	} `mapstructure:"linkedin"`
	
	Database struct {
		Path string `mapstructure:"path"`
	} `mapstructure:"database"`
	
	Session struct {
		CookiesPath string `mapstructure:"cookies_path"`
	} `mapstructure:"session"`
}

