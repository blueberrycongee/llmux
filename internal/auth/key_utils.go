// Package auth provides API key authentication and multi-tenant support.
package auth

import (
	"time"

	"github.com/google/uuid"
)

// GenerateUUID generates a new UUID string.
func GenerateUUID() string {
	return uuid.New().String()
}

// ParseDuration parses a duration string (e.g., "30d", "1h", "7d") and returns the expiry time.
func ParseDuration(duration string) *time.Time {
	if duration == "" || duration == "-1" {
		return nil // Never expires
	}

	seconds := DurationInSeconds(duration)
	if seconds <= 0 {
		return nil
	}

	expiry := time.Now().Add(time.Duration(seconds) * time.Second)
	return &expiry
}

// DurationInSeconds converts a duration string to seconds.
func DurationInSeconds(duration string) int64 {
	if duration == "" {
		return 0
	}

	// Parse the duration string
	var value int64
	var unit string

	for i, c := range duration {
		if c >= '0' && c <= '9' {
			value = value*10 + int64(c-'0')
		} else {
			unit = duration[i:]
			break
		}
	}

	switch unit {
	case "s", "sec", "second", "seconds":
		return value
	case "m", "min", "minute", "minutes":
		return value * 60
	case "h", "hr", "hour", "hours":
		return value * 3600
	case "d", "day", "days":
		return value * 86400
	case "w", "week", "weeks":
		return value * 604800
	case "mo", "month", "months":
		return value * 2592000 // 30 days
	case "y", "year", "years":
		return value * 31536000 // 365 days
	default:
		return 0
	}
}

// CalculateRotationTime calculates the next rotation time based on the interval.
func CalculateRotationTime(interval string) *time.Time {
	seconds := DurationInSeconds(interval)
	if seconds <= 0 {
		return nil
	}

	rotationTime := time.Now().Add(time.Duration(seconds) * time.Second)
	return &rotationTime
}
