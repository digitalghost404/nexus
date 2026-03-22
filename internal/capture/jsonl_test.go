// internal/capture/jsonl_test.go
package capture

import (
	"os"
	"strings"
	"testing"
)

// writeJSONL writes lines to a temp file and returns the path.
func writeJSONL(t *testing.T, lines []string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "session-*.jsonl")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer f.Close()
	for _, line := range lines {
		if _, err := f.WriteString(line + "\n"); err != nil {
			t.Fatalf("write temp file: %v", err)
		}
	}
	return f.Name()
}

// TestParseJSONL_UserMessage verifies that user text blocks are extracted verbatim.
func TestParseJSONL_UserMessage(t *testing.T) {
	path := writeJSONL(t, []string{
		`{"type":"user","message":{"content":[{"type":"text","text":"fix the bug"}]},"timestamp":"2026-03-22T12:00:00Z"}`,
	})

	digest, err := ParseJSONL(path)
	if err != nil {
		t.Fatalf("ParseJSONL error: %v", err)
	}
	if len(digest.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(digest.Messages))
	}
	m := digest.Messages[0]
	if m.Role != "user" {
		t.Errorf("role: got %q, want %q", m.Role, "user")
	}
	if m.Text != "fix the bug" {
		t.Errorf("text: got %q, want %q", m.Text, "fix the bug")
	}
	if m.Ts != "2026-03-22T12:00:00Z" {
		t.Errorf("ts: got %q, want %q", m.Ts, "2026-03-22T12:00:00Z")
	}
}

// TestParseJSONL_AssistantTruncation verifies that assistant text >200 chars is truncated.
func TestParseJSONL_AssistantTruncation(t *testing.T) {
	longText := strings.Repeat("a", 250)
	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"` + longText + `"}]},"timestamp":"2026-03-22T12:00:01Z"}`

	path := writeJSONL(t, []string{line})

	digest, err := ParseJSONL(path)
	if err != nil {
		t.Fatalf("ParseJSONL error: %v", err)
	}
	if len(digest.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(digest.Messages))
	}
	got := digest.Messages[0].Text
	if len(got) != 203 { // 200 chars + "..."
		t.Errorf("truncated length: got %d, want 203", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected truncated text to end with '...', got %q", got[len(got)-5:])
	}
}

// TestParseJSONL_AssistantNoTruncation verifies short assistant text is not modified.
func TestParseJSONL_AssistantNoTruncation(t *testing.T) {
	path := writeJSONL(t, []string{
		`{"type":"assistant","message":{"content":[{"type":"text","text":"short reply"}]},"timestamp":"2026-03-22T12:00:01Z"}`,
	})

	digest, err := ParseJSONL(path)
	if err != nil {
		t.Fatalf("ParseJSONL error: %v", err)
	}
	if len(digest.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(digest.Messages))
	}
	if digest.Messages[0].Text != "short reply" {
		t.Errorf("text: got %q, want %q", digest.Messages[0].Text, "short reply")
	}
}

// TestParseJSONL_ToolCallEdit verifies extraction of an Edit tool call with file_path.
func TestParseJSONL_ToolCallEdit(t *testing.T) {
	path := writeJSONL(t, []string{
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu1","name":"Edit","input":{"file_path":"cmd/hook.go","old_string":"foo","new_string":"bar"}}]},"timestamp":"2026-03-22T12:00:01Z"}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu1","content":"OK"}]},"timestamp":"2026-03-22T12:00:02Z"}`,
	})

	digest, err := ParseJSONL(path)
	if err != nil {
		t.Fatalf("ParseJSONL error: %v", err)
	}
	if len(digest.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(digest.ToolCalls))
	}
	tc := digest.ToolCalls[0]
	if tc.Tool != "Edit" {
		t.Errorf("tool: got %q, want %q", tc.Tool, "Edit")
	}
	if tc.Target != "cmd/hook.go" {
		t.Errorf("target: got %q, want %q", tc.Target, "cmd/hook.go")
	}
	if tc.Outcome != "OK" {
		t.Errorf("outcome: got %q, want %q", tc.Outcome, "OK")
	}
}

