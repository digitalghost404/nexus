// cmd/parseduration_test.go
package cmd

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	now := time.Now()

	tests := []struct {
		input   string
		wantErr bool
		// When valid, verify the returned time is approximately this far in the past.
		approxAgo time.Duration
	}{
		{input: "7d", wantErr: false, approxAgo: 7 * 24 * time.Hour},
		{input: "24h", wantErr: false, approxAgo: 24 * time.Hour},
		{input: "30m", wantErr: false, approxAgo: 30 * time.Minute},
		{input: "0d", wantErr: true},  // zero duration rejected
		{input: "", wantErr: true},
		{input: "d", wantErr: true},
		{input: "1", wantErr: true},
		{input: "7D", wantErr: true},
		{input: "-1d", wantErr: true}, // negative duration rejected
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseDuration(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseDuration(%q): expected error, got nil (result=%v)", tt.input, result)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseDuration(%q): unexpected error: %v", tt.input, err)
			}
			if result == nil {
				t.Fatalf("parseDuration(%q): expected non-nil result", tt.input)
			}

			// Allow a small margin for execution time between now and the call.
			const margin = 2 * time.Second
			expected := now.Add(-tt.approxAgo)
			diff := result.Sub(expected)
			if diff < 0 {
				diff = -diff
			}
			if diff > margin {
				t.Errorf("parseDuration(%q): result %v not within %v of expected %v (diff=%v)",
					tt.input, result, margin, expected, diff)
			}
		})
	}
}
