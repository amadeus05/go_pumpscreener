package httpserver

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"pumpscreener/src/core"
)

type Server struct {
	httpServer *http.Server
	state      *core.AppState
}

func New(port string, state *core.AppState) *Server {
	server := &Server{state: state}

	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleRoot)
	mux.HandleFunc("/uptime", server.handleUptime)
	mux.HandleFunc("/health", server.handleHealth)

	server.httpServer = &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return server
}

func (s *Server) Run(ctx context.Context) error {
	errs := make(chan error, 1)
	go func() {
		log.Printf("http server listening on %s", s.httpServer.Addr)
		errs <- s.httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(shutdownCtx)
	case err := <-errs:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	snapshot := s.state.Snapshot()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintf(w, "pumpscreener is running\nuptime: %s\nwebsocket: %s\n", core.HumanDuration(snapshot.Uptime), snapshot.WebSocketState)
}

func (s *Server) handleUptime(w http.ResponseWriter, r *http.Request) {
	snapshot := s.state.Snapshot()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintf(w, "Uptime: %s\n", core.HumanDuration(snapshot.Uptime))
	_, _ = fmt.Fprintf(w, "WebSocket: %s\n", snapshot.WebSocketState)
	_, _ = fmt.Fprintf(w, "Known pairs: %d\n", snapshot.KnownPairs)
	_, _ = fmt.Fprintf(w, "Tracked symbols: %d\n", snapshot.TrackedSymbols)
	_, _ = fmt.Fprintf(w, "Active rules: %d\n", snapshot.ActiveRules)
	if !snapshot.LastTickAt.IsZero() {
		_, _ = fmt.Fprintf(w, "Last tick: %s\n", snapshot.LastTickAt.Format(time.RFC3339))
	}
	if snapshot.LastError != "" {
		_, _ = fmt.Fprintf(w, "Last error: %s\n", snapshot.LastError)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("OK\n"))
}
