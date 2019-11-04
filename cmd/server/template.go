package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/kaginawa/kaginawa-server"
)

const (
	templateDir     = "template"
	templateExt     = ".html"
	authCookieName  = "kaginawa-auth"
	contentTypeJSON = "application/json"
)

var (
	templates = make(map[string]*template.Template)
	funcMap   = template.FuncMap{
		// time format
		"t_fmt": func(ts int64, fmt string) string {
			return time.Unix(ts, 0).Format(fmt)
		},
		// fresh check
		"t_fresh": func(ts int64, min int) bool {
			return time.Unix(ts, 0).After(time.Now().Add(-time.Duration(min) * time.Minute))
		},
		// human readable byte size
		"b_fmt": func(b uint64) string {
			if b > 1024*1024*1024 {
				return fmt.Sprintf("%dGB", b/1024/1024/1024)
			} else if b > 1024*1024 {
				return fmt.Sprintf("%dMB", b/1024/1024)
			} else if b > 1024 {
				return fmt.Sprintf("%dKB", b/1024)
			} else {
				return fmt.Sprintf("%dB", b)
			}
		},
	}
)

func init() {
	for _, name := range loadTemplates() {
		templates[name] = parseTemplate(name)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	execTemplate(w, "index", struct {
	}{})
}

func handleFavicon(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "assets/favicon.ico")
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodHead:
		fallthrough
	case http.MethodGet:
		execTemplate(w, "login", struct {
		}{})
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			log.Printf("failed to parse form: %v", err)
			http.Error(w, "Invalid form", http.StatusBadRequest)
			return
		}
		u := r.FormValue("user")
		p := r.FormValue("password")
		if len(u) == 0 {
			http.Error(w, "User is empty", http.StatusBadRequest)
			return
		}
		if len(p) == 0 {
			http.Error(w, "Password is empty", http.StatusBadRequest)
			return
		}
		if u != loginUser.Username() {
			log.Printf("Invalid login attempt %s:%s", u, p)
			http.Error(w, "Invalid user or password", http.StatusUnauthorized)
			return
		}
		if pw, _ := loginUser.Password(); p != pw {
			log.Printf("Invalid login attempt %s:%s", u, p)
			http.Error(w, "Invalid user or password", http.StatusUnauthorized)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     authCookieName,
			Value:    fmt.Sprintf("%x", loginToken),
			Expires:  time.Now().AddDate(1, 0, 0),
			HttpOnly: true,
		})
		http.Redirect(w, r, "/nodes", http.StatusSeeOther)
		log.Print("user login")
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}

func handleNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get("Accept") == contentTypeJSON {
		handleNodesAPI(w, r)
	} else {
		handleNodesWeb(w, r)
	}
}

