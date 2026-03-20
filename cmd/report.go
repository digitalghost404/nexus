// cmd/report.go
package cmd

import (
	"encoding/json"
	"os"
	"time"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/spf13/cobra"
)

var (
	reportWeek  bool
	reportMonth bool
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate activity summary for a time period",
	Long:  "Shows sessions, commits, files changed, most active projects, and language breakdown.",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		now := time.Now()
		var since time.Time
		if reportMonth {
			since = now.AddDate(0, -1, 0)
		} else {
			since = now.AddDate(0, 0, -7) // default: week
		}

		sessions, _ := database.ListSessions(db.SessionFilter{Since: &since, Limit: 1000})

		// Aggregate
		projectMap := map[string]*display.ProjectActivity{}
		langCount := map[string]int{}
		totalCommits := 0
		fileSet := map[string]bool{}
		totalSecs := 0

		for _, s := range sessions {
			// Project activity
			pa, ok := projectMap[s.ProjectName]
			if !ok {
				pa = &display.ProjectActivity{Name: s.ProjectName}
				projectMap[s.ProjectName] = pa
			}
			pa.Sessions++

			var commits []struct{ Hash, Message string }
			json.Unmarshal([]byte(s.CommitsMade), &commits)
			pa.Commits += len(commits)
			totalCommits += len(commits)

			var files []string
			json.Unmarshal([]byte(s.FilesChanged), &files)
			for _, f := range files {
				fileSet[f] = true
			}

			var tags []string
			json.Unmarshal([]byte(s.Tags), &tags)
			for _, t := range tags {
				if t != s.ProjectName { // skip project name tag
					langCount[t]++
				}
			}

			totalSecs += s.DurationSecs
		}

		// Sort projects by session count
		var topProjects []display.ProjectActivity
		for _, pa := range projectMap {
			topProjects = append(topProjects, *pa)
		}
		for i := 0; i < len(topProjects); i++ {
			for j := i + 1; j < len(topProjects); j++ {
				if topProjects[j].Sessions > topProjects[i].Sessions {
					topProjects[i], topProjects[j] = topProjects[j], topProjects[i]
				}
			}
		}
		if len(topProjects) > 5 {
			topProjects = topProjects[:5]
		}

		// Language percentages
		totalLang := 0
		for _, c := range langCount {
			totalLang += c
		}
		var languages []display.LangPercent
		for lang, count := range langCount {
			if totalLang > 0 {
				languages = append(languages, display.LangPercent{
					Lang:    lang,
					Percent: count * 100 / totalLang,
				})
			}
		}
		// Sort by percent desc
		for i := 0; i < len(languages); i++ {
			for j := i + 1; j < len(languages); j++ {
				if languages[j].Percent > languages[i].Percent {
					languages[i], languages[j] = languages[j], languages[i]
				}
			}
		}

		// Highlights: top 3 sessions by files changed
		type sessionFiles struct {
			summary string
			count   int
		}
		var sf []sessionFiles
		for _, s := range sessions {
			var files []string
			json.Unmarshal([]byte(s.FilesChanged), &files)
			sf = append(sf, sessionFiles{s.Summary, len(files)})
		}
		for i := 0; i < len(sf); i++ {
			for j := i + 1; j < len(sf); j++ {
				if sf[j].count > sf[i].count {
					sf[i], sf[j] = sf[j], sf[i]
				}
			}
		}
		var highlights []string
		limit := 3
		if len(sf) < limit {
			limit = len(sf)
		}
		for _, s := range sf[:limit] {
			if s.summary != "" {
				highlights = append(highlights, s.summary)
			}
		}

		r := display.ReportData{
			StartDate:     since.Format("Jan 02"),
			EndDate:       now.Format("Jan 02"),
			TotalSessions: len(sessions),
			TotalProjects: len(projectMap),
			TotalCommits:  totalCommits,
			TotalFiles:    len(fileSet),
			TotalHours:    float64(totalSecs) / 3600.0,
			TopProjects:   topProjects,
			Languages:     languages,
			Highlights:    highlights,
		}

		display.FormatReport(os.Stdout, r)
		return nil
	},
}

func init() {
	reportCmd.Flags().BoolVar(&reportWeek, "week", false, "Report for last week (default)")
	reportCmd.Flags().BoolVar(&reportMonth, "month", false, "Report for last month")
	reportCmd.GroupID = "workflow"
	rootCmd.AddCommand(reportCmd)
}
