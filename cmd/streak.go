// cmd/streak.go
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/spf13/cobra"
)

var streakCmd = &cobra.Command{
	Use:   "streak",
	Short: "Show your coding streak",
	Long:  "Shows consecutive days with Claude sessions, plus weekly activity bars.",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		dates, err := database.GetDistinctSessionDates()
		if err != nil {
			return err
		}

		if len(dates) == 0 {
			fmt.Println("No sessions recorded yet.")
			return nil
		}

		today := time.Now().Format("2006-01-02")
		dateSet := map[string]bool{}
		for _, d := range dates {
			dateSet[d] = true
		}

		// Calculate current streak
		current := 0
		day := today
		for dateSet[day] {
			current++
			t, _ := time.Parse("2006-01-02", day)
			day = t.AddDate(0, 0, -1).Format("2006-01-02")
		}

		// Calculate longest streak
		longest := 0
		longestStart, longestEnd := "", ""
		streak := 0
		streakStart := ""
		for i := len(dates) - 1; i >= 0; i-- {
			if i == len(dates)-1 {
				streak = 1
				streakStart = dates[i]
			} else {
				prev, _ := time.Parse("2006-01-02", dates[i+1])
				curr, _ := time.Parse("2006-01-02", dates[i])
				// Compare by date arithmetic instead of duration to avoid DST issues
				if curr.Equal(prev.AddDate(0, 0, 1)) {
					streak++
				} else {
					if streak > longest {
						longest = streak
						longestStart = streakStart
						longestEnd = dates[i+1]
					}
					streak = 1
					streakStart = dates[i]
				}
			}
		}
		if streak > longest {
			longest = streak
			longestStart = streakStart
			longestEnd = dates[0]
		}

		// Format longest dates
		fmtDate := func(s string) string {
			t, err := time.Parse("2006-01-02", s)
			if err != nil {
				return s
			}
			return t.Format("Jan 02")
		}

		// Weekly bars
		now := time.Now()
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}

		thisWeek := make([]bool, 7)
		lastWeek := make([]bool, 7)

		for i := 0; i < 7; i++ {
			d := now.AddDate(0, 0, -(weekday-1-i))
			thisWeek[i] = dateSet[d.Format("2006-01-02")]
		}
		for i := 0; i < 7; i++ {
			d := now.AddDate(0, 0, -(weekday-1-i+7))
			lastWeek[i] = dateSet[d.Format("2006-01-02")]
		}

		display.FormatStreak(os.Stdout, current, longest, fmtDate(longestStart), fmtDate(longestEnd), thisWeek, lastWeek)
		return nil
	},
}

func init() {
	streakCmd.GroupID = "workflow"
	rootCmd.AddCommand(streakCmd)
}
