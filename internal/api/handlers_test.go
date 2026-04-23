package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/embed"
)

func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func setupTestHandler(t *testing.T) *Handler {
	t.Helper()
	database := setupTestDB(t)
	worker := embed.NewWorker(nil, database)
	return NewHandler(database, worker, "http://localhost:11434", "nomic-embed-text", "0.2.0")
}

func TestCapture_Valid(t *testing.T) {
	h := setupTestHandler(t)
	body := `{"dir":"."}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/capture", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Capture(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCapture_EmptyDir(t *testing.T) {
	h := setupTestHandler(t)
	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/capture", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Capture(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCapture_PathTraversal(t *testing.T) {
	h := setupTestHandler(t)
	body := `{"dir":"../../../etc/passwd"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/capture", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Capture(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCapture_AbsoluteOutsideHome(t *testing.T) {
	h := setupTestHandler(t)
	body := `{"dir":"/tmp/outside-home"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/capture", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Capture(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListNotes_DefaultLimit(t *testing.T) {
	h := setupTestHandler(t)
	database := h.db

	_, err := database.InsertNote(db.Note{Content: "note 1"})
	if err != nil {
		t.Fatalf("insert note: %v", err)
	}
	_, err = database.InsertNote(db.Note{Content: "note 2"})
	if err != nil {
		t.Fatalf("insert note: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notes", nil)
	w := httptest.NewRecorder()

	h.ListNotes(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	notes, ok := resp["notes"].([]interface{})
	if !ok {
		t.Fatalf("expected notes array")
	}
	if len(notes) != 2 {
		t.Errorf("expected 2 notes, got %d", len(notes))
	}
}

func TestListNotes_MaxLimit(t *testing.T) {
	h := setupTestHandler(t)
	database := h.db

	for i := 0; i < 50; i++ {
		_, err := database.InsertNote(db.Note{Content: "note"})
		if err != nil {
			t.Fatalf("insert note: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notes?limit=200", nil)
	w := httptest.NewRecorder()

	h.ListNotes(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	notes := resp["notes"].([]interface{})
	if len(notes) > 100 {
		t.Errorf("expected max 100 notes, got %d", len(notes))
	}
}

func TestListNotes_NegativeLimit(t *testing.T) {
	h := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notes?limit=-5", nil)
	w := httptest.NewRecorder()

	h.ListNotes(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	notes := resp["notes"].([]interface{})
	if len(notes) != 0 {
		t.Errorf("expected 0 notes (empty db, negative limit defaults to 20), got %d", len(notes))
	}
}

func TestHealth_NoAuth(t *testing.T) {
	database := setupTestDB(t)
	worker := embed.NewWorker(nil, database)
	h := NewHandler(database, worker, "http://localhost:11434", "nomic-embed-text", "0.2.0")

	mux := http.NewServeMux()
	RegisterRoutes(mux, h, "test-token", nil, 60)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}
	if resp.Version != "0.2.0" {
		t.Errorf("expected version '0.2.0', got %q", resp.Version)
	}
}

func TestAuthMiddleware_WithToken(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	middleware := AuthMiddleware("secret-token")
	protected := middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	w := httptest.NewRecorder()

	protected.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthMiddleware_WithoutToken(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	middleware := AuthMiddleware("secret-token")
	protected := middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	protected.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	middleware := AuthMiddleware("secret-token")
	protected := middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()

	protected.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_NoTokenConfig(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	middleware := AuthMiddleware("")
	protected := middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	protected.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 when no token configured, got %d", w.Code)
	}
}

func TestCORSMiddleware_MatchingOrigin(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	middleware := CORSMiddleware([]string{"https://example.com"})
	wrapped := middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("expected origin header, got %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSMiddleware_NoOrigins(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	middleware := CORSMiddleware(nil)
	wrapped := middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected '*' origin, got %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSMiddleware_Options(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	middleware := CORSMiddleware(nil)
	wrapped := middleware(handler)

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS, got %d", w.Code)
	}
}

func TestRateLimiter(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	middleware := RateLimiterMiddleware(10)
	wrapped := middleware(handler)

	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 on 11th request, got %d", w.Code)
	}
}

func TestValidatePath_DotDot(t *testing.T) {
	err := ValidatePath("../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestValidatePath_AbsoluteOutsideHome(t *testing.T) {
	err := ValidatePath("/tmp/outside")
	if err == nil {
		t.Error("expected error for path outside home")
	}
}

func TestValidatePath_ValidPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	err = ValidatePath(home + "/projects/nexus")
	if err != nil {
		t.Errorf("expected valid path, got error: %v", err)
	}
}

func TestValidatePath_InvalidCharacters(t *testing.T) {
	err := ValidatePath("path/with spaces")
	if err == nil {
		t.Error("expected error for invalid characters")
	}
}

func TestTokenBucket_Allow(t *testing.T) {
	bucket := NewTokenBucket(5)

	for i := 0; i < 5; i++ {
		if !bucket.Allow() {
			t.Errorf("token %d should be allowed", i+1)
		}
	}

	if bucket.Allow() {
		t.Error("6th token should be denied")
	}
}

func TestJsonError(t *testing.T) {
	w := httptest.NewRecorder()
	jsonError(w, "test error", http.StatusBadRequest)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error != "test error" {
		t.Errorf("expected 'test error', got %q", resp.Error)
	}
}
