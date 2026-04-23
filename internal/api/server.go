package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/embed"
)

type Server struct {
	handler     *Handler
	mux         *http.ServeMux
	apiToken    string
	corsOrigins []string
	rateLimit   int
	embedWorker *embed.Worker
	httpServer  *http.Server
}

func NewServer(database *db.DB, embedWorker *embed.Worker, apiToken string, corsOrigins []string, rateLimit int, ollamaURL, ollamaModel, version string) *Server {
	h := NewHandler(database, embedWorker, ollamaURL, ollamaModel, version)
	mux := http.NewServeMux()
	RegisterRoutes(mux, h, apiToken, corsOrigins, rateLimit)

	return &Server{
		handler:     h,
		mux:         mux,
		apiToken:    apiToken,
		corsOrigins: corsOrigins,
		rateLimit:   rateLimit,
		embedWorker: embedWorker,
	}
}

func (s *Server) ListenAndServe(addr string) error {
	s.embedWorker.Start()

	s.httpServer = &http.Server{Addr: addr, Handler: s.mux}

	fmt.Printf("Nexus API listening on http://%s\n", addr)

	errCh := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-quit:
		fmt.Println("\nShutting down Nexus API...")
	case err := <-errCh:
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.embedWorker.Stop()
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.embedWorker.Stop()
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}
