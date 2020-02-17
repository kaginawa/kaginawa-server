package main

import (
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/kaginawa/kaginawa-server"
)

const defaultPort = "8080"

var db kaginawa.DB

func main() {
	// Initialize html template
	initTemplate("template")

	// Initialize OAuth
	if err := initOAuth(); err != nil {
		log.Fatal(err)
	}

	// Initialize session
	if err := initSession(); err != nil {
		log.Fatal(err)
	}

	// Initialize database
	mongoURI := os.Getenv("MONGODB_URI")
	dynamoKeys := os.Getenv("DYNAMO_KEYS")
	if len(mongoURI) > 0 {
		mongoDB, err := kaginawa.NewMongoDB(mongoURI)
		if err != nil {
			log.Fatalf("failed to initialize database: %v", err)
		}
		db = mongoDB
	} else if len(dynamoKeys) > 0 {
		dynamoDB, err := kaginawa.NewDynamoDB()
		if err != nil {
			log.Fatalf("failed to initialize database: %v", err)
		}
		db = dynamoDB
	} else {
		log.Fatal("Database not configured!")
	}

	// Load api keys
	apiKeys, err := db.ListAPIKeys()
	if err != nil {
		log.Fatalf("failed to list api keys: %v", err)
	}
	log.Printf("%d api keys loaded.", len(apiKeys))

	// Load ssh servers
	servers, err := db.ListSSHServers()
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
	r.HandleFunc("/login", handleOAuthLogin)
	r.HandleFunc("/callback", handleOAuthLoginCallback)
	r.HandleFunc("/logout", handleOAuthLogout)
	r.HandleFunc("/logout-complete", handleOAuthLogoutComplete)
	r.HandleFunc("/nodes", handleNodes)
	r.HandleFunc("/nodes/{id}", handleNode)
	r.HandleFunc("/nodes/{id}/command", handleCommand)
	r.HandleFunc("/nodes/{id}/histories", handleHistories)
	r.HandleFunc("/admin", handleAdmin)
	r.HandleFunc("/new-key", handleNewAPIKey)
	r.HandleFunc("/new-server", handleNewSSHServer)
	r.HandleFunc("/servers/{id}", handleSSHServer)
	r.HandleFunc("/measure/{kb}", handleMeasure)
	r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets"))))
	log.Printf("Starting kaginawa server at port %s", port)
	log.Println(http.ListenAndServe(":"+port, r))
}

func safeClose(closer io.Closer, name string) {
	if err := closer.Close(); err != nil {
		log.Printf("failed to close %s: %v", name, err)
	}
}

func getEnvs(keys ...string) string {
	for _, key := range keys {
		value := os.Getenv(key)
		if len(value) > 0 {
			return value
		}
	}
	return ""
}
