package backup

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestLimitedBuffer_UnderCapacity(t *testing.T) {
	b := NewLimitedBuffer()
	data := []byte("hello world")
	if n, err := b.Write(data); n != len(data) || err != nil {
		t.Fatalf("Write() = %d, %v", n, err)
	}
	if got := b.String(); got != "hello world" {
		t.Errorf("String() = %q, want %q", got, "hello world")
	}
	if b.Len() != len(data) {
		t.Errorf("Len() = %d, want %d", b.Len(), len(data))
	}
}

func TestLimitedBuffer_OverflowKeepsTail(t *testing.T) {
	b := NewLimitedBuffer()
	payload := strings.Repeat("A", 100*1024) + "TAIL-MARKER"
	if _, err := io.Copy(b, strings.NewReader(payload)); err != nil {
		t.Fatalf("io.Copy() error = %v", err)
	}

	got := b.String()
	if !strings.HasPrefix(got, truncationMarker) {
		t.Errorf("String() missing truncation marker prefix: %q", got[:min(40, len(got))])
	}
	if b.Len() > LimitedBufferCapacity {
		t.Errorf("Len() = %d exceeds capacity %d", b.Len(), LimitedBufferCapacity)
	}
	if !strings.HasSuffix(got, "TAIL-MARKER") {
		t.Error("String() does not preserve the tail")
	}
	// Leading bytes beyond capacity were dropped: the marker plus retained
	// window can never reproduce the full payload.
	if len(got) > len(truncationMarker)+LimitedBufferCapacity {
		t.Errorf("String() length = %d exceeds marker+capacity", len(got))
	}
}

func TestLimitedBuffer_SlidingAcrossWrites(t *testing.T) {
	b := NewLimitedBuffer()
	// Write 200 KiB in 1 KiB chunks: window must slide, not grow.
	chunk := strings.Repeat("x", 1024)
	for range 200 {
		if _, err := b.Write([]byte(chunk)); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}
	if b.Len() > LimitedBufferCapacity {
		t.Errorf("Len() = %d exceeds capacity %d", b.Len(), LimitedBufferCapacity)
	}
	if !strings.HasPrefix(b.String(), truncationMarker) {
		t.Error("String() missing truncation marker after overflow")
	}
}

func TestLimitedBuffer_SingleGiantWrite(t *testing.T) {
	b := NewLimitedBuffer()
	payload := strings.Repeat("y", 200*1024) + "END"
	if _, err := b.Write([]byte(payload)); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	got := b.String()
	if !strings.HasSuffix(got, "END") {
		t.Error("single giant write did not preserve tail")
	}
	if b.Len() > LimitedBufferCapacity {
		t.Errorf("Len() = %d exceeds capacity", b.Len())
	}
}

func TestLimitedBuffer_ExactCapacity(t *testing.T) {
	b := NewLimitedBuffer()
	payload := bytes.Repeat([]byte("z"), LimitedBufferCapacity)
	if _, err := b.Write(payload); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if b.Len() != LimitedBufferCapacity {
		t.Fatalf("Len() = %d, want %d", b.Len(), LimitedBufferCapacity)
	}
	if strings.Contains(b.String(), truncationMarker) {
		t.Error("exact-capacity write must not be marked truncated")
	}

	// One more byte tips it over.
	if _, err := b.Write([]byte("!")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if b.Len() != LimitedBufferCapacity {
		t.Errorf("Len() after overflow = %d, want %d", b.Len(), LimitedBufferCapacity)
	}
	if !strings.HasPrefix(b.String(), truncationMarker) || !strings.HasSuffix(b.String(), "!") {
		t.Error("overflow state wrong after boundary write")
	}
}

func TestLimitedBuffer_EmptyWrite(t *testing.T) {
	b := NewLimitedBuffer()
	if _, err := b.Write(nil); err != nil {
		t.Fatalf("Write(nil) error = %v", err)
	}
	if b.String() != "" {
		t.Errorf("String() = %q, want empty", b.String())
	}
}
