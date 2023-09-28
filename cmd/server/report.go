package main

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/kaginawa/kaginawa-server/internal/kaginawa"
)

// reply defines all reply message attributes
type reply struct {
	SSHServerHost string `json:"ssh_host,omitempty"`
	SSHServerPort int    `json:"ssh_port,omitempty"`
	SSHServerUser string `json:"ssh_user,omitempty"`
	SSHKey        string `json:"ssh_key,omitempty"`
	SSHPassword   string `json:"ssh_password,omitempty"`
}

// handleReport handles report submits.
//
// - Method: POST
// - Client: Kaginawa
// - Access: Normal
// - Response: JSON
func handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if !validateAPIKey(r, false) {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	reader := r.Body
	defer safeClose(reader, "Report body")
	if r.Header.Get("Content-Encoding") == "gzip" {
		r, err := gzip.NewReader(r.Body)
		if err != nil {
			log.Printf("failed to read gzipped request body: %v", err)
			http.Error(w, "Response read error", http.StatusInternalServerError)
			return
		}
		reader = r
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		log.Printf("failed to read request body: %v", err)
		http.Error(w, "Response read error", http.StatusInternalServerError)
		return
	}
	var report kaginawa.Report
	if err := json.Unmarshal(body, &report); err != nil {
		http.Error(w, "Response unmarshal error", http.StatusBadRequest)
		return
	}
	report.ServerTime = time.Now().UTC().Unix()
	report.APIKey = extractAPIKey(r)
	log.Printf("REPORT from %s %s %d", report.ID, report.CustomID, report.SSHRemotePort)

	// Pick global IP
	report.GlobalIP = remoteIP(r)

	// Reverse lookup
	if len(report.GlobalIP) > 0 {
		name, err := reverseLookup(report.GlobalIP)
		if err != nil {
			report.GlobalHost = report.GlobalIP
		}
		report.GlobalHost = name
	}
	if len(report.GlobalHost) == 0 {
		report.GlobalHost = report.GlobalIP
	}

	if err := db.PutReport(report); err != nil {
		log.Printf("failed to put Report (id=%s): %v", report.ID, err)
		http.Error(w, "Failed to put database", http.StatusInternalServerError)
		return
	}

	var msg reply
	if len(kaginawa.SSHServers) > 0 {
		i := rand.Int() % len(kaginawa.SSHServers)
		msg = reply{
			SSHServerHost: kaginawa.SSHServers[i].Host,
			SSHServerPort: kaginawa.SSHServers[i].Port,
			SSHServerUser: kaginawa.SSHServers[i].User,
			SSHKey:        kaginawa.SSHServers[i].Key,
			SSHPassword:   kaginawa.SSHServers[i].Password,
		}
	}
	rawReply, err := json.Marshal(msg)
	if err != nil {
		http.Error(w, "Response marshal error", http.StatusInternalServerError)
		return
	}
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusCreated)
		gz := gzip.NewWriter(w)
		defer safeClose(gz, "gzipped response")
		if _, err := gz.Write(rawReply); err != nil {
			log.Printf("failed to write response: %v", err)
		}
	} else {
		w.WriteHeader(http.StatusCreated)
		if _, err := w.Write(rawReply); err != nil {
			log.Printf("failed to write response: %v", err)
		}
	}
}

func reverseLookup(globalIP string) (string, error) {
	if globalIP == "[::1]" {
		return "", nil
	}
	names, err := net.LookupAddr(globalIP)
	if err != nil {
		return "", err
	}
	if len(names) == 0 {
		return globalIP, nil // no lookup address
	}
	return strings.TrimRight(names[0], "."), nil
}
