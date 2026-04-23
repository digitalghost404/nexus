// cmd/context_cmd.go
package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/digitalghost404/nexus/internal/capture"
	"github.com/digitalghost404/nexus/internal/context"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/digitalghost404/nexus/internal/embed"
	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context <project>",
	Short: "Export project context for pasting into Claude",
	Long:  "Outputs everything Nexus knows about a project in markdown format, optimized for sharing with Claude.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		injectMode, _ := cmd.Flags().GetBool("inject")

		database, err := db.Open(cfg.DBPath())
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		p, err := database.GetProjectByName(args[0])
		if err != nil {
			return err
		}
		if p == nil {
			return fmt.Errorf("project not found: %s", args[0])
		}

		if injectMode {
			return runInjectContext(database, p, cmd)
		}

		since := time.Now().AddDate(0, 0, -7)
		sessions, _ := database.GetSessionsInRange(p.ID, since, time.Now())
		notes, _, _ := database.ListNotes(p.ID, 10, 0)

		var linkedProjects []db.Project
		linkedProjects, _ = database.GetLinkedProjects(p.ID)

		digests := make(map[int64]string)
		for _, s := range sessions {
			if d, err := database.GetConversationDigest(s.ID); err == nil && d != "" {
				digests[s.ID] = d
			} else if s.ClaudeSessionID != "" {
				claudeDir := capture.DefaultClaudeDir()
				jsonlPath := capture.FindSessionJSONL(claudeDir, s.ClaudeSessionID, p.Path)
				if jsonlPath != "" {
					if parsed, err := capture.ParseJSONL(jsonlPath); err == nil && parsed != nil {
						if digestJSON, err := json.Marshal(parsed); err == nil {
							digestStr := string(digestJSON)
							_ = database.InsertConversationDigest(s.ID, digestStr)
							digests[s.ID] = digestStr
						}
					}
				}
			}
		}

		display.FormatContext(os.Stdout, p, sessions, notes, linkedProjects, digests)
		return nil
	},
}

func runInjectContext(database *db.DB, p *db.Project, cmd *cobra.Command) error {
	since := time.Now().AddDate(0, 0, -7)
	sessions, err := database.GetSessionsInRange(p.ID, since, time.Now())
	if err != nil {
		return fmt.Errorf("get sessions: %w", err)
	}

	ollamaAvailable := true
	var recallResults []context.RecallResult

	client := embed.NewClient(cfg.OllamaURL, cfg.OllamaModel, &http.Client{Timeout: 5 * time.Second})
	vec, err := client.Embed(cmd.Context(), p.Name+" "+p.LastCommitMsg)
	if err != nil {
		ollamaAvailable = false
	} else {
		for _, typ := range []string{"session", "note", "preference"} {
			similar, err := database.SearchSimilar(vec, typ, 5, 0.7)
			if err != nil {
				continue
			}
			for _, s := range similar {
				var date *time.Time
				var dateQuery string
				switch typ {
				case "session":
					dateQuery = "SELECT started_at FROM sessions WHERE id = ?"
				case "note":
					dateQuery = "SELECT created_at FROM notes WHERE id = ?"
				case "preference":
					dateQuery = "SELECT created_at FROM preferences WHERE id = ?"
				}
				if dateQuery != "" {
					var t time.Time
					if err := database.Conn().QueryRow(dateQuery, s.SourceID).Scan(&t); err == nil {
						date = &t
					}
				}
				recallResults = append(recallResults, context.RecallResult{
					SourceType: s.SourceType,
					SourceID:   s.SourceID,
					Content:    s.Content,
					Score:      s.Score,
					Date:       date,
				})
			}
		}
	}

	var projectID *int64
	pid := p.ID
	projectID = &pid
	prefs, _, err := database.ListPreferencesByProject(projectID, 0, 0)
	if err != nil {
		return fmt.Errorf("list preferences: %w", err)
	}

	opts := context.ContextOptions{
		Project:        p,
		RecentSessions: sessions,
		RecallResults:  recallResults,
		Preferences:    prefs,
		OllamaAvailable: ollamaAvailable,
	}

	output := context.BuildContext(opts)
	if output == "" {
		fmt.Println("Nexus memory is empty. Context will build as sessions accumulate. Use `remember` to save preferences manually.")
		return nil
	}

	fmt.Println(output)
	return nil
}

func init() {
	contextCmd.GroupID = "workflow"
	contextCmd.Flags().Bool("inject", false, "Use smart 3-pass context builder with semantic recall and preferences")
	rootCmd.AddCommand(contextCmd)
}
