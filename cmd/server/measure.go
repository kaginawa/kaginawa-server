package main

import (
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

const maxLengthKB = 10 * 1024 // 10MB

// handleMeasure handles network throughput measurement endpoint.
//
// - Method: GET or POST
//   - GET /measure/0 --- respond empty body for round trip time measurement
//   - GET /measure/500  --- respond 500KB body
//   - POST /measure/500 --- receive 500KB body
//
// - Client: Kaginawa
// - Access: Normal
// - Response: Raw
func handleMeasure(w http.ResponseWriter, r *http.Request) {
	kb, err := strconv.Atoi(mux.Vars(r)["kb"])
	if err != nil || kb < 0 || kb > maxLengthKB {
		http.NotFound(w, r)
		return
	}
	if r.ContentLength > maxLengthKB*1024 {
		http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
		return
	}
	switch r.Method {
	case http.MethodHead:
		return
	case http.MethodGet:
		raw := make([]byte, kb*1024)
		if _, err := w.Write(raw); err != nil {
			log.Printf("failed to write measure body: %v", err)
			return
		}
	case http.MethodPost:
		if int64(kb)*1024 != r.ContentLength {
			log.Printf("cl=%d rp=%d", r.ContentLength, int64(kb)*1024)
			http.Error(w, "content length != request path", http.StatusBadRequest)
			return
		}
		defer safeClose(r.Body, "measure body")
		_, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("failed to write measure body: %v", err)
			return
		}
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
}
