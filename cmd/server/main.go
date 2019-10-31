package main

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/gorilla/mux"
	"github.com/kaginawa/kaginawa-server"
	"golang.org/x/crypto/sha3"
)

const defaultPort = "8080"

var (
	database   *db
	loginUser  *url.Userinfo
	loginToken [32]byte
)

func main() {
	// Initialize database
	ep := os.Getenv("MONGODB_URI")
	if len(ep) == 0 {
		log.Fatalf("Database not configured!")
	}
	parsed, err := url.Parse(ep)
	if err != nil {
		log.Fatalf("invalid MONGODB_URI: %s", ep)
	}
	loginUser = parsed.User
	loginToken = sha3.Sum256([]byte(loginUser.Username()))
	db, err := newDB(ep)
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	database = db

	// Load api keys
	apiKeys, err := db.listAPIKeys()
	if err != nil {
		log.Fatalf("failed to list api keys: %v", err)
	}
	log.Printf("%d api keys loaded.", len(apiKeys))

	// Load ssh servers
	servers, err := db.listSSHServers()
	if err != nil {
		log.Fatalf("failed to list ssh servers: %v", err)
	}
	log.Printf("%d ssh servers loaded.", len(servers))

	// Start listing
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	r := mux.NewRouter()
	r.HandleFunc("/", handleIndex)
	r.HandleFunc("/favicon.ico", handleFavicon)
	r.HandleFunc("/report", handleReport)
	r.HandleFunc("/login", handleLogin)
	r.HandleFunc("/list", handleList)
	r.HandleFunc("/node/{id}", handleNode)
	r.HandleFunc("/admin", handleAdmin)
	r.HandleFunc("/new-key", handleNewAPIKey)
	r.HandleFunc("/new-server", handleNewSSHServer)
	r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets"))))
	log.Printf("Starting kaginawa server %s at port %s", kaginawa.Version(), port)
	log.Println(http.ListenAndServe(":"+port, r))
}

func safeClose(closer io.Closer, name string) {
	if err := closer.Close(); err != nil {
		log.Printf("failed to close %s: %v", name, err)
	}
}
