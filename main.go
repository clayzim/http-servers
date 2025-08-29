package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/clayzim/http-servers/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// Structure to hold all state that server maintains
// between requests
type serverState struct {
	// Atomic so multiple goroutines can share the value
	fileserverHits atomic.Int32
	db *database.Queries
	platform string
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

func main() {
	// Establish database connection
	// Load contents of .env file into environment variables
	godotenv.Load()
	platform := os.Getenv("PLATFORM")
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %s\n", err)
	}

	mux := http.NewServeMux()
	server := http.Server{Handler: mux, Addr: ":8080"}
	srvState := serverState{
		// Supply database connection for handler use
		db: database.New(db),
		platform: platform,
	}

	// StripPrefix means any path not prefixed "/app/" responds status 404 Not Found
	appHandler := http.StripPrefix("/app/", http.FileServer(http.Dir(".")))
	mux.Handle("/app/", srvState.mwMetricsInc(appHandler))
	// Readiness endpoint path based on Kubernetes pattern
	mux.HandleFunc("GET /api/healthz", readiness)
	mux.HandleFunc("GET /admin/metrics", srvState.metrics)
	mux.HandleFunc("POST /admin/reset", srvState.reset)
	mux.HandleFunc("POST /api/chirps", srvState.createChirp)
	mux.HandleFunc("GET /api/chirps", srvState.getAllChirps)
	mux.HandleFunc("GET /api/chirps/{chirpID}", srvState.getChirp)
	mux.HandleFunc("POST /api/users", srvState.createUser)

	err = server.ListenAndServe()
	if err != nil {
		fmt.Println(err)
	}
}