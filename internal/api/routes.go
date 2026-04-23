package api

import "net/http"

func RegisterRoutes(mux *http.ServeMux, h *Handler, apiToken string, corsOrigins []string, rateLimit int) {
	logging := LoggingMiddleware()
	cors := CORSMiddleware(corsOrigins)
	auth := AuthMiddleware(apiToken)

	healthHandler := logging(cors(http.HandlerFunc(h.Health)))
	mux.Handle("GET /api/v1/health", healthHandler)

	authMux := http.NewServeMux()

	authMux.Handle("POST /api/v1/capture", http.HandlerFunc(h.Capture))
	authMux.Handle("GET /api/v1/notes", http.HandlerFunc(h.ListNotes))
	authMux.Handle("POST /api/v1/notes", http.HandlerFunc(h.CreateNote))
	authMux.Handle("GET /api/v1/preferences", http.HandlerFunc(h.ListPreferences))
	authMux.Handle("POST /api/v1/preferences", http.HandlerFunc(h.CreatePreference))
	authMux.Handle("PATCH /api/v1/preferences/{id}", http.HandlerFunc(h.UpdatePreference))
	authMux.Handle("DELETE /api/v1/preferences/{id}", http.HandlerFunc(h.DeletePreference))

	recallRateLimit := RateLimiterMiddleware(rateLimit)
	authMux.Handle("POST /api/v1/recall", recallRateLimit(http.HandlerFunc(h.Recall)))

	authMux.Handle("POST /api/v1/inject", http.HandlerFunc(h.Inject))
	authMux.Handle("GET /api/v1/embed/status", http.HandlerFunc(h.EmbedStatus))

	wrapped := logging(cors(auth(authMux)))

	mux.Handle("/api/v1/", wrapped)
}