// TestParseJSONL_ToolCallBash verifies extraction of a Bash tool call with command.
func TestParseJSONL_ToolCallBash(t *testing.T) {
	path := writeJSONL(t, []string{
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu2","name":"Bash","input":{"command":"go test ./..."}}]},"timestamp":"2026-03-22T12:00:03Z"}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu2","content":"ok\tgithub.com/digitalghost404/nexus"}]},"timestamp":"2026-03-22T12:00:04Z"}`,
	})

	digest, err := ParseJSONL(path)
	if err != nil {
		t.Fatalf("ParseJSONL error: %v", err)
	}
	if len(digest.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(digest.ToolCalls))
	}
	tc := digest.ToolCalls[0]
	if tc.Tool != "Bash" {
		t.Errorf("tool: got %q, want %q", tc.Tool, "Bash")
	}
	if tc.Command != "go test ./..." {
		t.Errorf("command: got %q, want %q", tc.Command, "go test ./...")
	}
	if tc.Target != "go test ./..." {
		t.Errorf("target: got %q, want %q", tc.Target, "go test ./...")
	}
}

// TestParseJSONL_ErrorDetection verifies that tool_result errors are collected.
func TestParseJSONL_ErrorDetection(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"Error keyword", `"Error: something went wrong"`},
		{"error keyword", `"error: nil pointer dereference"`},
		{"FAIL keyword", `"FAIL\tgithub.com/foo/bar"`},
		{"non-zero exit", `"Exit code 1"`},
		{"exit code 127", `"Exit code 127"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeJSONL(t, []string{
				`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu1","name":"Bash","input":{"command":"run"}}]},"timestamp":"2026-03-22T12:00:00Z"}`,
				`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu1","content":` + tt.content + `}]},"timestamp":"2026-03-22T12:00:01Z"}`,
			})

			digest, err := ParseJSONL(path)
			if err != nil {
				t.Fatalf("ParseJSONL error: %v", err)
			}
			if len(digest.Errors) == 0 {
				t.Errorf("expected at least 1 error, got none")
			}
		})
	}
}

// TestParseJSONL_NoFalsePositiveExitZero verifies that "Exit code 0" is NOT an error.
func TestParseJSONL_NoFalsePositiveExitZero(t *testing.T) {
	path := writeJSONL(t, []string{
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu1","name":"Bash","input":{"command":"run"}}]},"timestamp":"2026-03-22T12:00:00Z"}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu1","content":"Exit code 0"}]},"timestamp":"2026-03-22T12:00:01Z"}`,
	})

	digest, err := ParseJSONL(path)
	if err != nil {
		t.Fatalf("ParseJSONL error: %v", err)
	}
	if len(digest.Errors) != 0 {
		t.Errorf("expected no errors for exit code 0, got %v", digest.Errors)
	}
}

// TestParseJSONL_FilesTouchedDedup verifies deduplication of file paths in FilesTouched.
func TestParseJSONL_FilesTouchedDedup(t *testing.T) {
	path := writeJSONL(t, []string{
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu1","name":"Read","input":{"file_path":"cmd/hook.go"}}]},"timestamp":"2026-03-22T12:00:00Z"}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu1","content":"content..."}]},"timestamp":"2026-03-22T12:00:01Z"}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu2","name":"Edit","input":{"file_path":"cmd/hook.go","old_string":"a","new_string":"b"}}]},"timestamp":"2026-03-22T12:00:02Z"}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu2","content":"OK"}]},"timestamp":"2026-03-22T12:00:03Z"}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu3","name":"Write","input":{"file_path":"cmd/other.go"}}]},"timestamp":"2026-03-22T12:00:04Z"}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu3","content":"OK"}]},"timestamp":"2026-03-22T12:00:05Z"}`,
	})

	digest, err := ParseJSONL(path)
	if err != nil {
		t.Fatalf("ParseJSONL error: %v", err)
	}
	if len(digest.FilesTouched) != 2 {
		t.Errorf("expected 2 unique files, got %d: %v", len(digest.FilesTouched), digest.FilesTouched)
	}
}

// TestParseJSONL_EmptyAndMalformedLinesSkipped verifies that blank/invalid lines are skipped.
func TestParseJSONL_EmptyAndMalformedLinesSkipped(t *testing.T) {
	path := writeJSONL(t, []string{
		``,
		`   `,
		`{not valid json`,
		`{"type":"user","message":{"content":[{"type":"text","text":"hello"}]},"timestamp":"2026-03-22T12:00:00Z"}`,
		`{}`,
		`{"type":"user","message":{"content":[{"type":"text","text":"world"}]},"timestamp":"2026-03-22T12:00:01Z"}`,
	})

	digest, err := ParseJSONL(path)
	if err != nil {
		t.Fatalf("ParseJSONL error: %v", err)
	}
	if len(digest.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(digest.Messages))
	}
}

// TestParseJSONL_SkippedTypes verifies that progress/system/etc lines produce no output.
func TestParseJSONL_SkippedTypes(t *testing.T) {
	path := writeJSONL(t, []string{
		`{"type":"progress","message":{},"timestamp":"2026-03-22T12:00:00Z"}`,
		`{"type":"system","message":{},"timestamp":"2026-03-22T12:00:00Z"}`,
		`{"type":"file-history-snapshot","message":{},"timestamp":"2026-03-22T12:00:00Z"}`,
		`{"type":"last-prompt","message":{},"timestamp":"2026-03-22T12:00:00Z"}`,
	})

	digest, err := ParseJSONL(path)
	if err != nil {
		t.Fatalf("ParseJSONL error: %v", err)
	}
	if len(digest.Messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(digest.Messages))
	}
	if len(digest.ToolCalls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(digest.ToolCalls))
	}
	if len(digest.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(digest.Errors))
	}
	if len(digest.FilesTouched) != 0 {
		t.Errorf("expected 0 files touched, got %d", len(digest.FilesTouched))
	}
}

