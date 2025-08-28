package main

import (
	"encoding/json"
	"log"
	"net/http"
	"slices"
	"strings"
)

// Typesafe whitelist of valid JSON parameters in
// request bodies
type jsonRequest struct {
	Body string `json:"body"`
	Email string `json:"email"`
}

// Callers to this must handle the zero value case for
// the JSON parameter value they intend to use
func readJSONBody(r *http.Request) (jsonRequest, error) {
	decoder := json.NewDecoder(r.Body)
	request := jsonRequest{}
	err := decoder.Decode((&request))
	return request, err
}

func respondWithJSON(w http.ResponseWriter, code int, in any) {
	out, err := json.Marshal(in)
	if err != nil {
		log.Printf("failed to marshal JSON: %s\n", err)
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

// TODO: Increase sensitivity so punctuation can't cause a false negative
func censorProfanity(in string) (cleaned string) {
	const censor string = "****"
	// TODO: Consider using regex to substitute in place
	words := strings.Split(in, " ")
	for i, word := range words {
		// Is lowercased word in profane dictionary?
		if slices.Contains(profanity, strings.ToLower(word)) {
			// Overwrite that word with asterisks
			words[i] = censor
		}
	}
	return strings.Join(words, " ")
}