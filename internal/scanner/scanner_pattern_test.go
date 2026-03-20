// internal/scanner/scanner_pattern_test.go
package scanner

import "testing"

func TestMatchPathPattern(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{
			pattern: "*/node_modules/*",
			path:    "/home/user/projects/myapp/node_modules/foo",
			want:    true,
		},
		{
			pattern: "*/node_modules/*",
			path:    "/home/user/projects/myapp/src/main.go",
			want:    false,
		},
		{
			pattern: "*/go/pkg/*",
			path:    "/home/user/go/pkg/mod",
			want:    true,
		},
		{
			// "go-tools" must not match the concrete part "go" — path components
			// must match exactly, not as a prefix.
			pattern: "*/go/pkg/*",
			path:    "/home/user/go-tools/pkg/stuff",
			want:    false,
		},
		{
			pattern: "*/vendor/*",
			path:    "/home/user/projects/myapp/vendor/lib",
			want:    true,
		},
		{
			// An all-wildcard pattern has no concrete parts and must return false.
			pattern: "*",
			path:    "/anything",
			want:    false,
		},
		{
			// An empty pattern has no concrete parts and must return false.
			pattern: "",
			path:    "/anything",
			want:    false,
		},
		{
			// "/node_modules" contains "node_modules" as an exact path component,
			// so the function returns true. The trailing wildcard in the pattern
			// is not enforced structurally — only the concrete parts are checked.
			pattern: "*/node_modules/*",
			path:    "/node_modules",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"|"+tt.path, func(t *testing.T) {
			got := matchPathPattern(tt.path, tt.pattern)
			if got != tt.want {
				t.Errorf("matchPathPattern(%q, %q) = %v, want %v", tt.path, tt.pattern, got, tt.want)
			}
		})
	}
}
