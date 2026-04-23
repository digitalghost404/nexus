package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/digitalghost404/nexus/internal/context"
	"github.com/digitalghost404/nexus/internal/db"
)

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	dir := t.TempDir()
	d, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestE2E_CreateProjectInsertPreferenceRecall(t *testing.T) {
	database := openTestDB(t)

	// Step 1: Create a test project via DB
	proj := db.Project{
		Name:          "e2e-test-project",
		Path:          "/tmp/e2e-test",
		Status:        "active",
		Branch:        "main",
		DiscoveredAt:  db.NullTime{Time: time.Now(), Valid: true},
		LastScannedAt: db.NullTime{Time: time.Now(), Valid: true},
	}
	projID, err := database.UpsertProject(proj)
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}
	if projID == 0 {
		t.Fatal("expected non-zero project id")
	}

	// Step 2: Insert a preference
	p := db.Preference{
		ProjectID:  &projID,
		Category:   "workflow",
		Content:    "Always run tests before committing code",
		Source:     "stated",
		Confidence: 1.0,
	}
	prefID, err := database.InsertPreference(p)
	if err != nil {
		t.Fatalf("InsertPreference: %v", err)
	}
	if prefID == 0 {
		t.Fatal("expected non-zero preference id")
	}

	// Step 3: Verify FTS recall returns the preference
	results, err := database.SearchPreferences("tests before committing")
	if err != nil {
		t.Fatalf("SearchPreferences: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 preference result from FTS search")
	}
	found := false
	for _, r := range results {
		if r.ID == prefID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected preference id %d in search results", prefID)
	}
}

func TestE2E_ContextBuilderIncludesPreferences(t *testing.T) {
	database := openTestDB(t)

	// Create project
	proj := db.Project{
		Name:          "context-test",
		Path:          "/tmp/context-test",
		Status:        "active",
		Branch:        "main",
		DiscoveredAt:  db.NullTime{Time: time.Now(), Valid: true},
		LastScannedAt: db.NullTime{Time: time.Now(), Valid: true},
	}
	projID, err := database.UpsertProject(proj)
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}

	// Insert sessions
	now := time.Now()
	s1 := db.Session{
		ProjectID:    projID,
		Summary:      "Implemented user authentication with JWT tokens",
		Source:       "test",
		StartedAt:    &now,
		FilesChanged: "[]",
		CommitsMade:  "[]",
	}
	_, err = database.InsertSession(s1)
	if err != nil {
		t.Fatalf("InsertSession: %v", err)
	}

	// Insert preferences
	pid := projID
	prefs := []db.Preference{
		{ProjectID: &pid, Category: "workflow", Content: "Always run tests before committing", Source: "stated", Confidence: 1.0},
		{ProjectID: &pid, Category: "style", Content: "Prefer terse responses", Source: "stated", Confidence: 1.0},
	}
	for _, p := range prefs {
		_, err := database.InsertPreference(p)
		if err != nil {
			t.Fatalf("InsertPreference: %v", err)
		}
	}

	// Fetch data for context builder
	project, err := database.GetProjectByName("context-test")
	if err != nil || project == nil {
		t.Fatalf("GetProjectByName: %v", err)
	}

	sessions, err := database.ListSessions(db.SessionFilter{ProjectID: project.ID, Limit: 5})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}

	fetchedPrefs, err := database.ListPreferencesByProject(&project.ID)
	if err != nil {
		t.Fatalf("ListPreferencesByProject: %v", err)
	}

	// Build context
	output := context.BuildContext(context.ContextOptions{
		Project:         project,
		RecentSessions:  sessions,
		Preferences:     fetchedPrefs,
		OllamaAvailable: false,
	})

	// Verify context includes project state
	if output == "" {
		t.Fatal("expected non-empty context output")
	}
	if !contains(output, "Project: context-test") {
		t.Errorf("context missing project header, got:\n%s", output)
	}
	if !contains(output, "main") {
		t.Errorf("context missing branch info, got:\n%s", output)
	}

	// Verify context includes preferences
	if !contains(output, "Always run tests before committing") {
		t.Errorf("context missing workflow preference, got:\n%s", output)
	}
	if !contains(output, "Prefer terse responses") {
		t.Errorf("context missing style preference, got:\n%s", output)
	}

	// Verify context includes sessions
	if !contains(output, "Implemented user authentication") {
		t.Errorf("context missing session summary, got:\n%s", output)
	}
}