func handleNodesWeb(w http.ResponseWriter, r *http.Request) {
	if !validateCookie(r) {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	reports, err := database.ListReports()
	if err != nil {
		log.Printf("failed to list reports: %v", err)
		http.Error(w, "Database unavailable", http.StatusInternalServerError)
		return
	}
	execTemplate(w, "nodes", struct {
		Reports       []kaginawa.Report
		GoVersion     string
		NumGoroutines int
		MemStats      runtime.MemStats
	}{
		reports,
		runtime.Version(),
		runtime.NumGoroutine(),
		mem,
	})
}

func handleNodesAPI(w http.ResponseWriter, r *http.Request) {
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
	customID := r.URL.Query().Get("custom-id")
	reports := make([]kaginawa.Report, 0)
	if len(customID) > 0 {
		records, err := database.GetReportByCustomID(customID)
		if err != nil {
			log.Printf("failed to list reports: %v", err)
			http.Error(w, "Database unavailable", http.StatusInternalServerError)
			return
		}
		reports = records
	} else {
		records, err := database.ListReports()
		if err != nil {
			log.Printf("failed to list reports: %v", err)
			http.Error(w, "Database unavailable", http.StatusInternalServerError)
			return
		}
		reports = records
	}
	body, err := json.Marshal(reports)
	if err != nil {
		log.Printf("failed to marshal response: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
	w.Header().Add("Content-Type", contentTypeJSON)
	if _, err := w.Write(body); err != nil {
		log.Printf("failed to write body: %v", err)
	}
}

func handleNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get("Accept") == contentTypeJSON {
		handleNodeAPI(w, r)
	} else {
		handleNodeWeb(w, r)
	}
}

func handleNodeWeb(w http.ResponseWriter, r *http.Request) {
	if !validateCookie(r) {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	id := mux.Vars(r)["id"]
	if len(id) == 0 {
		http.NotFound(w, r)
		return
	}
	rep, err := database.GetReportByID(id)
	if err != nil {
		log.Printf("failed to get Report (id=%s): %v", id, err)
		http.Error(w, "Database unavailable", http.StatusInternalServerError)
		return
	}
	execTemplate(w, "node", struct {
		Report kaginawa.Report
	}{
		*rep,
	})
}

func handleNodeAPI(w http.ResponseWriter, r *http.Request) {
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
	id := mux.Vars(r)["id"]
	if len(id) == 0 {
		http.NotFound(w, r)
		return
	}
	record, err := database.GetReportByID(id)
	if err != nil {
		log.Printf("failed to get report: %v", err)
		http.Error(w, "Database unavailable", http.StatusInternalServerError)
		return
	}
	if record == nil {
		http.NotFound(w, r)
		return
	}
	body, err := json.Marshal(record)
	if err != nil {
		log.Printf("failed to marshal response: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
	w.Header().Add("Content-Type", contentTypeJSON)
	if _, err := w.Write(body); err != nil {
		log.Printf("failed to write body: %v", err)
	}
}

func handleAdmin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if !validateCookie(r) {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	keys, err := database.ListAPIKeys()
	if err != nil {
		log.Printf("failed to list api keys: %v", err)
		http.Error(w, "Database unavailable", http.StatusInternalServerError)
		return
	}
	servers, err := database.ListSSHServers()
	if err != nil {
		log.Printf("failed to list ssh servers: %v", err)
		http.Error(w, "Database unavailable", http.StatusInternalServerError)
		return
	}
	execTemplate(w, "admin", struct {
		APIKeys    []kaginawa.APIKey
		SSHServers []kaginawa.SSHServer
	}{
		keys,
		servers,
	})
}

func handleNewAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if !validateCookie(r) {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		log.Printf("failed to parse form: %v", err)
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}
	k := strings.TrimSpace(r.FormValue("key"))
	l := strings.TrimSpace(r.FormValue("label"))
	a := r.FormValue("admin") == "yes"
	if len(k) == 0 {
		http.Error(w, "Key is empty", http.StatusBadRequest)
		return
	}
	if err := database.PutAPIKey(kaginawa.APIKey{Key: k, Label: l, Admin: a}); err != nil {
		log.Printf("failed to put api key: %v", err)
		http.Error(w, "Database unavailable", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func handleNewSSHServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if !validateCookie(r) {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		log.Printf("failed to parse form: %v", err)
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}
	h := strings.TrimSpace(r.FormValue("host"))
	p := strings.TrimSpace(r.FormValue("port"))
	u := strings.TrimSpace(r.FormValue("user"))
	k := strings.TrimSpace(r.FormValue("key"))
	pw := strings.TrimSpace(r.FormValue("password"))
	if len(h) == 0 {
		http.Error(w, "Host is empty", http.StatusBadRequest)
		return
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		http.Error(w, fmt.Sprintf("%s is not a port number", p), http.StatusBadRequest)
		return
	}
	if len(u) == 0 {
		http.Error(w, "User is empty", http.StatusBadRequest)
		return
	}
	if len(k) == 0 && len(p) == 0 {
		http.Error(w, "Key or password is empty", http.StatusBadRequest)
		return
	}
	if err := database.PutSSHServer(kaginawa.SSHServer{Host: h, Port: port, User: u, Key: k, Password: pw}); err != nil {
		log.Printf("failed to put ssh server: %v", err)
		http.Error(w, "Database unavailable", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func loadTemplates() []string {
	dirEntries, err := ioutil.ReadDir(templateDir)
	if err != nil {
		log.Fatalf("failed to read template directory \"%s\": %v", templateDir, err)
	}
	var templates []string
	for _, fileInfo := range dirEntries {
		if fileInfo.IsDir() {
			continue
		}
		if !strings.HasSuffix(fileInfo.Name(), templateExt) {
			continue
		}
		templates = append(templates, strings.Replace(fileInfo.Name(), templateExt, "", 1))
	}
	return templates
}

func parseTemplate(n string) *template.Template {
	list := []string{templateDir + "/" + n + templateExt}
	t, err := template.New(n + ".html").Funcs(funcMap).ParseFiles(list...)
	if err != nil {
		panic(err)
	}
	return t
}

func execTemplate(w http.ResponseWriter, name string, attributes interface{}) {
	if err := templates[name].Funcs(funcMap).Execute(w, attributes); err != nil {
		log.Printf("template error in %s: %v", name, err)
		http.Error(w, "Template error: "+name, http.StatusInternalServerError)
	}
}

func validateCookie(r *http.Request) bool {
	cookie, err := r.Cookie(authCookieName)
	if err != nil {
		return false
	}
	return cookie.Value == fmt.Sprintf("%x", loginToken)
}
