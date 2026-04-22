package embed

import (
	"testing"
)

func TestContentHash(t *testing.T) {
	h1 := ContentHash("hello")
	h2 := ContentHash("hello")
	h3 := ContentHash("world")

	if h1 != h2 {
		t.Error("same content should produce same hash")
	}
	if h1 == h3 {
		t.Error("different content should produce different hash")
	}
	if len(h1) != 64 {
		t.Errorf("expected 64-char hex string, got %d", len(h1))
	}
}

func TestTruncateContent(t *testing.T) {
	short := "hello"
	result, truncated := TruncateContent(short, 10)
	if result != short {
		t.Errorf("expected %q, got %q", short, result)
	}
	if truncated {
		t.Error("short content should not be truncated")
	}

	long := "abcdefghijklmnopqrstuvwxyz"
	result, truncated = TruncateContent(long, 10)
	if result != "abcdefghij" {
		t.Errorf("expected truncated to 10 chars, got %q", result)
	}
	if !truncated {
		t.Error("long content should be marked as truncated")
	}
}

func TestWorkerStartStop(t *testing.T) {
	client := NewClient("http://localhost:99999", "nomic-embed-text", nil)
	w := NewWorker(client, nil)
	w.Start()
	w.Stop()
}
