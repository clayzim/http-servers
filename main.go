package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"unicode/utf8"
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
	w.Header().Add("content-type", "text/html")
	// For status OK, this could be implicit with w.Write call
	// TODO: Optionally return 503: Service Unavailable if server isn't ready
	w.WriteHeader(http.StatusOK)
	// Atomically accesses the number of hits
	_, err := fmt.Fprintf(w, `<html>
	<body>
    	<h1>Welcome, Chirpy Admin</h1>
    	<p>Chirpy has been visited %d times!</p>
	</body>
</html>`, state.fileserverHits.Load())
	if err != nil {
		log.Printf("failed to write metrics response: %s\n", err)
		// TODO: Respond with 5XX status
	}
}

// Set fileserver hits to 0
func (state *serverState) reset(w http.ResponseWriter, r *http.Request) {
	state.fileserverHits.Store(0)
	w.WriteHeader(http.StatusOK)
}

// Silly rule that limits Chrips' character count
const MaxChirpLength = 140;

func respondWithJSON(w http.ResponseWriter, code int, in any) {
	out, err := json.Marshal(in)
	if err != nil {
		log.Printf("failed to marshal JSON: %s", err)
	}

	w.Header().Add("content-type", "application/json")
	w.WriteHeader(code)
	w.Write(out)
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	if code >= 500 {
		log.Printf("responding with server error: %s\n", msg)
	}
	type errResponse struct {
		Error string `json:"error"`
	}

	respondWithJSON(w, code, errResponse{Error: msg})
}

func validate_chirp(w http.ResponseWriter, r *http.Request) {
	// TODO: Refactor parsing body into its own function
	// Begin reading JSON Chirp body
	type parameters struct {
		Body string `json:"body"`
	}

	decoder := json.NewDecoder(r.Body)
	req := parameters{}
	err := decoder.Decode(&req)
	if err != nil {
		// Log failure, respond 500, end handler
		// We can't validate a body we can't parse
		log.Printf("failed to decode parameters: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// End reading JSON Chirp body

	len := utf8.RuneCountInString(req.Body)
	// Send error for empty chirp
	if len <= 0 {
		respondWithError(w, http.StatusBadRequest, "Chirp cannot be empty")
		return
	}
	// Send error for too-long chirp
	if len > MaxChirpLength {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long")
		return
	}

	validResponse := map[string]any{"valid": true}
	respondWithJSON(w, http.StatusOK, validResponse)
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
	mux.HandleFunc("GET /admin/metrics", srvState.metrics)
	mux.HandleFunc("POST /admin/reset", srvState.reset)
	mux.HandleFunc("POST /api/validate_chirp", validate_chirp)

	err := server.ListenAndServe()
	if err != nil {
		fmt.Println(err)
	}
}