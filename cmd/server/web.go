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
	templateExt     = ".html"
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
		"b_fmt": func(bytes interface{}) string {
			var b uint64
			switch bytes.(type) {
			case uint64:
				b = bytes.(uint64)
			case int64:
				b = uint64(bytes.(int64))
			}
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

type indexParams struct {
	Meta       meta
	RemoteIP   string
	RemoteHost string
	FindError  string
}

type meta struct {
	Title         string
	UserName      string
	GoVersion     string
	NumGoroutines int
	MemStats      runtime.MemStats
}

func newMeta(r *http.Request, title string) meta {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return meta{
		Title:         "Kaginawa Server | " + title,
		UserName:      getSession(r).name(),
		GoVersion:     runtime.Version(),
		NumGoroutines: runtime.NumGoroutine(),
		MemStats:      mem,
	}
}

func initTemplate(dir string) {
	for _, name := range loadTemplates(dir) {
		templates[name] = parseTemplate(name, dir)
	}
}

func remoteIP(r *http.Request) (ip string) {
	forwardedFor := r.Header.Get("X-Forwarded-For")
	if len(forwardedFor) > 0 {
		list := strings.Split(forwardedFor, ",")
		ip = list[len(list)-1]
	}
	if len(ip) == 0 {
		ip = trimPort(r.RemoteAddr)
	}
	return
}

// handleIndex handles index requests.
//
// - Method: GET or HEAD
// - Client: Any
// - Access: Public
// - Response: HTML
func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	ip := remoteIP(r)
	host, err := reverseLookup(ip)
	if err != nil {
		log.Printf("failed to execute reverse lookup for %s: %v", ip, err)
	}
	execTemplate(w, "index", indexParams{
		newMeta(r, "Welcome"),
		ip,
		host,
		"",
	})
}

// handleFavicon handles favicon requests.
//
// - Method: Any
// - Client: Any
// - Access: Public
// - Response: File
func handleFavicon(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "assets/favicon.ico")
}

// handleFind handles node finding requests.
//
// - Method: POST
// - Client: Browser
// - Access: Admin
// - Response: 303 redirect
func handleFind(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if !getSession(r).isLoggedIn() {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		log.Printf("failed to parse form: %v", err)
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}
	findBy := r.FormValue("find-by")
	findString := strings.TrimSpace(r.FormValue("find-string"))
	if len(findString) == 0 {
		handleFindError(w, r, "Please input find string")
		return
	}
	switch findBy {
	case "id":
		report, err := db.GetReportByID(findString)
		if err != nil {
			handleFindError(w, r, "Database unavailable")
			return
		}
		if report == nil {
			handleFindError(w, r, "Not found: "+findString)
			return
		}
		http.Redirect(w, r, "/nodes/"+report.ID, http.StatusSeeOther)
	case "custom-id":
		reports, err := db.ListReportsByCustomID(findString, -1, kaginawa.IDAttributes)
		if err != nil {
			handleFindError(w, r, "Database unavailable")
			return
		}
		handleFindResult(w, r, findBy, findString, reports)
	case "hostname":
		matches, err := kaginawa.MatchReports(db, 0, kaginawa.ListViewAttributes, func(r kaginawa.Report) bool {
			return r.Hostname == findString
		})
		if err != nil {
			handleFindError(w, r, "Database unavailable")
			return
		}
		handleFindResult(w, r, findBy, findString, matches)
	case "global-addr":
		matches, err := kaginawa.MatchReports(db, 0, kaginawa.ListViewAttributes, func(r kaginawa.Report) bool {
			return r.GlobalIP == findString || r.GlobalHost == findString
		})
		if err != nil {
			handleFindError(w, r, "Database unavailable")
			return
		}
		handleFindResult(w, r, findBy, findString, matches)
	case "local-addr":
		matches, err := kaginawa.MatchReports(db, 0, kaginawa.ListViewAttributes, func(r kaginawa.Report) bool {
			return r.LocalIPv4 == findString || r.LocalIPv6 == findString
		})
		if err != nil {
			handleFindError(w, r, "Database unavailable")
			return
		}
		handleFindResult(w, r, findBy, findString, matches)
	case "version":
		if !strings.HasPrefix(findString, "v") {
			findString = "v" + findString
		}
		matches, err := kaginawa.MatchReports(db, 0, kaginawa.ListViewAttributes, func(r kaginawa.Report) bool {
			return r.AgentVersion == findString
		})
		if err != nil {
			handleFindError(w, r, "Database unavailable")
			return
		}
		handleFindResult(w, r, findBy, findString, matches)
	default:
		handleFindError(w, r, "Unknown option: "+findBy)
	}
}

