package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kaginawa/kaginawa-server"
)

var (
	testAPIKey = "test-key"
	testReport = kaginawa.Report{
		ID:       "TEST",
		Success:  true,
		Sequence: 1,
	}
)

func TestHandleNodes_ok(t *testing.T) {
	initTemplate("../../template")

	// Prepare database
	db = kaginawa.NewMemDB()
	if err := db.PutReport(testReport); err != nil {
		t.Fatalf("failed to put test data: %v", err)
	}
	if err := db.PutAPIKey(kaginawa.APIKey{Key: testAPIKey, Label: "admin key", Admin: true}); err != nil {
		t.Fatalf("failed to put test key: %v", err)
	}

	// Build request
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/nodes", nil)
	req.Header.Set("Accept", contentTypeJSON)
	req.Header.Set("Authorization", "token "+testAPIKey)
	w := httptest.NewRecorder()

	// Execute
	handleNodes(w, req)
	resp := w.Result()

	// Validate
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	defer safeClose(resp.Body, "body")
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	var result []kaginawa.Report
	if err := json.Unmarshal(body, &result); err != nil {
		t.Errorf("failed to unmarshal response: %s", string(body))
	}
	if len(result) != 1 {
		t.Errorf("expected length of resut is %d, got %d", 1, len(result))
	}
}

func TestHandleNodes_limitedAccess(t *testing.T) {
	initTemplate("../../template")

	// Prepare database
	db = kaginawa.NewMemDB()
	if err := db.PutAPIKey(kaginawa.APIKey{Key: testAPIKey, Label: "Test API Key"}); err != nil {
		t.Fatalf("failed to put test key: %v", err)
	}

	// Build request
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/nodes", nil)
	req.Header.Set("Accept", contentTypeJSON)
	req.Header.Set("Authorization", "token "+testAPIKey)
	w := httptest.NewRecorder()

	// Execute
	handleNodes(w, req)
	resp := w.Result()

	// Validate
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}
