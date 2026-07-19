package sandbox

import (
	"sync"

	"github.com/gedex/batasd/internal/submission"
)

const defaultMaxOutputBytes int64 = 1024 * 1024

// MaxOutputBytes returns the output byte cap for limits.
func MaxOutputBytes(limits submission.Limits) int64 {
	if limits.MaxOutputKB <= 0 {
		return defaultMaxOutputBytes
	}
	return limits.MaxOutputKB * 1024
}

// OutputBuffer stores output up to a cap and reports overflow.
type OutputBuffer struct {
	mu       sync.Mutex
	buf      []byte
	limit    int64
	exceeded bool
	onLimit  func()
}

// NewOutputBuffer creates a capped output buffer.
func NewOutputBuffer(limit int64, onLimit func()) *OutputBuffer {
	if limit <= 0 {
		limit = defaultMaxOutputBytes
	}
	return &OutputBuffer{limit: limit, onLimit: onLimit}
}

// Write stores p up to the configured cap.
func (b *OutputBuffer) Write(p []byte) (int, error) {
	var overflow bool

	b.mu.Lock()
	available := b.limit - int64(len(b.buf))
	if available > 0 {
		if int64(len(p)) <= available {
			b.buf = append(b.buf, p...)
		} else {
			b.buf = append(b.buf, p[:int(available)]...)
			overflow = !b.exceeded
			b.exceeded = true
		}
	} else if len(p) > 0 {
		overflow = !b.exceeded
		b.exceeded = true
	}
	onLimit := b.onLimit
	b.mu.Unlock()

	if overflow && onLimit != nil {
		onLimit()
	}

	return len(p), nil
}

// String returns the captured output.
func (b *OutputBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return string(b.buf)
}

// Exceeded reports whether output crossed the configured cap.
func (b *OutputBuffer) Exceeded() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.exceeded
}
