package cmd

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/digitalghost404/nexus/internal/context"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/embed"
	"github.com/spf13/cobra"
)

var recallCmd = &cobra.Command{
	Use:   "recall <query>",
	Short: "Semantic search across sessions, notes, and preferences",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]
		limit, _ := cmd.Flags().GetInt("limit")
		types, _ := cmd.Flags().GetStringSlice("types")

		database, err := db.Open(cfg.DBPath())
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		client := embed.NewClient(cfg.OllamaURL, cfg.OllamaModel, &http.Client{Timeout: 5 * time.Second})

		vec, err := client.Embed(cmd.Context(), query)
		if err != nil {
			return recallFallback(database, query, limit, types)
		}

		type scoredResult struct {
			context.RecallResult
			score float64
		}
		var allResults []scoredResult

		for _, typ := range types {
			similar, err := database.SearchSimilar(vec, typ, limit, 0.7)
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

				allResults = append(allResults, scoredResult{
					RecallResult: context.RecallResult{
						SourceType: s.SourceType,
						SourceID:   s.SourceID,
						Content:    s.Content,
						Score:      s.Score,
						Date:       date,
					},
					score: s.Score,
				})
			}
		}

		sort.Slice(allResults, func(i, j int) bool {
			return allResults[i].score > allResults[j].score
		})

		if len(allResults) > limit {
			allResults = allResults[:limit]
		}

		results := make([]context.RecallResult, len(allResults))
		for i, r := range allResults {
			results[i] = r.RecallResult
		}

		if len(results) == 0 {
			fmt.Println("No results found.")
			return nil
		}

		fmt.Print(context.FormatRecall(results))
		return nil
	},
}

func recallFallback(database *db.DB, query string, limit int, types []string) error {
	fmt.Println("[semantic search unavailable, using keyword matching]")

	var results []context.RecallResult

	for _, typ := range types {
		switch typ {
		case "session":
			sessions, err := database.SearchSessions(query)
			if err != nil {
				continue
			}
			for _, s := range sessions {
				date := s.StartedAt
				results = append(results, context.RecallResult{
					SourceType: "session",
					SourceID:   s.ID,
					Content:    s.Summary,
					Date:       date,
				})
			}
		case "note":
			notes, err := database.SearchNotes(query)
			if err != nil {
				continue
			}
			for _, n := range notes {
				date := n.CreatedAt
				results = append(results, context.RecallResult{
					SourceType: "note",
					SourceID:   n.ID,
					Content:    n.Content,
					Date:       &date,
				})
			}
		case "preference":
			prefs, err := database.SearchPreferences(query)
			if err != nil {
				continue
			}
			for _, p := range prefs {
				date := p.CreatedAt
				results = append(results, context.RecallResult{
					SourceType: "preference",
					SourceID:   p.ID,
					Content:    p.Content,
					Date:       &date,
				})
			}
		}
	}

	if len(results) > limit {
		results = results[:limit]
	}

	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	fmt.Print(context.FormatRecall(results))
	return nil
}

func init() {
	recallCmd.GroupID = "query"
	recallCmd.Flags().Int("limit", 5, "Maximum results")
	recallCmd.Flags().StringSlice("types", []string{"session", "note", "preference"}, "Result types")
	rootCmd.AddCommand(recallCmd)
}
