package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"linkedin-automation/config"
	"linkedin-automation/internal/browser"
	"linkedin-automation/internal/core"
	"linkedin-automation/internal/repository"
	"linkedin-automation/internal/stealth"
	"linkedin-automation/internal/workflows"
	"linkedin-automation/pkg/utils"

	"go.uber.org/zap"
)

var (
	configPath = flag.String("config", "config/config.yaml", "Path to configuration file")
	keyword    = flag.String("keyword", "", "Search keyword (required)")
	maxResults = flag.Int("max", 10, "Maximum number of profiles to connect with")
	location   = flag.String("location", "", "Location filter for search (optional)")
	note       = flag.String("note", "", "Connection note template (overrides config)")
	scan       = flag.Bool("scan", false, "Scan for new connections")
	followup   = flag.Bool("followup", false, "Send follow-up messages to new connections")
)

func main() {
	flag.Parse()

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("LinkedIn Automation Bot - Starting",
		zap.String("version", "1.0.0"),
		zap.String("purpose", "Educational POC"),
	)

	// Validate required flags
	if !*scan && !*followup && *keyword == "" {
		logger.Fatal("Keyword is required for search mode. Use -keyword flag. Or use -scan / -followup.")
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	logger.Info("Configuration loaded", zap.String("config_path", *configPath))

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Info("Shutdown signal received, gracefully shutting down...")
		cancel()
	}()

	// Initialize components
	logger.Info("Initializing components...")

	// Initialize stealth engine
	stealthEngine := stealth.NewStealth(&cfg.Stealth)
	logger.Info("Stealth engine initialized")

	// Initialize browser
	browserInstance := browser.NewInstance(cfg, stealthEngine, logger)
	if err := browserInstance.Initialize(ctx); err != nil {
		logger.Fatal("Failed to initialize browser", zap.Error(err))
	}
	defer func() {
		if err := browserInstance.Close(ctx); err != nil {
			logger.Error("Failed to close browser", zap.Error(err))
		}
	}()

	logger.Info("Browser initialized")

	// Initialize repository
	repo, err := repository.NewSQLiteRepository(cfg.Database.Path)
	if err != nil {
		logger.Fatal("Failed to initialize repository", zap.Error(err))
	}
	defer func() {
		if err := repo.Close(); err != nil {
			logger.Error("Failed to close repository", zap.Error(err))
		}
	}()

	logger.Info("Repository initialized", zap.String("db_path", cfg.Database.Path))

	// Initialize workflows
	authWorkflow := workflows.NewAuthWorkflow(browserInstance, cfg, logger)
	searchWorkflow := workflows.NewSearchWorkflow(browserInstance, repo, cfg, logger)
	connectWorkflow := workflows.NewConnectWorkflow(browserInstance, repo, cfg, logger)
	messagingWorkflow := workflows.NewMessagingWorkflow(browserInstance, repo, cfg, logger)

	logger.Info("Workflows initialized")

	// Run main automation loop
	if err := runAutomation(ctx, cfg, repo, authWorkflow, searchWorkflow, connectWorkflow, messagingWorkflow, logger); err != nil {
		logger.Fatal("Automation failed", zap.Error(err))
	}

	logger.Info("Automation completed successfully")
}

