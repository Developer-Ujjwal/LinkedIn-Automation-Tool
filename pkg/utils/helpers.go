package utils

import (
	"fmt"
	"math/rand"
	"time"
)

// IsWithinWorkingHours checks if current time is within working hours
func IsWithinWorkingHours(startTime, endTime string) (bool, error) {
	now := time.Now()
	
	// Parse start time
	start, err := time.Parse("15:04", startTime)
	if err != nil {
		return false, fmt.Errorf("invalid start time format: %w", err)
	}
	
	// Parse end time
	end, err := time.Parse("15:04", endTime)
	if err != nil {
		return false, fmt.Errorf("invalid end time format: %w", err)
	}
	
	// Create time objects for today with the parsed hours
	startToday := time.Date(now.Year(), now.Month(), now.Day(), start.Hour(), start.Minute(), 0, 0, now.Location())
	endToday := time.Date(now.Year(), now.Month(), now.Day(), end.Hour(), end.Minute(), 0, 0, now.Location())
	
	// Handle case where end time is next day (e.g., 23:00 to 02:00)
	if endToday.Before(startToday) {
		endToday = endToday.Add(24 * time.Hour)
	}
	
	// Check if current time is within range
	return now.After(startToday) && now.Before(endToday), nil
}

// RandomCooldown returns a random cooldown duration between min and max minutes
func RandomCooldown(minMinutes, maxMinutes int) time.Duration {
	if minMinutes < 0 {
		minMinutes = 0
	}
	if maxMinutes < minMinutes {
		maxMinutes = minMinutes
	}
	
	if minMinutes == maxMinutes {
		return time.Duration(minMinutes) * time.Minute
	}
	
	minutes := minMinutes + rand.Intn(maxMinutes-minMinutes+1)
	return time.Duration(minutes) * time.Minute
}

// FormatDuration formats a duration in a human-readable way
func FormatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

