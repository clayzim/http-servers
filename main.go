package main

import (
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

// Readiness endpoint
// Route: /healthz
// Method: any
// Response:
// 	Headers:
//		Content-Type: text/plain; charset=utf-8
//	Body:
//		"OK"
func readiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("content-type", "text/plain; charset=utf-8")
	// For status OK, this could be implicit with w.Write call
	// TODO: Optionally return 503: Service Unavailable if server isn't ready
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(http.StatusText(http.StatusOK)))
	if err != nil {
		log.Printf("failed to write readiness response: %s\n", err)
		// TODO: Respond with 5XX status
	}
}

// Structure to hold all state that server maintains
// between requests
type serverState struct {
	// Atomic so multiple goroutines can share the value
	fileserverHits atomic.Int32
}

// Increment metrics then run typical handler
// Wraps a handler in a handler with added logic
func (state *serverState) mwMetricsInc(next http.Handler) http.Handler {
	// Wrap function with HandlerFunc to return a Handler
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

// Metrics endpoint
// Body: plaintext with number of requests processed since server was started
// Method on serverState to access its member fields
func (state *serverState) metrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("content-type", "text/plain; charset=utf-8")
	// For status OK, this could be implicit with w.Write call
	// TODO: Optionally return 503: Service Unavailable if server isn't ready
	w.WriteHeader(http.StatusOK)
	// Atomically accesses the number of hits
	_, err := fmt.Fprintf(w, "Hits: %d", state.fileserverHits.Load())
	if err != nil {
		log.Printf("failed to write metrics response: %s\n", err)
		// TODO: Respond with 5XX status
	}
}

// Set fileserver hits to 0
// TODO: Consider HTTP semantics, probably only some
// HTTP methods should work
func (state *serverState) reset(w http.ResponseWriter, r *http.Request) {
	state.fileserverHits.Store(0)
	w.WriteHeader(http.StatusOK)
}

func main() {
	mux := http.NewServeMux()
	server := http.Server{Handler: mux, Addr: ":8080"}
	srvState := serverState{}
	// StripPrefix means any path not prefixed "/app/" responds status 404 Not Found
	appHandler := http.StripPrefix("/app/", http.FileServer(http.Dir(".")))
	mux.Handle("/app/", srvState.mwMetricsInc(appHandler))
	// Readiness endpoint path based on Kubernetes pattern
	mux.HandleFunc("GET /api/healthz", readiness)
	mux.HandleFunc("GET /api/metrics", srvState.metrics)
	mux.HandleFunc("POST /api/reset", srvState.reset)

	err := server.ListenAndServe()
	if err != nil {
		fmt.Println(err)
	}
}