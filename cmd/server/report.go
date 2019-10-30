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
)

// report defines all of report attributes
type report struct {
	// Kagiwana shared fields
	ID             string   `json:"id" bson:"id"`                             // MAC address of the primary network interface
	Runtime        string   `json:"runtime" bson:"runtime"`                   // OS and arch
	Success        bool     `json:"success" bson:"success"`                   // Equals len(Errors) == 0
	Sequence       int      `json:"seq" bson:"seq"`                           // Report sequence number, resets by reboot
	DeviceTime     int64    `json:"device_time" bson:"device_time"`           // Device time (UTC) by time.Now().UTC().Unix()
	BootTime       int64    `json:"boot_time" bson:"boot_time"`               // Device boot time (UTC)
	GenMillis      int64    `json:"gen_ms" bson:"gen_ms"`                     // Generation time milliseconds
	AgentVersion   string   `json:"agent_version" bson:"agent_version"`       // Agent version
	CustomID       string   `json:"custom_id" bson:"custom_id"`               // User specified ID
	SSHServerHost  string   `json:"ssh_server_host" bson:"ssh_server_host"`   // Connected SSH server host
	SSHRemotePort  int      `json:"ssh_remote_port" bson:"ssh_remote_port"`   // Connected SSH remote port
	SSHConnectTime int64    `json:"ssh_connect_time" bson:"ssh_connect_time"` // Connected time of the SSH
	Adapter        string   `json:"adapter" bson:"adapter"`                   // Name of network adapter
	LocalIPv4      string   `json:"ip4_local" bson:"ip4_local"`               // Local IPv6 address
	LocalIPv6      string   `json:"ip6_local" bson:"ip6_local"`               // Local IPv6 address
	Hostname       string   `json:"hostname" bson:"hostname"`                 // OS Hostname
	PingMills      float64  `json:"ping_ms" bson:"ping_ms"`                   // Ping latency milliseconds
	PingTarget     string   `json:"ping_target" bson:"ping_target"`           // Ping target for result
	Errors         []string `json:"errors" bson:"errors"`                     // List of errors
	Payload        string   `json:"payload" bson:"payload"`                   // Custom content provided by payload command
	PayloadCmd     string   `json:"payload_cmd" bson:"payload_cmd"`           // Executed payload command

	// Server-side injected fields
	GlobalIP   string `json:"ip_global" bson:"ip_global"`     // Global IP address
	GlobalHost string `json:"host_global" bson:"host_global"` // Reverse lookup result for global IP address
	ServerTime int64  `json:"server_time" bson:"server_time"` // Server-side consumed UTC time
}

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
	apiKey := strings.Replace(r.Header.Get("Authorization"), "token ", "", 1)
	if len(apiKey) == 0 {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	if ok, _, err := database.validateAPIKey(apiKey); !ok || err != nil {
		if err != nil {
			log.Printf("failed to validate api key: %v", err)
			http.Error(w, "Database unavailable", http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("failed to read request: %v", err)
		http.Error(w, "Response read error", http.StatusInternalServerError)
		return
	}
	defer safeClose(r.Body, "report body")
	var report report
	if err := json.Unmarshal(body, &report); err != nil {
		http.Error(w, "Response unmarshal error", http.StatusBadRequest)
		return
	}
	report.ServerTime = time.Now().UTC().Unix()
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

	if err := database.putReport(report); err != nil {
		log.Printf("failed to put report (id=%s): %v", report.ID, err)
		http.Error(w, "Failed to put database", http.StatusInternalServerError)
		return
	}

	// TODO: implement reboot request feature

	var msg reply
	if len(sshServers) > 0 {
		i := rand.Int() % len(sshServers)
		msg = reply{
			Reboot:        false,
			SSHServerHost: sshServers[i].Host,
			SSHServerPort: sshServers[i].Port,
			SSHServerUser: sshServers[i].User,
			SSHKey:        sshServers[i].Key,
			SSHPassword:   sshServers[i].Password,
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
