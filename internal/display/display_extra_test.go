// internal/display/display_extra_test.go
package display

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/digitalghost404/nexus/internal/db"
)

func TestRelativeTimeFuture(t *testing.T) {
	future := time.Now().Add(5 * time.Minute)
	result := RelativeTime(future)
	if result != "just now" {
		t.Errorf("RelativeTime(future): expected 'just now', got %q", result)
	}
}

func TestTruncateMultiByte(t *testing.T) {
	// "日本語" is 3 rune characters but 9 bytes each (3 bytes per kanji).
	// A byte-based truncation would split in the middle of a rune; a rune-based
	// implementation must not do that.
	s := "abc日本語xyz"
	// max=5 → first 2 runes + "..." → "ab..."
	result := truncate(s, 5)
	if len(result) == 0 {
		t.Fatal("truncate returned empty string")
	}
	// Verify the result is valid UTF-8 (no broken multibyte sequences).
	for i, r := range result {
		if r == '\uFFFD' {
			t.Errorf("truncate produced replacement character at index %d: %q", i, result)
		}
	}
	if !strings.HasSuffix(result, "...") {
		t.Errorf("truncate(%q, 5): expected ellipsis suffix, got %q", s, result)
	}
	// Result must be at most max runes long.
	if len([]rune(result)) > 5 {
		t.Errorf("truncate(%q, 5): result %q exceeds max rune length", s, result)
	}
}

func TestTruncateEmpty(t *testing.T) {
	result := truncate("", 10)
	if result != "" {
		t.Errorf("truncate(\"\", 10): expected \"\", got %q", result)
	}
}

func TestTruncateExactLength(t *testing.T) {
	s := "hello"
	result := truncate(s, 5)
	if result != s {
		t.Errorf("truncate(%q, 5): expected string returned as-is, got %q", s, result)
	}
}

func TestFormatDiffSinceString(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	sessions := []db.Session{
		{ProjectName: "wraith", Summary: "Added retry logic", StartedAt: &now},
	}

	FormatDiff(&buf, "wraith", "24h", sessions)
	output := buf.String()

	if !strings.Contains(output, "24h") {
		t.Errorf("FormatDiff: expected header to contain the raw since string '24h', got:\n%s", output)
	}
	if !strings.Contains(output, "wraith") {
		t.Errorf("FormatDiff: expected project name 'wraith' in output, got:\n%s", output)
	}
}
