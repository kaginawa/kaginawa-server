package main

import (
	"log"
	"net/http"
	"strings"
)

func handleCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	// Validate API key
	apiKey := strings.Replace(r.Header.Get("Authorization"), "token ", "", 1)
	if len(apiKey) == 0 {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	if ok, _, err := database.ValidateAdminAPIKey(apiKey); !ok || err != nil {
		if err != nil {
			log.Printf("failed to validate admin api key: %v", err)
			http.Error(w, "Database unavailable", http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
	}

	// Get record

	// TODO:
}
