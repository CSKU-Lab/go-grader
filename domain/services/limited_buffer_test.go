package services

import (
	"strings"
	"testing"
)

func TestLimitedBuffer_CapsAtMax(t *testing.T) {
	b := &limitedBuffer{max: 10}

	n, err := b.Write([]byte("abcdefghijklmnop")) // 16 bytes into a 10-byte cap
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Write must report the full length so it never blocks the writer.
	if n != 16 {
		t.Fatalf("Write returned %d, want 16 (full length)", n)
	}
	if got := b.String(); got != "abcdefghij" {
		t.Fatalf("buffer = %q, want %q", got, "abcdefghij")
	}
	if !b.truncated {
		t.Fatal("expected truncated = true")
	}
}

func TestLimitedBuffer_UnderCap(t *testing.T) {
	b := &limitedBuffer{max: 1024}
	b.Write([]byte("hello "))
	b.Write([]byte("world"))
	if got := b.String(); got != "hello world" {
		t.Fatalf("buffer = %q, want %q", got, "hello world")
	}
	if b.truncated {
		t.Fatal("expected truncated = false")
	}
}

func TestLimitedBuffer_UnboundedWritesStayBounded(t *testing.T) {
	// Simulates an infinite-output program: many writes must never grow the
	// buffer past max (the OOM guard).
	b := &limitedBuffer{max: 16 << 20}
	chunk := []byte(strings.Repeat("x", 64<<10))
	for range 1000 { // 64 MiB of writes
		b.Write(chunk)
	}
	if len(b.buf) > b.max {
		t.Fatalf("buffer grew to %d, exceeds max %d", len(b.buf), b.max)
	}
	if !b.truncated {
		t.Fatal("expected truncated = true after exceeding cap")
	}
}
