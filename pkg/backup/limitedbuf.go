// Package backup — LimitedBuffer caps captured command stderr/stdout so a
// chatty subprocess cannot balloon process memory.
package backup

const (
	// LimitedBufferCapacity is the maximum retained bytes.
	LimitedBufferCapacity = 64 << 10 // 64 KiB
	truncationMarker      = "...[truncated]..."
)

// LimitedBuffer is a sliding-window bytes.Buffer replacement that keeps only
// the LAST LimitedBufferCapacity bytes written. Once anything is dropped,
// String() prefixes truncationMarker. Safe for use as exec.Cmd.Stderr.
type LimitedBuffer struct {
	buf  []byte // at most LimitedBufferCapacity
	full bool   // true once any bytes were dropped
}

// NewLimitedBuffer creates an empty LimitedBuffer.
func NewLimitedBuffer() *LimitedBuffer {
	return &LimitedBuffer{}
}

// Write implements io.Writer.
func (b *LimitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if n == 0 {
		return 0, nil
	}

	if len(b.buf)+n <= LimitedBufferCapacity {
		b.buf = append(b.buf, p...)
		return n, nil
	}

	b.full = true
	if n >= LimitedBufferCapacity {
		// The incoming write alone exceeds capacity: keep its tail.
		b.buf = append(b.buf[:0], p[n-LimitedBufferCapacity:]...)
		return n, nil
	}

	// Drop the oldest bytes to make room for p.
	drop := n - (LimitedBufferCapacity - len(b.buf))
	b.buf = append(b.buf[:0], b.buf[drop:]...)
	b.buf = append(b.buf, p...)
	return n, nil
}

// String returns the retained tail, prefixed with truncationMarker when any
// bytes were dropped.
func (b *LimitedBuffer) String() string {
	if b.full {
		return truncationMarker + string(b.buf)
	}
	return string(b.buf)
}

// Len returns the number of retained bytes.
func (b *LimitedBuffer) Len() int {
	return len(b.buf)
}
