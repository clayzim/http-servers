package main

import (
	"fmt"
	"log"
	"net/http"
	"unicode/utf8"

	"github.com/clayzim/http-servers/internal/database"
	"github.com/google/uuid"
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
// Delete all users from DB
func (cfg *serverState) reset(w http.ResponseWriter, r *http.Request) {
	// Not a development environment?
	// Respond "Forbidden" and do nothing
	if cfg.platform != devPlatform {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// Delete all users from the database
	err := cfg.db.DeleteAllUsers(r.Context())
	if err != nil {
		// If that query fails, 500 and return
		respondWithError(
			w,
			http.StatusInternalServerError,
			"Failed to delete all users",
		)
		return
	}
	cfg.fileserverHits.Store(0)
	// Confirm successful deletion by indicating
	// the lack of content
	w.WriteHeader(http.StatusNoContent)
}

// Valid parameters for a /chirps request
type chirpParameters struct {
	Body string `json:"body"`
	UserID uuid.UUID `json:"user_id"`
}

func (cfg *serverState) createChirp(w http.ResponseWriter, r *http.Request) {
	// Read JSON Chirp body
	params := chirpParameters{}
	err := readJSONBody(r, &params)
	if err != nil {
		respondWithError(
			w,
			http.StatusInternalServerError,
			"Failed to parse Chirp")
		return
	}
	// Nonexistant user_ids are disallowed by database schema
	// TODO: Add validation that the requester is authorized
	// to chirp on this user's behalf
	body := params.Body

	len := utf8.RuneCountInString(body)
	// Send error for empty chirp
	if len <= 0 {
		respondWithError(w, http.StatusBadRequest, "Chirp cannot be empty")
		return
	}
	// Send error for too-long chirp
	if len > maxChirpLength {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long")
		return
	}

	// If length check passes, replace profane words
	body = censorProfanity(body)
	p := database.CreateChirpParams{
		Body: body,
		UserID: params.UserID,
	}
	chirp, err := cfg.db.CreateChirp(r.Context(), p)
	if err != nil {
		respondWithError(
			w,
			http.StatusInternalServerError,
			"Failed to create chirp")
		return
	}
	respondWithJSON(w, http.StatusCreated, Chirp(chirp))
}

func (cfg *serverState) getAllChirps(w http.ResponseWriter, r *http.Request) {
	// No request body needed
	// Execute the database query
	dbChirps, err := cfg.db.GetAllChirps(r.Context())
	if err != nil {
		respondWithError(
			w,
			http.StatusInternalServerError,
			"Failed to retrieve all chirps")
		return
	}
	// Wrap all chirps in JSON-annotated model
	var chirps []Chirp
	for _, dbChirp := range dbChirps {
		chirps = append(chirps, Chirp(dbChirp))
	}
	// Write a JSON response with a list of all chirps
	respondWithJSON(w, http.StatusOK, chirps)
}

// Valid parameters for a /users request
type userParameters struct {
	Email string `json:"email"`
}

func (cfg *serverState) createUser(w http.ResponseWriter, r *http.Request) {
	// Read user email
	params := userParameters{}
	err := readJSONBody(r, &params)
	if err != nil {
		respondWithError(
			w,
			http.StatusInternalServerError,
			"Failed to parse user email",
		)
		return
	}
	// TODO: Handle zero value (empty email)
	// Duplicates are disallowed by database schema
	email := params.Email

	user, err := cfg.db.CreateUser(r.Context(), email)
	if err != nil {
		respondWithError(
			w,
			http.StatusInternalServerError,
			"Failed to create user")
		return
	}
	respondWithJSON(w, http.StatusCreated, User(user))
}