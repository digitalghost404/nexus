package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/digitalghost404/nexus/internal/capture"
	nctx "github.com/digitalghost404/nexus/internal/context"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/embed"
)

type Handler struct {
	db          *db.DB
	embedWorker *embed.Worker
	ollamaURL   string
	ollamaModel string
	version     string
}

func NewHandler(database *db.DB, embedWorker *embed.Worker, ollamaURL, ollamaModel, version string) *Handler {
	return &Handler{
		db:          database,
		embedWorker: embedWorker,
		ollamaURL:   ollamaURL,
		ollamaModel: ollamaModel,
		version:     version,
	}
}

func (h *Handler) writeJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (h *Handler) Capture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body CaptureRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Dir == "" {
		jsonError(w, "dir is required", http.StatusBadRequest)
		return
	}

	if err := ValidatePath(body.Dir); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := capture.CaptureSession(h.db, body.Dir, "")
	if err != nil {
		jsonError(w, "capture failed", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, map[string]interface{}{
		"project": result.ProjectName,
		"summary": result.Summary,
		"commits": result.Commits,
		"files":   result.Files,
	}, http.StatusOK)
}

func (h *Handler) ListNotes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectName := r.URL.Query().Get("project")
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
		limit = n
	}
	if limit > 100 {
		limit = 100
	}

	var projectID int64
	if projectName != "" {
		p, err := h.db.GetProjectByName(projectName)
		if err != nil {
			jsonError(w, "failed to lookup project", http.StatusInternalServerError)
			return
		}
		if p != nil {
			projectID = p.ID
		}
	}

	notes, _, err := h.db.ListNotes(projectID, limit, 0)
	if err != nil {
		jsonError(w, "failed to list notes", http.StatusInternalServerError)
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

	h.writeJSON(w, map[string]interface{}{"notes": out}, http.StatusOK)
}

func (h *Handler) CreateNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Project string `json:"project"`
		Text    string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Text == "" {
		jsonError(w, "text is required", http.StatusBadRequest)
		return
	}

	var projectID *int64
	if body.Project != "" {
		p, err := h.db.GetProjectByName(body.Project)
		if err != nil {
			jsonError(w, "failed to lookup project", http.StatusInternalServerError)
			return
		}
		if p != nil {
			id := p.ID
			projectID = &id
		}
	}

	noteID, err := h.db.InsertNote(db.Note{
		ProjectID: projectID,
		Content:   body.Text,
	})
	if err != nil {
		jsonError(w, "failed to insert note", http.StatusInternalServerError)
		return
	}

	type noteJSON struct {
		ID        int64  `json:"id"`
		Text      string `json:"text"`
		Timestamp string `json:"timestamp"`
	}

	h.writeJSON(w, map[string]interface{}{
		"note": noteJSON{
			ID:        noteID,
			Text:      body.Text,
			Timestamp: time.Now().Format(time.RFC3339),
		},
	}, http.StatusCreated)
}

