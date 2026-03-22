// internal/capture/jsonl.go
package capture

import (
	"bufio"
	"encoding/json"
	"os"
	"regexp"
	"strings"
)

// ConversationDigest holds a structured summary of a Claude Code JSONL session log.
type ConversationDigest struct {
	Messages     []DigestMessage  `json:"messages"`
	ToolCalls    []DigestToolCall `json:"tool_calls"`
	Errors       []string         `json:"errors"`
	FilesTouched []string         `json:"files_touched"`
}

// DigestMessage represents a single user or assistant message in the digest.
type DigestMessage struct {
	Role string `json:"role"`
	Text string `json:"text"`
	Ts   string `json:"ts"`
}

// DigestToolCall represents a single tool invocation in the digest.
type DigestToolCall struct {
	Tool     string `json:"tool"`
	Target   string `json:"target"`
	Command  string `json:"command,omitempty"`
	ExitCode *int   `json:"exit_code,omitempty"`
	Outcome  string `json:"outcome"`
}

// jsonlLine is the top-level structure of each line in the JSONL file.
type jsonlLine struct {
	Type      string          `json:"type"`
	Message   json.RawMessage `json:"message"`
	Timestamp string          `json:"timestamp"`
}

// messageWrapper wraps the content array inside a message field.
type messageWrapper struct {
	Content []contentBlock `json:"content"`
}

// contentBlock represents a single block within a message's content array.
type contentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
}

// nonZeroExitRe matches "Exit code" followed by a non-zero number.
var nonZeroExitRe = regexp.MustCompile(`Exit code [1-9][0-9]*`)

// errorKeywords are substrings that flag a tool_result as an error.
var errorKeywords = []string{"Error", "error", "FAIL"}

// ParseJSONL reads a Claude Code JSONL session log and returns a ConversationDigest.
func ParseJSONL(path string) (*ConversationDigest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	digest := &ConversationDigest{}

	// pendingTools maps tool_use ID → index in digest.ToolCalls so we can
	// back-fill the outcome when the matching tool_result arrives.
	pendingTools := map[string]int{}

	// seenFiles deduplicates FilesTouched.
	seenFiles := map[string]bool{}

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}

		var line jsonlLine
		if err := json.Unmarshal([]byte(raw), &line); err != nil {
			continue // skip malformed lines
		}

		switch line.Type {
		case "user":
			processUserLine(line, digest, pendingTools, seenFiles)
		case "assistant":
			processAssistantLine(line, digest, pendingTools, seenFiles)
		// "progress", "system", "file-history-snapshot", "last-prompt" and
		// anything else are intentionally skipped.
		}
	}

	return digest, nil
}

// processUserLine handles "user" typed JSONL lines.
func processUserLine(line jsonlLine, digest *ConversationDigest, pendingTools map[string]int, seenFiles map[string]bool) {
	var msg messageWrapper
	if err := json.Unmarshal(line.Message, &msg); err != nil {
		return
	}

	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			digest.Messages = append(digest.Messages, DigestMessage{
				Role: "user",
				Text: block.Text,
				Ts:   line.Timestamp,
			})

		case "tool_result":
			// Extract the result text — content may be a string or an array.
			resultText := extractToolResultText(block.Content)

			// Back-fill the outcome for the matching pending tool call.
			if idx, ok := pendingTools[block.ToolUseID]; ok {
				digest.ToolCalls[idx].Outcome = resultText
				delete(pendingTools, block.ToolUseID)
			}

			// Check for errors in the result.
			if isError(resultText) {
				digest.Errors = append(digest.Errors, truncateText(resultText, 200))
			}
		}
	}
}

// processAssistantLine handles "assistant" typed JSONL lines.
func processAssistantLine(line jsonlLine, digest *ConversationDigest, pendingTools map[string]int, seenFiles map[string]bool) {
	var msg messageWrapper
	if err := json.Unmarshal(line.Message, &msg); err != nil {
		return
	}

	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			digest.Messages = append(digest.Messages, DigestMessage{
				Role: "assistant",
				Text: truncateText(block.Text, 200),
				Ts:   line.Timestamp,
			})

		case "tool_use":
			tc := buildToolCall(block)
			idx := len(digest.ToolCalls)
			digest.ToolCalls = append(digest.ToolCalls, tc)
			if block.ID != "" {
				pendingTools[block.ID] = idx
			}

			// Track files touched (Read/Edit/Write targets that look like paths).
			if isFileOp(block.Name) && tc.Target != "" && tc.Target != block.Name {
				if !seenFiles[tc.Target] {
					seenFiles[tc.Target] = true
					digest.FilesTouched = append(digest.FilesTouched, tc.Target)
				}
			}
		}
	}
}

// buildToolCall constructs a DigestToolCall from a tool_use content block.
func buildToolCall(block contentBlock) DigestToolCall {
	tc := DigestToolCall{
		Tool:    block.Name,
		Target:  block.Name, // fallback
		Outcome: "pending",
	}

	if len(block.Input) == 0 {
		return tc
	}

	var input map[string]json.RawMessage
	if err := json.Unmarshal(block.Input, &input); err != nil {
		return tc
	}

	switch block.Name {
	case "Edit", "Read", "Write":
		if v, ok := input["file_path"]; ok {
			tc.Target = unquoteJSON(v)
		}
	case "Bash":
		if v, ok := input["command"]; ok {
			cmd := unquoteJSON(v)
			tc.Command = cmd
			tc.Target = cmd
		}
	case "Grep", "Glob":
		if v, ok := input["pattern"]; ok {
			tc.Target = unquoteJSON(v)
		}
	}

	return tc
}

// isFileOp returns true for tool names that operate on files.
func isFileOp(name string) bool {
	switch name {
	case "Edit", "Read", "Write":
		return true
	}
	return false
}

// extractToolResultText extracts the string content from a tool_result's
// content field, which may be a JSON string or a JSON array of objects.
func extractToolResultText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	// Try a plain JSON string first.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	// Try an array of objects with "type":"text" / "text" fields.
	var arr []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &arr); err == nil {
		var parts []string
		for _, item := range arr {
			if item.Type == "text" && item.Text != "" {
				parts = append(parts, item.Text)
			}
		}
		return strings.Join(parts, "\n")
	}

	return ""
}

// isError returns true if the text contains any error indicator.
func isError(text string) bool {
	for _, kw := range errorKeywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return nonZeroExitRe.MatchString(text)
}

// unquoteJSON unmarshals a JSON-encoded string value into a plain Go string.
func unquoteJSON(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return string(raw)
	}
	return s
}

// truncateText returns s if it is within max bytes, otherwise s[:max]+"...".
func truncateText(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