// runAutomation runs the main automation loop
func runAutomation(
	ctx context.Context,
	cfg *core.Config,
	repo core.RepositoryPort,
	authWorkflow *workflows.AuthWorkflow,
	searchWorkflow *workflows.SearchWorkflow,
	connectWorkflow *workflows.ConnectWorkflow,
	messagingWorkflow *workflows.MessagingWorkflow,
	logger *zap.Logger,
) error {
	// Step 1: Authenticate
	logger.Info("Step 1: Authenticating...")
	if err := authWorkflow.Authenticate(ctx); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}
	logger.Info("Authentication successful")

	// Step 2: Check working hours
	logger.Info("Step 2: Checking working hours...")
	withinHours, err := utils.IsWithinWorkingHours(cfg.Limits.WorkingHoursStart, cfg.Limits.WorkingHoursEnd)
	if err != nil {
		logger.Warn("Failed to check working hours", zap.Error(err))
		withinHours = true // Continue if check fails
	}

	if !withinHours {
		logger.Info("Outside working hours, waiting...",
			zap.String("start", cfg.Limits.WorkingHoursStart),
			zap.String("end", cfg.Limits.WorkingHoursEnd),
		)
		// Wait until working hours
		// For simplicity, we'll just log and continue
		// In production, you might want to wait or exit
	}

	// Handle Scan Mode
	if *scan {
		logger.Info("Running in Scan Mode")
		if err := messagingWorkflow.ScanNewConnections(ctx); err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}
		// If only scanning, we can return here unless followup is also requested
		if !*followup && *keyword == "" {
			return nil
		}
	}

	// Handle Follow-up Mode
	if *followup {
		logger.Info("Running in Follow-up Mode")
		if err := messagingWorkflow.SendFollowUpMessages(ctx); err != nil {
			return fmt.Errorf("follow-up failed: %w", err)
		}
		// If only followup, return here
		if *keyword == "" {
			return nil
		}
	}

	// If no keyword provided (and we handled scan/followup), we are done
	if *keyword == "" {
		return nil
	}

	// Step 3: Check rate limits
	logger.Info("Step 3: Checking rate limits...")
	canConnect, err := repo.CanPerformAction(
		ctx, "Connect", cfg.Limits.MaxActionsPerDay,
	)
	if err != nil {
		logger.Warn("Failed to check rate limits", zap.Error(err))
		canConnect = true // Continue if check fails
	}

	if !canConnect {
		logger.Warn("Daily connection limit reached",
			zap.Int("limit", cfg.Limits.MaxActionsPerDay),
		)
		return fmt.Errorf("daily connection limit reached")
	}

	// Step 4: Perform search
	logger.Info("Step 4: Performing search...",
		zap.String("keyword", *keyword),
		zap.Int("max_results", *maxResults),
	)

	searchParams := &core.SearchParams{
		Keyword:    *keyword,
		MaxResults: *maxResults,
		Location:   *location,
	}

	profileURLs, err := searchWorkflow.Search(ctx, searchParams)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(profileURLs) == 0 {
		logger.Warn("No profiles found in search results")
		return nil
	}

	logger.Info("Search completed",
		zap.Int("profiles_found", len(profileURLs)),
	)

	// Step 5: Send connection requests
	logger.Info("Step 5: Sending connection requests...")

	connectedCount := 0
	skippedCount := 0
	errorCount := 0

	for i, profileURL := range profileURLs {
		// Check context cancellation
		select {
		case <-ctx.Done():
			logger.Info("Context cancelled, stopping automation")
			return ctx.Err()
		default:
		}

		// Check rate limit before each connection
		canConnect, err := repo.CanPerformAction(
			ctx, "Connect", cfg.Limits.MaxActionsPerDay,
		)
		if err != nil {
			logger.Warn("Failed to check rate limit", zap.Error(err))
		} else if !canConnect {
			logger.Warn("Daily limit reached, stopping connections",
				zap.Int("connected_so_far", connectedCount),
			)
			break
		}

		logger.Info("Processing profile",
			zap.Int("index", i+1),
			zap.Int("total", len(profileURLs)),
			zap.String("url", profileURL),
		)

		// Determine note to use: flag overrides config
		noteToUse := *note
		if noteToUse == "" {
			noteToUse = cfg.Connection.NoteTemplate
		}

		// Send connection request
		connectParams := &core.ConnectParams{
			ProfileURL: profileURL,
			Note:       noteToUse,
		}

		if err := connectWorkflow.SendConnectionRequest(ctx, connectParams); err != nil {
			logger.Error("Failed to send connection request",
				zap.String("url", profileURL),
				zap.Error(err),
			)
			errorCount++
			continue
		}

		// Check if it was skipped (already connected, etc.)
		shouldSkip, _ := connectWorkflow.ShouldSkipProfile(ctx, profileURL)
		if shouldSkip {
			skippedCount++
			logger.Info("Profile skipped", zap.String("url", profileURL))
		} else {
			connectedCount++
			logger.Info("Connection request sent successfully",
				zap.String("url", profileURL),
				zap.Int("total_connected", connectedCount),
			)
		}

		// Cooldown between connections (except for the last one)
		if i < len(profileURLs)-1 {
			cooldown := utils.RandomCooldown(
				cfg.Limits.ConnectCooldownMin,
				cfg.Limits.ConnectCooldownMax,
			)
			logger.Info("Cooldown before next connection",
				zap.String("duration", utils.FormatDuration(cooldown)),
			)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(cooldown):
				// Continue
			}
		}
	}

	// Summary
	logger.Info("Automation summary",
		zap.Int("total_profiles", len(profileURLs)),
		zap.Int("connected", connectedCount),
		zap.Int("skipped", skippedCount),
		zap.Int("errors", errorCount),
	)

	return nil
}