func handleFindError(w http.ResponseWriter, r *http.Request, msg string) {
	ip := remoteIP(r)
	host, err := reverseLookup(ip)
	if err != nil {
		log.Printf("failed to execute reverse lookup for %s: %v", ip, err)
	}
	execTemplate(w, "index", indexParams{
		newMeta(r, "Welcome"),
		ip,
		host,
		msg,
	})
}

func handleFindResult(w http.ResponseWriter, r *http.Request, findBy, findString string, matches []kaginawa.Report) {
	switch len(matches) {
	case 0:
		handleFindError(w, r, "Not found: "+findString)
	case 1:
		http.Redirect(w, r, "/nodes/"+matches[0].ID, http.StatusSeeOther)
	default:
		http.Redirect(w, r, "/nodes?"+findBy+"="+findString, http.StatusSeeOther)
	}
}

// handleNodes handles list of nodes requests.
//
// - Method: GET or HEAD
// - Client: Browser or API
// - Access: Admin
// - Response: HTML or JSON
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
	if !getSession(r).isLoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	page := page(r)
	limit := limit(r)
	offset := (page - 1) * limit
	customID := r.URL.Query().Get("custom-id")
	hostname := r.URL.Query().Get("hostname")
	globalAddr := r.URL.Query().Get("global-addr")
	localAddr := r.URL.Query().Get("local-addr")
	version := r.URL.Query().Get("version")
	minutesStr := r.URL.Query().Get("minutes")
	minutes := 0
	var err error
	if len(minutesStr) > 0 {
		minutes, err = strconv.Atoi(minutesStr)
		if err != nil {
			http.Error(w, "Invalid parameter: minutes = "+minutesStr, http.StatusBadRequest)
			return
		}
	}
	filtered := minutes > 0
	var reports []kaginawa.Report
	var count int
	if len(customID+hostname+globalAddr+localAddr+version) == 0 {
		reports, count, err = db.CountAndListReports(offset, limit, minutes, kaginawa.ListViewAttributes)
		if err != nil {
			log.Printf("failed to list reports: %v", err)
			http.Error(w, "Database unavailable", http.StatusInternalServerError)
			return
		}
	} else if len(customID) > 0 && len(hostname+globalAddr+localAddr+version) == 0 {
		matches, err := db.ListReportsByCustomID(customID, minutes, kaginawa.ListViewAttributes)
		if err != nil {
			log.Printf("failed to list reports: %v", err)
			http.Error(w, "Database unavailable", http.StatusInternalServerError)
			return
		}
		count = len(matches)
		reports = kaginawa.SubReports(matches, offset, limit)
		filtered = true
	} else {
		if !strings.HasPrefix(version, "v") {
			version = "v" + version
		}
		matches, err := kaginawa.MatchReports(db, minutes, kaginawa.ListViewAttributes, func(r kaginawa.Report) bool {
			if len(customID) > 0 && r.CustomID == customID {
				return true
			}
			if len(hostname) > 0 && r.Hostname == hostname {
				return true
			}
			if len(globalAddr) > 0 && (r.GlobalIP == globalAddr || r.GlobalHost == globalAddr) {
				return true
			}
			if len(localAddr) > 0 && (r.LocalIPv4 == localAddr || r.LocalIPv6 == localAddr) {
				return true
			}
			if len(version) > 0 && r.AgentVersion == version {
				return true
			}
			return false
		})
		if err != nil {
			log.Printf("failed to list reports: %v", err)
			http.Error(w, "Database unavailable", http.StatusInternalServerError)
			return
		}
		count = len(matches)
		reports = kaginawa.SubReports(matches, offset, limit)
		filtered = true
	}
	execTemplate(w, "nodes", struct {
		Meta     meta
		Pager    Pager
		Reports  []kaginawa.Report
		Filtered bool
	}{
		newMeta(r, "List of Nodes"),
		newPager(count, len(reports), page, limit, r.URL.Query()),
		reports,
		filtered,
	})
}

