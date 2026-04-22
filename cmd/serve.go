// cmd/serve.go
package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/digitalghost404/nexus/internal/db"
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
		defer database.Close()

		mux := http.NewServeMux()

		// GET /api/notes?project=<name>&limit=<n>
		mux.HandleFunc("/api/notes", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			if r.Method == http.MethodGet {
				projectName := r.URL.Query().Get("project")
				limitStr := r.URL.Query().Get("limit")
				limit := 20
				if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
					limit = n
				}

				var projectID int64
				if projectName != "" {
					p, _ := database.GetProjectByName(projectName)
					if p != nil {
						projectID = p.ID
					}
				}

				notes, err := database.ListNotes(projectID, limit)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				type noteJSON struct {
					ID        int64  `json:"id"`
					Text      string `json:"text"`
					Timestamp string `json:"timestamp"`
				}

				out := make([]noteJSON, 0, len(notes))
				for _, n := range notes {
					out = append(out, noteJSON{
						ID:        n.ID,
						Text:      n.Content,
						Timestamp: n.CreatedAt.Format(time.RFC3339),
					})
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{"notes": out})
				return
			}

			if r.Method == http.MethodPost {
				var body struct {
					Project string `json:"project"`
					Text    string `json:"text"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Text == "" {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}

				var projectID *int64
				if body.Project != "" {
					p, _ := database.GetProjectByName(body.Project)
					if p != nil {
						id := p.ID
						projectID = &id
					}
				}

				noteID, err := database.InsertNote(db.Note{
					ProjectID: projectID,
					Content:   body.Text,
				})
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				type noteJSON struct {
					ID        int64  `json:"id"`
					Text      string `json:"text"`
					Timestamp string `json:"timestamp"`
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"note": noteJSON{
						ID:        noteID,
						Text:      body.Text,
						Timestamp: time.Now().Format(time.RFC3339),
					},
				})
				return
			}

			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		})

		addr := fmt.Sprintf("127.0.0.1:%d", port)
		fmt.Printf("Nexus API listening on http://%s\n", addr)
		return http.ListenAndServe(addr, mux)
	},
}

func init() {
	serveCmd.Flags().Int("port", 7600, "Port to listen on")
	serveCmd.GroupID = "maintenance"
	rootCmd.AddCommand(serveCmd)
}
