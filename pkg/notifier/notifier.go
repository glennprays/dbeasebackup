package notifier

import (
	"context"
	"time"
)

type EventStatus string

const (
	StatusSuccess EventStatus = "success"
	StatusFailure EventStatus = "failure"
)

type EventType string

const (
	EventBackup  EventType = "backup"
	EventCleanup EventType = "cleanup"
)

type Event struct {
	EventType   EventType   `json:"event_type"`
	Status      EventStatus `json:"status"`
	Filename    string      `json:"filename,omitempty"`
	FileSize    int64       `json:"file_size_bytes,omitempty"`
	Duration    string      `json:"duration"`
	Provider    string      `json:"provider"`
	StorageType string      `json:"storage_type"`
	Timestamp   time.Time   `json:"timestamp"`
	TraceID     string      `json:"trace_id"`
	Error       string      `json:"error,omitempty"`
}

type Notifier interface {
	Notify(ctx context.Context, event Event)
}
