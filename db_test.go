package kaginawa

import "testing"

func TestServer(t *testing.T) {
	server := SSHServer{
		Host: "localhost",
		Port: 22,
	}
	if server.Addr() != "localhost:22" {
		t.Errorf("expected Addr() = %s, got %s", "localhost:22", server.Addr())
	}
}

func TestDB_SSHServers(t *testing.T) {
	var db DB = NewMemDB()
	if err := db.PutSSHServer(SSHServer{
		Host:     "localhost",
		Port:     22,
		User:     "foo",
		Password: "bar",
	}); err != nil {
		t.Fatal(err)
	}
	servers, err := db.ListSSHServers()
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 {
		t.Errorf("expected len(servers) = %d, got %d", 1, len(servers))
	}
	server, err := db.GetSSHServerByHost("localhost")
	if err != nil {
		t.Fatal(err)
	}
	if server == nil {
		t.Error("expected GetSSHServerByHost(localhost) = non-nil, got nil")
	}
	if server.Host != "localhost" {
		t.Errorf("expected server.Host is %s, got %s", "localhost", server.Host)
	}
}

func TestDB_Reports(t *testing.T) {
	var db DB = NewMemDB()
	if err := db.PutReport(Report{
		ID:      "f0:18:98:eb:c7:27",
		Trigger: 3,
		Success: true,
	}); err != nil {
		t.Fatal(err)
	}
	count1, err := db.CountReports()
	if err != nil {
		t.Fatal(err)
	}
	if count1 != 1 {
		t.Errorf("expected CountRepots() = %d, got %d", 1, count1)
	}
	reports1, err := db.ListReports(0, 0, 0, ListViewAttributes)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports1) != 1 {
		t.Errorf("expected len(ListReports()) = %d, got %d", 1, len(reports1))
	}
	reports2, count2, err := db.CountAndListReports(0, 0, 0, ListViewAttributes)
	if err != nil {
		t.Fatal(err)
	}
	if count2 != 1 || len(reports2) != 1 {
		t.Errorf("expected CountAndListReports()/len = %d/%d, got %d/%d", 1, 1, count2, len(reports2))
	}
}

func TestDB_UserSessions(t *testing.T) {
	var db = NewMemDB()
	if err := db.PutUserSession(UserSession{
		ID: "test-session",
	}); err != nil {
		t.Fatal(err)
	}
	s, err := db.GetUserSession("test-session")
	if err != nil {
		t.Fatal(err)
	}
	if s == nil {
		t.Fatal("expected GetUserSession is non-nil, got nil")
	}
	if s.ID != "test-session" {
		t.Errorf("expected UserSession.ID is %s, got %s", "test-session", s.ID)
	}
	if err := db.DeleteUserSession(s.ID); err != nil {
		t.Fatal(err)
	}
	s2, err := db.GetUserSession("test-session")
	if err != nil {
		t.Fatal(err)
	}
	if s2 != nil {
		t.Errorf("expected GetUserSession after delete is nil, got %v", s2)
	}
}
