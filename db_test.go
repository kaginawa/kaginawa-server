package kaginawa

import (
	"reflect"
	"testing"
)

func TestServer(t *testing.T) {
	server := SSHServer{
		Host: "localhost",
		Port: 22,
	}
	if server.Addr() != "localhost:22" {
		t.Errorf("expected Addr() = %s, got %s", "localhost:22", server.Addr())
	}
}

func TestReport(t *testing.T) {
	tests := []struct {
		in           Report
		downloadMBPS string
		uploadMBPS   string
		diskUsage    string
		bootTrigger  bool
		conTrigger   bool
		intTrigger   bool
	}{
		{
			in: Report{
				ID:             "f0:18:98:eb:c7:27",
				DownloadKBPS:   10240,
				UploadKBPS:     20480,
				DiskUsedBytes:  100,
				DiskTotalBytes: 200,
				Trigger:        0,
				Success:        true,
			},
			downloadMBPS: "10.0",
			uploadMBPS:   "20.0",
			diskUsage:    "50.0%",
			bootTrigger:  true,
			conTrigger:   false,
			intTrigger:   false,
		},
		{
			in:           Report{ID: "f0:18:98:eb:c7:27", Trigger: -1, Success: true},
			downloadMBPS: "0.0",
			uploadMBPS:   "0.0",
			diskUsage:    "0%",
			bootTrigger:  false,
			conTrigger:   true,
			intTrigger:   false,
		},
		{
			in:           Report{ID: "f0:18:98:eb:c7:27", Trigger: 3, Success: true},
			downloadMBPS: "0.0",
			uploadMBPS:   "0.0",
			diskUsage:    "0%",
			bootTrigger:  false,
			conTrigger:   false,
			intTrigger:   true,
		},
	}
	for i, test := range tests {
		if test.in.DownloadMBPS() != test.downloadMBPS {
			t.Errorf("#%d: expected DownloadMBPS() = %s, got %s", i, test.downloadMBPS, test.in.DownloadMBPS())
		}
		if test.in.UploadMBPS() != test.uploadMBPS {
			t.Errorf("#%d: expected UploadMBPS() = %s, got %s", i, test.uploadMBPS, test.in.UploadMBPS())
		}
		if test.in.DiskUsageAsPercentage() != test.diskUsage {
			t.Errorf("#%d: expected DiskUsageAP() = %s, got %s", i, test.diskUsage, test.in.DiskUsageAsPercentage())
		}
		if test.in.IsBootTimeReport() != test.bootTrigger {
			t.Errorf("#%d: expected IsBootTimeR() = %v, got %v", i, test.bootTrigger, test.in.IsBootTimeReport())
		}
		if test.in.IsSSHConnectedReport() != test.conTrigger {
			t.Errorf("#%d: expected IsSSHConnectedR() = %v, got %v", i, test.conTrigger, test.in.IsSSHConnectedReport())
		}
		if test.in.IsIntervalReport() != test.intTrigger {
			t.Errorf("#%d: expected IsIntervalR() = %v, got %v", i, test.intTrigger, test.in.IsIntervalReport())
		}
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

func TestMatchReports(t *testing.T) {
	var db DB = NewMemDB()
	if err := db.PutReport(Report{
		ID:       "00:00:00:00:00:01",
		CustomID: "test1",
		Trigger:  3,
		Success:  true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.PutReport(Report{
		ID:       "00:00:00:00:00:02",
		CustomID: "test2",
		Trigger:  3,
		Success:  true,
	}); err != nil {
		t.Fatal(err)
	}
	res, err := MatchReports(db, 0, ListViewAttributes, func(r Report) bool {
		return r.CustomID == "test1"
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Errorf("expected len(MatchReports()) = %d, got %d", 1, len(res))
	}
	if len(res) > 0 && res[0].ID != "00:00:00:00:00:01" {
		t.Errorf("expected MatchReports()[0].ID = %s, got %s", "00:00:00:00:00:01", res[0].ID)
	}
}

func TestSubReports(t *testing.T) {
	tests := []struct {
		reports  []Report
		skip     int
		limit    int
		expected []Report
	}{
		{
			reports:  []Report{{ID: "00:00:00:00:00:01"}, {ID: "00:00:00:00:00:02"}, {ID: "00:00:00:00:00:03"}},
			skip:     1,
			limit:    1,
			expected: []Report{{ID: "00:00:00:00:00:02"}},
		},
		{
			reports:  []Report{{ID: "00:00:00:00:00:01"}, {ID: "00:00:00:00:00:02"}, {ID: "00:00:00:00:00:03"}},
			skip:     0,
			limit:    0,
			expected: []Report{{ID: "00:00:00:00:00:01"}, {ID: "00:00:00:00:00:02"}, {ID: "00:00:00:00:00:03"}},
		},
		{
			reports:  []Report{{ID: "00:00:00:00:00:01"}, {ID: "00:00:00:00:00:02"}, {ID: "00:00:00:00:00:03"}},
			skip:     0,
			limit:    1,
			expected: []Report{{ID: "00:00:00:00:00:01"}},
		},
		{
			reports:  []Report{{ID: "00:00:00:00:00:01"}, {ID: "00:00:00:00:00:02"}, {ID: "00:00:00:00:00:03"}},
			skip:     1,
			limit:    0,
			expected: []Report{{ID: "00:00:00:00:00:02"}, {ID: "00:00:00:00:00:03"}},
		},
		{
			reports:  nil,
			skip:     0,
			limit:    0,
			expected: nil,
		},
	}
	for i, test := range tests {
		actual := SubReports(test.reports, test.skip, test.limit)
		if !reflect.DeepEqual(actual, test.expected) {
			t.Errorf("#%d: expected %v, got %v", i, test.expected, actual)
		}
	}
}
