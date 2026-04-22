package cmd

import (
	"fmt"
	"net/http"
	"time"

	"github.com/digitalghost404/nexus/internal/context"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/embed"
	"github.com/spf13/cobra"
)

var injectCmd = &cobra.Command{
	Use:   "inject <project>",
	Short: "Build smart context for session start",
	Long:  "Assembles project state, semantic recall, and preferences into a single markdown output for AI agents.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]
		taskDesc, _ := cmd.Flags().GetString("task")

		database, err := db.Open(cfg.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		p, err := database.GetProjectByName(projectName)
		if err != nil {
			return err
		}
		if p == nil {
			return fmt.Errorf("project %q not found", projectName)
		}

		since := time.Now().AddDate(0, 0, -7)
		sessions, err := database.GetSessionsInRange(p.ID, since, time.Now())
		if err != nil {
			return fmt.Errorf("get sessions: %w", err)
		}

		ollamaAvailable := true
		var recallResults []context.RecallResult

		client := embed.NewClient(cfg.OllamaURL, cfg.OllamaModel, &http.Client{Timeout: 5 * time.Second})

		if taskDesc != "" {
			vec, err := client.Embed(cmd.Context(), taskDesc)
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
		}

		var projectID *int64
		pid := p.ID
		projectID = &pid
		prefs, err := database.ListPreferencesByProject(projectID)
		if err != nil {
			return fmt.Errorf("list preferences: %w", err)
		}

		opts := context.ContextOptions{
			Project:         p,
			RecentSessions:  sessions,
			RecallResults:   recallResults,
			Preferences:     prefs,
			TaskDescription: taskDesc,
			OllamaAvailable: ollamaAvailable,
		}

		output := context.BuildContext(opts)
		if output == "" {
			fmt.Println("Nexus memory is empty. Context will build as sessions accumulate. Use `remember` to save preferences manually.")
			return nil
		}

		fmt.Println(output)
		return nil
	},
}

func init() {
	injectCmd.GroupID = "workflow"
	injectCmd.Flags().String("task", "", "What you seem to be working on (triggers semantic recall)")
	rootCmd.AddCommand(injectCmd)
}
