package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kaginawa/kaginawa-server"
)

const (
	templateDir    = "template"
	templateExt    = ".html"
	authCookieName = "kaginawa-auth"
)

var (
	templates = make(map[string]*template.Template)
	funcMap   = template.FuncMap{
		"time": func(ts int64, fmt string) string { return time.Unix(ts, 0).Format(fmt) },
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
		Version string
	}{
		kaginawa.Version(),
	})
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodHead:
		fallthrough
	case http.MethodGet:
		execTemplate(w, "login", struct {
			Version string
		}{
			kaginawa.Version(),
		})
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
		http.Redirect(w, r, "/list", http.StatusSeeOther)
		log.Print("user login")
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}

func handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if !validateCookie(r) {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	reports, err := database.listReports()
	if err != nil {
		log.Printf("failed to list reports: %v", err)
		http.Error(w, "Database unavailable", http.StatusInternalServerError)
		return
	}
	execTemplate(w, "list", struct {
		Version string
		Reports []report
	}{
		kaginawa.Version(),
		reports,
	})
}

func handleNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if !validateCookie(r) {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	id := mux.Vars(r)["id"]
	if len(id) == 0 {
		http.NotFound(w, r)
		return
	}
	rep, err := database.getReportByID(id)
	if err != nil {
		log.Printf("failed to get report (id=%s): %v", id, err)
		http.Error(w, "Database unavailable", http.StatusInternalServerError)
		return
	}
	execTemplate(w, "node", struct {
		Version string
		Report  report
	}{
		kaginawa.Version(),
		*rep,
	})
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
	keys, err := database.listAPIKeys()
	if err != nil {
		log.Printf("failed to list api keys: %v", err)
		http.Error(w, "Database unavailable", http.StatusInternalServerError)
		return
	}
	servers, err := database.listSSHServers()
	if err != nil {
		log.Printf("failed to list ssh servers: %v", err)
		http.Error(w, "Database unavailable", http.StatusInternalServerError)
		return
	}
	execTemplate(w, "admin", struct {
		Version    string
		APIKeys    []apiKey
		SSHServers []sshServer
	}{
		kaginawa.Version(),
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
	if len(k) == 0 {
		http.Error(w, "Key is empty", http.StatusBadRequest)
		return
	}
	if err := database.putAPIKey(apiKey{Key: k, Label: l}); err != nil {
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
	if err := database.putSSHServer(sshServer{Host: h, Port: port, User: u, Key: k, Password: pw}); err != nil {
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