func (h *Handler) ListPreferences(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectStr := r.URL.Query().Get("project_id")
	var projectID *int64
	if projectStr != "" {
		id, err := strconv.ParseInt(projectStr, 10, 64)
		if err == nil {
			projectID = &id
		}
	}

	prefs, _, err := h.db.ListPreferencesByProject(projectID, 0, 0)
	if err != nil {
		jsonError(w, "failed to list preferences", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, map[string]interface{}{"preferences": prefs}, http.StatusOK)
}

func (h *Handler) CreatePreference(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Category  string  `json:"category"`
		Content   string  `json:"content"`
		Source    string  `json:"source"`
		ProjectID *int64  `json:"project_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Content == "" {
		jsonError(w, "content is required", http.StatusBadRequest)
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

	id, err := h.db.InsertPreference(db.Preference{
		ProjectID:  body.ProjectID,
		Category:   body.Category,
		Content:    body.Content,
		Source:     body.Source,
		Confidence: confidence,
	})
	if err != nil {
		jsonError(w, "failed to insert preference", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, map[string]interface{}{"id": id, "content": body.Content}, http.StatusCreated)
}

func (h *Handler) UpdatePreference(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	var body struct {
		Category   string   `json:"category"`
		Content    string   `json:"content"`
		Source     string   `json:"source"`
		Confidence *float64 `json:"confidence"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "bad request", http.StatusBadRequest)
		return
	}

	existing, err := h.db.GetPreference(id)
	if err != nil {
		jsonError(w, "preference not found", http.StatusNotFound)
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

	if err := h.db.UpdatePreference(id, category, content, source, confidence); err != nil {
		jsonError(w, "failed to update preference", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, map[string]interface{}{"id": id, "updated": true}, http.StatusOK)
}

func (h *Handler) DeletePreference(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := h.db.DeletePreference(id); err != nil {
		jsonError(w, "failed to delete preference", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, map[string]interface{}{"id": id, "deleted": true}, http.StatusOK)
}

func (h *Handler) Recall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Query string   `json:"query"`
		Limit int      `json:"limit"`
		Types []string `json:"types"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Query == "" {
		jsonError(w, "query is required", http.StatusBadRequest)
		return
	}

	if body.Limit <= 0 {
		body.Limit = 5
	}
	if len(body.Types) == 0 {
		body.Types = []string{"session", "note", "preference"}
	}

	ollamaClient := embed.NewClient(h.ollamaURL, h.ollamaModel, nil)

	var results []nctx.RecallResult

	vec, vecErr := ollamaClient.Embed(r.Context(), body.Query)
	if vecErr != nil {
		for _, typ := range body.Types {
			switch typ {
			case "session":
				sessions, err := h.db.SearchSessions(body.Query)
				if err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "search sessions: %v\n", err)
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
				notes, err := h.db.SearchNotes(body.Query)
				if err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "search notes: %v\n", err)
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
				prefs, err := h.db.SearchPreferences(body.Query)
				if err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "search preferences: %v\n", err)
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
		for _, typ := range body.Types {
			similar, err := h.db.SearchSimilar(vec, typ, body.Limit, 0.7)
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

	if len(results) > body.Limit {
		results = results[:body.Limit]
	}

	h.writeJSON(w, map[string]interface{}{"results": results}, http.StatusOK)
}

func (h *Handler) Inject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Project  string `json:"project"`
		TaskDesc string `json:"task_description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Project == "" {
		jsonError(w, "project is required", http.StatusBadRequest)
		return
	}

	project, err := h.db.GetProjectByName(body.Project)
	if err != nil || project == nil {
		jsonError(w, fmt.Sprintf("project %q not found", body.Project), http.StatusNotFound)
		return
	}

	sessions, _, _ := h.db.ListSessions(db.SessionFilter{ProjectID: project.ID, Limit: 5})

	ollamaClient := embed.NewClient(h.ollamaURL, h.ollamaModel, nil)

	var recallResults []nctx.RecallResult
	vec, err := ollamaClient.Embed(r.Context(), body.TaskDesc)
	ollamaAvailable := err == nil
	if ollamaAvailable {
		for _, typ := range []string{"session", "note", "preference"} {
			similar, sErr := h.db.SearchSimilar(vec, typ, 3, 0.7)
			if sErr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "search similar %s: %v\n", typ, sErr)
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

	prefs, _, _ := h.db.ListPreferencesByProject(&project.ID, 0, 0)

	ctxOutput := nctx.BuildContext(nctx.ContextOptions{
		Project:         project,
		RecentSessions:  sessions,
		RecallResults:   recallResults,
		Preferences:     prefs,
		TaskDescription: body.TaskDesc,
		OllamaAvailable: ollamaAvailable,
	})

	if ctxOutput == "" {
		ctxOutput = "Nexus memory is empty. Context will build as sessions accumulate. Use `remember` to save preferences manually."
	}

	w.Header().Set("Content-Type", "text/plain")
	_, _ = fmt.Fprint(w, ctxOutput)
}

func (h *Handler) EmbedStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var queueDepth int
	err := h.db.Conn().QueryRow(`
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
		jsonError(w, "failed to get embed status", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, map[string]interface{}{
		"queue_depth":   queueDepth,
		"model":         h.ollamaModel,
		"ollama_url":    h.ollamaURL,
		"poll_interval": 30,
	}, http.StatusOK)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var dbSizeBytes int64
	_ = h.db.Conn().QueryRow("SELECT page_count * page_size FROM pragma_page_count, pragma_page_size").Scan(&dbSizeBytes)

	var queueDepth int
	_ = h.db.Conn().QueryRow(`
		SELECT COUNT(*) FROM embedding_meta WHERE status = 'pending'
	`).Scan(&queueDepth)

	ollamaClient := embed.NewClient(h.ollamaURL, h.ollamaModel, &http.Client{Timeout: 2 * time.Second})
	ollamaOK := false
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	_, err := ollamaClient.Embed(ctx, "test")
	if err == nil {
		ollamaOK = true
	}

	h.writeJSON(w, HealthResponse{
		Status:          "ok",
		Version:         h.version,
		Ollama:          ollamaOK,
		DBSizeBytes:     dbSizeBytes,
		EmbedQueueDepth: queueDepth,
	}, http.StatusOK)
}
