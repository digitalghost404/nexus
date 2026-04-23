// cmd/serve.go
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/digitalghost404/nexus/internal/capture"
	nctx "github.com/digitalghost404/nexus/internal/context"
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
		defer database.Close()

		// Start embed worker goroutine
		ollamaClient := embed.NewClient(cfg.OllamaURL, cfg.OllamaModel, nil)
		worker := embed.NewWorker(ollamaClient, database)
		worker.Start()
		defer worker.Stop()

		mux := http.NewServeMux()

		// GET /api/notes?project=<name>&limit=<n>
		// POST /api/notes
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
					jsonResponse(w, http.StatusInternalServerError, "failed to list notes")
					return
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
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"notes": out})
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
				jsonResponse(w, http.StatusInternalServerError, "failed to insert note")
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
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

		// POST /api/capture — for probe-before-write from capture command
		mux.HandleFunc("/api/capture", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}

			var body struct {
				Dir string `json:"dir"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Dir == "" {
				http.Error(w, `{"error":"dir is required"}`, http.StatusBadRequest)
				return
			}

			result, err := capture.CaptureSession(database, body.Dir, "")
			if err != nil {
				jsonResponse(w, http.StatusInternalServerError, "capture failed")
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"project": result.ProjectName,
				"summary": result.Summary,
				"commits": result.Commits,
				"files":   result.Files,
			})
		})

		// GET /api/preferences — list preferences
		// POST /api/preferences — create preference
		mux.HandleFunc("/api/preferences", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			if r.Method == http.MethodGet {
				projectStr := r.URL.Query().Get("project_id")
				var projectID *int64
				if projectStr != "" {
					id, err := strconv.ParseInt(projectStr, 10, 64)
					if err == nil {
						projectID = &id
					}
				}

				prefs, err := database.ListPreferencesByProject(projectID)
				if err != nil {
					jsonResponse(w, http.StatusInternalServerError, "failed to list preferences")
					return
				}

				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"preferences": prefs})
				return
			}

			if r.Method == http.MethodPost {
				var body struct {
					Category  string  `json:"category"`
					Content   string  `json:"content"`
					Source    string  `json:"source"`
					ProjectID *int64  `json:"project_id"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Content == "" {
					http.Error(w, `{"error":"content is required"}`, http.StatusBadRequest)
					return
				}

				if body.Category == "" {
					body.Category = "preference"
				}
				if body.Source == "" {
					body.Source = "stated"
				}

				confidence := 1.0
				switch body.Source {
				case "observed":
					confidence = 0.7
				case "inferred":
					confidence = 0.4
				}

				id, err := database.InsertPreference(db.Preference{
					ProjectID:  body.ProjectID,
					Category:   body.Category,
					Content:    body.Content,
					Source:     body.Source,
					Confidence: confidence,
				})
				if err != nil {
					jsonResponse(w, http.StatusInternalServerError, "failed to insert preference")
					return
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "content": body.Content})
				return
			}

			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		})

		// PATCH /api/preferences/{id} — update preference
		// DELETE /api/preferences/{id} — delete preference
		mux.HandleFunc("/api/preferences/{id}", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			idStr := r.PathValue("id")
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
				return
			}

			if r.Method == http.MethodPatch {
				var body struct {
					Category   string   `json:"category"`
					Content    string   `json:"content"`
					Source     string   `json:"source"`
					Confidence *float64 `json:"confidence"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					jsonResponse(w, http.StatusBadRequest, "bad request")
					return
				}

				existing, err := database.GetPreference(id)
				if err != nil {
					jsonResponse(w, http.StatusNotFound, "preference not found")
					return
				}

				category := existing.Category
				if body.Category != "" {
					category = body.Category
				}
				content := existing.Content
				if body.Content != "" {
					content = body.Content
				}
				source := existing.Source
				if body.Source != "" {
					source = body.Source
				}
				confidence := existing.Confidence
				if body.Confidence != nil {
					confidence = *body.Confidence
				}

				if err := database.UpdatePreference(id, category, content, source, confidence); err != nil {
					jsonResponse(w, http.StatusInternalServerError, "failed to update preference")
					return
				}

				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "updated": true})
				return
			}

			if r.Method == http.MethodDelete {
				if err := database.DeletePreference(id); err != nil {
					jsonResponse(w, http.StatusInternalServerError, "failed to delete preference")
					return
				}

				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "deleted": true})
				return
			}

			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		})

		// POST /api/recall — semantic search
		mux.HandleFunc("/api/recall", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}

			var body struct {
				Query string   `json:"query"`
				Limit int      `json:"limit"`
				Types []string `json:"types"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Query == "" {
				http.Error(w, `{"error":"query is required"}`, http.StatusBadRequest)
				return
			}

			if body.Limit <= 0 {
				body.Limit = 5
			}
			if len(body.Types) == 0 {
				body.Types = []string{"session", "note", "preference"}
			}

			var results []nctx.RecallResult

			// Try semantic search via ollama
			vec, vecErr := ollamaClient.Embed(r.Context(), body.Query)
			if vecErr != nil {
				// Fallback to FTS5 keyword search
				for _, typ := range body.Types {
					switch typ {
					case "session":
						sessions, err := database.SearchSessions(body.Query)
						if err != nil {
							fmt.Fprintf(os.Stderr, "search sessions: %v\n", err)
							continue
						}
						for _, s := range sessions {
							results = append(results, nctx.RecallResult{
								SourceType: "session",
								SourceID:   s.ID,
								Content:    s.Summary,
								Score:      0.5,
							})
						}
					case "note":
						notes, err := database.SearchNotes(body.Query)
						if err != nil {
							fmt.Fprintf(os.Stderr, "search notes: %v\n", err)
							continue
						}
						for _, n := range notes {
							results = append(results, nctx.RecallResult{
								SourceType: "note",
								SourceID:   n.ID,
								Content:    n.Content,
								Score:      0.5,
							})
						}
					case "preference":
						prefs, err := database.SearchPreferences(body.Query)
						if err != nil {
							fmt.Fprintf(os.Stderr, "search preferences: %v\n", err)
							continue
						}
						for _, p := range prefs {
							results = append(results, nctx.RecallResult{
								SourceType: "preference",
								SourceID:   p.ID,
								Content:    p.Content,
								Score:      float64(p.Confidence),
							})
						}
					}
				}
			} else {
				// Semantic search with vector similarity
				for _, typ := range body.Types {
					similar, err := database.SearchSimilar(vec, typ, body.Limit, 0.7)
					if err != nil {
						continue
					}
					for _, s := range similar {
						results = append(results, nctx.RecallResult{
							SourceType: s.SourceType,
							SourceID:   s.SourceID,
							Content:    s.Content,
							Score:      s.Score,
						})
					}
				}
			}

			// Limit results
			if len(results) > body.Limit {
				results = results[:body.Limit]
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"results": results})
		})

		// POST /api/inject — build smart context
		mux.HandleFunc("/api/inject", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}

			var body struct {
				Project       string `json:"project"`
				TaskDesc      string `json:"task_description"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Project == "" {
				http.Error(w, `{"error":"project is required"}`, http.StatusBadRequest)
				return
			}

			project, err := database.GetProjectByName(body.Project)
			if err != nil || project == nil {
				http.Error(w, fmt.Sprintf(`{"error":"project %q not found"}`, body.Project), http.StatusNotFound)
				return
			}

			// Pass 1: Recent sessions
			sessions, _ := database.ListSessions(db.SessionFilter{ProjectID: project.ID, Limit: 5})

			// Pass 2: Semantic recall (if ollama available)
			var recallResults []nctx.RecallResult
			vec, err := ollamaClient.Embed(r.Context(), body.TaskDesc)
			ollamaAvailable := err == nil
			if ollamaAvailable {
				for _, typ := range []string{"session", "note", "preference"} {
					similar, sErr := database.SearchSimilar(vec, typ, 3, 0.7)
					if sErr != nil {
						fmt.Fprintf(os.Stderr, "search similar %s: %v\n", typ, sErr)
						continue
					}
					for _, s := range similar {
						recallResults = append(recallResults, nctx.RecallResult{
							SourceType: s.SourceType,
							SourceID:   s.SourceID,
							Content:    s.Content,
							Score:      s.Score,
						})
					}
				}
			}

			// Pass 3: Preferences
			prefs, _ := database.ListPreferencesByProject(&project.ID)

			ctxOutput := nctx.BuildContext(nctx.ContextOptions{
				Project:         project,
				RecentSessions:  sessions,
				RecallResults:   recallResults,
				Preferences:     prefs,
				TaskDescription: body.TaskDesc,
				OllamaAvailable: ollamaAvailable,
			})

			// Cold start check
			if ctxOutput == "" {
				ctxOutput = "Nexus memory is empty. Context will build as sessions accumulate. Use `remember` to save preferences manually."
			}

			w.Header().Set("Content-Type", "text/plain")
			_, _ = fmt.Fprint(w, ctxOutput)
		})

		// GET /api/embed/status — embedding queue status
		mux.HandleFunc("/api/embed/status", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}

			// Count unembedded items
			var queueDepth int
			err := database.Conn().QueryRow(`
				SELECT COUNT(*) FROM (
					SELECT s.id FROM sessions s
					LEFT JOIN embedding_meta em ON em.source_type = 'session' AND em.source_id = s.id
					WHERE em.id IS NULL AND s.summary IS NOT NULL AND s.summary != ''
					UNION ALL
					SELECT n.id FROM notes n
					LEFT JOIN embedding_meta em ON em.source_type = 'note' AND em.source_id = n.id
					WHERE em.id IS NULL AND n.content IS NOT NULL AND n.content != ''
					UNION ALL
					SELECT p.id FROM preferences p
					LEFT JOIN embedding_meta em ON em.source_type = 'preference' AND em.source_id = p.id
					WHERE em.id IS NULL AND p.content IS NOT NULL AND p.content != '' AND p.superseded_by IS NULL
				)
			`).Scan(&queueDepth)
			if err != nil {
				jsonResponse(w, http.StatusInternalServerError, "failed to get embed status")
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"queue_depth":   queueDepth,
				"model":         cfg.OllamaModel,
				"ollama_url":    cfg.OllamaURL,
				"poll_interval": embedPollIntervalSecs,
			})
		})

		addr := fmt.Sprintf("127.0.0.1:%d", port)
		fmt.Printf("Nexus API listening on http://%s\n", addr)

		// Graceful shutdown
		srv := &http.Server{Addr: addr, Handler: mux}
		errCh := make(chan error, 1)
		go func() {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				errCh <- err
			}
		}()

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		select {
		case <-quit:
			fmt.Println("\nShutting down Nexus API...")
		case err := <-errCh:
			fmt.Fprintf(os.Stderr, "serve: %v\n", err)
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	},
}

const embedPollIntervalSecs = 30

type noteJSON struct {
	ID        int64  `json:"id"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
}

// jsonResponse writes a JSON error response safely
func jsonResponse(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": msg})
}

func init() {
	serveCmd.Flags().Int("port", 7600, "Port to listen on")
	serveCmd.GroupID = "maintenance"
	rootCmd.AddCommand(serveCmd)
}
