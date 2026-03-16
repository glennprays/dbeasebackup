package traceid

import (
	"context"

	"github.com/google/uuid"
)

// ctxKey is the context key for trace_id
type ctxKey struct{}

// FromContext extracts trace_id from context.
// Returns an empty string if no trace_id is found.
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if traceID, ok := ctx.Value(ctxKey{}).(string); ok {
		return traceID
	}
	return ""
}

// NewContext creates a new context with the given trace_id.
func NewContext(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, ctxKey{}, traceID)
}

// Generate creates a new UUID-based trace_id.
func Generate() string {
	return uuid.New().String()
}
