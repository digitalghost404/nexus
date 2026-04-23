package cmd

import (
	"os"
	"strconv"

	"github.com/digitalghost404/nexus/internal/api"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/embed"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Nexus HTTP API server on localhost",
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")

		database, err := db.Open(cfg.DBPath())
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		ollamaClient := embed.NewClient(cfg.OllamaURL, cfg.OllamaModel, nil)
		worker := embed.NewWorker(ollamaClient, database)

		apiToken := os.Getenv("NEXUS_API_TOKEN")
		corsOrigins := []string{}
		if origin := os.Getenv("NEXUS_CORS_ORIGIN"); origin != "" {
			corsOrigins = []string{origin}
		}
		rateLimit := 60
		if rl := os.Getenv("NEXUS_RATE_LIMIT"); rl != "" {
			if n, err := strconv.Atoi(rl); err == nil && n > 0 {
				rateLimit = n
			}
		}

		srv := api.NewServer(database, worker, apiToken, corsOrigins, rateLimit, cfg.OllamaURL, cfg.OllamaModel, Version)
		addr := "127.0.0.1:" + strconv.Itoa(port)
		return srv.ListenAndServe(addr)
	},
}

func init() {
	serveCmd.Flags().Int("port", 7600, "Port to listen on")
	serveCmd.GroupID = "maintenance"
	rootCmd.AddCommand(serveCmd)
}
