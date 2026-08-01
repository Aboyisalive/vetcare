package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type apiError struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiError{Error: msg})
}

func readJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	return nil
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

// parseOrigins splits a comma-separated ALLOWED_ORIGINS value into a lookup set.
func parseOrigins(raw string) map[string]bool {
	allowed := map[string]bool{}
	for _, o := range strings.Split(raw, ",") {
		if o = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(o), "/")); o != "" {
			allowed[o] = true
		}
	}
	return allowed
}

// cors grants credentialed access to the origins in allowed, and to nothing else.
//
// The deployed setup needs no entries at all: the static site rewrites /api/* to
// this service, and vite proxies /api in dev, so the browser considers every call
// same-origin and CORS never engages. Populate ALLOWED_ORIGINS only if the front
// end is ever served from a genuinely different origin.
//
// Never reflect an arbitrary Origin here. Paired with
// Access-Control-Allow-Credentials, that lets any site a signed-in user visits
// read their VetCare data with their session cookie attached.
func cors(allowed map[string]bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && allowed[strings.TrimSuffix(origin, "/")] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
		// Vary regardless: the response differs by Origin even when it is rejected,
		// so a cache must not serve one origin's response to another.
		w.Header().Add("Vary", "Origin")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