func handleNodesAPI(w http.ResponseWriter, r *http.Request) {
	if !validateAPIKey(r, true) {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	customID := r.URL.Query().Get("custom-id")
	minutesStr := r.URL.Query().Get("minutes")
	minutes := 0
	if len(minutesStr) > 0 {
		if n, err := strconv.Atoi(minutesStr); err == nil {
			minutes = n
		}
	}
	projection := kaginawa.AllAttributes
	switch r.URL.Query().Get("projection") {
	case "id":
		projection = kaginawa.IDAttributes
	case "list-view":
		projection = kaginawa.ListViewAttributes
	case "measurement":
		projection = kaginawa.MeasurementAttributes
	}
	var reports []kaginawa.Report
	if len(customID) > 0 {
		records, err := db.ListReportsByCustomID(customID, minutes, projection)
		if err != nil {
			log.Printf("failed to list reports: %v", err)
			http.Error(w, "Database unavailable", http.StatusInternalServerError)
			return
		}
		reports = records
	} else {
		count, err := db.CountReports()
		if err != nil {
			log.Printf("failed to count reports: %v", err)
			http.Error(w, "Database unavailable", http.StatusInternalServerError)
			return
		}
		records, err := db.ListReports(0, count, minutes, projection)
		if err != nil {
			log.Printf("failed to list reports: %v", err)
			http.Error(w, "Database unavailable", http.StatusInternalServerError)
			return
		}
		reports = records
	}
	if reports == nil {
		reports = []kaginawa.Report{}
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

// handleNodes handles single node requests.
//
// - Method: GET or HEAD
// - Client: Browser or API
// - Access: Admin
// - Response: HTML or JSON
func handleNode(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if len(id) == 0 {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get("Accept") == contentTypeJSON {
		handleNodeAPI(w, r, id)
	} else {
		handleNodeWeb(w, r, id, "", "", "")
	}
}

func handleNodeWeb(w http.ResponseWriter, r *http.Request, id, user, password, response string) {
	if !getSession(r).isLoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	rep, err := db.GetReportByID(id)
	if err != nil {
		log.Printf("failed to get Report (id=%s): %v", id, err)
		http.Error(w, "Database unavailable", http.StatusInternalServerError)
		return
	}
	execTemplate(w, "node", struct {
		Meta     meta
		Report   kaginawa.Report
		User     string
		Password string
		Response string
	}{
		newMeta(r, "Node Detail"),
		*rep,
		user,
		password,
		response,
	})
}

func handleNodeAPI(w http.ResponseWriter, r *http.Request, id string) {
	apiKey := strings.Replace(r.Header.Get("Authorization"), "token ", "", 1)
	if len(apiKey) == 0 {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	if !validateAPIKey(r, true) {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	record, err := db.GetReportByID(id)
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

// handleAdmin handles admin console requests.
//
// - Method: GET or HEAD
// - Client: Browser
// - Access: Admin
// - Response: HTML
func handleAdmin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if !getSession(r).isLoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	keys, err := db.ListAPIKeys()
	if err != nil {
		log.Printf("failed to list api keys: %v", err)
		http.Error(w, "Database unavailable", http.StatusInternalServerError)
		return
	}
	servers, err := db.ListSSHServers()
	if err != nil {
		log.Printf("failed to list ssh servers: %v", err)
		http.Error(w, "Database unavailable", http.StatusInternalServerError)
		return
	}
	execTemplate(w, "admin", struct {
		Meta       meta
		APIKeys    []kaginawa.APIKey
		SSHServers []kaginawa.SSHServer
	}{
		newMeta(r, "Admin"),
		keys,
		servers,
	})
}

// handleNewAPIKey handles API key creation requests.
//
// - Method: POST
// - Client: Browser
// - Access: Admin
// - Response: 303 redirect
func handleNewAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if !getSession(r).isLoggedIn() {
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
	if err := db.PutAPIKey(kaginawa.APIKey{Key: k, Label: l, Admin: a}); err != nil {
		log.Printf("failed to put api key: %v", err)
		http.Error(w, "Database unavailable", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// handleNewAPIKey handles SSH server registration requests.
//
// - Method: POST
// - Client: Browser
// - Access: Admin
// - Response: 303 redirect
func handleNewSSHServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if !getSession(r).isLoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
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
	if err := db.PutSSHServer(kaginawa.SSHServer{Host: h, Port: port, User: u, Key: k, Password: pw}); err != nil {
		log.Printf("failed to put ssh server: %v", err)
		http.Error(w, "Database unavailable", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// handleSSHServer handles single SSH server requests.
//
// - Method: GET, HEAD
// - Client: API
// - Access: Admin
// - Response: JSON
func handleSSHServer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if len(id) == 0 {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if !validateAPIKey(r, true) {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	server, err := db.GetSSHServerByHost(id)
	if err != nil {
		log.Printf("failed to get ssh server: %v", err)
		http.Error(w, "Database unavailable", http.StatusInternalServerError)
		return
	}
	if server == nil {
		http.NotFound(w, r)
		return
	}
	body, err := json.Marshal(server)
	if err != nil {
		log.Printf("failed to marshal response: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
	w.Header().Add("Content-Type", contentTypeJSON)
	if _, err := w.Write(body); err != nil {
		log.Printf("failed to write body: %v", err)
	}
}

// handleHistories handles list of histories for specified node.
//
// - Method: GET
// - Client: Browser or API
// - Access: Admin
// - Response: JSON
func handleHistories(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if len(id) == 0 {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if !validateAPIKey(r, true) {
		if !getSession(r).isLoggedIn() {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
	}
	end := time.Now()
	endParam := r.URL.Query().Get("end")
	if len(endParam) > 0 {
		if raw, err := strconv.ParseInt(endParam, 10, 64); err == nil {
			end = time.Unix(raw, 0)
		}
	}
	begin := end.AddDate(0, 0, -1)
	beginParam := r.URL.Query().Get("begin")
	if len(beginParam) > 0 {
		if raw, err := strconv.ParseInt(beginParam, 10, 64); err == nil {
			begin = time.Unix(raw, 0)
		}
	}
	projection := kaginawa.AllAttributes
	switch r.URL.Query().Get("projection") {
	case "id":
		projection = kaginawa.IDAttributes
	case "list-view":
		projection = kaginawa.ListViewAttributes
	case "measurement":
		projection = kaginawa.MeasurementAttributes
	}
	logs, err := db.ListHistory(id, begin, end, projection)
	if err != nil {
		log.Printf("failed to query history: %v", err)
		http.Error(w, "Database unavailable", http.StatusInternalServerError)
		return
	}
	body, err := json.Marshal(logs)
	if err != nil {
		log.Printf("failed to marshal response: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
	w.Header().Add("Content-Type", contentTypeJSON)
	if _, err := w.Write(body); err != nil {
		log.Printf("failed to write body: %v", err)
	}
}

// handleNodeDelete handles delete request from web form.
//
// - Method: POST
// - Client: Browser
// - Access: Admin
// - Response: 303 redirect
func handleNodeDelete(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if len(id) == 0 {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if !getSession(r).isLoggedIn() {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	if err := db.DeleteReport(id); err != nil {
		log.Printf("failed to delete report %s: %v", id, err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
	http.Redirect(w, r, "/nodes", http.StatusSeeOther)
}

func loadTemplates(dir string) []string {
	dirEntries, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatalf("failed to read template directory \"%s\": %v", dir, err)
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

func parseTemplate(n, dir string) *template.Template {
	list := []string{
		dir + "/" + n + templateExt,
		dir + "/_header" + templateExt,
		dir + "/_footer" + templateExt,
	}
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