func TestE2E_ColdStartContext(t *testing.T) {
	database := openTestDB(t)

	// Create project with no sessions, no preferences
	proj := db.Project{
		Name:          "empty-project",
		Path:          "/tmp/empty-project",
		Status:        "active",
		Branch:        "main",
		DiscoveredAt:  db.NullTime{Time: time.Now(), Valid: true},
		LastScannedAt: db.NullTime{Time: time.Now(), Valid: true},
	}
	_, err := database.UpsertProject(proj)
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}

	project, err := database.GetProjectByName("empty-project")
	if err != nil || project == nil {
		t.Fatalf("GetProjectByName: %v", err)
	}

	sessions, _ := database.ListSessions(db.SessionFilter{ProjectID: project.ID, Limit: 5})
	prefs, _ := database.ListPreferencesByProject(&project.ID)

	output := context.BuildContext(context.ContextOptions{
		Project:         project,
		RecentSessions:  sessions,
		Preferences:     prefs,
		OllamaAvailable: false,
	})

	// With no sessions and no preferences, only project header should exist
	if !contains(output, "Project: empty-project") {
		t.Errorf("expected project header in cold start context, got:\n%s", output)
	}
}

func TestE2E_HTTPServerPreferencesEndpoint(t *testing.T) {
	database := openTestDB(t)

	// Build a minimal test server mirroring serve.go's /api/preferences handler
	mux := http.NewServeMux()

	mux.HandleFunc("/api/preferences", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodGet {
			prefs, err := database.ListPreferencesByProject(nil)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to list preferences"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"preferences": prefs})
			return
		}

		if r.Method == http.MethodPost {
			var body struct {
				Category  string `json:"category"`
				Content   string `json:"content"`
				Source    string `json:"source"`
				ProjectID *int64 `json:"project_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Content == "" {
				w.WriteHeader(http.StatusBadRequest)
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
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "content": body.Content})
			return
		}

		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// Step 1: POST to create a global preference via HTTP (no project_id)
	createBody := map[string]interface{}{
		"category": "tool",
		"content":  "Use golangci-lint for static analysis",
		"source":   "stated",
	}
	jsonBody, _ := json.Marshal(createBody)
	resp, err := http.Post(server.URL+"/api/preferences", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		t.Fatalf("POST /api/preferences: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201 Created, got %d", resp.StatusCode)
	}

	var createResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createResp["content"] != "Use golangci-lint for static analysis" {
		t.Errorf("unexpected response content: %v", createResp["content"])
	}

	// Step 2: GET to verify the preference was stored
	resp, err = http.Get(server.URL + "/api/preferences")
	if err != nil {
		t.Fatalf("GET /api/preferences: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var listResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	prefsRaw, ok := listResp["preferences"].([]interface{})
	if !ok {
		t.Fatal("expected preferences array in response")
	}
	if len(prefsRaw) != 1 {
		t.Errorf("expected 1 preference, got %d", len(prefsRaw))
	}
}

func TestE2E_HTTPServerRecallEndpoint(t *testing.T) {
	database := openTestDB(t)

	// Create project and preferences
	proj := db.Project{
		Name:          "recall-http-test",
		Path:          "/tmp/recall-http-test",
		Status:        "active",
		DiscoveredAt:  db.NullTime{Time: time.Now(), Valid: true},
		LastScannedAt: db.NullTime{Time: time.Now(), Valid: true},
	}
	_, err := database.UpsertProject(proj)
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}

	_, err = database.InsertPreference(db.Preference{
		Category:   "workflow",
		Content:    "Always run tests before committing code",
		Source:     "stated",
		Confidence: 1.0,
	})
	if err != nil {
		t.Fatalf("InsertPreference: %v", err)
	}

	// Build test server with /api/recall handler (FTS5 fallback path)
	mux := http.NewServeMux()

	mux.HandleFunc("/api/recall", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			Query string   `json:"query"`
			Limit int      `json:"limit"`
			Types []string `json:"types"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Query == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body.Limit <= 0 {
			body.Limit = 5
		}
		if len(body.Types) == 0 {
			body.Types = []string{"session", "note", "preference"}
		}

		var results []context.RecallResult
		for _, typ := range body.Types {
			switch typ {
			case "preference":
				prefs, err := database.SearchPreferences(body.Query)
				if err != nil {
					continue
				}
				for _, p := range prefs {
					results = append(results, context.RecallResult{
						SourceType: "preference",
						SourceID:   p.ID,
						Content:    p.Content,
						Score:      p.Confidence,
					})
				}
			case "session":
				sessions, err := database.SearchSessions(body.Query)
				if err != nil {
					continue
				}
				for _, s := range sessions {
					results = append(results, context.RecallResult{
						SourceType: "session",
						SourceID:   s.ID,
						Content:    s.Summary,
						Score:      0.5,
					})
				}
			case "note":
				notes, err := database.SearchNotes(body.Query)
				if err != nil {
					continue
				}
				for _, n := range notes {
					results = append(results, context.RecallResult{
						SourceType: "note",
						SourceID:   n.ID,
						Content:    n.Content,
						Score:      0.5,
					})
				}
			}
		}

		if len(results) > body.Limit {
			results = results[:body.Limit]
		}

		json.NewEncoder(w).Encode(map[string]interface{}{"results": results})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// POST to /api/recall with a query that should match our preference
	recallBody := map[string]interface{}{
		"query": "tests before committing",
		"limit": 5,
		"types": []string{"preference"},
	}
	jsonBody, _ := json.Marshal(recallBody)
	resp, err := http.Post(server.URL+"/api/recall", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		t.Fatalf("POST /api/recall: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	var recallResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&recallResp); err != nil {
		t.Fatalf("decode recall response: %v", err)
	}

	resultsRaw, ok := recallResp["results"].([]interface{})
	if !ok {
		t.Fatal("expected results array in response")
	}
	if len(resultsRaw) == 0 {
		t.Fatal("expected at least 1 recall result")
	}
}

func TestE2E_HTTPServerInjectEndpoint(t *testing.T) {
	database := openTestDB(t)

	// Create project, session, and preference
	proj := db.Project{
		Name:            "inject-test",
		Path:            "/tmp/inject-test",
		Status:          "active",
		Branch:          "main",
		LastCommitMsg:   "feat: add auth",
		LastCommitAt:    db.NullTime{Time: time.Now(), Valid: true},
		DiscoveredAt:    db.NullTime{Time: time.Now(), Valid: true},
		LastScannedAt:   db.NullTime{Time: time.Now(), Valid: true},
	}
	projID, err := database.UpsertProject(proj)
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}

	now := time.Now()
	_, err = database.InsertSession(db.Session{
		ProjectID:    projID,
		Summary:      "Implemented JWT authentication system",
		Source:       "test",
		StartedAt:    &now,
		FilesChanged: "[]",
		CommitsMade:  "[]",
	})
	if err != nil {
		t.Fatalf("InsertSession: %v", err)
	}

	pid := projID
	_, err = database.InsertPreference(db.Preference{
		ProjectID:  &pid,
		Category:   "workflow",
		Content:    "Write tests for all new endpoints",
		Source:     "stated",
		Confidence: 1.0,
	})
	if err != nil {
		t.Fatalf("InsertPreference: %v", err)
	}

	// Build test server with /api/inject handler
	mux := http.NewServeMux()

	mux.HandleFunc("/api/inject", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			Project      string `json:"project"`
			TaskDesc     string `json:"task_description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Project == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		project, err := database.GetProjectByName(body.Project)
		if err != nil || project == nil {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"error":"project not found"}`)
			return
		}

		sessions, _ := database.ListSessions(db.SessionFilter{ProjectID: project.ID, Limit: 5})
		prefs, _ := database.ListPreferencesByProject(&project.ID)

		ctxOutput := context.BuildContext(context.ContextOptions{
			Project:         project,
			RecentSessions:  sessions,
			Preferences:     prefs,
			TaskDescription: body.TaskDesc,
			OllamaAvailable: false,
		})

		if ctxOutput == "" {
			ctxOutput = "Nexus memory is empty. Context will build as sessions accumulate."
		}

		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, ctxOutput)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// POST to /api/inject
	injectBody := map[string]interface{}{
		"project":          "inject-test",
		"task_description": "adding rate limiting to auth endpoints",
	}
	jsonBody, _ := json.Marshal(injectBody)
	resp, err := http.Post(server.URL+"/api/inject", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		t.Fatalf("POST /api/inject: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	output := buf.String()

	if output == "" {
		t.Fatal("expected non-empty inject output")
	}
	if !contains(output, "Project: inject-test") {
		t.Errorf("inject output missing project header, got:\n%s", output)
	}
	if !contains(output, "Write tests for all new endpoints") {
		t.Errorf("inject output missing preference, got:\n%s", output)
	}
	if !contains(output, "Implemented JWT authentication") {
		t.Errorf("inject output missing session, got:\n%s", output)
	}
}

func TestE2E_HTTPServerEmbedStatusEndpoint(t *testing.T) {
	database := openTestDB(t)

	// Build test server with /api/embed/status handler
	mux := http.NewServeMux()

	mux.HandleFunc("/api/embed/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

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
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"queue_depth":   queueDepth,
			"model":         "nomic-embed-text",
			"ollama_url":    "http://localhost:11434",
			"poll_interval": 30,
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// GET /api/embed/status with no items in queue
	resp, err := http.Get(server.URL + "/api/embed/status")
	if err != nil {
		t.Fatalf("GET /api/embed/status: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	var statusResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		t.Fatalf("decode status response: %v", err)
	}

	depth, ok := statusResp["queue_depth"].(float64)
	if !ok {
		t.Fatal("expected queue_depth in response")
	}
	if depth != 0 {
		t.Errorf("expected queue_depth 0 for empty database, got %f", depth)
	}

	// Now insert a session with summary to create a pending embed item
	proj := db.Project{
		Name:          "embed-status-test",
		Path:          "/tmp/embed-status-test",
		Status:        "active",
		DiscoveredAt:  db.NullTime{Time: time.Now(), Valid: true},
		LastScannedAt: db.NullTime{Time: time.Now(), Valid: true},
	}
	projID, err := database.UpsertProject(proj)
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}

	now := time.Now()
	_, err = database.InsertSession(db.Session{
		ProjectID:    projID,
		Summary:      "Test session for embed status",
		Source:       "test",
		StartedAt:    &now,
		FilesChanged: "[]",
		CommitsMade:  "[]",
	})
	if err != nil {
		t.Fatalf("InsertSession: %v", err)
	}

	// GET again — queue should now have 1 item
	resp, err = http.Get(server.URL + "/api/embed/status")
	if err != nil {
		t.Fatalf("GET /api/embed/status (after insert): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		t.Fatalf("decode status response (after insert): %v", err)
	}

	depth, ok = statusResp["queue_depth"].(float64)
	if !ok {
		t.Fatal("expected queue_depth in response")
	}
	if depth != 1 {
		t.Errorf("expected queue_depth 1 after inserting session, got %f", depth)
	}
}

func TestE2E_PreferenceDecayAndMaintenance(t *testing.T) {
	database := openTestDB(t)

	// Insert an inferred preference with old timestamp
	oldTime := time.Now().Add(-60 * 24 * time.Hour) // 60 days ago
	id, err := database.InsertPreference(db.Preference{
		Category:         "pattern",
		Content:          "Works late at night",
		Source:           "inferred",
		Confidence:       0.4,
		CreatedAt:        oldTime,
		UpdatedAt:        oldTime,
		LastReferencedAt: &oldTime,
	})
	if err != nil {
		t.Fatalf("InsertPreference: %v", err)
	}

	// Run decay
	err = database.DecayPreferences()
	if err != nil {
		t.Fatalf("DecayPreferences: %v", err)
	}

	// Verify confidence decayed
	fetched, err := database.GetPreference(id)
	if err != nil {
		t.Fatalf("GetPreference: %v", err)
	}
	if fetched.Confidence >= 0.4 {
		t.Errorf("expected confidence to decay below 0.4, got %f", fetched.Confidence)
	}

	// Insert a low-confidence inferred preference
	_, err = database.InsertPreference(db.Preference{
		Category:   "pattern",
		Content:    "Very uncertain pattern",
		Source:     "inferred",
		Confidence: 0.1,
	})
	if err != nil {
		t.Fatalf("InsertPreference: %v", err)
	}

	// Delete low-confidence preferences
	deleted, err := database.DeleteLowConfidencePreferences(0.15)
	if err != nil {
		t.Fatalf("DeleteLowConfidencePreferences: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}
}

func TestE2E_AgentIsolation(t *testing.T) {
	// Verify that different agents get different database paths
	// This is tested at the config level, but we verify the DB files are separate

	dir := t.TempDir()
	claudePath := filepath.Join(dir, "claude", "test.db")
	opencodePath := filepath.Join(dir, "opencode", "test.db")

	claudeDB, err := db.Open(claudePath)
	if err != nil {
		t.Fatalf("open claude db: %v", err)
	}
	defer claudeDB.Close()

	opencodeDB, err := db.Open(opencodePath)
	if err != nil {
		t.Fatalf("open opencode db: %v", err)
	}
	defer opencodeDB.Close()

	// Insert different data in each
	proj1 := db.Project{
		Name:          "claude-project",
		Path:          "/tmp/claude-project",
		Status:        "active",
		DiscoveredAt:  db.NullTime{Time: time.Now(), Valid: true},
		LastScannedAt: db.NullTime{Time: time.Now(), Valid: true},
	}
	_, err = claudeDB.UpsertProject(proj1)
	if err != nil {
		t.Fatalf("UpsertProject (claude): %v", err)
	}

	proj2 := db.Project{
		Name:          "opencode-project",
		Path:          "/tmp/opencode-project",
		Status:        "active",
		DiscoveredAt:  db.NullTime{Time: time.Now(), Valid: true},
		LastScannedAt: db.NullTime{Time: time.Now(), Valid: true},
	}
	_, err = opencodeDB.UpsertProject(proj2)
	if err != nil {
		t.Fatalf("UpsertProject (opencode): %v", err)
	}

	// Verify isolation: claude DB should not see opencode project
	p, _ := claudeDB.GetProjectByName("opencode-project")
	if p != nil {
		t.Error("claude DB should not see opencode project — agent isolation broken")
	}

	// Verify isolation: opencode DB should not see claude project
	p, _ = opencodeDB.GetProjectByName("claude-project")
	if p != nil {
		t.Error("opencode DB should not see claude project — agent isolation broken")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
