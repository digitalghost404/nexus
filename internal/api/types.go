package api

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type PaginatedResponse[T any] struct {
	Data   []T `json:"data"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
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
	Status          string `json:"status"`
	Version         string `json:"version"`
	Ollama          bool   `json:"ollama"`
	DBSizeBytes     int64  `json:"db_size_bytes"`
	EmbedQueueDepth int    `json:"embed_queue_depth"`
}

func ValidatePath(dir string) error {
	if !regexp.MustCompile(`^[a-zA-Z0-9_./-]+$`).MatchString(dir) {
		return fmt.Errorf("dir contains invalid characters")
	}
	cleaned := filepath.Clean(dir)
	if strings.Contains(cleaned, "..") {
		return fmt.Errorf("dir contains path traversal")
	}
	if filepath.IsAbs(cleaned) {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot determine home directory")
		}
		if !strings.HasPrefix(cleaned, home) {
			return fmt.Errorf("dir is outside home directory")
		}
	}
	return nil
}
