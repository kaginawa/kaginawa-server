package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/kaginawa/kaginawa-server"
)

// reply defines all of reply message attributes
type reply struct {
	Reboot        bool   `json:"reboot,omitempty"` // Reboot requested from the server
	SSHServerHost string `json:"ssh_host,omitempty"`
	SSHServerPort int    `json:"ssh_port,omitempty"`
	SSHServerUser string `json:"ssh_user,omitempty"`
	SSHKey        string `json:"ssh_key,omitempty"`
	SSHPassword   string `json:"ssh_password,omitempty"`
}

func handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if !validateAPIKey(r, false) {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("failed to read request: %v", err)
		http.Error(w, "Response read error", http.StatusInternalServerError)
		return
	}
	defer safeClose(r.Body, "Report body")
	var report kaginawa.Report
	if err := json.Unmarshal(body, &report); err != nil {
		http.Error(w, "Response unmarshal error", http.StatusBadRequest)
		return
	}
	report.ServerTime = time.Now().UTC().Unix()
	report.APIKey = extractAPIKey(r)
	log.Printf("REPORT from %s %s %d", report.ID, report.CustomID, report.SSHRemotePort)

	// Pick global IP
	forwardedFor := r.Header.Get("X-Forwarded-For")
	if len(forwardedFor) > 0 {
		list := strings.Split(forwardedFor, ",")
		report.GlobalIP = list[len(list)-1]
	}
	if len(report.GlobalIP) == 0 {
		report.GlobalIP = trimPort(r.RemoteAddr)
	}

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

	// TODO: implement reboot request feature

	var msg reply
	if len(kaginawa.SSHServers) > 0 {
		i := rand.Int() % len(kaginawa.SSHServers)
		msg = reply{
			Reboot:        false,
			SSHServerHost: kaginawa.SSHServers[i].Host,
			SSHServerPort: kaginawa.SSHServers[i].Port,
			SSHServerUser: kaginawa.SSHServers[i].User,
			SSHKey:        kaginawa.SSHServers[i].Key,
			SSHPassword:   kaginawa.SSHServers[i].Password,
		}
	}
	rawReply, err := json.MarshalIndent(msg, "", "  ")
	if err != nil {
		http.Error(w, "Response marshal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	if _, err := w.Write(rawReply); err != nil {
		log.Printf("failed to write response: %v", err)
	}

	// TODO: reset reboot request
}

func reverseLookup(globalIP string) (string, error) {
	names, err := net.LookupAddr(globalIP)
	if err != nil {
		return "", err
	}
	if len(names) == 0 {
		return globalIP, nil // no lookup address
	}
	return strings.TrimRight(names[0], "."), nil
}

func trimPort(addr string) string {
	i := strings.LastIndex(addr, ":")
	if i > 0 {
		return addr[0:i]
	}
	return addr
}
