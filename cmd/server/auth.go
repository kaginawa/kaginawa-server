package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

func validateAPIKey(r *http.Request, admin bool) bool {
	apiKey := extractAPIKey(r)
	if len(apiKey) == 0 {
		return false
	}
	if admin {
		if ok, _, err := database.ValidateAdminAPIKey(apiKey); !ok {
			if err != nil {
				log.Printf("failed to validate admin api key: %v", err)
				return false
			}
			if !ok {
				return false
			}
		}
	} else {
		if ok, _, err := database.ValidateAPIKey(apiKey); !ok {
			if err != nil {
				log.Printf("failed to validate api key: %v", err)
				return false
			}
			if !ok {
				return false
			}
		}
	}
	return true
}

func extractAPIKey(r *http.Request) string {
	return strings.Replace(r.Header.Get("Authorization"), "token ", "", 1)
}

func validateCookie(r *http.Request) bool {
	cookie, err := r.Cookie(authCookieName)
	if err != nil {
		return false
	}
	return cookie.Value == fmt.Sprintf("%x", loginToken)
}
