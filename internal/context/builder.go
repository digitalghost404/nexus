package context

import (
	"strings"
	"time"

	"github.com/digitalghost404/nexus/internal/db"
)

type RecallResult struct {
	SourceType string
	SourceID   int64
	Content    string
	Score      float64
	Date       *time.Time
}

type ContextOptions struct {
	Project         *db.Project
	RecentSessions  []db.Session
	RecallResults   []RecallResult
	Preferences     []db.Preference
	TaskDescription string
	OllamaAvailable bool
}

func BuildContext(opts ContextOptions) string {
	var sections []string

	// Pass 1: Project state
	sections = append(sections, FormatProjectState(opts.Project, opts.RecentSessions))

	// Pass 2: Semantic recall
	if len(opts.RecallResults) > 0 {
		sections = append(sections, FormatRecall(opts.RecallResults))
	} else if !opts.OllamaAvailable {
		sections = append(sections, "[semantic recall skipped — ollama not running]")
	}

	// Pass 3: Preferences
	prefsSection := FormatPreferences(opts.Preferences)
	if prefsSection != "" {
		sections = append(sections, prefsSection)
	}

	return strings.Join(sections, "\n\n")
}

