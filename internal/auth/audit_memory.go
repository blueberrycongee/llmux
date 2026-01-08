// Package auth provides API key authentication and multi-tenant support.
package auth

import (
	"sync"
	"time"
)

// MemoryAuditLogStore implements AuditLogStore using in-memory storage.
// Suitable for development and testing. For production, use a persistent store.
type MemoryAuditLogStore struct {
	mu   sync.RWMutex
	logs []*AuditLog
}

// NewMemoryAuditLogStore creates a new in-memory audit log store.
func NewMemoryAuditLogStore() *MemoryAuditLogStore {
	return &MemoryAuditLogStore{
		logs: make([]*AuditLog, 0),
	}
}

// CreateAuditLog records a new audit log entry.
func (s *MemoryAuditLogStore) CreateAuditLog(log *AuditLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Set ID if not provided
	if log.ID == "" {
		log.ID = generateAuditID()
	}

	// Set timestamp if not provided
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now().UTC()
	}

	// Append to logs
	logCopy := *log
	s.logs = append(s.logs, &logCopy)

	return nil
}

// GetAuditLog retrieves a single audit log by ID.
func (s *MemoryAuditLogStore) GetAuditLog(id string) (*AuditLog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, log := range s.logs {
		if log.ID == id {
			logCopy := *log
			return &logCopy, nil
		}
	}

	return nil, nil
}

// ListAuditLogs returns audit logs matching the filter.
func (s *MemoryAuditLogStore) ListAuditLogs(filter AuditLogFilter) ([]*AuditLog, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []*AuditLog

	for _, log := range s.logs {
		// Apply filters
		if !s.matchesFilter(log, filter) {
			continue
		}
		logCopy := *log
		filtered = append(filtered, &logCopy)
	}

	// Sort by timestamp descending (most recent first)
	// Using simple bubble sort for small datasets
	for i := 0; i < len(filtered)-1; i++ {
		for j := 0; j < len(filtered)-i-1; j++ {
			if filtered[j].Timestamp.Before(filtered[j+1].Timestamp) {
				filtered[j], filtered[j+1] = filtered[j+1], filtered[j]
			}
		}
	}

	total := int64(len(filtered))

	// Apply pagination
	if filter.Offset >= len(filtered) {
		return []*AuditLog{}, total, nil
	}

	end := filter.Offset + filter.Limit
	if end > len(filtered) || filter.Limit == 0 {
		end = len(filtered)
	}

	return filtered[filter.Offset:end], total, nil
}

// matchesFilter checks if an audit log matches the given filter.
func (s *MemoryAuditLogStore) matchesFilter(log *AuditLog, filter AuditLogFilter) bool {
	// Time range filter
	if !filter.StartTime.IsZero() && log.Timestamp.Before(filter.StartTime) {
		return false
	}
	if !filter.EndTime.IsZero() && log.Timestamp.After(filter.EndTime) {
		return false
	}

	// Actor filters
	if filter.ActorID != nil && log.ActorID != *filter.ActorID {
		return false
	}
	if filter.ActorType != nil && log.ActorType != *filter.ActorType {
		return false
	}

	// Action filter
	if filter.Action != nil && log.Action != *filter.Action {
		return false
	}

	// Object filters
	if filter.ObjectType != nil && log.ObjectType != *filter.ObjectType {
		return false
	}
	if filter.ObjectID != nil && log.ObjectID != *filter.ObjectID {
		return false
	}

	// Context filters
	if filter.TeamID != nil && (log.TeamID == nil || *log.TeamID != *filter.TeamID) {
		return false
	}
	if filter.OrganizationID != nil && (log.OrganizationID == nil || *log.OrganizationID != *filter.OrganizationID) {
		return false
	}

	// Success filter
	if filter.Success != nil && log.Success != *filter.Success {
		return false
	}

	return true
}

// GetAuditLogStats returns aggregated audit statistics.
func (s *MemoryAuditLogStore) GetAuditLogStats(filter AuditLogFilter) (*AuditLogStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &AuditLogStats{
		ActionCounts:     make(map[string]int64),
		ObjectTypeCounts: make(map[string]int64),
	}

	actors := make(map[string]bool)

	for _, log := range s.logs {
		if !s.matchesFilter(log, filter) {
			continue
		}

		stats.TotalEvents++

		if log.Success {
			stats.SuccessCount++
		} else {
			stats.FailureCount++
		}

		actors[log.ActorID] = true
		stats.ActionCounts[string(log.Action)]++
		stats.ObjectTypeCounts[string(log.ObjectType)]++
	}

	stats.UniqueActors = len(actors)

	return stats, nil
}

// DeleteAuditLogs deletes audit logs older than the specified time.
func (s *MemoryAuditLogStore) DeleteAuditLogs(olderThan time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var remaining []*AuditLog
	var deleted int64

	for _, log := range s.logs {
		if log.Timestamp.Before(olderThan) {
			deleted++
		} else {
			remaining = append(remaining, log)
		}
	}

	s.logs = remaining
	return deleted, nil
}

// Ensure MemoryAuditLogStore implements AuditLogStore
var _ AuditLogStore = (*MemoryAuditLogStore)(nil)
