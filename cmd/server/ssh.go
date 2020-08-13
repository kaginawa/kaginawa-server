package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/kaginawa/kaginawa-server"
	"golang.org/x/crypto/ssh"
)

const (
	defaultTimeoutSec = 30
	eofRetries        = 3
)

var eofError = errors.New("EOF")

type commandResponse struct {
	data []byte
	err  error
}

// handleCommand handles execute a command via ssh.
//
// - Method: HEAD
// - Client: Browser or API
// - Access: Admin
// - Response: Text
func handleCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	id := mux.Vars(r)["id"]
	if len(id) == 0 {
		http.NotFound(w, r)
		return
	}

	// Validate API key or session
	browser := false
	if !validateAPIKey(r, true) {
		if !getSession(r).isLoggedIn() {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		browser = true
	}

	// Target information
	if err := r.ParseForm(); err != nil {
		log.Printf("failed to parse form: %v", err)
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}
	user := strings.TrimSpace(r.FormValue("user"))
	password := strings.TrimSpace(r.FormValue("password"))
	key := strings.TrimSpace(r.FormValue("key"))
	command := strings.TrimSpace(r.FormValue("command"))
	timeoutSec := strings.TrimSpace(r.FormValue("timeout"))
	if len(user) == 0 {
		http.Error(w, "User name required", http.StatusBadRequest)
		return
	}
	if len(command) == 0 {
		http.Error(w, "Command required", http.StatusBadRequest)
		return
	}
	timeout := time.Duration(defaultTimeoutSec) * time.Second
	if len(timeoutSec) > 0 {
		n, err := strconv.Atoi(timeoutSec)
		if err != nil || n < 1 {
			http.Error(w, "Invalid timeout value", http.StatusBadRequest)
			return
		}
		timeout = time.Duration(n) * time.Second
	}
	targetConfig, err := createSSHConfig(user, key, password)
	if err != nil {
		log.Printf("failed to parse key: %v", err)
		http.Error(w, "Invalid ssh key", http.StatusBadRequest)
		return
	}

	// Get record
	report, err := db.GetReportByID(id)
	if err != nil {
		log.Printf("failed to get report %s: %v", id, err)
		http.Error(w, "Database unavailable", http.StatusInternalServerError)
		return
	}
	if report.SSHRemotePort < 1 {
		http.Error(w, "SSH not connected", http.StatusServiceUnavailable)
		return
	}

	// Get ssh server information
	servers, err := db.ListSSHServers()
	if err != nil {
		log.Printf("failed to list ssh servers: %v", err)
		http.Error(w, "Database unavailable", http.StatusInternalServerError)
		return
	}
	var server kaginawa.SSHServer
	for _, s := range servers {
		if s.Host == report.SSHServerHost {
			server = s
		}
	}
	if len(server.Host) == 0 {
		log.Printf("server %s is currently unavailable", report.SSHServerHost)
		http.Error(w, "SSH server unavailable", http.StatusServiceUnavailable)
		return
	}

	serverConfig, err := createSSHConfig(server.User, server.Key, server.Password)
	if err != nil {
		log.Printf("failed to parse server key: %v", err)
		http.Error(w, "SSH server configuration error", http.StatusServiceUnavailable)
		return
	}
	var resp []byte
	eofCount := 0
	for {
		resp, err = execWithTimeout(server, report, serverConfig, targetConfig, command, timeout)
		if err != nil {
			if err == eofError {
				eofCount++
				if eofCount >= eofRetries {
					log.Printf("EOF occurred %d times", eofCount)
					http.Error(w, fmt.Sprintf("EOF occurred %d times", eofCount), http.StatusServiceUnavailable)
					return
				}
				continue
			}
			log.Print(err)
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		break
	}
	if browser {
		handleNodeWeb(w, r, id, user, password, string(resp))
	} else {
		w.Header().Add("Content-Type", "text/plain")
		if _, err := w.Write(resp); err != nil {
			log.Printf("failed to write body: %v", err)
		}
	}
}

func createSSHConfig(user, key, password string) (*ssh.ClientConfig, error) {
	config := ssh.ClientConfig{
		User:            user,
		Auth:            make([]ssh.AuthMethod, 0),
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	if len(key) > 0 {
		parsed, err := ssh.ParsePrivateKey([]byte(key))
		if err != nil {
			return nil, err
		}
		config.Auth = append(config.Auth, ssh.PublicKeys(parsed))
	}
	if len(password) > 0 {
		config.Auth = append(config.Auth, ssh.Password(password))
	}
	return &config, nil
}

func execWithTimeout(s kaginawa.SSHServer, r *kaginawa.Report, sc, tc *ssh.ClientConfig, cmd string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ch := make(chan commandResponse, 1)
	go func() { ch <- exec(s, r, sc, tc, cmd) }()
	select {
	case wrapped := <-ch:
		if wrapped.err != nil {
			return nil, wrapped.err
		}
		return wrapped.data, nil
	case <-ctx.Done():
		return nil, errors.New("timeout")
	}
}

func exec(s kaginawa.SSHServer, r *kaginawa.Report, sc, tc *ssh.ClientConfig, cmd string) commandResponse {
	// Connect to the ssh server
	conn, err := ssh.Dial("tcp", s.Addr(), sc)
	if err != nil {
		return commandResponse{err: fmt.Errorf("failed to connect remote ssh server %s: %w", s.Host, err)}
	}
	defer safeClose(conn, "ssh server connection")

	// Make a TCP connection from ssh server to target node
	targetAddr := fmt.Sprintf("%s:%d", "localhost", r.SSHRemotePort)
	target, err := conn.Dial("tcp", targetAddr)
	if err != nil {
		return commandResponse{err: fmt.Errorf("failed to connect target %s: %w", r.ID, err)}
	}
	c, nc, req, err := ssh.NewClientConn(target, targetAddr, tc)
	if err != nil {
		if strings.HasSuffix(err.Error(), "EOF") {
			return commandResponse{err: eofError}
		}
		return commandResponse{err: fmt.Errorf("failed to open target ssh connection %s: %w", r.ID, err)}
	}
	client := ssh.NewClient(c, nc, req)
	defer safeClose(client, "ssh target connection")

	// Exec command
	session, err := client.NewSession()
	if err != nil {
		return commandResponse{err: fmt.Errorf("failed to create ssh session: %w", err)}
	}
	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return commandResponse{err: fmt.Errorf("failed to submit ssh command: %w", err)}
	}
	return commandResponse{data: output}
}
