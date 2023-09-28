package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kaginawa/kaginawa-server/internal/database"
)

func TestValidateAPIKey(t *testing.T) {
	db = database.NewMemDB()
	if err := db.PutAPIKey(database.APIKey{Key: "test-normal", Label: "normal key label"}); err != nil {
		t.Fatalf("failed to put test data: %v", err)
	}
	if err := db.PutAPIKey(database.APIKey{Key: "test-admin", Label: "admin key label", Admin: true}); err != nil {
		t.Fatalf("failed to put test data: %v", err)
	}

	// normal key tests
	req1 := httptest.NewRequest(http.MethodGet, "http://localhost:8080/nodes", nil)
	req1.Header.Set("Authorization", "token test-normal")
	if !validateAPIKey(req1, false) {
		t.Errorf("unexpected validation result: non-admin key with non-admin option")
	}
	if validateAPIKey(req1, true) {
		t.Errorf("unexpected validation result: non-admin key with admin option")
	}

	// admin key tests
	req2 := httptest.NewRequest(http.MethodGet, "http://localhost:8080/nodes", nil)
	req2.Header.Set("Authorization", "token test-admin")
	if !validateAPIKey(req2, false) {
		t.Errorf("unexpected validation result: admin key with non-admin option")
	}
	if !validateAPIKey(req2, true) {
		t.Errorf("unexpected validation result: admin key with admin option")
	}

	// unregistered key tests
	req3 := httptest.NewRequest(http.MethodGet, "http://localhost:8080/nodes", nil)
	req2.Header.Set("Authorization", "token unknown")
	if validateAPIKey(req3, false) {
		t.Errorf("unexpected validation result: unregistered key with non-admin option")
	}
	if validateAPIKey(req3, true) {
		t.Errorf("unexpected validation result: unregistered key with admin option")
	}
}

func TestExtractAPIKey(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/nodes", nil)
	req.Header.Set("Authorization", "token test-key")
	key := extractAPIKey(req)
	if key != "test-key" {
		t.Errorf("expected %s, got %s", "test-key", key)
	}
}
