package repository

import (
	"context"
	"time"

	"linkedin-automation/internal/core"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SQLiteRepository implements RepositoryPort using SQLite via GORM
type SQLiteRepository struct {
	db *gorm.DB
}

// NewSQLiteRepository creates a new SQLite repository
func NewSQLiteRepository(dbPath string) (*SQLiteRepository, error) {
	// Configure GORM logger (silent in production, can be verbose for debugging)
	config := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	db, err := gorm.Open(sqlite.Open(dbPath), config)
	if err != nil {
		return nil, err
	}

	repo := &SQLiteRepository{db: db}

	// Auto-migrate schema
	if err := repo.Migrate(context.Background()); err != nil {
		return nil, err
	}

	return repo, nil
}

// Migrate runs database migrations
func (r *SQLiteRepository) Migrate(ctx context.Context) error {
	return r.db.WithContext(ctx).AutoMigrate(
		&core.Profile{},
		&core.History{},
	)
}

// CreateProfile creates a new profile record
func (r *SQLiteRepository) CreateProfile(ctx context.Context, profile *core.Profile) error {
	if profile.CreatedAt.IsZero() {
		profile.CreatedAt = time.Now()
	}
	if profile.UpdatedAt.IsZero() {
		profile.UpdatedAt = time.Now()
	}

	result := r.db.WithContext(ctx).Create(profile)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

// GetProfileByURL retrieves a profile by LinkedIn URL
func (r *SQLiteRepository) GetProfileByURL(ctx context.Context, url string) (*core.Profile, error) {
	var profile core.Profile
	result := r.db.WithContext(ctx).Where("linked_in_url = ?", url).First(&profile)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil // Profile not found, not an error
		}
		return nil, result.Error
	}

	return &profile, nil
}

// UpdateProfileStatus updates the status of a profile
func (r *SQLiteRepository) UpdateProfileStatus(ctx context.Context, url string, status string) error {
	profile := &core.Profile{
		UpdatedAt: time.Now(),
		Status:    status,
	}

	result := r.db.WithContext(ctx).
		Model(&core.Profile{}).
		Where("linked_in_url = ?", url).
		Updates(profile)

	if result.Error != nil {
		return result.Error
	}

	return nil
}

// GetProfilesByStatus retrieves all profiles with a specific status
func (r *SQLiteRepository) GetProfilesByStatus(ctx context.Context, status string) ([]*core.Profile, error) {
	var profiles []*core.Profile
	result := r.db.WithContext(ctx).Where("status = ?", status).Find(&profiles)
	if result.Error != nil {
		return nil, result.Error
	}

	return profiles, nil
}

// GetPendingFollowups returns profiles that are connected but haven't received a message
func (r *SQLiteRepository) GetPendingFollowups(ctx context.Context, limit int) ([]*core.Profile, error) {
	var profiles []*core.Profile
	result := r.db.WithContext(ctx).
		Where("status = ? AND last_message_sent_at IS NULL", core.ProfileStatusConnected).
		Limit(limit).
		Find(&profiles)

	if result.Error != nil {
		return nil, result.Error
	}

	return profiles, nil
}

// MarkAsConnected updates a profile status to Connected
func (r *SQLiteRepository) MarkAsConnected(ctx context.Context, linkedinURL string) error {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&core.Profile{}).
		Where("linked_in_url = ?", linkedinURL).
		Updates(map[string]interface{}{
			"status":       core.ProfileStatusConnected,
			"connected_at": &now,
			"updated_at":   now,
		})

	return result.Error
}

// LogMessageSent updates the profile status and logs the message in history
func (r *SQLiteRepository) LogMessageSent(ctx context.Context, profileID uint, content string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		
		// Update profile
		if err := tx.WithContext(ctx).Model(&core.Profile{}).
			Where("id = ?", profileID).
			Updates(map[string]interface{}{
				"status":               core.ProfileStatusMessageSent,
				"last_message_sent_at": &now,
				"updated_at":           now,
			}).Error; err != nil {
			return err
		}

		// Create history entry
		history := &core.History{
			ActionType: "Message",
			Details:    content,
			Timestamp:  now,
		}
		
		if err := tx.WithContext(ctx).Create(history).Error; err != nil {
			return err
		}

		return nil
	})
}

// CreateHistory creates a new history record
func (r *SQLiteRepository) CreateHistory(ctx context.Context, history *core.History) error {
	if history.Timestamp.IsZero() {
		history.Timestamp = time.Now()
	}

	result := r.db.WithContext(ctx).Create(history)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

// GetTodayActionCount counts actions of a specific type performed today
func (r *SQLiteRepository) GetTodayActionCount(ctx context.Context, actionType string) (int64, error) {
	// Get start of today
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var count int64
	result := r.db.WithContext(ctx).
		Model(&core.History{}).
		Where("action_type = ? AND timestamp >= ?", actionType, startOfDay).
		Count(&count)

	if result.Error != nil {
		return 0, result.Error
	}

	return count, nil
}

// GetHistoryByDateRange retrieves history records within a date range
func (r *SQLiteRepository) GetHistoryByDateRange(ctx context.Context, start, end time.Time) ([]*core.History, error) {
	var histories []*core.History
	result := r.db.WithContext(ctx).
		Where("timestamp >= ? AND timestamp <= ?", start, end).
		Order("timestamp DESC").
		Find(&histories)

	if result.Error != nil {
		return nil, result.Error
	}

	return histories, nil
}

// CanPerformAction checks if an action can be performed based on daily limits
func (r *SQLiteRepository) CanPerformAction(ctx context.Context, actionType string, dailyLimit int) (bool, error) {
	count, err := r.GetTodayActionCount(ctx, actionType)
	if err != nil {
		return false, err
	}

	return count < int64(dailyLimit), nil
}

// Close closes the database connection
func (r *SQLiteRepository) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}

// GetDB returns the underlying GORM database instance (for advanced usage)
func (r *SQLiteRepository) GetDB() *gorm.DB {
	return r.db
}

