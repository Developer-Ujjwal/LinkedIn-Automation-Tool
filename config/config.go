package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
	"linkedin-automation/internal/core"
)

// Load loads configuration from config.yaml and environment variables
func Load(configPath string) (*core.Config, error) {
	cfg := &core.Config{}

	// Set default values
	setDefaults()

	// Set config file path
	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		// Default to config.yaml in current directory
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")
	}

	// Enable environment variable support
	viper.SetEnvPrefix("LINKEDIN_BOT")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found, but we can continue with defaults and env vars
	}

	// Unmarshal into struct
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Override credentials from environment if present
	if email := os.Getenv("LINKEDIN_BOT_EMAIL"); email != "" {
		cfg.Credentials.Email = email
	}
	if password := os.Getenv("LINKEDIN_BOT_PASSWORD"); password != "" {
		cfg.Credentials.Password = password
	}

	// Validate required fields
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Credentials (should be set via env or config)
	viper.SetDefault("credentials.email", "")
	viper.SetDefault("credentials.password", "")

	// Stealth defaults
	viper.SetDefault("stealth.typing_speed_min", 40)
	viper.SetDefault("stealth.typing_speed_max", 80)
	viper.SetDefault("stealth.typo_probability", 0.02) // 1 in 50 chars
	viper.SetDefault("stealth.mouse_speed_min", 0.5)
	viper.SetDefault("stealth.mouse_speed_max", 1.5)
	viper.SetDefault("stealth.overshoot_chance", 0.3)
	viper.SetDefault("stealth.scroll_chunk_min", 50)
	viper.SetDefault("stealth.scroll_chunk_max", 200)
	viper.SetDefault("stealth.base_delay_min", 0.1)
	viper.SetDefault("stealth.base_delay_max", 0.5)
	viper.SetDefault("stealth.viewport_width_min", 1920)
	viper.SetDefault("stealth.viewport_width_max", 1920)
	viper.SetDefault("stealth.viewport_height_min", 1080)
	viper.SetDefault("stealth.viewport_height_max", 1080)

	// Limits defaults
	viper.SetDefault("limits.max_actions_per_day", 50)
	viper.SetDefault("limits.working_hours_start", "09:00")
	viper.SetDefault("limits.working_hours_end", "17:00")
	viper.SetDefault("limits.connect_cooldown_min", 3)
	viper.SetDefault("limits.connect_cooldown_max", 8)

	// LinkedIn URLs
	viper.SetDefault("linkedin.base_url", "https://www.linkedin.com")
	viper.SetDefault("linkedin.login_url", "https://www.linkedin.com/login")
	viper.SetDefault("linkedin.search_url", "https://www.linkedin.com/search/results/people/")

	// Database
	viper.SetDefault("database.path", "data/bot.db")

	// Session
	viper.SetDefault("session.cookies_path", "data/cookies.json")

	// Selectors (default LinkedIn selectors - may need updates)
	viper.SetDefault("selectors.login_email_input", "#username")
	viper.SetDefault("selectors.login_password_input", "#password")
	viper.SetDefault("selectors.login_submit_button", "button[type='submit']")
	viper.SetDefault("selectors.search_input", "input[placeholder*='Search']")
	viper.SetDefault("selectors.search_results", ".reusable-search__result-container")
	viper.SetDefault("selectors.profile_connect_button", "button[aria-label*='Connect']")
	viper.SetDefault("selectors.connect_note_textarea", "textarea[name='message']")
	viper.SetDefault("selectors.connect_send_button", "button[aria-label*='Send']")
	viper.SetDefault("selectors.two_factor_challenge", "input[type='text'][name='pin']")
}

// validateConfig validates that required configuration fields are set
func validateConfig(cfg *core.Config) error {
	if cfg.Credentials.Email == "" {
		return fmt.Errorf("credentials.email is required (set via config or LINKEDIN_BOT_EMAIL env var)")
	}
	if cfg.Credentials.Password == "" {
		return fmt.Errorf("credentials.password is required (set via config or LINKEDIN_BOT_PASSWORD env var)")
	}
	if cfg.Database.Path == "" {
		return fmt.Errorf("database.path is required")
	}
	if cfg.Session.CookiesPath == "" {
		return fmt.Errorf("session.cookies_path is required")
	}
	return nil
}