// TestParseJSONL_ToolResultArrayContent verifies array-format tool_result content is handled.
func TestParseJSONL_ToolResultArrayContent(t *testing.T) {
	path := writeJSONL(t, []string{
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu1","name":"Read","input":{"file_path":"main.go"}}]},"timestamp":"2026-03-22T12:00:00Z"}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu1","content":[{"type":"text","text":"package main"},{"type":"text","text":"func main() {}"}]}]},"timestamp":"2026-03-22T12:00:01Z"}`,
	})

	digest, err := ParseJSONL(path)
	if err != nil {
		t.Fatalf("ParseJSONL error: %v", err)
	}
	if len(digest.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(digest.ToolCalls))
	}
	got := digest.ToolCalls[0].Outcome
	if !strings.Contains(got, "package main") {
		t.Errorf("outcome should contain 'package main', got %q", got)
	}
	if !strings.Contains(got, "func main()") {
		t.Errorf("outcome should contain 'func main()', got %q", got)
	}
}

// TestParseJSONL_FullConversation exercises the full sample from the task description.
func TestParseJSONL_FullConversation(t *testing.T) {
	path := writeJSONL(t, []string{
		`{"type":"user","message":{"content":[{"type":"text","text":"fix the bug"}]},"timestamp":"2026-03-22T12:00:00Z","uuid":"abc"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"I'll fix that now."},{"type":"tool_use","id":"tu1","name":"Edit","input":{"file_path":"cmd/hook.go","old_string":"foo","new_string":"bar"}}]},"timestamp":"2026-03-22T12:00:01Z","uuid":"def"}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu1","content":"OK"}]},"timestamp":"2026-03-22T12:00:02Z","uuid":"ghi"}`,
	})

	digest, err := ParseJSONL(path)
	if err != nil {
		t.Fatalf("ParseJSONL error: %v", err)
	}

	// 1 user text message + 1 assistant text message
	if len(digest.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(digest.Messages))
	}
	if digest.Messages[0].Role != "user" || digest.Messages[0].Text != "fix the bug" {
		t.Errorf("unexpected first message: %+v", digest.Messages[0])
	}
	if digest.Messages[1].Role != "assistant" || digest.Messages[1].Text != "I'll fix that now." {
		t.Errorf("unexpected second message: %+v", digest.Messages[1])
	}

	// 1 tool call
	if len(digest.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(digest.ToolCalls))
	}
	tc := digest.ToolCalls[0]
	if tc.Tool != "Edit" || tc.Target != "cmd/hook.go" || tc.Outcome != "OK" {
		t.Errorf("unexpected tool call: %+v", tc)
	}

	// 1 file touched
	if len(digest.FilesTouched) != 1 || digest.FilesTouched[0] != "cmd/hook.go" {
		t.Errorf("unexpected files touched: %v", digest.FilesTouched)
	}

	// no errors
	if len(digest.Errors) != 0 {
		t.Errorf("expected no errors, got %v", digest.Errors)
	}
}

// TestParseJSONL_FileNotFound verifies that a missing file returns an error.
func TestParseJSONL_FileNotFound(t *testing.T) {
	_, err := ParseJSONL("/nonexistent/path/session.jsonl")
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
}

// TestParseJSONL_GrepGlobTarget verifies target extraction for Grep and Glob tools.
func TestParseJSONL_GrepGlobTarget(t *testing.T) {
	path := writeJSONL(t, []string{
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu1","name":"Grep","input":{"pattern":"func ParseJSONL"}}]},"timestamp":"2026-03-22T12:00:00Z"}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu1","content":"internal/capture/jsonl.go"}]},"timestamp":"2026-03-22T12:00:01Z"}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu2","name":"Glob","input":{"pattern":"**/*.go"}}]},"timestamp":"2026-03-22T12:00:02Z"}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu2","content":"many files"}]},"timestamp":"2026-03-22T12:00:03Z"}`,
	})

	digest, err := ParseJSONL(path)
	if err != nil {
		t.Fatalf("ParseJSONL error: %v", err)
	}
	if len(digest.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(digest.ToolCalls))
	}
	if digest.ToolCalls[0].Target != "func ParseJSONL" {
		t.Errorf("grep target: got %q, want %q", digest.ToolCalls[0].Target, "func ParseJSONL")
	}
	if digest.ToolCalls[1].Target != "**/*.go" {
		t.Errorf("glob target: got %q, want %q", digest.ToolCalls[1].Target, "**/*.go")
	}
}
