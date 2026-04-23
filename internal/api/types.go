package api

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type PaginatedResponse[T any] struct {
	Data  []T `json:"data"`
	Total int `json:"total"`
	Limit int `json:"limit"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type CaptureRequest struct {
	Dir string `json:"dir"`
}

func (r *CaptureRequest) Validate() error {
	if r.Dir == "" {
		return fmt.Errorf("dir is required")
	}
	return nil
}

type InjectRequest struct {
	Dir string `json:"dir"`
}

func (r *InjectRequest) Validate() error {
	if r.Dir == "" {
		return fmt.Errorf("dir is required")
	}
	return nil
}

type HealthResponse struct {
	Status        string `json:"status"`
	Version       string `json:"version"`
	Ollama        string `json:"ollama"`
	DBSizeBytes   int64  `json:"db_size_bytes"`
	EmbedQueueDepth int  `json:"embed_queue_depth"`
}

var safePathRe = regexp.MustCompile(`^[a-zA-Z0-9._\-/]+$`)

func ValidatePath(dir string) error {
	if !safePathRe.MatchString(dir) {
		return fmt.Errorf("path contains invalid characters")
	}

	cleaned := dir
	if strings.Contains(dir, "..") {
		return fmt.Errorf("path traversal not allowed")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory")
	}

	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return fmt.Errorf("cannot resolve path: %w", err)
	}

	if !strings.HasPrefix(abs, home) {
		return fmt.Errorf("path must be within home directory")
	}

	return nil
}
